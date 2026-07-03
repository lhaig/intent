# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: builtin arg typing started)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + 18 CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a-48h** (inference engine,
condition-boolean, let-mismatch, typed scope, function/variant arg-type, assignment mismatch),
**48i.1/48i.2** (self + field access, method-call arity/arg-types), **48e** binary operator
typing, **48j-b** contract well-typedness, **48j-a/48j-a2** the complete match checking (all
seven checkMatchExpr diagnostics), and **48j-c** builtin argument typing for the uniform-type
group + print. `make diff-checker` → **74/74**, **253** checker tests. Full stage1 type-system
parity is a large, open-ended goal; the rest (48j-c2 / phase-53) is below.

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

## What 48j-c shipped (this session)

- **uniform-type builtins** (42b3f0c): `builtin_arg_type` maps each builtin whose args all
  require one simple type → that type (assert→Bool, char_from_codepoint/sleep→Int, read_file/
  write_file/create_dir/file_exists/env_get/http_post/http_get/json_get/json_path/emit_event→
  String). In the builtin arity-match path, a confidently other-typed arg → `NAME() argument
  [N ]must be T, got X` at the call (numbered iff arity>1). No-type_args + confident args only.
- **print** (bd3d22f): accepts only Int/Float/Bool/String (not Char) → `print() cannot print
  type X (accepts Int, Float, Bool, String)` (uses base .name, byte-equal for generic/entity).
- **assert_close** (87d04c3): each of the 3 args must be Float → `assert_close() argument N
  (label) must be Float, got X` (labels actual/expected/epsilon).

**Match checking (48j-a/a2) is COMPLETE** — all seven checkMatchExpr diagnostics byte-equal.
Also pushed this session: **48i.2** method calls, **48e** binary operators, **48j-b** contracts.

## Next: Phase 48j-c2 / phase-53 gaps

Remaining, in rough value order:

- **48j-c2 — remaining builtin arg typing + `await_*` async-context**: assert_eq
  (`assert_eq() type mismatch: actual is X, expected is Y` + the entity-eq / comparable-set
  rules — complex, uses .String()), len (`len() requires Array, Map, or String argument, got
  X` — needs generic .String()), assert_panics (`Fn() -> Void`). The async builtins
  await_all/await_any/timeout emit `<name> can only be used inside async functions` —
  this needs an async-context flag threaded through the checker (stage1 c.inAsyncFunc), which
  stage2 does NOT track yet; that flag is the real unlock (also gates the deferred `await`
  expression check). Hang off builtin_arg_type / a new table.
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
2. Continue Phase 48 at **48j-c2** — the remaining builtin arg checks (assert_close/assert_eq/
   len/assert_panics) hang off the same builtin_arg_type path; the async await_* builtins need
   an inAsyncFunc flag threaded through the checker first. Or pick a phase-53 gap (generic-
   entity arity is well-scoped, but mind the let-mismatch caveat above). Keep inference SOUND
   (Unknown skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
