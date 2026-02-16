package testgen

import (
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

func TestGenerateTestForSimpleFunction(t *testing.T) {
	source := `
module test version "1.0.0";

function fib(n: Int) returns Int
    requires n >= 0
    ensures result >= 0
{
    if n <= 0 { return 0; }
    if n == 1 { return 1; }
    let mutable a: Int = 0;
    let mutable b: Int = 1;
    let mutable i: Int = 2;
    while i <= n {
        let temp: Int = a + b;
        a = b;
        b = temp;
        i = i + 1;
    }
    return b;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "#[cfg(test)]") {
		t.Error("Expected #[cfg(test)] module attribute")
	}
	if !strings.Contains(result, "mod __contract_tests") {
		t.Error("Expected __contract_tests module")
	}
	if !strings.Contains(result, "fn test_fib_contracts()") {
		t.Error("Expected test_fib_contracts test function")
	}
	if !strings.Contains(result, "#[test]") {
		t.Error("Expected #[test] attribute")
	}
	if !strings.Contains(result, "__xorshift64") {
		t.Error("Expected xorshift64 PRNG helper")
	}
	if !strings.Contains(result, "__rand_range") {
		t.Error("Expected rand_range helper")
	}
	// Should have precondition filter
	if !strings.Contains(result, "if !((n >= 0i64)) { continue; }") {
		t.Error("Expected precondition filter for n >= 0")
	}
	// Should have postcondition check
	if !strings.Contains(result, "__result >= 0i64") {
		t.Error("Expected postcondition check for result >= 0")
	}
	// Should call the function
	if !strings.Contains(result, "let __result = fib(n)") {
		t.Error("Expected function call to fib(n)")
	}
}

func TestGenerateTestNoContracts(t *testing.T) {
	source := `
module test version "1.0.0";

function add(a: Int, b: Int) returns Int {
    return a + b;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if result != "" {
		t.Errorf("Expected empty output for function without contracts, got:\n%s", result)
	}
}

func TestGenerateTestEntryFunction(t *testing.T) {
	source := `
module test version "1.0.0";

entry function main() returns Int
    requires true
    ensures result >= 0
{
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	// Entry functions should be skipped
	if strings.Contains(result, "test_main") {
		t.Error("Expected entry function to be skipped")
	}
}

func TestGenerateTestEntityConstructor(t *testing.T) {
	source := `
module test version "1.0.0";

entity BankAccount {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initial_balance: Int)
        requires initial_balance >= 0
        ensures self.balance == initial_balance
    {
        self.balance = initial_balance;
    }
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_bankaccount_constructor_contracts()") {
		t.Error("Expected constructor test function")
	}
	if !strings.Contains(result, "BankAccount::new(") {
		t.Error("Expected BankAccount::new() call")
	}
	// Should check constructor postcondition
	if !strings.Contains(result, "__entity.balance == initial_balance") {
		t.Error("Expected constructor postcondition check")
	}
	// Should check invariant
	if !strings.Contains(result, "__entity.balance >= 0i64") {
		t.Error("Expected invariant check after construction")
	}
}

func TestGenerateTestEntityMethod(t *testing.T) {
	source := `
module test version "1.0.0";

entity BankAccount {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(initial_balance: Int)
        requires initial_balance >= 0
    {
        self.balance = initial_balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_bankaccount_deposit_contracts()") {
		t.Error("Expected deposit test function")
	}
	// Should capture old value
	if !strings.Contains(result, "__old_self_balance") {
		t.Error("Expected old value capture for self.balance")
	}
	// Should check postcondition with old value
	if !strings.Contains(result, "__entity.balance ==") {
		t.Error("Expected postcondition check with old value")
	}
	// Should check invariant
	if !strings.Contains(result, "Invariant") {
		t.Error("Expected invariant check after method call")
	}
	// Should re-construct entity after each iteration
	if !strings.Contains(result, "__entity = BankAccount::new(") {
		t.Error("Expected entity re-construction for next iteration")
	}
}

func TestGenerateTestArrayParam(t *testing.T) {
	source := `
module test version "1.0.0";

function check_sorted(arr: Array<Int>) returns Bool
    requires len(arr) > 0
{
    return true;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_check_sorted_contracts()") {
		t.Error("Expected check_sorted test function")
	}
	// Should generate array values
	if !strings.Contains(result, "Vec<Vec<i64>>") {
		t.Error("Expected Vec<Vec<i64>> type for array values")
	}
	if !strings.Contains(result, "vec![") {
		t.Error("Expected vec! literal for array values")
	}
	// Should pass array by reference
	if !strings.Contains(result, "check_sorted(&arr)") {
		t.Error("Expected array passed by reference")
	}
	// Should filter by len precondition
	if !strings.Contains(result, "arr.len() as i64") {
		t.Error("Expected len check in precondition filter")
	}
}

func TestGenerateTestForallConstraint(t *testing.T) {
	source := `
module test version "1.0.0";

function is_all_positive(arr: Array<Int>) returns Bool
    requires len(arr) > 0
    requires forall i in 0..len(arr): arr[i] > 0
{
    return true;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_is_all_positive_contracts()") {
		t.Error("Expected is_all_positive test function")
	}
	// Should have forall precondition filter
	if !strings.Contains(result, "__forall_holds") {
		t.Error("Expected forall quantifier in precondition filter")
	}
}

func TestGenerateTestMultipleRequires(t *testing.T) {
	source := `
module test version "1.0.0";

function clamp(n: Int, lo: Int, hi: Int) returns Int
    requires lo >= 0
    requires hi >= lo
    ensures result >= lo
    ensures result <= hi
{
    if n < lo { return lo; }
    if n > hi { return hi; }
    return n;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_clamp_contracts()") {
		t.Error("Expected clamp test function")
	}
	// Should have both precondition filters
	if !strings.Contains(result, "lo >= 0i64") {
		t.Error("Expected first precondition filter")
	}
	if !strings.Contains(result, "hi >= lo") {
		t.Error("Expected second precondition filter")
	}
	// Should check both postconditions
	if !strings.Contains(result, "__result >= lo") {
		t.Error("Expected first postcondition check")
	}
	if !strings.Contains(result, "__result <= hi") {
		t.Error("Expected second postcondition check")
	}
}

func TestGenerateTestWorkflow(t *testing.T) {
	source := `
module test version "1.0.0";

entity Counter {
    field value: Int;

    invariant self.value >= 0;

    constructor(initial: Int)
        requires initial >= 0
    {
        self.value = initial;
    }

    method increment() returns Void {
        self.value = self.value + 1;
    }
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_counter_workflow()") {
		t.Error("Expected workflow test function")
	}
	if !strings.Contains(result, "Counter::new(") {
		t.Error("Expected Counter construction in workflow")
	}
	if !strings.Contains(result, "__entity.increment()") {
		t.Error("Expected increment call in workflow")
	}
	// Should check invariant after method call
	if !strings.Contains(result, "Invariant") {
		t.Error("Expected invariant check in workflow")
	}
}

func TestConstraintAnalysis(t *testing.T) {
	// Test basic lower bound
	t.Run("LowerBound", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(n: Int) returns Int
    requires n >= 0
{ return n; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["n"]
		if c == nil {
			t.Fatal("Expected constraint for n")
		}
		if c.Lower == nil || *c.Lower != 0 {
			t.Errorf("Expected lower bound 0, got %v", c.Lower)
		}
	})

	// Test upper bound
	t.Run("UpperBound", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(n: Int) returns Int
    requires n <= 100
{ return n; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["n"]
		if c == nil {
			t.Fatal("Expected constraint for n")
		}
		if c.Upper == nil || *c.Upper != 100 {
			t.Errorf("Expected upper bound 100, got %v", c.Upper)
		}
	})

	// Test strict greater than
	t.Run("StrictGT", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(n: Int) returns Int
    requires n > 0
{ return n; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["n"]
		if c == nil {
			t.Fatal("Expected constraint for n")
		}
		if c.Lower == nil || *c.Lower != 1 {
			t.Errorf("Expected lower bound 1 (from n > 0), got %v", c.Lower)
		}
	})

	// Test not equal
	t.Run("NotEqual", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(n: Int) returns Int
    requires n != 0
{ return n; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["n"]
		if c == nil {
			t.Fatal("Expected constraint for n")
		}
		if len(c.NotEqual) != 1 || c.NotEqual[0] != 0 {
			t.Errorf("Expected NotEqual [0], got %v", c.NotEqual)
		}
	})

	// Test AND combination
	t.Run("AndCombination", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(n: Int) returns Int
    requires n >= 0 and n <= 100
{ return n; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["n"]
		if c == nil {
			t.Fatal("Expected constraint for n")
		}
		if c.Lower == nil || *c.Lower != 0 {
			t.Errorf("Expected lower bound 0, got %v", c.Lower)
		}
		if c.Upper == nil || *c.Upper != 100 {
			t.Errorf("Expected upper bound 100, got %v", c.Upper)
		}
	})

	// Test len constraint
	t.Run("LenConstraint", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(arr: Array<Int>) returns Int
    requires len(arr) > 0
{ return 0; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["arr"]
		if c == nil {
			t.Fatal("Expected constraint for arr")
		}
		if c.TypeName != "Array" {
			t.Errorf("Expected Array type, got %s", c.TypeName)
		}
		if c.ElemType != "Int" {
			t.Errorf("Expected Int element type, got %s", c.ElemType)
		}
		if c.MinLen == nil || *c.MinLen != 1 {
			t.Errorf("Expected MinLen 1 (from len(arr) > 0), got %v", c.MinLen)
		}
	})

	// Test forall element bounds
	t.Run("ForallElementBounds", func(t *testing.T) {
		source := `
module test version "1.0.0";
function f(arr: Array<Int>) returns Int
    requires forall i in 0..len(arr): arr[i] > 0
{ return 0; }
entry function main() returns Int { return 0; }
`
		prog := parser.New(source).Parse()
		fn := prog.Functions[0]
		constraints := AnalyzeConstraints(fn.Params, fn.Requires)

		c := constraints["arr"]
		if c == nil {
			t.Fatal("Expected constraint for arr")
		}
		if c.ElemLower == nil || *c.ElemLower != 1 {
			t.Errorf("Expected ElemLower 1 (from arr[i] > 0), got %v", c.ElemLower)
		}
	})
}

func TestValueGeneration(t *testing.T) {
	t.Run("IntValues", func(t *testing.T) {
		c := &ParamConstraint{
			Name:     "n",
			TypeName: "Int",
			Lower:    int64Ptr(0),
			Upper:    int64Ptr(100),
		}
		values := GenerateIntValues(c)
		if len(values) == 0 {
			t.Fatal("Expected non-empty values")
		}
		// Should include boundary values
		found0 := false
		found100 := false
		for _, v := range values {
			if v == "0i64" {
				found0 = true
			}
			if v == "100i64" {
				found100 = true
			}
		}
		if !found0 {
			t.Error("Expected 0i64 in boundary values")
		}
		if !found100 {
			t.Error("Expected 100i64 in boundary values")
		}
	})

	t.Run("IntWithExclusion", func(t *testing.T) {
		c := &ParamConstraint{
			Name:     "n",
			TypeName: "Int",
			Lower:    int64Ptr(0),
			NotEqual: []int64{0},
		}
		values := GenerateIntValues(c)
		// 0 should be excluded from boundary values
		for _, v := range values[:6] { // check first few boundary values
			if v == "0i64" {
				t.Error("Expected 0i64 to be excluded")
			}
		}
	})

	t.Run("FloatValues", func(t *testing.T) {
		c := &ParamConstraint{
			Name:     "f",
			TypeName: "Float",
		}
		values := GenerateFloatValues(c)
		if len(values) == 0 {
			t.Fatal("Expected non-empty values")
		}
		// Should include 0.0
		found := false
		for _, v := range values {
			if strings.Contains(v, "0") && strings.Contains(v, "f64") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected a zero-like value in float values")
		}
	})

	t.Run("BoolValues", func(t *testing.T) {
		values := GenerateBoolValues()
		if len(values) != 2 {
			t.Errorf("Expected 2 bool values, got %d", len(values))
		}
	})

	t.Run("StringValues", func(t *testing.T) {
		values := GenerateStringValues()
		if len(values) != 3 {
			t.Errorf("Expected 3 string values, got %d", len(values))
		}
		// Should contain .to_string()
		for _, v := range values {
			if !strings.Contains(v, ".to_string()") {
				t.Errorf("Expected .to_string() suffix, got %s", v)
			}
		}
	})

	t.Run("ArrayValues", func(t *testing.T) {
		c := &ParamConstraint{
			Name:     "arr",
			TypeName: "Array",
			ElemType: "Int",
			MinLen:   int64Ptr(1),
		}
		values := GenerateArrayIntValues(c)
		if len(values) == 0 {
			t.Fatal("Expected non-empty array values")
		}
		for _, v := range values {
			if !strings.HasPrefix(v, "vec![") {
				t.Errorf("Expected vec! prefix, got %s", v)
			}
		}
	})

	t.Run("ArrayWithElementBounds", func(t *testing.T) {
		c := &ParamConstraint{
			Name:      "arr",
			TypeName:  "Array",
			ElemType:  "Int",
			MinLen:    int64Ptr(2),
			ElemLower: int64Ptr(1),
			ElemUpper: int64Ptr(10),
		}
		values := GenerateArrayIntValues(c)
		if len(values) == 0 {
			t.Fatal("Expected non-empty array values")
		}
		// Each array should have at least 2 elements
		for _, v := range values {
			// Count commas + 1 = element count (rough check)
			elems := strings.Count(v, "i64")
			if elems < 2 {
				t.Errorf("Expected at least 2 elements, got %s", v)
			}
		}
	})
}

func TestGenerateTestOnlyRequires(t *testing.T) {
	source := `
module test version "1.0.0";

function divide(a: Int, b: Int) returns Int
    requires b != 0
{
    return a / b;
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_divide_contracts()") {
		t.Error("Expected divide test function")
	}
	// Should have precondition filter
	if !strings.Contains(result, "b != 0i64") {
		t.Error("Expected precondition filter for b != 0")
	}
	// Should call function (no postcondition to check, so no __result)
	if !strings.Contains(result, "divide(") {
		t.Error("Expected function call to divide")
	}
}

func TestGenerateTestStringParam(t *testing.T) {
	source := `
module test version "1.0.0";

entity Account {
    field name: String;

    constructor(name: String)
        requires true
    {
        self.name = name;
    }
}

entry function main() returns Int {
    return 0;
}
`
	prog := parser.New(source).Parse()
	result := Generate(prog)

	if !strings.Contains(result, "fn test_account_constructor_contracts()") {
		t.Error("Expected constructor test function")
	}
	// String values should use .to_string()
	if !strings.Contains(result, ".to_string()") {
		t.Error("Expected .to_string() in string test values")
	}
	// String params should use .clone()
	if !strings.Contains(result, ".clone()") {
		t.Error("Expected .clone() for string parameter passing")
	}
}
