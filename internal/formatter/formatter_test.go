package formatter

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// helper: parse source, format, return formatted string
func formatSource(t *testing.T, source string) string {
	t.Helper()
	p := parser.New(source)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse error: %s", p.Diagnostics().Format("<test>"))
	}
	return Format(prog)
}

// --- Per-construct tests ---

func TestFormatModuleDecl(t *testing.T) {
	src := `module hello version "1.0.0";
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.HasPrefix(got, `module hello version "1.0.0";`) {
		t.Errorf("expected module decl, got:\n%s", got)
	}
}

func TestFormatImportDecl(t *testing.T) {
	src := `module main version "0.1.0";
import "math.intent";
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, `import "math.intent";`) {
		t.Errorf("expected import decl, got:\n%s", got)
	}
}

func TestFormatLetStmt(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 42;
    let mutable y: Int = 0;
    return x;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "let x: Int = 42;") {
		t.Errorf("expected let stmt, got:\n%s", got)
	}
	if !strings.Contains(got, "let mutable y: Int = 0;") {
		t.Errorf("expected mutable let stmt, got:\n%s", got)
	}
}

func TestFormatAssignStmt(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let mutable x: Int = 0;
    x = 42;
    return x;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "    x = 42;") {
		t.Errorf("expected assign stmt, got:\n%s", got)
	}
}

func TestFormatReturnStmt(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "    return 0;") {
		t.Errorf("expected return stmt, got:\n%s", got)
	}
}

func TestFormatIfElse(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 1;
    if x > 0 {
        return 1;
    } else {
        return 0;
    }
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "} else {") {
		t.Errorf("expected K&R else, got:\n%s", got)
	}
}

func TestFormatWhileLoop(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 {
        i = i + 1;
    }
    return i;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "while i < 10 {") {
		t.Errorf("expected while on same line as brace, got:\n%s", got)
	}
}

func TestFormatForIn(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        print(x);
    }
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "for x in arr {") {
		t.Errorf("expected for-in loop, got:\n%s", got)
	}
}

func TestFormatForInRange(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    for i in 0..10 {
        print(i);
    }
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "for i in 0..10 {") {
		t.Errorf("expected for-in range, got:\n%s", got)
	}
}

func TestFormatBreakContinue(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 {
        if i == 5 { break; }
        i = i + 1;
        continue;
    }
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "break;") {
		t.Errorf("expected break, got:\n%s", got)
	}
	if !strings.Contains(got, "continue;") {
		t.Errorf("expected continue, got:\n%s", got)
	}
}

func TestFormatBinaryExpr(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 1 + 2 * 3;
    return x;
}
`
	got := formatSource(t, src)
	// Should produce "1 + 2 * 3" since * binds tighter
	if !strings.Contains(got, "1 + 2 * 3") {
		t.Errorf("expected correct precedence, got:\n%s", got)
	}
}

func TestFormatUnaryExpr(t *testing.T) {
	src := `module test version "1.0";
function check(x: Bool) returns Bool { return not x; }
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "not x") {
		t.Errorf("expected 'not x', got:\n%s", got)
	}
}

func TestFormatArrayLit(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    return arr[0];
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "[1, 2, 3]") {
		t.Errorf("expected array lit, got:\n%s", got)
	}
	if !strings.Contains(got, "arr[0]") {
		t.Errorf("expected index expr, got:\n%s", got)
	}
}

func TestFormatFunctionWithContracts(t *testing.T) {
	src := `module test version "1.0";
function fib(n: Int) returns Int
    requires n >= 0
    ensures result >= 0
{
    return n;
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "    requires n >= 0\n") {
		t.Errorf("expected indented requires, got:\n%s", got)
	}
	if !strings.Contains(got, "    ensures result >= 0\n") {
		t.Errorf("expected indented ensures, got:\n%s", got)
	}
}

func TestFormatFunctionNoContracts(t *testing.T) {
	src := `module test version "1.0";
function add(a: Int, b: Int) returns Int { return a + b; }
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "function add(a: Int, b: Int) returns Int {") {
		t.Errorf("expected brace on same line, got:\n%s", got)
	}
}

func TestFormatEntityDecl(t *testing.T) {
	src := `module test version "1.0";
entity Foo {
    field x: Int;
    field y: String;

    invariant self.x >= 0;

    constructor(x: Int)
        requires x >= 0
    {
        self.x = x;
        self.y = "hello";
    }

    method get_x() returns Int {
        return self.x;
    }
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "field x: Int;") {
		t.Errorf("expected field decl, got:\n%s", got)
	}
	if !strings.Contains(got, "invariant self.x >= 0;") {
		t.Errorf("expected invariant, got:\n%s", got)
	}
	if !strings.Contains(got, "constructor(x: Int)") {
		t.Errorf("expected constructor, got:\n%s", got)
	}
}

func TestFormatEnumDecl(t *testing.T) {
	src := `module test version "1.0";
enum Color {
    Red,
    Green,
    Blue,
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "    Red,\n") {
		t.Errorf("expected enum variant with trailing comma, got:\n%s", got)
	}
}

