# Attractor Phase 2: String Standard Library

## Context

Phase 1 implemented the Attractor type model, edge selection, retry policy, and graph validation. The biggest hack remaining is `edge_matches_condition()`, which hard-codes ~15 string patterns instead of parsing conditions dynamically. The `normalize_label()` function is a stub that returns its input unchanged.

Both problems are blocked on String methods. This phase adds 6 methods to the String type: `len()`, `to_lowercase()`, `trim()`, `starts_with(prefix)`, `contains(substr)`, and `split(delim)`.

## Method Specifications

| Method | Signature | Returns | Rust mapping |
|--------|-----------|---------|-------------|
| `len` | `s.len()` | `Int` | `(s.len() as i64)` |
| `to_lowercase` | `s.to_lowercase()` | `String` | `s.to_lowercase()` |
| `trim` | `s.trim()` | `String` | `s.trim().to_string()` |
| `starts_with` | `s.starts_with(prefix)` | `Bool` | `s.starts_with(prefix.as_str())` |
| `contains` | `s.contains(substr)` | `Bool` | `s.contains(substr.as_str())` |
| `split` | `s.split(delim)` | `Array<String>` | `s.split(delim.as_str()).map(\|s\| s.to_string()).collect::<Vec<String>>()` |

### JS backend mapping

| Method | JS mapping |
|--------|-----------|
| `len` | `BigInt(s.length)` |
| `to_lowercase` | `s.toLowerCase()` |
| `trim` | `s.trim()` |
| `starts_with` | `s.startsWith(prefix)` |
| `contains` | `s.includes(substr)` |
| `split` | `s.split(delim)` |

### WASM backend

String methods in the WASM backend will be stubs (matching the existing pattern for array methods). Full WASM string support requires a string runtime, which is out of scope.

## Implementation Plan

No lexer, parser, or AST changes are needed. Method calls are already parsed generically as `MethodCallExpr{Object, Method, Args}`. The work is entirely in the checker and the code generation backends.

### Bug 1: Checker — type-check String methods

**File:** `internal/checker/checker.go` — `checkMethodCallExpr`

**Change:** After the existing Array method block (`if objType.Name == "Array"`) and before the entity method check (`if !objType.IsEntity`), add a String method block:

- `len()`: no args, returns `TypeInt`
- `to_lowercase()`, `trim()`: no args, returns `TypeString`
- `starts_with(s)`, `contains(s)`: 1 String arg, returns `TypeBool`
- `split(s)`: 1 String arg, returns `Array<String>` (i.e., `&Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{TypeString}}`)
- Unknown method on String: error

**Tests:**
- `TestCheckStringLen` — `s.len()` returns Int
- `TestCheckStringToLowercase` — `s.to_lowercase()` returns String
- `TestCheckStringTrim` — `s.trim()` returns String
- `TestCheckStringStartsWith` — `s.starts_with("x")` returns Bool
- `TestCheckStringContains` — `s.contains("x")` returns Bool
- `TestCheckStringSplit` — `s.split(",")` returns Array\<String\>
- `TestCheckStringMethodChaining` — `s.trim().to_lowercase()` works
- `TestCheckStringMethodWrongArgs` — `s.to_lowercase("extra")` errors
- `TestCheckStringMethodBadArgType` — `s.starts_with(42)` errors
- `TestCheckStringUnknownMethod` — `s.reverse()` errors

### Bug 2: Legacy codegen — generate Rust for String methods

**File:** `internal/codegen/codegen.go` — `generateExpr`, `*ast.MethodCallExpr` case

**Change:** After the `is_ok`/`is_err`/`is_some`/`is_none` block (~line 1008) and before the generic method call fallthrough, add a switch on `expr.Method` for the 6 String methods. Generate the Rust mappings from the table above.

Note: `trim()` returns `&str` in Rust, so we need `.to_string()` to get back to owned `String`. `starts_with`/`contains` take `&str` patterns, so the argument needs `.as_str()`.

