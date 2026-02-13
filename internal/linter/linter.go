package linter

import (
	"strings"
	"unicode"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
)

// Linter performs style and best-practice checks on an AST program.
// It reports warnings (never errors) using the diagnostic system.
type Linter struct {
	prog *ast.Program
	diag *diagnostic.Diagnostics
}

// Lint runs all lint rules on the given program and returns diagnostics.
func Lint(prog *ast.Program) *diagnostic.Diagnostics {
	l := &Linter{
		prog: prog,
		diag: diagnostic.New(),
	}

	l.lintFunctions()
	l.lintEntities()
	l.lintEnums()
	l.lintIntents()
	// Note: ImportDecl nodes are intentionally skipped -- no lint rules for imports yet

	return l.diag
}

// lintFunctions checks all top-level functions.
func (l *Linter) lintFunctions() {
	for _, fn := range l.prog.Functions {
		l.checkEmptyFunctionBody(fn.Name, fn.Body, fn.Line, fn.Column)
		l.checkMissingContracts(fn.Name, fn.Requires, fn.Ensures, fn.Line, fn.Column)
		l.checkFunctionNaming(fn.Name, fn.Line, fn.Column)

		if fn.Body != nil {
			usedNames := l.collectUsedNames(fn.Body.Statements)
			l.checkUnusedParams(fn.Name, fn.Params, usedNames)
			l.checkUnusedVariables(fn.Body.Statements, usedNames)
			l.checkMutableNeverReassigned(fn.Body.Statements, usedNames)
		}
	}
}

// lintEntities checks all entity declarations.
func (l *Linter) lintEntities() {
	for _, entity := range l.prog.Entities {
		l.checkEntityNaming(entity.Name, entity.Line, entity.Column)
		l.checkEntityWithoutInvariant(entity)

		// Check constructor
		if entity.Constructor != nil {
			ctor := entity.Constructor
			if ctor.Body != nil {
				usedNames := l.collectUsedNames(ctor.Body.Statements)
				l.checkUnusedParams(entity.Name+".constructor", ctor.Params, usedNames)
			}
		}

		// Check methods
		for _, m := range entity.Methods {
			l.checkEmptyFunctionBody(entity.Name+"."+m.Name, m.Body, m.Line, m.Column)
			l.checkMissingMethodContracts(entity.Name, m)
			l.checkFunctionNaming(m.Name, m.Line, m.Column)

			if m.Body != nil {
				usedNames := l.collectUsedNames(m.Body.Statements)
				l.checkUnusedParams(entity.Name+"."+m.Name, m.Params, usedNames)
				l.checkUnusedVariables(m.Body.Statements, usedNames)
				l.checkMutableNeverReassigned(m.Body.Statements, usedNames)
			}
		}
	}
}

// lintEnums checks all enum declarations.
func (l *Linter) lintEnums() {
	for _, enum := range l.prog.Enums {
		l.checkEnumNaming(enum.Name, enum.Line, enum.Column)
		for _, variant := range enum.Variants {
			l.checkVariantNaming(enum.Name, variant.Name, variant.Line, variant.Column)
		}
	}
}

// lintIntents checks all intent declarations.
func (l *Linter) lintIntents() {
	for _, intent := range l.prog.Intents {
		l.checkIntentEmptyVerifiedBy(intent)
	}
}

// --- Lint rules ---

// checkEmptyFunctionBody warns if a function/method body has no statements.
func (l *Linter) checkEmptyFunctionBody(name string, body *ast.Block, line, col int) {
	if body == nil || len(body.Statements) == 0 {
		l.diag.Warningf(line, col, "function '%s' has an empty body", name)
	}
}

// checkEntityWithoutInvariant warns if an entity has fields but no invariant.
func (l *Linter) checkEntityWithoutInvariant(entity *ast.EntityDecl) {
	if len(entity.Fields) > 0 && len(entity.Invariants) == 0 {
		l.diag.Warningf(entity.Line, entity.Column,
			"entity '%s' has fields but no invariant", entity.Name)
	}
}

// checkMissingContracts warns if a function has no requires or ensures clauses.
func (l *Linter) checkMissingContracts(name string, requires, ensures []*ast.ContractClause, line, col int) {
	if len(requires) == 0 && len(ensures) == 0 {
		l.diag.Warningf(line, col,
			"function '%s' has no requires or ensures contracts", name)
	}
}

