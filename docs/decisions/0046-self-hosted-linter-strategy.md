# 0046: Self-Hosted Linter Strategy

**Date:** 2026-06-23
**Status:** accepted
**Phase:** 43 — Self-Hosted Linter (stage2)

## Context

Per `[[project-self-hosting-priority]]`, the confirmed end goal is a **fully
self-hosted Intent compiler** — `intentc` written in Intent. [ADR 0040](0040-self-hosted-formatter-strategy.md)
committed to building stage2 tooling in Intent under the Zig-style stage1/stage2
model and chose the formatter as the smallest-surface first artefact. As of
Phase 42 the stage2 **formatter** is complete: it byte-equal self-formats its own
four source files and is byte-for-byte identical to stage1 `intentc fmt` across the
full `examples/*.intent` corpus (22/22), wired as `intentc fmt --self-hosted` and
gated by `make diff-formatter` + `make selfcheck-formatter`.

ADR 0040 named the linter as the next tool to follow the same pattern (HARNESS.md
§7: "one small subsystem (the linter is the best candidate) rewritten in Intent as
a proof point"). This ADR records the strategy for that rewrite. It is a *faithful
port*, not a redesign — the Go linter (`internal/linter/`) defines the behaviour;
stage2 must reproduce it.

### The target (stage1 Go linter)

`internal/linter/linter.go` exposes `Lint(prog) -> *diagnostic.Diagnostics`:
single-pass, no configuration, no suppression. Every diagnostic is a **warning** in
the exact format `warning[<file>:<line>:<col>]: <message>`, followed by a CLI
summary (`N warning(s) found.` / `No lint warnings.`). It implements **16 rule
families** spanning functions (missing contracts, naming, empty body, unused
params/type-params, discarded spawn), entities (PascalCase, no-invariant, unused
type-param), methods, constructors, enums/variants, traits, variables (unused,
mutable-never-reassigned), intent blocks, externs, and impl methods. Helpers
`is_snake_case` / `is_pascal_case` and a used-name / assigned-name collector
(walking every Stmt and Expr kind) power the usage rules.

### Measured corpus baseline (2026-06-23)

`intentc lint examples/*.intent` fires **76 warnings across 13 files**, exercising
**8 of 16 rule families** (unused variable 25, mutable-never-reassigned 14, method
missing-contracts 14, function missing-contracts 12, function naming 4, entity
no-invariant 4, unused parameter 2, trait method contracts 1). The clean canonical
examples do not trigger the other 8 families, which therefore need dedicated lint
fixtures with golden expected-output to be differentially tested.

## Decision

The linter reuses the stage2 lexer/parser/AST already built for the formatter and
adds a read-only AST walk that emits diagnostics. Three decisions frame the work.

### D1 — Location: reuse `selfhost/formatter/` (do not add a new directory)

| Option | Trade-offs |
|---|---|
| **Reuse `selfhost/formatter/`** [**chosen**] | Linter files (`lint.intent`, `lint_main.intent`, `lint_test.intent`, fixtures) sit beside the existing `formatter_lexer/parser/ast` modules and reach them by proven same-directory relative imports. Zero import risk, no module renames. Cost: a directory named "formatter" also houses the linter (cosmetic). |
| New `selfhost/linter/` importing `../formatter/` | Cleaner separation, but a grep confirms **no cross-directory or subdirectory imports exist anywhere in the codebase** — `import "../formatter/parser.intent"` is unproven and would be the first such use. Needless risk for this phase. |
| Restructure into `selfhost/shared/` + `formatter/` + `linter/` | Cleanest long-term, but re-points every formatter import, the Makefile, and ADR 0040/0042/0044 references, and renames the `formatter_*` modules. Disproportionate churn now. |

The cosmetic smell of D1 is explicitly deferred: when a **third** stage2 tool
arrives, a dedicated restructure phase (with its own ADR) can split the shared
lexer/parser/AST into `selfhost/shared/`. Simplicity over premature abstraction.

### D2 — Parity: byte-equal including the column

| Option | Trade-offs |
|---|---|
| **Byte-equal incl. `:col`** [**chosen**] | The differential compares stage2 output to stage1 `intentc lint` byte-for-byte, matching `warning[file:line:col]: message` exactly — the same rigor the formatter differential uses. Requires threading a `column: Int` onto the stage2 AST nodes the linter anchors on (decls, `let` statements, params), captured from the same token anchor stage1 uses. The two parsers differ, so columns must be made to agree per construct; the differential surfaces each mismatch as a small fix task (the gap-driven loop from ADR 0040 §"gap-driven"). |
| Line + message only | Strips column before comparing. Less work (stage2 decls already carry `line`), but leaves a permanent fidelity gap versus stage1 and a weaker gate. Rejected — parity should mean parity. |

### D3 — Coverage: all 16 rule families this phase, gated by the differential

| Option | Trade-offs |
|---|---|
| **All 16, gated by `make diff-linter`** [**chosen**] | Partial coverage would diverge on the corpus (and fixtures), so the byte-equal gate forces completeness. The 8 corpus-exercised families are covered by the examples differential; the 8 non-corpus families by golden fixtures under `selfhost/.../fixtures/`. |
| MVP subset first, widen later | Defers the hard usage analysis (unused vars/params), but a half-implemented linter has no meaningful gate and risks ossifying (the D-language precedent in ADR 0040). Rejected. |

### Build / gate flow

Mirrors the formatter. `lint_main.intent` is built by stage1 to a binary;
`intentc lint --self-hosted <file>` is a thin Go shim delegating to it (deferred
behind the flag exactly as `fmt --self-hosted`). `make diff-linter` builds the
binary and runs it over the corpus + fixtures, diffing against stage1 `intentc
lint`. As with the formatter, the **built binary** (8 MB main stack) is used rather
than an in-language `intentc test` probe (2 MB libtest stack overflows on deep
parse — see progress.md).

## Consequences

### Benefits

- **Second self-hosted artefact.** Proves the stage2 lexer/parser/AST generalise
  beyond the formatter — the same AST now drives two distinct tools.
- **Compiler-rewrite groundwork.** The linter's full Stmt/Expr usage walk
  (`collect_used_names`) is the kind of whole-AST analysis the eventual self-hosted
  checker needs; building it here de-risks that.
- **Column fidelity.** D2 adds real source-column tracking to the stage2 AST, useful
  for any future stage2 diagnostic tooling (checker, LSP).

### Costs

- **Double maintenance.** Stage1 Go linter and stage2 Intent linter coexist; a lint
  rule change must land in both until stage1 retires. Same trade-off ADR 0040 made.
- **Column agreement work.** D2 means per-construct column tuning where the two
  parsers' token positions differ; surfaced empirically by the differential.
- **Fixture maintenance.** Golden expected-output files for the 8 non-corpus rules
  must be regenerated if a rule's message ever changes.

### Non-goals

- New lint rules beyond the 16 stage1 has — faithful port only.
- Lint configuration / suppression / per-rule enable-disable (stage1 has none).
- Retiring the stage1 Go linter (deferred, as with the formatter).
- Restructuring `selfhost/` into `shared/` (deferred to a future tool + ADR).

## References

- [ADR 0040](0040-self-hosted-formatter-strategy.md) — self-hosted formatter
  strategy; establishes the stage1/stage2 model, precedents, and gap-driven delivery
  this ADR inherits.
- [ADR 0042](0042-stage2-source-order-tracking.md) — stage2 `line` tracking that D2
  extends with `column`.
- [ADR 0045](0045-args-builtin.md) — `args()` builtin used by `lint_main.intent`.
- `internal/linter/linter.go` — the behaviour being ported (the spec).
- PRD: `prds/active/prd-phase-43-self-hosted-linter.md`.
- HARNESS.md §7 — names the linter as the best self-hosting proof point.
