package codegen

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/parser"
)

func TestGenerateSimpleFunction(t *testing.T) {
	source := `
module test version "1.0.0";

function add(a: Int, b: Int) returns Int {
	return a + b;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn add(a: i64, b: i64) -> i64") {
		t.Error("Expected function signature not found")
	}
	if !strings.Contains(result, "a + b") {
		t.Error("Expected addition expression not found")
	}
}

func TestGenerateEntryFunction(t *testing.T) {
	source := `
module test version "1.0.0";

entry function main() returns Int {
	return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn __intent_main() -> i64") {
		t.Error("Expected __intent_main function not found")
	}
	if !strings.Contains(result, "fn main()") {
		t.Error("Expected main wrapper not found")
	}
	if !strings.Contains(result, "std::process::exit(__exit_code as i32)") {
		t.Error("Expected process::exit call not found")
	}
}

func TestGenerateFunctionWithRequires(t *testing.T) {
	source := `
module test version "1.0.0";

function divide(a: Int, b: Int) returns Int
requires b != 0 {
	return a / b;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "assert!(") {
		t.Error("Expected assert! for requires clause")
	}
	if !strings.Contains(result, "Precondition failed") {
		t.Error("Expected precondition error message")
	}
	if !strings.Contains(result, "b != 0") {
		t.Error("Expected requires condition in assert")
	}
}

func TestGenerateFunctionWithEnsures(t *testing.T) {
	source := `
module test version "1.0.0";

function abs(n: Int) returns Int
ensures result >= 0 {
	if n < 0 {
		return 0 - n;
	}
	return n;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "'body: {") {
		t.Error("Expected labeled block for ensures")
	}
	if !strings.Contains(result, "let __result: i64 = 'body:") {
		t.Error("Expected __result variable declaration")
	}
	if !strings.Contains(result, "break 'body") {
		t.Error("Expected break 'body for return in labeled block")
	}
	if !strings.Contains(result, "__result >= 0") {
		t.Error("Expected ensures condition with __result")
	}
	if !strings.Contains(result, "Postcondition failed") {
		t.Error("Expected postcondition error message")
	}
}

func TestGenerateEntity(t *testing.T) {
	source := `
module test version "1.0.0";

entity Counter {
	field value: Int;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "#[derive(Clone, Debug)]") {
		t.Error("Expected derive attribute")
	}
	if !strings.Contains(result, "struct Counter {") {
		t.Error("Expected struct declaration")
	}
	if !strings.Contains(result, "value: i64") {
		t.Error("Expected field with correct type")
	}
	if !strings.Contains(result, "impl Counter {") {
		t.Error("Expected impl block")
	}
}

func TestGenerateEntityWithInvariants(t *testing.T) {
	source := `
module test version "1.0.0";

entity PositiveCounter {
	field value: Int;

	invariant value >= 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn __check_invariants(&self)") {
		t.Error("Expected __check_invariants method")
	}
	if !strings.Contains(result, "value >= 0") {
		t.Error("Expected invariant condition")
	}
	if !strings.Contains(result, "Invariant failed") {
		t.Error("Expected invariant error message")
	}
}

