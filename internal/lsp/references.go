package lsp

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

// textDocument/references handler. Phase 26 / ADR 0035.
//
// Scope (v1):
//   - Top-level decls: function, entity, enum, trait, test, extern function
//   - Locals: let, param, self (scope-bound to the enclosing function body)
//   - Same-name disambiguation: name-match only across the workspace.
//     If two modules each export `add`, references include both — a known
//     v1 limitation; ADR 0035 documents the trade-off.
//
// Method and field references are deferred to a later phase (they need
// receiver-type disambiguation per use-site).

// referenceKind classifies how a name should be matched in the AST walk.
type referenceKind int

const (
	refFunction    referenceKind = iota // CallExpr.Function
	refType                             // TypeRef.Name (+ entity-constructor CallExpr.Function)
	refEnumVariant                      // EnumName.Variant patterns + Variant constructions
	refLocal                            // Identifier.Name within an enclosing function frame
	refDeclOnly                         // declaration-only kinds (tests)
)

// referenceTarget describes what the walker is looking for.
type referenceTarget struct {
	name string
	kind referenceKind
	// For refLocal: the function-body frame to which the scope is
	// bounded. Set by the handler before invoking the walker.
	frameFn           *ast.FunctionDecl
	frameCtor         *ast.ConstructorDecl
	frameMethod       *ast.MethodDecl
	frameMethodEntity string // for method scope
}

func (s *Server) handleReferences(id json.RawMessage, params json.RawMessage) {
	var p ReferenceParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode references params: %v", err))
		return
	}

	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, []Location{})
		return
	}

	text := doc.snapshotText()
	line, col := doc.positionToLineCol(p.Position)
	name := wordAtPosition(text, line, col)
	if name == "" {
		s.t.writeResponse(id, []Location{})
		return
	}

	pp := parser.New(text)
	prog := pp.Parse()
	ownPath := uriToPath(p.TextDocument.URI)
	ws := s.workspaces.workspaceForURI(p.TextDocument.URI)
	siblings := ws.siblingModules(ownPath)
	scope := newScopeResolver(prog, ownPath)

	res := resolveAtPosition(prog, scope, text, ownPath, name, line, col, siblings)

	target, ok := referenceTargetFromResolution(res, prog, scope, line, col)
	if !ok {
		s.t.writeResponse(id, []Location{})
		return
	}

	var locs []Location

	// Locals/params/self stay within the open document's frame; no
	// cross-file walk.
	if target.kind == refLocal {
		locs = collectReferencesInFrame(prog, ownPath, target)
	} else {
		// Top-level kinds: walk every AST in the workspace.
		locs = append(locs, collectReferencesInProgram(prog, ownPath, target)...)
		for siblingPath, siblingProg := range siblings {
			locs = append(locs, collectReferencesInProgram(siblingProg, siblingPath, target)...)
		}
	}

	// Prepend declaration site if requested.
	if p.Context.IncludeDeclaration {
		if decl, ok := resolutionToLocation(res); ok {
			locs = append([]Location{decl}, locs...)
		}
	}

	sortLocations(locs)
	s.t.writeResponse(id, locs)
}

// referenceTargetFromResolution maps a resolver result onto a
// referenceTarget the walker can iterate against.
func referenceTargetFromResolution(res resolution, prog *ast.Program, scope *scopeResolver, line, col int) (referenceTarget, bool) {
	if res.decl != nil {
		switch res.decl.Kind {
		case declFunction, declExternFunction:
			return referenceTarget{name: res.decl.Name, kind: refFunction}, true
		case declEntity, declTrait:
			return referenceTarget{name: res.decl.Name, kind: refType}, true
		case declEnum:
			return referenceTarget{name: res.decl.Name, kind: refType}, true
		case declTest:
			return referenceTarget{name: res.decl.Name, kind: refDeclOnly}, true
		}
	}
	if res.local != nil {
		t := referenceTarget{name: res.local.Name, kind: refLocal}
		// Determine the frame the local belongs to so the walker stays
		// inside it.
		if fn := scope.enclosingFunction(line, col); fn != nil {
			t.frameFn = fn
		} else if ent, m := scope.enclosingMethod(line, col); m != nil {
			t.frameMethod = m
			t.frameMethodEntity = ent.Name
		} else if ent, c := scope.enclosingConstructor(line, col); c != nil {
			t.frameCtor = c
			t.frameMethodEntity = ent.Name
		}
		return t, true
	}
	return referenceTarget{}, false
}

