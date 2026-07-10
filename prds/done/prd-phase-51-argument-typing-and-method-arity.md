# PRD — Phase 51: Argument-Type Mismatch + Method-Call Arity

## 1. Introduction / Overview

With operand/receiver types available (Phases 49–50), Phase 51 lands the call-site type
checks: **argument-type mismatch** for function / variant / builtin calls (including the
builtin argument-typing and `await_*` async-context deferred from Phase 47), and
**method-call arity** (which needs the receiver's entity type). Strategic frame:
[ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md) +
[ADR 0055](../../docs/decisions/0055-self-hosted-builtin-arity.md) (the builtin table
argument-typing hangs off).

## 2. Goals

- Function/variant argument-type mismatch: each argument's inferred type vs the declared
  param/field type (`function 'f' argument ...` / `variant 'V' field 'x' expects <T>, got
  <U>` — confirm exact stage1 wording at the sites).
- Builtin **argument typing** (byte-equal, from Phase 47's deferral): `print() cannot print
  type <T> (accepts Int, Float, Bool, String)`, `assert() argument must be Bool, got <T>`,
  `assert_eq() type mismatch: ...`, `len() requires Array, Map, or String argument, got
  <T>`, the file/env/http/json String-arg checks, `sleep()`/`char_from_codepoint()` Int
  args, and the `await_all`/`await_any`/`timeout` `Array<Future<T>>` + async-context checks.
- **Method-call arity**: infer the receiver's entity type, look up the method's arity on
  that entity, emit `method 'm' expects N arguments, got M` (confirm wording), byte-equal.
- SOUND: emit only when arg/receiver types are confidently inferred; Unknown → skip.

## 3. Design decisions (record in the Phase-51 ADR)

- Reuse the Phase-47 builtin table for the argument slots; add per-builtin expected arg
  types (the argument-typing was deliberately deferred there).
- Method arity: build an entity → method-name → param-count registry (like the function/
  variant arity registries); resolve the receiver via `infer_expr_type` (needs Phase 49's
  entity-typed idents / field access / `self`).
- Preserve stage1 emit order: arity before argument-type within a call; builtin/variant/
  function/method precedence as in `checkCallExpr` / `checkMethodCallExpr`.

## 4. Tasks (indicative)

- US-1: function + variant argument-type mismatch; fixtures.
- US-2: builtin argument-typing + `await_*` async-context (the Phase-47 deferral); fixtures.
- US-3: method-call arity via receiver-type inference + a method-arity registry; fixtures.
- US-4: no-false-positive sweep + tests; docs + validate + push.

Ref stage1: `checkCallExpr` (1655–2028 builtin arg-typing; function/variant arg checks),
`checkMethodCallExpr` (2282), `checkModuleQualifiedCall` (2547).

## 5. Non-Goals

- Return-type, match arm consistency/exhaustiveness, contract well-typedness (Phase 52).

## 6. Success Metrics

`make diff-checker` green (argument-type + method-arity fixtures byte-equal, 22 examples
clean); all gates + `make validate` green.
