package lsp

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// Semantic tokens (Phase 20 task 20.2).
//
// LSP's textDocument/semanticTokens/full lets the server tell the editor
// what kind of thing each identifier is — function vs variable vs
// method, etc. — using the actual AST instead of TextMate regex
// heuristics. The client highlights tokens according to the user's
// theme.
//
// The wire format is delta-encoded: each token is a 5-tuple
// [deltaLine, deltaStartChar, length, tokenType, tokenModifiers].
// deltaLine is relative to the previous token's line; deltaStartChar is
// relative to the previous token's start when on the same line, or
// absolute when on a new line. tokenType is an index into the legend's
// TokenTypes; tokenModifiers is a bitmask of legend's TokenModifiers.

// Legend — the wire contract. Order is part of the protocol; only
// append, never reorder.
var semanticTokenTypes = []string{
	"function",   // 0
	"method",     // 1
	"parameter",  // 2
	"variable",   // 3
	"property",   // 4
	"class",      // 5
	"enum",       // 6
	"enumMember", // 7
	"interface",  // 8
	"decorator",  // 9
}

const (
	tokFunction uint32 = iota
	tokMethod
	tokParameter
	tokVariable
	tokProperty
	tokClass
	tokEnum
	tokEnumMember
	tokInterface
	tokDecorator
)

var semanticTokenModifiers = []string{
	"declaration",    // bit 0
	"async",          // bit 1
	"defaultLibrary", // bit 2
}

const (
	modDeclaration    uint32 = 1 << 0
	modAsync          uint32 = 1 << 1
	modDefaultLibrary uint32 = 1 << 2
)

// Built-in function names get the defaultLibrary modifier so themes
// render them distinctly from user-defined functions. Matches the
// builtin-calls list in syntaxes/intent.tmLanguage.json.
var builtinFunctions = map[string]bool{
	"print":         true,
	"len":           true,
	"assert":        true,
	"assert_eq":     true,
	"assert_close":  true,
	"assert_panics": true,
	"await_all":     true,
	"await_any":     true,
	"timeout":       true,
	"sleep":         true,
	"Ok":            true,
	"Err":           true,
	"Some":          true,
	"None":          true,
}

func semanticTokensLegend() SemanticTokensLegend {
	return SemanticTokensLegend{
		TokenTypes:     append([]string(nil), semanticTokenTypes...),
		TokenModifiers: append([]string(nil), semanticTokenModifiers...),
	}
}

// semanticToken is the unencoded form. We collect all tokens, sort by
// position, then delta-encode for the wire.
type semanticToken struct {
	Line      int // 0-indexed
	StartChar int // 0-indexed
	Length    int
	Type      uint32
	Modifiers uint32
}

func (s *Server) handleSemanticTokensFull(id json.RawMessage, params json.RawMessage) {
	var p SemanticTokensParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode semanticTokens params: %v", err))
		return
	}
	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, SemanticTokens{Data: []uint32{}})
		return
	}
	text := doc.snapshotText()
	pp := parser.New(text)
	prog := pp.Parse()
	// We still emit tokens even when there are parse errors — partial
	// highlighting is better than no highlighting on a half-broken file.
	tokens := collectSemanticTokens(prog)
	s.t.writeResponse(id, SemanticTokens{Data: encodeSemanticTokens(tokens)})
}

