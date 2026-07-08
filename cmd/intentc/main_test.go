package main

import (
	"fmt"
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

// --- helpers for --self-hosted fmt tests ---

// makeFakeFormatter creates a fake formatter binary in dir/<name>.
// It writes output to a sidecar file and cats it (avoids shell escape issues
// with newlines), then exits with exitCode.
func makeFakeFormatter(t *testing.T, dir, name, output string, exitCode int) string {
	t.Helper()
	contentFile := filepath.Join(dir, name+".content")
	if err := os.WriteFile(contentFile, []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	script := fmt.Sprintf("#!/bin/sh\ncat %q\nexit %d\n", contentFile, exitCode)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunStage2Formatter tests the runStage2Formatter helper with fake binaries.
func TestRunStage2Formatter(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("success: strips exactly one trailing newline", func(t *testing.T) {
		// The stage2 formatter emits formatted source + one trailing newline.
		content := "module foo version \"1.0\";\n\nfunction bar() returns Int { return 1; }\n"
		// fake binary prints content + a trailing newline (like println!/console.log).
		binPath := makeFakeFormatter(t, tmpDir, "fmt-ok", content+"\n", 0)

		// Create a dummy file to pass as the filePath argument.
		dummyFile := filepath.Join(tmpDir, "dummy.intent")
		if err := os.WriteFile(dummyFile, []byte("anything"), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := runStage2Formatter(binPath, dummyFile)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got != content {
			t.Errorf("expected content without extra newline\ngot:  %q\nwant: %q", got, content)
		}
	})

	t.Run("error: non-zero exit includes output in error", func(t *testing.T) {
		binPath := makeFakeFormatter(t, tmpDir, "fmt-err", "parse error: boom\n", 3)

		dummyFile := filepath.Join(tmpDir, "dummy2.intent")
		if err := os.WriteFile(dummyFile, []byte("anything"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := runStage2Formatter(binPath, dummyFile)
		if err == nil {
			t.Fatal("expected an error on non-zero exit, got nil")
		}
		if !strings.Contains(err.Error(), "parse error: boom") {
			t.Errorf("expected error to mention 'parse error: boom', got: %v", err)
		}
	})
}

// TestParseFmtFlags tests flag parsing for the fmt subcommand.
func TestParseFmtFlags(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCheck  bool
		wantSelf   bool
		wantFile   string
		wantErrMsg string
	}{
		{
			name:      "check only",
			args:      []string{"--check", "foo.intent"},
			wantCheck: true, wantSelf: false, wantFile: "foo.intent",
		},
		{
			name:      "self-hosted only",
			args:      []string{"--self-hosted", "bar.intent"},
			wantCheck: false, wantSelf: true, wantFile: "bar.intent",
		},
		{
			name:      "both flags check first",
			args:      []string{"--check", "--self-hosted", "baz.intent"},
			wantCheck: true, wantSelf: true, wantFile: "baz.intent",
		},
		{
			name:      "both flags self-hosted first",
			args:      []string{"--self-hosted", "--check", "qux.intent"},
			wantCheck: true, wantSelf: true, wantFile: "qux.intent",
		},
		{
			name:       "unknown flag errors",
			args:       []string{"--bogus", "x.intent"},
			wantErrMsg: "Unknown option: --bogus",
		},
		{
			name:      "no flags",
			args:      []string{"hello.intent"},
			wantCheck: false, wantSelf: false, wantFile: "hello.intent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCheck, gotSelf, gotFile, gotErr := parseFmtFlags(tc.args)
			if tc.wantErrMsg != "" {
				if !strings.Contains(gotErr, tc.wantErrMsg) {
					t.Errorf("expected errMsg containing %q, got %q", tc.wantErrMsg, gotErr)
				}
				return
			}
			if gotErr != "" {
				t.Errorf("unexpected error: %s", gotErr)
			}
			if gotCheck != tc.wantCheck {
				t.Errorf("checkOnly: got %v, want %v", gotCheck, tc.wantCheck)
			}
			if gotSelf != tc.wantSelf {
				t.Errorf("selfHosted: got %v, want %v", gotSelf, tc.wantSelf)
			}
			if gotFile != tc.wantFile {
				t.Errorf("filePath: got %q, want %q", gotFile, tc.wantFile)
			}
		})
	}
}

// TestFmtSelfHostedEnvOverride tests --self-hosted with INTENT_STAGE2_FMT set.
func TestFmtSelfHostedEnvOverride(t *testing.T) {
	binary := buildTestBinary(t)
	tmpDir := t.TempDir()

	// Write a simple .intent file that the stage1 formatter can also format
	// (we just need a real file on disk for the binary to receive as argument).
	srcPath := filepath.Join(tmpDir, "hello.intent")
	src := "module hello version \"1.0\";\n\nentry function main() returns Int {\n    return 0;\n}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("INTENT_STAGE2_FMT with non-existent binary errors", func(t *testing.T) {
		cmd := exec.Command(binary, "fmt", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_FMT=/nonexistent/path/to/fmt")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit, got success")
		}
		outStr := string(out)
		if !strings.Contains(outStr, "INTENT_STAGE2_FMT") {
			t.Errorf("expected error mentioning INTENT_STAGE2_FMT, got: %s", outStr)
		}
	})

	t.Run("INTENT_STAGE2_FMT with fake binary that succeeds formats file", func(t *testing.T) {
		fmtContent := src // stage2 would return the same content
		// The fake binary prints fmtContent + trailing newline (simulating println!).
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-fmt-ok", fmtContent+"\n", 0)

		// Work on a copy so we can inspect the result.
		workPath := filepath.Join(tmpDir, "work.intent")
		if err := os.WriteFile(workPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(binary, "fmt", "--self-hosted", workPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_FMT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got error: %v\n%s", err, out)
		}
		result, err := os.ReadFile(workPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(result) != fmtContent {
			t.Errorf("file not written correctly\ngot:  %q\nwant: %q", string(result), fmtContent)
		}
	})

	t.Run("INTENT_STAGE2_FMT check mode: already formatted exits 0", func(t *testing.T) {
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-fmt-check-ok", src+"\n", 0)

		checkPath := filepath.Join(tmpDir, "check-already-fmt.intent")
		if err := os.WriteFile(checkPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(binary, "fmt", "--self-hosted", "--check", checkPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_FMT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected exit 0 for already-formatted file, got: %v\n%s", err, out)
		}
		// File should be unchanged.
		result, _ := os.ReadFile(checkPath)
		if string(result) != src {
			t.Error("--check should not modify the file")
		}
	})

	t.Run("INTENT_STAGE2_FMT check mode: not formatted exits non-zero", func(t *testing.T) {
		differentContent := src + "// extra\n"
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-fmt-check-diff", differentContent+"\n", 0)

		checkPath2 := filepath.Join(tmpDir, "check-not-fmt.intent")
		if err := os.WriteFile(checkPath2, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(binary, "fmt", "--self-hosted", "--check", checkPath2)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_FMT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit when file not formatted")
		}
		outStr := string(out)
		if !strings.Contains(outStr, "not formatted") {
			t.Errorf("expected 'not formatted' message, got: %s", outStr)
		}
		// File should be unchanged.
		result, _ := os.ReadFile(checkPath2)
		if string(result) != src {
			t.Error("--check should not modify the file")
		}
	})

	t.Run("INTENT_STAGE2_FMT stage2 parse error: exits non-zero, no fallback", func(t *testing.T) {
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-fmt-parse-err", "parse error: boom\n", 3)

		errPath := filepath.Join(tmpDir, "bad.intent")
		if err := os.WriteFile(errPath, []byte("garbage source"), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command(binary, "fmt", "--self-hosted", errPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_FMT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit on stage2 parse error")
		}
		outStr := string(out)
		if !strings.Contains(outStr, "parse error: boom") {
			t.Errorf("expected stage2 error message in output, got: %s", outStr)
		}
	})
}

// Phase 22 / ADR 0033: --strip-contracts swaps contract assert! for
// debug_assert! on rust and prints a one-line stderr warning.

func TestBuildStripContracts(t *testing.T) {
	binary := buildTestBinary(t)

	src := `module bank version "1.0";

entity Account {
    field balance: Int;
    invariant self.balance >= 0;
    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
    }
}

entry function main() returns Int {
    let a: Account = Account(10);
    return 0;
}
`

	t.Run("no flag preserves assert!", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "bank.intent")
		if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "build", "--emit", srcPath)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, out)
		}
		rs, err := os.ReadFile(filepath.Join(dir, "bank.rs"))
		if err != nil {
			t.Fatalf("read emitted rust: %v", err)
		}
		text := string(rs)
		if !strings.Contains(text, "assert!(") {
			t.Errorf("expected assert! in default emit, got:\n%s", text)
		}
		if strings.Contains(text, "debug_assert!(") {
			t.Errorf("did not expect debug_assert! in default emit, got:\n%s", text)
		}
		if strings.Contains(string(out), "--strip-contracts removes") {
			t.Errorf("warning should not fire without --strip-contracts: %s", out)
		}
	})

	t.Run("--strip-contracts swaps to debug_assert! and warns", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "bank.intent")
		if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "build", "--emit", "--strip-contracts", srcPath)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, out)
		}
		rs, err := os.ReadFile(filepath.Join(dir, "bank.rs"))
		if err != nil {
			t.Fatalf("read emitted rust: %v", err)
		}
		text := string(rs)
		if !strings.Contains(text, "debug_assert!(") {
			t.Errorf("expected debug_assert! in stripped emit, got:\n%s", text)
		}
		// No contract assert! lines should remain — only test-body asserts (this
		// example has no test bodies, so no assert! at all).
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "assert!(") && !strings.Contains(line, "debug_assert!(") {
				t.Errorf("unexpected non-debug assert! line in stripped emit: %s", line)
			}
		}
		if !strings.Contains(string(out), "--strip-contracts removes runtime contract checks") {
			t.Errorf("expected stderr warning, got: %s", out)
		}
	})
}

