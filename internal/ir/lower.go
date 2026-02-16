package ir

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/lexer"
)

// lowerer transforms an AST + CheckResult into IR nodes.
type lowerer struct {
	exprTypes map[ast.Expression]*checker.Type
	entities  map[string]*checker.EntityInfo
	enums     map[string]*checker.EnumInfo

	// old() capture state for current method/constructor/while
	oldCounter  int
	oldCaptures []*OldCapture
	oldMap      map[ast.Expression]string // maps AST OldExpr to capture name
}

// Lower transforms a single-file AST program into an IR Module.
func Lower(prog *ast.Program, result *checker.CheckResult) *Module {
	l := &lowerer{
		exprTypes: result.ExprTypes,
		entities:  result.Entities,
		enums:     result.Enums,
	}

	modName := ""
	if prog.Module != nil {
		modName = prog.Module.Name
	}

	mod := &Module{
		Name:    modName,
		IsEntry: true,
	}

	for _, e := range prog.Entities {
		mod.Entities = append(mod.Entities, l.lowerEntity(e))
	}
	for _, e := range prog.Enums {
		mod.Enums = append(mod.Enums, l.lowerEnum(e))
	}
	for _, f := range prog.Functions {
		mod.Functions = append(mod.Functions, l.lowerFunction(f))
	}
	for _, i := range prog.Intents {
		mod.Intents = append(mod.Intents, l.lowerIntent(i))
	}

	return mod
}

// LowerAll transforms a multi-file project into an IR Program.
func LowerAll(registry map[string]*ast.Program, sortedPaths []string, result *checker.CheckAllResult) *Program {
	if len(sortedPaths) == 0 {
		return &Program{}
	}

	entryPath := sortedPaths[len(sortedPaths)-1]

	l := &lowerer{
		exprTypes: result.ExprTypes,
		entities:  result.Entities,
		enums:     result.Enums,
	}

	prog := &Program{}

	for _, filePath := range sortedPaths {
		p := registry[filePath]
		if p == nil {
			continue
		}

		modName := strings.TrimSuffix(filepath.Base(filePath), ".intent")
		isEntry := filePath == entryPath

		mod := &Module{
			Name:    modName,
			IsEntry: isEntry,
			Path:    filePath,
		}

		for _, e := range p.Entities {
			mod.Entities = append(mod.Entities, l.lowerEntity(e))
		}
		for _, e := range p.Enums {
			mod.Enums = append(mod.Enums, l.lowerEnum(e))
		}
		for _, f := range p.Functions {
			mod.Functions = append(mod.Functions, l.lowerFunction(f))
		}
		for _, i := range p.Intents {
			mod.Intents = append(mod.Intents, l.lowerIntent(i))
		}

		prog.Modules = append(prog.Modules, mod)
	}

	return prog
}

// --- Top-level lowering ---

func (l *lowerer) lowerFunction(f *ast.FunctionDecl) *Function {
	fn := &Function{
		Name:       f.Name,
		IsEntry:    f.IsEntry,
		IsPublic:   f.IsPublic,
		ReturnType: l.resolveTypeRef(f.ReturnType),
	}

	for _, p := range f.Params {
		fn.Params = append(fn.Params, &Param{
			Name: p.Name,
			Type: l.resolveTypeRef(p.Type),
		})
	}

	for _, req := range f.Requires {
		fn.Requires = append(fn.Requires, l.lowerContract(req))
	}
	for _, ens := range f.Ensures {
		fn.Ensures = append(fn.Ensures, l.lowerContract(ens))
	}

	fn.Body = l.lowerBlock(f.Body)

	return fn
}

func (l *lowerer) lowerEntity(e *ast.EntityDecl) *Entity {
	ent := &Entity{
		Name:     e.Name,
		IsPublic: e.IsPublic,
	}

	for _, f := range e.Fields {
		ent.Fields = append(ent.Fields, &Field{
			Name: f.Name,
			Type: l.resolveTypeRef(f.Type),
		})
	}

	for _, inv := range e.Invariants {
		ent.Invariants = append(ent.Invariants, &Contract{
			Expr:    l.lowerExpr(inv.Expr),
			RawText: inv.RawText,
		})
	}

	if e.Constructor != nil {
		ent.Constructor = l.lowerConstructor(e.Constructor)
	}

	for _, m := range e.Methods {
		ent.Methods = append(ent.Methods, l.lowerMethod(m))
	}

	return ent
}

