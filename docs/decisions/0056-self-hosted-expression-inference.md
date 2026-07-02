# 0056: Self-Hosted Checker — Expression Type Inference (sound-but-incomplete)

**Date:** 2026-07-02
**Status:** accepted
**Phase:** 48 — expression type inference + type-rule checks

## Context

Phase 46 gave the self-hosted checker a `Type` tree + `parse_type` + `type_is_known`
resolver; Phase 47 added builtin-call arity. Every REMAINING stage1 diagnostic is
type-rule-based — condition-must-be-boolean, `type mismatch`, operator typing,
argument-type mismatch, return-type, match arm consistency, contract well-typedness — and
each needs a **type for each expression** (stage1 `checkExpression` returns a `*Type`,
storing it in `exprTypes`). Building this all-at-once is high-risk: a type-rule check that
fires on an expression the checker infers *wrongly* would false-positive on the valid
corpus and break `make diff-checker`.

## Decision

### D1 — `infer_expr_type` is SOUND but may be INCOMPLETE
`infer_expr_type(e) -> Type` returns a concrete `Type` **only when it is certain** that
stage1's `checkExpression(e)` would return the same type; otherwise it returns an
**Unknown sentinel** (`Type` with `name == ""`, tested via `is_unknown_type`). It never
returns a *wrong* type. This mirrors stage1's own `condType != nil` guard: every type-rule
site already skips when the type is nil, so mapping "can't infer" → Unknown → skip is
exactly stage1's behaviour for the cases stage1 also can't type — and for the cases stage1
CAN type but we can't yet, skipping is a false *negative* (never a false positive), which
is invisible on the valid corpus.

### D2 — Type-rule checks fire only on a confident inference
A check emits only when `infer_expr_type` returns a known type violating the rule (e.g.
condition type known and `!= Bool`). Unknown → skip. So each check is corpus-safe the
moment it lands, regardless of how incomplete inference still is, and byte-equality is
proven per-check with crafted fixtures where inference IS confident.

### D3 — Grow inference and checks in independent slices
Land inference for more `Expr` kinds (and, later, a type-carrying scope for idents /
field / call-return types) incrementally, each slice keeping `diff-checker` green. Wire
type-rule checks one at a time behind the same gate. First slices need no typed scope:
- **48a** — `infer_expr_type` for literals (`Int/Float/String/Bool/Char`), comparison +
  logical binops → `Bool`, arithmetic binops → operand type when both operands are the
  same known type, `not` → `Bool`, unary `-` → operand type, paren → inner. Everything
  else (ident, call, method, field, index, match, array, range, lambda, …) → Unknown.
- **48b** — `condition must be boolean` (`if`/`while`) using 48a; emits on a confident
  non-Bool literal/arithmetic condition, skips idents/calls/comparisons(Bool)/unknown.
- Later — a type-carrying scope (idents, params, `self`, let-inferred), then `type
  mismatch`, operator typing, argument-type mismatch, method-call arity (receiver type),
  return-type, match/contract checks.

### D4 — Gate: `make diff-checker` (unchanged shape)
Inference is validated by (a) staying 44/44 while it is internal-only, (b) in-language
unit tests asserting inferred types, and (c) per-check fixtures byte-equal vs stage1 with
no new false positives on the 22 valid examples.

## Consequences

### Benefits
- De-risks the largest remaining area: every increment is provably corpus-safe because
  inference is sound; incompleteness only ever *under*-reports.
- Reuses the Phase 46 `Type`/`parse_type`/`type_is_known` and the Phase 47 builtin table.

### Costs
- Deferred coverage (idents/calls/etc. until a typed scope lands) means some stage1
  diagnostics are not yet reproduced — tracked, corpus-invisible false negatives.
- Duplicates stage1's `checkExpression`/`checkBinaryExpr` typing rules by hand; the
  differential keeps them honest.

### Non-goals (this ADR's first slices)
- A type-carrying scope, operator-typing errors, `type mismatch`, argument-type mismatch,
  method-call arity, return-type, match/contract checks — later slices on this foundation.

## References
- [ADR 0053](0053-self-hosted-checker-type-foundation.md) — `Type` / `parse_type` / `type_is_known`.
- [ADR 0055](0055-self-hosted-builtin-arity.md) — builtin arity (argument typing hangs off this inference).
- `internal/checker/checker.go` — `checkExpression` (1490), `checkBinaryExpr` (1554), `checkIfStmt`/`checkWhileStmt` (1356/1378).
