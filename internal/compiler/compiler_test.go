package compiler

import (
	"os"
	"strings"
	"testing"
)

func TestCompileValidProgram(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    return 0;
}`

	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		t.Fatalf("Expected no errors, got:\n%s", res.Diagnostics.Format("test"))
	}

	if res.RustSource == "" {
		t.Fatal("Expected Rust source output")
	}

	if !strings.Contains(res.RustSource, "fn main()") {
		t.Error("Expected generated Rust to contain main function")
	}
}

func TestCompileParseError(t *testing.T) {
	source := `module test version;` // missing version string

	res := Compile(source)
	if res.Diagnostics == nil || !res.Diagnostics.HasErrors() {
		t.Error("Expected parse errors")
	}

	if res.RustSource != "" {
		t.Error("Expected no Rust source on parse error")
	}
}

func TestCompileCheckError(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    return x;
}`

	res := Compile(source)
	if res.Diagnostics == nil || !res.Diagnostics.HasErrors() {
		t.Error("Expected check errors")
	}

	if res.RustSource != "" {
		t.Error("Expected no Rust source on check error")
	}
}

func TestCheckValidProgram(t *testing.T) {
	source := `module test version "1.0.0";

function add(a: Int, b: Int) returns Int {
    return a + b;
}`

	diag := Check(source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestCheckInvalidProgram(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let x: Int = "hello";
}`

	diag := Check(source)
	if !diag.HasErrors() {
		t.Error("Expected errors for type mismatch")
	}
}

func TestCompileEntityWithContracts(t *testing.T) {
	source := `module banking version "0.1.0";

entity BankAccount {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
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
}

entry function main() returns Int {
    let account: BankAccount = BankAccount(100);
    account.deposit(50);
    let success: Bool = account.withdraw(30);
    return 0;
}

intent "Safe banking" {
    goal: "Account balance never goes negative";
    guarantee: "Balance invariant is maintained";
    verified_by: [BankAccount.invariant, BankAccount.deposit.requires, BankAccount.deposit.ensures];
}`

	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		t.Fatalf("Expected no errors, got:\n%s", res.Diagnostics.Format("test"))
	}

	rust := res.RustSource

	// Check that Rust output has key elements
	checks := []struct {
		substr string
		desc   string
	}{
		{"struct BankAccount", "entity struct"},
		{"fn __check_invariants", "invariant checker"},
		{"fn new(", "constructor"},
		{"fn deposit(", "deposit method"},
		{"fn withdraw(", "withdraw method"},
		{"fn main()", "main function"},
		{"assert!", "contract assertions"},
		{"__old_", "old value capture"},
		{"BankAccount::new(", "constructor call"},
		{"// Intent:", "intent comment"},
	}

	for _, c := range checks {
		if !strings.Contains(rust, c.substr) {
			t.Errorf("Expected Rust output to contain %s (%s)", c.substr, c.desc)
		}
	}
}

func TestEmitRustCreatesFile(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    return 42;
}`

	outPath := t.TempDir() + "/test.rs"
	err := EmitRust(source, outPath)
	if err != nil {
		t.Fatalf("EmitRust failed: %s", err)
	}
}

// --- Multi-file CompileProject tests ---

func TestCompileProjectValidMultiFile(t *testing.T) {
	// Create temp directory with two Intent files
	tmpDir := t.TempDir()

	mathSource := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}

public function multiply(a: Int, b: Int) returns Int {
    return a * b;
}
`
	mainSource := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.add(3, 4);
    let product: Int = math.multiply(val, 2);
    print(product);
    return 0;
}
`

	if err := os.WriteFile(tmpDir+"/math.intent", []byte(mathSource), 0644); err != nil {
		t.Fatalf("Failed to write math.intent: %s", err)
	}
	if err := os.WriteFile(tmpDir+"/main.intent", []byte(mainSource), 0644); err != nil {
		t.Fatalf("Failed to write main.intent: %s", err)
	}

	res := CompileProject(tmpDir + "/main.intent")
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		t.Fatalf("Expected no errors, got:\n%s", res.Diagnostics.Format("test"))
	}

	if res.RustSource == "" {
		t.Fatal("Expected Rust source output")
	}

	// Check for mangled math functions
	if !strings.Contains(res.RustSource, "math_add") {
		t.Errorf("Expected mangled 'math_add' function in output:\n%s", res.RustSource)
	}
	if !strings.Contains(res.RustSource, "math_multiply") {
		t.Errorf("Expected mangled 'math_multiply' function in output:\n%s", res.RustSource)
	}

	// Check entry file main wrapper
	if !strings.Contains(res.RustSource, "fn main()") {
		t.Errorf("Expected 'fn main()' wrapper in output:\n%s", res.RustSource)
	}
}

