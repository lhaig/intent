# PRD — Phase 52: Return-Type + Match + Contract Type Checks

## 1. Introduction / Overview

Phase 52 lands the remaining statement/expression-level type-rule checks on the inference
foundation: **return-type** conformance, **match** arm-type consistency + exhaustiveness,
and **contract well-typedness** (requires/ensures/invariant must be Bool; decreases must be
Int). Strategic frame:
[ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md) (add a Phase-52
ADR for the match/exhaustiveness modelling).

## 2. Goals

- **Return-type**: a `return <expr>` whose inferred type differs from the enclosing
  function/method's declared return type → stage1's mismatch message (confirm wording at
  `checkReturnStmt`); `return;` in a non-Void function, etc. SOUND — skip on Unknown.
- **Match**: arm bodies must share a consistent type; the scrutinee's variants must be
  covered (exhaustiveness) or a wildcard present. Byte-equal messages from `checkMatchExpr`.
- **Contract well-typedness**: `requires`/`ensures` and loop `invariant` predicates must be
  `Bool` (`... clause must be boolean, got <T>` — see `checkConstructor` 1132–1151,
  `checkWhileStmt` invariants 1387); `decreases` must be `Int` (`checkWhileStmt` 1399).

## 3. Design decisions (record in the Phase-52 ADR)

- Thread the enclosing declared return type into `check_body_stmts` (like `type_params`
  was threaded) so `st_return` can compare.
- Match exhaustiveness needs the scrutinee's enum type + that enum's variant set — reuse
  the enum registry; model "covered variants" as a name list (no `Map`).
- Contract clauses are already parsed (Phase 41.1); infer each predicate and check `Bool`.
  Keep the contract-context nuances (`old()`/`result`) out of scope unless a corpus example
  needs them.
- SOUND throughout: skip when the inferred type is Unknown.

## 4. Tasks (indicative)

- US-1: return-type check (thread declared return type; handle `return;`/Void); fixtures.
- US-2: match arm-type consistency + exhaustiveness; fixtures (missing variant, wildcard).
- US-3: contract predicate Bool + decreases Int; fixtures.
- US-4: no-false-positive sweep + tests; docs + validate + push.

Ref stage1: `checkReturnStmt` (1345), `checkMatchExpr`, `checkConstructor` (1108),
`checkMethod` (1161), `checkWhileStmt` (1378–1400), function contract checking.

## 5. Non-Goals

- Generic instantiation arity, extern typing, trait-method contracts (Phase 53).

## 6. Success Metrics

`make diff-checker` green (return/match/contract fixtures byte-equal, 22 examples clean);
all gates + `make validate` green.
