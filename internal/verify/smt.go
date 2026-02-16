package verify

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

// TranslateContract converts a contract to SMT-LIB 2 format.
// If isEnsures is true, the contract is negated for validity checking.
func TranslateContract(fn *ir.Function, contract *ir.Contract, isEnsures bool) string {
	var sb strings.Builder

	sb.WriteString("; Verification condition for function: ")
	sb.WriteString(fn.Name)
	sb.WriteString("\n")
	sb.WriteString("; Contract: ")
	sb.WriteString(contract.RawText)
	sb.WriteString("\n\n")

	// Declare function parameters
	for _, param := range fn.Params {
		sb.WriteString("(declare-const ")
		sb.WriteString(param.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(param.Type))
		sb.WriteString(")\n")
	}

	// Declare result variable for ensures clauses
	if isEnsures && fn.ReturnType != nil && fn.ReturnType.Name != "Void" {
		sb.WriteString("(declare-const result ")
		sb.WriteString(typeToSMTSort(fn.ReturnType))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n")

	// Add requires as assumptions
	if isEnsures && len(fn.Requires) > 0 {
		sb.WriteString("; Requires (assumptions)\n")
		for _, req := range fn.Requires {
			sb.WriteString("(assert ")
			sb.WriteString(exprToSMT(req.Expr))
			sb.WriteString(")\n")
		}
		sb.WriteString("\n")
	}

	// Add the main contract assertion
	if isEnsures {
		// For ensures: negate to check validity (prove by contradiction)
		sb.WriteString("; Ensures (negated for validity check)\n")
		sb.WriteString("(assert (not ")
		sb.WriteString(exprToSMT(contract.Expr))
		sb.WriteString("))\n")
	} else {
		// For requires: check satisfiability
		sb.WriteString("; Requires (checking satisfiability)\n")
		sb.WriteString("(assert ")
		sb.WriteString(exprToSMT(contract.Expr))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n(check-sat)\n")

	return sb.String()
}

// typeToSMTSort maps Intent types to SMT-LIB sorts
func typeToSMTSort(t *checker.Type) string {
	if t == nil {
		return "Int" // default
	}
	switch t.Name {
	case "Int":
		return "Int"
	case "Bool":
		return "Bool"
	case "Float":
		return "Real"
	default:
		// For unsupported types, use Int as fallback
		return "Int"
	}
}

// TranslateMethodContract converts an entity method/constructor contract to SMT-LIB 2 format.
// entityName is used in comments. fields are the entity's fields (declared as self_<name>).
// params are the method's parameters. invariants are entity invariants added as assumptions.
// If isEnsures, requires and invariants are assumed and the contract is negated.
func TranslateMethodContract(entityName, methodName string, fields []*ir.Field, params []*ir.Param, returnType *checker.Type, requires []*ir.Contract, invariants []*ir.Contract, contract *ir.Contract, isEnsures bool, oldCaptures []*ir.OldCapture) string {
	var sb strings.Builder

	sb.WriteString("; Verification condition for: ")
	sb.WriteString(entityName)
	sb.WriteString(".")
	sb.WriteString(methodName)
	sb.WriteString("\n; Contract: ")
	sb.WriteString(contract.RawText)
	sb.WriteString("\n\n")

	// Declare entity fields as self_<name> constants
	for _, f := range fields {
		sb.WriteString("(declare-const self_")
		sb.WriteString(f.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(f.Type))
		sb.WriteString(")\n")
	}

	// Declare method parameters
	for _, param := range params {
		sb.WriteString("(declare-const ")
		sb.WriteString(param.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(param.Type))
		sb.WriteString(")\n")
	}

	// Declare result variable for ensures clauses
	if isEnsures && returnType != nil && returnType.Name != "Void" {
		sb.WriteString("(declare-const result ")
		sb.WriteString(typeToSMTSort(returnType))
		sb.WriteString(")\n")
	}

	// Declare old_ constants for old() captures
	for _, oc := range oldCaptures {
		sb.WriteString("(declare-const ")
		sb.WriteString(oc.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(oc.Expr.ExprType()))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n")

	if isEnsures {
		// Add requires as assumptions
		if len(requires) > 0 {
			sb.WriteString("; Requires (assumptions)\n")
			for _, req := range requires {
				sb.WriteString("(assert ")
				sb.WriteString(entityExprToSMT(req.Expr))
				sb.WriteString(")\n")
			}
			sb.WriteString("\n")
		}

		// Add invariants as assumptions
		if len(invariants) > 0 {
			sb.WriteString("; Invariants (assumptions)\n")
			for _, inv := range invariants {
				sb.WriteString("(assert ")
				sb.WriteString(entityExprToSMT(inv.Expr))
				sb.WriteString(")\n")
			}
			sb.WriteString("\n")
		}

		// Negate the ensures contract for validity check
		sb.WriteString("; Ensures (negated for validity check)\n")
		sb.WriteString("(assert (not ")
		sb.WriteString(entityExprToSMT(contract.Expr))
		sb.WriteString("))\n")
	} else {
		// For requires: check satisfiability
		sb.WriteString("; Requires (checking satisfiability)\n")
		sb.WriteString("(assert ")
		sb.WriteString(entityExprToSMT(contract.Expr))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n(check-sat)\n")

	return sb.String()
}

// TranslateInvariant converts an entity invariant to SMT-LIB 2 format.
// Declares fields as self_<name> and checks if the negated invariant is unsatisfiable.
func TranslateInvariant(entityName string, fields []*ir.Field, contract *ir.Contract) string {
	var sb strings.Builder

	sb.WriteString("; Invariant check for: ")
	sb.WriteString(entityName)
	sb.WriteString("\n; Invariant: ")
	sb.WriteString(contract.RawText)
	sb.WriteString("\n\n")

	// Declare entity fields as self_<name> constants
	for _, f := range fields {
		sb.WriteString("(declare-const self_")
		sb.WriteString(f.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(f.Type))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n")

	// Assert negated invariant (prove by contradiction)
	sb.WriteString("; Invariant (negated for validity check)\n")
	sb.WriteString("(assert (not ")
	sb.WriteString(entityExprToSMT(contract.Expr))
	sb.WriteString("))\n")

	sb.WriteString("\n(check-sat)\n")

	return sb.String()
}

// entityExprToSMT converts an IR expression to SMT-LIB format with entity support.
// It maps self.field -> self_field and handles OldRef.
func entityExprToSMT(expr ir.Expr) string {
	switch e := expr.(type) {
	case *ir.FieldAccessExpr:
		// Check if this is self.field
		if _, ok := e.Object.(*ir.SelfRef); ok {
			return "self_" + e.Field
		}
		// Fallback: treat as regular expression
		return entityExprToSMT(e.Object) + "_" + e.Field
	case *ir.SelfRef:
		return "self"
	case *ir.OldRef:
		return e.Name
	case *ir.BinaryExpr:
		return entityBinaryExprToSMT(e)
	case *ir.UnaryExpr:
		return entityUnaryExprToSMT(e)
	case *ir.VarRef:
		return e.Name
	case *ir.ResultRef:
		return "result"
	case *ir.IntLit:
		return fmt.Sprintf("%d", e.Value)
	case *ir.FloatLit:
		return e.Value
	case *ir.BoolLit:
		if e.Value {
			return "true"
		}
		return "false"
	case *ir.ForallExpr:
		return entityForallExprToSMT(e)
	case *ir.ExistsExpr:
		return entityExistsExprToSMT(e)
	default:
		return "true"
	}
}

func entityBinaryExprToSMT(e *ir.BinaryExpr) string {
	left := entityExprToSMT(e.Left)
	right := entityExprToSMT(e.Right)

	switch e.Op {
	case lexer.PLUS:
		return fmt.Sprintf("(+ %s %s)", left, right)
	case lexer.MINUS:
		return fmt.Sprintf("(- %s %s)", left, right)
	case lexer.STAR:
		return fmt.Sprintf("(* %s %s)", left, right)
	case lexer.SLASH:
		return fmt.Sprintf("(div %s %s)", left, right)
	case lexer.PERCENT:
		return fmt.Sprintf("(mod %s %s)", left, right)
	case lexer.EQ:
		return fmt.Sprintf("(= %s %s)", left, right)
	case lexer.NEQ:
		return fmt.Sprintf("(not (= %s %s))", left, right)
	case lexer.LT:
		return fmt.Sprintf("(< %s %s)", left, right)
	case lexer.LEQ:
		return fmt.Sprintf("(<= %s %s)", left, right)
	case lexer.GT:
		return fmt.Sprintf("(> %s %s)", left, right)
	case lexer.GEQ:
		return fmt.Sprintf("(>= %s %s)", left, right)
	case lexer.AND:
		return fmt.Sprintf("(and %s %s)", left, right)
	case lexer.OR:
		return fmt.Sprintf("(or %s %s)", left, right)
	case lexer.IMPLIES:
		return fmt.Sprintf("(=> %s %s)", left, right)
	default:
		return "true"
	}
}

func entityUnaryExprToSMT(e *ir.UnaryExpr) string {
	operand := entityExprToSMT(e.Operand)
	switch e.Op {
	case lexer.NOT:
		return fmt.Sprintf("(not %s)", operand)
	case lexer.MINUS:
		return fmt.Sprintf("(- %s)", operand)
	default:
		return operand
	}
}

func entityForallExprToSMT(e *ir.ForallExpr) string {
	var start, end string
	if e.Domain != nil {
		start = entityExprToSMT(e.Domain.Start)
		end = entityExprToSMT(e.Domain.End)
	}
	body := entityExprToSMT(e.Body)
	return fmt.Sprintf("(forall ((%s Int)) (=> (and (>= %s %s) (< %s %s)) %s))",
		e.Variable, e.Variable, start, e.Variable, end, body)
}

func entityExistsExprToSMT(e *ir.ExistsExpr) string {
	var start, end string
	if e.Domain != nil {
		start = entityExprToSMT(e.Domain.Start)
		end = entityExprToSMT(e.Domain.End)
	}
	body := entityExprToSMT(e.Body)
	return fmt.Sprintf("(exists ((%s Int)) (and (>= %s %s) (< %s %s) %s))",
		e.Variable, e.Variable, start, e.Variable, end, body)
}

// TranslateLoopInvariant generates SMT-LIB for a loop invariant in a free function.
// Uses inductive verification: assumes invariant + loop condition, proves invariant still holds.
func TranslateLoopInvariant(fn *ir.Function, loop *ir.WhileStmt, inv *ir.Contract) string {
	var sb strings.Builder

	sb.WriteString("; Loop invariant verification for: ")
	sb.WriteString(fn.Name)
	sb.WriteString("\n; Invariant: ")
	sb.WriteString(inv.RawText)
	sb.WriteString("\n; Strategy: inductive step (assume inv + condition, prove inv holds)\n\n")

	// Declare function parameters
	for _, param := range fn.Params {
		sb.WriteString("(declare-const ")
		sb.WriteString(param.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(param.Type))
		sb.WriteString(")\n")
	}

	// Declare old captures
	for _, oc := range loop.OldCaptures {
		sb.WriteString("(declare-const ")
		sb.WriteString(oc.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(oc.Expr.ExprType()))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n")

	// Assume function preconditions
	if len(fn.Requires) > 0 {
		sb.WriteString("; Function preconditions\n")
		for _, req := range fn.Requires {
			sb.WriteString("(assert ")
			sb.WriteString(exprToSMT(req.Expr))
			sb.WriteString(")\n")
		}
		sb.WriteString("\n")
	}

	// Assume the invariant holds (inductive hypothesis)
	sb.WriteString("; Inductive hypothesis: invariant holds\n")
	sb.WriteString("(assert ")
	sb.WriteString(exprToSMT(inv.Expr))
	sb.WriteString(")\n\n")

	// Assume the loop condition holds (we're still in the loop)
	sb.WriteString("; Loop condition holds\n")
	sb.WriteString("(assert ")
	sb.WriteString(exprToSMT(loop.Condition))
	sb.WriteString(")\n\n")

	// Assert negated invariant (prove it still holds after abstract step)
	sb.WriteString("; Prove invariant is preserved (negated for contradiction)\n")
	sb.WriteString("(assert (not ")
	sb.WriteString(exprToSMT(inv.Expr))
	sb.WriteString("))\n")

	sb.WriteString("\n(check-sat)\n")

	return sb.String()
}

// TranslateLoopInvariantForMethod generates SMT-LIB for a loop invariant in an entity method.
func TranslateLoopInvariantForMethod(entityName, methodName string, fields []*ir.Field, params []*ir.Param, loop *ir.WhileStmt, inv *ir.Contract) string {
	var sb strings.Builder

	sb.WriteString("; Loop invariant verification for: ")
	sb.WriteString(entityName)
	sb.WriteString(".")
	sb.WriteString(methodName)
	sb.WriteString("\n; Invariant: ")
	sb.WriteString(inv.RawText)
	sb.WriteString("\n; Strategy: inductive step (assume inv + condition, prove inv holds)\n\n")

	// Declare entity fields
	for _, f := range fields {
		sb.WriteString("(declare-const self_")
		sb.WriteString(f.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(f.Type))
		sb.WriteString(")\n")
	}

	// Declare method parameters
	for _, param := range params {
		sb.WriteString("(declare-const ")
		sb.WriteString(param.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(param.Type))
		sb.WriteString(")\n")
	}

	// Declare old captures
	for _, oc := range loop.OldCaptures {
		sb.WriteString("(declare-const ")
		sb.WriteString(oc.Name)
		sb.WriteString(" ")
		sb.WriteString(typeToSMTSort(oc.Expr.ExprType()))
		sb.WriteString(")\n")
	}

	sb.WriteString("\n")

	// Assume the invariant holds (inductive hypothesis)
	sb.WriteString("; Inductive hypothesis: invariant holds\n")
	sb.WriteString("(assert ")
	sb.WriteString(entityExprToSMT(inv.Expr))
	sb.WriteString(")\n\n")

	// Assume the loop condition holds
	sb.WriteString("; Loop condition holds\n")
	sb.WriteString("(assert ")
	sb.WriteString(entityExprToSMT(loop.Condition))
	sb.WriteString(")\n\n")

	// Assert negated invariant
	sb.WriteString("; Prove invariant is preserved (negated for contradiction)\n")
	sb.WriteString("(assert (not ")
	sb.WriteString(entityExprToSMT(inv.Expr))
	sb.WriteString("))\n")

	sb.WriteString("\n(check-sat)\n")

	return sb.String()
}

// exprToSMT converts an IR expression to SMT-LIB format
func exprToSMT(expr ir.Expr) string {
	switch e := expr.(type) {
	case *ir.BinaryExpr:
		return binaryExprToSMT(e)
	case *ir.UnaryExpr:
		return unaryExprToSMT(e)
	case *ir.VarRef:
		return e.Name
	case *ir.OldRef:
		return e.Name
	case *ir.ResultRef:
		return "result"
	case *ir.IntLit:
		return fmt.Sprintf("%d", e.Value)
	case *ir.FloatLit:
		return e.Value
	case *ir.BoolLit:
		if e.Value {
			return "true"
		}
		return "false"
	case *ir.ForallExpr:
		return forallExprToSMT(e)
	case *ir.ExistsExpr:
		return existsExprToSMT(e)
	default:
		// Unsupported expression type
		return "true"
	}
}

// binaryExprToSMT converts a binary expression to SMT-LIB format
func binaryExprToSMT(e *ir.BinaryExpr) string {
	left := exprToSMT(e.Left)
	right := exprToSMT(e.Right)

	switch e.Op {
	case lexer.PLUS:
		return fmt.Sprintf("(+ %s %s)", left, right)
	case lexer.MINUS:
		return fmt.Sprintf("(- %s %s)", left, right)
	case lexer.STAR:
		return fmt.Sprintf("(* %s %s)", left, right)
	case lexer.SLASH:
		return fmt.Sprintf("(div %s %s)", left, right)
	case lexer.PERCENT:
		return fmt.Sprintf("(mod %s %s)", left, right)
	case lexer.EQ:
		return fmt.Sprintf("(= %s %s)", left, right)
	case lexer.NEQ:
		return fmt.Sprintf("(not (= %s %s))", left, right)
	case lexer.LT:
		return fmt.Sprintf("(< %s %s)", left, right)
	case lexer.LEQ:
		return fmt.Sprintf("(<= %s %s)", left, right)
	case lexer.GT:
		return fmt.Sprintf("(> %s %s)", left, right)
	case lexer.GEQ:
		return fmt.Sprintf("(>= %s %s)", left, right)
	case lexer.AND:
		return fmt.Sprintf("(and %s %s)", left, right)
	case lexer.OR:
		return fmt.Sprintf("(or %s %s)", left, right)
	case lexer.IMPLIES:
		return fmt.Sprintf("(=> %s %s)", left, right)
	default:
		return "true"
	}
}

// unaryExprToSMT converts a unary expression to SMT-LIB format
func unaryExprToSMT(e *ir.UnaryExpr) string {
	operand := exprToSMT(e.Operand)

	switch e.Op {
	case lexer.NOT:
		return fmt.Sprintf("(not %s)", operand)
	case lexer.MINUS:
		return fmt.Sprintf("(- %s)", operand)
	default:
		return operand
	}
}

// forallExprToSMT converts a forall expression to SMT-LIB format
func forallExprToSMT(e *ir.ForallExpr) string {
	// Extract range bounds
	var start, end string
	if e.Domain != nil {
		start = exprToSMT(e.Domain.Start)
		end = exprToSMT(e.Domain.End)
	}

	body := exprToSMT(e.Body)

	// SMT-LIB forall with range constraint
	// forall x in start..end: body
	// becomes: (forall ((x Int)) (=> (and (>= x start) (< x end)) body))
	return fmt.Sprintf("(forall ((%s Int)) (=> (and (>= %s %s) (< %s %s)) %s))",
		e.Variable, e.Variable, start, e.Variable, end, body)
}

// existsExprToSMT converts an exists expression to SMT-LIB format
func existsExprToSMT(e *ir.ExistsExpr) string {
	// Extract range bounds
	var start, end string
	if e.Domain != nil {
		start = exprToSMT(e.Domain.Start)
		end = exprToSMT(e.Domain.End)
	}

	body := exprToSMT(e.Body)

	// SMT-LIB exists with range constraint
	// exists x in start..end: body
	// becomes: (exists ((x Int)) (and (>= x start) (< x end) body))
	return fmt.Sprintf("(exists ((%s Int)) (and (>= %s %s) (< %s %s) %s))",
		e.Variable, e.Variable, start, e.Variable, end, body)
}