// --- tests for --self-hosted lint (phase 43.11) ---

// TestParseLintFlags tests flag parsing for the lint subcommand.
func TestParseLintFlags(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantSelf   bool
		wantFile   string
		wantErrMsg string
	}{
		{name: "no flags", args: []string{"hello.intent"}, wantSelf: false, wantFile: "hello.intent"},
		{name: "self-hosted", args: []string{"--self-hosted", "bar.intent"}, wantSelf: true, wantFile: "bar.intent"},
		{name: "self-hosted after file", args: []string{"baz.intent", "--self-hosted"}, wantSelf: true, wantFile: "baz.intent"},
		{name: "unknown flag errors", args: []string{"--bogus", "x.intent"}, wantErrMsg: "Unknown option: --bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSelf, gotFile, gotErr := parseLintFlags(tc.args)
			if tc.wantErrMsg != "" {
				if !strings.Contains(gotErr, tc.wantErrMsg) {
					t.Errorf("expected errMsg containing %q, got %q", tc.wantErrMsg, gotErr)
				}
				return
			}
			if gotErr != "" {
				t.Errorf("unexpected error: %s", gotErr)
			}
			if gotSelf != tc.wantSelf {
				t.Errorf("selfHosted: got %v, want %v", gotSelf, tc.wantSelf)
			}
			if gotFile != tc.wantFile {
				t.Errorf("filePath: got %q, want %q", gotFile, tc.wantFile)
			}
		})
	}
}

