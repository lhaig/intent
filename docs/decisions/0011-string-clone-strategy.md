# 0011: Conservative String Cloning for Rust Ownership

**Date:** 2026-02-20
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 1)

## Context

Intent compiles to Rust, which has strict ownership rules. When a String field is accessed from a borrowed context (e.g., an entity in a Vec), Rust's borrow checker rejects moves. Similarly, passing a String variable to multiple function calls moves it on the first call.

This manifested as two bugs in the Attractor example:
- `edges[i].from_node` moved the String out of a Vec element
- `preferred_label` passed to two functions moved on the first call

## Options

- **Borrow analysis** -- Track lifetimes and emit `&str` references where possible, `.clone()` only when ownership transfer is needed. Correct but complex; requires building a borrow checker in the Intent compiler.
- **Conservative cloning** -- Emit `.clone()` on all String field accesses and String function arguments. Simple, always correct, minor performance cost.
- **Pass strings by reference** -- Generate function signatures with `&str` instead of `String`. Requires changes to the type mapping and complicates the codegen model.

## Decision

Conservative cloning. The codegen emits `.clone()` for:
1. All `FieldAccessExpr` where the field type is `String`
2. All function call arguments where the parameter type is `String` (both regular and module-qualified calls)

## Consequences

- Generated Rust always compiles without ownership errors for String types.
- Minor performance overhead from unnecessary clones. This is negligible for the programs Intent targets (pipeline orchestration, not hot loops).
- The approach is simple to implement: two targeted checks in codegen, no borrow analysis needed.
- If performance becomes a concern, this decision can be revisited with a more sophisticated analysis. The conservative strategy is a correct starting point.
- The same pattern (entity-typed parameters passed by reference) was applied to entity types to avoid similar ownership issues.
