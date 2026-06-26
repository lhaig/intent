# Pickup Notes — 2026-06-26 (Phase 44 COMPLETE: selfhost/shared restructure)

## Where we are

**Phase 44 (selfhost/shared restructure) — COMPLETE.** Pure refactor that split the
shared stage2 front-end into `selfhost/shared/` and made the tools siblings:

```
selfhost/
  shared/      lexer.intent, ast.intent, parser.intent   (modules shared_lexer/ast/parser)
  formatter/   format.intent, main.intent, format_test.intent + harness scripts
  linter/      lint.intent, lint_main.intent, lint_test.intent, lint-fixtures/
  (checker/    Phase 45)
```

Everything imports the front-end via `../shared/…`. Zero behaviour change — all four
gates identical to before: `make selfcheck-formatter` 4 EQUAL (over
`shared/{lexer,ast,parser}` + `formatter/format`), `make diff-formatter` 22/22,
`make diff-linter` 26/26, full Go suite + stage2 suites (207 / 188) green;
`fmt`/`lint --self-hosted` byte-identical. ADR 0051. This was the ADR 0050 D1
"third-tool" trigger, paid down before the checker arrives.

## Next: Phase 45 — Self-Hosted Checker (first slice). ADR 0052 (to be written).

This was scoped during Phase 44 planning (two Explore recon agents). Key findings:

- **The Go checker is ~4,281 LOC / ~167 diagnostics / a full type system** (Type
  struct, lexical scope stack, symbol table, generics — `internal/checker/`). Far too
  big for one phase; Phase 45 is a deliberate FIRST SLICE.
- **Stage2 AST readiness:** types are flat `String`s (no structured `TypeRef`), `Expr`
  has no type field, there is NO symbol table and NO `Map` (only `Array`, linear
  scans). So **type-inference checks are out of reach** without major AST enrichment,
  but **structural checks are feasible today**, reusing the linter's `Array<String>`
  machinery.
- **Approved first-slice scope (user, 2026-06-25):** scope stack + name-resolution +
  arity, i.e.:
  - Zero-machinery structural checks: duplicate top-level decl, duplicate enum
    variant, break/continue outside loop, return-in-test.
  - An Array-based scope stack / symbol table (globals + function/block/entity scopes)
    → **undeclared-variable** detection.
  - **Arity** checks (function/method/builtin/variant calls) using the registry.
- **Diagnostic format:** same `diagnostic` package → errors render `error[file:line:col]: message`. NOT sorted — emit order = stage1 walk order (verify per-check, as with the linter).
- **Differential gate is two-directional:** the VALID corpus produces ZERO errors, so
  `make diff-checker` needs (a) **invalid fixtures** (one per check) where stage1
  `intentc check` errors and stage2 matches byte-equal, AND (b) **no false positives**
  on the 22 valid examples. Verify the exact `intentc check` stdout/stderr split +
  emit order before locking the harness (`handleCheck`, cmd/intentc/main.go:220-265).

Indicative Phase 45 tasks: 45.1 ADR 0052 → 45.2 scaffold (`selfhost/checker/check.intent`,
`CheckDiag`, dispatch, `error[…]` format + duplicate-decl) → 45.3 structural
no-symbol-table checks → 45.4 Array-based scope stack → 45.5 undeclared-variable →
45.6 arity → 45.7 `check_main.intent` + `intentc check --self-hosted` shim →
45.8 `make diff-checker` (invalid fixtures + no-false-positives) → 45.9 docs + push.
Type inference and the rest of the 167 diagnostics are later phases (need a structured
`TypeRef`/`Type` entity + richer symbol table).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md`.
2. `continue norman` finds nothing queued — scope Phase 45 (checker), write ADR 0052,
   add TASKS.md rows, then proceed. The checker is a `selfhost/checker/` sibling
   importing `../shared/…` (the layout Phase 44 just established).
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter` (and the new `make diff-checker` once it exists).
