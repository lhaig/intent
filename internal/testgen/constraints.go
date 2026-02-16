package testgen

import (
	"strconv"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/lexer"
)

// ParamConstraint holds extracted bounds for a single parameter.
type ParamConstraint struct {
	Name      string
	TypeName  string // "Int", "Float", "String", "Bool", "Array"
	ElemType  string // inner type for Array<T>
	Lower     *int64 // inclusive lower bound (nil = unbounded)
	Upper     *int64 // inclusive upper bound (nil = unbounded)
	NotEqual  []int64
	MinLen    *int64 // min array length from len(arr) > N
	ElemLower *int64 // element lower bound from forall
	ElemUpper *int64 // element upper bound from forall
}

// AnalyzeConstraints extracts parameter constraints from requires clauses.
func AnalyzeConstraints(params []*ast.Param, requires []*ast.ContractClause) map[string]*ParamConstraint {
	constraints := make(map[string]*ParamConstraint)

	// Initialize constraints from parameter declarations
	for _, p := range params {
		c := &ParamConstraint{
			Name:     p.Name,
			TypeName: p.Type.Name,
		}
		if p.Type.Name == "Array" && len(p.Type.TypeArgs) > 0 {
			c.ElemType = p.Type.TypeArgs[0].Name
		}
		constraints[p.Name] = c
	}

	// Walk each requires clause
	for _, req := range requires {
		extractFromExpr(req.Expr, constraints)
	}

	return constraints
}

// extractFromExpr recursively extracts constraints from a single expression.
func extractFromExpr(expr ast.Expression, constraints map[string]*ParamConstraint) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == lexer.AND {
			// Recurse both sides of "and"
			extractFromExpr(e.Left, constraints)
			extractFromExpr(e.Right, constraints)
			return
		}
		// Try: ident >= intLit, ident > intLit, ident <= intLit, etc.
		extractComparison(e, constraints)

	case *ast.ForallExpr:
		extractForall(e, constraints)

	case *ast.BoolLit:
		// Skip: no constraint from "true"

	default:
		// Conservative: skip anything we don't understand
	}
}

// extractComparison handles binary comparisons like `n >= 0`, `len(arr) > 0`, `n != 5`.
func extractComparison(e *ast.BinaryExpr, constraints map[string]*ParamConstraint) {
	// Pattern: ident OP intLit
	if ident, ok := e.Left.(*ast.Identifier); ok {
		if intLit, ok := e.Right.(*ast.IntLit); ok {
			val, err := strconv.ParseInt(intLit.Value, 10, 64)
			if err != nil {
				return
			}
			c, exists := constraints[ident.Name]
			if !exists {
				return
			}
			applyBound(c, e.Op, val)
			return
		}
	}

	// Pattern: intLit OP ident (reversed)
	if intLit, ok := e.Left.(*ast.IntLit); ok {
		if ident, ok := e.Right.(*ast.Identifier); ok {
			val, err := strconv.ParseInt(intLit.Value, 10, 64)
			if err != nil {
				return
			}
			c, exists := constraints[ident.Name]
			if !exists {
				return
			}
			// Reverse the operator: 5 < n means n > 5
			applyBound(c, reverseOp(e.Op), val)
			return
		}
	}

	// Pattern: len(ident) OP intLit (array length constraint)
	if call, ok := e.Left.(*ast.CallExpr); ok {
		if call.Function == "len" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Identifier); ok {
				if intLit, ok := e.Right.(*ast.IntLit); ok {
					val, err := strconv.ParseInt(intLit.Value, 10, 64)
					if err != nil {
						return
					}
					c, exists := constraints[ident.Name]
					if !exists {
						return
					}
					applyLenBound(c, e.Op, val)
					return
				}
			}
		}
	}
}

