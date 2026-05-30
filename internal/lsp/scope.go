package lsp

import (
	"github.com/lhaig/intent/internal/ast"
)

// Scope-aware symbol resolution. Phase 19 task 19.1.
//
// Phase 18's resolver was name-based and top-level-only. Phase 19 expands
// the surface to locals (let-bindings), function parameters, method
// receivers (`self`), entity fields, and entity methods, so hover and
// go-to-definition work on the references users actually write — not
// just top-level decls.
//
// The implementation deliberately avoids a full scope-stack walker. We
// don't have end-position info on AST nodes, and a precise scope walker
// would require either:
//   - Re-lexing to find scope-closing tokens (`}`), or
//   - Computing every block's source range during parsing.
//
// Neither earns its complexity for v1. Instead, we use a simpler rule:
// to resolve a name at (line, col), find the enclosing function/method,
// check its parameters, then check `let` statements in its body that
// appear at or before (line, col) in source order. For methods, also
// search the entity context (`self`, fields, sibling methods). Top-level
// lookup is the final fallback.
//
// Trade-off: a let-binding inside an inner block is visible "everywhere
// after the let appears" inside the function, even outside the block.
// Real lexical scope would mask it once the block closes. This is wrong
// in edge cases but covers the common case (locals declared at function
// top, used throughout) without needing block-end tracking. v1.1 can add
// block-aware scoping when a real example exposes the limitation.

// localKind identifies the source of a resolved local binding.
type localKind int

const (
	localLet localKind = iota
	localParam
	localSelf
	localField
	localMethod
)

// localRef is a richer resolution result than declHit. Locals don't have
// a "qualified declaration" file/line in the way functions do — they
// belong to an enclosing function or method. Fields and methods belong
// to an entity. Hover and goto-def consume this to render content and
// produce Locations.
type localRef struct {
	Kind   localKind
	Name   string
	Path   string // file path of the enclosing function/method/entity (for goto-def Location)
	Line   int    // 1-indexed declaration line
	Column int    // 1-indexed declaration column

	// Exactly one of these is non-nil, matching Kind.
	Let    *ast.LetStmt
	Param  *ast.Param
	Method *ast.MethodDecl // for localMethod (method dispatch result) and the method body owning the cursor (for localSelf)
	Field  *ast.FieldDecl
	Entity *ast.EntityDecl // set for localSelf, localField, localMethod
}

// scopeResolver answers position-based queries against a parsed program.
// It's cheap to construct (no eager walking); resolution work happens on
// demand.
type scopeResolver struct {
	prog *ast.Program
	path string // file path used for goto-def Locations
}

func newScopeResolver(prog *ast.Program, path string) *scopeResolver {
	return &scopeResolver{prog: prog, path: path}
}

// resolveLocal looks up name as a local binding visible at (line, col).
// Returns nil when name is not a local — the caller falls back to
// top-level resolution.
func (r *scopeResolver) resolveLocal(line, col int, name string) *localRef {
	if r.prog == nil || name == "" {
		return nil
	}

	if fn := r.enclosingFunction(line, col); fn != nil {
		if hit := r.searchFunctionScope(fn, line, col, name); hit != nil {
			return hit
		}
	}

	if ent, m := r.enclosingMethod(line, col); m != nil {
		if hit := r.searchMethodScope(ent, m, line, col, name); hit != nil {
			return hit
		}
	}

	if ent, c := r.enclosingConstructor(line, col); c != nil {
		if hit := r.searchConstructorScope(ent, c, line, col, name); hit != nil {
			return hit
		}
	}

	return nil
}

// inScopeLocals returns every locally-visible binding at (line, col).
// Used by completion (19.6) to seed the suggestion list. Locals are
// dedup'd by name — an inner shadow doesn't get emitted twice.
func (r *scopeResolver) inScopeLocals(line, col int) []*localRef {
	if r.prog == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []*localRef
	add := func(ref *localRef) {
		if ref == nil || seen[ref.Name] {
			return
		}
		seen[ref.Name] = true
		out = append(out, ref)
	}

	if fn := r.enclosingFunction(line, col); fn != nil {
		for _, p := range fn.Params {
			add(&localRef{Kind: localParam, Name: p.Name, Path: r.path, Line: p.Line, Column: p.Column, Param: p})
		}
		for _, let := range r.collectLetsBefore(fn.Body, line, col) {
			add(&localRef{Kind: localLet, Name: let.Name, Path: r.path, Line: let.Line, Column: let.Column, Let: let})
		}
	}

	if ent, m := r.enclosingMethod(line, col); m != nil {
		for _, p := range m.Params {
			add(&localRef{Kind: localParam, Name: p.Name, Path: r.path, Line: p.Line, Column: p.Column, Param: p})
		}
		for _, let := range r.collectLetsBefore(m.Body, line, col) {
			add(&localRef{Kind: localLet, Name: let.Name, Path: r.path, Line: let.Line, Column: let.Column, Let: let})
		}
		add(&localRef{Kind: localSelf, Name: "self", Path: r.path, Line: m.Line, Column: m.Column, Entity: ent, Method: m})
	}

	return out
}

