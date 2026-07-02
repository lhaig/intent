# selfhost/checker/

Stage2 Intent semantic checker — Intent-implemented `intentc check`. Reuses the shared
front-end in [`../shared/`](../shared/) and lives alongside stage1's Go checker
(`internal/checker/`). The first compiler subsystem to be self-hosted.

**Status (Phase 46, [ADR 0053](../../docs/decisions/0053-self-hosted-checker-type-foundation.md)):**
type-representation foundation + the `unknown type` diagnostic — byte-equal with stage1
`intentc check` across the examples corpus + fixtures (`make diff-checker` 44/44: 22 valid
examples produce no errors, 22 invalid fixtures match byte-for-byte). Wired as `intentc
check --self-hosted`. 188 in-language tests (rust + js). (Phase 45,
[ADR 0052](../../docs/decisions/0052-self-hosted-checker-strategy.md), shipped the first
slice — the checks below needing no type inference.)

## Implemented checks

Structural / name-resolution / arity (Phase 45):

- Duplicate top-level declaration (`entity/enum/function/trait 'X' already defined`)
- Duplicate enum variant (`duplicate variant name 'X' in enum 'Y'`)
- `break`/`continue` statement outside loop
- `return` inside a test body
- Undeclared variable (`undeclared variable 'X'`) + variable redefinition in a scope
- Call arity: function (`function 'X' expects N arguments, got M`), variant, and
  builtin (Phase 47, ADR 0055 — 23 builtins, e.g. `print() expects 1 argument, got 2`)

Type foundation + `unknown type` (Phase 46, ADR 0053 + 0054):

- `Type` tree + `parse_type(s)` (parses the flat type strings the AST carries) +
  `type_is_known` resolver (ports stage1 `ResolveTypeWithParams`).
- `unknown type 'X'` over every annotation site the corpus uses — function param/return,
  entity field, entity method param/return, `let` statement, and enum-variant field —
  each byte-equal with stage1 (outer-ref base name; `registerEnums`-before-entities
  quirk matched).

Deferred: expression type inference and all type-rule checks (assignability, operators,
generics substitution, match exhaustiveness, contracts) — Phase 47. Extern param/return
`unknown type` (0 corpus usage) and method-call arity (needs the receiver type) remain
small tracked gaps, along with builtin argument *typing* (`print() cannot print type …`,
`assert() argument must be Bool`, the `await_*` async-context checks — Phase 48). Multi-file
`CheckAll` is also later.

| File | Module | Purpose |
|------|--------|---------|
| `check.intent` | `checker` | `CheckDiag`, register+check dispatch, `Scope`, the checks |
| `check_main.intent` | `check_main` | entry: parse → `check_program` → stdout; exit 0 clean / 1 on error |
| `check_test.intent` | `checker_test` | in-language tests |
| `check-fixtures/` | — | one invalid fixture per check for `make diff-checker` |

```bash
intentc test --all-targets selfhost/checker/check_test.intent   # checker tests
make diff-checker                                               # vs stage1 intentc check (44/44)
intentc check --self-hosted <file.intent>                       # run the stage2 checker
```

Design notes: the symbol table is a flattened `Scope` (local + outer name `Array`s, no
recursive field, no `Map`) since stage2 lacks those; the global scope seeds all decl
names + enum variant names + free builtins. Types are a `Type` tree built by `parse_type`
from the strings the AST already carries (ADR 0053 D1 — no structured types in the AST).
Front-end prerequisites added gap-driven: `break`/`continue` statements + `Expr`
positions (Phase 45), and `FieldDecl` positions ([ADR 0054](../../docs/decisions/0054-additive-ast-positions-for-diagnostics.md),
Phase 46 — additive positions are inert to the formatter). The differential's
no-false-positives direction (valid corpus → zero errors) is what keeps the resolver
honest.