// TestRunStage2Linter tests the runStage2Linter helper with fake binaries.
// Unlike the formatter, the linter's stdout is emitted verbatim (no trailing-
// newline trimming) because it already matches stage1 byte-for-byte.
func TestRunStage2Linter(t *testing.T) {
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "dummy.intent")
	if err := os.WriteFile(dummyFile, []byte("anything"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("success: returns stdout verbatim", func(t *testing.T) {
		content := "warning[dummy.intent:1:1]: variable 'x' is declared but never used\n1 warning(s) found.\n"
		binPath := makeFakeFormatter(t, tmpDir, "lint-ok", content, 0)
		got, err := runStage2Linter(binPath, dummyFile)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got != content {
			t.Errorf("expected verbatim output\ngot:  %q\nwant: %q", got, content)
		}
	})

	t.Run("success: no-warnings output verbatim", func(t *testing.T) {
		content := "No lint warnings.\n"
		binPath := makeFakeFormatter(t, tmpDir, "lint-clean", content, 0)
		got, err := runStage2Linter(binPath, dummyFile)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got != content {
			t.Errorf("got %q, want %q", got, content)
		}
	})

	t.Run("error: non-zero exit includes output", func(t *testing.T) {
		binPath := makeFakeFormatter(t, tmpDir, "lint-err", "parse error: boom\n", 3)
		_, err := runStage2Linter(binPath, dummyFile)
		if err == nil {
			t.Fatal("expected an error on non-zero exit, got nil")
		}
		if !strings.Contains(err.Error(), "parse error: boom") {
			t.Errorf("expected error to mention 'parse error: boom', got: %v", err)
		}
	})
}

// TestLintSelfHostedEnvOverride tests `intentc lint --self-hosted` with
// INTENT_STAGE2_LINT set, exercising the full CLI path without needing cargo.
func TestLintSelfHostedEnvOverride(t *testing.T) {
	binary := buildTestBinary(t)
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "hello.intent")
	src := "module hello version \"1.0\";\n\nentry function main() returns Int {\n    return 0;\n}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("INTENT_STAGE2_LINT non-existent binary errors", func(t *testing.T) {
		cmd := exec.Command(binary, "lint", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_LINT=/nonexistent/path/to/lint")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit, got success")
		}
		if !strings.Contains(string(out), "INTENT_STAGE2_LINT") {
			t.Errorf("expected error mentioning INTENT_STAGE2_LINT, got: %s", out)
		}
	})

	t.Run("INTENT_STAGE2_LINT fake binary output emitted verbatim", func(t *testing.T) {
		want := "No lint warnings.\n"
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-lint-ok", want, 0)
		cmd := exec.Command(binary, "lint", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_LINT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got %v\noutput: %s", err, out)
		}
		if string(out) != want {
			t.Errorf("got %q, want %q", string(out), want)
		}
	})

	t.Run("INTENT_STAGE2_LINT stage2 parse error exits non-zero, no fallback", func(t *testing.T) {
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-lint-err", "parse error: bad\n", 3)
		cmd := exec.Command(binary, "lint", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_LINT="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit on stage2 parse error")
		}
		if !strings.Contains(string(out), "parse error: bad") {
			t.Errorf("expected stage2 error in output, got: %s", out)
		}
	})
}

