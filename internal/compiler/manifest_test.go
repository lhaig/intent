package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestParseBasic(t *testing.T) {
	input := `[package]
name = "my-project"
version = "1.0.0"

[dependencies]
graph-types = "0.2.0"
utils = { path = "../utils" }
`
	m, err := ParseManifest([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Package.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", m.Package.Name)
	}
	if m.Package.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", m.Package.Version)
	}

	if dep, ok := m.Dependencies["graph-types"]; !ok {
		t.Error("missing dependency 'graph-types'")
	} else if dep.Version != "0.2.0" {
		t.Errorf("expected version '0.2.0', got %q", dep.Version)
	}

	if dep, ok := m.Dependencies["utils"]; !ok {
		t.Error("missing dependency 'utils'")
	} else if dep.Path != "../utils" {
		t.Errorf("expected path '../utils', got %q", dep.Path)
	}
}

func TestManifestParseDescription(t *testing.T) {
	input := `[package]
name = "test"
version = "0.1.0"
description = "A test package"
`
	m, err := ParseManifest([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Package.Description != "A test package" {
		t.Errorf("expected description 'A test package', got %q", m.Package.Description)
	}
}

func TestManifestParseComments(t *testing.T) {
	input := `# This is a comment
[package]
# Package name
name = "test"
version = "0.1.0"
`
	m, err := ParseManifest([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Package.Name != "test" {
		t.Errorf("expected name 'test', got %q", m.Package.Name)
	}
}

func TestManifestParseErrorVersionAndPath(t *testing.T) {
	input := `[package]
name = "test"
version = "0.1.0"

[dependencies]
mylib = { version = "1.0.0", path = "../mylib" }
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error when both version and path are specified")
	}
	if !strings.Contains(err.Error(), "cannot have both version") {
		t.Errorf("expected 'cannot have both version' in error, got: %v", err)
	}
}

func TestManifestParseNoDependencies(t *testing.T) {
	input := `[package]
name = "minimal"
version = "0.0.1"
`
	m, err := ParseManifest([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Dependencies) != 0 {
		t.Errorf("expected no dependencies, got %d", len(m.Dependencies))
	}
}

func TestManifestParseErrorUnclosedSection(t *testing.T) {
	input := `[package
name = "test"
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for unclosed section")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("expected line number in error, got: %v", err)
	}
}

func TestManifestParseErrorMissingEquals(t *testing.T) {
	input := `[package]
name "test"
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing equals")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected line 2 in error, got: %v", err)
	}
}

func TestManifestParseErrorUnquotedValue(t *testing.T) {
	input := `[package]
name = test
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for unquoted value")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected line 2 in error, got: %v", err)
	}
}

func TestManifestParseErrorUnknownSection(t *testing.T) {
	input := `[unknown]
key = "value"
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
	if !strings.Contains(err.Error(), "unknown section") {
		t.Errorf("expected 'unknown section' in error, got: %v", err)
	}
}

func TestManifestParseErrorKeyOutsideSection(t *testing.T) {
	input := `name = "test"
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for key outside section")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("expected line 1 in error, got: %v", err)
	}
}

func TestManifestWriteAndReparse(t *testing.T) {
	original := &Manifest{
		Package: PackageInfo{
			Name:        "roundtrip",
			Version:     "2.0.0",
			Description: "Test round-trip",
		},
		Dependencies: map[string]DependencySpec{
			"alpha": {Version: "1.0.0"},
			"beta":  {Path: "../beta"},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "intent.toml")

	if err := WriteManifest(path, original); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	reparsed, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("failed to re-parse written manifest: %v\nContent:\n%s", err, string(data))
	}

	if reparsed.Package.Name != original.Package.Name {
		t.Errorf("name mismatch: %q vs %q", reparsed.Package.Name, original.Package.Name)
	}
	if reparsed.Package.Version != original.Package.Version {
		t.Errorf("version mismatch: %q vs %q", reparsed.Package.Version, original.Package.Version)
	}
	if reparsed.Package.Description != original.Package.Description {
		t.Errorf("description mismatch: %q vs %q", reparsed.Package.Description, original.Package.Description)
	}

	for name, origDep := range original.Dependencies {
		reDep, ok := reparsed.Dependencies[name]
		if !ok {
			t.Errorf("missing dependency %q after round-trip", name)
			continue
		}
		if reDep.Version != origDep.Version {
			t.Errorf("dep %q version mismatch: %q vs %q", name, reDep.Version, origDep.Version)
		}
		if reDep.Path != origDep.Path {
			t.Errorf("dep %q path mismatch: %q vs %q", name, reDep.Path, origDep.Path)
		}
	}
}

func TestManifestLoadFromDirectory(t *testing.T) {
	dir := t.TempDir()
	content := `[package]
name = "loaded"
version = "3.0.0"

[dependencies]
foo = "1.0.0"
`
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if m.Package.Name != "loaded" {
		t.Errorf("expected name 'loaded', got %q", m.Package.Name)
	}
	if dep, ok := m.Dependencies["foo"]; !ok || dep.Version != "1.0.0" {
		t.Errorf("expected dependency foo=1.0.0, got %+v", m.Dependencies)
	}
}

func TestManifestLoadMissingFile(t *testing.T) {
	_, err := LoadManifest(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing intent.toml")
	}
}

// Known limitation: parseDependencyValue splits inline tables on comma,
// so quoted values containing commas produce incorrect results.
func TestManifestParseInlineTableCommaInQuotedValue(t *testing.T) {
	input := `[package]
name = "test"
version = "0.1.0"

[dependencies]
mylib = { version = "1.0.0", description = "foo, bar" }
`
	// This parse fails because the comma inside "foo, bar" splits the
	// inline table incorrectly, producing a malformed key-value entry.
	// When the TOML parser is improved, this test should be updated to
	// expect successful parsing with description = "foo, bar".
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error due to comma-in-quoted-value limitation, but parse succeeded")
	}
}

