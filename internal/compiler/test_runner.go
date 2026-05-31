package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/jsbe"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/rustbe"
)

// TestResult is the per-test, per-target outcome from `intentc test`.
//
// Skipped is true when the test did not execute on this target. Two skip
// reasons exist; SkipKind disambiguates them. The CLI report uses Error as
// the user-facing reason text.
type TestResult struct {
	Name     string
	Target   string
	Passed   bool
	Skipped  bool     // true when the test did not execute on this target
	SkipKind SkipKind // why the test was skipped (only meaningful when Skipped=true)
	Listed   bool     // true when produced by --list rather than a real run
	Output   string   // any stdout/stderr captured from the test
	Error    string   // failure message (or skip reason text), empty on plain pass
}

// SkipKind classifies why a test was skipped on a target. ADR 0031 distinguishes
// the WASM-rejection skip (target-level limitation; phase 16 task 16.6) from
// the @target_specific annotation skip (user opt-out for a specific target).
type SkipKind int

const (
	SkipNone       SkipKind = iota // not skipped
	SkipWASMReject                 // target rejects all tests (wasm)
	SkipAnnotation                 // @target_specific excludes this target
)

// TestRunOptions controls the runner.
//
// Targets is the list of targets to run on. If empty, defaults to []string{"rust"}.
// AllTargets is a convenience that sets Targets to all supported targets.
// AllTargets honours the WASM scope reduction from phase 16 task 16.6:
// WASM rejects tests, so "wasm" is reported as Skipped rather than executed.
//
// Phase 17 / section 17.F DX additions:
//   - Filter: substring matched against test names; non-matching tests are
//     skipped entirely (not executed, not in the report).
//   - List: when true, RunTests returns one TestResult per declared test
//     with Listed=true and skips execution. The CLI prints names and exits.
//   - Quiet: only the summary line is printed by the CLI; per-test results
//     are suppressed.
type TestRunOptions struct {
	Targets    []string
	AllTargets bool
	Filter     string
	List       bool
	Quiet      bool
}

