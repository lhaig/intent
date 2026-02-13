# 0002: Go Toolchain for Compiler Implementation

**Date:** 2025-02-12
**Status:** accepted
**Phase:** POC

## Context

The compiler itself needs an implementation language. This is the language
we write the lexer, parser, checker, and codegen in.

## Options

- **Rust** -- Same as target; could share runtime code. Slower iteration.
- **Go** -- Fast compilation, simple toolchain, easy single-binary distribution.
- **Python** -- Rapid prototyping, but slow and hard to distribute.
- **TypeScript** -- Good string handling, but Node.js dependency.

## Decision

Go. Fast compilation speeds up the edit-test cycle. Simple toolchain with
no complex build system needed. Good string handling for code generation.
Easy to distribute as a single static binary. The compiler is a transpiler,
not an optimizer -- it does not need Rust-level performance.

## Consequences

- Compiler and target are different languages. Cannot reuse runtime code
  between the compiler and generated output.
- Go's simplicity keeps the compiler codebase accessible to contributors.
- Single binary distribution simplifies installation.
- No generics-heavy patterns needed; Go's type system is sufficient for
  AST manipulation and tree-walking.
