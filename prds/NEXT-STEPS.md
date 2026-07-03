# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: match-arm checks shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + 14 CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine,
**48b** `if/while condition must be boolean`, **48c** `let` type-mismatch, **48d/48d+** the
type-carrying scope, **48f** function arg-type, **48g** variant arg-type, **48h** assignment
mismatch, **48i.1** `self` + field-access inference, **48i.2** method-call arity + arg-types,
**48e** binary operator typing, **48j-b** contract well-typedness, and **48j-a** the five
match-arm structural checks (variant-exists, duplicate, binding-count, unreachable,
exhaustiveness). `make diff-checker` → **68/68**, **242** checker tests. Full stage1
type-system parity is a large, open-ended goal; the rest (48j-a2 onward) is below.

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

## What 48j-a shipped (this session)

- **match-pos front-end** (88807f7, ADR 0054): MatchArm gains line/col (the pattern's first
  token); parse_match_expr anchors ex_match at the `match` keyword (stage1 MatchArm/
  MatchPattern.Pos() + MatchExpr.Pos()). Additive/inert (ast.intent + parser.intent stay
  formatter fixpoints; all gates unchanged).
- **48j-a match checks** (80b6857): ports stage1 checkMatchExpr for a confident USER-enum
  scrutinee — `variant 'V' is not a variant of enum 'E'`, `duplicate match arm for variant
  'V'`, `variant 'V' has N fields but pattern has M bindings`, `unreachable pattern after
  wildcard '_'`, `non-exhaustive match on enum 'E': missing variants: ...` (enum order, at
  the match keyword). Helpers find_enum_index/enum_has_variant/variant_field_count/
  check_arm_body. Unreachable/unknown-variant arms don't recurse their body (stage1 early
  `continue`). Option/Result + non-user-enum/unknown scrutinees take the fallback.

Earlier this session (also pushed): **48i.2** method-call arity/arg-types (587f084, +fld-pos
033e7dd), **48e** binary operator typing (e2a986f, +binop-pos b77bdf3), and **48j-b**
contract well-typedness (7ca2dab, +contract-pos bc060ae).

## Next: Phase 48j-a2 / phase-53 gaps

Remaining, in rough value order:

- **48j-a2 — match arm-type consistency + scrutinee-must-be-enum**: the two match
  diagnostics deferred from 48j-a. `match arm type mismatch: expected X, got Y` needs
  arm-body inference — type each arm body (in the arm scope, with bindings TYPED from the
  variant's field types) and compare to the first arm's type; emit on a confident mismatch
  (stage1 checkMatchExpr:2996). `match scrutinee must be an enum type, got X` needs certainty
  that the scrutinee type is NOT an enum — safe only for confident PRIMITIVES (Int/Float/
  String/Bool/Char), since Option/Result are enums stage2 doesn't have in prog.enums and
  must not be flagged. This also means `infer_expr_type(ex_match)` could return the arm
  result type (unlocking match-as-let-RHS mismatch) — but only when confident.
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
- **48j-c — builtin argument typing + `await_*` async-context** (deferred from Phase 47;
  hang off the builtin table). Plus **unary operator typing** (`unary '-' not defined for
  X`, `unary 'not' requires boolean operand, got X`) — needs ex_unary positions + tightening
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
2. Continue Phase 48 at **48j-a2** — match arm-type consistency (needs arm-body inference
   with typed bindings) + scrutinee-must-be-enum (confident primitives only). Or pick a
   phase-53 gap (generic-entity arity is well-scoped, but mind the let-mismatch caveat
   above). Keep inference SOUND (Unknown skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
