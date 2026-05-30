package ir

import (
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/parser"
)

func parseAndLower(t *testing.T, src string) *Module {
	t.Helper()
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("test"))
	}
	result := checker.CheckWithResult(prog)
	if result.Diagnostics.HasErrors() {
		t.Fatalf("check errors: %s", result.Diagnostics.Format("test"))
	}
	return Lower(prog, result)
}

func TestLowerSimpleFunction(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    return 42;
}
`
	mod := parseAndLower(t, src)
	if len(mod.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(mod.Functions))
	}
	fn := mod.Functions[0]
	if fn.Name != "main" {
		t.Errorf("expected function name 'main', got %q", fn.Name)
	}
	if !fn.IsEntry {
		t.Error("expected IsEntry=true")
	}
	if fn.ReturnType == nil || fn.ReturnType.Name != "Int" {
		t.Error("expected return type Int")
	}
}

func TestLowerStringConcat(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let s: String = "hello" + " world";
    return 0;
}
`
	mod := parseAndLower(t, src)
	fn := mod.Functions[0]
	if len(fn.Body) < 1 {
		t.Fatal("expected at least 1 statement")
	}
	letStmt, ok := fn.Body[0].(*LetStmt)
	if !ok {
		t.Fatalf("expected LetStmt, got %T", fn.Body[0])
	}
	if _, ok := letStmt.Value.(*StringConcat); !ok {
		t.Errorf("expected StringConcat, got %T", letStmt.Value)
	}
}

func TestLowerOldCaptures(t *testing.T) {
	src := `module test version "1.0";
entity Counter {
    field count: Int;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.count == initial
    {
        self.count = initial;
    }

    method increment() returns Void
        ensures self.count == old(self.count) + 1
    {
        self.count = self.count + 1;
    }
}
entry function main() returns Int {
    return 0;
}
`
	mod := parseAndLower(t, src)
	if len(mod.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(mod.Entities))
	}
	ent := mod.Entities[0]
	if len(ent.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(ent.Methods))
	}
	method := ent.Methods[0]

	// Should have old() captures
	if len(method.OldCaptures) != 1 {
		t.Fatalf("expected 1 old capture, got %d", len(method.OldCaptures))
	}
	cap := method.OldCaptures[0]
	if cap.Name != "__old_self_count" {
		t.Errorf("expected old capture name '__old_self_count', got %q", cap.Name)
	}

	// Ensures should reference OldRef
	if len(method.Ensures) != 1 {
		t.Fatalf("expected 1 ensures clause, got %d", len(method.Ensures))
	}
	ensExpr := method.Ensures[0].Expr
	binExpr, ok := ensExpr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ensExpr)
	}
	rightBin, ok := binExpr.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr on right, got %T", binExpr.Right)
	}
	if _, ok := rightBin.Left.(*OldRef); !ok {
		t.Errorf("expected OldRef on right.left, got %T", rightBin.Left)
	}
}

