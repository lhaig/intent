# 0057: Self-Hosted Checker — Async Context via the Scope

**Date:** 2026-07-03
**Status:** accepted
**Phase:** 48j-c2d — async-context builtins (`await_all`/`await_any`/`timeout`)

## Context

Stage1 (`internal/checker/checker.go`) tracks a single boolean `c.inAsyncFunc`
on the checker struct. It is set to `fn.IsAsync` on entry to `checkFunction`
(927) and to `t.IsAsync` on entry to `checkTest` (1002), restored on exit, and
left untouched (i.e. `false`) while checking entity/impl methods and
constructors. Three builtins consult it — `await_all` (1956), `await_any`
(1983), `timeout` (2009) — each emitting `<name> can only be used inside async
functions` after its arity check passes; the `await` **expression** (3161)
consults it too (a still-deferred stage2 check).

The self-hosted checker has no mutable per-run checker object: it is a set of
pure functions threading an immutable `Scope` and the program. The async flag
must reach the deeply-nested `check_expr_names` builtin site, which is called
recursively from ~40 places across `check_expr_names` and `check_body_stmts`.

## Decision

**Ride the async flag on the already-threaded `Scope`, not as a new parameter.**

- Add `field in_async: Bool` to the `Scope` entity.
- `scope_empty` seeds it `false`; `scope_define_typed` and `scope_enter`
  **preserve** it (`s.in_async`), so it flows unchanged into every child block,
  match arm, and lambda scope — matching stage1, which never resets
  `inAsyncFunc` for nested blocks or lambdas.
- A new `scope_set_async(s, flag)` flips it once per function/test entry:
  `check_functions` sets `f.is_async`, `check_tests` sets `t.is_async`. Method,
  constructor, and impl-body scopes are built from `scope_enter(global)` and so
  inherit `false` — exactly stage1's behaviour (it never sets `inAsyncFunc`
  during entity/impl checking, so async-only builtins are rejected there).
- The async-only builtins emit their error in the builtin path **after** the
  arity check passes and **before** recursing arguments (stage1 order), reading
  `scope.in_async`. The error is unconditional on that flag — it does not depend
  on `infer_expr_type` confidence — because `scope.in_async` is computed exactly
  as stage1 computes `inAsyncFunc`, so it is byte-equal at every call site.

Rejected alternative: adding an `in_async: Bool` parameter to `check_expr_names`
and `check_body_stmts`. It would touch ~40 call sites, each a chance to thread
the wrong value, for a flag that the immutable-`Scope` design already carries
everywhere for free (the same vehicle the Phase 48d typed-scope arrays use).

## Consequences

### Benefits
- Zero signature churn: the flag reaches every expression site via the vehicle
  already passed to all of them.
- Correct propagation into nested blocks/arms/lambdas falls out of
  `scope_enter` preserving the field — no per-construct wiring.
- Reuses cleanly for the deferred `await`-expression async check (same read).

### Costs / non-goals
- The `Array<Future<T>>` / `Int` **argument-type** checks for these builtins
  remain deferred (they need generic `.String()` rendering; sound false
  negatives per ADR 0056).
- The separate `test "…" declared 'async' but contains no 'await' expression`
  warning (stage1 `testSawAwait`) is a distinct, still-unported diagnostic; this
  ADR does not address it. Fixtures therefore use async **functions**, not async
  **tests**, for their clean-context cases.

## References
- [ADR 0056](0056-self-hosted-expression-inference.md) — the governing
  sound-but-incomplete inference strategy this builds on.
- `internal/checker/checker.go` — `inAsyncFunc` (60), set at 927/1002; consulted
  at 1956/1983/2009 (builtins) and 3161 (`await` expression).
- `selfhost/checker/check.intent` — `Scope.in_async`, `scope_set_async`,
  `is_async_only_builtin`.
