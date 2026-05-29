package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 16 / ADR 0029 task 16.7: `intentc test` runner.
//
// The runner needs `cargo` for rust and `node` for js. Tests that need a
// real toolchain skip cleanly when the tool is missing so CI without that
// toolchain doesn't fail spuriously.

func TestRunTestsRequiresAtLeastOneTest(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	_, err := RunTests(path, TestRunOptions{Targets: []string{"js"}})
	if err == nil {
		t.Fatal("expected error for program with no tests")
	}
	if !strings.Contains(err.Error(), "no tests found") {
		t.Errorf("expected 'no tests found' diagnostic, got: %v", err)
	}
}

func TestRunTestsJSPassFail(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	src := `module test version "1.0";

test "passes" {
    assert(1 == 1);
}

test "fails" {
    assert(false);
}

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"js"}})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	byName := map[string]TestResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if !byName["passes"].Passed {
		t.Errorf("'passes' should pass; got %+v", byName["passes"])
	}
	if byName["fails"].Passed {
		t.Errorf("'fails' should fail; got %+v", byName["fails"])
	}
	if !AnyFailures(results) {
		t.Error("AnyFailures should be true when a JS test fails")
	}
}

func TestRunTestsAssertEqJS(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	src := `module test version "1.0";

test "eq matches" {
    assert_eq(2 + 2, 4);
}

test "eq mismatches" {
    assert_eq(2 + 2, 5);
}

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"js"}})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	byName := map[string]TestResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if !byName["eq matches"].Passed {
		t.Errorf("'eq matches' should pass; got %+v", byName["eq matches"])
	}
	if byName["eq mismatches"].Passed {
		t.Errorf("'eq mismatches' should fail; got %+v", byName["eq mismatches"])
	}
}

func TestRunTestsAssertCloseJS(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	src := `module test version "1.0";

test "close passes" {
    assert_close(3.14, 3.141, 0.01);
}

test "close fails" {
    assert_close(1.0, 2.0, 0.001);
}

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"js"}})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	byName := map[string]TestResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if !byName["close passes"].Passed {
		t.Errorf("'close passes' should pass; got %+v", byName["close passes"])
	}
	if byName["close fails"].Passed {
		t.Errorf("'close fails' should fail; got %+v", byName["close fails"])
	}
}

func TestRunTestsWasmIsAllSkipped(t *testing.T) {
	src := `module test version "1.0";

test "any test" { assert(true); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"wasm"}})
	if err != nil {
		t.Fatalf("runner should not error on wasm target; got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 wasm result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Errorf("wasm result should be Skipped; got %+v", results[0])
	}
	if !strings.Contains(results[0].Error, "wasm target") {
		t.Errorf("skip error should mention wasm target, got: %s", results[0].Error)
	}
	if AnyFailures(results) {
		t.Error("AnyFailures should be false when only result is a skip")
	}
}

func TestRunTestsRustPassFail(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not available")
	}
	src := `module test version "1.0";

test "passes" {
    assert(1 == 1);
}

test "fails" {
    assert(false);
}

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"rust"}})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	byName := map[string]TestResult{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if !byName["passes"].Passed {
		t.Errorf("'passes' should pass; got %+v", byName["passes"])
	}
	if byName["fails"].Passed {
		t.Errorf("'fails' should fail; got %+v", byName["fails"])
	}
}

func TestRunTestsAllTargetsAgreement(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not available")
	}
	src := `module test version "1.0";

test "rust and js agree" {
    assert(1 + 1 == 2);
}

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{AllTargets: true})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	// Expect 3 results: rust, js, wasm-skip
	if len(results) != 3 {
		t.Fatalf("expected 3 results across rust+js+wasm, got %d: %+v", len(results), results)
	}
	for _, r := range results {
		if r.Target == "wasm" {
			if !r.Skipped {
				t.Errorf("wasm result should be skipped, got %+v", r)
			}
			continue
		}
		if !r.Passed {
			t.Errorf("%s should pass, got %+v", r.Target, r)
		}
	}
	if AnyFailures(results) {
		t.Error("AnyFailures should be false when rust+js agree (PASS) and wasm is skipped")
	}
}

func TestFormatResultsBasic(t *testing.T) {
	results := []TestResult{
		{Name: "a", Target: "rust", Passed: true},
		{Name: "a", Target: "js", Passed: true},
		{Name: "b", Target: "rust", Passed: false, Error: "oops"},
		{Name: "b", Target: "js", Passed: true},
		{Name: "c", Target: "wasm", Skipped: true, Error: "not supported"},
	}
	out := FormatResults(results)

	if !strings.Contains(out, "PASS") || !strings.Contains(out, "DIFF") {
		t.Errorf("expected PASS and DIFF verdicts, got:\n%s", out)
	}
	if !strings.Contains(out, "1 passed") {
		t.Errorf("expected '1 passed' (a), got:\n%s", out)
	}
	if !strings.Contains(out, "1 diverged") {
		t.Errorf("expected '1 diverged' (b), got:\n%s", out)
	}
}

func TestAnyFailuresDivergence(t *testing.T) {
	results := []TestResult{
		{Name: "x", Target: "rust", Passed: true},
		{Name: "x", Target: "js", Passed: false, Error: "diverged"},
	}
	if !AnyFailures(results) {
		t.Error("expected AnyFailures to detect cross-target divergence")
	}
}

// writeTempIntent writes src to a temp .intent file and returns the path.
func writeTempIntent(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.intent")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write temp intent: %v", err)
	}
	return path
}
