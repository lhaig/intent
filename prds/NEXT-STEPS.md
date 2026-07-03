# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: contract typing shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + NINE CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine,
**48b** `if/while condition must be boolean`, **48c** `let` type-mismatch, **48d/48d+** the
type-carrying scope (params + let-bound vars), **48f** function arg-type, **48g** variant
arg-type, **48h** assignment mismatch, **48i.1** `self` + field-access inference, **48i.2**
method-call arity + arg-types, **48e** binary operator typing, and **48j-b** contract
well-typedness (requires/ensures/invariant must be boolean). `make diff-checker` →
**63/63**, **234** checker tests. Full stage1 type-system parity is a large, open-ended
goal; the rest (48j-a onward) is the continuation below.

**Phase 47 (builtin-call arity) — COMPLETE**. ADR 0055.
**Phase 46 (type foundation + `unknown type`) — COMPLETE**. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 63/63)
```

## What 48j-b shipped (this session)

- **contract-pos front-end** (bc060ae, ADR 0054): the 3 contract-clause parse loops +
  parse_invariant_decl stamp the clause Expr with the requires/ensures/invariant KEYWORD
  position (stage1 RequiresClause/Invariant.Pos() is the keyword, not the predicate).
  Additive/inert (parser.intent stays a formatter fixpoint; all gates unchanged).
- **48j-b contract typing** (7ca2dab): `check_bool_contracts` infers each requires/ensures/
  invariant clause and emits `<kind> clause must be boolean, got X` / `invariant must be
  boolean, got X` at the clause keyword when confidently non-Bool; Unknown skips (old()/
  result/quantifiers/calls infer Unknown). check_functions + check_entities check contracts
  before bodies in stage1 order; check_entities now skips generic entities (stage1
  checkEntities:1057). Impl-method contracts deferred (trait+impl clause mix; parser gap).

Earlier this session (also pushed): **48i.2** method-call arity/arg-types (587f084, +fld-pos
033e7dd) and **48e** binary operator typing (e2a986f, +binop-pos b77bdf3).

## Next: Phase 48j-a — match-arm consistency/exhaustiveness, then the phase-53 gaps

Remaining, in rough value order:

- **48j-a (recommended next) — match-arm consistency / exhaustiveness**: stage1
  `checkMatchExpr` (checker.go:2915) emits `duplicate match arm for variant 'V'`,
  `match arm type mismatch: expected X, got Y`, and `non-exhaustive match on enum 'E':
  missing variants: ...`. Needs match-expr inference: the scrutinee's enum type, each arm
  body's type (arm-consistency), and variant coverage (exhaustiveness). `infer_expr_type`
  currently returns Unknown for ex_match. Likely multiple slices: duplicate-arm (no types
  needed — pure structural) is the easiest first; then exhaustiveness (needs the scrutinee
  enum); then arm-type consistency (needs arm-body inference). Keep each SOUND.
- **phase-53 gaps** (independent, smaller): **generic-entity-instantiation arity** (stage1
  checker.go:2068 `generic entity 'X' requires type arguments` / `entity 'X' expects N type
  arguments, got M`), **extern param/return `unknown type`** (0 corpus usage; stage2 parses
  `ExternDecl`), and the stage2 **trait-method contract** parser gap.
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
2. Continue Phase 48 at **48j-a** — match-arm checking. Read stage1 checkMatchExpr first;
   start with the pure-structural `duplicate match arm` (no inference needed), then
   exhaustiveness, then arm-type consistency. Or pick a phase-53 gap (generic-entity arity
   is well-scoped) for a smaller win. Keep inference SOUND (Unknown skips); one check per
   slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
