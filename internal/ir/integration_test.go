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
	rustCode := rustbe.Generate(mod)

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
			rustCode := rustbe.Generate(mod)

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
