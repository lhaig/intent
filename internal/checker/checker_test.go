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
	source := `module test version "1.0.0";
entry function main() returns Int {
    let x: Array<Int> = [];
    return 0;
}`
	diag := parseAndCheck(t, source)
	if !diag.HasErrors() {
		t.Error("Expected error for empty array literal")
	}
	found := false
	for _, d := range diag.All() {
		if strings.Contains(d.Message, "empty array literal requires type annotation") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'empty array literal requires type annotation' error, got: %s", diag.Format("test"))
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
		if strings.Contains(d.Message, "len() requires Array argument") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'len() requires Array argument' error, got: %s", diag.Format("test"))
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

	result := CheckAll(registry, sortedPaths)
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

	result := CheckAll(registry, sortedPaths)
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

	result := CheckAll(registry, sortedPaths)
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

	result := CheckAll(registry, sortedPaths)
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

	result := CheckAll(registry, sortedPaths)
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

	result := CheckAll(registry, sortedPaths)
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
