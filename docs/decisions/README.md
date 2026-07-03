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
| 0010 | [Attractor as driving example](0010-attractor-as-driving-example.md) | 2026-02-20 | accepted |
| 0011 | [Conservative String cloning for Rust ownership](0011-string-clone-strategy.md) | 2026-02-20 | accepted |
| 0012 | [Method self-mutability analysis](0012-self-mutability-analysis.md) | 2026-02-20 | accepted |
| 0013 | [String standard library](0013-string-standard-library.md) | 2026-02-20 | accepted |
| 0014 | [Remove legacy codegen package](0014-remove-legacy-codegen.md) | 2026-02-20 | accepted |
| 0015 | [Array\<String\> on entity fields](0015-array-string-entity-fields.md) | 2026-02-23 | accepted |
| 0016 | [Map\<K,V\> type](0016-map-type.md) | 2026-02-23 | accepted |
| 0017 | [Error handling patterns in Attractor examples](0017-error-handling-attractor.md) | 2026-02-24 | accepted |
| 0018 | [Trait system (static dispatch)](0018-trait-system.md) | 2026-02-24 | accepted |
| 0019 | [I/O standard library](0019-io-standard-library.md) | 2026-02-27 | accepted |
| 0020 | [HTTP, JSON, and event builtins](0020-http-json-builtins.md) | 2026-02-27 | accepted |
| 0021 | [Phase 9 completion (lint rules, HandlerRegistry, Map key rejection, json_path)](0021-phase9-completion.md) | 2026-03-20 | accepted |
| 0022 | [Rust codegen mutability analysis](0022-rust-mutability-analysis.md) | 2026-03-20 | accepted |
| 0023 | [Closures and first-class functions](0023-closures-first-class-functions.md) | 2026-03-20 | accepted |
| 0024 | [JavaScript multi-file codegen fix](0024-js-multifile-codegen-fix.md) | 2026-03-20 | accepted |
| 0025 | [User-defined generics design](0025-user-defined-generics-design.md) | 2026-03-20 | accepted |
| 0026 | [Concurrency and async design](0026-concurrency-async-design.md) | 2026-03-20 | accepted (revised in Phase 14) |
| 0027 | [Package management design](0027-package-management-design.md) | 2026-03-20 | accepted; revised by ADR 0039 (MVS replaces constraint solver) |
| 0028 | [Rust FFI / crate imports](0028-rust-ffi-crate-imports.md) | 2026-05-28 | accepted |
| 0029 | [In-language testing framework](0029-in-language-testing.md) | 2026-05-29 | accepted |
| 0030 | [Cross-package test visibility](0030-cross-package-test-visibility.md) | 2026-05-30 | accepted |
| 0031 | [`@target_specific` annotation for tests](0031-target-specific-annotation.md) | 2026-05-30 | accepted; implemented in 4dacd6c |
| 0032 | [LSP v1 surface](0032-lsp-v1-surface.md) | 2026-05-30 | accepted; revised five times (Phase 19, Phase 20, Phase 21, Phase 25, Phase 26); see ADR for commit ranges |
| 0033 | [`--strip-contracts` flag and contract strip policy](0033-release-flag-strip-policy.md) | 2026-05-31 | accepted; revised same day (dropped redundant `--release` flag); PRD pending |
| 0034 | [Per-contract source positions in verify results](0034-per-contract-source-positions.md) | 2026-05-31 | accepted (PRD pending) |
| 0035 | [LSP textDocument/references scope and semantics](0035-lsp-find-references.md) | 2026-05-31 | accepted (PRD pending) |
| 0036 | [Entity and method auto-test emission for `--target intent`](0036-testgen-entity-method-emission.md) | 2026-05-31 | accepted (PRD pending) |
| 0037 | [Multi-param iteration in `--target intent` test generation](0037-testgen-multi-param-iteration.md) | 2026-06-01 | accepted (PRD pending) |
| 0038 | [Retire the legacy Rust `testgen` path](0038-retire-legacy-rust-testgen.md) | 2026-06-02 | accepted |
| 0039 | [Package registry — git-based + MVS](0039-package-registry-git-mvs.md) | 2026-06-03 | accepted (PRD pending) |
| 0040 | [Self-hosted formatter strategy](0040-self-hosted-formatter-strategy.md) | 2026-06-03 | accepted (strategic; per-phase ADRs follow) |
| 0041 | [String indexing and `Char` type](0041-string-indexing-and-char-type.md) | 2026-06-03 | accepted (Phase 31 shipped 2026-06-03) |
| 0042 | [Stage2 source-order tracking](0042-stage2-source-order-tracking.md) | 2026-06-09 | accepted |
| 0043 | [Stage2 formatter paren stripping](0043-stage2-paren-stripping.md) | 2026-06-09 | accepted |
| 0044 | [Stage2 comment preservation](0044-stage2-comment-preservation.md) | 2026-06-09 | accepted |
| 0045 | [`args()` builtin](0045-args-builtin.md) | 2026-06-15 | accepted |
| 0046 | [Counterexample-driven self-repair](0046-counterexample-driven-repair.md) | 2026-06-15 | proposed (research) |
| 0047 | [Contract integrity (vacuity, intent-agreement, cross-target)](0047-contract-integrity.md) | 2026-06-15 | proposed (research) |
| 0048 | [Verification trust manifest](0048-verification-trust-manifest.md) | 2026-06-15 | proposed (research) |
| 0049 | [Agent interface (MCP / structured tool API)](0049-agent-interface.md) | 2026-06-15 | proposed (research) |
| 0050 | [Self-hosted linter strategy](0050-self-hosted-linter-strategy.md) | 2026-06-24 | accepted (Phase 43 shipped 2026-06-24) |
| 0051 | [`selfhost/shared/` restructure](0051-selfhost-shared-restructure.md) | 2026-06-25 | accepted (Phase 44) |
| 0052 | [Self-hosted checker strategy](0052-self-hosted-checker-strategy.md) | 2026-06-26 | accepted (Phase 45) |
| 0053 | [Self-hosted checker — type representation foundation](0053-self-hosted-checker-type-foundation.md) | 2026-06-26 | accepted (Phase 46) |
| 0054 | [Additive AST position fields for checker diagnostics](0054-additive-ast-positions-for-diagnostics.md) | 2026-07-02 | accepted (Phase 46.4b) |
| 0055 | [Self-hosted checker — builtin-call arity](0055-self-hosted-builtin-arity.md) | 2026-07-02 | accepted (Phase 47) |
| 0056 | [Self-hosted checker — expression type inference (sound-but-incomplete)](0056-self-hosted-expression-inference.md) | 2026-07-02 | accepted (Phase 48) |
| 0057 | [Self-hosted checker — async context via the Scope](0057-self-hosted-checker-async-context.md) | 2026-07-03 | accepted (Phase 48j-c2d) |