func TestCompileProjectPrivateFunctionError(t *testing.T) {
	tmpDir := t.TempDir()

	mathSource := `module math version "0.1.0";

function secret(x: Int) returns Int {
    return x + 1;
}
`
	mainSource := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.secret(5);
    return val;
}
`

	if err := os.WriteFile(tmpDir+"/math.intent", []byte(mathSource), 0644); err != nil {
		t.Fatalf("Failed to write math.intent: %s", err)
	}
	if err := os.WriteFile(tmpDir+"/main.intent", []byte(mainSource), 0644); err != nil {
		t.Fatalf("Failed to write main.intent: %s", err)
	}

	res := CompileProject(tmpDir + "/main.intent")
	if res.Diagnostics == nil || !res.Diagnostics.HasErrors() {
		t.Fatal("Expected error when calling private function")
	}
}

func TestCheckProjectValid(t *testing.T) {
	tmpDir := t.TempDir()

	mathSource := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}
`
	mainSource := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.add(1, 2);
    return val;
}
`

	if err := os.WriteFile(tmpDir+"/math.intent", []byte(mathSource), 0644); err != nil {
		t.Fatalf("Failed to write math.intent: %s", err)
	}
	if err := os.WriteFile(tmpDir+"/main.intent", []byte(mainSource), 0644); err != nil {
		t.Fatalf("Failed to write main.intent: %s", err)
	}

	diag := CheckProject(tmpDir + "/main.intent")
	if diag.HasErrors() {
		t.Fatalf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestIsMultiFileWithImports(t *testing.T) {
	tmpDir := t.TempDir()

	source := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    return 0;
}
`
	path := tmpDir + "/main.intent"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write file: %s", err)
	}

	isMulti, err := IsMultiFile(path)
	if err != nil {
		t.Fatalf("IsMultiFile failed: %s", err)
	}
	if !isMulti {
		t.Error("Expected IsMultiFile to return true for file with imports")
	}
}

func TestIsMultiFileWithoutImports(t *testing.T) {
	tmpDir := t.TempDir()

	source := `module main version "0.1.0";

entry function main() returns Int {
    return 0;
}
`
	path := tmpDir + "/main.intent"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write file: %s", err)
	}

	isMulti, err := IsMultiFile(path)
	if err != nil {
		t.Fatalf("IsMultiFile failed: %s", err)
	}
	if isMulti {
		t.Error("Expected IsMultiFile to return false for file without imports")
	}
}

func TestHasImportsFunction(t *testing.T) {
	withImport := `module main version "0.1.0";
import "math.intent";
entry function main() returns Int { return 0; }`

	withoutImport := `module main version "0.1.0";
entry function main() returns Int { return 0; }`

	if !HasImports(withImport) {
		t.Error("Expected HasImports to return true for source with import")
	}
	if HasImports(withoutImport) {
		t.Error("Expected HasImports to return false for source without import")
	}
}

func TestCompileStringInterpolation(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let name: String = "Alice";
    let age: Int = 30;
    let msg: String = "Hello {name}, you are {age} years old";
    return 0;
}
`
	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		t.Fatalf("Compile failed: %s", res.Diagnostics.Format("test"))
	}
	if !strings.Contains(res.RustSource, "format!") {
		t.Errorf("Expected format!() in Rust output, got: %s", res.RustSource)
	}
	if !strings.Contains(res.RustSource, "name") {
		t.Errorf("Expected 'name' variable in format args, got: %s", res.RustSource)
	}
}