// RunTests is the top-level entry for `intentc test`. It loads the program at
// filePath, compiles tests per target, executes them, and returns per-test
// results. The returned error is non-nil only for harness failures
// (parse/check/lower errors, target tools missing); per-test failures are
// reported via TestResult.Passed=false.
func RunTests(filePath string, opts TestRunOptions) ([]TestResult, error) {
	if opts.AllTargets {
		opts.Targets = []string{"rust", "js", "wasm"}
	}
	if len(opts.Targets) == 0 {
		opts.Targets = []string{"rust"}
	}

	// Parse + check the input. Reused across targets.
	isMulti, err := IsMultiFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}

	var (
		mod  *ir.Module
		prog *ir.Program
	)

	if isMulti {
		registry, regErr := NewModuleRegistry(filePath)
		if regErr != nil {
			return nil, fmt.Errorf("initialize module registry: %w", regErr)
		}
		diag, depErr := registry.DiscoverDependencies()
		if depErr != nil {
			return nil, fmt.Errorf("discover dependencies: %w", depErr)
		}
		if diag.HasErrors() {
			return nil, fmt.Errorf("discovery errors:\n%s", diag.Format(filePath))
		}
		sortedPaths, sortErr := registry.TopologicalSort()
		if sortErr != nil {
			return nil, fmt.Errorf("topological sort: %w", sortErr)
		}
		allModules := registry.AllModules()
		checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
		if checkResult.Diagnostics.HasErrors() {
			return nil, fmt.Errorf("type-check errors:\n%s", checkResult.Diagnostics.Format(filePath))
		}
		prog = ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())
	} else {
		source, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", filePath, readErr)
		}
		p := parser.New(string(source))
		astProg := p.Parse()
		if p.Diagnostics().HasErrors() {
			return nil, fmt.Errorf("parse errors:\n%s", p.Diagnostics().Format(filePath))
		}
		checkResult := checker.CheckWithResult(astProg)
		if checkResult.Diagnostics.HasErrors() {
			return nil, fmt.Errorf("type-check errors:\n%s", checkResult.Diagnostics.Format(filePath))
		}
		mod = ir.Lower(astProg, checkResult)
	}

	// Collect declared tests (with annotations) so per-target filtering can
	// happen below. Names are extracted in declaration order for stable
	// reporting.
	declaredTests := collectTests(mod, prog)
	if len(declaredTests) == 0 {
		return nil, fmt.Errorf("no tests found in %s", filePath)
	}
	declared := make([]string, 0, len(declaredTests))
	for _, t := range declaredTests {
		declared = append(declared, t.Name)
	}

	// Phase 17 / 17.F: --list short-circuits execution.
	if opts.List {
		var listed []TestResult
		for _, name := range declared {
			if opts.Filter != "" && !strings.Contains(name, opts.Filter) {
				continue
			}
			listed = append(listed, TestResult{Name: name, Target: "list", Listed: true})
		}
		return listed, nil
	}

	// Phase 17 / 17.F: --filter narrows the runnable set. Filtering happens
	// before per-target compilation so we don't pay cargo/node cost for
	// tests the user explicitly excluded.
	runnable := declared
	if opts.Filter != "" {
		runnable = nil
		for _, n := range declared {
			if strings.Contains(n, opts.Filter) {
				runnable = append(runnable, n)
			}
		}
		if len(runnable) == 0 {
			return nil, fmt.Errorf("no tests matched filter %q", opts.Filter)
		}
	}

	// ADR 0031: in multi-target mode (--all-targets or explicit multi-target),
	// tests excluded by @target_specific show up as SKIP rows so the user can
	// see they were intentionally omitted. In single-target mode the user has
	// explicitly chosen one target; excluded tests are silently dropped.
	multiTarget := len(opts.Targets) > 1

	var results []TestResult
	for _, target := range opts.Targets {
		switch target {
		case "rust", "js":
			runnableTests, annotationSkipped := partitionTestsForTarget(declaredTests, target)
			runnableNames := namesOf(runnableTests)
			filteredMod, filteredProg := filterForTarget(mod, prog, target)
			var (
				res []TestResult
				err error
			)
			if len(runnableNames) > 0 {
				if target == "rust" {
					res, err = runRustTests(filteredMod, filteredProg, runnableNames)
				} else {
					res, err = runJSTests(filteredMod, filteredProg, runnableNames)
				}
				if err != nil {
					return nil, fmt.Errorf("%s target: %w", target, err)
				}
			}
			results = append(results, res...)
			if multiTarget {
				for _, t := range annotationSkipped {
					results = append(results, TestResult{
						Name:     t.Name,
						Target:   target,
						Skipped:  true,
						SkipKind: SkipAnnotation,
						Error:    annotationSkipReason(t, target),
					})
				}
			}
		case "wasm":
			// Phase 16 task 16.6: WASM rejects all test declarations as a
			// target-level limitation. The annotation surface (ADR 0031) does
			// not change that — a @target_specific("rust") test is still
			// reported as a wasm-rejection skip on the wasm row.
			for _, t := range declaredTests {
				results = append(results, TestResult{
					Name:     t.Name,
					Target:   "wasm",
					Skipped:  true,
					SkipKind: SkipWASMReject,
					Error:    "tests are not supported on the wasm target (phase 16 / ADR 0029)",
				})
			}
		default:
			return nil, fmt.Errorf("unknown target %q (expected rust, js, wasm)", target)
		}
	}

	// Apply --filter at the result level so per-target runners stay simple.
	// The actual cargo/node invocations still build everything; only the
	// reported results are narrowed. This keeps the runner self-contained
	// and avoids per-target argument-shaping for filtering.
	if opts.Filter != "" {
		filtered := results[:0]
		for _, r := range results {
			if strings.Contains(r.Name, opts.Filter) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	return results, nil
}

// FormatList formats the output of a --list run: one test name per line.
// Names are deduplicated across targets (--list collapses to one entry per
// declared test).
func FormatList(results []TestResult) string {
	seen := map[string]bool{}
	var b strings.Builder
	for _, r := range results {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		b.WriteString(r.Name)
		b.WriteByte('\n')
	}
	return b.String()
}

// FormatResultsQuiet formats just the summary line — no per-test detail.
// Used by `intentc test --quiet`.
func FormatResultsQuiet(results []TestResult) string {
	byName := make(map[string]map[string]TestResult)
	var nameOrder []string
	for _, r := range results {
		if _, ok := byName[r.Name]; !ok {
			byName[r.Name] = make(map[string]TestResult)
			nameOrder = append(nameOrder, r.Name)
		}
		byName[r.Name][r.Target] = r
	}
	passed, failed, diverged, skipped := 0, 0, 0, 0
	for _, name := range nameOrder {
		row := byName[name]
		var hasFail, hasPass, hasSkip bool
		for _, r := range row {
			if r.Skipped {
				hasSkip = true
			} else if r.Passed {
				hasPass = true
			} else {
				hasFail = true
			}
		}
		switch {
		case hasFail && hasPass:
			diverged++
		case hasFail:
			failed++
		case hasPass:
			passed++
		case hasSkip:
			skipped++
		}
	}
	return fmt.Sprintf("%d passed, %d failed, %d diverged, %d skipped\n", passed, failed, diverged, skipped)
}

// collectTests returns the declared tests (with annotations) from either a
// single Module or a multi-file Program (whichever is non-nil). Order matches
// declaration. Used by RunTests so per-target filtering (ADR 0031) can see
// annotations without re-parsing.
func collectTests(mod *ir.Module, prog *ir.Program) []*ir.Test {
	var tests []*ir.Test
	if mod != nil {
		return append(tests, mod.Tests...)
	}
	if prog != nil {
		for _, m := range prog.Modules {
			tests = append(tests, m.Tests...)
		}
	}
	return tests
}

// partitionTestsForTarget splits declared tests into those that should run on
// the given target and those that are excluded by an @target_specific
// annotation (ADR 0031). The relative order of each subset is preserved.
func partitionTestsForTarget(tests []*ir.Test, target string) (runnable []*ir.Test, skipped []*ir.Test) {
	for _, t := range tests {
		if t.RunsOnTarget(target) {
			runnable = append(runnable, t)
		} else {
			skipped = append(skipped, t)
		}
	}
	return
}

func namesOf(tests []*ir.Test) []string {
	out := make([]string, len(tests))
	for i, t := range tests {
		out[i] = t.Name
	}
	return out
}

// filterForTarget returns shallow copies of mod and prog where each module's
// Tests slice contains only tests that run on the given target. Exactly one of
// mod/prog is non-nil at call time, mirroring the convention in RunTests.
func filterForTarget(mod *ir.Module, prog *ir.Program, target string) (*ir.Module, *ir.Program) {
	if mod != nil {
		return filterModuleForTarget(mod, target), nil
	}
	if prog != nil {
		out := &ir.Program{Modules: make([]*ir.Module, len(prog.Modules))}
		for i, m := range prog.Modules {
			out.Modules[i] = filterModuleForTarget(m, target)
		}
		return nil, out
	}
	return nil, nil
}

func filterModuleForTarget(mod *ir.Module, target string) *ir.Module {
	cp := *mod
	cp.Tests = cp.Tests[:0:0]
	for _, t := range mod.Tests {
		if t.RunsOnTarget(target) {
			cp.Tests = append(cp.Tests, t)
		}
	}
	return &cp
}

// annotationSkipReason formats the user-facing reason string for an
// annotation-driven skip. The format matches ADR 0031's example:
//
//	@target_specific("rust", "js") — skipped on wasm
func annotationSkipReason(t *ir.Test, target string) string {
	targets := t.TargetSpecificTargets()
	quoted := make([]string, len(targets))
	for i, a := range targets {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	return fmt.Sprintf("@target_specific(%s) — skipped on %s", strings.Join(quoted, ", "), target)
}

// runRustTests writes a temp cargo project, runs `cargo test`, and parses
// libtest's JSON output to recover per-test results. Returns a harness error
// when cargo is not available.
func runRustTests(mod *ir.Module, prog *ir.Program, declared []string) ([]TestResult, error) {
	if _, err := exec.LookPath("cargo"); err != nil {
		return nil, fmt.Errorf("cargo not found on PATH: %w (install rustup to enable rust test runs)", err)
	}

	var rustSource string
	if mod != nil {
		rustSource = rustbe.Generate(mod, rustbe.Options{})
	} else {
		rustSource = rustbe.GenerateAll(prog, rustbe.Options{})
	}

	tmpDir, err := os.MkdirTemp("", "intent-test-rust-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cargoToml := buildCargoToml(rustSource, false, nil)
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return nil, fmt.Errorf("write Cargo.toml: %w", err)
	}
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir src: %w", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(rustSource), 0644); err != nil {
		return nil, fmt.Errorf("write main.rs: %w", err)
	}

	// Run cargo test with libtest's machine-readable JSON output. The unstable
	// flag is required for the JSON format, but is supported on stable since
	// Rust 1.70 when nightly is not available we fall back to text parsing.
	cmd := exec.Command("cargo", "test", "--", "--test-threads=1")
	cmd.Dir = tmpDir
	out, runErr := cmd.CombinedOutput()
	// runErr is non-nil when any test fails; we still parse the output.

	results := parseCargoTestOutput(string(out), declared)
	if len(results) == 0 && runErr != nil {
		// Compilation or harness failure — surface raw output.
		return nil, fmt.Errorf("cargo test failed and produced no parseable results: %w\n%s", runErr, out)
	}
	return results, nil
}

// parseCargoTestOutput recovers per-test pass/fail from libtest text output.
// Lines look like:
//
//	test __test_addition_works ... ok
//	test __test_div_by_zero ... FAILED
//
// The declared list is used to map back from sanitised names to the
// human-readable Intent test names.
func parseCargoTestOutput(out string, declared []string) []TestResult {
	sanitisedToName := make(map[string]string, len(declared))
	for _, n := range declared {
		sanitisedToName[rustbe.SanitiseTestNameExternal(n)] = n
	}

	// Map declared name -> result, populated as we parse, then emitted in
	// declaration order so the runner's report shows source order.
	resultByName := make(map[string]TestResult)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "test ") {
			continue
		}
		rest := strings.TrimPrefix(line, "test ")
		var testFn, verdict string
		if idx := strings.LastIndex(rest, " ... "); idx >= 0 {
			testFn = strings.TrimSpace(rest[:idx])
			verdict = strings.TrimSpace(rest[idx+len(" ... "):])
		} else {
			continue
		}
		if !strings.HasPrefix(testFn, "__test_") {
			continue
		}
		sanitised := strings.TrimPrefix(testFn, "__test_")
		humanName, ok := sanitisedToName[sanitised]
		if !ok {
			humanName = sanitised
		}
		if _, already := resultByName[humanName]; already {
			continue
		}
		passed := verdict == "ok"
		tr := TestResult{Name: humanName, Target: "rust", Passed: passed}
		if !passed {
			tr.Error = "rust target: " + verdict
		}
		resultByName[humanName] = tr
	}
	// Emit in declared order; cover any declared tests that didn't appear
	// in output (build failure or harness mismatch).
	var results []TestResult
	for _, n := range declared {
		if r, ok := resultByName[n]; ok {
			results = append(results, r)
		} else {
			results = append(results, TestResult{
				Name:   n,
				Target: "rust",
				Passed: false,
				Error:  "test did not run (build or harness failure)",
			})
		}
	}
	return results
}

