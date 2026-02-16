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
		{"wasm", false},
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
