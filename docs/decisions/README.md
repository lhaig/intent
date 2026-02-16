# Decision Log

This directory captures architectural and design decisions using lightweight ADRs (Architecture Decision Records). Every time we make a non-obvious choice, we record the context, options considered, and reasoning.

## Format

Each decision is a numbered markdown file: `NNNN-short-title.md`

Template:
```
# NNNN: Title

**Date:** YYYY-MM-DD
**Status:** accepted | superseded by NNNN | deprecated
**Phase:** which milestone/phase prompted this

## Context
What situation are we in? What problem or question arose?

## Options
What alternatives did we consider?

## Decision
What did we choose and why?

## Consequences
What follows from this decision? Trade-offs accepted.
```

## Index

| # | Decision | Date | Status |
|---|----------|------|--------|
| 0000 | [Why Intent exists](0000-why-intent-exists.md) | 2025-02-12 | accepted |
| 0001 | [Rust as compilation target](0001-rust-as-compilation-target.md) | 2025-02-12 | accepted |
| 0002 | [Go toolchain for compiler](0002-go-toolchain.md) | 2025-02-12 | accepted |
| 0003 | [Runtime assertions over static proofs](0003-runtime-assertions.md) | 2025-02-12 | accepted |
| 0004 | [Separate linter from checker](0004-separate-linter.md) | 2025-02-12 | accepted |
| 0005 | [While loops before for loops](0005-while-before-for.md) | 2025-02-12 | accepted |
| 0006 | [Print as built-in function](0006-print-builtin.md) | 2025-02-12 | accepted |
| 0007 | [Arrays before enums](0007-arrays-before-enums.md) | 2025-02-12 | accepted |
| 0008 | [Intermediate representation](0008-intermediate-representation.md) | 2026-02-16 | accepted |
| 0009 | [Multi-target code generation](0009-multi-target-codegen.md) | 2026-02-16 | accepted |
