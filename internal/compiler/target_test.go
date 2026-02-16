package compiler

import (
	"os"
	"strings"
	"testing"
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

	err := EmitToTarget(source, "js", baseName)
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

	err := EmitToTarget(source, "rust", baseName)
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

	err := EmitToTarget(source, "wasm", baseName)
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