// --- tests for --self-hosted check (phase 45.9) ---

// TestParseCheckFlags tests flag parsing for the check subcommand.
func TestParseCheckFlags(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantSelf   bool
		wantFile   string
		wantErrMsg string
	}{
		{name: "no flags", args: []string{"hello.intent"}, wantSelf: false, wantFile: "hello.intent"},
		{name: "self-hosted", args: []string{"--self-hosted", "bar.intent"}, wantSelf: true, wantFile: "bar.intent"},
		{name: "self-hosted after file", args: []string{"baz.intent", "--self-hosted"}, wantSelf: true, wantFile: "baz.intent"},
		{name: "unknown flag errors", args: []string{"--bogus", "x.intent"}, wantErrMsg: "Unknown option: --bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSelf, gotFile, gotErr := parseCheckFlags(tc.args)
			if tc.wantErrMsg != "" {
				if !strings.Contains(gotErr, tc.wantErrMsg) {
					t.Errorf("expected errMsg containing %q, got %q", tc.wantErrMsg, gotErr)
				}
				return
			}
			if gotErr != "" {
				t.Errorf("unexpected error: %s", gotErr)
			}
			if gotSelf != tc.wantSelf {
				t.Errorf("selfHosted: got %v, want %v", gotSelf, tc.wantSelf)
			}
			if gotFile != tc.wantFile {
				t.Errorf("filePath: got %q, want %q", gotFile, tc.wantFile)
			}
		})
	}
}

// TestRunStage2Checker tests the runStage2Checker helper with fake binaries.
// Unlike runStage2Linter, a non-zero exit is NOT an error — it means the
// checker found semantic errors. The caller inspects (stdout, exitCode).
func TestRunStage2Checker(t *testing.T) {
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "dummy.intent")
	if err := os.WriteFile(dummyFile, []byte("anything"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("exit 0: stdout returned verbatim", func(t *testing.T) {
		content := "No errors found.\n"
		binPath := makeFakeFormatter(t, tmpDir, "check-ok", content, 0)
		out, code, err := runStage2Checker(binPath, []string{dummyFile})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
		if out != content {
			t.Errorf("expected verbatim output\ngot:  %q\nwant: %q", out, content)
		}
	})

	t.Run("exit 1: stdout returned verbatim with exit code 1", func(t *testing.T) {
		content := "error[foo.intent:2:1]: function 'f' already defined\n"
		binPath := makeFakeFormatter(t, tmpDir, "check-err", content, 1)
		out, code, err := runStage2Checker(binPath, []string{dummyFile})
		if err != nil {
			t.Fatalf("expected no error (non-zero exit is not a run error), got: %v", err)
		}
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		if out != content {
			t.Errorf("expected verbatim output\ngot:  %q\nwant: %q", out, content)
		}
	})
}

// TestCheckSelfHostedEnvOverride tests `intentc check --self-hosted` with
// INTENT_STAGE2_CHECK set, exercising the full CLI path without needing cargo.
func TestCheckSelfHostedEnvOverride(t *testing.T) {
	binary := buildTestBinary(t)
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "hello.intent")
	src := "module hello version \"1.0\";\n\nentry function main() returns Int {\n    return 0;\n}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("INTENT_STAGE2_CHECK non-existent binary errors", func(t *testing.T) {
		cmd := exec.Command(binary, "check", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_CHECK=/nonexistent/path/to/check")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit, got success")
		}
		if !strings.Contains(string(out), "INTENT_STAGE2_CHECK") {
			t.Errorf("expected error mentioning INTENT_STAGE2_CHECK, got: %s", out)
		}
	})

	t.Run("INTENT_STAGE2_CHECK fake binary clean: stdout No errors found.", func(t *testing.T) {
		want := "No errors found.\n"
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-check-ok", want, 0)
		cmd := exec.Command(binary, "check", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_CHECK="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got %v\noutput: %s", err, out)
		}
		if string(out) != want {
			t.Errorf("got %q, want %q", string(out), want)
		}
	})

	t.Run("INTENT_STAGE2_CHECK fake binary errors: routed to stderr, non-zero exit", func(t *testing.T) {
		diagBlock := "error[hello.intent:2:1]: function 'f' already defined\n"
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-check-err", diagBlock, 1)
		cmd := exec.Command(binary, "check", "--self-hosted", srcPath)
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_CHECK="+fakeBin)
		stdoutBuf, stderrBuf := &strings.Builder{}, &strings.Builder{}
		cmd.Stdout = stdoutBuf
		cmd.Stderr = stderrBuf
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit when checker reports errors")
		}
		if stdoutBuf.String() != "" {
			t.Errorf("expected empty stdout, got %q", stdoutBuf.String())
		}
		// Stderr should contain the diagnostic (trailing newline stripped by shim, then
		// Fprintf adds none — so the diag block minus its trailing newline is on stderr).
		if !strings.Contains(stderrBuf.String(), "function 'f' already defined") {
			t.Errorf("expected diag block on stderr, got: %q", stderrBuf.String())
		}
	})
}

