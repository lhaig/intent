package lsp

import (
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// Phase 19 task 19.1: scope resolver tests.

func parseProg(t *testing.T, src string) *scopeResolver {
	t.Helper()
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("test"))
	}
	return newScopeResolver(prog, "test.intent")
}

func TestScopeResolveLetInFunctionBody(t *testing.T) {
	src := `module m version "1.0";
function f(a: Int) returns Int {
    let x: Int = 10;
    let y: Int = x + 1;
    return y;
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	// 'x' on the line of 'let y' (line 4) should resolve.
	got := r.resolveLocal(4, 22, "x")
	if got == nil || got.Kind != localLet || got.Name != "x" {
		t.Errorf("expected local 'x' resolved, got %+v", got)
	}
	// 'x' before its declaration (line 2) should not resolve.
	got = r.resolveLocal(2, 10, "x")
	if got != nil {
		t.Errorf("expected no resolution before let, got %+v", got)
	}
}

func TestScopeResolveParam(t *testing.T) {
	src := `module m version "1.0";
function f(a: Int, b: Int) returns Int {
    return a + b;
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	got := r.resolveLocal(3, 12, "a")
	if got == nil || got.Kind != localParam || got.Name != "a" {
		t.Errorf("expected param 'a' resolved, got %+v", got)
	}
}

func TestScopeResolveSelfInsideMethod(t *testing.T) {
	src := `module m version "1.0";
entity E {
    field n: Int;
    method get() returns Int { return self.n; }
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	got := r.resolveLocal(4, 38, "self")
	if got == nil || got.Kind != localSelf {
		t.Errorf("expected self resolved, got %+v", got)
	}
	if got != nil && got.Entity != nil && got.Entity.Name != "E" {
		t.Errorf("expected self bound to entity E, got %q", got.Entity.Name)
	}
}

func TestScopeResolveOutsideFunctionReturnsNil(t *testing.T) {
	src := `module m version "1.0";
function f() returns Int { return 0; }
entry function main() returns Int { return f(); }
`
	r := parseProg(t, src)
	got := r.resolveLocal(1, 5, "f") // on the module line
	if got != nil {
		t.Errorf("expected no local at module line, got %+v", got)
	}
}

func TestScopeInScopeLocalsCollectsParamsAndLets(t *testing.T) {
	src := `module m version "1.0";
function f(a: Int) returns Int {
    let x: Int = 1;
    let y: Int = 2;
    return a + x + y;
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	// On the 'return' line (line 5), all of a, x, y are visible.
	locals := r.inScopeLocals(5, 5)
	names := map[string]bool{}
	for _, l := range locals {
		names[l.Name] = true
	}
	for _, want := range []string{"a", "x", "y"} {
		if !names[want] {
			t.Errorf("expected %q in inScopeLocals, got %v", want, names)
		}
	}
}

func TestScopeInScopeLocalsExcludesFutureLets(t *testing.T) {
	src := `module m version "1.0";
function f() returns Int {
    let a: Int = 1;
    let b: Int = a + 1;
    let c: Int = b + 1;
    return c;
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	// On the 'let b' line (line 4), 'a' is visible, 'b' and 'c' are not
	// (b is on the same line at later column; c is a later line).
	locals := r.inScopeLocals(4, 5)
	names := map[string]bool{}
	for _, l := range locals {
		names[l.Name] = true
	}
	if !names["a"] {
		t.Errorf("expected 'a' visible at column before 'b' decl, got %v", names)
	}
	if names["c"] {
		t.Errorf("'c' should not be visible before its declaration, got %v", names)
	}
}

func TestFindFieldOnEntity(t *testing.T) {
	src := `module m version "1.0";
entity Point {
    field x: Int;
    field y: Int;
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	ent := r.prog.Entities[0]
	if f := findFieldOnEntity(ent, "x"); f == nil || f.Name != "x" {
		t.Errorf("expected to find field x, got %+v", f)
	}
	if f := findFieldOnEntity(ent, "z"); f != nil {
		t.Errorf("expected no field z, got %+v", f)
	}
}

func TestFindMethodOnEntity(t *testing.T) {
	src := `module m version "1.0";
entity E {
    method foo() returns Int { return 0; }
}
entry function main() returns Int { return 0; }
`
	r := parseProg(t, src)
	ent := r.prog.Entities[0]
	if m := findMethodOnEntity(r.prog, ent, "foo"); m == nil || m.Name != "foo" {
		t.Errorf("expected to find method foo, got %+v", m)
	}
	if m := findMethodOnEntity(r.prog, ent, "bar"); m != nil {
		t.Errorf("expected no method bar, got %+v", m)
	}
}