func TestFormatEnumWithFields(t *testing.T) {
	src := `module test version "1.0";
enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float),
    Point,
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "Circle(radius: Float),") {
		t.Errorf("expected data variant, got:\n%s", got)
	}
}

func TestFormatMatchExpr(t *testing.T) {
	src := `module test version "1.0";
enum Shape {
    Circle(radius: Float),
    Point,
}
entry function main() returns Int {
    let s: Shape = Point;
    let code: Int = match s {
        Circle(r) => 1,
        Point => 0
    };
    return code;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "match s {") {
		t.Errorf("expected match expr, got:\n%s", got)
	}
	if !strings.Contains(got, "Circle(r) => 1,") {
		t.Errorf("expected match arm, got:\n%s", got)
	}
}

func TestFormatTryExpr(t *testing.T) {
	src := `module test version "1.0";
function parse(s: String) returns Result<Int, String>
    requires true
    ensures true
{
    return Ok(42);
}
function use_parse(s: String) returns Result<Int, String>
    requires true
    ensures true
{
    let x: Int = parse(s)?;
    return Ok(x);
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "parse(s)?") {
		t.Errorf("expected try expr, got:\n%s", got)
	}
}

func TestFormatOldExpr(t *testing.T) {
	src := `module test version "1.0";
entity Counter {
    field count: Int;

    constructor(n: Int) {
        self.count = n;
    }

    method inc() returns Void
        ensures self.count == old(self.count) + 1
    {
        self.count = self.count + 1;
    }
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "old(self.count)") {
		t.Errorf("expected old expr, got:\n%s", got)
	}
}

func TestFormatForallExpr(t *testing.T) {
	src := `module test version "1.0";
function check(arr: Array<Int>) returns Bool
    requires forall i in 0..len(arr): arr[i] > 0
{
    return true;
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "forall i in 0..len(arr): arr[i] > 0") {
		t.Errorf("expected forall expr, got:\n%s", got)
	}
}

func TestFormatIntentDecl(t *testing.T) {
	src := `module test version "1.0";
intent "Test intent" {
    goal: "something";
    constraint: "another";
    guarantee: "third";
    verified_by: [Foo.bar];
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, `intent "Test intent" {`) {
		t.Errorf("expected intent decl, got:\n%s", got)
	}
	if !strings.Contains(got, `goal: "something";`) {
		t.Errorf("expected goal, got:\n%s", got)
	}
}

func TestFormatPublicFunction(t *testing.T) {
	src := `module test version "1.0";
public function add(a: Int, b: Int) returns Int { return a + b; }
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "public function add") {
		t.Errorf("expected public keyword, got:\n%s", got)
	}
}

func TestFormatEntryFunction(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "entry function main()") {
		t.Errorf("expected entry keyword, got:\n%s", got)
	}
}

func TestFormatSelfExpr(t *testing.T) {
	src := `module test version "1.0";
entity Foo {
    field x: Int;
    constructor(val: Int) { self.x = val; }
    method get() returns Int { return self.x; }
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "self.x") {
		t.Errorf("expected self expr, got:\n%s", got)
	}
}

func TestFormatMethodCall(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let mutable arr: Array<Int> = [1, 2];
    arr.push(3);
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "arr.push(3)") {
		t.Errorf("expected method call, got:\n%s", got)
	}
}

func TestFormatTypeRef(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let arr: Array<Int> = [1];
    let res: Result<Int, String> = Ok(1);
    let opt: Option<Int> = Some(1);
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "Array<Int>") {
		t.Errorf("expected Array<Int>, got:\n%s", got)
	}
	if !strings.Contains(got, "Result<Int, String>") {
		t.Errorf("expected Result<Int, String>, got:\n%s", got)
	}
	if !strings.Contains(got, "Option<Int>") {
		t.Errorf("expected Option<Int>, got:\n%s", got)
	}
}

// --- Idempotency tests ---

