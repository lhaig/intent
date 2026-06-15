package compiler

import (
	"os"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/backend"
)

func TestEmitToTargetJS(t *testing.T) {
	source := `module test version "1.0";
entry function main() returns Int {
    print("Hello, World!");
    return 0;
}
`
	baseName := "test_output_js"
	defer os.Remove(baseName + ".js")

	err := EmitToTarget(source, "js", baseName, backend.BuildOptions{})
	if err != nil {
		t.Fatalf("EmitToTarget failed: %v", err)
	}

	// Verify file exists
	content, err := os.ReadFile(baseName + ".js")
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, "function __intent_main()") {
		t.Errorf("Expected function __intent_main, got:\n%s", output)
	}
	if !strings.Contains(output, "console.log") {
		t.Errorf("Expected console.log for print, got:\n%s", output)
	}
	if !strings.Contains(output, "process.exit") {
		t.Errorf("Expected process.exit call, got:\n%s", output)
	}
}

func TestEmitToTargetRust(t *testing.T) {
	source := `module test version "1.0";
entry function main() returns Int {
    print("Hello, World!");
    return 0;
}
`
	baseName := "test_output_rust"
	defer os.Remove(baseName + ".rs")

	err := EmitToTarget(source, "rust", baseName, backend.BuildOptions{})
	if err != nil {
		t.Fatalf("EmitToTarget failed: %v", err)
	}

	// Verify file exists
	content, err := os.ReadFile(baseName + ".rs")
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, "fn __intent_main()") {
		t.Errorf("Expected function __intent_main, got:\n%s", output)
	}
	if !strings.Contains(output, "println!") {
		t.Errorf("Expected println! for print, got:\n%s", output)
	}
}

func TestGetBackend(t *testing.T) {
	tests := []struct {
		target      string
		shouldError bool
	}{
		{"rust", false},
		{"js", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			be, err := getBackend(tt.target)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for target %s, got none", tt.target)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for target %s: %v", tt.target, err)
				}
				if be == nil {
					t.Errorf("Expected backend for target %s, got nil", tt.target)
				}
			}
		})
	}
}

func TestGetBinaryBackend(t *testing.T) {
	tests := []struct {
		target      string
		shouldError bool
	}{
		{"wasm", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			be, err := getBinaryBackend(tt.target)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for target %s, got none", tt.target)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for target %s: %v", tt.target, err)
				}
				if be == nil {
					t.Errorf("Expected binary backend for target %s, got nil", tt.target)
				}
			}
		})
	}
}

func TestEmitToTargetWasm(t *testing.T) {
	source := `module test version "1.0";
entry function main() returns Int {
    return 42;
}
`
	baseName := t.TempDir() + "/test_output_wasm"

	err := EmitToTarget(source, "wasm", baseName, backend.BuildOptions{})
	if err != nil {
		t.Fatalf("EmitToTarget wasm failed: %v", err)
	}

	// Verify file exists and starts with WASM magic
	content, err := os.ReadFile(baseName + ".wasm")
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(content) < 8 {
		t.Fatalf("WASM output too short: %d bytes", len(content))
	}
	if content[0] != 0x00 || content[1] != 0x61 || content[2] != 0x73 || content[3] != 0x6D {
		t.Error("Expected WASM magic \\0asm")
	}
	if content[4] != 0x01 || content[5] != 0x00 || content[6] != 0x00 || content[7] != 0x00 {
		t.Error("Expected WASM version 1")
	}
}