// enclosingFunction returns the FunctionDecl whose body contains
// (line, col). We approximate the body's range by treating it as "from
// fn.Line to just before the next top-level declaration's Line" — sound
// because top-level decls don't nest.
func (r *scopeResolver) enclosingFunction(line, col int) *ast.FunctionDecl {
	for _, fn := range r.prog.Functions {
		if r.positionInTopLevel(line, fn.Line, r.nextTopLevelLine(fn.Line)) {
			return fn
		}
	}
	return nil
}

// enclosingMethod returns the EntityDecl + MethodDecl whose body
// contains (line, col).
func (r *scopeResolver) enclosingMethod(line, col int) (*ast.EntityDecl, *ast.MethodDecl) {
	for _, ent := range r.prog.Entities {
		entEnd := r.nextTopLevelLine(ent.Line)
		if line < ent.Line || line >= entEnd {
			continue
		}
		for _, m := range ent.Methods {
			methodEnd := r.nextMemberLine(ent, m.Line, entEnd)
			if line >= m.Line && line < methodEnd {
				return ent, m
			}
		}
	}
	return nil, nil
}

// enclosingConstructor returns the EntityDecl + ConstructorDecl whose
// body contains (line, col).
func (r *scopeResolver) enclosingConstructor(line, col int) (*ast.EntityDecl, *ast.ConstructorDecl) {
	for _, ent := range r.prog.Entities {
		entEnd := r.nextTopLevelLine(ent.Line)
		if line < ent.Line || line >= entEnd {
			continue
		}
		if ent.Constructor != nil {
			ctorEnd := r.nextMemberLine(ent, ent.Constructor.Line, entEnd)
			if line >= ent.Constructor.Line && line < ctorEnd {
				return ent, ent.Constructor
			}
		}
	}
	return nil, nil
}

// positionInTopLevel returns true if `line` falls inside the half-open
// interval [start, end) where end is the line of the next sibling at
// the same level (or a large sentinel if this was the last sibling).
func (r *scopeResolver) positionInTopLevel(line, start, end int) bool {
	return line >= start && line < end
}

// nextTopLevelLine returns the Line of the next top-level declaration
// after one at `afterLine`, or a large sentinel for the last decl.
func (r *scopeResolver) nextTopLevelLine(afterLine int) int {
	const sentinel = 1 << 30
	best := sentinel
	consider := func(ln int) {
		if ln > afterLine && ln < best {
			best = ln
		}
	}
	for _, fn := range r.prog.Functions {
		consider(fn.Line)
	}
	for _, ent := range r.prog.Entities {
		consider(ent.Line)
	}
	for _, en := range r.prog.Enums {
		consider(en.Line)
	}
	for _, tr := range r.prog.Traits {
		consider(tr.Line)
	}
	for _, ext := range r.prog.ExternFunctions {
		consider(ext.Line)
	}
	for _, t := range r.prog.Tests {
		consider(t.Line)
	}
	for _, i := range r.prog.Intents {
		consider(i.Line)
	}
	for _, i := range r.prog.ImplBlocks {
		consider(i.Line)
	}
	return best
}

// nextMemberLine finds the next entity member after `afterLine`, capped
// at the entity's own end. Used to delimit method/constructor bodies.
func (r *scopeResolver) nextMemberLine(ent *ast.EntityDecl, afterLine, entityEnd int) int {
	best := entityEnd
	consider := func(ln int) {
		if ln > afterLine && ln < best {
			best = ln
		}
	}
	for _, f := range ent.Fields {
		consider(f.Line)
	}
	for _, inv := range ent.Invariants {
		consider(inv.Line)
	}
	if ent.Constructor != nil {
		consider(ent.Constructor.Line)
	}
	for _, m := range ent.Methods {
		consider(m.Line)
	}
	return best
}

