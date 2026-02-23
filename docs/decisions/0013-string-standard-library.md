# 0013: String Standard Library (Phase 2)

**Date:** 2026-02-20
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 2)

## Context

The Attractor example requires string manipulation for dynamic condition parsing. Phase 1 used hard-coded pattern matching for edge conditions (e.g., `if edge.condition == "outcome=success"`), which was fragile and couldn't handle arbitrary condition strings. The `normalize_label` function was a stub that returned the label unchanged.

String methods are a natural addition since Intent already has String as a built-in type with equality comparison and concatenation.

## Decision

Add six methods to the String type as built-in methods (not entity methods):

| Method | Signature | Returns | Rust | JS |
|--------|-----------|---------|------|----|
| `len` | `s.len()` | `Int` | `(s.len() as i64)` | `BigInt(s.length)` |
| `to_lowercase` | `s.to_lowercase()` | `String` | `s.to_lowercase()` | `s.toLowerCase()` |
| `trim` | `s.trim()` | `String` | `s.trim().to_string()` | `s.trim()` |
| `starts_with` | `s.starts_with(prefix)` | `Bool` | `s.starts_with(arg.as_str())` | `s.startsWith(arg)` |
| `contains` | `s.contains(substr)` | `Bool` | `s.contains(arg.as_str())` | `s.includes(arg)` |
| `split` | `s.split(delim)` | `Array<String>` | `s.split(arg.as_str()).map(\|s\| s.to_string()).collect::<Vec<String>>()` | `s.split(arg)` |

Methods are type-checked in the checker (alongside Array and Result/Option methods), lowered through the IR unchanged, and mapped to idiomatic target language calls in each backend.

## Alternatives Considered

- **Free functions** (`trim(s)`, `split(s, delim)`) -- Inconsistent with `s.push()` for arrays. Method syntax is more natural for string operations.
- **Full standard library module** -- Over-engineered for current needs. These six methods cover the Attractor use case. More can be added incrementally.
- **Only add methods used by Attractor** -- We included `len` and `to_lowercase` beyond immediate need since they're trivial to implement and commonly expected.

## Consequences

- String methods chain naturally: `s.trim().to_lowercase()`.
- The `split` method returns `Array<String>`, which integrates with existing array operations.
- The Attractor example's `edge_matches_condition` can now dynamically parse condition strings instead of using hard-coded patterns.
- WASM backend stubs String methods (returns zero/default), consistent with existing method stub pattern.
- `String.len()` (method) and `len(arr)` (builtin function) coexist. The method form is only available on String; arrays use the builtin.