**Tests:**
- `TestGenerateStringToLowercase` — output contains `.to_lowercase()`
- `TestGenerateStringSplit` — output contains `.split(...)...collect::<Vec<String>>()`
- `TestGenerateStringLen` — output contains `.len() as i64`

### Bug 3: Rust backend — generate Rust for String methods

**File:** `internal/rustbe/rustbe.go` — `generateMethodCallExpr`

**Change:** Same pattern as legacy codegen. Add String method cases after `is_ok`/`is_err`/`is_some`/`is_none` and before the generic fallthrough.

**Tests:**
- Add test cases in `internal/rustbe/rustbe_test.go` following existing method call test patterns

### Bug 4: JS backend — generate JS for String methods

**File:** `internal/jsbe/jsbe.go` — `generateMethodCallExpr`

**Change:** Add String method cases using JS mappings. Note: JS uses `.length` (property) not `.length()` (method), and `.includes()` not `.contains()`.

**Tests:**
- Add test cases in `internal/jsbe/jsbe_test.go`

### Bug 5: WASM backend — stub String methods

**File:** `internal/wasmbe/wasmbe.go` — `compileMethodCallExpr`

**Change:** Add stubs for String methods matching the existing pattern for array method stubs (`push`, `pop`). This prevents the "regular method call — simplified" fallthrough for String methods.

### Bug 6: Update Attractor — implement condition parser

**File:** `examples/attractor/edge_selection.intent` (and `attractor.intent` single-file)

**Changes:**
- Replace `normalize_label` stub with `label.trim().to_lowercase()`
- Replace `edge_matches_condition` hard-coded patterns with dynamic parsing:
  - `condition.split(" && ")` to get clauses
  - For each clause, parse key/operator/value using `starts_with` and string position logic
  - Call `evaluate_clause` for each parsed clause
- Update `evaluate_clause` to use `starts_with("context.")` for context key detection

### Bug 7: ADR — document String standard library decision

**File:** `docs/decisions/0013-string-standard-library.md`

Document the decision to add built-in String methods vs. alternatives (free functions, trait-based extension, FFI to Rust stdlib).

## Design Decisions

### Method form vs. free function form

`s.len()` (method) coexists with `len(arr)` (free function on Arrays). Both are valid in the checker — they're different code paths (`checkMethodCallExpr` vs `checkCallExpr`). No conflict. Eventually we may want `len(s)` to work too, but that's a separate change (update `checkCallExpr` to accept String). Not in scope for Phase 2.

### Method chaining

`s.trim().to_lowercase()` works automatically because the parser's `parsePostfix` loop builds nested `MethodCallExpr` nodes, and both methods return `String`, so the checker resolves the chain type correctly. No special handling needed.

### `split` return type

`split` returns `Array<String>`. This is the first built-in method that returns a generic type. The checker constructs the type as `&Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{TypeString}}`. This matches how Array types are represented elsewhere in the checker.

## Execution Order

Bugs 1-5 (compiler changes) must be done before Bug 6 (Attractor update).

**Recommended order:**
1. Bug 1 (checker) — enables type-checking String method calls
2. Bug 2 (legacy codegen) — enables `intentc build --emit-rust` for String methods
3. Bug 3 (rustbe) — enables `intentc build` for String methods
4. Bug 4 (jsbe) — enables `intentc build --target js` for String methods
5. Bug 5 (wasmbe) — stubs for completeness
6. Bug 6 (attractor update) — validates the feature end-to-end
7. Bug 7 (ADR) — documents the decision

Bugs 2-5 (four codegen backends) are independent and can be parallelized.

## Verification

After all changes:
1. `go test ./... -timeout 30s` — all tests pass (including new String method tests)
2. `./intentc check examples/attractor/attractor.intent` — no errors
3. `./intentc check examples/attractor/main.intent` — no errors
4. `./intentc build examples/attractor/attractor.intent` — compiles and runs
5. `./intentc build examples/attractor/main.intent` — compiles and runs
6. Updated condition parser handles compound `&&` conditions dynamically
7. `normalize_label` produces lowercase trimmed output
8. `make test && make check-examples && make emit-examples` — full regression