func (l *lowerer) lowerConstructor(c *ast.ConstructorDecl) *Constructor {
	ctor := &Constructor{}

	for _, p := range c.Params {
		ctor.Params = append(ctor.Params, &Param{
			Name: p.Name,
			Type: l.resolveTypeRef(p.Type),
		})
	}

	for _, req := range c.Requires {
		ctor.Requires = append(ctor.Requires, l.lowerContract(req))
	}

	// Extract old() captures from ensures clauses
	l.resetOldCaptures()
	for _, ens := range c.Ensures {
		l.scanOldExprs(ens.Expr)
	}
	ctor.OldCaptures = l.oldCaptures

	for _, ens := range c.Ensures {
		ctor.Ensures = append(ctor.Ensures, l.lowerContractWithOld(ens))
	}

	ctor.Body = l.lowerBlock(c.Body)
	l.resetOldCaptures()

	return ctor
}

func (l *lowerer) lowerMethod(m *ast.MethodDecl) *Method {
	method := &Method{
		Name:       m.Name,
		ReturnType: l.resolveTypeRef(m.ReturnType),
	}

	for _, p := range m.Params {
		method.Params = append(method.Params, &Param{
			Name: p.Name,
			Type: l.resolveTypeRef(p.Type),
		})
	}

	for _, req := range m.Requires {
		method.Requires = append(method.Requires, l.lowerContract(req))
	}

	// Extract old() captures from ensures clauses
	l.resetOldCaptures()
	for _, ens := range m.Ensures {
		l.scanOldExprs(ens.Expr)
	}
	method.OldCaptures = l.oldCaptures

	for _, ens := range m.Ensures {
		method.Ensures = append(method.Ensures, l.lowerContractWithOld(ens))
	}

	method.Body = l.lowerBlock(m.Body)
	l.resetOldCaptures()

	return method
}

func (l *lowerer) lowerEnum(e *ast.EnumDecl) *Enum {
	en := &Enum{
		Name:     e.Name,
		IsPublic: e.IsPublic,
	}
	for _, v := range e.Variants {
		variant := &EnumVariant{Name: v.Name}
		for _, f := range v.Fields {
			variant.Fields = append(variant.Fields, &Field{
				Name: f.Name,
				Type: l.resolveTypeRef(f.Type),
			})
		}
		en.Variants = append(en.Variants, variant)
	}
	return en
}

func (l *lowerer) lowerIntent(i *ast.IntentDecl) *Intent {
	intent := &Intent{
		Description: i.Description,
		Goals:       i.Goals,
		Constraints: i.Constraints,
		Guarantees:  i.Guarantees,
	}
	for _, vb := range i.VerifiedBy {
		intent.VerifiedBy = append(intent.VerifiedBy, vb.Parts)
	}
	return intent
}

func (l *lowerer) lowerContract(c *ast.ContractClause) *Contract {
	return &Contract{
		Expr:    l.lowerExpr(c.Expr),
		RawText: c.RawText,
	}
}

// lowerContractWithOld lowers a contract clause, replacing OldExpr with OldRef.
func (l *lowerer) lowerContractWithOld(c *ast.ContractClause) *Contract {
	return &Contract{
		Expr:    l.lowerExprWithOld(c.Expr),
		RawText: c.RawText,
	}
}

// --- old() extraction ---

func (l *lowerer) resetOldCaptures() {
	l.oldCounter = 0
	l.oldCaptures = nil
	l.oldMap = make(map[ast.Expression]string)
}

