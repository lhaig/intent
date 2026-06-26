# selfhost/checker/

Stage2 Intent semantic checker — Intent-implemented `intentc check`. Reuses the shared
front-end in [`../shared/`](../shared/) and lives alongside stage1's Go checker
(`internal/checker/`). The first compiler subsystem to be self-hosted.

**Status (Phase 45, [ADR 0052](../../docs/decisions/0052-self-hosted-checker-strategy.md)):**
first slice — byte-equal with stage1 `intentc check` across the examples corpus +
fixtures (`make diff-checker` 34/34: 22 valid examples produce no errors, 12 invalid
fixtures match byte-for-byte). Wired as `intentc check --self-hosted`. ~150 in-language
tests (rust + js).

## Implemented checks (no type inference yet — see ADR 0052 D1/D2)

- Duplicate top-level declaration (`entity/enum/function/trait 'X' already defined`)
- Duplicate enum variant (`duplicate variant name 'X' in enum 'Y'`)
- `break`/`continue` statement outside loop
- `return` inside a test body
- Undeclared variable (`undeclared variable 'X'`) + variable redefinition in a scope
- Call arity: function (`function 'X' expects N arguments, got M`) and variant

Deferred to later phases: all type inference (assignability, operators, generics,
traits, match exhaustiveness, contracts), method-call arity (needs receiver type), and
builtin-call arity (~20 bespoke messages). Multi-file `CheckAll` is also later.

| File | Module | Purpose |
|------|--------|---------|
| `check.intent` | `checker` | `CheckDiag`, register+check dispatch, `Scope`, the checks |
| `check_main.intent` | `check_main` | entry: parse → `check_program` → stdout; exit 0 clean / 1 on error |
| `check_test.intent` | `checker_test` | in-language tests |
| `check-fixtures/` | — | one invalid fixture per check for `make diff-checker` |

```bash
intentc test --all-targets selfhost/checker/check_test.intent   # checker tests
make diff-checker                                               # vs stage1 intentc check (34/34)
intentc check --self-hosted <file.intent>                       # run the stage2 checker
```

Design notes: the symbol table is a flattened `Scope` (local + outer name `Array`s, no
recursive field, no `Map`) since stage2 lacks those; the global scope seeds all decl
names + enum variant names + free builtins. Two front-end prerequisites were added in
Phase 45: real `break`/`continue` statements and source positions on `Expr` (so
`undeclared variable` can anchor at the identifier). The differential's no-false-
positives direction (valid corpus → zero errors) is what keeps name resolution honest.
