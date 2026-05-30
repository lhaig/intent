package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// textDocument/signatureHelp. Phase 19 task 19.5.
//
// Walks back from the cursor to find the unmatched `(` that opens the
// enclosing call. The identifier before that `(` names the callee; the
// number of commas between the `(` and the cursor (at depth 0) is the
// active parameter index. Method calls (`receiver.foo(...)`) resolve via
// the same receiver-type path as hover/goto-def.
//
// Returns null when the cursor is not inside any call, or when the
// callee can't be resolved.

func (s *Server) handleSignatureHelp(id json.RawMessage, params json.RawMessage) {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode signatureHelp params: %v", err))
		return
	}
	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, nil)
		return
	}
	text := doc.snapshotText()
	line, col := doc.positionToLineCol(p.Position)

	callee, activeParam, ok := findEnclosingCall(text, line, col)
	if !ok {
		s.t.writeResponse(id, nil)
		return
	}

	pp := parser.New(text)
	prog := pp.Parse()
	ownPath := uriToPath(p.TextDocument.URI)
	ws := s.workspaces.workspaceForURI(p.TextDocument.URI)
	siblings := ws.siblingModules(ownPath)
	scope := newScopeResolver(prog, ownPath)

	sig := resolveSignature(prog, scope, text, ownPath, callee, line, col, siblings)
	if sig == nil {
		s.t.writeResponse(id, nil)
		return
	}
	s.t.writeResponse(id, SignatureHelp{
		Signatures:      []SignatureInformation{*sig},
		ActiveSignature: 0,
		ActiveParameter: clampActiveParam(activeParam, len(sig.Parameters)),
	})
}

// findEnclosingCall walks backward through `text` from (line, col),
// returning the identifier name of the enclosing call and the active
// parameter index. (callee="add", activeParam=1) means the cursor is at
// `add(x, |)`. Returns ok=false when the cursor is not inside a call's
// argument list.
//
// Receiver-dot prefixes are returned as part of the callee — e.g.
// "self.foo" or "account.deposit". The caller splits on `.` to resolve
// receiver vs. method.
func findEnclosingCall(text string, line, col int) (callee string, activeParam int, ok bool) {
	lineOffsets := computeLineStarts(text)
	if line-1 >= len(lineOffsets) {
		return "", 0, false
	}
	cursor := lineOffsets[line-1] + (col - 1)
	if cursor > len(text) {
		cursor = len(text)
	}

	depth := 0
	commas := 0
	i := cursor - 1
	for i >= 0 {
		c := text[i]
		switch c {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				// Found the unmatched opening paren.
				return readCalleeBefore(text, i), commas, true
			}
			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}
		i--
	}
	return "", 0, false
}

// readCalleeBefore returns the dotted identifier path immediately before
// position `parenAt`. Skips whitespace; collects identifiers and dots.
// "x.y.z" returns the full chain.
func readCalleeBefore(text string, parenAt int) string {
	i := parenAt - 1
	// Skip whitespace.
	for i >= 0 && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n') {
		i--
	}
	end := i + 1
	for i >= 0 && (isIdentByte(text[i]) || text[i] == '.') {
		i--
	}
	return text[i+1 : end]
}

// resolveSignature looks up the callee — either a bare function name or
// a `receiver.method` chain — and returns LSP SignatureInformation.
// Returns nil when the function/method can't be resolved.
func resolveSignature(prog *ast.Program, scope *scopeResolver, text, ownPath, callee string, line, col int, siblings map[string]*ast.Program) *SignatureInformation {
	if callee == "" {
		return nil
	}

	if dot := strings.LastIndexByte(callee, '.'); dot >= 0 {
		receiver := callee[:dot]
		method := callee[dot+1:]
		// Only single-step receivers in v1 (no chained `a.b.c.foo()`).
		if strings.ContainsRune(receiver, '.') {
			return nil
		}
		res := resolveMemberOnReceiver(prog, scope, line, col, receiver, method)
		if res.local != nil && res.local.Kind == localMethod && res.local.Method != nil {
			return methodSignature(res.local.Method, res.local.Entity)
		}
		return nil
	}

	// Bare identifier — function or extern function in the workspace.
	hit := resolveAcrossWorkspace(prog, ownPath, callee, siblings)
	if hit == nil {
		return nil
	}
	switch hit.Kind {
	case declFunction:
		return functionSignature(hit.Function)
	case declExternFunction:
		return externSignature(hit.ExternFunction)
	}
	return nil
}

func functionSignature(fn *ast.FunctionDecl) *SignatureInformation {
	return &SignatureInformation{
		Label:      formatFunctionLabel(fn.Name, fn.Params, fn.ReturnType),
		Parameters: paramInfos(fn.Params),
	}
}

func externSignature(ext *ast.ExternFunctionDecl) *SignatureInformation {
	return &SignatureInformation{
		Label:      formatFunctionLabel(ext.Name, ext.Params, ext.ReturnType),
		Parameters: paramInfos(ext.Params),
	}
}

func methodSignature(m *ast.MethodDecl, ent *ast.EntityDecl) *SignatureInformation {
	name := m.Name
	if ent != nil {
		name = ent.Name + "." + m.Name
	}
	return &SignatureInformation{
		Label:      formatFunctionLabel(name, m.Params, m.ReturnType),
		Parameters: paramInfos(m.Params),
	}
}

func formatFunctionLabel(name string, params []*ast.Param, retType *ast.TypeRef) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('(')
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		b.WriteString(": ")
		writeTypeRef(&b, p.Type)
	}
	b.WriteString(") returns ")
	writeTypeRef(&b, retType)
	return b.String()
}

func paramInfos(params []*ast.Param) []ParameterInformation {
	out := make([]ParameterInformation, 0, len(params))
	for _, p := range params {
		var b strings.Builder
		b.WriteString(p.Name)
		b.WriteString(": ")
		writeTypeRef(&b, p.Type)
		out = append(out, ParameterInformation{Label: b.String()})
	}
	return out
}

func clampActiveParam(idx, max int) int {
	if max == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= max {
		return max - 1
	}
	return idx
}

// computeLineStarts returns the byte offset of the start of each line
// in text. Mirrors Document.buildLineOffsets but is callable without a
// Document.
func computeLineStarts(text string) []int {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}
