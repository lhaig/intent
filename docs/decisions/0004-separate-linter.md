# 0004: Separate Linter Package

**Date:** 2025-02-12
**Status:** accepted
**Phase:** POC

## Context

Style and best-practice warnings were added to the compiler pipeline.
These need a home in the codebase.

## Options

- **Add to checker** -- Single pass for errors and warnings together.
- **Separate linter package** -- Independent package for warnings only.

## Decision

Separate linter package. The checker reports errors (type mismatches,
undeclared variables, arity problems). The linter reports warnings
(unused variables, naming conventions, missing contracts). Different
concerns, different severity levels.

## Consequences

- Checker stays focused on correctness. No warning logic clutters the
  type-checking code.
- Linter can run independently. Most lint rules do not require type
  information, so the linter can operate on the AST alone.
- Users can ignore lint warnings without affecting compilation. A program
  with lint warnings still compiles and runs.
- Two-pass architecture: check first (fail on errors), then lint (report
  warnings). Clear separation of blocking vs. advisory diagnostics.
- Lint rules are easy to add without risking checker regressions.
