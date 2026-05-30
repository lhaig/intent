package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/ir"
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

// Phase 17 / 17.F: DX flags.

func TestRunTestsListMode(t *testing.T) {
	src := `module test version "1.0";

test "alpha" { assert(true); }
test "beta" { assert(true); }
test "gamma" { assert(true); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{List: true})
	if err != nil {
		t.Fatalf("list mode returned error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 listed results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Listed {
			t.Errorf("expected Listed=true, got %+v", r)
		}
	}
	out := FormatList(results)
	if !strings.Contains(out, "alpha\n") || !strings.Contains(out, "beta\n") || !strings.Contains(out, "gamma\n") {
		t.Errorf("expected all three names in FormatList output, got:\n%s", out)
	}
}

func TestRunTestsListWithFilter(t *testing.T) {
	src := `module test version "1.0";

test "alpha" { assert(true); }
test "beta" { assert(true); }
test "gamma" { assert(true); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{List: true, Filter: "et"})
	if err != nil {
		t.Fatalf("list+filter returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (beta) for filter 'et', got %d", len(results))
	}
	if results[0].Name != "beta" {
		t.Errorf("expected 'beta', got %q", results[0].Name)
	}
}

func TestRunTestsFilterNoMatches(t *testing.T) {
	src := `module test version "1.0";

test "alpha" { assert(true); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	_, err := RunTests(path, TestRunOptions{Filter: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for filter with no matches")
	}
	if !strings.Contains(err.Error(), "no tests matched filter") {
		t.Errorf("expected 'no tests matched filter' diagnostic, got: %v", err)
	}
}

func TestRunTestsFilterScopesResults(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	src := `module test version "1.0";

test "addition" { assert_eq(2 + 2, 4); }
test "subtraction" { assert_eq(5 - 3, 2); }
test "multiplication" { assert_eq(2 * 3, 6); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"js"}, Filter: "tract"})
	if err != nil {
		t.Fatalf("runner returned error: %v", err)
	}
	// Only "subtraction" matches "tract"
	if len(results) != 1 {
		t.Fatalf("expected 1 result for filter 'tract', got %d: %+v", len(results), results)
	}
	if results[0].Name != "subtraction" {
		t.Errorf("expected 'subtraction', got %q", results[0].Name)
	}
}

// Phase 17 / 17.D + ADR 0030: cross-package test discovery.
//
// Tests defined in modules imported by the entry file should be discovered
// and run alongside the entry's own tests.
func TestRunTestsCrossModuleDiscovery(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	// lib.intent defines a public function and a test for it.
	lib := `module lib version "1.0";

public function triple(n: Int) returns Int
    ensures result == n * 3
{
    return n + n + n;
}

test "triple of 4 is 12" {
    assert_eq(triple(4), 12);
}
`
	// main.intent imports lib and adds its own test.
	main := `module main version "1.0";

import "lib.intent";

entry function main() returns Int { return 0; }

test "main module local test" { assert(true); }
`
	if err := os.WriteFile(filepath.Join(dir, "lib.intent"), []byte(lib), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.intent"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := RunTests(filepath.Join(dir, "main.intent"), TestRunOptions{Targets: []string{"js"}})
	if err != nil {
		t.Fatalf("runner returned error: %v", err)
	}
	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
		if !r.Passed {
			t.Errorf("expected %q to pass, got %+v", r.Name, r)
		}
	}
	if !names["triple of 4 is 12"] {
		t.Errorf("expected to discover test from lib.intent; got names: %v", names)
	}
	if !names["main module local test"] {
		t.Errorf("expected to discover entry-file test; got names: %v", names)
	}
}

