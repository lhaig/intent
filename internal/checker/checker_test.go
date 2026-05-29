package checker

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/parser"
)

func parseAndCheck(t *testing.T, source string) *diagnostic.Diagnostics {
	t.Helper()
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}

	return Check(prog)
}

func TestValidProgramWithEntityAndContracts(t *testing.T) {
	source := `module test version "1.0.0";

entity BankAccount {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initialBalance: Int)
        requires initialBalance >= 0
        ensures self.balance == initialBalance
    {
        self.balance = initialBalance;
    }

    method withdraw(amount: Int) returns Int
        requires amount > 0
        requires amount <= self.balance
        ensures result >= 0
        ensures self.balance == old(self.balance) - amount
    {
        self.balance = self.balance - amount;
        return self.balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}

intent "Safe banking" {
    goal: "Account balance never goes negative";
    guarantee: "Balance invariant is maintained";
    verified_by: [BankAccount.invariant, BankAccount.withdraw.requires, BankAccount.withdraw.ensures, BankAccount.deposit.requires, BankAccount.deposit.ensures];
}
`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestUndeclaredVariable(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    return x;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for undeclared variable")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "undeclared variable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected undeclared variable error, got: %s", diag.Format("test"))
	}
}

func TestTypeMismatchInLetBinding(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let x: Int = "hello";
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for type mismatch")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "type mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type mismatch error, got: %s", diag.Format("test"))
	}
}

func TestAssignmentToImmutableVariable(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let x: Int = 42;
    x = 100;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for assignment to immutable variable")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "immutable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected immutable assignment error, got: %s", diag.Format("test"))
	}
}

func TestResultOutsideEnsuresClause(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let x: Int = result;
    return x;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for 'result' outside ensures clause")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "result") && strings.Contains(d.Message, "ensures") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'result' context error, got: %s", diag.Format("test"))
	}
}

func TestOldOutsideEnsuresClause(t *testing.T) {
	source := `module test version "1.0.0";

function test(x: Int) returns Int
    requires old(x) > 0
{
    return x;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for 'old()' outside ensures clause")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "old()") && strings.Contains(d.Message, "ensures") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'old()' context error, got: %s", diag.Format("test"))
	}
}

func TestSelfOutsideEntityContext(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    return self;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for 'self' outside entity context")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "self") && strings.Contains(d.Message, "entity") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'self' context error, got: %s", diag.Format("test"))
	}
}

func TestInvalidVerifiedByPath(t *testing.T) {
	source := `module test version "1.0.0";

entity Counter {
    field value: Int;
    constructor(v: Int) {
        self.value = v;
    }
}

intent "Test intent" {
    verified_by: Counter.nonexistent;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for invalid verified_by path")
	}
}

func TestUnknownEntityInVerifiedBy(t *testing.T) {
	source := `module test version "1.0.0";

intent "Test intent" {
    verified_by: UnknownEntity.invariant;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for unknown entity in verified_by")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "unknown entity") || strings.Contains(d.Message, "Unknown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected unknown entity error, got: %s", diag.Format("test"))
	}
}

func TestMethodCallOnNonEntity(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let x: Int = 42;
    x.someMethod();
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for method call on non-entity")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "non-entity") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected non-entity method call error, got: %s", diag.Format("test"))
	}
}

func TestMutableAssignment(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let mutable x: Int = 42;
    x = 100;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for mutable assignment, got: %s", diag.Format("test"))
	}
}

func TestEntityFieldAccess(t *testing.T) {
	source := `module test version "1.0.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x: Int, y: Int) {
        self.x = x;
        self.y = y;
    }

    method getX() returns Int {
        return self.x;
    }
}

function test() returns Int {
    let p: Point = Point(10, 20);
    return p.x;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for entity field access, got: %s", diag.Format("test"))
	}
}

func TestArithmeticOperations(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let x: Int = 10;
    let y: Int = 20;
    let sum: Int = x + y;
    let diff: Int = x - y;
    let prod: Int = x * y;
    let quot: Int = x / y;
    let mod: Int = x % y;
    return sum;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for arithmetic operations, got: %s", diag.Format("test"))
	}
}

func TestLogicalOperations(t *testing.T) {
	source := `module test version "1.0.0";

function test(a: Bool, b: Bool) returns Bool {
    let r1: Bool = a and b;
    let r2: Bool = a or b;
    let r3: Bool = not a;
    let r4: Bool = a implies b;
    return r1;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for logical operations, got: %s", diag.Format("test"))
	}
}

func TestInvalidArithmeticOperands(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let x: Int = 10;
    let y: String = "hello";
    return x + y;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for invalid arithmetic operands")
	}
}

func TestUnknownMethod(t *testing.T) {
	source := `module test version "1.0.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x: Int, y: Int) {
        self.x = x;
        self.y = y;
    }
}

function test() returns Void {
    let p: Point = Point(10, 20);
    p.unknownMethod();
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for unknown method")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "no method") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected unknown method error, got: %s", diag.Format("test"))
	}
}

func TestUnknownField(t *testing.T) {
	source := `module test version "1.0.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x: Int, y: Int) {
        self.x = x;
        self.y = y;
    }
}

function test() returns Int {
    let p: Point = Point(10, 20);
    return p.z;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for unknown field")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "no field") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected unknown field error, got: %s", diag.Format("test"))
	}
}

func TestEntityMethodWithRequiresAndEnsures(t *testing.T) {
	source := `module test version "1.0.0";

entity Counter {
    field count: Int;

    invariant self.count >= 0;

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

intent "Counter safety" {
    verified_by: [Counter.invariant, Counter.increment.ensures];
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestVerifiedByMethodWithoutContract(t *testing.T) {
	source := `module test version "1.0.0";

entity Counter {
    field count: Int;

    constructor(initial: Int) {
        self.count = initial;
    }

    method increment() returns Void {
        self.count = self.count + 1;
    }
}

intent "Test" {
    verified_by: Counter.increment.requires;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for method without requires clause")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "no requires") || strings.Contains(d.Message, "requires") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'no requires' error, got: %s", diag.Format("test"))
	}
}

func TestWhileLoopTypeCheck(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        x = x + 1;
    }
    return x;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestWhileLoopNonBoolCondition(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    while 42 {
        return 0;
    }
    return 1;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for non-boolean while condition")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "while condition must be boolean") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'while condition must be boolean' error, got: %s", diag.Format("test"))
	}
}

func TestBreakInsideLoop(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        if x == 5 {
            break;
        }
        x = x + 1;
    }
    return x;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestBreakOutsideLoop(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    break;
    return 0;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for break outside loop")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "break statement outside loop") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'break statement outside loop' error, got: %s", diag.Format("test"))
	}
}

func TestContinueInsideLoop(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    let mutable count: Int = 0;
    while x < 10 {
        x = x + 1;
        if x == 5 {
            continue;
        }
        count = count + 1;
    }
    return count;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestContinueOutsideLoop(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    continue;
    return 0;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for continue outside loop")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "continue statement outside loop") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'continue statement outside loop' error, got: %s", diag.Format("test"))
	}
}

func TestNestedLoopBreak(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 3 {
        let mutable j: Int = 0;
        while j < 3 {
            if j == 1 {
                break;
            }
            j = j + 1;
        }
        i = i + 1;
    }
    return i;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestWhileWithVariableScope(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 5 {
        let y: Int = x * 2;
        x = x + 1;
    }
    return x;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintInt(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(42);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintFloat(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(3.14);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintBool(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(true);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintString(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print("hello");
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintVariable(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let x: Int = 5;
    print(x);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintExpression(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(2 + 3);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestPrintNoArgs(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print();
    return 0;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for print() with no arguments")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "print() expects 1 argument, got 0") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'print() expects 1 argument, got 0' error, got: %s", diag.Format("test"))
	}
}

func TestPrintTwoArgs(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(1, 2);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for print() with two arguments")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "print() expects 1 argument, got 2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'print() expects 1 argument, got 2' error, got: %s", diag.Format("test"))
	}
}

func TestPrintEntityType(t *testing.T) {
	source := `module test version "1.0.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x: Int, y: Int) {
        self.x = x;
        self.y = y;
    }
}

entry function main() returns Int {
    let p: Point = Point(1, 2);
    print(p);
    return 0;
}`
	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for print() with entity type")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "print() cannot print type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'print() cannot print type' error, got: %s", diag.Format("test"))
	}
}

func TestResolveArrayType(t *testing.T) {
	source := `module test version "1.0.0";

function test(arr: Array<Int>) returns Int {
    let x: Array<Int> = arr;
    return 0;
}`

	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}

	result := CheckWithResult(prog)

	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors, got: %s", result.Diagnostics.Format("test"))
	}

	// Find the let statement and check its resolved type
	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)

	// Get the parameter type to verify Array<Int> resolution
	paramType := result.Entities
	_ = paramType // Type checking validates Array<Int> properly resolves

	// Verify exprTypes map is populated
	if len(result.ExprTypes) == 0 {
		t.Error("Expected exprTypes map to be populated")
	}

	// Get the identifier type from exprTypes
	identExpr := letStmt.Value.(*ast.Identifier)
	exprType := result.ExprTypes[identExpr]
	if exprType == nil {
		t.Error("Expected identifier expression to have type in exprTypes map")
	} else if !exprType.IsGeneric || exprType.Name != "Array" {
		t.Errorf("Expected Array<Int> type, got: %s", exprType.String())
	} else if len(exprType.TypeParams) != 1 || !exprType.TypeParams[0].Equal(TypeInt) {
		t.Errorf("Expected Array<Int> with Int type parameter, got: %s", exprType.String())
	}
}

func TestArrayTypeEquality(t *testing.T) {
	arrayIntType1 := &Type{
		Name:       "Array",
		IsGeneric:  true,
		TypeParams: []*Type{TypeInt},
	}

	arrayIntType2 := &Type{
		Name:       "Array",
		IsGeneric:  true,
		TypeParams: []*Type{TypeInt},
	}

	arrayStringType := &Type{
		Name:       "Array",
		IsGeneric:  true,
		TypeParams: []*Type{TypeString},
	}

	if !arrayIntType1.Equal(arrayIntType2) {
		t.Error("Array<Int> should equal Array<Int>")
	}

	if arrayIntType1.Equal(arrayStringType) {
		t.Error("Array<Int> should not equal Array<String>")
	}
}

func TestNestedArrayTypeEquality(t *testing.T) {
	arrayArrayInt1 := &Type{
		Name:      "Array",
		IsGeneric: true,
		TypeParams: []*Type{
			{
				Name:       "Array",
				IsGeneric:  true,
				TypeParams: []*Type{TypeInt},
			},
		},
	}

	arrayArrayInt2 := &Type{
		Name:      "Array",
		IsGeneric: true,
		TypeParams: []*Type{
			{
				Name:       "Array",
				IsGeneric:  true,
				TypeParams: []*Type{TypeInt},
			},
		},
	}

	if !arrayArrayInt1.Equal(arrayArrayInt2) {
		t.Error("Array<Array<Int>> should equal Array<Array<Int>>")
	}
}

func TestArrayMissingTypeArg(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let x: Array = 42;
    return 0;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for Array without type argument")
	}

	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "unknown type 'Array'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'unknown type' error for Array without type argument, got: %s", diag.Format("test"))
	}
}

func TestGetExprType(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let x: Int = 42;
    let y: String = "hello";
    return x;
}`

	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}

	result := CheckWithResult(prog)

	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors, got: %s", result.Diagnostics.Format("test"))
	}

	// Verify exprTypes map is populated
	if len(result.ExprTypes) == 0 {
		t.Error("Expected exprTypes map to be populated")
	}

	// Check that we can find integer and string literal types
	foundInt := false
	foundString := false

	for _, exprType := range result.ExprTypes {
		if exprType.Equal(TypeInt) {
			foundInt = true
		}
		if exprType.Equal(TypeString) {
			foundString = true
		}
	}

	if !foundInt {
		t.Error("Expected to find Int type in exprTypes")
	}
	if !foundString {
		t.Error("Expected to find String type in exprTypes")
	}
}

