package lsp

import (
	"encoding/json"
	"fmt"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// textDocument/documentSymbol — outline view. Phase 19 task 19.3.
//
// Returns a tree: top-level functions / entities / enums / traits / tests
// / extern functions. Entity methods, constructors, and fields nest under
// their entity; enum variants nest under their enum; trait method
// signatures nest under their trait.
//
// Ranges are computed best-effort: a symbol's Range starts at its decl
// line/column and ends at the start of the next top-level (or sibling)
// symbol. SelectionRange is the name's start..start+len. This is enough
// for the outline-view click-to-jump behavior; precise span tracking is
// deferred to v1.1 / once the AST carries end positions.

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

func (s *Server) handleDocumentSymbol(id json.RawMessage, params json.RawMessage) {
	var p DocumentSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode documentSymbol params: %v", err))
		return
	}
	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, []DocumentSymbol{})
		return
	}
	text := doc.snapshotText()
	pp := parser.New(text)
	prog := pp.Parse()
	// Even with parse errors, we surface whatever symbols parsed cleanly.
	s.t.writeResponse(id, buildDocumentSymbols(prog))
}

// buildDocumentSymbols walks the program and emits a flat list of
// top-level symbols, each with its children nested.
func buildDocumentSymbols(prog *ast.Program) []DocumentSymbol {
	if prog == nil {
		return []DocumentSymbol{}
	}
	r := newScopeResolver(prog, "")
	out := make([]DocumentSymbol, 0)

	for _, fn := range prog.Functions {
		end := r.nextTopLevelLine(fn.Line)
		out = append(out, DocumentSymbol{
			Name:           fn.Name,
			Detail:         "function",
			Kind:           SymbolFunction,
			Range:          rangeForBlock(fn.Line, fn.Column, end),
			SelectionRange: rangeForName(fn.Line, fn.Column, fn.Name),
		})
	}
	for _, ext := range prog.ExternFunctions {
		end := r.nextTopLevelLine(ext.Line)
		out = append(out, DocumentSymbol{
			Name:           ext.Name,
			Detail:         "extern function",
			Kind:           SymbolFunction,
			Range:          rangeForBlock(ext.Line, ext.Column, end),
			SelectionRange: rangeForName(ext.Line, ext.Column, ext.Name),
		})
	}
	for _, ent := range prog.Entities {
		end := r.nextTopLevelLine(ent.Line)
		out = append(out, DocumentSymbol{
			Name:           ent.Name,
			Detail:         "entity",
			Kind:           SymbolClass,
			Range:          rangeForBlock(ent.Line, ent.Column, end),
			SelectionRange: rangeForName(ent.Line, ent.Column, ent.Name),
			Children:       buildEntityChildren(ent, end, r),
		})
	}
	for _, en := range prog.Enums {
		end := r.nextTopLevelLine(en.Line)
		out = append(out, DocumentSymbol{
			Name:           en.Name,
			Detail:         "enum",
			Kind:           SymbolEnum,
			Range:          rangeForBlock(en.Line, en.Column, end),
			SelectionRange: rangeForName(en.Line, en.Column, en.Name),
			Children:       buildEnumChildren(en),
		})
	}
	for _, tr := range prog.Traits {
		end := r.nextTopLevelLine(tr.Line)
		out = append(out, DocumentSymbol{
			Name:           tr.Name,
			Detail:         "trait",
			Kind:           SymbolInterface,
			Range:          rangeForBlock(tr.Line, tr.Column, end),
			SelectionRange: rangeForName(tr.Line, tr.Column, tr.Name),
			Children:       buildTraitChildren(tr),
		})
	}
	for _, te := range prog.Tests {
		end := r.nextTopLevelLine(te.Line)
		out = append(out, DocumentSymbol{
			Name:           te.Name,
			Detail:         "test",
			Kind:           SymbolFunction,
			Range:          rangeForBlock(te.Line, te.Column, end),
			SelectionRange: rangeForName(te.Line, te.Column, "test"),
		})
	}
	return out
}

func buildEntityChildren(ent *ast.EntityDecl, entEnd int, r *scopeResolver) []DocumentSymbol {
	var children []DocumentSymbol
	for _, f := range ent.Fields {
		children = append(children, DocumentSymbol{
			Name:           f.Name,
			Detail:         "field",
			Kind:           SymbolField,
			Range:          rangeForBlock(f.Line, f.Column, f.Line+1),
			SelectionRange: rangeForName(f.Line, f.Column, f.Name),
		})
	}
	if ent.Constructor != nil {
		end := r.nextMemberLine(ent, ent.Constructor.Line, entEnd)
		children = append(children, DocumentSymbol{
			Name:           "constructor",
			Detail:         "constructor",
			Kind:           SymbolConstructor,
			Range:          rangeForBlock(ent.Constructor.Line, ent.Constructor.Column, end),
			SelectionRange: rangeForName(ent.Constructor.Line, ent.Constructor.Column, "constructor"),
		})
	}
	for _, m := range ent.Methods {
		end := r.nextMemberLine(ent, m.Line, entEnd)
		children = append(children, DocumentSymbol{
			Name:           m.Name,
			Detail:         "method",
			Kind:           SymbolMethod,
			Range:          rangeForBlock(m.Line, m.Column, end),
			SelectionRange: rangeForName(m.Line, m.Column, m.Name),
		})
	}
	return children
}

func buildEnumChildren(en *ast.EnumDecl) []DocumentSymbol {
	var children []DocumentSymbol
	for _, v := range en.Variants {
		children = append(children, DocumentSymbol{
			Name:           v.Name,
			Detail:         "variant",
			Kind:           SymbolEnumMember,
			Range:          rangeForBlock(v.Line, v.Column, v.Line+1),
			SelectionRange: rangeForName(v.Line, v.Column, v.Name),
		})
	}
	return children
}

func buildTraitChildren(tr *ast.TraitDecl) []DocumentSymbol {
	var children []DocumentSymbol
	for _, m := range tr.Methods {
		children = append(children, DocumentSymbol{
			Name:           m.Name,
			Detail:         "trait method",
			Kind:           SymbolMethod,
			Range:          rangeForBlock(m.Line, m.Column, m.Line+1),
			SelectionRange: rangeForName(m.Line, m.Column, m.Name),
		})
	}
	return children
}

// rangeForBlock spans from (startLine, startCol) to the start of the
// next sibling (endLine, 0). Used as the outline-view selection target
// when clicking a symbol.
func rangeForBlock(startLine, startCol, endLine int) Range {
	return Range{
		Start: lineColToPosition(startLine, startCol),
		End:   lineColToPosition(endLine, 1),
	}
}

func rangeForName(line, col int, name string) Range {
	return Range{
		Start: lineColToPosition(line, col),
		End:   lineColToPosition(line, col+len(name)),
	}
}
