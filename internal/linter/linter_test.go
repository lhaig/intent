package linter

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

func parseAndLint(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}

	diag := Lint(prog)
	var warnings []string
	for _, d := range diag.All() {
		warnings = append(warnings, d.Message)
	}
	return warnings
}

func containsWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// --- Empty function body ---

func TestEmptyFunctionBody(t *testing.T) {
	source := `module test version "1.0.0";
function noop() returns Void {
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "empty body") {
		t.Errorf("Expected empty body warning, got: %v", warnings)
	}
}

func TestNonEmptyFunctionBodyNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function greet() returns Int {
    return 0;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "empty body") {
		t.Errorf("Did not expect empty body warning, got: %v", warnings)
	}
}

// --- Entity without invariant ---

func TestEntityWithoutInvariant(t *testing.T) {
	source := `module test version "1.0.0";
entity Point {
    field x: Int;
    field y: Int;
    constructor(x: Int, y: Int) {
        self.x = x;
        self.y = y;
    }
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "no invariant") {
		t.Errorf("Expected 'no invariant' warning, got: %v", warnings)
	}
}

func TestEntityWithInvariantNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
entity Counter {
    field value: Int;
    invariant self.value >= 0;
    constructor(v: Int)
        requires v >= 0
        ensures self.value == v
    {
        self.value = v;
    }
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "no invariant") {
		t.Errorf("Did not expect 'no invariant' warning, got: %v", warnings)
	}
}

// --- Missing contracts ---

func TestMissingContracts(t *testing.T) {
	source := `module test version "1.0.0";
function add(a: Int, b: Int) returns Int {
    return a + b;
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "no requires or ensures") {
		t.Errorf("Expected missing contracts warning, got: %v", warnings)
	}
}

func TestFunctionWithContractsNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function add(a: Int, b: Int) returns Int
    ensures result == a + b
{
    return a + b;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "no requires or ensures") {
		t.Errorf("Did not expect missing contracts warning, got: %v", warnings)
	}
}

func TestMissingMethodContracts(t *testing.T) {
	source := `module test version "1.0.0";
entity Counter {
    field value: Int;
    invariant self.value >= 0;
    constructor(v: Int)
        requires v >= 0
    {
        self.value = v;
    }
    method get_value() returns Int {
        return self.value;
    }
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "Counter.get_value") {
		t.Errorf("Expected missing contracts warning for method, got: %v", warnings)
	}
}

// --- Intent with empty verified_by ---

func TestIntentEmptyVerifiedBy(t *testing.T) {
	source := `module test version "1.0.0";
intent "Safety" {
    goal: "Be safe";
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "no verified_by") {
		t.Errorf("Expected empty verified_by warning, got: %v", warnings)
	}
}

func TestIntentWithVerifiedByNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
entity Counter {
    field value: Int;
    invariant self.value >= 0;
    constructor(v: Int)
        requires v >= 0
    {
        self.value = v;
    }
}
intent "Safety" {
    goal: "Be safe";
    verified_by: [Counter.invariant];
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "no verified_by") {
		t.Errorf("Did not expect empty verified_by warning, got: %v", warnings)
	}
}

// --- Naming conventions ---

func TestFunctionNamingSnakeCase(t *testing.T) {
	source := `module test version "1.0.0";
function badName() returns Int
    ensures result >= 0
{
    return 0;
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "snake_case") {
		t.Errorf("Expected snake_case naming warning, got: %v", warnings)
	}
}

func TestFunctionNamingCorrectNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function good_name() returns Int
    ensures result >= 0
{
    return 0;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "snake_case") {
		t.Errorf("Did not expect snake_case warning for 'good_name', got: %v", warnings)
	}
}

func TestEntityNamingPascalCase(t *testing.T) {
	source := `module test version "1.0.0";
entity bad_entity {
    field x: Int;
    constructor(x: Int) {
        self.x = x;
    }
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "PascalCase") {
		t.Errorf("Expected PascalCase naming warning, got: %v", warnings)
	}
}

func TestEntityNamingCorrectNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
entity GoodEntity {
    field x: Int;
    invariant self.x >= 0;
    constructor(x: Int)
        requires x >= 0
    {
        self.x = x;
    }
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "PascalCase") {
		t.Errorf("Did not expect PascalCase warning for 'GoodEntity', got: %v", warnings)
	}
}

// --- Unused variables ---

func TestUnusedVariable(t *testing.T) {
	source := `module test version "1.0.0";
function test() returns Int
    ensures result >= 0
{
    let unused: Int = 42;
    return 0;
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "'unused' is declared but never used") {
		t.Errorf("Expected unused variable warning, got: %v", warnings)
	}
}

func TestUsedVariableNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function test() returns Int
    ensures result >= 0
{
    let x: Int = 42;
    return x;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "'x' is declared but never used") {
		t.Errorf("Did not expect unused variable warning for 'x', got: %v", warnings)
	}
}

// --- Unused parameters ---

func TestUnusedParameter(t *testing.T) {
	source := `module test version "1.0.0";
function test(x: Int) returns Int
    ensures result >= 0
{
    return 0;
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "parameter 'x'") && !containsWarning(warnings, "'x' in 'test'") {
		t.Errorf("Expected unused parameter warning, got: %v", warnings)
	}
}

func TestUsedParameterNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function test(x: Int) returns Int
    ensures result >= 0
{
    return x;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "parameter 'x'") {
		t.Errorf("Did not expect unused parameter warning for 'x', got: %v", warnings)
	}
}

// --- Mutable never reassigned ---

func TestMutableNeverReassigned(t *testing.T) {
	source := `module test version "1.0.0";
function test() returns Int
    ensures result >= 0
{
    let mutable x: Int = 42;
    return x;
}`
	warnings := parseAndLint(t, source)
	if !containsWarning(warnings, "mutable but never reassigned") {
		t.Errorf("Expected mutable-never-reassigned warning, got: %v", warnings)
	}
}

func TestMutableReassignedNoWarning(t *testing.T) {
	source := `module test version "1.0.0";
function test() returns Int
    ensures result >= 0
{
    let mutable x: Int = 42;
    x = 100;
    return x;
}`
	warnings := parseAndLint(t, source)
	if containsWarning(warnings, "mutable but never reassigned") {
		t.Errorf("Did not expect mutable-never-reassigned warning, got: %v", warnings)
	}
}

// --- Integration: lint a full program ---

func TestLintBankAccountProgram(t *testing.T) {
	source := `module banking version "0.1.0";

entity BankAccount {
    field owner: String;
    field balance: Int;

    invariant self.balance >= 0;

    constructor(owner: String, initial_balance: Int)
        requires initial_balance >= 0
        ensures self.balance == initial_balance
    {
        self.owner = owner;
        self.balance = initial_balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }

    method withdraw(amount: Int) returns Bool
        requires amount > 0
    {
        if self.balance >= amount {
            self.balance = self.balance - amount;
            return true;
        }
        return false;
    }

    method get_balance() returns Int {
        return self.balance;
    }
}

intent "Safe withdrawal preserves non-negative balance" {
    goal: "BankAccount.withdraw never results in balance < 0";
    guarantee: "if withdraw returns false then balance is unchanged";
    verified_by: [BankAccount.invariant, BankAccount.withdraw.requires, BankAccount.deposit.requires, BankAccount.deposit.ensures];
}

entry function main() returns Int {
    let account: BankAccount = BankAccount("Alice", 100);
    account.deposit(50);
    let success: Bool = account.withdraw(30);
    return 0;
}
`
	warnings := parseAndLint(t, source)
	// Should have some warnings (e.g., get_balance has no contracts, main has no contracts)
	if len(warnings) == 0 {
		t.Error("Expected some warnings for bank_account program")
	}

	// get_balance should trigger missing contracts
	if !containsWarning(warnings, "get_balance") {
		t.Errorf("Expected warning for get_balance, got: %v", warnings)
	}
}

func TestLintHelloProgram(t *testing.T) {
	source := `module hello version "1.0.0";

entry function main() returns Int {
    return 0;
}
`
	warnings := parseAndLint(t, source)
	// main should trigger missing contracts warning
	if !containsWarning(warnings, "no requires or ensures") {
		t.Errorf("Expected missing contracts warning for main, got: %v", warnings)
	}
}

// --- Naming helper tests ---

func TestIsSnakeCase(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"hello", true},
		{"hello_world", true},
		{"get_balance", true},
		{"x", true},
		{"x2", true},
		{"HelloWorld", false},
		{"helloWorld", false},
		{"Hello", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isSnakeCase(tt.name); got != tt.want {
			t.Errorf("isSnakeCase(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsPascalCase(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Hello", true},
		{"HelloWorld", true},
		{"BankAccount", true},
		{"X", true},
		{"hello", false},
		{"hello_world", false},
		{"Hello_World", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isPascalCase(tt.name); got != tt.want {
			t.Errorf("isPascalCase(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
