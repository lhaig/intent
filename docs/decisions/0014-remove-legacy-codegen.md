# 0014: Remove Legacy Codegen Package

**Date:** 2026-02-20
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 2)

## Context

The Intent compiler had two Rust code generation pipelines:

- **Legacy**: `AST -> codegen/codegen.go -> Rust` (the original direct codegen)
- **Primary**: `AST -> checker -> ir/lower.go -> rustbe/rustbe.go -> Rust` (introduced in ADR-0008)

The primary pipeline was introduced with the IR and multi-target architecture (ADR-0008, ADR-0009). Once the Rust backend (`rustbe`) reached parity with the legacy codegen, the legacy package was only used for:

1. **testgen** -- `ExprToRust`, `MapType`, `EscapeRustString` helper functions for generating contract test assertions
2. **rustbe tests** -- `codegen.Generate()` as a comparison baseline for parity verification

## Decision

Remove `internal/codegen/` entirely:

1. **Move utility functions to testgen** -- `ExprToRust`, `MapType`, and `EscapeRustString` are copied into `internal/testgen/rustutil.go` as package-local functions. These are inherently tied to AST contract expressions, which is exactly what testgen works with. (~300 lines, self-contained, no external dependencies beyond `ast` and `lexer`.)

2. **Replace parity tests with standalone tests** -- The `rustbe_test.go` comparison tests served their purpose (verifying migration). They are replaced with pattern-based tests that check the new pipeline's output contains expected Rust constructs (function signatures, assertions, struct definitions) without comparing against legacy output.

3. **Delete `internal/codegen/`** -- The package (codegen.go, codegen_test.go) is removed.

## Consequences

- One fewer package to maintain. The build graph is simpler.
- No behavioral change: the primary pipeline was already used for all `build` and `emit-rust` operations.
- The testgen package is now self-contained for Rust code generation of contract expressions.
- Future Rust-specific code generation improvements only need to touch `rustbe`, not two packages.