// checkMissingMethodContracts warns if a method has no requires or ensures clauses.
func (l *Linter) checkMissingMethodContracts(entityName string, m *ast.MethodDecl) {
	if len(m.Requires) == 0 && len(m.Ensures) == 0 {
		l.diag.Warningf(m.Line, m.Column,
			"method '%s.%s' has no requires or ensures contracts", entityName, m.Name)
	}
}

// checkIntentEmptyVerifiedBy warns if an intent block has no verified_by references.
func (l *Linter) checkIntentEmptyVerifiedBy(intent *ast.IntentDecl) {
	if len(intent.VerifiedBy) == 0 {
		l.diag.Warningf(intent.Line, intent.Column,
			"intent block has no verified_by references")
	}
}

// checkFunctionNaming warns if a function/method name is not snake_case.
func (l *Linter) checkFunctionNaming(name string, line, col int) {
	if !isSnakeCase(name) {
		l.diag.Warningf(line, col,
			"function '%s' should use snake_case naming", name)
	}
}

// checkEntityNaming warns if an entity name is not PascalCase.
func (l *Linter) checkEntityNaming(name string, line, col int) {
	if !isPascalCase(name) {
		l.diag.Warningf(line, col,
			"entity '%s' should use PascalCase naming", name)
	}
}

// checkEnumNaming warns if an enum name is not PascalCase.
func (l *Linter) checkEnumNaming(name string, line, col int) {
	if !isPascalCase(name) {
		l.diag.Warningf(line, col,
			"enum '%s' should use PascalCase naming", name)
	}
}

// checkVariantNaming warns if a variant name is not PascalCase.
func (l *Linter) checkVariantNaming(enumName, variantName string, line, col int) {
	if !isPascalCase(variantName) {
		l.diag.Warningf(line, col,
			"variant '%s' in enum '%s' should use PascalCase naming", variantName, enumName)
	}
}

// checkUnusedParams warns about function/method parameters that are never read in the body.
func (l *Linter) checkUnusedParams(scopeName string, params []*ast.Param, usedNames map[string]bool) {
	for _, p := range params {
		if !usedNames[p.Name] {
			l.diag.Warningf(p.Line, p.Column,
				"parameter '%s' in '%s' is never used", p.Name, scopeName)
		}
	}
}

// checkUnusedVariables warns about let-bound variables that are never read.
func (l *Linter) checkUnusedVariables(stmts []ast.Statement, usedNames map[string]bool) {
	for _, stmt := range stmts {
		if letStmt, ok := stmt.(*ast.LetStmt); ok {
			if !usedNames[letStmt.Name] {
				l.diag.Warningf(letStmt.Line, letStmt.Column,
					"variable '%s' is declared but never used", letStmt.Name)
			}
		}
		// Recurse into nested blocks
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			if ifStmt.Then != nil {
				l.checkUnusedVariables(ifStmt.Then.Statements, usedNames)
			}
			if ifStmt.Else != nil {
				if block, ok := ifStmt.Else.(*ast.Block); ok {
					l.checkUnusedVariables(block.Statements, usedNames)
				} else if nestedIf, ok := ifStmt.Else.(*ast.IfStmt); ok {
					l.checkUnusedVariablesInIf(nestedIf, usedNames)
				}
			}
		}
		if whileStmt, ok := stmt.(*ast.WhileStmt); ok {
			if whileStmt.Body != nil {
				l.checkUnusedVariables(whileStmt.Body.Statements, usedNames)
			}
		}
		if forStmt, ok := stmt.(*ast.ForInStmt); ok {
			if forStmt.Body != nil {
				l.checkUnusedVariables(forStmt.Body.Statements, usedNames)
			}
		}
	}
}

func (l *Linter) checkUnusedVariablesInIf(ifStmt *ast.IfStmt, usedNames map[string]bool) {
	if ifStmt.Then != nil {
		l.checkUnusedVariables(ifStmt.Then.Statements, usedNames)
	}
	if ifStmt.Else != nil {
		if block, ok := ifStmt.Else.(*ast.Block); ok {
			l.checkUnusedVariables(block.Statements, usedNames)
		} else if nestedIf, ok := ifStmt.Else.(*ast.IfStmt); ok {
			l.checkUnusedVariablesInIf(nestedIf, usedNames)
		}
	}
}

