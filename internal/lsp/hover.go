package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// hover handler: returns markdown content describing the symbol at the
// cursor. ADR 0032 §O3 → C: type + signature + full contracts + (in
// future) a verification summary line. The verification line is left
// blank in this task; 18.4 hooks Z3 results into the hover.

func (s *Server) handleHover(id json.RawMessage, params json.RawMessage) {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode hover params: %v", err))
		return
	}

	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		// LSP returns null for unknown docs; clients tolerate this.
		s.t.writeResponse(id, nil)
		return
	}

	text := doc.snapshotText()
	line, col := doc.positionToLineCol(p.Position)
	name := wordAtPosition(text, line, col)
	if name == "" {
		s.t.writeResponse(id, nil)
		return
	}

	// Re-parse to get a fresh AST. The diagnostics path already parses on
	// edit; we don't cache the AST yet (deferred — caches add invalidation
	// complexity not worth v1's compute budget). Parse is fast: tens of ms
	// on typical files.
	pp := parser.New(text)
	prog := pp.Parse()

	ownPath := uriToPath(p.TextDocument.URI)
	ws := s.workspaces.workspaceForURI(p.TextDocument.URI)
	siblings := ws.siblingModules(ownPath)

	hit := resolveAcrossWorkspace(prog, ownPath, name, siblings)
	if hit == nil {
		s.t.writeResponse(id, nil)
		return
	}

	s.t.writeResponse(id, Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: renderHover(hit),
		},
	})
}

// renderHover formats a declHit as markdown. The structure for each kind:
//
//	```intent
//	<signature or declaration header>
//	```
//
// followed by a contract block (for functions / methods) if present.
func renderHover(hit *declHit) string {
	var b strings.Builder
	b.WriteString("```intent\n")
	switch hit.Kind {
	case declFunction:
		writeFunctionSig(&b, hit.Function)
	case declExternFunction:
		writeExternSig(&b, hit.ExternFunction)
	case declEntity:
		writeEntityHeader(&b, hit.Entity)
	case declEnum:
		writeEnumHeader(&b, hit.Enum)
	case declTrait:
		writeTraitHeader(&b, hit.Trait)
	case declTest:
		writeTestHeader(&b, hit.Test)
	}
	b.WriteString("\n```")

	// Contract block. ADR 0032 §O3 → C: hover surfaces the full requires/
	// ensures so the caller sees the contract without jumping.
	if reqs, ens := contractsFor(hit); len(reqs)+len(ens) > 0 {
		b.WriteString("\n\n")
		writeContractBlock(&b, reqs, ens)
	}
	return b.String()
}

func writeFunctionSig(b *strings.Builder, fn *ast.FunctionDecl) {
	if fn.IsPublic {
		b.WriteString("public ")
	}
	if fn.IsAsync {
		b.WriteString("async ")
	}
	if fn.IsEntry {
		b.WriteString("entry ")
	}
	b.WriteString("function ")
	b.WriteString(fn.Name)
	writeTypeParams(b, fn.TypeParams)
	b.WriteByte('(')
	writeParams(b, fn.Params)
	b.WriteString(") returns ")
	writeTypeRef(b, fn.ReturnType)
}

func writeExternSig(b *strings.Builder, ext *ast.ExternFunctionDecl) {
	b.WriteString("extern function ")
	b.WriteString(ext.Name)
	b.WriteByte('(')
	writeParams(b, ext.Params)
	b.WriteString(") returns ")
	writeTypeRef(b, ext.ReturnType)
	fmt.Fprintf(b, "\n    from %q", ext.RustPath)
}

func writeEntityHeader(b *strings.Builder, ent *ast.EntityDecl) {
	if ent.IsPublic {
		b.WriteString("public ")
	}
	b.WriteString("entity ")
	b.WriteString(ent.Name)
	writeTypeParams(b, ent.TypeParams)
	if len(ent.Fields) > 0 || len(ent.Invariants) > 0 {
		b.WriteString(" {")
		for _, f := range ent.Fields {
			b.WriteString("\n    field ")
			b.WriteString(f.Name)
			b.WriteString(": ")
			writeTypeRef(b, f.Type)
			b.WriteByte(';')
		}
		for _, inv := range ent.Invariants {
			b.WriteString("\n    invariant ")
			b.WriteString(inv.RawText)
			b.WriteByte(';')
		}
		b.WriteString("\n}")
	}
}