// runJSTests writes the generated JS to a temp file, appends a driver that
// invokes __intent_tests and prints JSON results, and runs node.
func runJSTests(mod *ir.Module, prog *ir.Program, declared []string) ([]TestResult, error) {
	if _, err := exec.LookPath("node"); err != nil {
		return nil, fmt.Errorf("node not found on PATH: %w (install Node.js to enable js test runs)", err)
	}

	var jsSource string
	if mod != nil {
		jsSource = jsbe.GenerateForTest(mod)
	} else {
		// Multi-file path: emit via GenerateAll then strip the entry call as a
		// safety net (multi-file output for the entry module may still include
		// an __intent_main invocation).
		jsSource = jsbe.GenerateAll(prog, jsbe.Options{})
		jsSource = stripJSEntryCall(jsSource)
	}

	driver := `
// Test driver appended by intentc test
(async () => {
    const results = [];
    if (typeof __intent_tests === "undefined") {
        process.stdout.write(JSON.stringify({ error: "no __intent_tests registry" }));
        process.exit(2);
    }
    for (const t of __intent_tests) {
        const r = { name: t.name, passed: true, error: "" };
        try {
            const v = t.fn();
            if (t.isAsync || (v && typeof v.then === "function")) {
                await v;
            }
        } catch (e) {
            r.passed = false;
            r.error = (e && e.message) ? e.message : String(e);
        }
        results.push(r);
    }
    process.stdout.write(JSON.stringify(results));
    process.exit(0);
})();
`

	tmp, err := os.CreateTemp("", "intent-test-*.js")
	if err != nil {
		return nil, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(jsSource + driver); err != nil {
		return nil, fmt.Errorf("write js: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close js: %w", err)
	}

	cmd := exec.Command("node", tmp.Name())
	out, runErr := cmd.Output()
	if runErr != nil {
		return nil, fmt.Errorf("node failed: %w; output: %s", runErr, out)
	}

	var jsResults []struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(out, &jsResults); err != nil {
		return nil, fmt.Errorf("parse node output: %w; output: %s", err, out)
	}

	var results []TestResult
	seen := make(map[string]bool)
	for _, r := range jsResults {
		seen[r.Name] = true
		results = append(results, TestResult{
			Name:   r.Name,
			Target: "js",
			Passed: r.Passed,
			Error:  r.Error,
		})
	}
	for _, n := range declared {
		if !seen[n] {
			results = append(results, TestResult{
				Name:   n,
				Target: "js",
				Passed: false,
				Error:  "test did not run (driver missed it)",
			})
		}
	}
	return results, nil
}

// stripJSEntryCall removes the trailing entry-point invocation from generated
// JS so the test driver can run __intent_tests without process.exit firing
// first.
func stripJSEntryCall(js string) string {
	marker := "// Entry point invocation"
	idx := strings.Index(js, marker)
	if idx < 0 {
		return js
	}
	return js[:idx]
}

// FormatResults turns a slice of TestResult into a human-readable report.
// Tests are grouped by name (preserving the order in which they first appear
// in the result slice — runners pass results in declaration order); targets
// appear as columns. Used by both the CLI and tests.
func FormatResults(results []TestResult) string {
	if len(results) == 0 {
		return "No tests ran.\n"
	}
	// Group by name, preserving first-appearance order in `results` for the
	// outer iteration. Targets pass results in declaration order, so this
	// preserves source order in the report.
	byName := make(map[string]map[string]TestResult)
	var nameOrder []string
	targetSet := make(map[string]bool)
	for _, r := range results {
		if _, ok := byName[r.Name]; !ok {
			byName[r.Name] = make(map[string]TestResult)
			nameOrder = append(nameOrder, r.Name)
		}
		byName[r.Name][r.Target] = r
		targetSet[r.Target] = true
	}
	var targets []string
	for t := range targetSet {
		targets = append(targets, t)
	}
	sort.Strings(targets)

	var b strings.Builder
	passed, failed, diverged, skipped := 0, 0, 0, 0
	for _, name := range nameOrder {
		row := byName[name]
		verdict := classify(row, targets)
		switch verdict {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		case "DIFF":
			diverged++
		case "SKIP":
			skipped++
		}
		fmt.Fprintf(&b, "  %-5s  %s\n", verdict, name)
		for _, target := range targets {
			r, ok := row[target]
			if !ok {
				fmt.Fprintf(&b, "         %s: (not run)\n", target)
				continue
			}
			if r.Skipped {
				fmt.Fprintf(&b, "         %s: skipped — %s\n", target, r.Error)
			} else if r.Passed {
				fmt.Fprintf(&b, "         %s: ok\n", target)
			} else {
				fmt.Fprintf(&b, "         %s: FAIL — %s\n", target, r.Error)
			}
		}
	}
	fmt.Fprintf(&b, "\n%d passed, %d failed, %d diverged, %d skipped\n", passed, failed, diverged, skipped)
	return b.String()
}

func classify(row map[string]TestResult, targets []string) string {
	var hasFail, hasPass, hasSkip bool
	for _, t := range targets {
		r, ok := row[t]
		if !ok {
			continue
		}
		switch {
		case r.Skipped:
			hasSkip = true
		case r.Passed:
			hasPass = true
		default:
			hasFail = true
		}
	}
	if hasFail && !hasPass {
		return "FAIL"
	}
	if hasFail && hasPass {
		return "DIFF"
	}
	if hasPass {
		return "PASS"
	}
	if hasSkip {
		return "SKIP"
	}
	return "FAIL"
}

// AnyFailures returns true if any result is a failure or cross-target
// divergence. The CLI uses this to set the exit code.
func AnyFailures(results []TestResult) bool {
	// Group by name to detect divergence.
	byName := make(map[string]map[string]TestResult)
	for _, r := range results {
		if _, ok := byName[r.Name]; !ok {
			byName[r.Name] = make(map[string]TestResult)
		}
		byName[r.Name][r.Target] = r
	}
	for _, row := range byName {
		var hasFail, hasPass bool
		for _, r := range row {
			if r.Skipped {
				continue
			}
			if r.Passed {
				hasPass = true
			} else {
				hasFail = true
			}
		}
		if hasFail {
			return true
		}
		_ = hasPass
	}
	return false
}
