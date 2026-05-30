package lsp

import (
	"github.com/lhaig/intent/internal/ast"
)

// Symbol resolution for hover and go-to-definition.
//
// V1 strategy is deliberately simple: extract the identifier under the
// cursor from the source text, then look it up by name in the program's
// top-level declarations (function, entity, enum, trait, extern function,
// test). Cross-file resolution walks the workspace's sibling modules.
//
// What we deliberately do NOT do in v1:
//   - Method dispatch (would need type information on the receiver expression)
//   - Local variable resolution (let-binding scope walking is non-trivial)
//   - Distinguishing call sites from declarations (hover on a declaration
//     and a call return the same info — acceptable)
//
// These are tracked as v1.1 follow-ups in ops/plans/phase-18-lsp-server.md.

// declKind enumerates the top-level declaration kinds the resolver knows
// how to find and render.
type declKind int

const (
	declUnknown declKind = iota
	declFunction
	declExternFunction
	declEntity
	declEnum
	declTrait
	declTest
)

// declHit is a successful symbol lookup. Path is the absolute file path of
// the file containing the declaration (used for goto-def Location); Line
// and Column are 1-indexed source coordinates of the declaration's name.
type declHit struct {
	Kind   declKind
	Name   string
	Path   string
	Line   int
	Column int

	// Concrete declaration node — handlers introspect this to render
	// hover content or compute the goto-def range. Exactly one is non-nil
	// per hit, matching Kind.
	Function       *ast.FunctionDecl
	ExternFunction *ast.ExternFunctionDecl
	Entity         *ast.EntityDecl
	Enum           *ast.EnumDecl
	Trait          *ast.TraitDecl
	Test           *ast.TestDecl
}

