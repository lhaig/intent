package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// textDocument/completion. Phase 19 task 19.6; member completion added
// in Phase 21.
//
// Two modes:
//
//  1. Member-completion position — cursor follows `<receiver>.` (possibly
//     with a partial member identifier typed). The response is the
//     receiver entity's fields + methods only; keywords and top-level
//     decls are intentionally omitted so the list isn't noise.
//
//  2. Otherwise (identifier-completion position), the response includes:
//     - in-scope locals and params at the cursor (from the scope walker)
//     - top-level declarations in the own document (function, entity,
//       enum, trait, extern, test)
//     - top-level declarations in sibling modules in the same package
//     - Intent keywords
//     - Built-in type names
//
// Chained access (`a.b.c`), function-result chains (`foo().bar`), trait
// methods on `dyn`/generic receivers, and `EntityName.` (static) all
// fall through to identifier completion — receiver typing in the LSP is
// still single-step and entity-local.

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

	items := buildCompletionItems(prog, scope, text, line, col, siblings)
	s.t.writeResponse(id, items)
}

// memberCompletionContext returns the receiver identifier when the
// cursor is immediately after `<ident>.` (with an optional partial
// member identifier already typed). Returns ("", false) otherwise.
//
// The receiver must be a single identifier — chained access (`a.b.c`)
// and function-call chains (`foo().bar`) both return ("", false). The
// LSP's receiver-type resolver is single-step and entity-local, so
// reporting `b` as the receiver in `a.b.c<cursor>` would mis-resolve.
func memberCompletionContext(text string, line, col int) (string, bool) {
	if text == "" || line < 1 || col < 1 {
		return "", false
	}
	lineStart := 0
	cur := 1
	for i := 0; i < len(text) && cur < line; i++ {
		if text[i] == '\n' {
			lineStart = i + 1
			cur++
		}
	}
	if cur < line {
		return "", false
	}
	off := lineStart + (col - 1)
	if off < 0 || off > len(text) {
		return "", false
	}

	// Walk back over the partial member identifier (possibly empty).
	i := off
	for i > 0 && isIdentByte(text[i-1]) {
		i--
	}
	// Skip whitespace between the partial member and the dot.
	for i > 0 && (text[i-1] == ' ' || text[i-1] == '\t') {
		i--
	}
	if i == 0 || text[i-1] != '.' {
		return "", false
	}
	i--
	// Skip whitespace between the dot and the receiver.
	for i > 0 && (text[i-1] == ' ' || text[i-1] == '\t') {
		i--
	}
	if i == 0 || !isIdentByte(text[i-1]) {
		// Could be `).foo` (call chain) or `].foo` (index chain) —
		// receiver-typing for those isn't modelled. Fall through to
		// identifier completion.
		return "", false
	}
	end := i
	for i > 0 && isIdentByte(text[i-1]) {
		i--
	}
	receiver := text[i:end]
	// Reject chained access: if a `.` precedes the receiver (modulo
	// whitespace), the *real* receiver expression is more complex than
	// a single identifier — bail.
	j := i
	for j > 0 && (text[j-1] == ' ' || text[j-1] == '\t') {
		j--
	}
	if j > 0 && text[j-1] == '.' {
		return "", false
	}
	return receiver, true
}

// buildMemberCompletionItems returns completion items for fields and
// methods of the receiver's entity. Returns (items, true) on a
// successful resolve and (nil, false) when the receiver doesn't
// correspond to a known entity (so the caller can surface an empty
// list rather than fall back to identifier completion).
func buildMemberCompletionItems(prog *ast.Program, scope *scopeResolver, line, col int, receiver string) ([]CompletionItem, bool) {
	ent := resolveReceiverEntity(prog, scope, line, col, receiver)
	if ent == nil {
		return nil, false
	}

	items := make([]CompletionItem, 0, len(ent.Fields)+len(ent.Methods))
	for _, f := range ent.Fields {
		items = append(items, CompletionItem{
			Label:  f.Name,
			Kind:   CompletionField,
			Detail: typeRefString(f.Type),
		})
	}
	addMethod := func(m *ast.MethodDecl) {
		items = append(items, CompletionItem{
			Label:  m.Name,
			Kind:   CompletionMethod,
			Detail: methodSignatureDetail(m),
		})
	}
	for _, m := range ent.Methods {
		addMethod(m)
	}
	if prog != nil {
		for _, impl := range prog.ImplBlocks {
			if impl.EntityName != ent.Name {
				continue
			}
			for _, m := range impl.Methods {
				addMethod(m)
			}
		}
	}
	return items, true
}

// resolveReceiverEntity maps a receiver identifier at (line, col) to
// the entity it carries: `self` via the enclosing method or
// constructor, any other identifier via its typed binding (let or
// param) whose declared type names an entity in `prog`.
func resolveReceiverEntity(prog *ast.Program, scope *scopeResolver, line, col int, receiver string) *ast.EntityDecl {
	if scope == nil || receiver == "" {
		return nil
	}
	if receiver == "self" {
		if ent, _ := scope.enclosingMethod(line, col); ent != nil {
			return ent
		}
		if ent, _ := scope.enclosingConstructor(line, col); ent != nil {
			return ent
		}
		return nil
	}
	lref := scope.resolveLocal(line, col, receiver)
	if lref == nil {
		return nil
	}
	typeName := localTypeName(lref)
	if typeName == "" {
		return nil
	}
	return findEntityByName(prog, typeName)
}

// methodSignatureDetail renders a method's parameter list and return
// type for completion-detail strings: `(amount: Int) -> Void`. (The
// richer `methodSignature` in signature.go produces full
// SignatureInformation for signatureHelp; here we only need a one-line
// detail string for the editor's completion popover.)
func methodSignatureDetail(m *ast.MethodDecl) string {
	var b strings.Builder
	b.WriteByte('(')
	writeParams(&b, m.Params)
	b.WriteString(") -> ")
	writeTypeRef(&b, m.ReturnType)
	return b.String()
}

// buildCompletionItems assembles the candidate list. Order is not
// significant — the client sorts/filters by typed prefix.
//
// Caller may pass `text` empty when the test exercises identifier
// completion directly; in that case member-position detection is
// skipped.
func buildCompletionItems(prog *ast.Program, scope *scopeResolver, text string, line, col int, siblings map[string]*ast.Program) []CompletionItem {
	if recv, ok := memberCompletionContext(text, line, col); ok {
		if items, hit := buildMemberCompletionItems(prog, scope, line, col, recv); hit {
			return items
		}
		// Receiver doesn't resolve to a known entity. Return an empty
		// list rather than the full identifier set — surfacing
		// `account.if` etc. after a dot is more confusing than nothing.
		return []CompletionItem{}
	}

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
