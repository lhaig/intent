# 0051: `selfhost/shared/` Restructure

**Date:** 2026-06-25
**Status:** accepted
**Phase:** 44 — selfhost/shared restructure (precedes the self-hosted checker, Phase 45)

## Context

The stage2 self-hosted toolchain has two tools that share one lexer/parser/AST: the
**formatter** ([ADR 0040](0040-self-hosted-formatter-strategy.md), Phases 38-42) and
the **linter** ([ADR 0050](0050-self-hosted-linter-strategy.md), Phase 43). Both live
in `selfhost/formatter/`. ADR 0050 **D1** chose that single directory deliberately —
the proven flat same-directory imports, zero risk — and explicitly deferred splitting
the shared front-end into `selfhost/shared/` **"until a third stage2 tool lands,"**
calling the shared-dir name a known cosmetic smell to be paid down then.

The **checker** (Phase 45, [ADR 0052](0052-self-hosted-checker-strategy.md)) is that
third tool. This ADR records the decision to perform the restructure now, before the
checker is written.

## Decision

Restructure `selfhost/` into a shared front-end plus one directory per tool:

```
selfhost/
  shared/      lexer.intent, ast.intent, parser.intent
  formatter/   format.intent, main.intent, format_test.intent
  linter/      lint.intent, lint_main.intent, lint_test.intent, lint-fixtures/
  (checker/    Phase 45)
```

### D1 — Do it now (the third tool is landing)
ADR 0050 D1 set the trigger: split when a third stage2 tool arrives. Doing the split
before the checker means (a) the checker is authored against the final layout — no
later move of checker files — and (b) the formatter/linter re-point happens while only
two tools depend on the front-end, which is less churn than after a third is added.

### D2 — `shared/` + per-tool sibling dirs
Chosen over keeping everything flat in `formatter/` (the status quo smell) and over a
single flat `selfhost/` (loses tool boundaries). Each tool dir imports `../shared/…`.

### D3 — Cross-directory imports (verified feasible, not a new feature)
Module imports resolve via `filepath.Clean(filepath.Join(entryDir, importPath))`
(`internal/compiler/registry.go:509`), so `import "../shared/parser.intent"` resolves
correctly. No cross-directory import existed in the codebase before only because
nothing needed one — not because it is unsupported. Phase 44 task 44.2 confirms this
empirically before any move; if it somehow fails, a stage1 fix is scoped first.

### D4 — Rename the shared modules to `shared_*`
`formatter_{lexer,ast,parser}` → `shared_{lexer,ast,parser}`, and the linter module
`formatter_linter` → `linter`, with every qualified reference updated. This is the
churniest part of the refactor, but keeping `formatter_*` names inside `shared/` would
re-introduce the exact smell this phase removes. Module/identifier renames preserve the
self-format fixpoint as long as the files remain canonically formatted.

### D5 — Pure refactor, gate-protected
No behaviour change. The four green gates — `make selfcheck-formatter` (now over
`shared/{lexer,ast,parser}` + `formatter/format`), `make diff-formatter` (22/22),
`make diff-linter` (26/26), and `make validate` / full Go suite — must produce
identical results before and after. Moved files stay self-format fixpoints, verified
via the stage2 formatter binary (never stage1 `intentc fmt`, which diverges on stage2
sources).

## Consequences

### Benefits
- The checker (Phase 45) starts as a clean `selfhost/checker/` sibling.
- Tool boundaries are explicit; the shared front-end has an honest name.
- Establishes the `../shared/…` import pattern every future stage2 tool reuses.

### Costs
- Wide mechanical churn: the `shared_*`/`linter` module rename touches the front-end
  files, both `main`s, both test files, the formatter, and the linter.
- Three harness scripts (`selfcheck.sh`, `difftest.sh`, `difftest-lint.sh`), the
  Makefile, and the Go shim path (`stage2LinterBinary`) must be updated in lockstep —
  a half-done move breaks builds until complete.
- Temporary risk to four currently-green gates; mitigated by doing the move as a pure
  refactor with the gates as the acceptance criteria.

### Non-goals
- Any checker code (Phase 45).
- Behaviour changes to the formatter/linter.
- New import features beyond what 44.2 confirms already works.

## References
- [ADR 0050](0050-self-hosted-linter-strategy.md) — D1 set the "third tool" trigger
  this ADR acts on.
- [ADR 0040](0040-self-hosted-formatter-strategy.md) — stage1/stage2 model.
- `internal/compiler/registry.go:509` — import path resolution (`..` supported).
- PRD: `prds/active/prd-phase-44-selfhost-shared-restructure.md`.