func TestLowerCallResolution(t *testing.T) {
	src := `module test version "1.0";
enum Color {
    Red,
    Green,
    Blue,
}
function helper(x: Int) returns Int {
    return x + 1;
}
entity Box {
    field size: Int;
    constructor(s: Int)
        requires s > 0
        ensures self.size == s
    {
        self.size = s;
    }
}
entry function main() returns Int {
    let c: Color = Red;
    let x: Int = helper(5);
    let b: Box = Box(10);
    print(x);
    let n: Int = len([1, 2, 3]);
    return 0;
}
`
	mod := parseAndLower(t, src)

	var mainFn *Function
	for _, f := range mod.Functions {
		if f.Name == "main" {
			mainFn = f
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// let c: Color = Red -> CallExpr with CallVariant
	letC := mainFn.Body[0].(*LetStmt)
	callC, ok := letC.Value.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr for Red, got %T", letC.Value)
	}
	if callC.Kind != CallVariant {
		t.Errorf("expected CallVariant for Red, got %v", callC.Kind)
	}
	if callC.EnumName != "Color" {
		t.Errorf("expected EnumName 'Color', got %q", callC.EnumName)
	}

	// let x: Int = helper(5) -> CallExpr with CallFunction
	letX := mainFn.Body[1].(*LetStmt)
	callX, ok := letX.Value.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr for helper, got %T", letX.Value)
	}
	if callX.Kind != CallFunction {
		t.Errorf("expected CallFunction for helper, got %v", callX.Kind)
	}

	// let b: Box = Box(10) -> CallExpr with CallConstructor
	letB := mainFn.Body[2].(*LetStmt)
	callB, ok := letB.Value.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr for Box, got %T", letB.Value)
	}
	if callB.Kind != CallConstructor {
		t.Errorf("expected CallConstructor for Box, got %v", callB.Kind)
	}

	// print(x) -> CallExpr with CallBuiltin
	exprStmt := mainFn.Body[3].(*ExprStmt)
	callPrint, ok := exprStmt.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr for print, got %T", exprStmt.Expr)
	}
	if callPrint.Kind != CallBuiltin {
		t.Errorf("expected CallBuiltin for print, got %v", callPrint.Kind)
	}

	// len([1,2,3]) -> CallExpr with CallBuiltin
	letN := mainFn.Body[4].(*LetStmt)
	callLen, ok := letN.Value.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr for len, got %T", letN.Value)
	}
	if callLen.Kind != CallBuiltin {
		t.Errorf("expected CallBuiltin for len, got %v", callLen.Kind)
	}
}

func TestLowerMatchExpr(t *testing.T) {
	src := `module test version "1.0";
enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float),
}
entry function main() returns Int {
    let s: Shape = Circle(5.0);
    let area: Float = match s {
        Circle(r) => r,
        Rectangle(w, h) => w,
    };
    return 0;
}
`
	mod := parseAndLower(t, src)
	mainFn := mod.Functions[0]

	letArea := mainFn.Body[1].(*LetStmt)
	matchExpr, ok := letArea.Value.(*MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", letArea.Value)
	}

	if len(matchExpr.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(matchExpr.Arms))
	}

	arm0 := matchExpr.Arms[0]
	if arm0.Pattern.EnumName != "Shape" {
		t.Errorf("expected EnumName 'Shape', got %q", arm0.Pattern.EnumName)
	}
	if arm0.Pattern.VariantName != "Circle" {
		t.Errorf("expected VariantName 'Circle', got %q", arm0.Pattern.VariantName)
	}
	if len(arm0.Pattern.FieldNames) != 1 || arm0.Pattern.FieldNames[0] != "radius" {
		t.Errorf("expected FieldNames ['radius'], got %v", arm0.Pattern.FieldNames)
	}
}

func TestLowerIntLitParsing(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    return 42;
}
`
	mod := parseAndLower(t, src)
	fn := mod.Functions[0]
	retStmt := fn.Body[0].(*ReturnStmt)
	intLit, ok := retStmt.Value.(*IntLit)
	if !ok {
		t.Fatalf("expected IntLit, got %T", retStmt.Value)
	}
	if intLit.Value != 42 {
		t.Errorf("expected 42, got %d", intLit.Value)
	}
}

func TestLowerTypesAttached(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let x: Int = 10;
    let y: Bool = true;
    let s: String = "hello";
    return x;
}
`
	mod := parseAndLower(t, src)
	fn := mod.Functions[0]

	letX := fn.Body[0].(*LetStmt)
	intLit, ok := letX.Value.(*IntLit)
	if !ok {
		t.Fatalf("expected IntLit, got %T", letX.Value)
	}
	if intLit.Type == nil || intLit.Type.Name != "Int" {
		t.Error("expected Int type on int literal")
	}

	letY := fn.Body[1].(*LetStmt)
	boolLit, ok := letY.Value.(*BoolLit)
	if !ok {
		t.Fatalf("expected BoolLit, got %T", letY.Value)
	}
	if boolLit.Type == nil || boolLit.Type.Name != "Bool" {
		t.Error("expected Bool type on bool literal")
	}
}

