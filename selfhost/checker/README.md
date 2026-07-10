# selfhost/checker/

Stage2 Intent semantic checker — Intent-implemented `intentc check`. Reuses the shared
front-end in [`../shared/`](../shared/) and lives alongside stage1's Go checker
(`internal/checker/`). The first compiler subsystem to be self-hosted.

**Status (Phases 45-54 complete, incl. Phase 48/53 type-rule tail):**
structural + name-resolution + arity checks, the `unknown type` diagnostic, expression type
inference ([ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md)), and the
full type-rule check set — byte-equal with stage1 `intentc check` across the examples corpus +
invalid fixtures (`make diff-checker` **100/100**), plus multi-file self-checking
([ADR 0058](../../docs/decisions/0058-self-hosted-checker-cross-module-resolution.md),
`make selfcheck-checker` 13/13). Wired as `intentc check --self-hosted`. ~296 in-language
tests (rust + js). A small tail of sound-false-negative diagnostics is tracked in
[`prds/backlog/prd-phase-58-checker-parity-tail.md`](../../prds/backlog/prd-phase-58-checker-parity-tail.md).

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

Type inference + type-rule checks (Phases 48 & 53): expression inference (`infer_expr_type`,
ADR 0056/0057), operator / assignment / argument typing, match checks, contract
well-typedness, async-context checks, the assert_eq comparable-set, unary operator typing,
entity has-no-constructor, and extern param/return `unknown type`. Multi-file `CheckAll`
shipped in Phase 54 (ADR 0058).

Method-call return-type inference (user-entity methods, with type-param substitution),
contract-clause name recursion (`result`/`old()` handled as contract keywords), impl-block
method contracts, and the immutable-target checks (assign / index-assign / push / set /
remove, via per-binding mutability in the `Scope`) also shipped (Phase 58).

Deferred (sound false negatives — never emit a wrong diagnostic, never fire on valid code):
built-in-method return typing, extern FFI-bridgeability messages, the module-qualified
has-no-constructor variant, and the `@target_specific("wasm")` warning — all catalogued in
[`prds/backlog/prd-phase-58-checker-parity-tail.md`](../../prds/backlog/prd-phase-58-checker-parity-tail.md).

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
