# PRD — Phase 48: Expression Type Inference + first type-rule checks

## 1. Introduction / Overview

Every remaining stage1 checker diagnostic is type-rule-based and needs a type for each
expression (stage1 `checkExpression` returns `*Type`). Phase 48 builds `infer_expr_type`
and lands the type-rule checks that need **no type-carrying scope** yet. Strategic frame:
[ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md).

## 2. Goals

- `infer_expr_type(e) -> Type` — **sound but incomplete**: a concrete `Type` only when
  certain stage1 agrees, else an Unknown sentinel (`Type` with `name == ""`).
- Wire the first type-rule checks on it, emitting only on a confident (non-Unknown)
  result so they are corpus-safe while inference is incomplete.
- Byte-equal via `make diff-checker`; the 22 valid examples stay clean.

## 3. Design decisions

Per ADR 0056: D1 sound-but-incomplete inference (Unknown → skip, mirroring stage1's
`condType != nil` guards); D2 checks fire only on confident inferences; D3 grow inference
and checks in independent slices behind the gate; D4 diff-checker gate.

## 4. Tasks

### US-001 (48a): `infer_expr_type` engine — DONE
**AC:** literals (`Int/Float/String/Bool/Char`), comparison + logical binops → `Bool`,
arithmetic → operand type when both match else Unknown, `not` → `Bool`, unary `-` →
operand type, paren → inner; idents/calls/method/field/index/match/array/range/lambda →
Unknown. `type_unknown`/`is_unknown_type`/`type_named` helpers. Gate stays 44/44.

### US-002 (48b): `if`/`while` condition-must-be-boolean — DONE
**AC:** emit `if/while condition must be boolean, got <T>` at the stmt position, only when
`infer_expr_type(cond)` is a confident non-Bool. Ref stage1 `checkIfStmt`:1359 /
`checkWhileStmt`:1380. Fixtures per keyword + no-false-positive tests (comparison/ident/
bool cases).

### US-003 (48c): `let` type-mismatch — IN PROGRESS
**AC:** in `check_body_stmts` st_let, after the RHS undeclared check and only when the
declared type is known, `infer_expr_type(rhs)`; if confident and its name differs from the
declared type's → emit `type mismatch: cannot assign <rhs> to <declared>` at the stmt
position. Unknown RHS → skip (stage1 guards on `valueType != nil`, so an undeclared RHS is
not also a mismatch). Ref stage1 `checkLetStmt`:1279. Fixture + tests.

### US-004 (48.d): docs + validate + push
**AC:** ROADMAP + NEXT-STEPS + checker README + this PRD; `make validate` + all gates
green; commit + push each slice.

## 5. Non-Goals (deferred to Phase 49+)

- A type-carrying scope (ident/param/self/field/call-return inference) — Phase 49.
- Operator-typing errors, assignment type-mismatch — Phase 50.
- Argument-type mismatch, method-call arity, builtin argument-typing — Phase 51.
- Return-type, match arm consistency/exhaustiveness, contract well-typedness — Phase 52.

## 6. Success Metrics

`make diff-checker` green (fixtures byte-equal, 22 examples clean), 190+ checker tests,
all formatter/linter/self-check gates + `make validate` green.

## 7. Completion (2026-07-10) — COMPLETE

The inference engine (`infer_expr_type`, ADR 0056) plus every type-rule check that
fires on the corpus shipped across many slices. The originally-planned Phases 49-52
were **superseded** by these Phase 48 slices (48e binary operators, 48i method-call
arity/arg-types, 48j-a match, 48j-b contracts, 48j-c builtin argument typing, 48j-c2d/e
async-context). The closing tail landed 2026-07-10: **spawn/try operand recursion**,
the **async-test-no-await warning** (added warning-severity support to the checker),
**full assert_eq comparable-set parity** (eq-method signature sub-checks + Map/Future
rejection + generic recursion, via new `type_to_string`/`type_equal`), and **unary
operator typing**. Sibling Phase 53 items also landed: **entity has-no-constructor** and
**extern param/return unknown-type**. `make diff-checker` **100/100**, ~296 in-language
checker tests, all self-check / formatter / linter / emit-sweep gates + `make validate`
green throughout.

Method-call return-type inference, contract-clause name recursion (`result`/`old()` as
contract keywords), impl-block-method contracts, and the immutable-target checks (Phase 58)
also shipped 2026-07-10 — `make diff-checker` **110/110**, ~311 checker tests. The remaining
**sound false negatives** (built-in-method return typing, extern FFI-bridgeability messages,
the module-qualified has-no-constructor variant, and the `@target_specific("wasm")` warning)
are catalogued in [prd-phase-58-checker-parity-tail.md](../backlog/prd-phase-58-checker-parity-tail.md).
They never emit a wrong diagnostic and never fire on valid code — safe to defer.
