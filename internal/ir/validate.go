package ir

import (
	"fmt"
)

// Validate checks an IR module for correctness and returns a list of error messages.
// An empty slice indicates the module is valid.
func Validate(mod *Module) []string {
	var errors []string

	// Check entry module requirements
	if mod.IsEntry {
		hasMainFunc := false
		for _, fn := range mod.Functions {
			if fn.Name == "main" {
				hasMainFunc = true
				if !fn.IsEntry {
					errors = append(errors, "entry module has function named 'main' but IsEntry is false")
				}
				break
			}
		}
		if !hasMainFunc {
			errors = append(errors, "entry module must have a function named 'main'")
		}
	}

	// Validate functions
	for _, fn := range mod.Functions {
		if fn.ReturnType == nil {
			errors = append(errors, fmt.Sprintf("function %s has nil ReturnType", fn.Name))
		}
		errors = append(errors, validateContracts(fn.Requires, fmt.Sprintf("function %s requires", fn.Name))...)
		errors = append(errors, validateContracts(fn.Ensures, fmt.Sprintf("function %s ensures", fn.Name))...)
		errors = append(errors, validateStmts(fn.Body, fmt.Sprintf("function %s", fn.Name))...)
	}

	// Validate entities
	for _, ent := range mod.Entities {
		errors = append(errors, validateContracts(ent.Invariants, fmt.Sprintf("entity %s invariants", ent.Name))...)

		if ent.Constructor != nil {
			errors = append(errors, validateContracts(ent.Constructor.Requires, fmt.Sprintf("entity %s constructor requires", ent.Name))...)
			errors = append(errors, validateContracts(ent.Constructor.Ensures, fmt.Sprintf("entity %s constructor ensures", ent.Name))...)
			errors = append(errors, validateOldCaptures(ent.Constructor.OldCaptures, fmt.Sprintf("entity %s constructor", ent.Name))...)
			errors = append(errors, validateStmts(ent.Constructor.Body, fmt.Sprintf("entity %s constructor", ent.Name))...)
		}

		for _, method := range ent.Methods {
			if method.ReturnType == nil {
				errors = append(errors, fmt.Sprintf("entity %s method %s has nil ReturnType", ent.Name, method.Name))
			}
			errors = append(errors, validateContracts(method.Requires, fmt.Sprintf("entity %s method %s requires", ent.Name, method.Name))...)
			errors = append(errors, validateContracts(method.Ensures, fmt.Sprintf("entity %s method %s ensures", ent.Name, method.Name))...)
			errors = append(errors, validateOldCaptures(method.OldCaptures, fmt.Sprintf("entity %s method %s", ent.Name, method.Name))...)
			errors = append(errors, validateStmts(method.Body, fmt.Sprintf("entity %s method %s", ent.Name, method.Name))...)
		}
	}

	return errors
}

// validateContracts checks that all contracts have non-nil expressions.
func validateContracts(contracts []*Contract, context string) []string {
	var errors []string
	for i, c := range contracts {
		if c.Expr == nil {
			errors = append(errors, fmt.Sprintf("%s: contract %d has nil Expr", context, i))
		}
	}
	return errors
}

// validateOldCaptures checks that OldCapture names are unique.
func validateOldCaptures(captures []*OldCapture, context string) []string {
	var errors []string
	seen := make(map[string]bool)
	for _, cap := range captures {
		if seen[cap.Name] {
			errors = append(errors, fmt.Sprintf("%s: duplicate OldCapture name %q", context, cap.Name))
		}
		seen[cap.Name] = true
		if cap.Expr == nil {
			errors = append(errors, fmt.Sprintf("%s: OldCapture %q has nil Expr", context, cap.Name))
		}
	}
	return errors
}

// validateStmts checks statements for nil expressions and other invariants.
func validateStmts(stmts []Stmt, context string) []string {
	var errors []string
	for i, stmt := range stmts {
		errors = append(errors, validateStmt(stmt, fmt.Sprintf("%s statement %d", context, i))...)
	}
	return errors
}

