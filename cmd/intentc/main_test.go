package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestBinary builds the intentc binary to a temp dir and returns its path.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "intentc-test")
	cmd := exec.Command("go", "build", "-o", binary, "./")
	cmd.Dir = "." // go test sets cwd to the package directory, so "." is cmd/intentc/
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}
	return binary
}

// setupManifest creates an intent.toml in dir so that handlePkgAdd can load it.
func setupManifest(t *testing.T, dir string) {
	t.Helper()
	content := `[package]
name = "test-pkg"
version = "1.0.0"

[dependencies]
`
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// runPkgAdd runs "intentc pkg add" with the given args in the given working directory.
func runPkgAdd(binary, dir string, args ...string) (string, int) {
	fullArgs := append([]string{"pkg", "add"}, args...)
	cmd := exec.Command(binary, fullArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(out), exitCode
}

// runPkgInit runs "intentc pkg init" in the given working directory and returns
// combined stdout+stderr output along with the exit code.
func runPkgInit(binary, dir string) (string, int) {
	cmd := exec.Command(binary, "pkg", "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(out), exitCode
}

func TestHandlePkgInit(t *testing.T) {
	binary := buildTestBinary(t)

	t.Run("no log when main.intent exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.intent"), []byte("module mymod version \"1.0.0\";\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out, code := runPkgInit(binary, dir)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, out)
		}
		if strings.Contains(out, "no main.intent found") {
			t.Errorf("should not log fallback note when main.intent exists, got: %s", out)
		}
	})

	t.Run("logs fallback file when main.intent absent", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "alpha.intent"), []byte("module alphamod version \"1.0.0\";\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out, code := runPkgInit(binary, dir)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, out)
		}
		if !strings.Contains(out, "Note: Using module name from alpha.intent (no main.intent found)") {
			t.Errorf("expected fallback note mentioning alpha.intent, got: %s", out)
		}
	})

	t.Run("no log when no intent files exist", func(t *testing.T) {
		dir := t.TempDir()
		out, code := runPkgInit(binary, dir)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, out)
		}
		if strings.Contains(out, "no main.intent found") {
			t.Errorf("should not log fallback note when no intent files exist, got: %s", out)
		}
	})
}

func TestHandlePkgAdd(t *testing.T) {
	binary := buildTestBinary(t)

	t.Run("missing package name", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir)
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "missing package name") {
			t.Errorf("expected 'missing package name' in output, got: %s", out)
		}
	})

	t.Run("no version or path", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg")
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "must specify a version or --path") {
			t.Errorf("expected 'must specify a version or --path' in output, got: %s", out)
		}
	})

	t.Run("valid version", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "^1.0.0")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, out)
		}
		// Verify the manifest was updated
		data, err := os.ReadFile(filepath.Join(dir, "intent.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "mypkg") {
			t.Error("expected 'mypkg' in manifest after add")
		}
	})

	t.Run("valid path", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		pkgDir := filepath.Join(dir, "local-pkg")
		if err := os.Mkdir(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		out, code := runPkgAdd(binary, dir, "mypkg", "--path", pkgDir)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, out)
		}
	})

	t.Run("duplicate version", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "^1.0.0", "^2.0.0")
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "duplicate version argument") {
			t.Errorf("expected 'duplicate version argument' in output, got: %s", out)
		}
	})

	t.Run("duplicate path", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "--path", filepath.Join(t.TempDir(), "a"), "--path", filepath.Join(t.TempDir(), "b"))
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "duplicate --path argument") {
			t.Errorf("expected 'duplicate --path argument' in output, got: %s", out)
		}
	})

	t.Run("both version and path", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		pkgDir := filepath.Join(dir, "local-pkg")
		if err := os.Mkdir(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		out, code := runPkgAdd(binary, dir, "mypkg", "^1.0.0", "--path", pkgDir)
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "cannot specify both") {
			t.Errorf("expected 'cannot specify both' in output, got: %s", out)
		}
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "not-a-version")
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "invalid version constraint") {
			t.Errorf("expected 'invalid version constraint' in output, got: %s", out)
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "--bogus")
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "Error: unknown option") {
			t.Errorf("expected 'Error: unknown option' in output, got: %s", out)
		}
	})

	t.Run("path missing directory argument", func(t *testing.T) {
		dir := t.TempDir()
		setupManifest(t, dir)
		out, code := runPkgAdd(binary, dir, "mypkg", "--path")
		if code == 0 {
			t.Fatal("expected non-zero exit code")
		}
		if !strings.Contains(out, "--path requires a directory argument") {
			t.Errorf("expected '--path requires a directory argument' in output, got: %s", out)
		}
	})
}