func TestFormatResultsQuietShape(t *testing.T) {
	results := []TestResult{
		{Name: "a", Target: "rust", Passed: true},
		{Name: "b", Target: "rust", Passed: false, Error: "x"},
	}
	out := FormatResultsQuiet(results)
	if strings.Contains(out, "PASS") || strings.Contains(out, "FAIL ") {
		t.Errorf("quiet output should not contain per-test verdicts, got:\n%s", out)
	}
	if !strings.Contains(out, "1 passed, 1 failed") {
		t.Errorf("expected summary line, got:\n%s", out)
	}
	// Should be exactly one line.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("quiet output should be one line, got %d:\n%s", len(lines), out)
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

// ADR 0031: @target_specific annotation filtering.

func TestRunTestsAnnotationSilentSkipSingleTarget(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	// `--target js` of a test marked @target_specific("rust") — silently
	// excluded from the report. The other test still runs.
	src := `module test version "1.0";

@target_specific("rust")
test "rust only" { assert(1 == 1); }

test "everywhere" { assert(1 == 1); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{Targets: []string{"js"}})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (silent skip in single-target mode), got %d: %+v", len(results), results)
	}
	if results[0].Name != "everywhere" {
		t.Errorf("expected unannotated test to run, got %+v", results[0])
	}
}

func TestRunTestsAnnotationSkipAllTargets(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not available")
	}
	// In --all-targets mode the annotation-excluded target reports SKIP with
	// the annotation as the reason.
	src := `module test version "1.0";

@target_specific("rust")
test "rust only" { assert(1 == 1); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{AllTargets: true})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	byTarget := map[string]TestResult{}
	for _, r := range results {
		byTarget[r.Target] = r
	}
	if r := byTarget["rust"]; !r.Passed {
		t.Errorf("rust row should pass, got %+v", r)
	}
	jsRow := byTarget["js"]
	if !jsRow.Skipped || jsRow.SkipKind != SkipAnnotation {
		t.Errorf("js row should be annotation-skipped, got %+v", jsRow)
	}
	if !strings.Contains(jsRow.Error, "@target_specific") {
		t.Errorf("js skip reason should mention annotation, got %q", jsRow.Error)
	}
	wasmRow := byTarget["wasm"]
	if !wasmRow.Skipped || wasmRow.SkipKind != SkipWASMReject {
		t.Errorf("wasm row should be WASM-rejection skipped, got %+v", wasmRow)
	}
	if AnyFailures(results) {
		t.Error("annotation skips must not cause AnyFailures to return true")
	}
}

func TestRunTestsAnnotationOverallVerdictIsPass(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not available")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	src := `module test version "1.0";

@target_specific("rust")
test "rust only" { assert(1 == 1); }

entry function main() returns Int { return 0; }
`
	path := writeTempIntent(t, src)
	results, err := RunTests(path, TestRunOptions{AllTargets: true})
	if err != nil {
		t.Fatalf("runner returned harness error: %v", err)
	}
	out := FormatResults(results)
	// Per the classify() rules: hasPass + hasSkip → PASS overall verdict.
	if !strings.Contains(out, "PASS") {
		t.Errorf("expected overall PASS verdict, got:\n%s", out)
	}
	if !strings.Contains(out, "@target_specific") {
		t.Errorf("expected js skip line to surface the annotation, got:\n%s", out)
	}
}

// Filter helpers are exercised without invoking any target toolchain.
func TestPartitionTestsForTarget(t *testing.T) {
	makeTest := func(name string, annArgs []string) *ir.Test {
		tt := &ir.Test{Name: name}
		if annArgs != nil {
			tt.Annotations = []*ir.TestAnnotation{{Name: "target_specific", Args: annArgs}}
		}
		return tt
	}
	tests := []*ir.Test{
		makeTest("plain", nil),
		makeTest("rust only", []string{"rust"}),
		makeTest("js only", []string{"js"}),
		makeTest("rust+js", []string{"rust", "js"}),
	}
	runnable, skipped := partitionTestsForTarget(tests, "js")
	if names := namesOf(runnable); len(names) != 3 || names[0] != "plain" || names[1] != "js only" || names[2] != "rust+js" {
		t.Errorf("runnable for js: got %v, want [plain, js only, rust+js]", names)
	}
	if names := namesOf(skipped); len(names) != 1 || names[0] != "rust only" {
		t.Errorf("skipped for js: got %v, want [rust only]", names)
	}
}

func TestAnnotationSkipReasonFormat(t *testing.T) {
	tt := &ir.Test{
		Name:        "x",
		Annotations: []*ir.TestAnnotation{{Name: "target_specific", Args: []string{"rust", "js"}}},
	}
	got := annotationSkipReason(tt, "wasm")
	want := `@target_specific("rust", "js") — skipped on wasm`
	if got != want {
		t.Errorf("annotationSkipReason: got %q want %q", got, want)
	}
}