// checkMutableNeverReassigned warns about let mutable variables that are only assigned once.
func (l *Linter) checkMutableNeverReassigned(stmts []ast.Statement, _ map[string]bool) {
	assignedNames := l.collectAssignedNames(stmts)
	for _, stmt := range stmts {
		if letStmt, ok := stmt.(*ast.LetStmt); ok {
			if letStmt.Mutable && !assignedNames[letStmt.Name] {
				l.diag.Warningf(letStmt.Line, letStmt.Column,
					"variable '%s' is declared mutable but never reassigned", letStmt.Name)
			}
		}
	}
}

// --- Name collection helpers ---

// collectUsedNames walks all expressions in a slice of statements and collects
// all identifier names that are read (referenced). This is used to detect
// unused variables and parameters.
func (l *Linter) collectUsedNames(stmts []ast.Statement) map[string]bool {
	used := make(map[string]bool)
	for _, stmt := range stmts {
		l.collectUsedNamesFromStmt(stmt, used)
	}
	return used
}

func (l *Linter) collectUsedNamesFromStmt(stmt ast.Statement, used map[string]bool) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		// The initializer expression reads names, but the declared name is not a read
		l.collectUsedNamesFromExpr(s.Value, used)
	case *ast.AssignStmt:
		// The target is a write, but if it's a field access or index expr the object is read.
		// Only collect from value side and field/index access objects.
		if fa, ok := s.Target.(*ast.FieldAccessExpr); ok {
			l.collectUsedNamesFromExpr(fa.Object, used)
		}
		if ie, ok := s.Target.(*ast.IndexExpr); ok {
			l.collectUsedNamesFromExpr(ie.Object, used)
			l.collectUsedNamesFromExpr(ie.Index, used)
		}
		l.collectUsedNamesFromExpr(s.Value, used)
	case *ast.ReturnStmt:
		if s.Value != nil {
			l.collectUsedNamesFromExpr(s.Value, used)
		}
	case *ast.IfStmt:
		l.collectUsedNamesFromExpr(s.Condition, used)
		if s.Then != nil {
			for _, inner := range s.Then.Statements {
				l.collectUsedNamesFromStmt(inner, used)
			}
		}
		if s.Else != nil {
			l.collectUsedNamesFromStmt(s.Else, used)
		}
	case *ast.WhileStmt:
		l.collectUsedNamesFromExpr(s.Condition, used)
		// Collect names from invariants
		for _, inv := range s.Invariants {
			l.collectUsedNamesFromExpr(inv.Expr, used)
		}
		// Collect names from decreases clause
		if s.Decreases != nil {
			l.collectUsedNamesFromExpr(s.Decreases.Expr, used)
		}
		if s.Body != nil {
			for _, inner := range s.Body.Statements {
				l.collectUsedNamesFromStmt(inner, used)
			}
		}
	case *ast.ForInStmt:
		l.collectUsedNamesFromExpr(s.Iterable, used)
		if s.Body != nil {
			for _, inner := range s.Body.Statements {
				l.collectUsedNamesFromStmt(inner, used)
			}
		}
	case *ast.ExprStmt:
		l.collectUsedNamesFromExpr(s.Expr, used)
	case *ast.Block:
		for _, inner := range s.Statements {
			l.collectUsedNamesFromStmt(inner, used)
		}
	}
}