func TestCheckArrayLiteral(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [1, 2, 3];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckMixedArrayLiteral(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [1, true, 3];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for mixed-type array literal")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "array element type mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'array element type mismatch' error, got: %s", diag.Format("test"))
	}
}

func TestCheckEmptyArrayLiteral(t *testing.T) {
	// Empty array with type annotation should be accepted
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Empty array with Array<Int> annotation should be accepted, got: %s", diag.Format("test"))
	}
}

func TestCheckEmptyArrayLiteralNoArrayAnnotation(t *testing.T) {
	// Empty array assigned to non-Array type should still error
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = [];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for empty array literal with non-Array type annotation")
	}
}

func TestCheckIndexExpr(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    let y: Int = arr[0];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckIndexNonArray(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = 5;
    let y: Int = x[0];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for indexing non-array")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "cannot index into non-array") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'cannot index into non-array' error, got: %s", diag.Format("test"))
	}
}

func TestCheckIndexNonInt(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    let y: Int = arr[true];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for non-Int index")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "array index must be Int") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'array index must be Int' error, got: %s", diag.Format("test"))
	}
}

func TestCheckLen(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    let n: Int = len(arr);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckLenNonArray(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let n: Int = len(42);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for len() on non-array")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "len() requires Array or Map argument") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'len() requires Array or Map argument' error, got: %s", diag.Format("test"))
	}
}

func TestCheckPush(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable arr: Array<Int> = [1, 2, 3];
    arr.push(5);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckPushImmutable(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    arr.push(5);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for push() on immutable array")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "cannot call push() on immutable array") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'cannot call push() on immutable array' error, got: %s", diag.Format("test"))
	}
}

func TestCheckPushTypeMismatch(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable arr: Array<Int> = [1, 2, 3];
    arr.push(true);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for push() type mismatch")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "push() argument type mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'push() argument type mismatch' error, got: %s", diag.Format("test"))
	}
}

func TestCheckLenInContract(t *testing.T) {
	source := `module test version "1.0.0";
function process(arr: Array<Int>, i: Int) returns Int
    requires i >= 0 and i < len(arr)
{
    return arr[i];
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckForInArray(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        print(x);
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckForInRange(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    for i in 0..10 {
        print(i);
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckForInNonArray(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Int = 42;
    for i in x {
        print(i);
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for iterating over non-array")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "cannot iterate over type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'cannot iterate over type' error, got: %s", diag.Format("test"))
	}
}

func TestCheckForInRangeNonInt(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    for i in true..10 {
        print(i);
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for range with non-Int start")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "range start must be Int") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'range start must be Int' error, got: %s", diag.Format("test"))
	}
}

func TestCheckForInVariableScope(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        print(x);
    }
    print(x);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for using loop variable outside for-in body")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "undeclared variable 'x'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'undeclared variable' error, got: %s", diag.Format("test"))
	}
}

func TestCheckForInBreakContinue(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        if x == 2 {
            break;
        }
        continue;
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for break/continue in for-in, got: %s", diag.Format("test"))
	}
}

func TestWhileInvariantBoolCheck(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 invariant i >= 0 {
        i = i + 1;
    }
    return i;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for boolean invariant, got: %s", diag.Format("test"))
	}
}

func TestWhileInvariantNonBool(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 invariant i {
        i = i + 1;
    }
    return i;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for non-Bool invariant expression")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "loop invariant must be boolean") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'loop invariant must be boolean' error, got: %s", diag.Format("test"))
	}
}

func TestWhileDecreasesIntCheck(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable n: Int = 10;
    while n > 0 decreases n {
        n = n - 1;
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for Int decreases metric, got: %s", diag.Format("test"))
	}
}

func TestWhileDecreasesNonInt(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable n: Bool = true;
    while n decreases n {
        n = false;
    }
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for non-Int decreases metric")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "decreases metric must be Int") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'decreases metric must be Int' error, got: %s", diag.Format("test"))
	}
}

func TestWhileMultipleInvariants(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    let mutable sum: Int = 0;
    while i < 10 invariant i >= 0 invariant sum >= 0 {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for multiple invariants, got: %s", diag.Format("test"))
	}
}

func TestWhileInvariantWithOld(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable sum: Int = 0;
    let mutable i: Int = 0;
    while i < 10 invariant sum >= old(sum) {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for old() in invariant, got: %s", diag.Format("test"))
	}
}

func TestCheckForallInEnsures(t *testing.T) {
	source := `module test version "1.0.0";

function check_positive(arr: Array<Int>, n: Int) returns Bool
    ensures forall i in 0..n: arr[i] >= 0
{
    return true;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for forall in ensures, got: %s", diag.Format("test"))
	}
}

func TestCheckForallInRequires(t *testing.T) {
	source := `module test version "1.0.0";

function use_positive(arr: Array<Int>, n: Int) returns Bool
    requires forall i in 0..n: arr[i] > 0
{
    return true;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for forall in requires, got: %s", diag.Format("test"))
	}
}

func TestCheckForallNonBoolBody(t *testing.T) {
	source := `module test version "1.0.0";

function check(arr: Array<Int>, n: Int) returns Bool
    ensures forall i in 0..n: arr[i]
{
    return true;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for forall with non-Bool body")
	}
	found := false
	for _, msg := range diag.Errors() {
		if msg.Message == "forall body must be boolean, got Int" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'forall body must be boolean' error, got: %s", diag.Format("test"))
	}
}

