# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: match checking complete)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + 16 CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine,
**48b** `if/while condition must be boolean`, **48c** `let` type-mismatch, **48d/48d+** the
type-carrying scope, **48f** function arg-type, **48g** variant arg-type, **48h** assignment
mismatch, **48i.1** `self` + field-access inference, **48i.2** method-call arity + arg-types,
**48e** binary operator typing, **48j-b** contract well-typedness, **48j-a** the five
match-arm structural checks, and **48j-a2** match arm-type consistency + scrutinee-must-be-
enum. **Match checking is now COMPLETE** — all seven stage1 checkMatchExpr diagnostics are
byte-equal. `make diff-checker` → **70/70**, **246** checker tests. Full stage1 type-system
parity is a large, open-ended goal; the rest (48j-c / phase-53) is below.

**Phase 47 (builtin-call arity) — COMPLETE**. ADR 0055.
**Phase 46 (type foundation + `unknown type`) — COMPLETE**. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 68/68)
```

## What 48j-a2 shipped (this session)

- **scrutinee-must-be-enum** (e65292f): a match scrutinee confidently inferred to a PRIMITIVE
  (is_primitive_type) is flagged `match scrutinee must be an enum type, got X` at the match
  keyword (emit + return, like stage1); Option/Result/entity/collection scrutinees fall to
  the fallback (sound skip).
- **arm-type consistency** (d461273): each reached arm's body is inferred in `arm_typed_scope`
  (bindings TYPED from variant fields, only where index < field count, like stage1). Arm 0 is
  the baseline (stage1 i==0; skipped/untypeable arm 0 → no comparisons); a later arm of a
  confidently DIFFERENT type (no type_args, so String()==name) → `match arm type mismatch:
  expected X, got Y` at that arm.

**Match checking is COMPLETE**: all seven stage1 checkMatchExpr diagnostics byte-equal.
Earlier this session (also pushed): **48i.2** method-call arity/arg-types, **48e** binary
operator typing, **48j-b** contract well-typedness, **48j-a** the five match structural
checks (+match-pos front-end 88807f7).

## Next: Phase 48j-c / phase-53 gaps

Remaining, in rough value order:

- **48j-c — builtin argument typing + `await_*` async-context** (deferred from Phase 47):
  hang off the builtin table (builtin_arity_names). stage1 checkCallExpr (checker.go:1659-
  2028) type-checks each builtin's args with ~20 bespoke messages (e.g. `print() cannot
  print type X`, `assert() argument must be Bool, got X`, `len() ...`). Emit only on a
  confident arg type. Plus `await` outside an async context. Larger; slice per builtin group.
- **phase-53 gaps** (independent, smaller): **generic-entity-instantiation arity** (stage1
  checker.go:2068 `generic entity 'X' requires type arguments` / `entity 'X' expects N type
  arguments, got M`), **extern param/return `unknown type`** (0 corpus usage; stage2 parses
  `ExternDecl`), and the stage2 **trait-method contract** parser gap.
  - **CAVEAT (verified 2026-07-03)** for generic-entity arity: a generic constructor call
    parses as an ex_call whose callee is an ex_ident with the type args BAKED INTO THE NAME
    (`Stack<Int>()` → callee.name == "Stack<Int>"; `parse_type(callee.name)` splits base +
    type_args). Anchor at the callee position (= stage1 CallExpr.Pos() = the ident), and use
    the BASE name in the message (stage1 uses expr.Function = "Stack", not "Stack<Int>").
    Arity checks return early, so a wrong-arity fixture emits only the arity error — BUT only
    if there is no `let`: stage1 ALSO infers the constructor's return type and emits a second
    `type mismatch: cannot assign Box to Box` for `let b: Box<Int> = Box<Int,String>(5)`
    (its Equal() compares type args; both print as "Box"). The self-hosted checker returns
    Unknown for constructor calls, so it would MISS that second diagnostic → fixtures must
    use a BARE constructor-call statement, not a let-binding, to stay byte-equal.
- **unary operator typing** (`unary '-' not defined for X`, `unary 'not' requires boolean
  operand, got X`): companion to 48e for checkUnaryExpr — needs ex_unary positions + tightening
  unary inference (currently eager Bool/operand-type). Low value, corpus-invisible.
- **method-call RETURN-type inference**: `infer_expr_type` on a method call still returns
  Unknown; typing it needs generic type-param substitution through the receiver's type args.
- **contract-clause recursion**: the checker does NOT yet recurse contract clauses for
  undeclared-var/arg/operator errors (only boolean-typedness). Needs old()/result/quantifier
  scope handling to avoid false positives. Corpus-invisible gap.

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Known deferred (need new machinery): impl-block-method contracts; immutable-target
assignment/push (needs mutability tracking in Scope).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056.
2. Continue Phase 48 at **48j-c** — builtin argument typing (hang off builtin_arity_names;
   port stage1 checkCallExpr's per-builtin arg messages, one builtin-group per slice) — or
   pick a phase-53 gap (generic-entity arity is well-scoped, but mind the let-mismatch caveat
   above). Keep inference SOUND (Unknown skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