func (l *Linter) collectUsedNamesFromExpr(expr ast.Expression, used map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		used[e.Name] = true
	case *ast.BinaryExpr:
		l.collectUsedNamesFromExpr(e.Left, used)
		l.collectUsedNamesFromExpr(e.Right, used)
	case *ast.UnaryExpr:
		l.collectUsedNamesFromExpr(e.Operand, used)
	case *ast.CallExpr:
		for _, arg := range e.Args {
			l.collectUsedNamesFromExpr(arg, used)
		}
	case *ast.MethodCallExpr:
		l.collectUsedNamesFromExpr(e.Object, used)
		for _, arg := range e.Args {
			l.collectUsedNamesFromExpr(arg, used)
		}
	case *ast.FieldAccessExpr:
		l.collectUsedNamesFromExpr(e.Object, used)
	case *ast.OldExpr:
		l.collectUsedNamesFromExpr(e.Expr, used)
	case *ast.ArrayLit:
		for _, elem := range e.Elements {
			l.collectUsedNamesFromExpr(elem, used)
		}
	case *ast.IndexExpr:
		l.collectUsedNamesFromExpr(e.Object, used)
		l.collectUsedNamesFromExpr(e.Index, used)
	case *ast.RangeExpr:
		l.collectUsedNamesFromExpr(e.Start, used)
		l.collectUsedNamesFromExpr(e.End, used)
	case *ast.ForallExpr:
		// Collect from domain and body, but not the bound variable
		if e.Domain != nil {
			l.collectUsedNamesFromExpr(e.Domain.Start, used)
			l.collectUsedNamesFromExpr(e.Domain.End, used)
		}
		l.collectUsedNamesFromExpr(e.Body, used)
		// Note: e.Variable is NOT an external used name - it's defined by the quantifier
	case *ast.ExistsExpr:
		// Collect from domain and body, but not the bound variable
		if e.Domain != nil {
			l.collectUsedNamesFromExpr(e.Domain.Start, used)
			l.collectUsedNamesFromExpr(e.Domain.End, used)
		}
		l.collectUsedNamesFromExpr(e.Body, used)
		// Note: e.Variable is NOT an external used name - it's defined by the quantifier
	case *ast.MatchExpr:
		// Collect from scrutinee
		l.collectUsedNamesFromExpr(e.Scrutinee, used)
		// Collect from each arm's body (but not pattern bindings)
		for _, arm := range e.Arms {
			l.collectUsedNamesFromExpr(arm.Body, used)
			// Note: pattern bindings are NOT external used names - they're defined by the pattern
		}
	case *ast.TryExpr:
		// Collect from inner expression
		l.collectUsedNamesFromExpr(e.Expr, used)
	}
}

// collectAssignedNames walks statements and collects names that appear as
// assignment targets (not let initializers).
func (l *Linter) collectAssignedNames(stmts []ast.Statement) map[string]bool {
	assigned := make(map[string]bool)
	l.collectAssignedNamesFromStmts(stmts, assigned)
	return assigned
}

func (l *Linter) collectAssignedNamesFromStmts(stmts []ast.Statement, assigned map[string]bool) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if ident, ok := s.Target.(*ast.Identifier); ok {
				assigned[ident.Name] = true
			}
		case *ast.IfStmt:
			if s.Then != nil {
				l.collectAssignedNamesFromStmts(s.Then.Statements, assigned)
			}
			if s.Else != nil {
				if block, ok := s.Else.(*ast.Block); ok {
					l.collectAssignedNamesFromStmts(block.Statements, assigned)
				} else if nestedIf, ok := s.Else.(*ast.IfStmt); ok {
					l.collectAssignedNamesFromIfStmt(nestedIf, assigned)
				}
			}
		case *ast.WhileStmt:
			if s.Body != nil {
				l.collectAssignedNamesFromStmts(s.Body.Statements, assigned)
			}
		case *ast.ForInStmt:
			if s.Body != nil {
				l.collectAssignedNamesFromStmts(s.Body.Statements, assigned)
			}
		case *ast.Block:
			l.collectAssignedNamesFromStmts(s.Statements, assigned)
		}
	}
}

func (l *Linter) collectAssignedNamesFromIfStmt(ifStmt *ast.IfStmt, assigned map[string]bool) {
	if ifStmt.Then != nil {
		l.collectAssignedNamesFromStmts(ifStmt.Then.Statements, assigned)
	}
	if ifStmt.Else != nil {
		if block, ok := ifStmt.Else.(*ast.Block); ok {
			l.collectAssignedNamesFromStmts(block.Statements, assigned)
		} else if nestedIf, ok := ifStmt.Else.(*ast.IfStmt); ok {
			l.collectAssignedNamesFromIfStmt(nestedIf, assigned)
		}
	}
}

// --- Naming convention helpers ---

// isSnakeCase returns true if the name follows snake_case conventions:
// lowercase letters, digits, and underscores only, not starting with a digit.
func isSnakeCase(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if !unicode.IsLower(r) && r != '_' && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isPascalCase returns true if the name starts with an uppercase letter
// and contains no underscores.
func isPascalCase(name string) bool {
	if len(name) == 0 {
		return false
	}
	runes := []rune(name)
	if !unicode.IsUpper(runes[0]) {
		return false
	}
	return !strings.ContainsRune(name, '_')
}