// wordAtPosition extracts the identifier (or identifier-like) substring at
// the given (line, col) — both 1-indexed. Returns empty when the cursor
// is not on an identifier character.
func wordAtPosition(text string, line, col int) string {
	if line < 1 || col < 1 {
		return ""
	}
	// Find the start of the requested line.
	lineStart := 0
	curLine := 1
	for i := 0; i < len(text) && curLine < line; i++ {
		if text[i] == '\n' {
			lineStart = i + 1
			curLine++
		}
	}
	if curLine < line {
		return ""
	}
	// Compute byte offset of (line, col).
	off := lineStart + (col - 1)
	if off < 0 || off >= len(text) {
		return ""
	}
	if !isIdentByte(text[off]) {
		// Allow cursor immediately after the identifier (a common editor
		// convention): step back one byte if the previous char is one.
		if off > 0 && isIdentByte(text[off-1]) {
			off--
		} else {
			return ""
		}
	}
	start := off
	for start > 0 && isIdentByte(text[start-1]) {
		start--
	}
	end := off
	for end < len(text) && isIdentByte(text[end]) {
		end++
	}
	return text[start:end]
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// resolveDecl looks up `name` as a top-level declaration in prog. Returns
// nil when no declaration of that name exists. Path is attached to the
// returned hit so goto-def has a file location.
func resolveDecl(prog *ast.Program, name, path string) *declHit {
	if prog == nil || name == "" {
		return nil
	}
	for _, fn := range prog.Functions {
		if fn.Name == name {
			return &declHit{Kind: declFunction, Name: name, Path: path, Line: fn.Line, Column: fn.Column, Function: fn}
		}
	}
	for _, ext := range prog.ExternFunctions {
		if ext.Name == name {
			return &declHit{Kind: declExternFunction, Name: name, Path: path, Line: ext.Line, Column: ext.Column, ExternFunction: ext}
		}
	}
	for _, ent := range prog.Entities {
		if ent.Name == name {
			return &declHit{Kind: declEntity, Name: name, Path: path, Line: ent.Line, Column: ent.Column, Entity: ent}
		}
	}
	for _, en := range prog.Enums {
		if en.Name == name {
			return &declHit{Kind: declEnum, Name: name, Path: path, Line: en.Line, Column: en.Column, Enum: en}
		}
	}
	for _, tr := range prog.Traits {
		if tr.Name == name {
			return &declHit{Kind: declTrait, Name: name, Path: path, Line: tr.Line, Column: tr.Column, Trait: tr}
		}
	}
	for _, te := range prog.Tests {
		if te.Name == name {
			return &declHit{Kind: declTest, Name: name, Path: path, Line: te.Line, Column: te.Column, Test: te}
		}
	}
	return nil
}

// resolveAcrossWorkspace searches the open document's AST first, then
// sibling modules in the same workspace. Returns the first hit. ADR 0032
// §O4 → B: same-file + same-package only; cross-package deferred.
func resolveAcrossWorkspace(ownProg *ast.Program, ownPath, name string, siblings map[string]*ast.Program) *declHit {
	if hit := resolveDecl(ownProg, name, ownPath); hit != nil {
		return hit
	}
	for siblingPath, prog := range siblings {
		if hit := resolveDecl(prog, name, siblingPath); hit != nil {
			return hit
		}
	}
	return nil
}

// resolution is the unified result type produced by resolveAtPosition.
// At most one of `decl` (top-level declaration) or `local` (scope-resolved
// binding) is non-nil per result.
type resolution struct {
	decl  *declHit
	local *localRef
}

// resolveAtPosition is the primary entry point used by hover and
// goto-definition. It tries, in order:
//
//  1. Member access (`receiver.member`) — if the char immediately before
//     the cursor word is `.`, look up `member` on the receiver's entity
//     type. The receiver must be a simple identifier (local, param, or
//     `self`); chained access (`a.b.c`) is not resolved past the first
//     hop in v1.
//  2. Local binding — let, param, `self`.
//  3. Top-level declaration — function, entity, enum, etc.
//  4. Sibling module top-level declaration.
//
// Returns a zero resolution{} when no match exists.
func resolveAtPosition(prog *ast.Program, scope *scopeResolver, text, ownPath, name string, line, col int, siblings map[string]*ast.Program) resolution {
	// (1) Member access?
	if recv, member, ok := receiverBeforeMember(text, line, col); ok && member == name {
		if hit := resolveMemberOnReceiver(prog, scope, line, col, recv, name); hit.decl != nil || hit.local != nil {
			return hit
		}
		// Fall through — if we can't resolve the member, try other
		// interpretations rather than returning nothing.
	}

	// (2) Local binding.
	if scope != nil {
		if lref := scope.resolveLocal(line, col, name); lref != nil {
			return resolution{local: lref}
		}
	}

	// (3) + (4) Top-level lookup in own program, then siblings.
	if hit := resolveAcrossWorkspace(prog, ownPath, name, siblings); hit != nil {
		return resolution{decl: hit}
	}
	return resolution{}
}

// receiverBeforeMember inspects the text immediately before (line, col)
// and, if the cursor word is preceded by `.` and another identifier,
// returns (receiverIdent, memberIdent, true). Otherwise (.., false).
//
// Whitespace between the receiver, dot, and member is tolerated.
func receiverBeforeMember(text string, line, col int) (receiver, member string, ok bool) {
	if line < 1 || col < 1 {
		return "", "", false
	}
	// Compute the byte offset of (line, col-1) — the char before the cursor.
	lineStart := 0
	cur := 1
	for i := 0; i < len(text) && cur < line; i++ {
		if text[i] == '\n' {
			lineStart = i + 1
			cur++
		}
	}
	if cur < line {
		return "", "", false
	}
	memberStart := lineStart + (col - 1)
	if memberStart < 0 || memberStart >= len(text) || !isIdentByte(text[memberStart]) {
		// Tolerate "cursor immediately after identifier"
		if memberStart > 0 && memberStart <= len(text) && isIdentByte(text[memberStart-1]) {
			memberStart--
		} else {
			return "", "", false
		}
	}
	// Walk back over the member identifier.
	for memberStart > 0 && isIdentByte(text[memberStart-1]) {
		memberStart--
	}
	memberEnd := memberStart
	for memberEnd < len(text) && isIdentByte(text[memberEnd]) {
		memberEnd++
	}
	member = text[memberStart:memberEnd]

	// Skip whitespace backwards looking for a '.'.
	i := memberStart - 1
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i < 0 || text[i] != '.' {
		return "", "", false
	}
	// Skip whitespace before the dot.
	i--
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i < 0 || !isIdentByte(text[i]) {
		return "", "", false
	}
	receiverEnd := i + 1
	receiverStart := receiverEnd - 1
	for receiverStart > 0 && isIdentByte(text[receiverStart-1]) {
		receiverStart--
	}
	receiver = text[receiverStart:receiverEnd]
	return receiver, member, true
}

// resolveMemberOnReceiver resolves a `receiver.member` reference. The
// receiver is matched against locals/params/self at (line, col); its
// type — for typed bindings — is then used to look up `member` as a
// field or method on the receiving entity.
func resolveMemberOnReceiver(prog *ast.Program, scope *scopeResolver, line, col int, receiver, member string) resolution {
	if scope == nil {
		return resolution{}
	}

	var receiverEntity *ast.EntityDecl
	var receiverPath string = scope.path

	if receiver == "self" {
		if ent, _ := scope.enclosingMethod(line, col); ent != nil {
			receiverEntity = ent
		} else if ent, _ := scope.enclosingConstructor(line, col); ent != nil {
			receiverEntity = ent
		}
	} else if lref := scope.resolveLocal(line, col, receiver); lref != nil {
		// Look up the binding's declared type and resolve to an entity.
		typeName := localTypeName(lref)
		if typeName != "" {
			receiverEntity = findEntityByName(prog, typeName)
			receiverPath = scope.path
		}
	}

	if receiverEntity == nil {
		return resolution{}
	}

	if f := findFieldOnEntity(receiverEntity, member); f != nil {
		return resolution{local: &localRef{
			Kind: localField, Name: member, Path: receiverPath,
			Line: f.Line, Column: f.Column,
			Field: f, Entity: receiverEntity,
		}}
	}
	if m := findMethodOnEntity(prog, receiverEntity, member); m != nil {
		return resolution{local: &localRef{
			Kind: localMethod, Name: member, Path: receiverPath,
			Line: m.Line, Column: m.Column,
			Method: m, Entity: receiverEntity,
		}}
	}
	return resolution{}
}

// localTypeName returns the receiver's declared type name (the head of
// the TypeRef) for typed locals/params, or empty when the type is
// inferred or absent.
func localTypeName(lref *localRef) string {
	switch lref.Kind {
	case localLet:
		if lref.Let != nil && lref.Let.Type != nil {
			return lref.Let.Type.Name
		}
	case localParam:
		if lref.Param != nil && lref.Param.Type != nil {
			return lref.Param.Type.Name
		}
	}
	return ""
}

// findEntityByName looks up an entity by name in prog. Used during
// member-access resolution.
func findEntityByName(prog *ast.Program, name string) *ast.EntityDecl {
	if prog == nil {
		return nil
	}
	for _, ent := range prog.Entities {
		if ent.Name == name {
			return ent
		}
	}
	return nil
}
