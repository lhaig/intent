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

func TestBuildCargoTomlNoDeps(t *testing.T) {
	source := `fn main() { println!("hello"); }`
	toml := buildCargoToml(source, false, nil)
	if strings.Contains(toml, "[dependencies]") {
		t.Errorf("Expected no [dependencies] section, got:\n%s", toml)
	}
	if !strings.Contains(toml, "[package]") {
		t.Errorf("Expected [package] section, got:\n%s", toml)
	}
}

func TestBuildCargoTomlWithReqwest(t *testing.T) {
	source := `use reqwest; fn main() { reqwest::blocking::get("url"); }`
	toml := buildCargoToml(source, false, nil)
	if !strings.Contains(toml, "[dependencies]") {
		t.Errorf("Expected [dependencies] section, got:\n%s", toml)
	}
	if !strings.Contains(toml, "reqwest") {
		t.Errorf("Expected reqwest dependency, got:\n%s", toml)
	}
	if !strings.Contains(toml, "blocking") {
		t.Errorf("Expected blocking feature, got:\n%s", toml)
	}
}

func TestBuildCargoTomlWithBothDeps(t *testing.T) {
	source := `use reqwest; use serde_json; fn main() { reqwest::blocking::get("url"); serde_json::from_str("{}"); }`
	toml := buildCargoToml(source, false, nil)
	if !strings.Contains(toml, "[dependencies]") {
		t.Errorf("Expected [dependencies] section, got:\n%s", toml)
	}
	if !strings.Contains(toml, "reqwest") {
		t.Errorf("Expected reqwest dependency, got:\n%s", toml)
	}
	if !strings.Contains(toml, "serde_json") {
		t.Errorf("Expected serde_json dependency, got:\n%s", toml)
	}
	if strings.Contains(toml, "[lib]") {
		t.Errorf("Expected no [lib] section for non-cdylib, got:\n%s", toml)
	}
}

func TestBuildCargoTomlCdylib(t *testing.T) {
	source := `fn main() {}`
	toml := buildCargoToml(source, true, nil)
	if !strings.Contains(toml, "[lib]") {
		t.Errorf("Expected [lib] section for cdylib, got:\n%s", toml)
	}
	if !strings.Contains(toml, "cdylib") {
		t.Errorf("Expected cdylib crate-type, got:\n%s", toml)
	}
}

// Phase 15 / ADR 0028: [rust_dependencies] in intent.toml carries Cargo
// crate pins through to the generated Cargo.toml.
func TestBuildCargoTomlWithRustDeps(t *testing.T) {
	source := `fn main() {}`
	rustDeps := map[string]RustDependencySpec{
		"blake3":      {Version: "1.5"},
		"intent_zstd": {Version: "0.13", Features: []string{"std"}},
	}
	toml := buildCargoToml(source, false, rustDeps)
	if !strings.Contains(toml, `blake3 = "1.5"`) {
		t.Errorf("expected pinned blake3 dep, got:\n%s", toml)
	}
	if !strings.Contains(toml, `intent_zstd = { version = "0.13", features = ["std"] }`) {
		t.Errorf("expected intent_zstd dep with features, got:\n%s", toml)
	}
}

func TestBuildCargoTomlUserPinOverridesSniffer(t *testing.T) {
	// Source triggers the tokio sniffer; user pin for tokio should take precedence.
	source := `#[tokio::main] async fn main() {}`
	rustDeps := map[string]RustDependencySpec{
		"tokio": {Version: "1.40", Features: []string{"rt", "macros"}},
	}
	toml := buildCargoToml(source, false, rustDeps)
	// Only the user-pinned line should appear; the sniffer default (1, full)
	// must NOT.
	if strings.Contains(toml, `tokio = { version = "1", features = ["full"] }`) {
		t.Errorf("sniffer default leaked despite user pin, got:\n%s", toml)
	}
	if !strings.Contains(toml, `tokio = { version = "1.40", features = ["rt", "macros"] }`) {
		t.Errorf("expected user-pinned tokio, got:\n%s", toml)
	}
}

func TestParseManifestWithRustDependencies(t *testing.T) {
	input := `[package]
name = "demo"
version = "0.1.0"

[rust_dependencies]
blake3 = "1.5"
intent_zstd = { version = "0.13", features = ["std", "experimental"] }
`
	m, err := ParseManifest([]byte(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(m.RustDependencies) != 2 {
		t.Fatalf("expected 2 rust deps, got %d", len(m.RustDependencies))
	}
	if m.RustDependencies["blake3"].Version != "1.5" {
		t.Errorf("blake3 version: expected 1.5, got %q", m.RustDependencies["blake3"].Version)
	}
	zstd := m.RustDependencies["intent_zstd"]
	if zstd.Version != "0.13" {
		t.Errorf("intent_zstd version: expected 0.13, got %q", zstd.Version)
	}
	if len(zstd.Features) != 2 || zstd.Features[0] != "std" || zstd.Features[1] != "experimental" {
		t.Errorf("intent_zstd features: expected [std experimental], got %v", zstd.Features)
	}
}
