# Pickup Notes — 2026-07-02 (Phase 48 IN PROGRESS: expression inference foundation shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION SHIPPED, IN
PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only when certain,
else an Unknown sentinel); type-rule checks fire only on a confident result, so each is
corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine
(literals + operator result types; idents/calls/etc.→Unknown pending a typed scope),
**48b** `if/while condition must be boolean`, **48c** `let` type-mismatch. `make
diff-checker` → **47/47**, **198** checker tests. This is a large, open-ended phase (full
stage1 type-system parity); the foundation + first two checks are done, the rest (48d-48f)
are the continuation below.

**Phase 47 (builtin-call arity) — COMPLETE** (prior): all 23 builtins arity-checked, three
message shapes, byte-equal. ADR 0055.

**Phase 46 (type foundation + `unknown type`) — COMPLETE** (prior): a structured `Type` +
`parse_type` + `type_is_known` resolver, and `unknown type 'X'` across every corpus
annotation site. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 47/47)
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

## Next: Phase 48d — type-carrying scope (the keystone), then more type-rule checks

The inference engine (48a) + condition-boolean (48b) + let-mismatch (48c) are shipped.
`infer_expr_type` currently returns Unknown for idents/calls/fields/etc. because there is
no type-carrying scope yet — that is the next and highest-value step:

1. **48d — type-carrying scope** (keystone): a `TypeEnv` (name → `Type`) threaded through
   `check_body_stmts`, seeded with function/method params + `self` + let-inferred bindings,
   so `infer_expr_type(e, tenv)` resolves idents. Build it behind the diff-checker gate
   (stays 47/47 until wired). CORRECTNESS-SENSITIVE — inference must stay sound (a wrong
   scope type would false-positive), so mirror `Scope`'s local/outer shadowing carefully.
   Unlocks argument-type mismatch + broader let/condition coverage.
2. **48e — operator-typing** (`operator '+' not defined for X and Y`, `requires boolean
   operands`): needs an **ex_binop-positions front-end change** (ADR 0054 pattern — the
   parser sets `Expr.line/column` at binop nodes; currently 0). Low real-bug value; emit
   only when both operands are confidently known and invalid.
3. **48f** — argument-type mismatch, **method-call arity** (needs the receiver's entity
   type), match-arm consistency, contract well-typedness; plus the **builtin argument
   typing** + `await_*` async-context deferred from Phase 47.

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Smaller tracked gaps (independent): **extern param/return `unknown type`** (0 corpus usage;
stage2 parses `ExternDecl` — TASKS.md 46.4b.5) and the stage2 **trait-method contract**
parser gap.

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056.
2. Continue Phase 48 at **48d** — the type-carrying scope. Add a `TypeEnv` seeded from
   params/`self`/let bindings, thread it through `check_body_stmts` alongside the name
   `Scope`, and give `infer_expr_type` a `tenv` param so idents resolve. Keep it SOUND
   (Unknown when unsure). Build behind the gate (47/47) until a new check is wired, then
   add argument-type mismatch etc. Reuse `Type`/`parse_type`/`type_is_known` (Phase 46) and
   the builtin table (Phase 47).
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
