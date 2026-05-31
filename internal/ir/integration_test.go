package ir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/rustbe"
)

func TestRoundTripBankAccount(t *testing.T) {
	// Read the bank_account.intent example
	src, err := os.ReadFile("../../examples/bank_account.intent")
	if err != nil {
		t.Fatalf("failed to read bank_account.intent: %v", err)
	}

	// Parse
	p := parser.New(string(src))
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("bank_account.intent"))
	}

	// Check
	result := checker.CheckWithResult(prog)
	if result.Diagnostics.HasErrors() {
		t.Fatalf("check errors: %s", result.Diagnostics.Format("bank_account.intent"))
	}

	// Lower to IR
	mod := ir.Lower(prog, result)

	// Validate IR
	errors := ir.Validate(mod)
	if len(errors) > 0 {
		t.Fatalf("IR validation errors: %v", errors)
	}

	// Generate Rust code
	rustCode := rustbe.Generate(mod, rustbe.Options{})

	// Verify non-empty output
	if rustCode == "" {
		t.Fatal("generated Rust code is empty")
	}

	// Basic sanity checks
	if len(rustCode) < 100 {
		t.Errorf("generated Rust code is suspiciously short: %d bytes", len(rustCode))
	}
}

func TestRoundTripAllExamples(t *testing.T) {
	examplesDir := "../../examples"

	// Read all .intent files in examples directory
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("failed to read examples directory: %v", err)
	}

	testCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip multi_file directory
			if entry.Name() == "multi_file" {
				continue
			}
			continue
		}

		if filepath.Ext(entry.Name()) != ".intent" {
			continue
		}

		testCount++
		t.Run(entry.Name(), func(t *testing.T) {
			filePath := filepath.Join(examplesDir, entry.Name())
			src, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			// Parse
			p := parser.New(string(src))
			prog := p.Parse()
			if p.Diagnostics().HasErrors() {
				t.Fatalf("parse errors in %s: %s", entry.Name(), p.Diagnostics().Format(entry.Name()))
			}

			// Check
			result := checker.CheckWithResult(prog)
			if result.Diagnostics.HasErrors() {
				t.Fatalf("check errors in %s: %s", entry.Name(), result.Diagnostics.Format(entry.Name()))
			}

			// Lower to IR
			mod := ir.Lower(prog, result)

			// Validate IR
			errors := ir.Validate(mod)
			if len(errors) > 0 {
				t.Fatalf("IR validation errors in %s: %v", entry.Name(), errors)
			}

			// Generate Rust code
			rustCode := rustbe.Generate(mod, rustbe.Options{})

			// Verify non-empty output
			if rustCode == "" {
				t.Fatalf("generated Rust code is empty for %s", entry.Name())
			}

			// Basic sanity check
			if len(rustCode) < 50 {
				t.Errorf("generated Rust code for %s is suspiciously short: %d bytes", entry.Name(), len(rustCode))
			}
		})
	}

	if testCount == 0 {
		t.Fatal("no .intent files found in examples directory")
	}

	t.Logf("Successfully tested %d example files", testCount)
}

// Phase 24 / ADR 0034: IR contract clauses carry source positions copied
// from the AST, so the verifier can attach them to VerifyResult and the
// LSP can anchor diagnostics on the failing clause.

func TestContractPositionsThreadThroughLowering(t *testing.T) {
	// Multi-line source with contracts on known lines.
	// Line numbers (1-indexed):
	//   1: module
	//   2: (blank)
	//   3: function abs(n: Int) returns Int
	//   4:     requires n >= -1000000     ← Line 4
	//   5:     ensures result >= 0        ← Line 5
	//   6: {
	//   7:     if n < 0 { return -n; }
	//   8:     return n;
	//   9: }
	src := "module t version \"1.0\";\n" +
		"\n" +
		"function abs(n: Int) returns Int\n" +
		"    requires n >= -1000000\n" +
		"    ensures result >= 0\n" +
		"{\n" +
		"    if n < 0 { return 0 - n; }\n" +
		"    return n;\n" +
		"}\n" +
		"entry function main() returns Int { return abs(-5); }\n"

	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse: %s", p.Diagnostics().Format("test"))
	}
	res := checker.CheckWithResult(prog)
	if res.Diagnostics.HasErrors() {
		t.Fatalf("check: %s", res.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, res)

	var absFn *ir.Function
	for _, f := range mod.Functions {
		if f.Name == "abs" {
			absFn = f
		}
	}
	if absFn == nil {
		t.Fatal("abs function not found in IR")
	}
	if len(absFn.Requires) != 1 {
		t.Fatalf("expected 1 requires clause, got %d", len(absFn.Requires))
	}
	if absFn.Requires[0].Line != 4 {
		t.Errorf("requires line: got %d, want 4", absFn.Requires[0].Line)
	}
	if absFn.Requires[0].Column == 0 {
		t.Errorf("requires column: got 0, want >0")
	}
	if len(absFn.Ensures) != 1 {
		t.Fatalf("expected 1 ensures clause, got %d", len(absFn.Ensures))
	}
	if absFn.Ensures[0].Line != 5 {
		t.Errorf("ensures line: got %d, want 5", absFn.Ensures[0].Line)
	}
}

func TestEntityInvariantPositionsThreadThroughLowering(t *testing.T) {
	// Line numbers:
	//   1: module
	//   2: (blank)
	//   3: entity Account {
	//   4:     field balance: Int;
	//   5:     invariant self.balance >= 0;   ← Line 5
	//   6: }
	src := "module t version \"1.0\";\n" +
		"\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    invariant self.balance >= 0;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"

	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse: %s", p.Diagnostics().Format("test"))
	}
	res := checker.CheckWithResult(prog)
	if res.Diagnostics.HasErrors() {
		t.Fatalf("check: %s", res.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, res)

	if len(mod.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(mod.Entities))
	}
	ent := mod.Entities[0]
	if len(ent.Invariants) != 1 {
		t.Fatalf("expected 1 invariant, got %d", len(ent.Invariants))
	}
	if ent.Invariants[0].Line != 5 {
		t.Errorf("invariant line: got %d, want 5", ent.Invariants[0].Line)
	}
}
