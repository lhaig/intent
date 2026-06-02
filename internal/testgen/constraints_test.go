package testgen

import (
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

func TestConstraintAnalysis(t *testing.T) {
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
