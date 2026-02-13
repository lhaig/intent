package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIntentFile creates a .intent file with the given content in the specified directory.
func writeIntentFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	// Ensure subdirectory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

func TestRegistrySingleFileNoImports(t *testing.T) {
	tmpDir := t.TempDir()

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", diag.Format("test"))
	}

	// Should have exactly one module
	if len(reg.AllModules()) != 1 {
		t.Fatalf("expected 1 module, got %d", len(reg.AllModules()))
	}

	// GetModule should return the parsed program
	prog := reg.GetModule(entryPath)
	if prog == nil {
		t.Fatal("GetModule returned nil for entry path")
	}
	if prog.Module.Name != "main" {
		t.Errorf("expected module name 'main', got %q", prog.Module.Name)
	}

	// TopologicalSort should return just the entry file
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 1 {
		t.Fatalf("expected 1 file in sort, got %d", len(sorted))
	}
	if sorted[0] != entryPath {
		t.Errorf("expected sorted[0] = %q, got %q", entryPath, sorted[0])
	}
}

func TestRegistryTwoFilesAImportsB(t *testing.T) {
	tmpDir := t.TempDir()

	writeIntentFile(t, tmpDir, "b.intent", `module b version "1.0.0";

public function helper() returns Int {
    return 42;
}`)

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "b.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", diag.Format("test"))
	}

	// Should have 2 modules
	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}

	// TopologicalSort: B first (dependency), A last (entry/dependent)
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 files in sort, got %d", len(sorted))
	}

	bPath := filepath.Join(tmpDir, "b.intent")
	if sorted[0] != bPath {
		t.Errorf("expected sorted[0] = %q (b.intent), got %q", bPath, sorted[0])
	}
	if sorted[1] != entryPath {
		t.Errorf("expected sorted[1] = %q (a.intent), got %q", entryPath, sorted[1])
	}
}

func TestRegistryThreeFileChain(t *testing.T) {
	tmpDir := t.TempDir()

	cPath := writeIntentFile(t, tmpDir, "c.intent", `module c version "1.0.0";

public function base() returns Int {
    return 1;
}`)

	writeIntentFile(t, tmpDir, "b.intent", `module b version "1.0.0";

import "c.intent";

public function middle() returns Int {
    return 2;
}`)

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "b.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", diag.Format("test"))
	}

	if len(reg.AllModules()) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(reg.AllModules()))
	}

	// TopologicalSort: C first, then B, then A
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 files in sort, got %d", len(sorted))
	}

	bPath := filepath.Join(tmpDir, "b.intent")
	if sorted[0] != cPath {
		t.Errorf("expected sorted[0] = c.intent, got %q", filepath.Base(sorted[0]))
	}
	if sorted[1] != bPath {
		t.Errorf("expected sorted[1] = b.intent, got %q", filepath.Base(sorted[1]))
	}
	if sorted[2] != entryPath {
		t.Errorf("expected sorted[2] = a.intent, got %q", filepath.Base(sorted[2]))
	}
}

func TestRegistryDiamondDependency(t *testing.T) {
	tmpDir := t.TempDir()

	dPath := writeIntentFile(t, tmpDir, "d.intent", `module d version "1.0.0";

public function shared() returns Int {
    return 99;
}`)

	writeIntentFile(t, tmpDir, "b.intent", `module b version "1.0.0";

import "d.intent";

public function fb() returns Int {
    return 1;
}`)

	writeIntentFile(t, tmpDir, "c.intent", `module c version "1.0.0";

import "d.intent";

public function fc() returns Int {
    return 2;
}`)

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "b.intent";
import "c.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", diag.Format("test"))
	}

	if len(reg.AllModules()) != 4 {
		t.Fatalf("expected 4 modules, got %d", len(reg.AllModules()))
	}

	// TopologicalSort: D must come first, A must come last
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 files in sort, got %d", len(sorted))
	}

	// D must be first (leaf dependency)
	if sorted[0] != dPath {
		t.Errorf("expected sorted[0] = d.intent, got %q", filepath.Base(sorted[0]))
	}

	// A must be last (entry point, depends on everything)
	if sorted[3] != entryPath {
		t.Errorf("expected sorted[3] = a.intent, got %q", filepath.Base(sorted[3]))
	}

	// B and C can be in any order (both depend on D, A depends on both)
	middleFiles := []string{filepath.Base(sorted[1]), filepath.Base(sorted[2])}
	hasB := false
	hasC := false
	for _, f := range middleFiles {
		if f == "b.intent" {
			hasB = true
		}
		if f == "c.intent" {
			hasC = true
		}
	}
	if !hasB || !hasC {
		t.Errorf("expected b.intent and c.intent in middle positions, got %v", middleFiles)
	}
}