func TestCheckForallInNormalCode(t *testing.T) {
	source := `module test version "1.0.0";

function check(arr: Array<Int>, n: Int) returns Bool {
    let x: Bool = forall i in 0..n: arr[i] > 0;
    return x;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for forall outside contract context")
	}
	found := false
	for _, msg := range diag.Errors() {
		if msg.Message == "forall quantifier only allowed in contract expressions (requires, ensures, invariant)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'forall quantifier only allowed in contract expressions' error, got: %s", diag.Format("test"))
	}
}

func TestCheckExistsInEnsures(t *testing.T) {
	source := `module test version "1.0.0";

function find_target(arr: Array<Int>, n: Int, target: Int) returns Bool
    ensures exists i in 0..n: arr[i] == target
{
    return false;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for exists in ensures, got: %s", diag.Format("test"))
	}
}

func TestCheckExistsNonBoolBody(t *testing.T) {
	source := `module test version "1.0.0";

function find(arr: Array<Int>, n: Int) returns Bool
    requires exists i in 0..n: arr[i]
{
    return false;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for exists with non-Bool body")
	}
	found := false
	for _, msg := range diag.Errors() {
		if msg.Message == "exists body must be boolean, got Int" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'exists body must be boolean' error, got: %s", diag.Format("test"))
	}
}

func TestCheckForallRangeNonInt(t *testing.T) {
	source := `module test version "1.0.0";

function check(arr: Array<Int>) returns Bool
    ensures forall i in true..false: arr[i] > 0
{
    return true;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for forall with non-Int range bounds")
	}
	found := false
	for _, msg := range diag.Errors() {
		if msg.Message == "quantifier range start must be Int, got Bool" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'quantifier range start must be Int' error, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumSimpleVariants(t *testing.T) {
	source := `module test version "1.0.0";

enum Status {
	Pending,
	Running,
	Complete,
}

function getStatus() returns Status {
	return Running;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumDataVariants(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
	Rectangle(width: Float, height: Float),
	Point,
}

function makeCircle(r: Float) returns Shape {
	return Circle(r);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumVariantArgCountMismatch(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
}

function test() returns Shape {
	return Circle(1.0, 2.0);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for wrong argument count")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "variant 'Circle' expects 1 arguments, got 2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument count mismatch error, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumVariantArgTypeMismatch(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
}

function test() returns Shape {
	return Circle("hello");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for wrong argument type")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "expects Float, got String") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument type mismatch error, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumUnitVariantAsIdentifier(t *testing.T) {
	source := `module test version "1.0.0";

enum Status {
	Running,
	Complete,
}

function test() returns Status {
	let s: Status = Running;
	return s;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumDuplicateName(t *testing.T) {
	source := `module test version "1.0.0";

enum Status {
	Pending,
}

enum Status {
	Running,
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for duplicate enum name")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "enum 'Status' already defined") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected duplicate enum name error, got: %s", diag.Format("test"))
	}
}

func TestCheckEnumDuplicateVariant(t *testing.T) {
	source := `module test version "1.0.0";

enum Result {
	Ok,
	Ok,
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for duplicate variant name")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "duplicate variant name 'Ok'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected duplicate variant name error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchExhaustive(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Pending => 0,
        Complete => 2
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("unexpected checker errors: %s", diag.Format("test"))
	}
}

func TestCheckMatchNonExhaustive(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Pending => 0
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for non-exhaustive match")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "non-exhaustive match") && strings.Contains(msg.Message, "Complete") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected non-exhaustive match error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchWithWildcard(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        _ => 0
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("unexpected checker errors: %s", diag.Format("test"))
	}
}

func TestCheckMatchWildcardCoversAll(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        _ => 0
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("unexpected checker errors: %s", diag.Format("test"))
	}
}

func TestCheckMatchArmTypeMismatch(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Pending => 2.5,
        Complete => 3
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for match arm type mismatch")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "type mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type mismatch error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchNonEnumScrutinee(t *testing.T) {
	input := `module test version "1.0.0";

function f(x: Int) returns Int {
    return match x {
        _ => 0
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for non-enum scrutinee")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "must be an enum type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected enum type error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchUnknownVariant(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Paused => 0,
        Pending => 2,
        Complete => 3
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for unknown variant")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "not a variant of enum") && strings.Contains(msg.Message, "Paused") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected unknown variant error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchDestructuringBindings(t *testing.T) {
	input := `module test version "1.0.0";

enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float)
}

function f(s: Shape) returns Float {
    return match s {
        Circle(r) => r,
        Rectangle(w, h) => w
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("unexpected checker errors: %s", diag.Format("test"))
	}
}

func TestCheckMatchWrongBindingCount(t *testing.T) {
	input := `module test version "1.0.0";

enum Shape {
    Circle(radius: Float)
}

function f(s: Shape) returns Float {
    return match s {
        Circle(r, x) => r
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for wrong binding count")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "has 1 fields but pattern has 2 bindings") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected binding count error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchUnreachableAfterWildcard(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        _ => 0,
        Complete => 2
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for unreachable pattern")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "unreachable pattern") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected unreachable pattern error, got: %s", diag.Format("test"))
	}
}

func TestCheckMatchDuplicateVariant(t *testing.T) {
	input := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Pending => 0,
        Running => 2,
        Complete => 3
    };
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for duplicate variant")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "duplicate match arm for variant 'Running'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected duplicate variant error, got: %s", diag.Format("test"))
	}
}

// Result and Option built-in enum tests

func TestResultTypeResolves(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Result<Int, String>
    requires true
    ensures true
{
    return Ok(42);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestOptionTypeResolves(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Option<Int>
    requires true
    ensures true
{
    return Some(42);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestResultVariantConstructors(t *testing.T) {
	input := `module test version "1.0.0";

function divide(a: Int, b: Int) returns Result<Int, String>
    requires true
    ensures true
{
    if b == 0 {
        return Err("division by zero");
    }
    return Ok(a / b);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestOptionVariantConstructors(t *testing.T) {
	input := `module test version "1.0.0";

function find_positive(x: Int) returns Option<Int>
    requires true
    ensures true
{
    if x > 0 {
        return Some(x);
    }
    return None;
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestResultMatchExhaustiveness(t *testing.T) {
	input := `module test version "1.0.0";

function test(r: Result<Int, String>) returns Int
    requires true
    ensures true
{
    return match r {
        Ok(v) => v,
        Err(e) => 0
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestOptionMatchExhaustiveness(t *testing.T) {
	input := `module test version "1.0.0";

function test(o: Option<Int>) returns Int
    requires true
    ensures true
{
    return match o {
        Some(v) => v,
        None => -1
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestResultWrongArgType(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Result<Int, String>
    requires true
    ensures true
{
    return Ok("hello");
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected type error for Ok with wrong argument type")
	}
	found := false
	for _, msg := range diag.Errors() {
		if strings.Contains(msg.Message, "type mismatch") || strings.Contains(msg.Message, "expected Int") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type mismatch error, got: %s", diag.Format("test"))
	}
}

func TestNestedResultType(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Result<Array<Int>, String>
    requires true
    ensures true
{
    let arr: Array<Int> = [1, 2, 3];
    return Ok(arr);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestResultWithLetAnnotation(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Int
    requires true
    ensures true
{
    let r: Result<Int, String> = Ok(42);
    return match r {
        Ok(v) => v,
        Err(e) => 0
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestOptionWithLetAnnotation(t *testing.T) {
	input := `module test version "1.0.0";

function test() returns Int
    requires true
    ensures true
{
    let o: Option<Int> = Some(42);
    return match o {
        Some(v) => v,
        None => -1
    };
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestTryExprOnResult(t *testing.T) {
	input := `module test version "1.0.0";

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
    let val: Int = parse(s)?;
    return Ok(val);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestTryExprOnOption(t *testing.T) {
	input := `module test version "1.0.0";

function get_value() returns Option<Int>
    requires true
    ensures true
{
    return Some(42);
}

function use_value() returns Option<Int>
    requires true
    ensures true
{
    let val: Int = get_value()?;
    return Some(val);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestTryExprWrongReturnType(t *testing.T) {
	input := `module test version "1.0.0";

function parse(s: String) returns Result<Int, String>
    requires true
    ensures true
{
    return Ok(42);
}

function use_parse(s: String) returns Int
    requires true
    ensures true
{
    let val: Int = parse(s)?;
    return val;
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for try operator in function not returning Result")
	}
	if !strings.Contains(diag.Format("test"), "try operator (?) on Result can only be used in a function returning Result") {
		t.Errorf("Expected specific error message, got:\n%s", diag.Format("test"))
	}
}

func TestTryExprOnNonResult(t *testing.T) {
	input := `module test version "1.0.0";

function get_int() returns Int
    requires true
    ensures true
{
    return 42;
}

function use_int() returns Result<Int, String>
    requires true
    ensures true
{
    let val: Int = get_int()?;
    return Ok(val);
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for try operator on non-Result type")
	}
	if !strings.Contains(diag.Format("test"), "try operator (?) requires Result or Option type") {
		t.Errorf("Expected specific error message, got:\n%s", diag.Format("test"))
	}
}

func TestTryExprErrorTypeMismatch(t *testing.T) {
	input := `module test version "1.0.0";

function parse(s: String) returns Result<Int, String>
    requires true
    ensures true
{
    return Ok(42);
}

function use_parse(s: String) returns Result<Int, Int>
    requires true
    ensures true
{
    let val: Int = parse(s)?;
    return Ok(val);
}`
	diag := parseAndCheck(t, input)

	if !diag.HasErrors() {
		t.Error("Expected error for error type mismatch in try operator")
	}
	if !strings.Contains(diag.Format("test"), "error type mismatch") {
		t.Errorf("Expected error type mismatch message, got:\n%s", diag.Format("test"))
	}
}

func TestPredicateMethodIsOk(t *testing.T) {
	input := `module test version "1.0.0";

function test(r: Result<Int, String>) returns Bool
    requires true
    ensures true
{
    return r.is_ok();
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestPredicateMethodIsErr(t *testing.T) {
	input := `module test version "1.0.0";

function test(r: Result<Int, String>) returns Bool
    requires true
    ensures true
{
    return r.is_err();
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestPredicateMethodIsSome(t *testing.T) {
	input := `module test version "1.0.0";

function test(o: Option<Int>) returns Bool
    requires true
    ensures true
{
    return o.is_some();
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestPredicateMethodIsNone(t *testing.T) {
	input := `module test version "1.0.0";

function test(o: Option<Int>) returns Bool
    requires true
    ensures true
{
    return o.is_none();
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

func TestPredicateMethodInEnsures(t *testing.T) {
	input := `module test version "1.0.0";

function safe_divide(a: Int, b: Int) returns Result<Int, String>
    ensures result.is_ok() implies b != 0
    ensures result.is_err() implies b == 0
{
    if b == 0 {
        return Err("division by zero");
    }
    return Ok(a / b);
}`
	diag := parseAndCheck(t, input)

	if diag.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", diag.Format("test"))
	}
}

// ===== Cross-File (CheckAll) Tests =====

func makeProgram(t *testing.T, source string) *ast.Program {
	t.Helper()
	p := parser.New(source)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}
	return prog
}

func TestCheckAllPublicFunctionCallAcrossModules(t *testing.T) {
	// math.intent: public add function
	mathSrc := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int
    requires true
    ensures result == a + b
{
    return a + b;
}
`
	// main.intent: imports math and calls math.add()
	mainSrc := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let sum: Int = math.add(3, 4);
    return sum;
}
`
	registry := map[string]*ast.Program{
		"/project/math.intent": makeProgram(t, mathSrc),
		"/project/main.intent": makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllPrivateFunctionProducesError(t *testing.T) {
	// math.intent: private function (no public keyword)
	mathSrc := `module math version "0.1.0";

function internal_helper(x: Int) returns Int {
    return x + 1;
}
`
	// main.intent: tries to call math.internal_helper()
	mainSrc := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.internal_helper(5);
    return val;
}
`
	registry := map[string]*ast.Program{
		"/project/math.intent": makeProgram(t, mathSrc),
		"/project/main.intent": makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if !result.Diagnostics.HasErrors() {
		t.Error("Expected error for calling private function from another module")
	}

	found := false
	for _, d := range result.Diagnostics.Errors() {
		if strings.Contains(d.Message, "not exported") || strings.Contains(d.Message, "private") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected visibility/export error, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllNonExistentFunctionInModule(t *testing.T) {
	mathSrc := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}
`
	mainSrc := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.nonexistent(5);
    return val;
}
`
	registry := map[string]*ast.Program{
		"/project/math.intent": makeProgram(t, mathSrc),
		"/project/main.intent": makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if !result.Diagnostics.HasErrors() {
		t.Error("Expected error for calling non-existent function in module")
	}

	found := false
	for _, d := range result.Diagnostics.Errors() {
		if strings.Contains(d.Message, "not exported") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'not exported' error, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllQualifiedEntityConstructor(t *testing.T) {
	geomSrc := `module geometry version "0.1.0";

public entity Circle {
    field radius: Float;

    constructor(r: Float) {
        self.radius = r;
    }
}
`
	mainSrc := `module main version "0.1.0";

import "geometry.intent";

entry function main() returns Int {
    let c: Circle = geometry.Circle(5.0);
    return 0;
}
`
	registry := map[string]*ast.Program{
		"/project/geometry.intent": makeProgram(t, geomSrc),
		"/project/main.intent":     makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/geometry.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllContractEnforcementOnImportedFunction(t *testing.T) {
	// math.intent: add with requires clause
	mathSrc := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int
    requires a >= 0
{
    return a + b;
}
`
	// main.intent: calls math.add with wrong argument type (String instead of Int)
	mainSrc := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.add("hello", 4);
    return val;
}
`
	registry := map[string]*ast.Program{
		"/project/math.intent": makeProgram(t, mathSrc),
		"/project/main.intent": makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if !result.Diagnostics.HasErrors() {
		t.Error("Expected error for type mismatch on cross-file function call")
	}

	found := false
	for _, d := range result.Diagnostics.Errors() {
		if strings.Contains(d.Message, "expected Int, got String") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument type mismatch error, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllUnimportedModuleProducesError(t *testing.T) {
	mathSrc := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}
`
	// main.intent: does NOT import math, but tries to use math.add()
	mainSrc := `module main version "0.1.0";

entry function main() returns Int {
    let val: Int = math.add(3, 4);
    return val;
}
`
	registry := map[string]*ast.Program{
		"/project/math.intent": makeProgram(t, mathSrc),
		"/project/main.intent": makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if !result.Diagnostics.HasErrors() {
		t.Error("Expected error for using unimported module")
	}
	// Should fail because "math" is not in moduleImports, so it falls through to
	// normal checkExpression path which reports "undeclared variable 'math'"
	// or "cannot call method on non-entity type"
}

func TestCheckAllSingleFileProgramRegression(t *testing.T) {
	// Verify that Check() still works for single-file programs (no changes to behavior)
	source := `module test version "1.0.0";

function add(a: Int, b: Int) returns Int {
    return a + b;
}

entry function main() returns Int {
    let sum: Int = add(3, 4);
    return sum;
}
`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for single-file program, got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodLen(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Int {
    return s.len();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.len(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodToLowercase(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns String {
    return s.to_lowercase();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.to_lowercase(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodTrim(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns String {
    return s.trim();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.trim(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodStartsWith(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Bool {
    return s.starts_with("hello");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.starts_with(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodContains(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Bool {
    return s.contains("hello");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.contains(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodSplit(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Array<String> {
    return s.split(",");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for String.split(), got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodChaining(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns String {
    return s.trim().to_lowercase();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for chained String methods, got:\n%s", diag.Format("test"))
	}
}

func TestStringMethodUnknown(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns String {
    return s.nonexistent();
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for unknown String method")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "no method") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'no method' error, got: %s", diag.Format("test"))
	}
}

func TestStringMethodStartsWithWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Bool {
    return s.starts_with(42);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for starts_with() with Int argument")
	}
}

func TestStringMethodLenWithArgs(t *testing.T) {
	source := `module test version "1.0.0";

function test(s: String) returns Int {
    return s.len("extra");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for len() with arguments")
	}
}

// ===== Map Type Tests =====

func TestResolveMapType(t *testing.T) {
	source := `module test version "1.0.0";

function test(m: Map<String, Int>) returns Int {
    let x: Map<String, Int> = m;
    return 0;
}`

	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		t.Fatalf("Parser errors: %s", p.Diagnostics().Format("test"))
	}

	result := CheckWithResult(prog)

	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors, got: %s", result.Diagnostics.Format("test"))
	}

	// Find the let statement and check its resolved type
	fn := prog.Functions[0]
	letStmt := fn.Body.Statements[0].(*ast.LetStmt)

	// Get the identifier type from exprTypes
	identExpr := letStmt.Value.(*ast.Identifier)
	exprType := result.ExprTypes[identExpr]
	if exprType == nil {
		t.Error("Expected identifier expression to have type in exprTypes map")
	} else if !exprType.IsGeneric || exprType.Name != "Map" {
		t.Errorf("Expected Map<String, Int> type, got: %s", exprType.String())
	} else if len(exprType.TypeParams) != 2 || !exprType.TypeParams[0].Equal(TypeString) || !exprType.TypeParams[1].Equal(TypeInt) {
		t.Errorf("Expected Map<String, Int> with correct type parameters, got: %s", exprType.String())
	}
}

func TestMapTypeEquality(t *testing.T) {
	mapStringInt1 := &Type{
		Name:       "Map",
		IsGeneric:  true,
		TypeParams: []*Type{TypeString, TypeInt},
	}

	mapStringInt2 := &Type{
		Name:       "Map",
		IsGeneric:  true,
		TypeParams: []*Type{TypeString, TypeInt},
	}

	mapIntString := &Type{
		Name:       "Map",
		IsGeneric:  true,
		TypeParams: []*Type{TypeInt, TypeString},
	}

	mapStringBool := &Type{
		Name:       "Map",
		IsGeneric:  true,
		TypeParams: []*Type{TypeString, TypeBool},
	}

	if !mapStringInt1.Equal(mapStringInt2) {
		t.Error("Map<String, Int> should equal Map<String, Int>")
	}

	if mapStringInt1.Equal(mapIntString) {
		t.Error("Map<String, Int> should not equal Map<Int, String>")
	}

	if mapStringInt1.Equal(mapStringBool) {
		t.Error("Map<String, Int> should not equal Map<String, Bool>")
	}
}

func TestCheckMapMethods(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let mutable m: Map<String, Int> = [];
    m.set("key", 42);
    let v: Int = m.get("key", 0);
    let has: Bool = m.contains("key");
    let k: Array<String> = m.keys();
    m.remove("key");
    return len(m);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestCheckMapMethodTypeMismatch(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let mutable m: Map<String, Int> = [];
    m.set(42, "wrong");
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for Map method type mismatch")
	}
	foundKey := false
	foundVal := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "set() key type mismatch") {
			foundKey = true
		}
		if strings.Contains(d.Message, "set() value type mismatch") {
			foundVal = true
		}
	}
	if !foundKey {
		t.Errorf("Expected 'set() key type mismatch' error, got: %s", diag.Format("test"))
	}
	if !foundVal {
		t.Errorf("Expected 'set() value type mismatch' error, got: %s", diag.Format("test"))
	}
}

func TestCheckMapSetMutability(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let m: Map<String, Int> = [];
    m.set("key", 42);
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for set() on immutable map")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "cannot call set() on immutable map") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'cannot call set() on immutable map' error, got: %s", diag.Format("test"))
	}
}

func TestCheckMapRemoveMutability(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let m: Map<String, Int> = [];
    m.remove("key");
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for remove() on immutable map")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "cannot call remove() on immutable map") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'cannot call remove() on immutable map' error, got: %s", diag.Format("test"))
	}
}

func TestCheckEmptyMapLiteral(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let m: Map<String, Int> = [];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Empty map literal with Map<String, Int> annotation should be accepted, got: %s", diag.Format("test"))
	}
}

func TestCheckLenMap(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let m: Map<String, Int> = [];
    let n: Int = len(m);
    return n;
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for len() on Map, got: %s", diag.Format("test"))
	}
}

func TestCheckTraitBasic(t *testing.T) {
	source := `module test version "1.0.0";
entity Ctx { field v: Int; constructor(n: Int) { self.v = n; } method get() returns Int { return self.v; } }
trait Handler { method execute(c: Ctx) returns Int; }
entity Start { field code: Int; constructor(c: Int) { self.code = c; } }
impl Handler for Start { method execute(c: Ctx) returns Int { return self.code + c.get(); } }
entry function main() returns Int { let s: Start = Start(5); let c: Ctx = Ctx(10); return s.execute(c); }`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for trait basic, got: %s", diag.Format("test"))
	}
}

func TestCheckTraitMissingMethod(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute() returns Int; }
entity Foo { field x: Int; constructor() { self.x = 0; } }
impl Handler for Foo { }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Errorf("Expected errors for trait missing method, got none")
	}
}

func TestCheckTraitSignatureMismatch(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute() returns Int; }
entity Foo { field x: Int; constructor() { self.x = 0; } }
impl Handler for Foo { method execute() returns Bool { return true; } }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Errorf("Expected errors for trait signature mismatch, got none")
	}
}

func TestCheckTraitParamCountMismatch(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute(x: Int) returns Int; }
entity Foo { field x: Int; constructor() { self.x = 0; } }
impl Handler for Foo { method execute() returns Int { return 0; } }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Errorf("Expected errors for trait param count mismatch, got none")
	}
}

func TestCheckTraitUnknownTrait(t *testing.T) {
	source := `module test version "1.0.0";
entity Foo { field x: Int; constructor() { self.x = 0; } }
impl Unknown for Foo { method run() returns Int { return 0; } }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Errorf("Expected errors for unknown trait, got none")
	}
}

func TestCheckTraitExtraMethod(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute() returns Int; }
entity Foo { field x: Int; constructor() { self.x = 0; } }
impl Handler for Foo { method execute() returns Int { return 0; } method extra() returns Int { return 1; } }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Errorf("Expected errors for trait extra method, got none")
	}
}

func TestVerifiedByTraitMethodRequires(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute(x: Int) returns Int requires x > 0; }
entity Foo { field v: Int; constructor() { self.v = 0; } }
impl Handler for Foo { method execute(x: Int) returns Int { return x; } }
intent "Test" { verified_by: [Handler.execute.requires]; }`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for verified_by trait method requires, got: %s", diag.Format("test"))
	}
}

func TestVerifiedByTraitMethodEnsures(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute(x: Int) returns Int ensures result > 0; }
entity Foo { field v: Int; constructor() { self.v = 0; } }
impl Handler for Foo { method execute(x: Int) returns Int { return x; } }
intent "Test" { verified_by: [Handler.execute.ensures]; }`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors for verified_by trait method ensures, got: %s", diag.Format("test"))
	}
}

func TestVerifiedByTraitMethodNoContract(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute(x: Int) returns Int; }
entity Foo { field v: Int; constructor() { self.v = 0; } }
impl Handler for Foo { method execute(x: Int) returns Int { return x; } }
intent "Test" { verified_by: [Handler.execute.requires]; }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for verified_by referencing trait method with no requires")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "no requires clause") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'no requires clause' error, got: %s", diag.Format("test"))
	}
}

func TestVerifiedByUnknownTraitMethod(t *testing.T) {
	source := `module test version "1.0.0";
trait Handler { method execute(x: Int) returns Int requires x > 0; }
entity Foo { field v: Int; constructor() { self.v = 0; } }
impl Handler for Foo { method execute(x: Int) returns Int { return x; } }
intent "Test" { verified_by: [Handler.nonexistent.requires]; }`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for verified_by referencing nonexistent trait method")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "has no method") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'has no method' error, got: %s", diag.Format("test"))
	}
}

func TestCheckAllCrossModuleTraitMethod(t *testing.T) {
	// handlers.intent: defines trait + entity + impl
	handlersSrc := `module handlers version "0.1.0";

public entity Ctx { field v: Int; constructor(n: Int) { self.v = n; } }

public trait Handler {
    method execute(c: Ctx) returns Int
        requires c.v >= 0;
}

public entity Start {
    field code: Int;
    constructor(c: Int) { self.code = c; }
}

impl Handler for Start {
    method execute(c: Ctx) returns Int {
        return self.code + c.v;
    }
}
`
	// main.intent: imports handlers and calls trait method on entity
	mainSrc := `module main version "0.1.0";

import "handlers.intent";

entry function main() returns Int {
    let s: Start = Start(5);
    let c: Ctx = Ctx(10);
    return s.execute(c);
}
`
	registry := map[string]*ast.Program{
		"/project/handlers.intent": makeProgram(t, handlersSrc),
		"/project/main.intent":     makeProgram(t, mainSrc),
	}
	sortedPaths := []string{"/project/handlers.intent", "/project/main.intent"}

	result := CheckAll(registry, sortedPaths, nil)
	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors for cross-module trait method call, got:\n%s", result.Diagnostics.Format("test"))
	}
}

// --- I/O Built-in Tests ---

func TestReadFile(t *testing.T) {
	source := `module test version "1.0.0";

function load(path: String) returns Result<String, String> {
    return read_file(path);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestReadFileWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function load() returns Result<String, String> {
    return read_file();
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for read_file() with no arguments")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "read_file() requires exactly 1 argument") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument count error, got: %s", diag.Format("test"))
	}
}

func TestReadFileWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function load() returns Result<String, String> {
    return read_file(42);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for read_file() with Int argument")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "read_file() argument must be String") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type error, got: %s", diag.Format("test"))
	}
}

func TestWriteFile(t *testing.T) {
	source := `module test version "1.0.0";

function save(path: String, content: String) returns Result<Void, String> {
    return write_file(path, content);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestWriteFileWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function save(path: String) returns Result<Void, String> {
    return write_file(path);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for write_file() with 1 argument")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "write_file() requires exactly 2 arguments") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument count error, got: %s", diag.Format("test"))
	}
}

func TestCreateDir(t *testing.T) {
	source := `module test version "1.0.0";

function make_dir(path: String) returns Result<Void, String> {
    return create_dir(path);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestFileExists(t *testing.T) {
	source := `module test version "1.0.0";

function check(path: String) returns Bool {
    return file_exists(path);
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestFileExistsWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function check() returns Bool {
    return file_exists(123);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for file_exists() with Int argument")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "file_exists() argument must be String") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type error, got: %s", diag.Format("test"))
	}
}

func TestEnvGet(t *testing.T) {
	source := `module test version "1.0.0";

function get_home() returns Option<String> {
    return env_get("HOME");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestEnvGetWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function get_env() returns Option<String> {
    return env_get(42);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for env_get() with Int argument")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "env_get() argument must be String") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type error, got: %s", diag.Format("test"))
	}
}

func TestToStringInt(t *testing.T) {
	source := `module test version "1.0.0";

function convert(n: Int) returns String {
    return n.to_string();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestToStringFloat(t *testing.T) {
	source := `module test version "1.0.0";

function convert(n: Float) returns String {
    return n.to_string();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestToStringBool(t *testing.T) {
	source := `module test version "1.0.0";

function convert(b: Bool) returns String {
    return b.to_string();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestToStringWrongArgs(t *testing.T) {
	source := `module test version "1.0.0";

function convert(n: Int) returns String {
    return n.to_string(42);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for to_string() with arguments")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "to_string() requires no arguments") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected argument count error, got: %s", diag.Format("test"))
	}
}

func TestHttpPost(t *testing.T) {
	source := `module test version "1.0.0";

function do_post() returns Result<String, String> {
    return http_post("url", "headers", "body");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestHttpPostWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function do_post() returns Result<String, String> {
    return http_post("url");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for http_post() with wrong arg count")
	}
}

func TestHttpPostWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function do_post() returns Result<String, String> {
    return http_post("url", "headers", 42);
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for http_post() with Int argument")
	}
}

func TestHttpGet(t *testing.T) {
	source := `module test version "1.0.0";

function do_get() returns Result<String, String> {
    return http_get("url", "headers");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestHttpGetWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function do_get() returns Result<String, String> {
    return http_get("url", "headers", "extra");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for http_get() with wrong arg count")
	}
}

func TestJsonGet(t *testing.T) {
	source := `module test version "1.0.0";

function extract() returns Option<String> {
    return json_get("data", "key");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestJsonGetWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function extract() returns Option<String> {
    return json_get(42, "key");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for json_get() with Int argument")
	}
}

func TestEmitEvent(t *testing.T) {
	source := `module test version "1.0.0";

function notify() returns Void {
    emit_event("node_start", "payload");
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestEmitEventWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function notify() returns Void {
    emit_event("only_one");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for emit_event() with wrong arg count")
	}
}

func TestTimestampMs(t *testing.T) {
	source := `module test version "1.0.0";

function get_time() returns Int {
    return timestamp_ms();
}`
	diag := parseAndCheck(t, source)
	if diag.HasErrors() {
		t.Errorf("Expected no errors, got: %s", diag.Format("test"))
	}
}

func TestTimestampMsWrongArgs(t *testing.T) {
	source := `module test version "1.0.0";

function get_time() returns Int {
    return timestamp_ms("arg");
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for timestamp_ms() with arguments")
	}
}

func TestMapFloatKeyRejected(t *testing.T) {
	source := `module test version "1.0";
function foo() returns Void {
    let m: Map<Float, String> = [];
}
`
	diags := parseAndCheck(t, source)
	if !diags.HasErrors() {
		t.Error("Expected error for Map<Float, String> but got none")
	}
}

func TestMapValidKeys(t *testing.T) {
	source := `module test version "1.0";
function foo() returns Void {
    let m1: Map<String, String> = [];
    let m2: Map<Int, String> = [];
}
`
	diags := parseAndCheck(t, source)
	if diags.HasErrors() {
		t.Errorf("Expected no errors for Map<String/Int, ...> but got: %s", diags.Format("test"))
	}
}

func TestJsonPath(t *testing.T) {
	source := `module test version "1.0";
function foo() returns Option<String> {
    return json_path("data", "a.b.c");
}
`
	diags := parseAndCheck(t, source)
	if diags.HasErrors() {
		t.Errorf("Unexpected errors: %s", diags.Format("test"))
	}
}

func TestJsonPathWrongArgCount(t *testing.T) {
	source := `module test version "1.0";
function foo() returns Option<String> {
    return json_path("data");
}
`
	diags := parseAndCheck(t, source)
	if !diags.HasErrors() {
		t.Error("Expected error for wrong arg count")
	}
}

func TestJsonPathWrongArgType(t *testing.T) {
	source := `module test version "1.0";
function foo() returns Option<String> {
    return json_path("data", 42);
}
`
	diags := parseAndCheck(t, source)
	if !diags.HasErrors() {
		t.Error("Expected error for wrong arg type")
	}
}

func TestLambdaTypeChecking(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let f: Fn(Int) -> Int = |x: Int| -> Int => x + 1;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for valid lambda, got: %s", diag.Format("test"))
	}
}

func TestLambdaMultipleParams(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let add: Fn(Int, Int) -> Int = |x: Int, y: Int| -> Int => x + y;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for two-param lambda, got: %s", diag.Format("test"))
	}
}

func TestFnTypeInFunctionParam(t *testing.T) {
	source := `module test version "1.0.0";

function apply(f: Fn(Int) -> Int, x: Int) returns Int {
    return f(x);
}

function test() returns Int {
    let double: Fn(Int) -> Int = |n: Int| -> Int => n * 2;
    return apply(double, 5);
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for Fn-typed param, got: %s", diag.Format("test"))
	}
}

func TestCallingFnTypedVariable(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let increment: Fn(Int) -> Int = |x: Int| -> Int => x + 1;
    return increment(10);
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for calling Fn-typed variable, got: %s", diag.Format("test"))
	}
}

func TestLambdaReturnTypeMismatch(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Void {
    let f: Fn(Int) -> Int = |x: Int| -> Int => "not an int";
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected type mismatch error for lambda body returning wrong type")
	}
}

func TestCallingFnTypedVariableWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let f: Fn(Int) -> Int = |x: Int| -> Int => x + 1;
    return f("wrong");
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for calling Fn-typed variable with wrong argument type")
	}
}

func TestCallingFnTypedVariableWrongArgCount(t *testing.T) {
	source := `module test version "1.0.0";

function test() returns Int {
    let f: Fn(Int) -> Int = |x: Int| -> Int => x + 1;
    return f(1, 2);
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for calling Fn-typed variable with wrong number of arguments")
	}
}

func TestLambdaInFunctionCall(t *testing.T) {
	source := `module test version "1.0.0";

function apply(f: Fn(Int) -> Int, x: Int) returns Int {
    return f(x);
}

function test() returns Int {
    return apply(|n: Int| -> Int => n * 3, 7);
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for inline lambda in call, got: %s", diag.Format("test"))
	}
}

// === Async/Await/Spawn Tests ===

func TestAsyncFunctionDeclaration(t *testing.T) {
	source := `module test version "1.0.0";

async function fetchData(url: String) returns Future<String> {
    return url;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for valid async function, got: %s", diag.Format("test"))
	}
}

func TestAwaitOutsideAsyncFunction(t *testing.T) {
	source := `module test version "1.0.0";

async function getData() returns Future<Int> {
    return 42;
}

function main() returns Int {
    let x: Int = await getData();
    return x;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for await outside async function")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "await can only be used inside async functions") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'await can only be used inside async functions' error, got: %s", diag.Format("test"))
	}
}

func TestAwaitOnNonFutureType(t *testing.T) {
	source := `module test version "1.0.0";

async function main() returns Future<Int> {
    let x: Int = 42;
    let y: Int = await x;
    return y;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for await on non-Future type")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "await requires Future type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'await requires Future type' error, got: %s", diag.Format("test"))
	}
}

func TestSpawnOnNonAsyncFunction(t *testing.T) {
	source := `module test version "1.0.0";

function compute(x: Int) returns Int {
    return x;
}

async function main() returns Future<Int> {
    let handle: Int = spawn compute(42);
    return handle;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for spawn on non-async function")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "spawn requires an async function") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'spawn requires an async function' error, got: %s", diag.Format("test"))
	}
}

func TestValidSpawnAndAwait(t *testing.T) {
	source := `module test version "1.0.0";

async function compute(x: Int) returns Future<Int> {
    return x;
}

async function main() returns Future<Int> {
    let handle: Future<Int> = spawn compute(42);
    let val: Int = await handle;
    return val;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for valid spawn and await, got: %s", diag.Format("test"))
	}
}

func TestFutureTypeResolution(t *testing.T) {
	source := `module test version "1.0.0";

async function fetchName() returns Future<String> {
    return "hello";
}

async function main() returns Future<String> {
    let f: Future<String> = spawn fetchName();
    let name: String = await f;
    return name;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for Future type resolution, got: %s", diag.Format("test"))
	}
}

func TestAsyncFunctionWithContracts(t *testing.T) {
	source := `module test version "1.0.0";

async function fetchPositive(x: Int) returns Future<Int>
    requires x > 0
{
    return x;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for async function with contracts, got: %s", diag.Format("test"))
	}
}

func TestAwaitInsideAsyncFunction(t *testing.T) {
	source := `module test version "1.0.0";

async function inner() returns Future<Int> {
    return 42;
}

async function outer() returns Future<Int> {
    let val: Int = await inner();
    return val;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for await inside async function, got: %s", diag.Format("test"))
	}
}

func TestSleepBuiltin(t *testing.T) {
	source := `module test version "1.0.0";

async function delayedWork() returns Future<Void> {
    let f: Future<Void> = sleep(1000);
    await f;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for sleep builtin, got: %s", diag.Format("test"))
	}
}

func TestSleepWrongArgType(t *testing.T) {
	source := `module test version "1.0.0";

async function delayedWork() returns Future<Void> {
    let f: Future<Void> = sleep("hello");
    await f;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for sleep with String argument")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "sleep() argument must be Int") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'sleep() argument must be Int' error, got: %s", diag.Format("test"))
	}
}

func TestTimeoutBuiltin(t *testing.T) {
	source := `module test version "1.0.0";

async function fetchData() returns Future<String> {
    return "data";
}

async function main() returns Future<String> {
    let f: Future<String> = spawn fetchData();
    let r: Result<String, String> = timeout(f, 5000);
    let value: String = match r {
        Ok(v) => v,
        Err(e) => e
    };
    return value;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for timeout builtin, got: %s", diag.Format("test"))
	}
}

func TestAwaitAllBuiltin(t *testing.T) {
	source := `module test version "1.0.0";

async function compute(x: Int) returns Future<Int> {
    return x;
}

async function main() returns Future<Int> {
    let f1: Future<Int> = spawn compute(1);
    let f2: Future<Int> = spawn compute(2);
    let futures: Array<Future<Int>> = [f1, f2];
    let results: Array<Int> = await_all(futures);
    return results[0];
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for await_all builtin, got: %s", diag.Format("test"))
	}
}

func TestAwaitAllOutsideAsync(t *testing.T) {
	source := `module test version "1.0.0";

async function compute(x: Int) returns Future<Int> {
    return x;
}

function main() returns Int {
    let f1: Future<Int> = spawn compute(1);
    let futures: Array<Future<Int>> = [f1];
    let results: Array<Int> = await_all(futures);
    return results[0];
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for await_all outside async function")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "await_all can only be used inside async functions") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'await_all can only be used inside async functions' error, got: %s", diag.Format("test"))
	}
}

func TestAwaitAnyBuiltin(t *testing.T) {
	source := `module test version "1.0.0";

async function compute(x: Int) returns Future<Int> {
    return x;
}

async function main() returns Future<Int> {
    let f1: Future<Int> = spawn compute(1);
    let f2: Future<Int> = spawn compute(2);
    let futures: Array<Future<Int>> = [f1, f2];
    let first: Int = await_any(futures);
    return first;
}`

	diag := parseAndCheck(t, source)

	if diag.HasErrors() {
		t.Errorf("Expected no errors for await_any builtin, got: %s", diag.Format("test"))
	}
}

func TestTimeoutOutsideAsync(t *testing.T) {
	source := `module test version "1.0.0";

async function fetchData() returns Future<String> {
    return "data";
}

function main() returns String {
    let f: Future<String> = spawn fetchData();
    let r: Result<String, String> = timeout(f, 5000);
    let value: String = match r {
        Ok(v) => v,
        Err(e) => e
    };
    return value;
}`

	diag := parseAndCheck(t, source)

	if !diag.HasErrors() {
		t.Error("Expected error for timeout outside async function")
	}
	found := false
	for _, err := range diag.Errors() {
		if strings.Contains(err.Message, "timeout can only be used inside async functions") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'timeout can only be used inside async functions' error, got: %s", diag.Format("test"))
	}
}

func TestCheckAllPackageImportCrossPackageCall(t *testing.T) {
	// Package module: types_pkg/types.intent
	typesSrc := `module types version "1.0.0";

public function distance(x: Int, y: Int) returns Int {
    return x + y;
}
`
	// Main module with package import
	mainSrc := `module main version "1.0.0";

import types_pkg;

entry function main() returns Int {
    let d: Int = types_pkg.distance(3, 4);
    return d;
}
`
	// The package import sets IsPackage=true, PackageName="types_pkg"
	mainProg := makeProgram(t, mainSrc)

	registry := map[string]*ast.Program{
		"/project/libs/types_pkg/types.intent": makeProgram(t, typesSrc),
		"/project/main.intent":                 mainProg,
	}
	sortedPaths := []string{
		"/project/libs/types_pkg/types.intent",
		"/project/main.intent",
	}

	result := CheckAll(registry, sortedPaths, nil)
	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors for cross-package call, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestCheckAllPackageImportMultipleFiles(t *testing.T) {
	// Package has two files, both with public functions
	mathSrc := `module math version "1.0.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}
`
	utilsSrc := `module utils version "1.0.0";

public function negate(x: Int) returns Int {
    return 0 - x;
}
`
	// Main module imports the package
	mainSrc := `module main version "1.0.0";

import types_pkg;

entry function main() returns Int {
    let a: Int = types_pkg.add(1, 2);
    let b: Int = types_pkg.negate(a);
    return b;
}
`
	registry := map[string]*ast.Program{
		"/project/libs/types_pkg/math.intent":  makeProgram(t, mathSrc),
		"/project/libs/types_pkg/utils.intent": makeProgram(t, utilsSrc),
		"/project/main.intent":                 makeProgram(t, mainSrc),
	}
	sortedPaths := []string{
		"/project/libs/types_pkg/math.intent",
		"/project/libs/types_pkg/utils.intent",
		"/project/main.intent",
	}

	result := CheckAll(registry, sortedPaths, nil)
	if result.Diagnostics.HasErrors() {
		t.Errorf("Expected no errors for cross-package multi-file call, got:\n%s", result.Diagnostics.Format("test"))
	}
}

func TestIsFileInPackage_WithPackageDirs(t *testing.T) {
	packageDirs := map[string]string{
		"math_utils": "/project/libs/math_utils",
		"strings":    "/project/libs/strings",
	}

	tests := []struct {
		name     string
		filePath string
		pkgName  string
		want     bool
	}{
		{
			name:     "file in known package directory",
			filePath: "/project/libs/math_utils/add.intent",
			pkgName:  "math_utils",
			want:     true,
		},
		{
			name:     "file not in known package directory",
			filePath: "/project/other/add.intent",
			pkgName:  "math_utils",
			want:     false,
		},
		{
			name:     "file in different package directory same name",
			filePath: "/elsewhere/math_utils/add.intent",
			pkgName:  "math_utils",
			want:     false,
		},
		{
			name:     "second package directory match",
			filePath: "/project/libs/strings/helpers.intent",
			pkgName:  "strings",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFileInPackage(tt.filePath, tt.pkgName, packageDirs)
			if got != tt.want {
				t.Errorf("isFileInPackage(%q, %q, packageDirs) = %v, want %v",
					tt.filePath, tt.pkgName, got, tt.want)
			}
		})
	}
}

func TestIsFileInPackage_FallbackWithoutPackageDirs(t *testing.T) {
	// Empty packageDirs forces fallback to directory-name matching
	packageDirs := map[string]string{}

	tests := []struct {
		name     string
		filePath string
		pkgName  string
		want     bool
	}{
		{
			name:     "parent dir matches package name",
			filePath: "/project/math_utils/add.intent",
			pkgName:  "math_utils",
			want:     true,
		},
		{
			name:     "parent dir does not match",
			filePath: "/project/other/add.intent",
			pkgName:  "math_utils",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFileInPackage(tt.filePath, tt.pkgName, packageDirs)
			if got != tt.want {
				t.Errorf("isFileInPackage(%q, %q, {}) = %v, want %v",
					tt.filePath, tt.pkgName, got, tt.want)
			}
		})
	}
}

// Phase 15 / ADR 0028: extern function FFI declarations.

func checkExternSrc(t *testing.T, src string) *diagnostic.Diagnostics {
	t.Helper()
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected parse errors: %s", p.Diagnostics().Format("test"))
	}
	return Check(prog)
}

func TestCheckExternFunctionValid(t *testing.T) {
	src := `module test version "1.0";

extern function hash(input: String) returns String
    from "blake3::hash"
    requires len(input) > 0
    ensures len(result) == 64;

entry function main() returns Int { return 0; }
`
	d := checkExternSrc(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckExternRejectsEntityParam(t *testing.T) {
	src := `module test version "1.0";

entity Point { field x: Int; field y: Int; constructor() { self.x = 0; self.y = 0; } }

extern function bad(p: Point) returns String
    from "crate::bad";

entry function main() returns Int { return 0; }
`
	d := checkExternSrc(t, src)
	if !d.HasErrors() {
		t.Fatal("expected error for entity param")
	}
	if !strings.Contains(d.Format("test"), "entity types are not supported") {
		t.Errorf("expected diagnostic about entity types, got: %s", d.Format("test"))
	}
}

func TestCheckExternRejectsMapType(t *testing.T) {
	src := `module test version "1.0";

extern function bad(m: Map<String, Int>) returns String
    from "crate::bad";

entry function main() returns Int { return 0; }
`
	d := checkExternSrc(t, src)
	if !d.HasErrors() {
		t.Fatal("expected error for Map param")
	}
	if !strings.Contains(d.Format("test"), "Map<K,V> is not supported") {
		t.Errorf("expected diagnostic about Map, got: %s", d.Format("test"))
	}
}

func TestCheckExternRejectsBadFromPath(t *testing.T) {
	src := `module test version "1.0";

extern function bad(x: Int) returns Int
    from "just_a_name";

entry function main() returns Int { return 0; }
`
	d := checkExternSrc(t, src)
	if !d.HasErrors() {
		t.Fatal("expected error for missing :: in from path")
	}
	if !strings.Contains(d.Format("test"), `must be of the form "crate::path`) {
		t.Errorf("expected diagnostic about from path shape, got: %s", d.Format("test"))
	}
}

func TestCheckExternAllowsBridgeableNestedTypes(t *testing.T) {
	src := `module test version "1.0";

extern function fetch(url: String) returns Result<String, String>
    from "intent_http::fetch";

extern function chunk_lines(text: String) returns Array<String>
    from "text_utils::split_lines";

extern function maybe_user(id: Int) returns Option<String>
    from "users::lookup";

entry function main() returns Int { return 0; }
`
	d := checkExternSrc(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors for Result/Array/Option bridge types, got: %s", d.Format("test"))
	}
}

func TestIsFileInPackage_NilPackageDirs(t *testing.T) {
	// nil packageDirs should also use fallback path
	got := isFileInPackage("/project/mylib/foo.intent", "mylib", nil)
	if !got {
		t.Error("isFileInPackage with nil packageDirs should fallback to dir name matching")
	}
	got = isFileInPackage("/project/other/foo.intent", "mylib", nil)
	if got {
		t.Error("isFileInPackage with nil packageDirs should not match wrong dir")
	}
}

// Phase 16 / ADR 0029: in-language testing framework — checker tests.

func TestCheckValidTest(t *testing.T) {
	src := `module test version "1.0";

function helper(n: Int) returns Int { return n + 1; }

test "uses helper" {
    let x: Int = helper(5);
    assert(x == 6);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckTestRejectsReturn(t *testing.T) {
	src := `module test version "1.0";

test "tries to return" {
    return;
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for return inside test body")
	}
	if !strings.Contains(d.Format("test"), "'return' is not allowed inside a test body") {
		t.Errorf("unexpected diagnostic: %s", d.Format("test"))
	}
}

func TestCheckAssertNonBool(t *testing.T) {
	src := `module test version "1.0";

test "assert wrong type" {
    assert(42);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert with non-Bool arg")
	}
	if !strings.Contains(d.Format("test"), "assert() argument must be Bool") {
		t.Errorf("unexpected diagnostic: %s", d.Format("test"))
	}
}

func TestCheckAssertEqMatchingInts(t *testing.T) {
	src := `module test version "1.0";

test "ints equal" {
    assert_eq(1 + 1, 2);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckAssertEqTypeMismatch(t *testing.T) {
	src := `module test version "1.0";

test "mismatch" {
    assert_eq(1, "two");
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_eq with mismatched types")
	}
	if !strings.Contains(d.Format("test"), "assert_eq() type mismatch") {
		t.Errorf("unexpected diagnostic: %s", d.Format("test"))
	}
}

func TestCheckAssertEqRejectsFloat(t *testing.T) {
	src := `module test version "1.0";

test "floats not allowed" {
    assert_eq(1.0, 1.0);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_eq on Float")
	}
	if !strings.Contains(d.Format("test"), "assert_eq does not support Float") {
		t.Errorf("expected Float-rejection diagnostic, got: %s", d.Format("test"))
	}
	if !strings.Contains(d.Format("test"), "assert_close") {
		t.Errorf("expected diagnostic to mention assert_close, got: %s", d.Format("test"))
	}
}

func TestCheckAssertEqRejectsEntityWithoutEq(t *testing.T) {
	src := `module test version "1.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x_init: Int, y_init: Int)
        ensures self.x == x_init
        ensures self.y == y_init
    {
        self.x = x_init;
        self.y = y_init;
    }
}

test "entity without eq" {
    let p1: Point = Point(1, 2);
    let p2: Point = Point(1, 2);
    assert_eq(p1, p2);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_eq on entity without eq method")
	}
	if !strings.Contains(d.Format("test"), "has no eq method") {
		t.Errorf("expected eq-method diagnostic, got: %s", d.Format("test"))
	}
}

func TestCheckAssertEqEntityWithEq(t *testing.T) {
	src := `module test version "1.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(x_init: Int, y_init: Int) {
        self.x = x_init;
        self.y = y_init;
    }

    method eq(other: Point) returns Bool {
        return self.x == other.x and self.y == other.y;
    }
}

test "entity with eq" {
    let p1: Point = Point(1, 2);
    let p2: Point = Point(1, 2);
    assert_eq(p1, p2);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors with eq method present, got: %s", d.Format("test"))
	}
}

func TestCheckAssertEqRejectsMap(t *testing.T) {
	src := `module test version "1.0";

test "map not allowed" {
    let m1: Map<String, Int> = Map<String, Int>();
    let m2: Map<String, Int> = Map<String, Int>();
    assert_eq(m1, m2);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_eq on Map")
	}
	if !strings.Contains(d.Format("test"), "does not support Map") {
		t.Errorf("expected Map-rejection diagnostic, got: %s", d.Format("test"))
	}
}

func TestCheckAssertCloseFloats(t *testing.T) {
	src := `module test version "1.0";

test "close floats" {
    assert_close(3.14, 3.14, 0.001);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckAssertCloseWrongType(t *testing.T) {
	src := `module test version "1.0";

test "close ints rejected" {
    assert_close(1, 1, 1);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_close on Int")
	}
	if !strings.Contains(d.Format("test"), "assert_close() argument 1") || !strings.Contains(d.Format("test"), "must be Float") {
		t.Errorf("expected Float-required diagnostic, got: %s", d.Format("test"))
	}
}

func TestCheckAssertCloseWrongArity(t *testing.T) {
	src := `module test version "1.0";

test "wrong arity" {
    assert_close(1.0, 1.0);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_close with 2 args instead of 3")
	}
	if !strings.Contains(d.Format("test"), "assert_close() expects 3 arguments") {
		t.Errorf("expected arity diagnostic, got: %s", d.Format("test"))
	}
}

func TestCheckAsyncTestWithAwait(t *testing.T) {
	src := `module test version "1.0";

async function delayed(n: Int) returns Future<Int>
    requires n >= 0
{
    return n + 1;
}

async test "uses await" {
    let f: Future<Int> = spawn delayed(5);
    let r: Int = await f;
    assert(r == 6);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckAsyncTestWithoutAwaitWarns(t *testing.T) {
	src := `module test version "1.0";

async test "no await" {
    let x: Int = 1;
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("async-without-await should be a warning, not an error; got: %s", d.Format("test"))
	}
	if d.WarningCount() == 0 {
		t.Error("expected a warning for async test with no await")
	}
	if !strings.Contains(d.Format("test"), "declared 'async' but contains no 'await'") {
		t.Errorf("unexpected warning text: %s", d.Format("test"))
	}
}

func TestCheckNonAsyncTestRejectsAwait(t *testing.T) {
	src := `module test version "1.0";

async function delayed() returns Future<Int> { return 1; }

test "tries to await without async" {
    let f: Future<Int> = spawn delayed();
    let r: Int = await f;
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for await in non-async test")
	}
	if !strings.Contains(d.Format("test"), "await can only be used inside async functions") {
		t.Errorf("expected await-only-in-async diagnostic, got: %s", d.Format("test"))
	}
}

func TestCheckAssertPanicsFnVoid(t *testing.T) {
	src := `module test version "1.0";

test "panics expected" {
    let bomb: Fn() -> Void = || -> Void => assert(false);
    assert_panics(bomb);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if d.HasErrors() {
		t.Errorf("expected no errors, got: %s", d.Format("test"))
	}
}

func TestCheckAssertPanicsWrongShape(t *testing.T) {
	src := `module test version "1.0";

test "wrong fn shape" {
    let f: Fn(Int) -> Int = |x: Int| -> Int => x + 1;
    assert_panics(f);
}

entry function main() returns Int { return 0; }
`
	d := parseAndCheck(t, src)
	if !d.HasErrors() {
		t.Error("expected error for assert_panics with non-Fn()->Void argument")
	}
	if !strings.Contains(d.Format("test"), "assert_panics() argument must be Fn() -> Void") {
		t.Errorf("expected Fn-shape diagnostic, got: %s", d.Format("test"))
	}
}
