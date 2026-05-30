package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// textDocument/completion. Phase 19 task 19.6.
//
// v1 completion is identifier-only. The response includes:
//   - in-scope locals and params at the cursor (from the scope walker)
//   - top-level declarations in the own document (function, entity,
//     enum, trait, extern, test)
//   - top-level declarations in sibling modules in the same package
//   - Intent keywords
//   - Built-in type names
//
// Member completion (`.field` / `.method` after `.`) is deferred to v1.2.
// In v1 the request still succeeds when the cursor is after a `.`, just
// without filtering — the client surfaces every identifier and the user
// scrolls or types more to narrow.

var intentKeywords = []string{
	"module", "version", "function", "entry", "returns", "requires", "ensures",
	"entity", "field", "invariant", "constructor", "method",
	"intent", "goal", "constraint", "guarantee", "verified_by",
	"let", "mutable", "if", "else", "return",
	"while", "break", "continue", "for", "in", "decreases",
	"forall", "exists", "and", "or", "not", "implies",
	"true", "false", "self", "old", "result",
	"enum", "match", "import", "public", "trait", "impl",
	"async", "await", "spawn", "extern", "from", "test",
}

var intentBuiltinTypes = []string{
	"Int", "Float", "String", "Bool", "Void",
	"Array", "Map", "Result", "Option", "Future", "Fn",
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

func (s *Server) handleCompletion(id json.RawMessage, params json.RawMessage) {
	var p CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode completion params: %v", err))
		return
	}
	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, []CompletionItem{})
		return
	}
	text := doc.snapshotText()
	line, col := doc.positionToLineCol(p.Position)

	pp := parser.New(text)
	prog := pp.Parse()
	ownPath := uriToPath(p.TextDocument.URI)
	ws := s.workspaces.workspaceForURI(p.TextDocument.URI)
	siblings := ws.siblingModules(ownPath)
	scope := newScopeResolver(prog, ownPath)

	items := buildCompletionItems(prog, scope, line, col, siblings)
	s.t.writeResponse(id, items)
}

// buildCompletionItems assembles the candidate list. Order is not
// significant — the client sorts/filters by typed prefix.
func buildCompletionItems(prog *ast.Program, scope *scopeResolver, line, col int, siblings map[string]*ast.Program) []CompletionItem {
	out := make([]CompletionItem, 0)
	seen := map[string]bool{}
	add := func(label string, kind CompletionItemKind, detail string) {
		if label == "" || seen[label] {
			return
		}
		seen[label] = true
		out = append(out, CompletionItem{Label: label, Kind: kind, Detail: detail})
	}

	// Locals + params at the cursor.
	for _, l := range scope.inScopeLocals(line, col) {
		switch l.Kind {
		case localLet:
			detail := typeRefString(l.Let.Type)
			add(l.Name, CompletionVariable, detail)
		case localParam:
			detail := typeRefString(l.Param.Type)
			add(l.Name, CompletionVariable, detail)
		case localSelf:
			detail := ""
			if l.Entity != nil {
				detail = l.Entity.Name
			}
			add("self", CompletionVariable, detail)
		}
	}

	// Top-level decls in the own program.
	if prog != nil {
		for _, fn := range prog.Functions {
			add(fn.Name, CompletionFunction, "function")
		}
		for _, ext := range prog.ExternFunctions {
			add(ext.Name, CompletionFunction, "extern function")
		}
		for _, ent := range prog.Entities {
			add(ent.Name, CompletionClass, "entity")
		}
		for _, en := range prog.Enums {
			add(en.Name, CompletionEnum, "enum")
		}
		for _, tr := range prog.Traits {
			add(tr.Name, CompletionInterface, "trait")
		}
	}

	// Sibling modules' top-level decls. We surface all of them — public
	// or not — since the cross-package visibility rules are enforced by
	// the checker, not the LSP.
	for _, sibProg := range siblings {
		for _, fn := range sibProg.Functions {
			add(fn.Name, CompletionFunction, "function (from import)")
		}
		for _, ent := range sibProg.Entities {
			add(ent.Name, CompletionClass, "entity (from import)")
		}
		for _, en := range sibProg.Enums {
			add(en.Name, CompletionEnum, "enum (from import)")
		}
		for _, tr := range sibProg.Traits {
			add(tr.Name, CompletionInterface, "trait (from import)")
		}
	}

	for _, kw := range intentKeywords {
		add(kw, CompletionKeyword, "keyword")
	}
	for _, t := range intentBuiltinTypes {
		add(t, CompletionStruct, "built-in type")
	}

	return out
}

func typeRefString(t *ast.TypeRef) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	writeTypeRef(&b, t)
	return b.String()
}