// scanOldExprs walks an AST expression to find all old() expressions
// and creates OldCapture entries for them.
func (l *lowerer) scanOldExprs(e ast.Expression) {
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		l.scanOldExprs(expr.Left)
		l.scanOldExprs(expr.Right)
	case *ast.UnaryExpr:
		l.scanOldExprs(expr.Operand)
	case *ast.CallExpr:
		for _, arg := range expr.Args {
			l.scanOldExprs(arg)
		}
	case *ast.MethodCallExpr:
		l.scanOldExprs(expr.Object)
		for _, arg := range expr.Args {
			l.scanOldExprs(arg)
		}
	case *ast.FieldAccessExpr:
		l.scanOldExprs(expr.Object)
	case *ast.OldExpr:
		// Use the same mangling scheme as the old codegen for compatibility
		name := l.mangleOldExpr(expr.Expr)
		if _, exists := l.oldMap[e]; !exists {
			l.oldMap[e] = name
			l.oldCaptures = append(l.oldCaptures, &OldCapture{
				Name: name,
				Expr: l.lowerExpr(expr.Expr),
			})
		}
	case *ast.ForallExpr:
		if expr.Domain != nil {
			l.scanOldExprs(expr.Domain.Start)
			l.scanOldExprs(expr.Domain.End)
		}
		l.scanOldExprs(expr.Body)
	case *ast.ExistsExpr:
		if expr.Domain != nil {
			l.scanOldExprs(expr.Domain.Start)
			l.scanOldExprs(expr.Domain.End)
		}
		l.scanOldExprs(expr.Body)
	case *ast.IndexExpr:
		l.scanOldExprs(expr.Object)
		l.scanOldExprs(expr.Index)
	}
}

// mangleOldExpr creates a mangled variable name for an old() expression,
// matching the codegen's naming scheme for compatibility.
func (l *lowerer) mangleOldExpr(e ast.Expression) string {
	text := l.exprToText(e)
	text = strings.ReplaceAll(text, ".", "_")
	text = strings.ReplaceAll(text, "(", "")
	text = strings.ReplaceAll(text, ")", "")
	text = strings.ReplaceAll(text, " ", "_")
	return "__old_" + text
}

func (l *lowerer) exprToText(e ast.Expression) string {
	switch expr := e.(type) {
	case *ast.FieldAccessExpr:
		return l.exprToText(expr.Object) + "." + expr.Field
	case *ast.SelfExpr:
		return "self"
	case *ast.Identifier:
		return expr.Name
	default:
		return "expr"
	}
}

// lowerExprWithOld lowers an expression, replacing OldExpr nodes with OldRef.
func (l *lowerer) lowerExprWithOld(e ast.Expression) Expr {
	switch expr := e.(type) {
	case *ast.OldExpr:
		name := l.mangleOldExpr(expr.Expr)
		t := l.typeOf(e)
		return &OldRef{Name: name, Type: t}
	case *ast.BinaryExpr:
		left := l.lowerExprWithOld(expr.Left)
		right := l.lowerExprWithOld(expr.Right)
		t := l.typeOf(e)
		// Check for string concatenation
		if expr.Op == lexer.PLUS && l.isStringType(expr.Left) && l.isStringType(expr.Right) {
			return &StringConcat{Left: left, Right: right, Type: t}
		}
		return &BinaryExpr{Left: left, Op: expr.Op, Right: right, Type: t}
	case *ast.UnaryExpr:
		return &UnaryExpr{Op: expr.Op, Operand: l.lowerExprWithOld(expr.Operand), Type: l.typeOf(e)}
	case *ast.CallExpr:
		return l.lowerCallExprWithOldArgs(expr, e)
	case *ast.MethodCallExpr:
		return l.lowerMethodCallExprWithOld(expr, e)
	case *ast.FieldAccessExpr:
		return &FieldAccessExpr{Object: l.lowerExprWithOld(expr.Object), Field: expr.Field, Type: l.typeOf(e)}
	case *ast.ForallExpr:
		domain := l.lowerRangeExprWithOld(expr.Domain)
		return &ForallExpr{Variable: expr.Variable, Domain: domain, Body: l.lowerExprWithOld(expr.Body), Type: l.typeOf(e)}
	case *ast.ExistsExpr:
		domain := l.lowerRangeExprWithOld(expr.Domain)
		return &ExistsExpr{Variable: expr.Variable, Domain: domain, Body: l.lowerExprWithOld(expr.Body), Type: l.typeOf(e)}
	case *ast.IndexExpr:
		return &IndexExpr{Object: l.lowerExprWithOld(expr.Object), Index: l.lowerExprWithOld(expr.Index), Type: l.typeOf(e)}
	default:
		return l.lowerExpr(e)
	}
}

