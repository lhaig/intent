# Phase 11: User-Defined Generics

**Status:** Shipped (v1.1, 2026-03-25; checklist verified retroactively in Phase 14)
**Milestone:** v1.1 — Attractor Integration & Feature Expansion
**Decision:** [ADR 0025](../../docs/decisions/0025-user-defined-generics-design.md)

## Goal

Enable user-defined generic entities and functions in Intent, with monomorphization-based compilation across all three backends (Rust, JS, WASM).

## Success Criteria

- [x] `entity Stack<T> { ... }` parses and type-checks
- [x] `function identity<T>(x: T) returns T` parses and type-checks
- [x] `let s: Stack<Int> = Stack<Int>();` instantiates with concrete types
- [x] Contracts work with type parameters: `ensures result == x` where `x: T`
- [x] Rust backend emits monomorphized structs and functions
- [x] JS backend emits monomorphized classes and functions
- [x] All existing tests pass (no regressions)
- [x] New tests cover: generic parsing, type checking, monomorphization, codegen
- [x] `examples/generic_stack.intent` compiles and runs on Rust and JS (JS verified end-to-end; Rust source verified, full `cargo build` not run locally)
- [x] `intentc fmt` handles `<T>` in declarations and instantiations

> Verified retroactively under [Phase 14](phase-14-phase11-13-gaps.md).

## Reference

- Design: `docs/decisions/0025-user-defined-generics-design.md`
- Type system: `internal/checker/types.go`
- AST: `internal/ast/nodes.go`
- Parser: `internal/parser/parser.go`

## Tasks

### 11.1 Parser + AST

**Files:** `internal/ast/nodes.go`, `internal/parser/parser.go`

Add `TypeParam` AST node:
```go
type TypeParam struct {
    Name   string
    Line   int
    Column int
}
```

Add `TypeParams []*TypeParam` to `EntityDecl` and `FunctionDecl`.

Parse:
- `entity Name<T, U> { ... }` -- type params after entity name
- `function name<T>(...) returns T` -- type params after function name
- `Stack<Int>` in type position -- already handled by `TypeRef.TypeArgs`

**Acceptance:**
- `go test ./internal/parser/... -v` passes with new tests for generic syntax
- Existing parser tests still pass

### 11.2 Type System + Checker

**Files:** `internal/checker/types.go`, `internal/checker/checker.go`, `internal/checker/scope.go`

Add to `Type`:
```go
IsTypeParam bool // true for unresolved type parameters like T
```

Checker changes:
1. When entering a generic entity/function, push type params into scope as `Type{Name: "T", IsTypeParam: true}`
2. Fields, method params, return types, contracts can reference `T`
3. At instantiation site (`Stack<Int>`), build substitution map `{T: Int}` and validate
4. Type equality: `Stack<Int> != Stack<String>`
5. Error on: unused type params, wrong number of type args, type param in non-generic context

**Acceptance:**
- `go test ./internal/checker/... -v` passes with tests for:
  - Valid generic entity declaration
  - Valid generic function declaration
  - Type mismatch at instantiation (`Stack<Int>` used where `Stack<String>` expected)
  - Wrong number of type args
  - Unused type parameter warning

### 11.3 IR Monomorphization

**Files:** `internal/ir/lower.go`, `internal/ir/nodes.go`, `internal/ir/validate.go`

During IR lowering:
1. Track all concrete instantiations encountered (e.g., `Stack<Int>`, `Stack<String>`)
2. For each unique instantiation, clone the generic entity's IR with substituted types
3. Name mangling: `Stack<Int>` becomes `Stack__Int`, `Stack<Array<Int>>` becomes `Stack__Array_Int`
4. Clone methods with substituted param/return types and contract expressions
5. Replace references to generic entity with monomorphized name

**Acceptance:**
- `go test ./internal/ir/... -v` passes
- IR validation accepts monomorphized entities
- `TestRoundTripAllExamples` passes with `generic_stack.intent`

### 11.4 Backend Updates

**Files:** `internal/rustbe/rustbe.go`, `internal/jsbe/jsbe.go`, `internal/wasmbe/`

Rust backend:
- `mapType()` handles monomorphized names (`Stack__Int` -> `StackInt` struct)
- Constructor calls use monomorphized names
- Methods emitted per-instantiation

JS backend:
- Same monomorphization approach as Rust
- Class names use monomorphized names

WASM backend:
- Monomorphized types in function signatures

**Acceptance:**
- `go test ./internal/rustbe/... -v` and `go test ./internal/jsbe/... -v` pass
- `intentc build --emit-rust examples/generic_stack.intent` produces valid Rust
- `intentc build --target js --emit examples/generic_stack.intent` produces valid JS
- Both compile and run correctly

### 11.5 Formatter + Linter + Example

**Files:** `internal/formatter/formatter.go`, `internal/linter/linter.go`, `examples/generic_stack.intent`

Formatter:
- `formatTypeRef()` already handles `<T>` via `TypeArgs`
- `formatEntityDecl()` emits type params after name
- `formatFunctionDecl()` emits type params after name

Linter:
- Warn on unused type parameters in generic entities/functions

Example (`examples/generic_stack.intent`):
```intent
module generic_stack version "0.1.0";

entity Stack<T> {
    field items: Array<T>;
    field count: Int;

    invariant self.count >= 0;

    constructor()
        ensures self.count == 0
    {
        self.items = [];
        self.count = 0;
    }

    method push(item: T) returns Void
        ensures self.count == old(self.count) + 1
    {
        self.items.push(item);
        self.count = self.count + 1;
    }

    method is_empty() returns Bool
        ensures (result == true) implies (self.count == 0)
    {
        return self.count == 0;
    }
}

entry function main() returns Int {
    let mutable int_stack: Stack<Int> = Stack<Int>();
    int_stack.push(10);
    int_stack.push(20);
    print(int_stack.count);

    let mutable str_stack: Stack<String> = Stack<String>();
    str_stack.push("hello");
    print(str_stack.is_empty());

    return 0;
}
```

**Acceptance:**
- `intentc fmt --check examples/generic_stack.intent` passes
- `intentc lint examples/generic_stack.intent` produces no errors
- All existing examples still pass `make check-examples`

### 11.6 ADR + Documentation

**Files:** `docs/decisions/0025-user-defined-generics-design.md`, `docs/DESIGN.md`, `INTENT.md`, `docs/grammar.ebnf`

- Update ADR 0025 status from "Proposed" to "Accepted"
- Update DESIGN.md: move generics from non-goals to implemented
- Update INTENT.md: add Generics section with syntax and examples
- Update grammar.ebnf: add type_param productions
- Run `gofmt -w` on all changed Go files before committing

**Acceptance:**
- All docs consistent with implementation
- `go test ./... -timeout 30s` passes (all 12 packages)
- `make clean` leaves no artifacts
