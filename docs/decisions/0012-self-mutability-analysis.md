# 0012: Method Self-Mutability Analysis

**Date:** 2026-02-20
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 1)

## Context

In Rust, methods that modify struct fields require `&mut self`, while read-only methods use `&self`. The Intent codegen was generating `&mut self` for all entity methods unconditionally. This caused Rust borrow checker errors when read-only methods were called on elements accessed through immutable references (e.g., `nodes[i].is_start()` where `nodes` is `&Vec<NodeAttr>`).

The Attractor example has many read-only predicate methods (`is_start`, `is_exit`, `is_conditional`, `is_success`) that should use `&self`.

## Options

- **Always `&mut self`** -- Simple but incorrect. Breaks when methods are called on borrowed data.
- **User annotation** -- Add a `mutating` keyword to the Intent language. Explicit but adds syntax burden.
- **AST-based mutation analysis** -- Walk the method body at codegen time, looking for `self.field = expr` assignments. If none found, emit `&self`.

## Decision

AST-based mutation analysis. The codegen walks each method's body (statements, blocks, if/else branches, while loops, match arms) checking for assignment statements where the target is a field access on `self`. If no such assignments exist, the method gets `&self`; otherwise `&mut self`.

Helper functions: `methodMutatesSelf()`, `blockMutatesSelf()`, `stmtMutatesSelf()`.

## Consequences

- Read-only methods automatically get `&self`, enabling calls on borrowed data without explicit user annotation.
- The analysis is conservative: any `self.field = expr` in any branch marks the method as mutating. This is correct -- a method that conditionally mutates still needs `&mut self`.
- No new syntax needed. The compiler infers the correct receiver type.
- Edge case: methods that call other mutating methods on self are not detected (would require interprocedural analysis). This has not been an issue in practice since Intent methods typically operate on their own fields directly.