func (l *lowerer) lowerRangeExprWithOld(r *ast.RangeExpr) *RangeExpr {
	if r == nil {
		return nil
	}
	return &RangeExpr{
		Start: l.lowerExprWithOld(r.Start),
		End:   l.lowerExprWithOld(r.End),
		Type:  l.typeOf(r),
	}
}

func (l *lowerer) lowerCallExprWithOldArgs(expr *ast.CallExpr, orig ast.Expression) Expr {
	args := make([]Expr, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = l.lowerExprWithOld(a)
	}
	kind, enumName := l.resolveCallKind(expr)
	return &CallExpr{
		Function: expr.Function,
		Args:     args,
		Kind:     kind,
		EnumName: enumName,
		Type:     l.typeOf(orig),
	}
}

func (l *lowerer) lowerMethodCallExprWithOld(expr *ast.MethodCallExpr, orig ast.Expression) Expr {
	args := make([]Expr, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = l.lowerExprWithOld(a)
	}
	obj := l.lowerExprWithOld(expr.Object)
	return &MethodCallExpr{
		Object: obj,
		Method: expr.Method,
		Args:   args,
		Type:   l.typeOf(orig),
	}
}

// --- Statement lowering ---

func (l *lowerer) lowerBlock(b *ast.Block) []Stmt {
	if b == nil {
		return nil
	}
	var stmts []Stmt
	for _, s := range b.Statements {
		stmts = append(stmts, l.lowerStmt(s))
	}
	return stmts
}

func (l *lowerer) lowerStmt(s ast.Statement) Stmt {
	switch stmt := s.(type) {
	case *ast.LetStmt:
		return &LetStmt{
			Name:    stmt.Name,
			Mutable: stmt.Mutable,
			Type:    l.resolveTypeRef(stmt.Type),
			Value:   l.lowerExpr(stmt.Value),
		}
	case *ast.AssignStmt:
		return &AssignStmt{
			Target: l.lowerExpr(stmt.Target),
			Value:  l.lowerExpr(stmt.Value),
		}
	case *ast.ReturnStmt:
		var val Expr
		if stmt.Value != nil {
			val = l.lowerExpr(stmt.Value)
		}
		return &ReturnStmt{Value: val}
	case *ast.IfStmt:
		return l.lowerIfStmt(stmt)
	case *ast.WhileStmt:
		return l.lowerWhileStmt(stmt)
	case *ast.ForInStmt:
		return &ForInStmt{
			Variable: stmt.Variable,
			Iterable: l.lowerExpr(stmt.Iterable),
			Body:     l.lowerBlock(stmt.Body),
		}
	case *ast.BreakStmt:
		return &BreakStmt{}
	case *ast.ContinueStmt:
		return &ContinueStmt{}
	case *ast.ExprStmt:
		return &ExprStmt{Expr: l.lowerExpr(stmt.Expr)}
	case *ast.Block:
		// Inline blocks become a sequence; wrap in a single-element if needed
		// For now, flatten into surrounding scope (same as codegen)
		stmts := l.lowerBlock(stmt)
		if len(stmts) == 1 {
			return stmts[0]
		}
		// Return first stmt; this edge case shouldn't normally occur
		if len(stmts) > 0 {
			return stmts[0]
		}
		return &ExprStmt{Expr: &BoolLit{Value: true, Type: checker.TypeBool}}
	default:
		return &ExprStmt{Expr: &BoolLit{Value: true, Type: checker.TypeBool}}
	}
}