// collectSemanticTokens walks the program and returns tokens in source
// order. Sort happens explicitly at the end since AST traversal order
// doesn't always match source order (e.g. contract clauses appear in
// the AST before the body but in source after the signature line).
func collectSemanticTokens(prog *ast.Program) []semanticToken {
	if prog == nil {
		return nil
	}
	w := &tokenWalker{}

	for _, fn := range prog.Functions {
		w.emitFunctionDecl(fn)
	}
	for _, ext := range prog.ExternFunctions {
		w.emitExternFunctionDecl(ext)
	}
	for _, ent := range prog.Entities {
		w.emitEntityDecl(ent)
	}
	for _, en := range prog.Enums {
		w.emitEnumDecl(en)
	}
	for _, tr := range prog.Traits {
		w.emitTraitDecl(tr)
	}
	for _, impl := range prog.ImplBlocks {
		w.emitImplBlock(impl)
	}
	for _, te := range prog.Tests {
		w.emitTestDecl(te)
	}

	// Source-order sort. Tokens beginning on the same line sort by column.
	sort.SliceStable(w.tokens, func(i, j int) bool {
		if w.tokens[i].Line != w.tokens[j].Line {
			return w.tokens[i].Line < w.tokens[j].Line
		}
		return w.tokens[i].StartChar < w.tokens[j].StartChar
	})

	// Drop duplicate or overlapping tokens (defensive — the walker
	// occasionally emits the same identifier twice via different code
	// paths). Same line+start means same token.
	deduped := w.tokens[:0]
	for i, t := range w.tokens {
		if i > 0 && w.tokens[i-1].Line == t.Line && w.tokens[i-1].StartChar == t.StartChar {
			continue
		}
		deduped = append(deduped, t)
	}
	return deduped
}

// encodeSemanticTokens delta-encodes the source-ordered tokens per the
// LSP wire format.
func encodeSemanticTokens(tokens []semanticToken) []uint32 {
	out := make([]uint32, 0, len(tokens)*5)
	prevLine, prevStart := 0, 0
	for _, t := range tokens {
		deltaLine := t.Line - prevLine
		deltaStart := t.StartChar
		if deltaLine == 0 {
			deltaStart = t.StartChar - prevStart
		}
		out = append(out, uint32(deltaLine), uint32(deltaStart), uint32(t.Length), t.Type, t.Modifiers)
		prevLine, prevStart = t.Line, t.StartChar
	}
	return out
}

// tokenWalker collects tokens as it walks the AST.
type tokenWalker struct {
	tokens []semanticToken
}

func (w *tokenWalker) push(line, col, length int, ty, mods uint32) {
	if line < 1 || col < 1 || length <= 0 {
		return
	}
	w.tokens = append(w.tokens, semanticToken{
		Line:      line - 1,
		StartChar: col - 1,
		Length:    length,
		Type:      ty,
		Modifiers: mods,
	})
}

func (w *tokenWalker) emitFunctionDecl(fn *ast.FunctionDecl) {
	mods := modDeclaration
	if fn.IsAsync {
		mods |= modAsync
	}
	w.push(fn.Line, fnNameColumn(fn), len(fn.Name), tokFunction, mods)
	for _, p := range fn.Params {
		w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
	}
	for _, req := range fn.Requires {
		w.emitExpr(req.Expr)
	}
	for _, ens := range fn.Ensures {
		w.emitExpr(ens.Expr)
	}
	w.emitBlock(fn.Body)
}

func (w *tokenWalker) emitExternFunctionDecl(ext *ast.ExternFunctionDecl) {
	w.push(ext.Line, externFnNameColumn(ext), len(ext.Name), tokFunction, modDeclaration|modDefaultLibrary)
	for _, p := range ext.Params {
		w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
	}
	for _, req := range ext.Requires {
		w.emitExpr(req.Expr)
	}
	for _, ens := range ext.Ensures {
		w.emitExpr(ens.Expr)
	}
}

func (w *tokenWalker) emitEntityDecl(ent *ast.EntityDecl) {
	w.push(ent.Line, entityNameColumn(ent), len(ent.Name), tokClass, modDeclaration)
	for _, f := range ent.Fields {
		w.push(f.Line, f.Column, len(f.Name), tokProperty, modDeclaration)
	}
	for _, inv := range ent.Invariants {
		w.emitExpr(inv.Expr)
	}
	if ent.Constructor != nil {
		for _, p := range ent.Constructor.Params {
			w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
		}
		for _, req := range ent.Constructor.Requires {
			w.emitExpr(req.Expr)
		}
		for _, ens := range ent.Constructor.Ensures {
			w.emitExpr(ens.Expr)
		}
		w.emitBlock(ent.Constructor.Body)
	}
	for _, m := range ent.Methods {
		w.emitMethodDecl(m)
	}
}

