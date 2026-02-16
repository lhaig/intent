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

// exprToSMT converts an IR expression to SMT-LIB format
func exprToSMT(expr ir.Expr) string {
	switch e := expr.(type) {
	case *ir.BinaryExpr:
		return binaryExprToSMT(e)
	case *ir.UnaryExpr:
		return unaryExprToSMT(e)
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
