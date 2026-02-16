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

func TestTranslateMethodContract(t *testing.T) {
	// Test self.field -> self_field mapping in method contracts
	fields := []*ir.Field{
		{Name: "balance", Type: checker.TypeInt},
		{Name: "owner", Type: checker.TypeString},
	}
	params := []*ir.Param{
		{Name: "amount", Type: checker.TypeInt},
	}
	contract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left: &ir.FieldAccessExpr{
				Object: &ir.SelfRef{Type: checker.TypeInt},
				Field:  "balance",
				Type:   checker.TypeInt,
			},
			Op:    lexer.GEQ,
			Right: &ir.VarRef{Name: "amount", Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		RawText: "self.balance >= amount",
	}

	smtLib := TranslateMethodContract("BankAccount", "withdraw", fields, params, checker.TypeBool, nil, nil, contract, false, nil)

	if !strings.Contains(smtLib, "(declare-const self_balance Int)") {
		t.Errorf("Expected self_balance declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(declare-const amount Int)") {
		t.Errorf("Expected amount declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(>= self_balance amount)") {
		t.Errorf("Expected self_balance >= amount in SMT, got: %s", smtLib)
	}
}

func TestTranslateMethodEnsuresWithOld(t *testing.T) {
	// Test old() references in ensures clauses
	fields := []*ir.Field{
		{Name: "balance", Type: checker.TypeInt},
	}
	params := []*ir.Param{
		{Name: "amount", Type: checker.TypeInt},
	}
	requires := []*ir.Contract{
		{
			Expr: &ir.BinaryExpr{
				Left:  &ir.VarRef{Name: "amount", Type: checker.TypeInt},
				Op:    lexer.GT,
				Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
				Type:  checker.TypeBool,
			},
			RawText: "amount > 0",
		},
	}
	oldCaptures := []*ir.OldCapture{
		{Name: "__old_self_balance", Expr: &ir.FieldAccessExpr{
			Object: &ir.SelfRef{Type: checker.TypeInt},
			Field:  "balance",
			Type:   checker.TypeInt,
		}},
	}
	contract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left: &ir.FieldAccessExpr{
				Object: &ir.SelfRef{Type: checker.TypeInt},
				Field:  "balance",
				Type:   checker.TypeInt,
			},
			Op: lexer.EQ,
			Right: &ir.BinaryExpr{
				Left:  &ir.OldRef{Name: "__old_self_balance", Type: checker.TypeInt},
				Op:    lexer.PLUS,
				Right: &ir.VarRef{Name: "amount", Type: checker.TypeInt},
				Type:  checker.TypeInt,
			},
			Type: checker.TypeBool,
		},
		RawText: "self.balance == old(self.balance) + amount",
	}

	smtLib := TranslateMethodContract("BankAccount", "deposit", fields, params, nil, requires, nil, contract, true, oldCaptures)

	if !strings.Contains(smtLib, "(declare-const __old_self_balance Int)") {
		t.Errorf("Expected old capture declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (> amount 0))") {
		t.Errorf("Expected requires assumption, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (not (= self_balance (+ __old_self_balance amount))))") {
		t.Errorf("Expected negated ensures with old ref, got: %s", smtLib)
	}
}

func TestTranslateInvariant(t *testing.T) {
	fields := []*ir.Field{
		{Name: "balance", Type: checker.TypeInt},
	}
	contract := &ir.Contract{
		Expr: &ir.BinaryExpr{
			Left: &ir.FieldAccessExpr{
				Object: &ir.SelfRef{Type: checker.TypeInt},
				Field:  "balance",
				Type:   checker.TypeInt,
			},
			Op:    lexer.GEQ,
			Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		RawText: "self.balance >= 0",
	}

	smtLib := TranslateInvariant("BankAccount", fields, contract)

	if !strings.Contains(smtLib, "(declare-const self_balance Int)") {
		t.Errorf("Expected self_balance declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (not (>= self_balance 0)))") {
		t.Errorf("Expected negated invariant, got: %s", smtLib)
	}
}

func TestBuildIntentReports(t *testing.T) {
	mod := &ir.Module{
		Intents: []*ir.Intent{
			{
				Description: "Test intent",
				VerifiedBy: [][]string{
					{"MyEntity", "invariant"},
					{"MyEntity", "withdraw", "requires"},
					{"nonexistent", "ensures"},
				},
			},
		},
	}

	results := []*VerifyResult{
		{
			EntityName:   "MyEntity",
			FunctionName: "",
			ContractKind: "invariant",
			ContractText: "self.x >= 0",
			Status:       "verified",
		},
		{
			EntityName:   "MyEntity",
			FunctionName: "withdraw",
			ContractKind: "requires",
			ContractText: "amount > 0",
			Status:       "unverified",
			Message:      "counterexample found",
		},
	}

	reports := BuildIntentReports(mod, results)

	if len(reports) != 1 {
		t.Fatalf("Expected 1 report, got %d", len(reports))
	}

	report := reports[0]
	if report.Description != "Test intent" {
		t.Errorf("Expected description 'Test intent', got %q", report.Description)
	}
	if len(report.Refs) != 3 {
		t.Fatalf("Expected 3 refs, got %d", len(report.Refs))
	}

	// First ref: verified
	if report.Refs[0].Status != "verified" {
		t.Errorf("Expected first ref verified, got %s", report.Refs[0].Status)
	}
	// Second ref: unverified
	if report.Refs[1].Status != "unverified" {
		t.Errorf("Expected second ref unverified, got %s", report.Refs[1].Status)
	}
	// Third ref: not found
	if report.Refs[2].Status != "not_found" {
		t.Errorf("Expected third ref not_found, got %s", report.Refs[2].Status)
	}

	if report.AllVerified() {
		t.Errorf("Expected AllVerified() to be false")
	}
}

func TestFormatReport(t *testing.T) {
	reports := []*IntentReport{
		{
			Description: "Safe withdrawal",
			Refs: []*RefStatus{
				{Ref: "BankAccount.invariant", Status: "verified"},
				{Ref: "BankAccount.withdraw.requires", Status: "verified"},
			},
		},
	}

	output := FormatReport(reports)

	if !strings.Contains(output, "Safe withdrawal") {
		t.Errorf("Expected intent description in report, got: %s", output)
	}
	if !strings.Contains(output, "VERIFIED") {
		t.Errorf("Expected VERIFIED status in report, got: %s", output)
	}
	if !strings.Contains(output, "all 2 contracts verified") {
		t.Errorf("Expected 'all 2 contracts verified', got: %s", output)
	}
}

func TestQualifiedName(t *testing.T) {
	tests := []struct {
		result   VerifyResult
		expected string
	}{
		{VerifyResult{EntityName: "BankAccount", FunctionName: "withdraw", ContractKind: "requires"}, "BankAccount.withdraw.requires"},
		{VerifyResult{EntityName: "BankAccount", FunctionName: "", ContractKind: "invariant"}, "BankAccount.invariant"},
		{VerifyResult{EntityName: "", FunctionName: "abs", ContractKind: "ensures"}, "abs.ensures"},
	}

	for _, tt := range tests {
		got := tt.result.QualifiedName()
		if got != tt.expected {
			t.Errorf("QualifiedName() = %q, want %q", got, tt.expected)
		}
	}
}

func TestTranslateLoopInvariant(t *testing.T) {
	fn := &ir.Function{
		Name: "sum_to_n",
		Params: []*ir.Param{
			{Name: "n", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
		Requires: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left:  &ir.VarRef{Name: "n", Type: checker.TypeInt},
					Op:    lexer.GEQ,
					Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "n >= 0",
			},
		},
	}

	loop := &ir.WhileStmt{
		Condition: &ir.BinaryExpr{
			Left:  &ir.VarRef{Name: "i", Type: checker.TypeInt},
			Op:    lexer.LT,
			Right: &ir.VarRef{Name: "n", Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		Invariants: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left:  &ir.VarRef{Name: "i", Type: checker.TypeInt},
					Op:    lexer.GEQ,
					Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "i >= 0",
			},
		},
	}

	smtLib := TranslateLoopInvariant(fn, loop, loop.Invariants[0])

	// Should declare function params
	if !strings.Contains(smtLib, "(declare-const n Int)") {
		t.Errorf("Expected n declaration, got: %s", smtLib)
	}
	// Should assume precondition
	if !strings.Contains(smtLib, "(assert (>= n 0))") {
		t.Errorf("Expected precondition assumption, got: %s", smtLib)
	}
	// Should assume invariant (inductive hypothesis)
	if !strings.Contains(smtLib, "(assert (>= i 0))") {
		t.Errorf("Expected inductive hypothesis, got: %s", smtLib)
	}
	// Should assume loop condition
	if !strings.Contains(smtLib, "(assert (< i n))") {
		t.Errorf("Expected loop condition, got: %s", smtLib)
	}
	// Should negate invariant for contradiction proof
	if !strings.Contains(smtLib, "(assert (not (>= i 0)))") {
		t.Errorf("Expected negated invariant, got: %s", smtLib)
	}
}

func TestTranslateLoopInvariantForMethod(t *testing.T) {
	fields := []*ir.Field{
		{Name: "count", Type: checker.TypeInt},
	}
	params := []*ir.Param{
		{Name: "limit", Type: checker.TypeInt},
	}

	loop := &ir.WhileStmt{
		Condition: &ir.BinaryExpr{
			Left: &ir.FieldAccessExpr{
				Object: &ir.SelfRef{Type: checker.TypeInt},
				Field:  "count",
				Type:   checker.TypeInt,
			},
			Op:    lexer.LT,
			Right: &ir.VarRef{Name: "limit", Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		Invariants: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left: &ir.FieldAccessExpr{
						Object: &ir.SelfRef{Type: checker.TypeInt},
						Field:  "count",
						Type:   checker.TypeInt,
					},
					Op:    lexer.GEQ,
					Right: &ir.IntLit{Value: 0, Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "self.count >= 0",
			},
		},
	}

	smtLib := TranslateLoopInvariantForMethod("Counter", "increment", fields, params, loop, loop.Invariants[0])

	if !strings.Contains(smtLib, "(declare-const self_count Int)") {
		t.Errorf("Expected self_count declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (>= self_count 0))") {
		t.Errorf("Expected inductive hypothesis, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (< self_count limit))") {
		t.Errorf("Expected loop condition, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "(assert (not (>= self_count 0)))") {
		t.Errorf("Expected negated invariant, got: %s", smtLib)
	}
}

func TestFindWhileStmts(t *testing.T) {
	// Nested loops with invariants
	inner := &ir.WhileStmt{
		Condition: &ir.BoolLit{Value: true, Type: checker.TypeBool},
		Invariants: []*ir.Contract{
			{Expr: &ir.BoolLit{Value: true, Type: checker.TypeBool}, RawText: "true"},
		},
	}
	outer := &ir.WhileStmt{
		Condition: &ir.BoolLit{Value: true, Type: checker.TypeBool},
		Invariants: []*ir.Contract{
			{Expr: &ir.BoolLit{Value: true, Type: checker.TypeBool}, RawText: "true"},
		},
		Body: []ir.Stmt{inner},
	}
	noInv := &ir.WhileStmt{
		Condition: &ir.BoolLit{Value: true, Type: checker.TypeBool},
	}

	stmts := []ir.Stmt{outer, noInv}
	loops := findWhileStmts(stmts)

	if len(loops) != 2 {
		t.Errorf("Expected 2 loops with invariants, got %d", len(loops))
	}
}

func TestLoopInvariantWithOld(t *testing.T) {
	fn := &ir.Function{
		Name: "accumulate",
		Params: []*ir.Param{
			{Name: "n", Type: checker.TypeInt},
		},
		ReturnType: checker.TypeInt,
	}

	loop := &ir.WhileStmt{
		Condition: &ir.BinaryExpr{
			Left:  &ir.VarRef{Name: "i", Type: checker.TypeInt},
			Op:    lexer.LT,
			Right: &ir.VarRef{Name: "n", Type: checker.TypeInt},
			Type:  checker.TypeBool,
		},
		OldCaptures: []*ir.OldCapture{
			{Name: "__old_sum", Expr: &ir.VarRef{Name: "sum", Type: checker.TypeInt}},
		},
		Invariants: []*ir.Contract{
			{
				Expr: &ir.BinaryExpr{
					Left:  &ir.VarRef{Name: "sum", Type: checker.TypeInt},
					Op:    lexer.GEQ,
					Right: &ir.OldRef{Name: "__old_sum", Type: checker.TypeInt},
					Type:  checker.TypeBool,
				},
				RawText: "sum >= old(sum)",
			},
		},
	}

	smtLib := TranslateLoopInvariant(fn, loop, loop.Invariants[0])

	if !strings.Contains(smtLib, "(declare-const __old_sum Int)") {
		t.Errorf("Expected old capture declaration, got: %s", smtLib)
	}
	if !strings.Contains(smtLib, "__old_sum") {
		t.Errorf("Expected old ref in SMT, got: %s", smtLib)
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
				Expr:    &ir.BoolLit{Value: true, Type: checker.TypeBool},
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