// validateStmt checks a single statement.
func validateStmt(stmt Stmt, context string) []string {
	var errors []string

	switch s := stmt.(type) {
	case *LetStmt:
		if s.Type == nil {
			errors = append(errors, fmt.Sprintf("%s: LetStmt has nil Type", context))
		}
		if s.Value == nil {
			errors = append(errors, fmt.Sprintf("%s: LetStmt has nil Value", context))
		} else {
			errors = append(errors, validateExpr(s.Value, context)...)
		}

	case *AssignStmt:
		if s.Target == nil {
			errors = append(errors, fmt.Sprintf("%s: AssignStmt has nil Target", context))
		} else {
			errors = append(errors, validateExpr(s.Target, context)...)
		}
		if s.Value == nil {
			errors = append(errors, fmt.Sprintf("%s: AssignStmt has nil Value", context))
		} else {
			errors = append(errors, validateExpr(s.Value, context)...)
		}

	case *ReturnStmt:
		// ReturnStmt.Value can be nil for Void returns
		if s.Value != nil {
			errors = append(errors, validateExpr(s.Value, context)...)
		}

	case *IfStmt:
		if s.Condition == nil {
			errors = append(errors, fmt.Sprintf("%s: IfStmt has nil Condition", context))
		} else {
			errors = append(errors, validateExpr(s.Condition, context)...)
		}
		errors = append(errors, validateStmts(s.Then, fmt.Sprintf("%s (then)", context))...)
		errors = append(errors, validateStmts(s.Else, fmt.Sprintf("%s (else)", context))...)

	case *WhileStmt:
		if s.Condition == nil {
			errors = append(errors, fmt.Sprintf("%s: WhileStmt has nil Condition", context))
		} else {
			errors = append(errors, validateExpr(s.Condition, context)...)
		}
		errors = append(errors, validateContracts(s.Invariants, fmt.Sprintf("%s (while invariants)", context))...)
		errors = append(errors, validateOldCaptures(s.OldCaptures, fmt.Sprintf("%s (while)", context))...)
		if s.Decreases != nil && s.Decreases.Expr != nil {
			errors = append(errors, validateExpr(s.Decreases.Expr, context)...)
		}
		errors = append(errors, validateStmts(s.Body, fmt.Sprintf("%s (while body)", context))...)

	case *ForInStmt:
		if s.Iterable == nil {
			errors = append(errors, fmt.Sprintf("%s: ForInStmt has nil Iterable", context))
		} else {
			errors = append(errors, validateExpr(s.Iterable, context)...)
		}
		errors = append(errors, validateStmts(s.Body, fmt.Sprintf("%s (for-in body)", context))...)

	case *ExprStmt:
		if s.Expr == nil {
			errors = append(errors, fmt.Sprintf("%s: ExprStmt has nil Expr", context))
		} else {
			errors = append(errors, validateExpr(s.Expr, context)...)
		}

	case *BreakStmt, *ContinueStmt:
		// No validation needed

	default:
		errors = append(errors, fmt.Sprintf("%s: unknown statement type %T", context, stmt))
	}

	return errors
}