func TestRegistrySimpleCycle(t *testing.T) {
	tmpDir := t.TempDir()

	writeIntentFile(t, tmpDir, "b.intent", `module b version "1.0.0";

import "a.intent";

public function fb() returns Int {
    return 1;
}`)

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "b.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	// Parse diagnostics may exist but we care about cycle detection in TopologicalSort
	_ = diag

	_, err = reg.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "import cycle detected") {
		t.Errorf("expected 'import cycle detected' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "a.intent") {
		t.Errorf("expected 'a.intent' in cycle error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "b.intent") {
		t.Errorf("expected 'b.intent' in cycle error, got: %s", errMsg)
	}
}

func TestRegistryThreeNodeCycle(t *testing.T) {
	tmpDir := t.TempDir()

	writeIntentFile(t, tmpDir, "c.intent", `module c version "1.0.0";

import "a.intent";

public function fc() returns Int {
    return 3;
}`)

	writeIntentFile(t, tmpDir, "b.intent", `module b version "1.0.0";

import "c.intent";

public function fb() returns Int {
    return 2;
}`)

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "b.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	_ = diag

	_, err = reg.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "import cycle detected") {
		t.Errorf("expected 'import cycle detected' in error, got: %s", errMsg)
	}
	// All three files should appear in the cycle path
	if !strings.Contains(errMsg, "a.intent") {
		t.Errorf("expected 'a.intent' in cycle error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "b.intent") {
		t.Errorf("expected 'b.intent' in cycle error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "c.intent") {
		t.Errorf("expected 'c.intent' in cycle error, got: %s", errMsg)
	}
}

func TestRegistryMissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	entryPath := writeIntentFile(t, tmpDir, "a.intent", `module a version "1.0.0";

import "nonexistent.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected error for missing import, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "imported file not found") {
		t.Errorf("expected 'imported file not found' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "nonexistent.intent") {
		t.Errorf("expected 'nonexistent.intent' in error, got: %s", errMsg)
	}
}

func TestRegistrySubdirectoryImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	helperPath := writeIntentFile(t, tmpDir, "sub/helper.intent", `module helper version "1.0.0";

public function help() returns Int {
    return 42;
}`)

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import "sub/helper.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}
	if diag.HasErrors() {
		t.Fatalf("unexpected diagnostics: %s", diag.Format("test"))
	}

	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}

	// Verify the subdirectory module was resolved correctly
	prog := reg.GetModule(helperPath)
	if prog == nil {
		t.Fatal("GetModule returned nil for sub/helper.intent")
	}
	if prog.Module.Name != "helper" {
		t.Errorf("expected module name 'helper', got %q", prog.Module.Name)
	}

	// TopologicalSort: helper first, main last
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 files in sort, got %d", len(sorted))
	}
	if sorted[0] != helperPath {
		t.Errorf("expected sorted[0] = helper.intent, got %q", filepath.Base(sorted[0]))
	}
	if sorted[1] != entryPath {
		t.Errorf("expected sorted[1] = main.intent, got %q", filepath.Base(sorted[1]))
	}
}

func TestRegistryParseErrorsIncludeFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with a parse error (missing semicolon after module)
	writeIntentFile(t, tmpDir, "bad.intent", `module bad version "1.0.0"

function broken() returns Int {
    return 0;
}`)

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import "bad.intent";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies: %v", err)
	}

	// Should have parse errors from bad.intent
	if !diag.HasErrors() {
		t.Fatal("expected parse errors from bad.intent")
	}

	// The diagnostics should include the file path
	formatted := diag.Format("default")
	badPath := filepath.Join(tmpDir, "bad.intent")
	if !strings.Contains(formatted, badPath) {
		t.Errorf("expected file path %q in diagnostics, got: %s", badPath, formatted)
	}
}

func TestRegistryResolveImportPath(t *testing.T) {
	tests := []struct {
		importPath  string
		projectRoot string
		want        string
	}{
		{
			importPath:  "math.intent",
			projectRoot: "/project",
			want:        "/project/math.intent",
		},
		{
			importPath:  "sub/helper.intent",
			projectRoot: "/project",
			want:        "/project/sub/helper.intent",
		},
		{
			importPath:  "deep/nested/lib.intent",
			projectRoot: "/project/root",
			want:        "/project/root/deep/nested/lib.intent",
		},
	}

	for _, tt := range tests {
		got := resolveImportPath(tt.importPath, tt.projectRoot)
		if got != tt.want {
			t.Errorf("resolveImportPath(%q, %q) = %q, want %q",
				tt.importPath, tt.projectRoot, got, tt.want)
		}
	}
}