func TestGenerateConstructor(t *testing.T) {
	source := `
module test version "1.0.0";

entity Counter {
	field value: Int;

	constructor(initial: Int) {
		self.value = initial;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn new(initial: i64) -> Counter") {
		t.Error("Expected new constructor function")
	}
	if !strings.Contains(result, "let mut __self = Counter {") {
		t.Error("Expected __self initialization")
	}
	if !strings.Contains(result, "value: 0i64") {
		t.Error("Expected default field initialization")
	}
	if !strings.Contains(result, "__self.value = initial") {
		t.Error("Expected __self field assignment in constructor")
	}
}

func TestGenerateMethodWithOld(t *testing.T) {
	source := `
module test version "1.0.0";

entity BankAccount {
	field balance: Int;

	method deposit(amount: Int) returns Void
	requires amount > 0
	ensures self.balance == old(self.balance) + amount {
		self.balance = self.balance + amount;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "let __old_self_balance = self.balance") {
		t.Error("Expected old value capture")
	}
	if !strings.Contains(result, "__old_self_balance") {
		t.Error("Expected old value reference in ensures")
	}
	if !strings.Contains(result, "self.balance ==") {
		t.Error("Expected balance comparison in ensures")
	}
}

func TestGenerateIntentBlock(t *testing.T) {
	source := `
module test version "1.0.0";

function divide(a: Int, b: Int) returns Int
requires b != 0 {
	return a / b;
}

intent "safe division" {
	goal: "prevent division by zero";
	constraint: "divisor must be non-zero";
	guarantee: "no runtime division errors";
	verified_by: divide.requires;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "// Intent: safe division") {
		t.Error("Expected intent description comment")
	}
	if !strings.Contains(result, "// Goal: prevent division by zero") {
		t.Error("Expected goal comment")
	}
	if !strings.Contains(result, "// Constraint: divisor must be non-zero") {
		t.Error("Expected constraint comment")
	}
	if !strings.Contains(result, "// Guarantee: no runtime division errors") {
		t.Error("Expected guarantee comment")
	}
	if !strings.Contains(result, "// Verified by: divide.requires") {
		t.Error("Expected verified_by comment")
	}
	if !strings.Contains(result, "#[cfg(test)]") {
		t.Error("Expected test module attribute")
	}
	if !strings.Contains(result, "mod __intent_safe_division") {
		t.Error("Expected mangled test module name")
	}
}

func TestGenerateLetBindings(t *testing.T) {
	source := `
module test version "1.0.0";

function test() returns Int {
	let x: Int = 42;
	let mutable y: Int = 0;
	y = x;
	return y;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "let x: i64 = 42i64") {
		t.Error("Expected immutable let binding")
	}
	if !strings.Contains(result, "let mut y: i64 = 0i64") {
		t.Error("Expected mutable let binding")
	}
	if !strings.Contains(result, "y = x") {
		t.Error("Expected assignment")
	}
}

func TestGenerateImpliesExpression(t *testing.T) {
	source := `
module test version "1.0.0";

function logical(a: Bool, b: Bool) returns Bool
ensures a implies result {
	if a {
		return b;
	}
	return true;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "(!a || __result)") {
		t.Error("Expected implies to be translated to (!a || b)")
	}
}

func TestGenerateEntityConstruction(t *testing.T) {
	source := `
module test version "1.0.0";

entity Point {
	field x: Int;
	field y: Int;

	constructor(px: Int, py: Int) {
		self.x = px;
		self.y = py;
	}
}

function makePoint() returns Point {
	return Point(10, 20);
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "Point::new(10i64, 20i64)") {
		t.Error("Expected entity constructor call as Point::new()")
	}
}

func TestGenerateTypeMapping(t *testing.T) {
	source := `
module test version "1.0.0";

function types(i: Int, f: Float, s: String, b: Bool) returns Void {
	return;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "i: i64") {
		t.Error("Expected Int -> i64")
	}
	if !strings.Contains(result, "f: f64") {
		t.Error("Expected Float -> f64")
	}
	if !strings.Contains(result, "s: String") {
		t.Error("Expected String -> String")
	}
	if !strings.Contains(result, "b: bool") {
		t.Error("Expected Bool -> bool")
	}
	if !strings.Contains(result, "-> ()") {
		t.Error("Expected Void -> ()")
	}
}

func TestGenerateStringConcatenation(t *testing.T) {
	source := `
module test version "1.0.0";

function greet() returns String {
	return "Hello, " + "World!";
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// String concatenation should use format! macro
	if !strings.Contains(result, "format!") {
		t.Error("Expected format! macro for string concatenation")
	}
}

func TestGenerateVoidMethodWithEnsures(t *testing.T) {
	source := `
module test version "1.0.0";

entity Counter {
	field value: Int;

	method increment() returns Void
	ensures self.value == old(self.value) + 1 {
		self.value = self.value + 1;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Void methods should NOT use labeled block pattern
	if strings.Contains(result, "let __result") {
		t.Error("Void method should not use __result pattern")
	}
	// But should still have ensures checks
	if !strings.Contains(result, "assert!") {
		t.Error("Expected ensures assertion for Void method")
	}
	if !strings.Contains(result, "__old_self_value") {
		t.Error("Expected old value capture")
	}
	if !strings.Contains(result, "self.value ==") {
		t.Error("Expected value comparison in ensures")
	}
}

func TestGenerateIfElse(t *testing.T) {
	source := `
module test version "1.0.0";

function max(a: Int, b: Int) returns Int {
	if a > b {
		return a;
	} else {
		return b;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "if (a > b)") {
		t.Error("Expected if condition")
	}
	if !strings.Contains(result, "} else {") {
		t.Error("Expected else clause")
	}
}

func TestGenerateBooleanOperators(t *testing.T) {
	source := `
module test version "1.0.0";

function logic(a: Bool, b: Bool) returns Bool {
	let x: Bool = a and b;
	let y: Bool = a or b;
	let z: Bool = not a;
	return x;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "a && b") {
		t.Error("Expected 'and' to become '&&'")
	}
	if !strings.Contains(result, "a || b") {
		t.Error("Expected 'or' to become '||'")
	}
	if !strings.Contains(result, "!a") {
		t.Error("Expected 'not' to become '!'")
	}
}

func TestGenerateArithmeticOperators(t *testing.T) {
	source := `
module test version "1.0.0";

function math(a: Int, b: Int) returns Int {
	let sum: Int = a + b;
	let diff: Int = a - b;
	let prod: Int = a * b;
	let quot: Int = a / b;
	let rem: Int = a % b;
	return sum;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	ops := []string{"+", "-", "*", "/", "%"}
	for _, op := range ops {
		if !strings.Contains(result, op) {
			t.Errorf("Expected operator %s", op)
		}
	}
}

func TestGenerateComparisonOperators(t *testing.T) {
	source := `
module test version "1.0.0";

function compare(a: Int, b: Int) returns Bool {
	let eq: Bool = a == b;
	let neq: Bool = a != b;
	let lt: Bool = a < b;
	let gt: Bool = a > b;
	let leq: Bool = a <= b;
	let geq: Bool = a >= b;
	return eq;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	ops := []string{"==", "!=", "<", ">", "<=", ">="}
	for _, op := range ops {
		if !strings.Contains(result, op) {
			t.Errorf("Expected operator %s", op)
		}
	}
}

func TestGenerateMethodCall(t *testing.T) {
	source := `
module test version "1.0.0";

entity Calculator {
	field value: Int;

	method add(n: Int) returns Void {
		self.value = self.value + n;
	}

	method getValue() returns Int {
		return self.value;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn add(&mut self, n: i64)") {
		t.Error("Expected method with mutable self parameter")
	}
	if !strings.Contains(result, "fn getValue(&mut self) -> i64") {
		t.Error("Expected method with return type")
	}
	if !strings.Contains(result, "self.value =") {
		t.Error("Expected self field access")
	}
}

func TestGenerateFieldAccess(t *testing.T) {
	source := `
module test version "1.0.0";

entity Point {
	field x: Int;
	field y: Int;

	constructor(px: Int, py: Int) {
		self.x = px;
		self.y = py;
	}

	method getX() returns Int {
		return self.x;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "__self.x = px") {
		t.Error("Expected __self field access in constructor")
	}
	if !strings.Contains(result, "return self.x") {
		t.Error("Expected self field access in method")
	}
}

func TestGenerateComplexEntity(t *testing.T) {
	source := `
module test version "1.0.0";

entity BankAccount {
	field balance: Int;
	field accountNumber: String;

	invariant balance >= 0;

	constructor(number: String)
	requires number != "" {
		self.accountNumber = number;
		self.balance = 0;
	}

	method deposit(amount: Int) returns Void
	requires amount > 0
	ensures self.balance == old(self.balance) + amount {
		self.balance = self.balance + amount;
	}

	method withdraw(amount: Int) returns Bool
	requires amount > 0 {
		if self.balance >= amount {
			self.balance = self.balance - amount;
			return true;
		}
		return false;
	}
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check struct definition
	if !strings.Contains(result, "struct BankAccount") {
		t.Error("Expected struct BankAccount")
	}

	// Check invariant method
	if !strings.Contains(result, "fn __check_invariants(&self)") {
		t.Error("Expected invariant check method")
	}
	if !strings.Contains(result, "balance >= 0") {
		t.Error("Expected invariant condition")
	}

	// Check constructor
	if !strings.Contains(result, "fn new(number: String) -> BankAccount") {
		t.Error("Expected constructor")
	}
	if !strings.Contains(result, "number != \"\"") {
		t.Error("Expected constructor precondition")
	}

	// Check deposit method
	if !strings.Contains(result, "fn deposit(&mut self, amount: i64)") {
		t.Error("Expected deposit method")
	}
	if !strings.Contains(result, "let __old_self_balance = self.balance") {
		t.Error("Expected old value capture in deposit")
	}

	// Check withdraw method
	if !strings.Contains(result, "fn withdraw(&mut self, amount: i64) -> bool") {
		t.Error("Expected withdraw method with bool return")
	}
	if !strings.Contains(result, "if (self.balance >= amount)") {
		t.Error("Expected conditional in withdraw")
	}

	// Check invariant calls
	if !strings.Contains(result, "__check_invariants") {
		t.Error("Expected invariant check calls")
	}
}

func TestCodegenWhileLoop(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        x = x + 1;
    }
    return x;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "while (x < 10i64)") {
		t.Error("Expected while loop with condition")
	}
	if !strings.Contains(result, "x = (x + 1i64);") {
		t.Error("Expected assignment inside while loop")
	}
}

func TestCodegenBreak(t *testing.T) {
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
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "break;") {
		t.Error("Expected break statement")
	}
	if !strings.Contains(result, "while") {
		t.Error("Expected while loop")
	}
}

func TestCodegenContinue(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable x: Int = 0;
    while x < 10 {
        x = x + 1;
        if x == 5 {
            continue;
        }
    }
    return x;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "continue;") {
		t.Error("Expected continue statement")
	}
	if !strings.Contains(result, "while") {
		t.Error("Expected while loop")
	}
}

func TestCodegenNestedWhile(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 3 {
        let mutable j: Int = 0;
        while j < 3 {
            j = j + 1;
        }
        i = i + 1;
    }
    return i;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Count while occurrences - should have at least 2
	count := strings.Count(result, "while")
	if count < 2 {
		t.Errorf("Expected at least 2 while loops, found %d", count)
	}
}

func TestCodegenPrintInt(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(42);
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "println!(\"{}\", 42i64);") {
		t.Error("Expected println!(\"{}\", 42i64);")
	}
}

func TestCodegenPrintString(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print("hello");
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "println!(\"{}\", \"hello\".to_string());") {
		t.Error("Expected println!(\"{}\", \"hello\".to_string());")
	}
}