func writeEnumHeader(b *strings.Builder, en *ast.EnumDecl) {
	if en.IsPublic {
		b.WriteString("public ")
	}
	b.WriteString("enum ")
	b.WriteString(en.Name)
	if len(en.Variants) > 0 {
		b.WriteString(" {")
		for _, v := range en.Variants {
			b.WriteString("\n    ")
			b.WriteString(v.Name)
			if len(v.Fields) > 0 {
				b.WriteByte('(')
				for i, f := range v.Fields {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(f.Name)
					b.WriteString(": ")
					writeTypeRef(b, f.Type)
				}
				b.WriteByte(')')
			}
			b.WriteByte(',')
		}
		b.WriteString("\n}")
	}
}

func writeTraitHeader(b *strings.Builder, tr *ast.TraitDecl) {
	if tr.IsPublic {
		b.WriteString("public ")
	}
	b.WriteString("trait ")
	b.WriteString(tr.Name)
	if len(tr.Methods) > 0 {
		b.WriteString(" {")
		for _, m := range tr.Methods {
			b.WriteString("\n    method ")
			b.WriteString(m.Name)
			b.WriteByte('(')
			writeParams(b, m.Params)
			b.WriteString(") returns ")
			writeTypeRef(b, m.ReturnType)
			b.WriteByte(';')
		}
		b.WriteString("\n}")
	}
}

func writeTestHeader(b *strings.Builder, te *ast.TestDecl) {
	for _, ann := range te.Annotations {
		b.WriteByte('@')
		b.WriteString(ann.Name)
		b.WriteByte('(')
		for i, a := range ann.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "%q", a)
		}
		b.WriteString(")\n")
	}
	if te.IsAsync {
		b.WriteString("async ")
	}
	fmt.Fprintf(b, "test %q", te.Name)
}

func writeTypeParams(b *strings.Builder, params []*ast.TypeParam) {
	if len(params) == 0 {
		return
	}
	b.WriteByte('<')
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteByte('>')
}

func writeParams(b *strings.Builder, params []*ast.Param) {
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		b.WriteString(": ")
		writeTypeRef(b, p.Type)
	}
}

func writeTypeRef(b *strings.Builder, t *ast.TypeRef) {
	if t == nil {
		b.WriteString("Void")
		return
	}
	if t.Name == "Fn" {
		b.WriteString("Fn(")
		for i, pt := range t.ParamTypes {
			if i > 0 {
				b.WriteString(", ")
			}
			writeTypeRef(b, pt)
		}
		b.WriteString(") -> ")
		writeTypeRef(b, t.ReturnType)
		return
	}
	b.WriteString(t.Name)
	if len(t.TypeArgs) > 0 {
		b.WriteByte('<')
		for i, a := range t.TypeArgs {
			if i > 0 {
				b.WriteString(", ")
			}
			writeTypeRef(b, a)
		}
		b.WriteByte('>')
	}
}

// contractsFor returns the requires + ensures clauses for whichever kind
// of declaration the hit carries. Entities surface invariants instead —
// rendered in writeEntityHeader, not here.
func contractsFor(hit *declHit) (requires, ensures []*ast.ContractClause) {
	switch hit.Kind {
	case declFunction:
		return hit.Function.Requires, hit.Function.Ensures
	case declExternFunction:
		return hit.ExternFunction.Requires, hit.ExternFunction.Ensures
	}
	return nil, nil
}

func writeContractBlock(b *strings.Builder, reqs, ens []*ast.ContractClause) {
	b.WriteString("**Contracts:**\n\n```intent\n")
	for _, r := range reqs {
		b.WriteString("requires ")
		b.WriteString(r.RawText)
		b.WriteString(";\n")
	}
	for _, e := range ens {
		b.WriteString("ensures ")
		b.WriteString(e.RawText)
		b.WriteString(";\n")
	}
	b.WriteString("```")
}