func TestLowerAsyncFunction(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
entry function main() returns Int {
    return 0;
}
`
	mod := parseAndLower(t, src)

	var asyncFn *Function
	for _, f := range mod.Functions {
		if f.Name == "fetchData" {
			asyncFn = f
			break
		}
	}
	if asyncFn == nil {
		t.Fatal("fetchData function not found")
	}
	if !asyncFn.IsAsync {
		t.Error("expected IsAsync=true for fetchData")
	}
	if asyncFn.ReturnType == nil || asyncFn.ReturnType.Name != "Future" {
		t.Error("expected return type Future")
	}

	// main should not be async
	for _, f := range mod.Functions {
		if f.Name == "main" {
			if f.IsAsync {
				t.Error("expected IsAsync=false for main")
			}
			break
		}
	}
}

func TestLowerAwaitExpr(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
async function main() returns Future<Int> {
    let f: Future<Int> = spawn fetchData();
    let val: Int = await f;
    return val;
}
`
	mod := parseAndLower(t, src)

	var mainFn *Function
	for _, f := range mod.Functions {
		if f.Name == "main" {
			mainFn = f
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}
	if !mainFn.IsAsync {
		t.Error("expected IsAsync=true for main")
	}

	// Statement 1: let result: Int = await f;
	if len(mainFn.Body) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(mainFn.Body))
	}
	letResult, ok := mainFn.Body[1].(*LetStmt)
	if !ok {
		t.Fatalf("expected LetStmt for result, got %T", mainFn.Body[1])
	}
	awaitExpr, ok := letResult.Value.(*AwaitExpr)
	if !ok {
		t.Fatalf("expected AwaitExpr, got %T", letResult.Value)
	}
	if awaitExpr.Type == nil || awaitExpr.Type.Name != "Int" {
		t.Errorf("expected await type Int, got %v", awaitExpr.Type)
	}
	if awaitExpr.Expr == nil {
		t.Error("expected non-nil inner expression in AwaitExpr")
	}
}

func TestScanInstantiationsInMatchExpr(t *testing.T) {
	src := `module test version "1.0";
entity Stack<T> {
    field count: Int;
    constructor()
        ensures self.count == 0
    {
        self.count = 0;
    }
}
enum Choice {
    A,
    B,
}
entry function main() returns Int {
    let c: Choice = A;
    let s: Stack<Int> = match c {
        A => Stack<Int>(),
        B => Stack<Int>(),
    };
    return s.count;
}
`
	mod := parseAndLower(t, src)

	// The monomorphized entity Stack__Int should be generated
	found := false
	for _, e := range mod.Entities {
		if e.Name == "Stack__Int" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(mod.Entities))
		for i, e := range mod.Entities {
			names[i] = e.Name
		}
		t.Errorf("expected monomorphized entity Stack__Int, got entities: %v", names)
	}
}

