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
