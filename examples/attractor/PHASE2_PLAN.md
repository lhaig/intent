# Attractor Phase 2: String Standard Library

## Status: COMPLETED (2026-02-20)

## Context

Phase 1 implemented the Attractor type model, edge selection, retry policy, and graph validation. The biggest hack remaining was `edge_matches_condition()`, which hard-coded ~15 string patterns instead of parsing conditions dynamically. The `normalize_label()` function was a stub that returned its input unchanged.

Both problems were blocked on String methods. This phase added 6 methods to the String type: `len()`, `to_lowercase()`, `trim()`, `starts_with(prefix)`, `contains(substr)`, and `split(delim)`.

## Results

### Compiler changes

1. **Checker** (`internal/checker/checker.go`) -- Added String method type-checking in `checkMethodCallExpr`, after Array methods and before entity method resolution. 10 tests added covering all methods, chaining, error cases.

2. **Legacy codegen removed** -- The `internal/codegen/` package was deleted entirely. Its three utility functions (`ExprToRust`, `MapType`, `EscapeRustString`) were moved to `internal/testgen/rustutil.go`. The rustbe parity tests were replaced with standalone pattern-based tests. See ADR-0014.

3. **Rust backend** (`internal/rustbe/rustbe.go`) -- Added String method cases in `generateMethodCallExpr`. Tests verify correct Rust output for all 6 methods plus chaining.

4. **JS backend** (`internal/jsbe/jsbe.go`) -- Added String method cases with JS-idiomatic mappings (`toLowerCase`, `startsWith`, `includes`, `BigInt(s.length)`). Tests verify correct JS output.

5. **WASM backend** (`internal/wasmbe/wasmbe.go`) -- Added stubs matching existing pattern (returns zero/default).

### Attractor updates

6. **`normalize_label`** -- Now returns `label.trim().to_lowercase()` (both single-file and multi-file versions).

7. **`edge_matches_condition`** -- Replaced ~15 hard-coded patterns with dynamic `parse_and_evaluate_clause` that uses `split`, `contains`, and `trim` to parse `key=value` and `key!=value` conditions at runtime.

### Documentation

8. **ADR-0013** -- Documents String standard library decision (method form vs free functions, chaining, split return type).

9. **ADR-0014** -- Documents legacy codegen removal.

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

## Design Notes

- `String.len()` (method) coexists with `len(arr)` (builtin function). No conflict.
- Method chaining (`s.trim().to_lowercase()`) works automatically through nested `MethodCallExpr` nodes.
- `split` returns `Array<String>`, the first built-in method returning a generic type.
- The Rust `build` step for the Attractor example has pre-existing borrow checker issues (entity method `&self` vs `&mut self` on borrowed references) unrelated to String methods. The `check` and `emit-rust` steps succeed.