func (w *tokenWalker) emitEnumDecl(en *ast.EnumDecl) {
	w.push(en.Line, enumNameColumn(en), len(en.Name), tokEnum, modDeclaration)
	for _, v := range en.Variants {
		w.push(v.Line, v.Column, len(v.Name), tokEnumMember, modDeclaration)
	}
}

func (w *tokenWalker) emitTraitDecl(tr *ast.TraitDecl) {
	w.push(tr.Line, traitNameColumn(tr), len(tr.Name), tokInterface, modDeclaration)
	for _, m := range tr.Methods {
		w.push(m.Line, methodSigNameColumn(m), len(m.Name), tokMethod, modDeclaration)
		for _, p := range m.Params {
			w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
		}
	}
}

func (w *tokenWalker) emitImplBlock(impl *ast.ImplBlock) {
	for _, m := range impl.Methods {
		w.emitMethodDecl(m)
	}
}

func (w *tokenWalker) emitTestDecl(te *ast.TestDecl) {
	for _, ann := range te.Annotations {
		w.push(ann.Line, ann.Column+1, len(ann.Name), tokDecorator, 0) // +1 to skip the '@'
	}
	w.emitBlock(te.Body)
}

func (w *tokenWalker) emitMethodDecl(m *ast.MethodDecl) {
	w.push(m.Line, methodNameColumn(m), len(m.Name), tokMethod, modDeclaration)
	for _, p := range m.Params {
		w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
	}
	for _, req := range m.Requires {
		w.emitExpr(req.Expr)
	}
	for _, ens := range m.Ensures {
		w.emitExpr(ens.Expr)
	}
	w.emitBlock(m.Body)
}

func (w *tokenWalker) emitBlock(block *ast.Block) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		w.emitStmt(stmt)
	}
}

func (w *tokenWalker) emitStmt(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		w.push(s.Line, letNameColumn(s), len(s.Name), tokVariable, modDeclaration)
		w.emitExpr(s.Value)
	case *ast.AssignStmt:
		w.emitExpr(s.Target)
		w.emitExpr(s.Value)
	case *ast.ReturnStmt:
		if s.Value != nil {
			w.emitExpr(s.Value)
		}
	case *ast.IfStmt:
		w.emitExpr(s.Condition)
		w.emitBlock(s.Then)
		if s.Else != nil {
			w.emitStmt(s.Else)
		}
	case *ast.Block:
		w.emitBlock(s)
	case *ast.WhileStmt:
		w.emitExpr(s.Condition)
		for _, inv := range s.Invariants {
			w.emitExpr(inv.Expr)
		}
		if s.Decreases != nil {
			w.emitExpr(s.Decreases.Expr)
		}
		w.emitBlock(s.Body)
	case *ast.ForInStmt:
		w.emitExpr(s.Iterable)
		w.emitBlock(s.Body)
	case *ast.ExprStmt:
		w.emitExpr(s.Expr)
	}
}