// applyBound applies a comparison bound to a constraint.
func applyBound(c *ParamConstraint, op lexer.TokenType, val int64) {
	switch op {
	case lexer.GEQ: // ident >= val
		c.Lower = int64Ptr(val)
	case lexer.GT: // ident > val
		c.Lower = int64Ptr(val + 1)
	case lexer.LEQ: // ident <= val
		c.Upper = int64Ptr(val)
	case lexer.LT: // ident < val
		c.Upper = int64Ptr(val - 1)
	case lexer.NEQ: // ident != val
		c.NotEqual = append(c.NotEqual, val)
	case lexer.EQ: // ident == val (both bounds)
		c.Lower = int64Ptr(val)
		c.Upper = int64Ptr(val)
	}
}

// applyLenBound applies a len() comparison to a constraint.
func applyLenBound(c *ParamConstraint, op lexer.TokenType, val int64) {
	switch op {
	case lexer.GT: // len(arr) > val
		c.MinLen = int64Ptr(val + 1)
	case lexer.GEQ: // len(arr) >= val
		c.MinLen = int64Ptr(val)
	case lexer.EQ: // len(arr) == val
		c.MinLen = int64Ptr(val)
	}
}

// reverseOp flips a comparison operator (for "intLit OP ident" -> "ident reverseOP intLit").
func reverseOp(op lexer.TokenType) lexer.TokenType {
	switch op {
	case lexer.GT:
		return lexer.LT
	case lexer.LT:
		return lexer.GT
	case lexer.GEQ:
		return lexer.LEQ
	case lexer.LEQ:
		return lexer.GEQ
	default:
		return op // ==, != are symmetric
	}
}

// extractForall handles: forall i in 0..len(arr): body
// Extracts element bounds from the body predicate.
func extractForall(f *ast.ForallExpr, constraints map[string]*ParamConstraint) {
	// Check the domain is 0..len(arrIdent)
	if f.Domain == nil {
		return
	}

	// Domain start should be 0
	startLit, ok := f.Domain.Start.(*ast.IntLit)
	if !ok || startLit.Value != "0" {
		return
	}

	// Domain end should be len(arrIdent)
	endCall, ok := f.Domain.End.(*ast.CallExpr)
	if !ok || endCall.Function != "len" || len(endCall.Args) != 1 {
		return
	}

	arrIdent, ok := endCall.Args[0].(*ast.Identifier)
	if !ok {
		return
	}

	c, exists := constraints[arrIdent.Name]
	if !exists || c.TypeName != "Array" {
		return
	}

	// Now analyze the body for element bounds: arr[i] > 0, arr[i] >= 0, etc.
	extractElementBounds(f.Body, f.Variable, c)
}

// extractElementBounds walks the body of a forall to find element comparisons.
func extractElementBounds(body ast.Expression, loopVar string, c *ParamConstraint) {
	switch e := body.(type) {
	case *ast.BinaryExpr:
		if e.Op == lexer.AND {
			extractElementBounds(e.Left, loopVar, c)
			extractElementBounds(e.Right, loopVar, c)
			return
		}
		// Pattern: arr[loopVar] OP intLit
		if idx, ok := e.Left.(*ast.IndexExpr); ok {
			if ident, ok := idx.Index.(*ast.Identifier); ok && ident.Name == loopVar {
				if intLit, ok := e.Right.(*ast.IntLit); ok {
					val, err := strconv.ParseInt(intLit.Value, 10, 64)
					if err != nil {
						return
					}
					applyElementBound(c, e.Op, val)
				}
			}
		}
	}
}

// applyElementBound applies element-level bounds.
func applyElementBound(c *ParamConstraint, op lexer.TokenType, val int64) {
	switch op {
	case lexer.GT: // arr[i] > val
		c.ElemLower = int64Ptr(val + 1)
	case lexer.GEQ: // arr[i] >= val
		c.ElemLower = int64Ptr(val)
	case lexer.LT: // arr[i] < val
		c.ElemUpper = int64Ptr(val - 1)
	case lexer.LEQ: // arr[i] <= val
		c.ElemUpper = int64Ptr(val)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
