package verify

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

func TestTranslateSimpleRequires(t *testing.T) {
	// Create a simple function with a requires clause
	fn := &ir.Function{
		Name: "test",
		Params: []*ir.Param{
			{Name: "x", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
	}

	contract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left:  &ir.VarRef{Name: "x", Type: checker.TypeInt},
			Op:    lexer.GT,
			Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		RawText: "x > 0",
	}

	smtLib := TranslateContract(fn, contract, false)

	// Check that SMT-LIB output contains expected elements
	if !strings.Contains(smtLib, "(declare-const x Int)") {
		t.Errorf("Expected parameter declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(> x 0)") {
		t.Errorf("Expected comparison, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(check-sat)") {
		t.Errorf("Expected check-sat, got: %s", smtLib)
	}
}

func TestTranslateEnsures(t *testing.T) {
	// Create a function with an ensures clause
	fn := &ir.Function{
		Name: "abs",
		Params: []*ir.Param{
			{Name: "x", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
	}

	contract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left:  &ir.ResultRef{Type: checker.TypeInt},
			Op:    lexer.GEQ,
			Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		RawText: "result >= 0",
	}

	smtLib := TranslateContract(fn, contract, true)

	// Check that SMT-LIB output contains expected elements
	if !strings.Contains(smtLib, "(declare-const result Int)") {
		t.Errorf("Expected result declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (not (>= result 0)))") {
		t.Errorf("Expected negated ensures, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(check-sat)") {
		t.Errorf("Expected check-sat, got: %s", smtLib)
	}
}

func TestTranslateWithQuantifier(t *testing.T) {
	// Create a function with a forall quantifier
	fn := &ir.Function{
		Name: "test",
		Params: []*ir.Param{
			{Name: "n", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeBool,
	}

	contract := &ir.Contract{
		Expr: &ir.ForallExpr{
			Variable: "i",
			Domain: &ir.RangeExpr{
				Start: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				End:   &ir.VarRef{Name: "n", Type: checker.TypeInt},
				Type:  checker.TypeInt,
			},
			Body: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "i", Type: checker.TypeInt},
				Op:    lexer.GEQ,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Type: checker.TypeBool,
		},
		RawText: "forall i in 0..n: i >= 0",
	}

	smtLib := TranslateContract(fn, contract, false)

	// Check that SMT-LIB output contains forall
	if !strings.Contains(smtLib, "(forall ((i Int))") {
		t.Errorf("Expected forall quantifier, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(>= i 0)") {
		t.Errorf("Expected quantifier body, got: %s", smtLib)
	}
}

func TestTranslateExistsQuantifier(t *testing.T) {
	// Create a function with an exists quantifier
	fn := &ir.Function{
		Name: "test",
		Params: []*ir.Param{
			{Name: "n", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeBool,
	}

	contract := &ir.Contract{
		Expr: &ir.ExistsExpr{
			Variable: "i",
			Domain: &ir.RangeExpr{
				Start: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				End:   &ir.VarRef{Name: "n", Type: checker.TypeInt},
				Type:  checker.TypeInt,
			},
			Body: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "i", Type: checker.TypeInt},
				Op:    lexer.EQ,
				Right: &ir.IntLit{Value: 5, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Type: checker.TypeBool,
		},
		RawText: "exists i in 0..n: i == 5",
	}

	smtLib := TranslateContract(fn, contract, false)

	// Check that SMT-LIB output contains exists
	if !strings.Contains(smtLib, "(exists ((i Int))") {
		t.Errorf("Expected exists quantifier, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(= i 5)") {
		t.Errorf("Expected quantifier body, got: %s", smtLib)
	}
}

func TestTranslateLogicalOperations(t *testing.T) {
	// Create a function with logical operations
	fn := &ir.Function{
		Name: "test",
		Params: []*ir.Param{
			{Name: "x", Type: checker.TypeInt},
			{Name: "y", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeBool,
	}

	// Test AND
	andContract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "x", Type: checker.TypeInt},
				Op:    lexer.GT,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Op: lexer.AND,
			Right: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "y", Type: checker.TypeInt},
				Op:    lexer.GT,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Type: checker.TypeBool,
		},
		RawText: "x > 0 and y > 0",
	}

	smtLib := TranslateContract(fn, andContract, false)
	if !strings.Contains(smtLib, "(and (> x 0) (> y 0))") {
		t.Errorf("Expected AND operation, got: %s", smtLib)
	}

	// Test OR
	orContract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "x", Type: checker.TypeInt},
				Op:    lexer.EQ,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Op: lexer.OR,
			Right: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "y", Type: checker.TypeInt},
				Op:    lexer.EQ,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Type: checker.TypeBool,
		},
		RawText: "x == 0 or y == 0",
	}

	smtLib = TranslateContract(fn, orContract, false)
	if !strings.Contains(smtLib, "(or (= x 0) (= y 0))") {
		t.Errorf("Expected OR operation, got: %s", smtLib)
	}

	// Test NOT
	notContract := &ir.Contract{
		Expr: &ir.UnaryExpr{
			Op: lexer.NOT,
			Operand: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "x", Type: checker.TypeInt},
				Op:    lexer.EQ,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			Type: checker.TypeBool,
		},
		RawText: "not x == 0",
	}

	smtLib = TranslateContract(fn, notContract, false)
	if !strings.Contains(smtLib, "(not (= x 0))") {
		t.Errorf("Expected NOT operation, got: %s", smtLib)
	}
}

func TestVerifySimple(t *testing.T) {
	// Skip if z3 is not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found on PATH, skipping integration test")
	}

	// Create a simple function with a trivial ensures clause
	fn := &ir.Function{
		Name: "identity",
		Params: []*ir.Param{
			{Name: "x", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
		Ensures: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left:  &ir.ResultRef{Type: checker.TypeInt},
					Op:    lexer.EQ,
					Right: &ir.VarRef{Name: "x", Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "result == x",
			},
		},
	}

	results := VerifyFunction(fn)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Status == "error" {
		t.Errorf("Verification failed with error: %s", result.Message)
	}

	// Note: This is a simple identity check, z3 might return "sat" (counterexample)
	// since we're checking if result == x is falsifiable, which it is without
	// knowing the function body. This is expected for a basic test.
	if result.Status != "verified" && result.Status != "unverified" {
		t.Errorf("Expected verified or unverified status, got: %s (%s)", result.Status, result.Message)
	}
}

func TestVerifyWithRequires(t *testing.T) {
	// Skip if z3 is not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found on PATH, skipping integration test")
	}

	// Create a function with requires and ensures
	fn := &ir.Function{
		Name: "abs",
		Params: []*ir.Param{
			{Name: "x", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
		Requires: []*ir.Contract{
			{
				Expr: &ir.BoolLit{Value: true, Type: checker.TypeBool},
				RawText: "true",
			},
		},
		Ensures: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left:  &ir.ResultRef{Type: checker.TypeInt},
					Op:    lexer.GEQ,
					Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "result >= 0",
			},
		},
	}

	results := VerifyFunction(fn)

	// Should have results for both requires and ensures
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check that we got results for both
	hasRequires := false
	hasEnsures := false
	for _, r := range results {
		if !r.IsEnsures {
			hasRequires = true
		} else {
			hasEnsures = true
		}
		if r.Status == "error" {
			t.Errorf("Verification failed with error: %s", r.Message)
		}
	}

	if !hasRequires || !hasEnsures {
		t.Errorf("Expected results for both requires and ensures")
	}
}