// collectReferencesInProgram returns every reference position in a
// program for top-level reference kinds. Locals never use this — they
// stay in their frame via collectReferencesInFrame.
func collectReferencesInProgram(prog *ast.Program, path string, target referenceTarget) []Location {
	if prog == nil || target.kind == refDeclOnly {
		return nil
	}
	w := &refWalker{path: path, target: target}

	for _, fn := range prog.Functions {
		w.walkFunction(fn)
	}
	for _, ext := range prog.ExternFunctions {
		_ = ext // extern functions don't have bodies; declaration-only
	}
	for _, ent := range prog.Entities {
		w.walkEntity(ent)
	}
	for _, en := range prog.Enums {
		for _, v := range en.Variants {
			for _, f := range v.Fields {
				w.walkTypeRef(f.Type)
			}
		}
	}
	for _, tr := range prog.Traits {
		for _, m := range tr.Methods {
			for _, p := range m.Params {
				w.walkTypeRef(p.Type)
			}
			w.walkTypeRef(m.ReturnType)
		}
	}
	for _, impl := range prog.ImplBlocks {
		// impl block headers reference the entity name and trait name.
		// EntityName matches refType.
		if target.kind == refType && impl.EntityName == target.name {
			w.add(impl.Line, impl.Column, target.name)
		}
		if target.kind == refType && impl.TraitName == target.name {
			w.add(impl.Line, impl.Column, target.name)
		}
		for _, m := range impl.Methods {
			w.walkMethod(m)
		}
	}
	for _, t := range prog.Tests {
		w.walkBlock(t.Body)
	}
	return w.locs
}

// collectReferencesInFrame walks only the enclosing function body for a
// local-binding target.
func collectReferencesInFrame(prog *ast.Program, path string, target referenceTarget) []Location {
	w := &refWalker{path: path, target: target}
	switch {
	case target.frameFn != nil:
		// Params themselves are part of the declaration; walk the body
		// for identifier uses.
		w.walkBlock(target.frameFn.Body)
	case target.frameMethod != nil:
		w.walkBlock(target.frameMethod.Body)
	case target.frameCtor != nil:
		w.walkBlock(target.frameCtor.Body)
	}
	return w.locs
}

// refWalker accumulates reference locations.
type refWalker struct {
	path   string
	target referenceTarget
	locs   []Location
}

func (w *refWalker) add(line, col int, name string) {
	w.locs = append(w.locs, Location{
		URI: pathToURI(w.path),
		Range: Range{
			Start: lineColToPosition(line, col),
			End:   lineColToPosition(line, col+len(name)),
		},
	})
}

func (w *refWalker) walkFunction(fn *ast.FunctionDecl) {
	for _, p := range fn.Params {
		w.walkTypeRef(p.Type)
	}
	w.walkTypeRef(fn.ReturnType)
	for _, c := range fn.Requires {
		w.walkExpr(c.Expr)
	}
	for _, c := range fn.Ensures {
		w.walkExpr(c.Expr)
	}
	w.walkBlock(fn.Body)
}

func (w *refWalker) walkEntity(ent *ast.EntityDecl) {
	for _, f := range ent.Fields {
		w.walkTypeRef(f.Type)
	}
	for _, c := range ent.Invariants {
		w.walkExpr(c.Expr)
	}
	if ent.Constructor != nil {
		for _, p := range ent.Constructor.Params {
			w.walkTypeRef(p.Type)
		}
		for _, c := range ent.Constructor.Requires {
			w.walkExpr(c.Expr)
		}
		for _, c := range ent.Constructor.Ensures {
			w.walkExpr(c.Expr)
		}
		w.walkBlock(ent.Constructor.Body)
	}
	for _, m := range ent.Methods {
		w.walkMethod(m)
	}
}

func (w *refWalker) walkMethod(m *ast.MethodDecl) {
	for _, p := range m.Params {
		w.walkTypeRef(p.Type)
	}
	w.walkTypeRef(m.ReturnType)
	for _, c := range m.Requires {
		w.walkExpr(c.Expr)
	}
	for _, c := range m.Ensures {
		w.walkExpr(c.Expr)
	}
	w.walkBlock(m.Body)
}

func (w *refWalker) walkTypeRef(t *ast.TypeRef) {
	if t == nil {
		return
	}
	if w.target.kind == refType && t.Name == w.target.name {
		w.add(t.Line, t.Column, t.Name)
	}
	for _, a := range t.TypeArgs {
		w.walkTypeRef(a)
	}
	for _, pt := range t.ParamTypes {
		w.walkTypeRef(pt)
	}
	w.walkTypeRef(t.ReturnType)
}

func (w *refWalker) walkBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		w.walkStmt(s)
	}
}

