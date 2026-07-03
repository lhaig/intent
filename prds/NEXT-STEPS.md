# Pickup Notes — 2026-07-02 (Phase 48 IN PROGRESS: expression inference foundation shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION SHIPPED, IN
PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only when certain,
else an Unknown sentinel); type-rule checks fire only on a confident result, so each is
corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine
(literals + operator result types), **48b** `if/while condition must be boolean`, **48c**
`let` type-mismatch, **48d** the type-carrying scope (params typed), **48d+** let-var binding, and
**48f** function arg-type, **48g** variant arg-type, **48h** assignment mismatch. `make diff-checker` → **51/51**, **211** checker tests. This is a large, open-ended phase (full
stage1 type-system parity); the foundation + these checks are shipped, the rest (48g+)
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
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 51/51)
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

## Next: Phase 48i — `self`/field-access inference, then the rest

Shipped: inference engine (literals + operators + idents via the typed scope for params &
let-bindings), condition-must-be-boolean, let/assignment type-mismatch, and function &
variant argument-type mismatch. `infer_expr_type` still returns Unknown for `self`, field
access, and call results (sound skip). Next steps, in rough value order:

- **48i (recommended next) — `self` + field-access inference**: type `self` as the
  enclosing entity and infer `self.field` / `x.field` → the field's type. This is the next
  big unlock — it enables method-call arity + arg-types (receiver type). (Params + let
  bindings already typed; function/variant arg-types + assignment mismatch shipped.)
- **48e — operator-typing** (`operator '+' not defined for X and Y`, `requires boolean
   operands`): needs an **ex_binop-positions front-end change** (ADR 0054 pattern — the
   parser sets `Expr.line/column` at binop nodes; currently 0). Low real-bug value; emit
   only when both operands are confidently known and invalid.
- **later — match-arm consistency, contract well-typedness**, and the **builtin argument
   typing** + `await_*` async-context deferred from Phase 47 (hang off the builtin table).

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Smaller tracked gaps (independent): **extern param/return `unknown type`** (0 corpus usage;
stage2 parses `ExternDecl` — TASKS.md 46.4b.5) and the stage2 **trait-method contract**
parser gap.

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056.
2. Continue Phase 48 at **48i** — `self`/field-access inference (unlocks method-call
   arity/arg-types). Then 48e operator-typing (ex_binop positions), 48j match/contract
   typing, phase-53 gaps. Keep inference SOUND (Unknown skips); one check per slice, gate
   after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