func TestIdempotency(t *testing.T) {
	sources := []struct {
		name   string
		source string
	}{
		{"hello", `module hello version "1.0.0";
entry function main() returns Int { return 0; }
`},
		{"with_contracts", `module test version "1.0";
function fib(n: Int) returns Int
    requires n >= 0
    ensures result >= 0
{
    return n;
}
entry function main() returns Int { return 0; }
`},
		{"entity", `module test version "1.0";
entity Foo {
    field x: Int;
    invariant self.x >= 0;
    constructor(val: Int)
        requires val >= 0
    {
        self.x = val;
    }
    method get() returns Int {
        return self.x;
    }
}
entry function main() returns Int { return 0; }
`},
		{"enum_match", `module test version "1.0";
enum Color { Red, Green, Blue, }
entry function main() returns Int {
    let c: Color = Red;
    let code: Int = match c {
        Red => 1,
        Green => 2,
        Blue => 3
    };
    return code;
}
`},
	}

	for _, tc := range sources {
		t.Run(tc.name, func(t *testing.T) {
			first := formatSource(t, tc.source)
			second := formatSource(t, first)
			if first != second {
				t.Errorf("format is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}

// --- Round-trip test ---

func TestRoundTrip(t *testing.T) {
	// Parse -> format -> parse again -> format again -> should be identical
	src := `module test version "1.0";

enum Shape {
    Circle(radius: Float),
    Point,
}

entity Counter {
    field count: Int;

    invariant self.count >= 0;

    constructor(n: Int)
        requires n >= 0
    {
        self.count = n;
    }

    method inc() returns Void {
        self.count = self.count + 1;
    }
}

function add(a: Int, b: Int) returns Int
    requires a >= 0
    ensures result >= a
{
    return a + b;
}

entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 {
        i = i + 1;
    }
    if i == 10 {
        return 0;
    } else {
        return 1;
    }
}

intent "Test intent" {
    goal: "something";
    verified_by: [Counter.invariant];
}
`
	first := formatSource(t, src)
	second := formatSource(t, first)
	if first != second {
		t.Errorf("round-trip not stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// --- Trailing newline test ---

func TestTrailingNewline(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.HasSuffix(got, "\n") {
		t.Error("expected trailing newline")
	}
}

// --- Canonical ordering test ---

func TestCanonicalOrdering(t *testing.T) {
	// Source has function before enum; formatter should reorder
	src := `module test version "1.0";
function add(a: Int, b: Int) returns Int { return a + b; }
enum Status { Active, Inactive, }
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	enumIdx := strings.Index(got, "enum Status")
	funcIdx := strings.Index(got, "function add")
	if enumIdx > funcIdx {
		t.Errorf("expected enum before function in canonical ordering, got:\n%s", got)
	}
}

// --- Indentation test ---

func TestIndentation(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
if true {
if false {
return 1;
}
}
return 0;
}
`
	got := formatSource(t, src)
	// The inner return should be indented 3 levels (12 spaces)
	if !strings.Contains(got, "            return 1;") {
		t.Errorf("expected proper indentation, got:\n%s", got)
	}
}

// --- Operator spacing test ---

func TestOperatorSpacing(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 1+2;
    let y: Bool = true and false;
    return x;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "1 + 2") {
		t.Errorf("expected spaces around +, got:\n%s", got)
	}
	if !strings.Contains(got, "true and false") {
		t.Errorf("expected spaces around and, got:\n%s", got)
	}
}

// --- Precedence-based parenthesization ---

func TestPrecedenceParens(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = (1 + 2) * 3;
    return x;
}
`
	got := formatSource(t, src)
	// (1 + 2) * 3 -- the parser produces BinaryExpr(*, BinaryExpr(+, 1, 2), 3)
	// Since + has lower precedence than *, the formatter must add parens
	if !strings.Contains(got, "(1 + 2) * 3") {
		t.Errorf("expected parens around lower-precedence sub-expr, got:\n%s", got)
	}
}

func TestNoPrecedenceParensWhenUnnecessary(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 1 + 2 * 3;
    return x;
}
`
	got := formatSource(t, src)
	// 2 * 3 has higher precedence, so no parens needed
	if strings.Contains(got, "(2 * 3)") {
		t.Errorf("expected no unnecessary parens, got:\n%s", got)
	}
}

// --- While with invariants ---

func TestFormatWhileWithInvariant(t *testing.T) {
	src := `module test version "1.0";
function loopy(n: Int) returns Int
    requires n >= 0
{
    let mutable i: Int = 0;
    let mutable sum: Int = 0;
    while i < n
        invariant sum >= 0
        decreases n - i
    {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}
entry function main() returns Int { return 0; }
`
	got := formatSource(t, src)
	if !strings.Contains(got, "while i < n\n") {
		t.Errorf("expected while on its own line when invariants present, got:\n%s", got)
	}
	if !strings.Contains(got, "    invariant sum >= 0\n") || !strings.Contains(got, "    decreases n - i\n") {
		t.Errorf("expected indented invariant/decreases, got:\n%s", got)
	}
}

// --- Else-if chain ---

func TestFormatElseIf(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 5;
    if x > 10 {
        return 2;
    } else if x > 0 {
        return 1;
    } else {
        return 0;
    }
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "} else if x > 0 {") {
		t.Errorf("expected else-if on same line, got:\n%s", got)
	}
	if !strings.Contains(got, "} else {") {
		t.Errorf("expected else on same line, got:\n%s", got)
	}
}

// --- ExprStmt ---

func TestFormatExprStmt(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    print(42);
    return 0;
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "    print(42);") {
		t.Errorf("expected expr stmt, got:\n%s", got)
	}
}