func (l *lowerer) lowerIfStmt(stmt *ast.IfStmt) *IfStmt {
	irIf := &IfStmt{
		Condition: l.lowerExpr(stmt.Condition),
		Then:      l.lowerBlock(stmt.Then),
	}
	if stmt.Else != nil {
		switch e := stmt.Else.(type) {
		case *ast.Block:
			irIf.Else = l.lowerBlock(e)
		case *ast.IfStmt:
			// Else-if becomes a single-element else containing an IfStmt
			irIf.Else = []Stmt{l.lowerIfStmt(e)}
		}
	}
	return irIf
}

func (l *lowerer) lowerWhileStmt(stmt *ast.WhileStmt) *WhileStmt {
	w := &WhileStmt{
		Condition: l.lowerExpr(stmt.Condition),
	}

	// Handle loop invariant old() captures
	hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil
	if hasContracts {
		savedOldCaptures := l.oldCaptures
		savedOldMap := l.oldMap
		savedOldCounter := l.oldCounter
		l.resetOldCaptures()

		for _, inv := range stmt.Invariants {
			l.scanOldExprs(inv.Expr)
		}
		w.OldCaptures = l.oldCaptures

		for _, inv := range stmt.Invariants {
			w.Invariants = append(w.Invariants, l.lowerContractWithOld(inv))
		}

		// Restore
		l.oldCaptures = savedOldCaptures
		l.oldMap = savedOldMap
		l.oldCounter = savedOldCounter
	} else {
		for _, inv := range stmt.Invariants {
			w.Invariants = append(w.Invariants, l.lowerContract(inv))
		}
	}

	if stmt.Decreases != nil {
		w.Decreases = &DecreasesClause{
			Expr:    l.lowerExpr(stmt.Decreases.Expr),
			RawText: stmt.Decreases.RawText,
		}
	}

	w.Body = l.lowerBlock(stmt.Body)

	return w
}

// --- Expression lowering ---