// Known limitation: parseString does not handle escape sequences.
// Backslash-escaped quotes inside strings cause incorrect parsing.
func TestManifestParseEscapeSequenceInString(t *testing.T) {
	input := `[package]
name = "test\"pkg"
version = "0.1.0"
`
	// The parser does not handle \" escape sequences. It sees the first
	// unescaped quote as the string terminator, so "test\"pkg" is parsed
	// as "test\" with trailing garbage causing an error.
	// When escape handling is added, this test should expect name = `test"pkg`.
	m, err := ParseManifest([]byte(input))
	if err == nil {
		// If it somehow parses, the name will be wrong (truncated).
		if m.Package.Name == `test"pkg` {
			t.Fatal("escape sequences are now handled — update this test")
		}
		t.Logf("parsed name as %q (expected incorrect due to missing escape handling)", m.Package.Name)
	}
	// Either an error or a wrong name is the expected outcome.
}

func TestManifestWriteRejectsUnsafePackageName(t *testing.T) {
	unsafe := []string{
		"bad=name",
		"bad\nname",
		"bad[name",
		"bad]name",
		"bad\tname",
		"bad name",
		"bad.name",
		"bad/name",
		"bad@name",
		"",
	}
	for _, name := range unsafe {
		m := &Manifest{
			Package:      PackageInfo{Name: name, Version: "1.0.0"},
			Dependencies: map[string]DependencySpec{},
		}
		err := WriteManifest(filepath.Join(t.TempDir(), "intent.toml"), m)
		if err == nil {
			t.Errorf("expected error for package name %q, got nil", name)
		}
	}
}

func TestManifestWriteRejectsUnsafeDependencyName(t *testing.T) {
	unsafe := []string{
		"dep=bad",
		"dep\nbad",
		"dep[bad",
		"dep]bad",
		"dep\tbad",
		"dep bad",
		"dep.bad",
		"dep/bad",
		"dep@bad",
		"",
	}
	for _, name := range unsafe {
		m := &Manifest{
			Package:      PackageInfo{Name: "ok", Version: "1.0.0"},
			Dependencies: map[string]DependencySpec{name: {Version: "1.0.0"}},
		}
		err := WriteManifest(filepath.Join(t.TempDir(), "intent.toml"), m)
		if err == nil {
			t.Errorf("expected error for dependency name %q, got nil", name)
		}
	}
}

// Known limitation: inline comments are not stripped. The # and everything
// after it becomes part of the parsed value.
func TestManifestParseInlineComment(t *testing.T) {
	input := `[package]
name = "test" # this is a comment
version = "0.1.0"
`
	// The parser does not strip inline comments. It treats the closing
	// quote as part of the key=value split, so parseString sees:
	//   "test" # this is a comment
	// which does not end with a quote → parse error.
	// When inline comment support is added, this test should expect
	// name = "test" with the comment stripped.
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error due to inline comment limitation, but parse succeeded")
	}
}

func TestDependencySpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		dep     DependencySpec
		wantErr bool
	}{
		{"version only", DependencySpec{Version: "1.0.0"}, false},
		{"path only", DependencySpec{Path: "../lib"}, false},
		{"empty", DependencySpec{}, false},
		{"both version and path", DependencySpec{Version: "1.0.0", Path: "../lib"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dep.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseManifestRejectsBothVersionAndPath(t *testing.T) {
	input := `[package]
name = "test"
version = "0.1.0"

[dependencies]
mylib = { version = "1.0.0", path = "../lib" }
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for dependency with both version and path, but got nil")
	}
	if !strings.Contains(err.Error(), "cannot have both version") {
		t.Errorf("expected error about both version and path, got: %v", err)
	}
}

func TestManifestParseErrorInlineTableBadKey(t *testing.T) {
	input := `[package]
name = "test"
version = "0.1.0"

[dependencies]
bad = { unknown = "value" }
`
	_, err := ParseManifest([]byte(input))
	if err == nil {
		t.Fatal("expected error for unknown inline table key")
	}
	if !strings.Contains(err.Error(), "unknown dependency key") {
		t.Errorf("expected 'unknown dependency key' in error, got: %v", err)
	}
}
