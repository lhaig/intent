# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: builtin arg typing + async-context (builtins + await expr) DONE)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + 18 CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a-48h** (inference engine,
condition-boolean, let-mismatch, typed scope, function/variant arg-type, assignment mismatch),
**48i.1/48i.2** (self + field access, method-call arity/arg-types), **48e** binary operator
typing, **48j-b** contract well-typedness, **48j-a/48j-a2** the complete match checking (all
seven checkMatchExpr diagnostics), **48j-c/48j-c2a-c** builtin argument typing
for the uniform-type group + print + assert_close + len + assert_panics + assert_eq
(mismatch + Float), and **48j-c2d/e** the async-context checks (await_all/await_any/
timeout builtins + the `await` expression, ADR 0057 — async flag threaded on the
Scope), plus **48j-c2f** the assert_eq entity no-eq-method comparable-set rule.
`make diff-checker` → **84/84**, **274** checker tests. Full stage1 type-system parity
is a large, open-ended goal; the rest (remaining Phase 48 gaps / phase-53) is below.

**Phase 47 (builtin-call arity) — COMPLETE**. ADR 0055.
**Phase 46 (type foundation + `unknown type`) — COMPLETE**. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 84/84)
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

- **Builtin arg typing + async-context — DONE this session**: len() (48j-c2a),
  assert_panics (48j-c2b), assert_eq mismatch+Float (48j-c2c), the async-only builtins
  await_all/await_any/timeout (48j-c2d, ADR 0057 — `<name> can only be used inside async
  functions`; async flag `scope.in_async` threaded on the Scope rather than as a ~40-site
  parameter), and the **`await` EXPRESSION** async check (48j-c2e — reused scope.in_async;
  stamped the ex_await keyword position first per ADR 0054, then added the ex_await case that
  also recurses the operand, closing the latent await-operand recursion gap).
- **Remaining Phase 48 gaps** (all sound false negatives / corpus-invisible today):
  - **assert_eq comparable-set — remainder** (entity no-eq-method is DONE, 48j-c2f): the
    eq-method SIGNATURE sub-checks (wrong return / param count / param type), plus Map/Future
    rejection and generic-type-param recursion (need generic .String() rendering).
  - **async-test no-await warning** — stage1 `test "…" declared 'async' but contains no
    'await' expression` (checker.go:1009); needs testSawAwait tracking (a warning, distinct
    from the async-context errors).
  - **spawn/try operand recursion** — ex_spawn/ex_try in check_expr_names still don't recurse
    their operand (the same latent gap the await case just closed for ex_await).
  - **unary operator-typing** — `unary '-'/'not'` errors; needs ex_unary positions + tighter
    unary inference (corpus-invisible).
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

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056
   (+ ADR 0057 for the Scope async flag).
2. All builtin arg typing + the full async-context surface (builtins + await expr) is DONE
   (48j-c/48j-c2a-e). Best next slices: the remaining Phase 48 gaps (spawn/try operand
   recursion is trivial and mirrors the await case; then assert_eq comparable-set, async-test
   no-await warning, unary operator-typing). Or pick a **phase-53 gap** (generic-entity arity
   is well-scoped, but mind the let-mismatch caveat below). Keep inference SOUND (Unknown
   skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