func (l *lowerer) lowerExpr(e ast.Expression) Expr {
	if e == nil {
		return nil
	}
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		left := l.lowerExpr(expr.Left)
		right := l.lowerExpr(expr.Right)
		t := l.typeOf(e)
		// Detect string concatenation
		if expr.Op == lexer.PLUS && l.isStringType(expr.Left) && l.isStringType(expr.Right) {
			return &StringConcat{Left: left, Right: right, Type: t}
		}
		return &BinaryExpr{Left: left, Op: expr.Op, Right: right, Type: t}

	case *ast.UnaryExpr:
		return &UnaryExpr{
			Op:      expr.Op,
			Operand: l.lowerExpr(expr.Operand),
			Type:    l.typeOf(e),
		}

	case *ast.CallExpr:
		args := make([]Expr, len(expr.Args))
		for i, a := range expr.Args {
			args[i] = l.lowerExpr(a)
		}
		kind, enumName := l.resolveCallKind(expr)
		return &CallExpr{
			Function: expr.Function,
			Args:     args,
			Kind:     kind,
			EnumName: enumName,
			Type:     l.typeOf(e),
		}

	case *ast.MethodCallExpr:
		return l.lowerMethodCallExpr(expr, e)

	case *ast.FieldAccessExpr:
		return &FieldAccessExpr{
			Object: l.lowerExpr(expr.Object),
			Field:  expr.Field,
			Type:   l.typeOf(e),
		}

	case *ast.OldExpr:
		// In non-ensures context, old() just evaluates to the inner expression
		return l.lowerExpr(expr.Expr)

	case *ast.Identifier:
		t := l.typeOf(e)
		// Check if it's a unit enum variant
		if t != nil && t.IsEnum {
			enumName := l.resolveEnumForVariant(expr.Name)
			if enumName != "" {
				return &CallExpr{
					Function: expr.Name,
					Kind:     CallVariant,
					EnumName: enumName,
					Type:     t,
				}
			}
		}
		// Handle None specifically
		if expr.Name == "None" {
			return &CallExpr{
				Function: "None",
				Kind:     CallBuiltin,
				Type:     t,
			}
		}
		return &VarRef{Name: expr.Name, Type: t}

	case *ast.SelfExpr:
		return &SelfRef{Type: l.typeOf(e)}

	case *ast.ResultExpr:
		return &ResultRef{Type: l.typeOf(e)}

	case *ast.IntLit:
		val, _ := strconv.ParseInt(expr.Value, 10, 64)
		return &IntLit{Value: val, Type: l.typeOf(e)}

	case *ast.FloatLit:
		return &FloatLit{Value: expr.Value, Type: l.typeOf(e)}

	case *ast.StringLit:
		return &StringLit{Value: expr.Value, Type: l.typeOf(e)}

	case *ast.StringInterp:
		interp := &StringInterp{Type: l.typeOf(e)}
		for _, part := range expr.Parts {
			if part.IsExpr {
				interp.Parts = append(interp.Parts, StringInterpPart{
					IsExpr: true,
					Expr:   l.lowerExpr(part.Expr),
				})
			} else {
				interp.Parts = append(interp.Parts, StringInterpPart{
					IsExpr: false,
					Static: part.Static,
				})
			}
		}
		return interp

	case *ast.BoolLit:
		return &BoolLit{Value: expr.Value, Type: l.typeOf(e)}

	case *ast.ArrayLit:
		elems := make([]Expr, len(expr.Elements))
		for i, el := range expr.Elements {
			elems[i] = l.lowerExpr(el)
		}
		return &ArrayLit{Elements: elems, Type: l.typeOf(e)}

	case *ast.IndexExpr:
		return &IndexExpr{
			Object: l.lowerExpr(expr.Object),
			Index:  l.lowerExpr(expr.Index),
			Type:   l.typeOf(e),
		}

	case *ast.RangeExpr:
		return &RangeExpr{
			Start: l.lowerExpr(expr.Start),
			End:   l.lowerExpr(expr.End),
			Type:  l.typeOf(e),
		}

	case *ast.ForallExpr:
		var domain *RangeExpr
		if expr.Domain != nil {
			domain = &RangeExpr{
				Start: l.lowerExpr(expr.Domain.Start),
				End:   l.lowerExpr(expr.Domain.End),
				Type:  l.typeOf(expr.Domain),
			}
		}
		return &ForallExpr{
			Variable: expr.Variable,
			Domain:   domain,
			Body:     l.lowerExpr(expr.Body),
			Type:     l.typeOf(e),
		}

	case *ast.ExistsExpr:
		var domain *RangeExpr
		if expr.Domain != nil {
			domain = &RangeExpr{
				Start: l.lowerExpr(expr.Domain.Start),
				End:   l.lowerExpr(expr.Domain.End),
				Type:  l.typeOf(expr.Domain),
			}
		}
		return &ExistsExpr{
			Variable: expr.Variable,
			Domain:   domain,
			Body:     l.lowerExpr(expr.Body),
			Type:     l.typeOf(e),
		}

	case *ast.MatchExpr:
		return l.lowerMatchExpr(expr, e)

	case *ast.TryExpr:
		return &TryExpr{
			Expr: l.lowerExpr(expr.Expr),
			Type: l.typeOf(e),
		}

	default:
		return &BoolLit{Value: true, Type: checker.TypeBool}
	}
}

func (l *lowerer) lowerMethodCallExpr(expr *ast.MethodCallExpr, orig ast.Expression) Expr {
	args := make([]Expr, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = l.lowerExpr(a)
	}

	// Check if Object is an identifier that could be a module name
	// We detect this by checking if the object has no type (module names aren't expressions)
	// or if the identifier resolves to nothing in the type map
	isModuleCall := false
	moduleName := ""
	if ident, ok := expr.Object.(*ast.Identifier); ok {
		objType := l.typeOf(expr.Object)
		if objType == nil {
			// Object has no type - likely a module name
			isModuleCall = true
			moduleName = ident.Name
		}
	}

	obj := l.lowerExpr(expr.Object)

	mc := &MethodCallExpr{
		Object:       obj,
		Method:       expr.Method,
		Args:         args,
		IsModuleCall: isModuleCall,
		ModuleName:   moduleName,
		Type:         l.typeOf(orig),
	}

	if isModuleCall {
		// Determine if it's a function call or entity constructor
		if _, isEntity := l.entities[expr.Method]; isEntity {
			mc.CallKind = CallConstructor
		} else {
			mc.CallKind = CallFunction
		}
	}

	return mc
}

