# Pickup Notes — 2026-07-02 (Phase 47 COMPLETE: builtin-call arity)

## Where we are

**Phase 47 (Self-Hosted Checker — builtin-call arity) — COMPLETE.** Closed the
builtin-call arity gap deferred from Phase 45: all 23 stage1 builtins are arity-checked
(arg count only) with byte-equal messages in three shapes. `make diff-checker` → **44/44**
(22 examples clean + 22 fixtures). ADR 0055. **188** checker tests.

**Phase 46 (type foundation + `unknown type`) — COMPLETE** (prior): a structured `Type` +
`parse_type` + `type_is_known` resolver, and `unknown type 'X'` across every corpus
annotation site. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-47, diff-checker 44/44)
```

### What Phase 46 shipped
- **Type foundation** (ADR 0053): a `Type` tree (`name` + `type_args` + `Fn(...)->R`) +
  `parse_type(s)` parsing the flat type strings the AST carries + a `type_is_known`
  resolver porting stage1 `ResolveTypeWithParams`. **No structured types in the AST** (D1)
  — the checker re-parses the strings it already receives.
- **`unknown type 'X'`** across every annotation site the corpus uses: function
  param/return, entity field, entity method param/return, `let` statement, enum-variant
  field. Byte-equal incl. outer-ref base name (`Array<Widget>` → `'Array'`) and the
  `registerEnums`-before-`registerEntities` quirk (entity-typed variant field still errors).
- **ADR 0054** (front-end): `FieldDecl` gained `line`/`column` (from the `field` keyword).
  Additive positions are permitted — inert to the formatter, distinct from D1's ban on
  structured types. Precedent: Phase 45.7 `Expr` positions.
- **Two latent bugs fixed** en route: rustbe now borrows an owned-temporary (e.g. a call
  result) passed to an `Array`/`Map` (`&Vec`) param (was E0308); the `intentc test` runner
  now surfaces swallowed Rust compile errors instead of a bare "did not run".

## Next: Phase 48 — Expression type inference (the big build on the foundation)

The remaining checker diagnostics are inference-heavy and need the scope to carry **types**
(not just names) and inference over every `Expr` kind. Natural next steps:

1. **Expression type inference** — infer a `Type` for each `Expr` (literals, idents via a
   type-carrying scope, binops, calls, field/index, match, etc.), storing results (the Go
   checker keeps `exprTypes`). Build it behind the diff-checker gate (keep 44/44 until a
   new diagnostic is wired in). This gates every type-rule check.
2. **Type-rule checks** — assignability (`type mismatch`), operator typing, condition-
   must-be-boolean, argument-type mismatch (incl. the deferred **builtin argument typing**
   + `await_*` async-context from Phase 47), return-type, generic instantiation, match
   exhaustiveness + arm-type consistency, contract well-typedness. Each a `make diff-checker`
   fixture + corpus no-false-positive coverage.
3. **method-call arity** — needs the receiver type (a first, self-contained use of the
   inference engine: infer the object's entity type, look up the method's arity).

Smaller tracked gaps (independent): **extern param/return `unknown type`** (0 corpus usage;
stage2 parses `ExternDecl` — TASKS.md 46.4b.5) and the stage2 **trait-method contract**
parser gap.

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md`.
2. Scope Phase 48 (start with a type-carrying scope + expression inference — it gates all
   type-rule checks), write its ADR, add TASKS.md rows, then proceed. The checker lives in
   `selfhost/checker/`; reuse `Type` / `parse_type` / `type_is_known` from Phase 46 and the
   builtin table from Phase 47 (hang argument typing off it).
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker`.
