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
