package compiler

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestRegistryPackageImportResolvesViaManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package directory with a module
	pkgDir := filepath.Join(tmpDir, "libs", "types_pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgDir, "types.intent", `module types version "1.0.0";

public function distance(x: Int, y: Int) returns Int {
    return x + y;
}`)

	// Create intent.toml in project root
	manifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
types_pkg = { path = "libs/types_pkg" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file with package import
	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import types_pkg;

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

	// Should have 2 modules: main + types
	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}

	// TopologicalSort should work
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 in sorted, got %d", len(sorted))
	}

	// Package directory should be resolved
	pkgDirs := reg.PackageDirs()
	if dir, ok := pkgDirs["types_pkg"]; !ok {
		t.Error("types_pkg not found in package dirs")
	} else if !strings.HasSuffix(dir, filepath.Join("libs", "types_pkg")) {
		t.Errorf("unexpected package dir: %s", dir)
	}
}

func TestRegistryPackageImportDotted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package directory with two modules
	pkgDir := filepath.Join(tmpDir, "libs", "graph_types")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgDir, "validation.intent", `module validation version "1.0.0";

public function validate(x: Int) returns Bool {
    return x > 0;
}`)
	writeIntentFile(t, pkgDir, "core.intent", `module core version "1.0.0";

public function compute(x: Int) returns Int {
    return x * 2;
}`)

	// Create intent.toml
	manifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
graph_types = { path = "libs/graph_types" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file with dotted package import
	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import graph_types.validation;

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

	// Should have 2 modules: main + validation (not core, since dotted import)
	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}
}

func TestRegistryPackageImportUnknownPackage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create intent.toml with no dependencies
	manifest := `[package]
name = "myproject"
version = "1.0.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file importing unknown package
	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import unknown_pkg;

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	diag, err := reg.DiscoverDependencies()
	if err != nil {
		t.Fatalf("DiscoverDependencies should not return fatal error: %v", err)
	}
	if !diag.HasErrors() {
		t.Fatal("expected diagnostic error for unknown package")
	}

	foundError := false
	for _, d := range diag.Errors() {
		if strings.Contains(d.Message, "unknown package") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("expected 'unknown package' error, got: %s", diag.Format("test"))
	}
}

func TestRegistryTransitiveDependencyResolution(t *testing.T) {
	tmpDir := t.TempDir()

	// Package C (leaf dependency)
	pkgCDir := filepath.Join(tmpDir, "libs", "pkg_c")
	if err := os.MkdirAll(pkgCDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgCDir, "core.intent", `module core version "1.0.0";

public function base_value() returns Int {
    return 100;
}`)

	// Package B depends on C
	pkgBDir := filepath.Join(tmpDir, "libs", "pkg_b")
	if err := os.MkdirAll(pkgBDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgBDir, "middle.intent", `module middle version "1.0.0";

import pkg_c;

public function compute() returns Int {
    return 42;
}`)
	// B's manifest declares dependency on C
	bManifest := `[package]
name = "pkg_b"
version = "1.0.0"

[dependencies]
pkg_c = { path = "../pkg_c" }
`
	if err := os.WriteFile(filepath.Join(pkgBDir, "intent.toml"), []byte(bManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Root project depends on B
	rootManifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
pkg_b = { path = "libs/pkg_b" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(rootManifest), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import pkg_b;

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

	// Should have 3 modules: main + middle (from B) + core (from C)
	if len(reg.AllModules()) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(reg.AllModules()))
	}

	// Both pkg_b and pkg_c should be in package dirs
	pkgDirs := reg.PackageDirs()
	if _, ok := pkgDirs["pkg_b"]; !ok {
		t.Error("pkg_b not found in package dirs")
	}
	if _, ok := pkgDirs["pkg_c"]; !ok {
		t.Error("pkg_c not found in package dirs (transitive dependency not resolved)")
	}

	// TopologicalSort should work without errors
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 files in sort, got %d", len(sorted))
	}
}

func TestRegistryCircularPackageDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Package A depends on B
	pkgADir := filepath.Join(tmpDir, "libs", "pkg_a")
	if err := os.MkdirAll(pkgADir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgADir, "a.intent", `module a version "1.0.0";

public function fa() returns Int {
    return 1;
}`)
	aManifest := `[package]
name = "pkg_a"
version = "1.0.0"

[dependencies]
pkg_b = { path = "../pkg_b" }
`
	if err := os.WriteFile(filepath.Join(pkgADir, "intent.toml"), []byte(aManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Package B depends on A (circular!)
	pkgBDir := filepath.Join(tmpDir, "libs", "pkg_b")
	if err := os.MkdirAll(pkgBDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgBDir, "b.intent", `module b version "1.0.0";

public function fb() returns Int {
    return 2;
}`)
	bManifest := `[package]
name = "pkg_b"
version = "1.0.0"

[dependencies]
pkg_a = { path = "../pkg_a" }
`
	if err := os.WriteFile(filepath.Join(pkgBDir, "intent.toml"), []byte(bManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Root depends on A
	rootManifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
pkg_a = { path = "libs/pkg_a" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(rootManifest), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import pkg_a;

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "circular package dependency detected") {
		t.Errorf("expected 'circular package dependency detected' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pkg_a") {
		t.Errorf("expected 'pkg_a' in cycle error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pkg_b") {
		t.Errorf("expected 'pkg_b' in cycle error, got: %s", errMsg)
	}
}

func TestRegistryDiamondShapedCycleDependency(t *testing.T) {
	// Diamond graph: root -> A, root -> C, A -> D, C -> D, D -> A
	// The cycle D -> A should be detected even when reached via C's path.
	// Uses nested deps/ directories with symlinks to create cycles without
	// path traversal (no ".." references).
	tmpDir := t.TempDir()

	// Package A at libs/pkg_a, depends on D at libs/pkg_a/deps/pkg_d
	pkgADir := filepath.Join(tmpDir, "libs", "pkg_a")
	pkgADepsD := filepath.Join(pkgADir, "deps", "pkg_d")
	if err := os.MkdirAll(pkgADepsD, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgADir, "a.intent", `module a version "1.0.0";

public function fa() returns Int {
    return 1;
}`)
	writeIntentFile(t, pkgADepsD, "d.intent", `module d version "1.0.0";

public function fd() returns Int {
    return 4;
}`)
	// D depends on A — symlink D/deps/pkg_a -> A to create the cycle
	pkgDDepsA := filepath.Join(pkgADepsD, "deps", "pkg_a")
	if err := os.MkdirAll(filepath.Dir(pkgDDepsA), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pkgADir, pkgDDepsA); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(pkgADir, "intent.toml"), []byte(`[package]
name = "pkg_a"
version = "1.0.0"

[dependencies]
pkg_d = { path = "deps/pkg_d" }
`), 0644); err != nil {
		t.Fatal(err)
	}

	dManifest := `[package]
name = "pkg_d"
version = "1.0.0"

[dependencies]
pkg_a = { path = "deps/pkg_a" }
`
	if err := os.WriteFile(filepath.Join(pkgADepsD, "intent.toml"), []byte(dManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Package C at libs/pkg_c, also depends on D (nested copy with same cycle)
	pkgCDir := filepath.Join(tmpDir, "libs", "pkg_c")
	pkgCDepsD := filepath.Join(pkgCDir, "deps", "pkg_d")
	if err := os.MkdirAll(pkgCDepsD, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgCDir, "c.intent", `module c version "1.0.0";

public function fc() returns Int {
    return 3;
}`)
	writeIntentFile(t, pkgCDepsD, "d2.intent", `module d2 version "1.0.0";

public function fd2() returns Int {
    return 5;
}`)
	pkgCDDepsA := filepath.Join(pkgCDepsD, "deps", "pkg_a")
	if err := os.MkdirAll(filepath.Dir(pkgCDDepsA), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pkgADir, pkgCDDepsA); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgCDir, "intent.toml"), []byte(`[package]
name = "pkg_c"
version = "1.0.0"

[dependencies]
pkg_d = { path = "deps/pkg_d" }
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgCDepsD, "intent.toml"), []byte(dManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Root depends on A and C
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(`[package]
name = "myproject"
version = "1.0.0"

[dependencies]
pkg_a = { path = "libs/pkg_a" }
pkg_c = { path = "libs/pkg_c" }
`), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import pkg_a;

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected circular dependency error for diamond-shaped cycle, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "circular package dependency detected") {
		t.Errorf("expected 'circular package dependency detected' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pkg_a") {
		t.Errorf("expected 'pkg_a' in cycle error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pkg_d") {
		t.Errorf("expected 'pkg_d' in cycle error, got: %s", errMsg)
	}
}

func TestResolvePkgDirNormalizesCaretVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := &ModuleRegistry{}
	dep := DependencySpec{Version: "^1.0.0"}
	result, err := reg.resolvePkgDir("", "mypkg", dep)
	if err != nil {
		t.Fatalf("resolvePkgDir returned error: %v", err)
	}
	if result == "" {
		t.Fatal("resolvePkgDir returned empty string for caret constraint")
	}
	// The path should contain the normalized version "1.0.0", not the raw "^1.0.0"
	if strings.Contains(result, "^") {
		t.Errorf("resolvePkgDir should normalize caret constraint, got path: %s", result)
	}
	if !strings.HasSuffix(result, filepath.Join("mypkg", "1.0.0")) {
		t.Errorf("expected path ending in mypkg/1.0.0, got: %s", result)
	}
}

func TestResolvePkgDirNormalizesTildeVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := &ModuleRegistry{}
	dep := DependencySpec{Version: "~2.3.4"}
	result, err := reg.resolvePkgDir("", "otherpkg", dep)
	if err != nil {
		t.Fatalf("resolvePkgDir returned error: %v", err)
	}
	if result == "" {
		t.Fatal("resolvePkgDir returned empty string for tilde constraint")
	}
	if strings.Contains(result, "~") {
		t.Errorf("resolvePkgDir should normalize tilde constraint, got path: %s", result)
	}
	if !strings.HasSuffix(result, filepath.Join("otherpkg", "2.3.4")) {
		t.Errorf("expected path ending in otherpkg/2.3.4, got: %s", result)
	}
}

func TestResolvePkgDirPathTraversalBlocked(t *testing.T) {
	reg := &ModuleRegistry{projectRoot: "/projects/myapp", workspaceRoot: "/projects/myapp"}
	baseDir := "/projects/myapp"
	traversalPaths := []string{"../../..", "../../../etc", "subdir/../../../../etc"}
	for _, p := range traversalPaths {
		dep := DependencySpec{Path: p}
		_, err := reg.resolvePkgDir(baseDir, "evil", dep)
		if err == nil {
			t.Errorf("expected error for traversal path %q, got nil", p)
		}
	}
	// Valid relative path should still work.
	dep := DependencySpec{Path: "libs/mylib"}
	result, err := reg.resolvePkgDir(baseDir, "mylib", dep)
	if err != nil {
		t.Fatalf("unexpected error for valid path: %v", err)
	}
	expected := filepath.Join(baseDir, "libs", "mylib")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolvePkgDirSiblingPackageAllowed(t *testing.T) {
	// A sibling package (../types_pkg) should be allowed when the manifest
	// declares a path dependency that points outside the project root.
	// workspaceRoot should be widened to the parent directory.
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "myapp")
	siblingDir := filepath.Join(tmpDir, "types_pkg")

	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a sibling package with a .intent file
	writeIntentFile(t, siblingDir, "types.intent", `module types version "1.0.0";

public function id(x: Int) returns Int {
    return x;
}`)

	// Create intent.toml with a relative path dep pointing to sibling
	manifest := `[package]
name = "myapp"
version = "1.0.0"

[dependencies]
types_pkg = { path = "../types_pkg" }
`
	if err := os.WriteFile(filepath.Join(projDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file
	entryPath := writeIntentFile(t, projDir, "main.intent", `module main version "1.0.0";

import types_pkg;

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

	// Should have 2 modules: main + types
	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}

	// The sibling package should be resolved
	pkgDirs := reg.PackageDirs()
	if _, ok := pkgDirs["types_pkg"]; !ok {
		t.Error("types_pkg not found in package dirs")
	}
}

func TestCommonAncestor(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"/a/b/c", "/a/b/d", "/a/b"},
		{"/a/b/c", "/a/x/y/z", "/a"},
		{"/projects/myapp", "/projects/myapp/sub", "/projects/myapp"},
		{"/home/user/repos/org/myapp", "/home/user/repos/libs/shared", "/home/user/repos"},
		{"/a", "/b", "/"},
	}
	for _, tt := range tests {
		got := commonAncestor(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("commonAncestor(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestWorkspaceRootWidensDeeperPaths(t *testing.T) {
	// A cousin package (../../cousin-pkg) two levels up should be allowed when
	// the manifest declares it. workspaceRoot should widen to the common ancestor.
	tmpDir := t.TempDir()
	orgDir := filepath.Join(tmpDir, "org")
	projDir := filepath.Join(orgDir, "apps", "myapp")
	cousinDir := filepath.Join(orgDir, "libs", "shared")

	for _, d := range []string{projDir, cousinDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a cousin package with a .intent file
	writeIntentFile(t, cousinDir, "shared.intent", `module shared version "1.0.0";

public function id(x: Int) returns Int {
    return x;
}`)

	// Create intent.toml with a deeper relative path dep
	manifest := `[package]
name = "myapp"
version = "1.0.0"

[dependencies]
shared = { path = "../../libs/shared" }
`
	if err := os.WriteFile(filepath.Join(projDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file
	entryPath := writeIntentFile(t, projDir, "main.intent", `module main version "1.0.0";

import shared;

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

	// Should have 2 modules: main + shared
	if len(reg.AllModules()) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(reg.AllModules()))
	}

	// The cousin package should be resolved
	pkgDirs := reg.PackageDirs()
	if _, ok := pkgDirs["shared"]; !ok {
		t.Error("shared not found in package dirs")
	}
}

func TestRegistryMalformedManifestReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a syntactically invalid intent.toml
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte("[invalid toml!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected error for malformed manifest, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to parse") {
		t.Errorf("expected 'failed to parse' in error, got: %s", errMsg)
	}
}

func TestRegistryTransitiveMalformedManifestReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package with a malformed intent.toml
	badPkgDir := filepath.Join(tmpDir, "libs", "badpkg")
	if err := os.MkdirAll(badPkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, badPkgDir, "lib.intent", `module lib version "1.0.0";

public function helper() returns Int {
    return 1;
}`)
	// Write a malformed intent.toml in the package
	if err := os.WriteFile(filepath.Join(badPkgDir, "intent.toml"), []byte("[broken syntax!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	// Root manifest declares dependency on badpkg
	rootManifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
badpkg = { path = "libs/badpkg" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(rootManifest), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import badpkg;

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected error for malformed transitive manifest, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to parse") {
		t.Errorf("expected 'failed to parse' in error, got: %s", errMsg)
	}
}

func TestRegistryManifestStatErrorReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests unreliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("permission tests unreliable when running as root")
	}

	tmpDir := t.TempDir()

	// Create intent.toml as a symlink whose target is behind an unreadable
	// directory. os.Stat follows symlinks, so it will fail with EACCES
	// (not ENOENT), exercising the non-IsNotExist error path.
	blockedDir := filepath.Join(tmpDir, "blocked")
	if err := os.MkdirAll(blockedDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(blockedDir, "target.toml")
	if err := os.WriteFile(targetFile, []byte(`[package]
name = "test"
version = "1.0.0"
`), 0644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(tmpDir, "intent.toml")
	if err := os.Symlink(targetFile, manifestPath); err != nil {
		t.Fatal(err)
	}
	// Make the directory unreadable so stat on the symlink fails with EACCES
	if err := os.Chmod(blockedDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(blockedDir, 0755); err != nil {
			t.Log("cleanup: failed to restore dir permissions:", err)
		}
	}()

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

entry function main() returns Int {
    return 0;
}`)

	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		t.Fatalf("NewModuleRegistry: %v", err)
	}

	_, err = reg.DiscoverDependencies()
	if err == nil {
		t.Fatal("expected error for unreadable manifest, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "cannot stat") {
		t.Errorf("expected 'cannot stat' in error, got: %s", errMsg)
	}
}

func TestHasCrossPackageImports_ManifestDepsButNoImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package directory with a module
	pkgDir := filepath.Join(tmpDir, "libs", "utils_pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgDir, "helpers.intent", `module helpers version "1.0.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}`)

	// Create intent.toml that declares a dependency
	manifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
utils_pkg = { path = "libs/utils_pkg" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Entry file does NOT import the package
	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

entry function main() returns Int {
    return 42;
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

	// packageDirs should be populated (manifest has deps)
	if len(reg.PackageDirs()) == 0 {
		t.Fatal("expected packageDirs to be non-empty from manifest")
	}

	// But HasCrossPackageImports should be false since no source file uses import
	if reg.HasCrossPackageImports() {
		t.Error("HasCrossPackageImports should be false when no source file imports a package")
	}
}

// TestRegistryDiamondDependencyNoSliceAliasing verifies that diamond-shaped
// dependency graphs (Root -> A, Root -> B, A -> C, B -> C) don't cause
// slice aliasing corruption in the chain parameter during DFS traversal.
func TestRegistryDiamondDependencyNoSliceAliasing(t *testing.T) {
	tmpDir := t.TempDir()

	// Package C (shared leaf dependency)
	pkgCDir := filepath.Join(tmpDir, "libs", "pkg_c")
	if err := os.MkdirAll(pkgCDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgCDir, "leaf.intent", `module leaf version "1.0.0";

public function leaf_val() returns Int {
    return 1;
}`)

	// Package A depends on C
	pkgADir := filepath.Join(tmpDir, "libs", "pkg_a")
	if err := os.MkdirAll(pkgADir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgADir, "a.intent", `module a version "1.0.0";

import pkg_c;

public function a_val() returns Int {
    return 2;
}`)
	aManifest := `[package]
name = "pkg_a"
version = "1.0.0"

[dependencies]
pkg_c = { path = "../pkg_c" }
`
	if err := os.WriteFile(filepath.Join(pkgADir, "intent.toml"), []byte(aManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Package B also depends on C (diamond shape)
	pkgBDir := filepath.Join(tmpDir, "libs", "pkg_b")
	if err := os.MkdirAll(pkgBDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgBDir, "b.intent", `module b version "1.0.0";

import pkg_c;

public function b_val() returns Int {
    return 3;
}`)
	bManifest := `[package]
name = "pkg_b"
version = "1.0.0"

[dependencies]
pkg_c = { path = "../pkg_c" }
`
	if err := os.WriteFile(filepath.Join(pkgBDir, "intent.toml"), []byte(bManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Root depends on both A and B
	rootManifest := `[package]
name = "diamond_project"
version = "1.0.0"

[dependencies]
pkg_a = { path = "libs/pkg_a" }
pkg_b = { path = "libs/pkg_b" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(rootManifest), 0644); err != nil {
		t.Fatal(err)
	}

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import pkg_a;
import pkg_b;

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

	// All 3 packages should be resolved
	pkgDirs := reg.PackageDirs()
	for _, name := range []string{"pkg_a", "pkg_b", "pkg_c"} {
		if _, ok := pkgDirs[name]; !ok {
			t.Errorf("package %s not found in package dirs", name)
		}
	}

	// Should have 4 modules: main + a + b + leaf(c)
	if len(reg.AllModules()) != 4 {
		t.Fatalf("expected 4 modules, got %d", len(reg.AllModules()))
	}

	// TopologicalSort should succeed without errors
	sorted, err := reg.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 files in sort, got %d", len(sorted))
	}
}

func TestHasCrossPackageDepsNoImports(t *testing.T) {
	tmpDir := t.TempDir()

	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

entry function main() returns Int {
    return 0;
}`)

	has, err := HasCrossPackageDeps(entryPath)
	if err != nil {
		t.Fatalf("HasCrossPackageDeps: %v", err)
	}
	if has {
		t.Error("expected no cross-package deps for single-file project with no imports")
	}
}

func TestHasCrossPackageDepsWithImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package directory with a module
	pkgDir := filepath.Join(tmpDir, "libs", "types_pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeIntentFile(t, pkgDir, "types.intent", `module types version "1.0.0";

public function identity(x: Int) returns Int {
    return x;
}`)

	// Create intent.toml with dependency
	manifest := `[package]
name = "myproject"
version = "1.0.0"

[dependencies]
types_pkg = { path = "libs/types_pkg" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create entry file with package import
	entryPath := writeIntentFile(t, tmpDir, "main.intent", `module main version "1.0.0";

import types_pkg;

entry function main() returns Int {
    return 0;
}`)

	has, err := HasCrossPackageDeps(entryPath)
	if err != nil {
		t.Fatalf("HasCrossPackageDeps: %v", err)
	}
	if !has {
		t.Error("expected cross-package deps when project imports a package")
	}
}
