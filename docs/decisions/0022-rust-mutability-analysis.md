# 0022: Rust Codegen Mutability Analysis

**Date:** 2026-03-20
**Status:** accepted
**Phase:** post-v1.0 (Attractor follow-up)

## Context

The Rust backend (`internal/rustbe/`) used a blanket rule to mark all entity-typed variables as `let mut` in generated Rust code. This caused Rust to emit `unused_mut` warnings for every entity-typed variable that was never actually reassigned or had fields mutated. For the Attractor example, this produced 29+ unnecessary `mut` warnings, and 3 `private_interfaces` warnings from `pub trait` declarations in single-file output.

## Decision

### 1. Mutability Analysis (`collectMutatedVars`)

Replace the blanket `isEntityType` rule with a static analysis pass that scans the function body to determine which variables are actually mutated. A variable is marked as needing `mut` if any of the following appear in the function body:

- Direct assignment: `x = ...`
- Field assignment: `x.field = ...` (walks nested field accesses to find root variable)
- Index assignment: `x[i] = ...`
- Method call receiver: `x.method(...)` (since methods may take `&mut self`)

The analysis runs once per function/constructor/method before code generation and stores the result on the generator. The `LetStmt` emission then uses `stmt.Mutable || g.mutatedVars[stmt.Name]` instead of `stmt.Mutable || g.isEntityType(stmt.Type)`.

### 2. Rust Allow Attributes

Added `unused_mut` and `private_interfaces` to the `#![allow(...)]` attribute in generated Rust code. This suppresses warnings from:

- Source-declared `let mutable` on variables that are not actually mutated (a source-level issue, not codegen)
- `pub trait` declarations in single-file output where struct types are private

## Consequences

- Attractor multi-file example compiles with 1 warning (unused assignment), down from 42
- Generated Rust is more idiomatic -- variables are only `let mut` when actually needed
- The `isEntityType` function is retained for other uses but no longer drives mutability decisions
- Source `.intent` files that over-use `let mutable` still compile cleanly due to the `unused_mut` allow
- All existing tests continue to pass
