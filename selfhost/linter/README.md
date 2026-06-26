# selfhost/linter/

Stage2 Intent linter — Intent-implemented `intentc lint`. Reuses the shared
front-end in [`../shared/`](../shared/) and adds a read-only AST walk that emits
diagnostics. Lives alongside stage1's Go linter (`internal/linter/`).

**Status (Phase 43, ADR 0050):** all 16 Go-linter rule families ported, byte-equal
with stage1 `intentc lint` across the examples corpus + fixtures
(`make diff-linter` 26/26), wired as `intentc lint --self-hosted`. 188 in-language
linter tests (rust + js).

| File | Module | Purpose |
|------|--------|---------|
| `lint.intent` | `linter` | `LintDiag`, dispatch walk, the 16 rules, `format_diags` |
| `lint_main.intent` | `lint_main` | entry: `args()` → parse → `lint_program` → stdout |
| `lint_test.intent` | `linter_test` | in-language rule tests |
| `lint-fixtures/` | — | non-corpus-rule fixtures for `make diff-linter` |

Imports the front-end via `../shared/…` (Phase 44 / ADR 0051).

```bash
intentc test --all-targets selfhost/linter/lint_test.intent   # linter tests (188)
make diff-linter                                              # vs stage1 intentc lint (26/26)
intentc lint --self-hosted <file.intent>                      # run the stage2 linter
```

The Go shim (`stage2LinterBinary` in `cmd/intentc/main.go`) builds `lint_main.intent`
to a cached binary; `INTENT_STAGE2_LINT` overrides with a prebuilt path. R4 (extern
contracts) is unit-test-only — stage1 and stage2 have incompatible extern syntax, so
no shared differential fixture exists (see ADR 0050).