func TestLowerSpawnExpr(t *testing.T) {
	src := `module test version "1.0";
async function compute(x: Int) returns Future<Int> {
    return x * 2;
}
async function main() returns Future<Int> {
    let handle: Future<Int> = spawn compute(42);
    let val: Int = await handle;
    return val;
}
`
	mod := parseAndLower(t, src)

	var mainFn *Function
	for _, f := range mod.Functions {
		if f.Name == "main" {
			mainFn = f
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Statement 0: let handle: Future<Int> = spawn compute(42);
	letHandle, ok := mainFn.Body[0].(*LetStmt)
	if !ok {
		t.Fatalf("expected LetStmt for handle, got %T", mainFn.Body[0])
	}
	spawnExpr, ok := letHandle.Value.(*SpawnExpr)
	if !ok {
		t.Fatalf("expected SpawnExpr, got %T", letHandle.Value)
	}
	if spawnExpr.Type == nil || spawnExpr.Type.Name != "Future" {
		t.Errorf("expected spawn type Future, got %v", spawnExpr.Type)
	}
	if spawnExpr.Expr == nil {
		t.Error("expected non-nil inner expression in SpawnExpr")
	}
	// The inner expression should be a CallExpr
	if _, ok := spawnExpr.Expr.(*CallExpr); !ok {
		t.Errorf("expected CallExpr inside SpawnExpr, got %T", spawnExpr.Expr)
	}
}

func TestMonomorphizeMismatchedTypeArgs(t *testing.T) {
	l := &lowerer{
		exprTypes:          make(map[ast.Expression]*checker.Type),
		entities:           make(map[string]*checker.EntityInfo),
		enums:              make(map[string]*checker.EnumInfo),
		genericEntityDecls: make(map[string]*ast.EntityDecl),
		genericFuncDecls:   make(map[string]*ast.FunctionDecl),
		instantiations:     make(map[string][][]*checker.Type),
		funcInstantiations: make(map[string][][]*checker.Type),
	}

	// Entity with 2 type params but only 1 type arg
	entityDecl := &ast.EntityDecl{
		Name:       "Pair",
		TypeParams: []*ast.TypeParam{{Name: "T"}, {Name: "U"}},
	}
	result := l.monomorphizeEntity(entityDecl, []*checker.Type{checker.TypeInt})
	if result != nil {
		t.Error("expected nil from monomorphizeEntity with mismatched type arg count")
	}

	// Function with 1 type param but 0 type args
	funcDecl := &ast.FunctionDecl{
		Name:       "identity",
		TypeParams: []*ast.TypeParam{{Name: "T"}},
	}
	fnResult := l.monomorphizeFunction(funcDecl, []*checker.Type{})
	if fnResult != nil {
		t.Error("expected nil from monomorphizeFunction with mismatched type arg count")
	}

	// Function with 1 type param but 2 type args (too many)
	fnResult2 := l.monomorphizeFunction(funcDecl, []*checker.Type{checker.TypeInt, checker.TypeString})
	if fnResult2 != nil {
		t.Error("expected nil from monomorphizeFunction with too many type args")
	}
}

// Regression: monomorphizing an entity whose body uses a type parameter
// (e.g. `field items: Array<T>`) must resolve T as a type parameter before
// substitution. Previously the lowerer called ResolveType without typeParams,
// so T failed to resolve, Array<T> collapsed to nil, and the Rust backend
// emitted `items: ()` (unit), breaking compilation.
func TestMonomorphizeUsesTypeParamsInBody(t *testing.T) {
	src := `module test version "1.0";
entity Box<T> {
    field value: T;
    field items: Array<T>;

    constructor(v: T)
        ensures self.value == v
    {
        self.value = v;
        self.items = [];
    }

    method get() returns T
        ensures result == self.value
    {
        return self.value;
    }
}

entry function main() returns Int {
    let b: Box<Int> = Box<Int>(42);
    return b.get();
}
`
	mod := parseAndLower(t, src)

	var box *Entity
	for _, e := range mod.Entities {
		if e.Name == "Box__Int" {
			box = e
			break
		}
	}
	if box == nil {
		t.Fatalf("expected monomorphized entity Box__Int, got %v", entityNames(mod))
	}

	// field value: T -> Int
	if got := box.Fields[0].Type; got == nil || got.Name != "Int" {
		t.Errorf("field 'value': expected Int, got %v", got)
	}

	// field items: Array<T> -> Array<Int>
	itemsType := box.Fields[1].Type
	if itemsType == nil || itemsType.Name != "Array" {
		t.Fatalf("field 'items': expected Array, got %v", itemsType)
	}
	if len(itemsType.TypeParams) != 1 || itemsType.TypeParams[0].Name != "Int" {
		t.Errorf("field 'items': expected Array<Int>, got Array<%v>", itemsType.TypeParams)
	}

	// constructor param v: T -> Int
	if got := box.Constructor.Params[0].Type; got == nil || got.Name != "Int" {
		t.Errorf("ctor param 'v': expected Int, got %v", got)
	}

	// method get() return type: T -> Int
	if got := box.Methods[0].ReturnType; got == nil || got.Name != "Int" {
		t.Errorf("method 'get' return: expected Int, got %v", got)
	}
}

func entityNames(mod *Module) []string {
	names := make([]string, len(mod.Entities))
	for i, e := range mod.Entities {
		names[i] = e.Name
	}
	return names
}

// Phase 16 / ADR 0029: in-language testing framework — IR lowering tests.

func TestLowerTestDecl(t *testing.T) {
	src := `module test version "1.0";

test "smoke" {
    let x: Int = 1;
    assert(x == 1);
}

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	if len(mod.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(mod.Tests))
	}
	tt := mod.Tests[0]
	if tt.Name != "smoke" {
		t.Errorf("expected name 'smoke', got %q", tt.Name)
	}
	if tt.IsAsync {
		t.Error("expected IsAsync=false")
	}
	if len(tt.Body) != 2 {
		t.Errorf("expected 2 body statements (let + assert), got %d", len(tt.Body))
	}
}

func TestLowerAsyncTestDecl(t *testing.T) {
	src := `module test version "1.0";

async function delayed() returns Future<Int> { return 1; }

async test "awaits" {
    let f: Future<Int> = spawn delayed();
    let r: Int = await f;
    assert(r == 1);
}

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	if len(mod.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(mod.Tests))
	}
	if !mod.Tests[0].IsAsync {
		t.Error("expected IsAsync=true on async test")
	}
}

// ADR 0031: lowering preserves @target_specific annotations.
func TestLowerAnnotatedTest(t *testing.T) {
	src := `module test version "1.0";

@target_specific("rust", "js")
test "annotated" { let x: Int = 1; }

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	if len(mod.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(mod.Tests))
	}
	tt := mod.Tests[0]
	if len(tt.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(tt.Annotations))
	}
	if tt.Annotations[0].Name != "target_specific" {
		t.Errorf("annotation name: got %q", tt.Annotations[0].Name)
	}
	if got := tt.TargetSpecificTargets(); len(got) != 2 || got[0] != "rust" || got[1] != "js" {
		t.Errorf("TargetSpecificTargets: got %v want [rust js]", got)
	}
	if !tt.RunsOnTarget("rust") {
		t.Error("expected RunsOnTarget(rust) true")
	}
	if tt.RunsOnTarget("wasm") {
		t.Error("expected RunsOnTarget(wasm) false")
	}
}

func TestLowerUnannotatedTestRunsEverywhere(t *testing.T) {
	src := `module test version "1.0";

test "everywhere" { let x: Int = 1; }

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	tt := mod.Tests[0]
	if len(tt.Annotations) != 0 {
		t.Errorf("expected no annotations, got %d", len(tt.Annotations))
	}
	if got := tt.TargetSpecificTargets(); got != nil {
		t.Errorf("expected nil targets, got %v", got)
	}
	if !tt.RunsOnTarget("rust") || !tt.RunsOnTarget("js") || !tt.RunsOnTarget("wasm") {
		t.Error("unannotated tests should run on every target")
	}
}

func TestLowerMultipleTests(t *testing.T) {
	src := `module test version "1.0";

test "one"   { let x: Int = 1; }
test "two"   { let y: Int = 2; }

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	if len(mod.Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(mod.Tests))
	}
	if mod.Tests[0].Name != "one" || mod.Tests[1].Name != "two" {
		t.Errorf("names: got %q, %q", mod.Tests[0].Name, mod.Tests[1].Name)
	}
}

func TestLowerEmptyTestBody(t *testing.T) {
	src := `module test version "1.0";

test "vacuous" {}

entry function main() returns Int { return 0; }
`
	mod := parseAndLower(t, src)
	if len(mod.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(mod.Tests))
	}
	if len(mod.Tests[0].Body) != 0 {
		t.Errorf("expected empty body, got %d stmts", len(mod.Tests[0].Body))
	}
}