func TestCodegenPrintBool(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    print(true);
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "println!(\"{}\", true);") {
		t.Error("Expected println!(\"{}\", true);")
	}
}

func TestCodegenPrintExpression(t *testing.T) {
	source := `module test version "1.0.0";

entry function main() returns Int {
    let x: Int = 10;
    print(x + 1);
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "println!(\"{}\", (x + 1i64));") {
		t.Error("Expected println!(\"{}\", (x + 1i64));")
	}
}

func TestCodegenArrayType(t *testing.T) {
	source := `module test version "1.0.0";

function test(arr: Array<Int>) returns Int {
    let x: Array<Int> = arr;
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "arr: &Vec<i64>") {
		t.Errorf("Expected arr: &Vec<i64> parameter (array params passed by ref), got:\n%s", result)
	}
	if !strings.Contains(result, "let x: Vec<i64> = arr.clone();") {
		t.Errorf("Expected let x: Vec<i64> = arr.clone(); (clone when assigning ref to owned), got:\n%s", result)
	}
}

func TestCodegenNestedArrayType(t *testing.T) {
	source := `module test version "1.0.0";

function test(arr: Array<Array<Int>>) returns Int {
    let x: Array<Array<Int>> = arr;
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "arr: &Vec<Vec<i64>>") {
		t.Errorf("Expected arr: &Vec<Vec<i64>> parameter (array params passed by ref), got:\n%s", result)
	}
	if !strings.Contains(result, "let x: Vec<Vec<i64>> = arr.clone();") {
		t.Errorf("Expected let x: Vec<Vec<i64>> = arr.clone(); (clone when assigning ref to owned), got:\n%s", result)
	}
}

func TestCodegenArrayLiteral(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "vec![1i64, 2i64, 3i64]") {
		t.Errorf("Expected vec![1i64, 2i64, 3i64], got:\n%s", result)
	}
}

func TestCodegenIndexExpr(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    let x: Int = arr[0];
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "arr[0i64 as usize]") {
		t.Errorf("Expected arr[0i64 as usize], got:\n%s", result)
	}
}

func TestCodegenLen(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    let n: Int = len(arr);
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "(arr.len() as i64)") {
		t.Errorf("Expected (arr.len() as i64), got:\n%s", result)
	}
}

func TestCodegenPush(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable arr: Array<Int> = [1, 2, 3];
    arr.push(5);
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "arr.push(5i64)") {
		t.Errorf("Expected arr.push(5i64), got:\n%s", result)
	}
}

func TestCodegenForInArray(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        print(x);
    }
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "for x in arr.iter()") {
		t.Errorf("Expected 'for x in arr.iter()', got:\n%s", result)
	}
}

func TestCodegenForInRange(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    for i in 0..10 {
        print(i);
    }
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "for i in (0i64..10i64)") {
		t.Errorf("Expected 'for i in (0i64..10i64)', got:\n%s", result)
	}
}

func TestCodegenForInWithBody(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let arr: Array<Int> = [1, 2, 3];
    for x in arr {
        if x > 1 {
            print(x);
        }
    }
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "for x in arr.iter()") {
		t.Errorf("Expected 'for x in arr.iter()', got:\n%s", result)
	}
	if !strings.Contains(result, "if (x > 1i64)") {
		t.Errorf("Expected 'if (x > 1i64)' inside loop body, got:\n%s", result)
	}
}

func TestCodegenWhileInvariant(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    while i < 10 invariant i >= 0 {
        i = i + 1;
    }
    return i;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "Loop invariant failed at entry") {
		t.Errorf("Expected 'Loop invariant failed at entry' assertion, got:\n%s", result)
	}
	if !strings.Contains(result, "Loop invariant failed after iteration") {
		t.Errorf("Expected 'Loop invariant failed after iteration' assertion, got:\n%s", result)
	}
	if !strings.Contains(result, "assert!((i >= 0i64)") {
		t.Errorf("Expected invariant assertion with condition, got:\n%s", result)
	}
}

func TestCodegenWhileDecreases(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable n: Int = 10;
    while n > 0 decreases n {
        n = n - 1;
    }
    return 0;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "let mut __decreases_prev: i64") {
		t.Errorf("Expected __decreases_prev variable, got:\n%s", result)
	}
	if !strings.Contains(result, "let __decreases_next: i64") {
		t.Errorf("Expected __decreases_next variable, got:\n%s", result)
	}
	if !strings.Contains(result, "Termination metric did not decrease") {
		t.Errorf("Expected strict decrease assertion, got:\n%s", result)
	}
	if !strings.Contains(result, "__decreases_next < __decreases_prev") {
		t.Errorf("Expected strict decrease check, got:\n%s", result)
	}
}

func TestCodegenWhileInvariantAndDecreases(t *testing.T) {
	source := `module test version "1.0.0";
entry function main() returns Int {
    let mutable i: Int = 0;
    let mutable n: Int = 10;
    while i < n invariant i >= 0 decreases n - i {
        i = i + 1;
    }
    return i;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "Loop invariant") {
		t.Errorf("Expected loop invariant assertions, got:\n%s", result)
	}
	if !strings.Contains(result, "__decreases_prev") {
		t.Errorf("Expected decreases tracking, got:\n%s", result)
	}
	if !strings.Contains(result, "Termination metric") {
		t.Errorf("Expected termination metric assertions, got:\n%s", result)
	}
}