// validateExpr checks an expression for validity.
func validateExpr(expr Expr, context string) []string {
	var errors []string

	if expr == nil {
		errors = append(errors, fmt.Sprintf("%s: nil expression", context))
		return errors
	}

	switch e := expr.(type) {
	case *BinaryExpr:
		if e.Left == nil {
			errors = append(errors, fmt.Sprintf("%s: BinaryExpr has nil Left", context))
		} else {
			errors = append(errors, validateExpr(e.Left, context)...)
		}
		if e.Right == nil {
			errors = append(errors, fmt.Sprintf("%s: BinaryExpr has nil Right", context))
		} else {
			errors = append(errors, validateExpr(e.Right, context)...)
		}

	case *UnaryExpr:
		if e.Operand == nil {
			errors = append(errors, fmt.Sprintf("%s: UnaryExpr has nil Operand", context))
		} else {
			errors = append(errors, validateExpr(e.Operand, context)...)
		}

	case *CallExpr:
		if e.Kind == CallVariant && e.EnumName == "" {
			errors = append(errors, fmt.Sprintf("%s: CallExpr with CallVariant has empty EnumName", context))
		}
		for i, arg := range e.Args {
			if arg == nil {
				errors = append(errors, fmt.Sprintf("%s: CallExpr arg %d is nil", context, i))
			} else {
				errors = append(errors, validateExpr(arg, fmt.Sprintf("%s (arg %d)", context, i))...)
			}
		}

	case *MethodCallExpr:
		if e.Object == nil {
			errors = append(errors, fmt.Sprintf("%s: MethodCallExpr has nil Object", context))
		} else {
			errors = append(errors, validateExpr(e.Object, context)...)
		}
		for i, arg := range e.Args {
			if arg == nil {
				errors = append(errors, fmt.Sprintf("%s: MethodCallExpr arg %d is nil", context, i))
			} else {
				errors = append(errors, validateExpr(arg, fmt.Sprintf("%s (arg %d)", context, i))...)
			}
		}

	case *FieldAccessExpr:
		if e.Object == nil {
			errors = append(errors, fmt.Sprintf("%s: FieldAccessExpr has nil Object", context))
		} else {
			errors = append(errors, validateExpr(e.Object, context)...)
		}

	case *IndexExpr:
		if e.Object == nil {
			errors = append(errors, fmt.Sprintf("%s: IndexExpr has nil Object", context))
		} else {
			errors = append(errors, validateExpr(e.Object, context)...)
		}
		if e.Index == nil {
			errors = append(errors, fmt.Sprintf("%s: IndexExpr has nil Index", context))
		} else {
			errors = append(errors, validateExpr(e.Index, context)...)
		}

	case *ArrayLit:
		for i, elem := range e.Elements {
			if elem == nil {
				errors = append(errors, fmt.Sprintf("%s: ArrayLit element %d is nil", context, i))
			} else {
				errors = append(errors, validateExpr(elem, fmt.Sprintf("%s (element %d)", context, i))...)
			}
		}

	case *RangeExpr:
		if e.Start == nil {
			errors = append(errors, fmt.Sprintf("%s: RangeExpr has nil Start", context))
		} else {
			errors = append(errors, validateExpr(e.Start, context)...)
		}
		if e.End == nil {
			errors = append(errors, fmt.Sprintf("%s: RangeExpr has nil End", context))
		} else {
			errors = append(errors, validateExpr(e.End, context)...)
		}

	case *ForallExpr:
		if e.Domain == nil {
			errors = append(errors, fmt.Sprintf("%s: ForallExpr has nil Domain", context))
		} else {
			errors = append(errors, validateExpr(e.Domain, context)...)
		}
		if e.Body == nil {
			errors = append(errors, fmt.Sprintf("%s: ForallExpr has nil Body", context))
		} else {
			errors = append(errors, validateExpr(e.Body, context)...)
		}

	case *ExistsExpr:
		if e.Domain == nil {
			errors = append(errors, fmt.Sprintf("%s: ExistsExpr has nil Domain", context))
		} else {
			errors = append(errors, validateExpr(e.Domain, context)...)
		}
		if e.Body == nil {
			errors = append(errors, fmt.Sprintf("%s: ExistsExpr has nil Body", context))
		} else {
			errors = append(errors, validateExpr(e.Body, context)...)
		}

	case *MatchExpr:
		if e.Scrutinee == nil {
			errors = append(errors, fmt.Sprintf("%s: MatchExpr has nil Scrutinee", context))
		} else {
			errors = append(errors, validateExpr(e.Scrutinee, context)...)
		}
		for i, arm := range e.Arms {
			if arm.Body == nil {
				errors = append(errors, fmt.Sprintf("%s: MatchArm %d has nil Body", context, i))
			} else {
				errors = append(errors, validateExpr(arm.Body, fmt.Sprintf("%s (arm %d)", context, i))...)
			}
		}

	case *TryExpr:
		if e.Expr == nil {
			errors = append(errors, fmt.Sprintf("%s: TryExpr has nil Expr", context))
		} else {
			errors = append(errors, validateExpr(e.Expr, context)...)
		}

	case *StringConcat:
		if e.Left == nil {
			errors = append(errors, fmt.Sprintf("%s: StringConcat has nil Left", context))
		} else {
			errors = append(errors, validateExpr(e.Left, context)...)
		}
		if e.Right == nil {
			errors = append(errors, fmt.Sprintf("%s: StringConcat has nil Right", context))
		} else {
			errors = append(errors, validateExpr(e.Right, context)...)
		}

	case *OldRef, *VarRef, *SelfRef, *ResultRef, *IntLit, *FloatLit, *StringLit, *BoolLit:
		// No validation needed for leaf nodes

	default:
		errors = append(errors, fmt.Sprintf("%s: unknown expression type %T", context, expr))
	}

	return errors
}
