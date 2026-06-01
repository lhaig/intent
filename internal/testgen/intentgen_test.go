package testgen

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// Phase 16 / ADR 0029 task 16.8: testgen Intent emission.

func TestGenerateIntentSingleIntParamFunction(t *testing.T) {
	src := `module mathy version "1.0";

function abs_safe(n: Int) returns Int
    requires n >= -100
    ensures result >= 0
{
    if n < 0 { return 0 - n; }
    return n;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Constraint analyser doesn't recognise negative-literal bounds yet;
	// generator falls back to the default range [-10, 10]. Acceptable v1
	// behaviour — documented as a follow-up.
	want := []string{
		"module mathy_auto_tests version \"1.0\";",
		`test "auto: abs_safe contracts"`,
		"let mutable n: Int = -10;",
		"while n <= 10 {",
		"let __r: Int = abs_safe(n);",
		"assert(__r >= 0);",
		"entry function main() returns Int { return 0; }",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestGenerateIntentNoParamFunction(t *testing.T) {
	src := `module test version "1.0";

function get_constant() returns Int
    ensures result == 42
{
    return 42;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	want := []string{
		`test "auto: get_constant contracts"`,
		"let __r: Int = get_constant();",
		"assert(__r == 42);",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestGenerateIntentSkipsEntryFunction(t *testing.T) {
	src := `module test version "1.0";

entry function main() returns Int
    ensures result >= 0
{
    return 0;
}
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	if strings.Contains(out, `"auto: main contracts"`) {
		t.Errorf("entry function should not be auto-tested, got:\n%s", out)
	}
	if !strings.Contains(out, "No contract-bearing standalone functions") {
		t.Errorf("expected 'no functions' note, got:\n%s", out)
	}
}

func TestGenerateIntentSkipsContractlessFunctions(t *testing.T) {
	src := `module test version "1.0";

function id(x: Int) returns Int { return x; }

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	if strings.Contains(out, `"auto: id contracts"`) {
		t.Errorf("contractless function should not be auto-tested, got:\n%s", out)
	}
}

// Phase 27 / ADR 0036: an entity with constructor contracts but no
// contract-bearing methods gets a skip note (constructor-only tests are
// a future enhancement) rather than an emitted test.
func TestGenerateIntentSkipConstructorOnlyEntity(t *testing.T) {
	src := `module test version "1.0";

entity Counter {
    field n: Int;

    invariant self.n >= 0;

    constructor(start: Int)
        requires start >= 0
        ensures self.n == start
    {
        self.n = start;
    }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	if !strings.Contains(out, "// auto-test: Counter skipped") {
		t.Errorf("expected Counter skip note, got:\n%s", out)
	}
	if strings.Contains(out, `"auto: Counter`) {
		t.Errorf("Counter has no contract-bearing methods; should not emit a test, got:\n%s", out)
	}
}

func TestGenerateIntentPreconditionGuard(t *testing.T) {
	// Functions with multiple preconditions should have one `if not (...)
	// { continue; }` guard per clause so invalid input is skipped, not run.
	src := `module test version "1.0";

function divide(a: Int, b: Int) returns Int
    requires a >= 0
    requires b != 0
    ensures true
{
    return a / b;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Two-arg case falls back to example call (no all-int range iteration),
	// so the preconditions don't get guards in this implementation. Verify
	// the fallback emits something sensible.
	if !strings.Contains(out, "divide(") {
		t.Errorf("expected fallback call to divide(...), got:\n%s", out)
	}
}

// Phase 27 / ADR 0036: entity / method auto-test emission.

func TestGenerateIntentEntityMethodWithOldCapture(t *testing.T) {
	src := `module test version "1.0";

entity Account {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Test block exists for deposit.
	if !strings.Contains(out, `test "auto: Account.deposit contracts"`) {
		t.Errorf("expected Account.deposit auto-test, got:\n%s", out)
	}
	// Construction.
	if !strings.Contains(out, "let mutable a: Account = Account(") {
		t.Errorf("expected entity construction binding, got:\n%s", out)
	}
	// Method param binding so contracts resolve.
	if !strings.Contains(out, "let amount: Int = 1;") {
		t.Errorf("expected method param binding, got:\n%s", out)
	}
	// old() capture.
	if !strings.Contains(out, "let __old_0: Int = a") {
		t.Errorf("expected old() capture binding, got:\n%s", out)
	}
	// Assert references the capture + param.
	if !strings.Contains(out, "__old_0") || !strings.Contains(out, "assert(") {
		t.Errorf("expected assert with old capture, got:\n%s", out)
	}
}

func TestGenerateIntentEntityNonVoidMethod(t *testing.T) {
	src := `module test version "1.0";

entity Counter {
    field n: Int;

    invariant self.n >= 0;

    constructor(start: Int)
        requires start >= 0
        ensures self.n == start
    {
        self.n = start;
    }

    method peek() returns Int
        ensures result == self.n
    {
        return self.n;
    }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	if !strings.Contains(out, `test "auto: Counter.peek contracts"`) {
		t.Errorf("expected Counter.peek auto-test, got:\n%s", out)
	}
	// Non-Void return → __r capture.
	if !strings.Contains(out, "let __r: Int = a.peek(") {
		t.Errorf("expected __r capture for non-Void method, got:\n%s", out)
	}
	// result → __r in the assert.
	if !strings.Contains(out, "__r ==") {
		t.Errorf("expected result→__r rewrite, got:\n%s", out)
	}
}

func TestGenerateIntentEntityContractlessMethodSkipped(t *testing.T) {
	src := `module test version "1.0";

entity Bag {
    field items: Int;

    constructor() ensures self.items == 0 { self.items = 0; }

    method add() returns Void { self.items = self.items + 1; }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Bag has a constructor + a contract-less method. Per ADR 0036 §O6,
	// the method is skipped — and there are no other contract-bearing
	// methods, so the entity gets a skip note (constructor-only).
	if strings.Contains(out, `"auto: Bag.add`) {
		t.Errorf("Bag.add has no contracts; should not be auto-tested, got:\n%s", out)
	}
	if !strings.Contains(out, "// auto-test: Bag skipped") {
		t.Errorf("expected Bag skip note (constructor-only entity), got:\n%s", out)
	}
}

func TestGenerateIntentGenericEntitySkipped(t *testing.T) {
	src := `module test version "1.0";

entity Box<T> {
    field item: T;

    constructor(item: T) { self.item = item; }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	if !strings.Contains(out, "// auto-test: Box skipped (generic") {
		t.Errorf("expected generic-entity skip note, got:\n%s", out)
	}
	if strings.Contains(out, `"auto: Box`) {
		t.Errorf("generic Box should not be auto-tested, got:\n%s", out)
	}
}

func TestGenerateIntentEntityOutputParses(t *testing.T) {
	// Smoke test: the output of GenerateIntent on a contract-bearing
	// entity must itself parse cleanly (with `public` markers added).
	// This catches emission shape regressions.
	src := `module test version "1.0";

public entity Account {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Combine the original source + generated tests, then parse the union.
	// (In practice the generator emits a separate file with an import; here
	// we splice for a fast in-memory parse check.)
	combined := strings.ReplaceAll(src, "entry function main() returns Int { return 0; }",
		strings.ReplaceAll(out, "module test_auto_tests version \"1.0\";", "")) +
		"\nentry function main() returns Int { return 0; }"
	p := parser.New(combined)
	p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Errorf("emitted Phase 27 output failed to parse in-context:\n%s", p.Diagnostics().Format("emitted"))
	}
}

// Phase 28 / ADR 0037: multi-param iteration for free functions.

func TestPerParamCap(t *testing.T) {
	cases := []struct {
		n, want int
	}{
		{1, 1000},
		{2, 31},
		{3, 10},
		{4, 5},
		{5, 3},
		{6, 3},
		{7, 2},
		{8, 2},
		{12, 2},
	}
	for _, tc := range cases {
		got := perParamCap(tc.n)
		if got != tc.want {
			t.Errorf("perParamCap(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

func TestGenerateIntentTwoIntParams(t *testing.T) {
	src := `module test version "1.0";

function add(a: Int, b: Int) returns Int
    requires a >= 0
    requires b >= 0
    ensures result == a + b
{
    return a + b;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	want := []string{
		`test "auto: add contracts"`,
		"let mutable a: Int = 0;", // lower bound from `a >= 0`
		"while a <= 10 {",         // upper default
		"let mutable b: Int = 0;",
		"while b <= 10 {",
		"if not (a >= 0) { continue; }",
		"if not (b >= 0) { continue; }",
		"let __r: Int = add(a, b);", // call site uses declaration order
		"assert(__r == a + b);",
		"b = b + 1;",
		"a = a + 1;",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestGenerateIntentThreeIntParamsRangesTrimmed(t *testing.T) {
	src := `module test version "1.0";

function clamp(x: Int, lo: Int, hi: Int) returns Int
    requires lo <= hi
    ensures result >= lo
    ensures result <= hi
{
    if x < lo { return lo; }
    if x > hi { return hi; }
    return x;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Three Int params, no per-param bounds in requires → default range
	// [-10, 10] = 21 values, cap=10 per param, centered → [-5, 4].
	want := []string{
		`test "auto: clamp contracts"`,
		"let mutable x: Int = -5;",
		"while x <= 4 {",
		"let mutable lo: Int = -5;",
		"while lo <= 4 {",
		"let mutable hi: Int = -5;",
		"while hi <= 4 {",
		"if not (lo <= hi) { continue; }",
		"let __r: Int = clamp(x, lo, hi);",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
	// Call must use source order, not alphabetical (hi/lo/x).
	if strings.Contains(out, "clamp(hi,") || strings.Contains(out, "clamp(lo,") {
		t.Errorf("call site sorted alphabetically; expected source order clamp(x, lo, hi):\n%s", out)
	}
}

func TestGenerateIntentMixedIntStringFallsBack(t *testing.T) {
	src := `module test version "1.0";

function greet(times: Int, name: String) returns String
    requires times >= 0
    ensures len(result) >= 0
{
    return name;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	// Mixed Int+String should fall back to single-example call (no while
	// loop emitted for this function).
	if strings.Contains(out, "while times") {
		t.Errorf("mixed-type param should not iterate; expected single call, got:\n%s", out)
	}
	if !strings.Contains(out, "greet(") {
		t.Errorf("expected fallback call to greet(...), got:\n%s", out)
	}
}

func TestGenerateIntentMultiParamOutputParses(t *testing.T) {
	// Smoke: the emitted multi-param output must parse cleanly when
	// merged into the source module.
	src := `module test version "1.0";

public function add(a: Int, b: Int) returns Int
    requires a >= 0
    requires b >= 0
    ensures result == a + b
{
    return a + b;
}

entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	out := GenerateIntent(prog, "")

	combined := strings.ReplaceAll(src, "entry function main() returns Int { return 0; }",
		strings.ReplaceAll(out, `module test_auto_tests version "1.0";`, "")) +
		"\nentry function main() returns Int { return 0; }"
	p := parser.New(combined)
	p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Errorf("emitted Phase 28 output failed to parse in-context:\n%s", p.Diagnostics().Format("emitted"))
	}
}
