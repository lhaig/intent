package ir

import (
	"testing"

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