func TestCodegenWhileInvariantWithOld(t *testing.T) {
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
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "__old_sum") {
		t.Errorf("Expected __old_sum capture variable, got:\n%s", result)
	}
	if !strings.Contains(result, "let __old_sum = sum;") {
		t.Errorf("Expected old value capture before loop, got:\n%s", result)
	}
}

func TestCodegenForallInEnsures(t *testing.T) {
	source := `module test version "1.0.0";

function check_positive(arr: Array<Int>, n: Int) returns Bool
    ensures forall i in 0..n: arr[i] >= 0
{
    return true;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "__forall_holds") {
		t.Errorf("Expected __forall_holds variable, got:\n%s", result)
	}
	if !strings.Contains(result, "for i in") {
		t.Errorf("Expected for loop with variable i, got:\n%s", result)
	}
	if !strings.Contains(result, "break") {
		t.Errorf("Expected break statement on failure, got:\n%s", result)
	}
}

func TestCodegenExistsInRequires(t *testing.T) {
	source := `module test version "1.0.0";

function find_target(arr: Array<Int>, n: Int, target: Int) returns Bool
    requires exists i in 0..n: arr[i] == target
{
    return false;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "__exists_found") {
		t.Errorf("Expected __exists_found variable, got:\n%s", result)
	}
	if !strings.Contains(result, "for i in") {
		t.Errorf("Expected for loop with variable i, got:\n%s", result)
	}
	if !strings.Contains(result, "break") {
		t.Errorf("Expected break statement on match, got:\n%s", result)
	}
}