func (w *tokenWalker) emitExpr(expr ast.Expression) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		// Bare identifier references — most are variables/params from
		// the lexical perspective. We don't have the type context here
		// to distinguish param vs let vs constant, so emit `variable`
		// uniformly. TextMate handles keyword-shaped ones; this catches
		// the rest.
		w.push(e.Line, e.Column, len(e.Name), tokVariable, 0)
	case *ast.CallExpr:
		mods := uint32(0)
		if builtinFunctions[e.Function] {
			mods |= modDefaultLibrary
		}
		w.push(e.Line, e.Column, len(e.Function), tokFunction, mods)
		for _, a := range e.Args {
			w.emitExpr(a)
		}
	case *ast.MethodCallExpr:
		w.emitExpr(e.Object)
		// e.Line/Column point at the method-name token in the parser.
		w.push(e.Line, e.Column, len(e.Method), tokMethod, 0)
		for _, a := range e.Args {
			w.emitExpr(a)
		}
	case *ast.FieldAccessExpr:
		w.emitExpr(e.Object)
		w.push(e.Line, e.Column, len(e.Field), tokProperty, 0)
	case *ast.IndexExpr:
		w.emitExpr(e.Object)
		w.emitExpr(e.Index)
	case *ast.BinaryExpr:
		w.emitExpr(e.Left)
		w.emitExpr(e.Right)
	case *ast.UnaryExpr:
		w.emitExpr(e.Operand)
	case *ast.OldExpr:
		w.emitExpr(e.Expr)
	case *ast.TryExpr:
		w.emitExpr(e.Expr)
	case *ast.AwaitExpr:
		w.emitExpr(e.Expr)
	case *ast.SpawnExpr:
		w.emitExpr(e.Expr)
	case *ast.RangeExpr:
		w.emitExpr(e.Start)
		w.emitExpr(e.End)
	case *ast.ForallExpr:
		w.emitExpr(e.Domain)
		w.emitExpr(e.Body)
	case *ast.ExistsExpr:
		w.emitExpr(e.Domain)
		w.emitExpr(e.Body)
	case *ast.MatchExpr:
		w.emitExpr(e.Scrutinee)
		for _, arm := range e.Arms {
			w.emitExpr(arm.Body)
		}
	case *ast.LambdaExpr:
		for _, p := range e.Params {
			w.push(p.Line, p.Column, len(p.Name), tokParameter, modDeclaration)
		}
		w.emitExpr(e.Body)
	case *ast.ArrayLit:
		for _, el := range e.Elements {
			w.emitExpr(el)
		}
	case *ast.StringInterp:
		for _, part := range e.Parts {
			if part.IsExpr {
				w.emitExpr(part.Expr)
			}
		}
	}
}

// The AST stores the declaration's Line/Column at the start of the
// declaration keyword (e.g. "function"), not the name. The helpers below
// compute the column of the name itself by adding the keyword length and
// the whitespace between. They're approximations — the AST doesn't carry
// the name's exact column — but match the canonical formatter's output.
//
// For non-canonical source the highlighting may be slightly off; semantic
// tokens degrade gracefully (worst case: a token misaligns by a few
// columns and the regex grammar fills in).

func fnNameColumn(fn *ast.FunctionDecl) int {
	prefix := keywordLen("function")
	if fn.IsAsync {
		prefix = keywordLen("async function")
	}
	if fn.IsEntry {
		if fn.IsAsync {
			prefix = keywordLen("async entry function")
		} else {
			prefix = keywordLen("entry function")
		}
	}
	return fn.Column + prefix
}

func externFnNameColumn(ext *ast.ExternFunctionDecl) int {
	return ext.Column + keywordLen("extern function")
}

func entityNameColumn(ent *ast.EntityDecl) int {
	if ent.IsPublic {
		return ent.Column + keywordLen("public entity")
	}
	return ent.Column + keywordLen("entity")
}

func enumNameColumn(en *ast.EnumDecl) int {
	if en.IsPublic {
		return en.Column + keywordLen("public enum")
	}
	return en.Column + keywordLen("enum")
}

func traitNameColumn(tr *ast.TraitDecl) int {
	if tr.IsPublic {
		return tr.Column + keywordLen("public trait")
	}
	return tr.Column + keywordLen("trait")
}

func methodNameColumn(m *ast.MethodDecl) int         { return m.Column + keywordLen("method") }
func methodSigNameColumn(m *ast.TraitMethodDecl) int { return m.Column + keywordLen("method") }
func letNameColumn(l *ast.LetStmt) int {
	if l.Mutable {
		return l.Column + keywordLen("let mutable")
	}
	return l.Column + keywordLen("let")
}

func keywordLen(s string) int { return len(s) + 1 } // +1 for the trailing space