func (w *refWalker) walkStmt(s ast.Statement) {
	if s == nil {
		return
	}
	switch n := s.(type) {
	case *ast.LetStmt:
		w.walkTypeRef(n.Type)
		w.walkExpr(n.Value)
	case *ast.AssignStmt:
		w.walkExpr(n.Target)
		w.walkExpr(n.Value)
	case *ast.ReturnStmt:
		w.walkExpr(n.Value)
	case *ast.IfStmt:
		w.walkExpr(n.Condition)
		w.walkBlock(n.Then)
		switch els := n.Else.(type) {
		case *ast.Block:
			w.walkBlock(els)
		case *ast.IfStmt:
			w.walkStmt(els)
		}
	case *ast.WhileStmt:
		w.walkExpr(n.Condition)
		for _, c := range n.Invariants {
			w.walkExpr(c.Expr)
		}
		if n.Decreases != nil {
			w.walkExpr(n.Decreases.Expr)
		}
		w.walkBlock(n.Body)
	case *ast.ForInStmt:
		w.walkExpr(n.Iterable)
		w.walkBlock(n.Body)
	case *ast.ExprStmt:
		w.walkExpr(n.Expr)
	}
}

func (w *refWalker) walkExpr(e ast.Expression) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.Identifier:
		// Locals get matched here. For top-level kinds we mostly match
		// in CallExpr / TypeRef; an `Identifier` referencing a function
		// name without calling it isn't legal Intent today, so this
		// arm only fires for refLocal.
		if w.target.kind == refLocal && n.Name == w.target.name {
			w.add(n.Line, n.Column, n.Name)
		}
	case *ast.CallExpr:
		// Match against function name + entity-constructor name +
		// enum-variant name (when the variant is used as a bare call).
		switch w.target.kind {
		case refFunction:
			if n.Function == w.target.name {
				w.add(n.Line, n.Column, n.Function)
			}
		case refType:
			// Entity constructors are spelled `EntityName(args)`.
			if n.Function == w.target.name {
				w.add(n.Line, n.Column, n.Function)
			}
		}
		for _, t := range n.TypeArgs {
			w.walkTypeRef(t)
		}
		for _, a := range n.Args {
			w.walkExpr(a)
		}
	case *ast.MethodCallExpr:
		// `mod.Name(args)` parses as MethodCallExpr(Object=Ident(mod),
		// Method="Name"). For refType / refFunction the qualified name
		// is a reference at the method-name position. We don't
		// disambiguate the receiver here — same-name collisions are a
		// documented v1 limitation (ADR 0035 §O5).
		if (w.target.kind == refType || w.target.kind == refFunction) && n.Method == w.target.name {
			w.add(n.Line, n.Column, n.Method)
		}
		w.walkExpr(n.Object)
		for _, a := range n.Args {
			w.walkExpr(a)
		}
	case *ast.FieldAccessExpr:
		w.walkExpr(n.Object)
		// FieldAccessExpr also represents `module.Name` for cross-package
		// references in expression position. When refType, treat the
		// `Field` as a type-name occurrence if it matches.
		if w.target.kind == refType && n.Field == w.target.name {
			// The field-name token starts after the dot. Compute its
			// column conservatively as Column + (len(receiver) + 1).
			// We don't have the receiver's source span at this layer;
			// use n.Line and n.Column + 0 as a safe anchor. (Editor
			// shows the underline near the start of the expression;
			// acceptable for v1.)
			w.add(n.Line, n.Column, n.Field)
		}
	case *ast.BinaryExpr:
		w.walkExpr(n.Left)
		w.walkExpr(n.Right)
	case *ast.UnaryExpr:
		w.walkExpr(n.Operand)
	case *ast.OldExpr:
		w.walkExpr(n.Expr)
	case *ast.IndexExpr:
		w.walkExpr(n.Object)
		w.walkExpr(n.Index)
	case *ast.RangeExpr:
		w.walkExpr(n.Start)
		w.walkExpr(n.End)
	case *ast.ForallExpr:
		w.walkExpr(n.Domain)
		w.walkExpr(n.Body)
	case *ast.ExistsExpr:
		w.walkExpr(n.Domain)
		w.walkExpr(n.Body)
	case *ast.MatchExpr:
		w.walkExpr(n.Scrutinee)
		for _, arm := range n.Arms {
			// Pattern Name is the variant name; refType-on-enum doesn't
			// natively match here today (variants render differently).
			// Walk the body.
			w.walkExpr(arm.Body)
		}
	case *ast.TryExpr:
		w.walkExpr(n.Expr)
	case *ast.LambdaExpr:
		for _, p := range n.Params {
			w.walkTypeRef(p.Type)
		}
		w.walkTypeRef(n.ReturnType)
		w.walkExpr(n.Body)
	case *ast.AwaitExpr:
		w.walkExpr(n.Expr)
	case *ast.SpawnExpr:
		w.walkExpr(n.Expr)
	}
}

func sortLocations(locs []Location) {
	sort.SliceStable(locs, func(i, j int) bool {
		if locs[i].URI != locs[j].URI {
			return locs[i].URI < locs[j].URI
		}
		if locs[i].Range.Start.Line != locs[j].Range.Start.Line {
			return locs[i].Range.Start.Line < locs[j].Range.Start.Line
		}
		return locs[i].Range.Start.Character < locs[j].Range.Start.Character
	})
}