func (r *scopeResolver) searchFunctionScope(fn *ast.FunctionDecl, line, col int, name string) *localRef {
	for _, p := range fn.Params {
		if p.Name == name {
			return &localRef{Kind: localParam, Name: name, Path: r.path, Line: p.Line, Column: p.Column, Param: p}
		}
	}
	for _, let := range r.collectLetsBefore(fn.Body, line, col) {
		if let.Name == name {
			return &localRef{Kind: localLet, Name: name, Path: r.path, Line: let.Line, Column: let.Column, Let: let}
		}
	}
	return nil
}

func (r *scopeResolver) searchMethodScope(ent *ast.EntityDecl, m *ast.MethodDecl, line, col int, name string) *localRef {
	if name == "self" {
		return &localRef{Kind: localSelf, Name: "self", Path: r.path, Line: m.Line, Column: m.Column, Entity: ent, Method: m}
	}
	for _, p := range m.Params {
		if p.Name == name {
			return &localRef{Kind: localParam, Name: name, Path: r.path, Line: p.Line, Column: p.Column, Param: p}
		}
	}
	for _, let := range r.collectLetsBefore(m.Body, line, col) {
		if let.Name == name {
			return &localRef{Kind: localLet, Name: name, Path: r.path, Line: let.Line, Column: let.Column, Let: let}
		}
	}
	return nil
}

func (r *scopeResolver) searchConstructorScope(ent *ast.EntityDecl, c *ast.ConstructorDecl, line, col int, name string) *localRef {
	if name == "self" {
		return &localRef{Kind: localSelf, Name: "self", Path: r.path, Line: c.Line, Column: c.Column, Entity: ent}
	}
	for _, p := range c.Params {
		if p.Name == name {
			return &localRef{Kind: localParam, Name: name, Path: r.path, Line: p.Line, Column: p.Column, Param: p}
		}
	}
	for _, let := range r.collectLetsBefore(c.Body, line, col) {
		if let.Name == name {
			return &localRef{Kind: localLet, Name: name, Path: r.path, Line: let.Line, Column: let.Column, Let: let}
		}
	}
	return nil
}

// collectLetsBefore returns every LetStmt in `block` (and its nested
// blocks) whose Line is strictly less than `line`, or whose Line equals
// `line` and Column is less than `col`. The order matches source order;
// callers iterate to find a name match.
//
// This treats lets inside inner blocks (if/while/for bodies) as visible
// outside their block, which is the simplification documented at the
// top of the file.
func (r *scopeResolver) collectLetsBefore(block *ast.Block, line, col int) []*ast.LetStmt {
	if block == nil {
		return nil
	}
	var out []*ast.LetStmt
	r.walkBlock(block, func(stmt ast.Statement) {
		if let, ok := stmt.(*ast.LetStmt); ok {
			if let.Line < line || (let.Line == line && let.Column < col) {
				out = append(out, let)
			}
		}
	})
	return out
}

func (r *scopeResolver) walkBlock(block *ast.Block, fn func(ast.Statement)) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		fn(stmt)
		switch s := stmt.(type) {
		case *ast.IfStmt:
			r.walkBlock(s.Then, fn)
			if elseBlock, ok := s.Else.(*ast.Block); ok {
				r.walkBlock(elseBlock, fn)
			} else if elseIf, ok := s.Else.(*ast.IfStmt); ok {
				r.walkBlock(elseIf.Then, fn)
			}
		case *ast.WhileStmt:
			r.walkBlock(s.Body, fn)
		case *ast.ForInStmt:
			r.walkBlock(s.Body, fn)
		}
	}
}

// findFieldOnEntity returns the field declaration with the given name
// on the entity, or nil if absent. Used by hover/goto-def for field
// access on a receiver whose type the checker already resolved.
func findFieldOnEntity(ent *ast.EntityDecl, name string) *ast.FieldDecl {
	if ent == nil {
		return nil
	}
	for _, f := range ent.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// findMethodOnEntity returns the method declaration with the given
// name. Looks at the entity's own methods plus any methods added via
// impl blocks in the same program.
func findMethodOnEntity(prog *ast.Program, ent *ast.EntityDecl, name string) *ast.MethodDecl {
	if ent == nil {
		return nil
	}
	for _, m := range ent.Methods {
		if m.Name == name {
			return m
		}
	}
	if prog != nil {
		for _, impl := range prog.ImplBlocks {
			if impl.EntityName != ent.Name {
				continue
			}
			for _, m := range impl.Methods {
				if m.Name == name {
					return m
				}
			}
		}
	}
	return nil
}