func TestCodegenForallWithIndexExpr(t *testing.T) {
	source := `module test version "1.0.0";

function check_sorted(arr: Array<Int>) returns Bool
    ensures forall i in 0..len(result)-1: result[i] <= result[i+1]
{
    return true;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "[i as usize]") {
		t.Errorf("Expected index conversion [i as usize], got:\n%s", result)
	}
	if !strings.Contains(result, "[(i + 1i64) as usize]") && !strings.Contains(result, "as usize") {
		t.Errorf("Expected index conversion for i+1, got:\n%s", result)
	}
}

func TestCodegenEnumSimple(t *testing.T) {
	source := `module test version "1.0.0";

enum Status {
	Pending,
	Running,
	Complete,
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "#[derive(Clone, Debug)]") {
		t.Error("Expected #[derive(Clone, Debug)] for enum")
	}
	if !strings.Contains(result, "enum Status {") {
		t.Error("Expected enum Status definition")
	}
	if !strings.Contains(result, "Pending,") {
		t.Error("Expected Pending variant")
	}
	if !strings.Contains(result, "Running,") {
		t.Error("Expected Running variant")
	}
	if !strings.Contains(result, "Complete,") {
		t.Error("Expected Complete variant")
	}
}

func TestCodegenEnumDataVariants(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
	Rectangle(width: Float, height: Float),
	Point,
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "enum Shape {") {
		t.Error("Expected enum Shape definition")
	}
	if !strings.Contains(result, "Circle { radius: f64 }") {
		t.Error("Expected Circle variant with radius field")
	}
	if !strings.Contains(result, "Rectangle { width: f64, height: f64 }") {
		t.Error("Expected Rectangle variant with width and height fields")
	}
	if !strings.Contains(result, "Point,") {
		t.Error("Expected Point unit variant")
	}
}