// TestBuildEmitSelfHostedEnvOverride tests `intentc build --emit --self-hosted`
// with INTENT_STAGE2_COMPILE set to a fake compiler, exercising the full CLI
// path (Phase 55 / ADR 0059) without needing cargo. The fake prints the Rust
// plus one trailing newline (as print() does); the harness strips exactly that
// newline and writes <base>.rs.
func TestBuildEmitSelfHostedEnvOverride(t *testing.T) {
	binary := buildTestBinary(t)
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "hello.intent")
	src := "module hello version \"1.0\";\n\nentry function main() returns Int {\n    return 0;\n}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("non-existent binary errors mentioning INTENT_STAGE2_COMPILE", func(t *testing.T) {
		cmd := exec.Command(binary, "build", "--emit", "--self-hosted", srcPath)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_COMPILE=/nonexistent/path/to/compile")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit, got success")
		}
		if !strings.Contains(string(out), "INTENT_STAGE2_COMPILE") {
			t.Errorf("expected error mentioning INTENT_STAGE2_COMPILE, got: %s", out)
		}
	})

	t.Run("fake compiler: writes <base>.rs with one trailing newline stripped", func(t *testing.T) {
		rust := "// Generated Rust code from Intent\nfn main() {}\n\n"
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-compile-ok", rust+"\n", 0)
		workDir := t.TempDir()
		cmd := exec.Command(binary, "build", "--emit", "--self-hosted", srcPath)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_COMPILE="+fakeBin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected success, got %v\noutput: %s", err, out)
		}
		got, rerr := os.ReadFile(filepath.Join(workDir, "hello.rs"))
		if rerr != nil {
			t.Fatalf("expected hello.rs written: %v", rerr)
		}
		if string(got) != rust {
			t.Errorf("emitted file mismatch\ngot:  %q\nwant: %q", string(got), rust)
		}
	})

	t.Run("fake compiler exit 1: routed to stderr, no file, non-zero exit", func(t *testing.T) {
		fakeBin := makeFakeFormatter(t, tmpDir, "fake-compile-err", "parse error: unexpected token\n", 1)
		workDir := t.TempDir()
		cmd := exec.Command(binary, "build", "--emit", "--self-hosted", srcPath)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "INTENT_STAGE2_COMPILE="+fakeBin)
		stdoutBuf, stderrBuf := &strings.Builder{}, &strings.Builder{}
		cmd.Stdout = stdoutBuf
		cmd.Stderr = stderrBuf
		if err := cmd.Run(); err == nil {
			t.Fatal("expected non-zero exit when stage2 compiler fails")
		}
		if !strings.Contains(stderrBuf.String(), "parse error") {
			t.Errorf("expected parse error on stderr, got: %q", stderrBuf.String())
		}
		if _, statErr := os.Stat(filepath.Join(workDir, "hello.rs")); statErr == nil {
			t.Error("expected no hello.rs written on compiler failure")
		}
	})
}
