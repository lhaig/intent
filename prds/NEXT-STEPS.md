# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: operator typing shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + EIGHT CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine,
**48b** `if/while condition must be boolean`, **48c** `let` type-mismatch, **48d/48d+** the
type-carrying scope (params + let-bound vars), **48f** function arg-type, **48g** variant
arg-type, **48h** assignment mismatch, **48i.1** `self` + field-access inference, **48i.2**
method-call arity + arg-types, and **48e** binary operator typing. `make diff-checker` →
**59/59**, **227** checker tests. Full stage1 type-system parity is a large, open-ended
goal; the rest (48j onward) is the continuation below.

**Phase 47 (builtin-call arity) — COMPLETE**. ADR 0055.
**Phase 46 (type foundation + `unknown type`) — COMPLETE**. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 59/59)
```

## What 48e shipped (this session)

- **binop-pos front-end** (b77bdf3, ADR 0054): all 8 ex_binop construction sites route
  through a new `Parser.make_binop` helper that anchors the node at the operator token
  (matches stage1 `BinaryExpr.Pos()`). Additive/inert (parser.intent stays a formatter
  fixpoint; all gates unchanged). Same pattern as `lit-pos`/`fld-pos`.
- **48e operator typing** (e2a986f): `binop_result_type` mirrors stage1 `checkBinaryExpr`
  (result Type per operator, or Unknown when undefined for the operands). `infer_expr_type`'s
  ex_binop case delegates to it, so comparison/logical ops now yield Bool ONLY for valid
  operands (previously eager-Bool — latently unsound). `check_expr_names` emits `operator
  'OP' not defined for X and Y` / `operator 'OP' requires boolean operands, got X and Y` at
  the operator when both operands are confident and `binop_result_type` is Unknown; an
  Unknown operand skips. `operator_display` reproduces stage1's quirk of printing the
  operator TOKEN NAME (MINUS/STAR/EQ/LT/AND/…) — except `+`, a literal. `=` excluded.

## Next: Phase 48j — match/contract typing, then the phase-53 gaps

Remaining, in rough value order:

- **48j — match-arm consistency / exhaustiveness + contract well-typedness**: the two
  remaining stage1 type-rule areas. Read stage1's match checking (arm-type consistency,
  exhaustiveness) and contract checking (requires/ensures/invariant well-typedness) in
  internal/checker/. Both need match/contract inference; emit only on confident inference,
  behind the diff-checker gate. Larger — likely multiple slices.
- **phase-53 gaps** (independent, smaller): **extern param/return `unknown type`** (0 corpus
  usage; stage2 parses `ExternDecl`), **generic-entity-instantiation arity**, and the stage2
  **trait-method contract** parser gap.
- **method-call RETURN-type inference**: `infer_expr_type` on a method call still returns
  Unknown; typing it needs generic type-param substitution through the receiver's type args
  (stage1 does this). Would unlock `let x: T = obj.method()` mismatch etc.
- **unary operator typing** (`unary '-' not defined for X`, `unary 'not' requires boolean
  operand, got X`): the companion to 48e for `checkUnaryExpr`. Needs ex_unary positions
  (front-end) + tightening unary inference (currently eager Bool / operand-type). Low value,
  corpus-invisible.

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Known deferred (need new machinery): builtin argument typing + `await_*` async-context
(Phase 47); immutable-target assignment/push (needs mutability tracking in Scope).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056.
2. Continue Phase 48 at **48j** — match-arm consistency/exhaustiveness + contract typing
   (read the stage1 sites first; scope into slices). Or, for a smaller win first, pick a
   phase-53 gap (extern unknown-type / generic-entity arity). Keep inference SOUND (Unknown
   skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