func TestCodegenEnumVariantConstructor(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
	Circle(radius: Float),
}

function makeCircle(r: Float) returns Shape {
	return Circle(r);
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "Shape::Circle { radius: r }") {
		t.Errorf("Expected Shape::Circle constructor with named field, got:\n%s", result)
	}
}

func TestCodegenEnumUnitVariantUsage(t *testing.T) {
	source := `module test version "1.0.0";

enum Status {
	Running,
	Complete,
}

function getStatus() returns Status {
	return Running;
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "Status::Running") {
		t.Errorf("Expected Status::Running for unit variant, got:\n%s", result)
	}
}

func TestCodegenMatchSimple(t *testing.T) {
	source := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        Pending => 0,
        Complete => 2
    };
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check for match keyword
	if !strings.Contains(result, "match s {") {
		t.Errorf("Expected 'match s {', got:\n%s", result)
	}

	// Check for variant patterns
	if !strings.Contains(result, "Status::Running => 1i64") {
		t.Errorf("Expected 'Status::Running => 1i64', got:\n%s", result)
	}
	if !strings.Contains(result, "Status::Pending => 0i64") {
		t.Errorf("Expected 'Status::Pending => 0i64', got:\n%s", result)
	}
	if !strings.Contains(result, "Status::Complete => 2i64") {
		t.Errorf("Expected 'Status::Complete => 2i64', got:\n%s", result)
	}
}

func TestCodegenMatchWithDestructuring(t *testing.T) {
	source := `module test version "1.0.0";

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
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check for destructuring patterns
	if !strings.Contains(result, "Shape::Circle { radius: r }") {
		t.Errorf("Expected 'Shape::Circle { radius: r }', got:\n%s", result)
	}
	if !strings.Contains(result, "Shape::Rectangle { width: w, height: h }") {
		t.Errorf("Expected 'Shape::Rectangle { width: w, height: h }', got:\n%s", result)
	}
}