// Regression for prds/done/phase-14-phase11-13-gaps.md item 14.5: the WASM
// backend has no async runtime and previously emitted invalid bytecode when
// the input contained async functions. EmitToTarget must reject async
// programs targeting wasm with a clear error instead of producing broken
// output silently.
func TestEmitToTargetWasmRejectsAsync(t *testing.T) {
	source := `module test version "1.0";
async function compute() returns Future<Int> {
    return 42;
}
entry function main() returns Int {
    return 0;
}
`
	baseName := t.TempDir() + "/test_async_wasm"

	err := EmitToTarget(source, "wasm", baseName, backend.BuildOptions{})
	if err == nil {
		t.Fatalf("expected EmitToTarget to reject async on wasm, got no error")
	}
	if !strings.Contains(err.Error(), "async functions are not supported on the wasm target") {
		t.Errorf("expected clear async-not-supported error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "compute") {
		t.Errorf("error should name the offending function, got: %v", err)
	}

	// And the wasm file should not have been written.
	if _, statErr := os.Stat(baseName + ".wasm"); statErr == nil {
		t.Error("wasm file must not be written when async is detected")
	}
}

// Phase 15 / ADR 0028: extern function declarations must be rejected on
// non-Rust targets, before any output file is written.
func TestEmitToTargetExternRejectedOnJS(t *testing.T) {
	source := `module test version "1.0";
extern function hash(input: String) returns String
    from "blake3::hash";
entry function main() returns Int { return 0; }
`
	baseName := t.TempDir() + "/test_extern_js"

	err := EmitToTarget(source, "js", baseName, backend.BuildOptions{})
	if err == nil {
		t.Fatal("expected EmitToTarget to reject extern on js")
	}
	if !strings.Contains(err.Error(), "extern (Rust FFI) declarations are not supported on the js target") {
		t.Errorf("expected clear FFI-not-supported error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hash") {
		t.Errorf("error should name the offending function, got: %v", err)
	}
	if _, statErr := os.Stat(baseName + ".js"); statErr == nil {
		t.Error("js file must not be written when extern is detected")
	}
}

func TestEmitToTargetExternRejectedOnWasm(t *testing.T) {
	source := `module test version "1.0";
extern function hash(input: String) returns String
    from "blake3::hash";
entry function main() returns Int { return 0; }
`
	baseName := t.TempDir() + "/test_extern_wasm"

	err := EmitToTarget(source, "wasm", baseName, backend.BuildOptions{})
	if err == nil {
		t.Fatal("expected EmitToTarget to reject extern on wasm")
	}
	if !strings.Contains(err.Error(), "extern (Rust FFI) declarations are not supported on the wasm target") {
		t.Errorf("expected clear FFI-not-supported error, got: %v", err)
	}
}

// Phase 16 / ADR 0029: WASM rejects test declarations. The WASM backend lacks
// the exception/trap model needed for an assertion-message channel, so test
// support there is deferred to a future phase.
func TestEmitToTargetWasmRejectsTests(t *testing.T) {
	source := `module test version "1.0";

test "would be rejected" {
    let x: Int = 1;
}

entry function main() returns Int {
    return 0;
}
`
	baseName := t.TempDir() + "/test_test_wasm"

	err := EmitToTarget(source, "wasm", baseName, backend.BuildOptions{})
	if err == nil {
		t.Fatalf("expected EmitToTarget to reject tests on wasm, got no error")
	}
	if !strings.Contains(err.Error(), "test declarations are not supported on the wasm target") {
		t.Errorf("expected clear test-not-supported error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "would be rejected") {
		t.Errorf("error should name the offending test, got: %v", err)
	}
	if !strings.Contains(err.Error(), "use --target rust or --target js") {
		t.Errorf("error should suggest alternative targets, got: %v", err)
	}

	// And the wasm file should not have been written.
	if _, statErr := os.Stat(baseName + ".wasm"); statErr == nil {
		t.Error("wasm file must not be written when tests are detected")
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		target   string
		expected string
	}{
		{"rust", ".rs"},
		{"js", ".js"},
		{"wasm", ".wasm"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			ext := getFileExtension(tt.target)
			if ext != tt.expected {
				t.Errorf("Expected extension %s for target %s, got %s", tt.expected, tt.target, ext)
			}
		})
	}
}