func (l *lowerer) lowerMatchExpr(expr *ast.MatchExpr, orig ast.Expression) *MatchExpr {
	m := &MatchExpr{
		Scrutinee: l.lowerExpr(expr.Scrutinee),
		Type:      l.typeOf(orig),
	}

	// Get scrutinee type to resolve enum name
	scrutType := l.typeOf(expr.Scrutinee)
	enumName := ""
	if scrutType != nil && scrutType.IsEnum {
		enumName = scrutType.Name
	}

	for _, arm := range expr.Arms {
		irArm := &MatchArm{
			Body: l.lowerExpr(arm.Body),
		}

		pattern := &MatchPattern{
			VariantName: arm.Pattern.VariantName,
			Bindings:    arm.Pattern.Bindings,
			IsWildcard:  arm.Pattern.IsWildcard,
			EnumName:    enumName,
		}

		// Detect builtin patterns (Ok, Err, Some, None)
		if arm.Pattern.VariantName == "Ok" || arm.Pattern.VariantName == "Err" ||
			arm.Pattern.VariantName == "Some" || arm.Pattern.VariantName == "None" {
			pattern.IsBuiltin = true
		}

		// Resolve field names for non-builtin, non-wildcard patterns
		if !pattern.IsWildcard && !pattern.IsBuiltin && len(arm.Pattern.Bindings) > 0 {
			pattern.FieldNames = l.resolveVariantFieldNames(enumName, arm.Pattern.VariantName)
		}

		irArm.Pattern = pattern
		m.Arms = append(m.Arms, irArm)
	}

	return m
}

// --- Helper methods ---

func (l *lowerer) typeOf(e ast.Expression) *checker.Type {
	if l.exprTypes != nil {
		if t, ok := l.exprTypes[e]; ok {
			return t
		}
	}
	return nil
}

func (l *lowerer) isStringType(e ast.Expression) bool {
	t := l.typeOf(e)
	return t != nil && t.Equal(checker.TypeString)
}

func (l *lowerer) resolveTypeRef(ref *ast.TypeRef) *checker.Type {
	if ref == nil {
		return checker.TypeVoid
	}
	return checker.ResolveType(ref, l.entities, l.enums)
}

// resolveCallKind determines the CallKind for a CallExpr.
func (l *lowerer) resolveCallKind(expr *ast.CallExpr) (CallKind, string) {
	// Builtins
	switch expr.Function {
	case "print", "len":
		return CallBuiltin, ""
	case "Ok", "Err", "Some":
		return CallBuiltin, ""
	}

	// Check if it's a variant constructor
	enumName := l.resolveEnumForVariant(expr.Function)
	if enumName != "" {
		return CallVariant, enumName
	}

	// Check if it's an entity constructor
	if _, isEntity := l.entities[expr.Function]; isEntity {
		return CallConstructor, ""
	}

	// Regular function
	return CallFunction, ""
}

// resolveEnumForVariant finds the parent enum for a variant name.
func (l *lowerer) resolveEnumForVariant(variantName string) string {
	for enumName, info := range l.enums {
		for _, v := range info.Variants {
			if v.Name == variantName {
				return enumName
			}
		}
	}
	return ""
}

// resolveVariantFieldNames returns the field names for a variant in an enum.
func (l *lowerer) resolveVariantFieldNames(enumName, variantName string) []string {
	if info, ok := l.enums[enumName]; ok {
		for _, v := range info.Variants {
			if v.Name == variantName {
				names := make([]string, len(v.Fields))
				for i, f := range v.Fields {
					names[i] = f.Name
				}
				return names
			}
		}
	}
	return nil
}