func TestCodegenMatchWithWildcard(t *testing.T) {
	source := `module test version "1.0.0";

enum Status { Running, Pending, Complete }

function f(s: Status) returns Int {
    return match s {
        Running => 1,
        _ => 0
    };
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check for wildcard pattern
	if !strings.Contains(result, "_ => 0i64") {
		t.Errorf("Expected '_ => 0i64', got:\n%s", result)
	}
}

func TestCodegenMatchNestedExpression(t *testing.T) {
	source := `module test version "1.0.0";

enum Shape {
    Circle(radius: Float)
}

function f(s: Shape) returns Float {
    return match s {
        Circle(r) => 3.14 * r * r
    };
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check that pattern and complex expression both appear
	if !strings.Contains(result, "Shape::Circle { radius: r }") {
		t.Errorf("Expected 'Shape::Circle { radius: r }', got:\n%s", result)
	}
	// Check for multiplication operations (may have parens)
	if !strings.Contains(result, "3.14 * r") || strings.Count(result, "* r") < 2 {
		t.Errorf("Expected multiplication expression with r in match arm body, got:\n%s", result)
	}
}

func TestCodegenTryExpr(t *testing.T) {
	source := `module test version "1.0.0";

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
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "parse(s)?") {
		t.Errorf("Expected 'parse(s)?', got:\n%s", result)
	}
}

func TestCodegenTryExprInLet(t *testing.T) {
	source := `module test version "1.0.0";

function step() returns Result<Int, String>
    requires true
    ensures true
{
    return Ok(42);
}

function f() returns Result<Int, String>
    requires true
    ensures true
{
    let val: Int = step()?;
    return Ok(val);
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "step()?") {
		t.Errorf("Expected 'step()?', got:\n%s", result)
	}
	if !strings.Contains(result, "let val: i64 = step()?;") {
		t.Errorf("Expected 'let val: i64 = step()?;', got:\n%s", result)
	}
}

func TestCodegenPredicateIsOk(t *testing.T) {
	source := `module test version "1.0.0";

function test(r: Result<Int, String>) returns Bool
    requires true
    ensures true
{
    return r.is_ok();
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "r.is_ok()") {
		t.Errorf("Expected 'r.is_ok()', got:\n%s", result)
	}
}

func TestCodegenPredicateIsErr(t *testing.T) {
	source := `module test version "1.0.0";

function test(r: Result<Int, String>) returns Bool
    requires true
    ensures true
{
    return r.is_err();
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "r.is_err()") {
		t.Errorf("Expected 'r.is_err()', got:\n%s", result)
	}
}

func TestCodegenEnsuresWithPredicate(t *testing.T) {
	source := `module test version "1.0.0";

function safe_divide(a: Int, b: Int) returns Result<Int, String>
    ensures result.is_ok() implies b != 0
{
    if b == 0 {
        return Err("division by zero");
    }
    return Ok(a / b);
}`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Check for the implies pattern: !result.is_ok() || b != 0
	if !strings.Contains(result, "__result.is_ok()") {
		t.Errorf("Expected '__result.is_ok()' in ensures check, got:\n%s", result)
	}
	if !strings.Contains(result, "b != 0i64") {
		t.Errorf("Expected 'b != 0i64' in ensures check, got:\n%s", result)
	}
}

// --- Multi-file GenerateAll tests ---

func TestGenerateAllTwoModules(t *testing.T) {
	mathSource := `module math version "0.1.0";

public function add(a: Int, b: Int) returns Int {
    return a + b;
}

public function multiply(a: Int, b: Int) returns Int {
    return a * b;
}

function internal_helper(x: Int) returns Int {
    return x + 1;
}`

	mainSource := `module main version "0.1.0";

import "math.intent";

entry function main() returns Int {
    let val: Int = math.add(3, 4);
    let product: Int = math.multiply(val, 2);
    print(product);
    return 0;
}`

	mathProg := parser.New(mathSource).Parse()
	mainProg := parser.New(mainSource).Parse()

	registry := map[string]*ast.Program{
		"/project/math.intent": mathProg,
		"/project/main.intent": mainProg,
	}
	sortedPaths := []string{"/project/math.intent", "/project/main.intent"}

	result := GenerateAll(registry, sortedPaths)

	// Check multi-file header
	if !strings.Contains(result, "multi-file") {
		t.Error("Expected multi-file comment in header")
	}

	// math module functions should be mangled with math_ prefix
	if !strings.Contains(result, "fn math_add(a: i64, b: i64) -> i64") {
		t.Errorf("Expected mangled function 'math_add', got:\n%s", result)
	}
	if !strings.Contains(result, "fn math_multiply(a: i64, b: i64) -> i64") {
		t.Errorf("Expected mangled function 'math_multiply', got:\n%s", result)
	}
	if !strings.Contains(result, "fn math_internal_helper(x: i64) -> i64") {
		t.Errorf("Expected mangled function 'math_internal_helper', got:\n%s", result)
	}

	// Entry file should NOT have mangled names
	if !strings.Contains(result, "fn __intent_main()") {
		t.Errorf("Expected un-mangled entry main function, got:\n%s", result)
	}

	// Calls to math module should use mangled names
	if !strings.Contains(result, "math_add(3i64, 4i64)") {
		t.Errorf("Expected 'math_add(3i64, 4i64)' call, got:\n%s", result)
	}
	if !strings.Contains(result, "math_multiply(val, 2i64)") {
		t.Errorf("Expected 'math_multiply(val, 2i64)' call, got:\n%s", result)
	}

	// Should have a fn main() wrapper
	if !strings.Contains(result, "fn main()") {
		t.Errorf("Expected 'fn main()' wrapper function, got:\n%s", result)
	}
}

func TestGenerateAllCrossModuleArrayParameter(t *testing.T) {
	validatorsSource := `module validators version "0.1.0";

public function validate(arr: Array<Int>) returns Bool {
    return len(arr) > 0;
}`

	mainSource := `module main version "0.1.0";

import "validators.intent";

entry function main() returns Int {
    let numbers: Array<Int> = [1, 2, 3];
    let valid: Bool = validators.validate(numbers);
    if valid {
        return 1;
    }
    return 0;
}`

	validatorsProg := parser.New(validatorsSource).Parse()
	mainProg := parser.New(mainSource).Parse()

	registry := map[string]*ast.Program{
		"/project/validators.intent": validatorsProg,
		"/project/main.intent":       mainProg,
	}
	sortedPaths := []string{"/project/validators.intent", "/project/main.intent"}

	result := GenerateAll(registry, sortedPaths)

	// Validators module function should be mangled with validators_ prefix and array parameter as reference
	if !strings.Contains(result, "fn validators_validate(arr: &Vec<i64>) -> bool") {
		t.Errorf("Expected mangled function 'validators_validate' with &Vec<i64> parameter, got:\n%s", result)
	}

	// Call to validators module should use mangled name and & prefix for array argument
	if !strings.Contains(result, "validators_validate(&numbers)") {
		t.Errorf("Expected 'validators_validate(&numbers)' call with & prefix, got:\n%s", result)
	}
}

func TestGenerateAllEntityMangling(t *testing.T) {
	geomSource := `module geometry version "0.1.0";

public entity Circle {
    field radius: Float;

    constructor(r: Float) {
        self.radius = r;
    }
}`

	mainSource := `module main version "0.1.0";

import "geometry.intent";

entry function main() returns Int {
    let c: Circle = geometry.Circle(5.0);
    return 0;
}`

	geomProg := parser.New(geomSource).Parse()
	mainProg := parser.New(mainSource).Parse()

	registry := map[string]*ast.Program{
		"/project/geometry.intent": geomProg,
		"/project/main.intent":    mainProg,
	}
	sortedPaths := []string{"/project/geometry.intent", "/project/main.intent"}

	result := GenerateAll(registry, sortedPaths)

	// Entity from non-entry module should be mangled with PascalCase prefix
	if !strings.Contains(result, "struct GeometryCircle") {
		t.Errorf("Expected mangled entity 'GeometryCircle', got:\n%s", result)
	}
	if !strings.Contains(result, "impl GeometryCircle") {
		t.Errorf("Expected 'impl GeometryCircle', got:\n%s", result)
	}

	// Constructor should use mangled name
	if !strings.Contains(result, "-> GeometryCircle") {
		t.Errorf("Expected constructor returning GeometryCircle, got:\n%s", result)
	}

	// Call from main should use mangled name
	if !strings.Contains(result, "GeometryCircle::new(5.0)") {
		t.Errorf("Expected 'GeometryCircle::new(5.0)' call, got:\n%s", result)
	}
}

func TestGenerateAllEmptyPaths(t *testing.T) {
	result := GenerateAll(nil, nil)
	if result != "" {
		t.Errorf("Expected empty result for nil paths, got: %s", result)
	}
}

func TestGenerateAllSingleFile(t *testing.T) {
	// Single file through GenerateAll should work (entry file, no mangling)
	source := `module main version "0.1.0";

entry function main() returns Int {
    return 42;
}`
	prog := parser.New(source).Parse()
	registry := map[string]*ast.Program{
		"/project/main.intent": prog,
	}
	sortedPaths := []string{"/project/main.intent"}

	result := GenerateAll(registry, sortedPaths)

	if !strings.Contains(result, "fn __intent_main()") {
		t.Errorf("Expected __intent_main, got:\n%s", result)
	}
	if !strings.Contains(result, "fn main()") {
		t.Errorf("Expected main wrapper, got:\n%s", result)
	}
}
