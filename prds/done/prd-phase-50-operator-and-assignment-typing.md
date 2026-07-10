> **SUPERSEDED by Phase 48** — this planned work was folded into the Phase 48 expression-inference slices (see `prds/done/prd-phase-48-expression-inference.md`). Kept for history.

# PRD — Phase 50: Operator Typing + Assignment Type-Mismatch

## 1. Introduction / Overview

With the type-carrying scope (Phase 49), operands of binary/unary expressions and
assignment targets/values now infer to concrete types. Phase 50 lands the two type-rule
checks that depend directly on operand typing: **operator-typing errors** (in
`checkBinaryExpr`) and **assignment type-mismatch** (`checkAssignStmt`). Strategic frame:
[ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md) (add a Phase-50
ADR if the emit-ordering needs recording).

## 2. Goals

- Emit stage1's operator errors, byte-equal, when both operands are confidently typed and
  incompatible:
  - `operator '+' not defined for <L> and <R>` (PLUS on non-Int/Float/String).
  - `operator '<op>' not defined for <L> and <R>` (MINUS/STAR/SLASH/PERCENT; EQ/NEQ;
    LT/GT/LEQ/GEQ).
  - `operator '<op>' requires boolean operands, got <L> and <R>` (AND/OR/IMPLIES).
- Emit assignment type-mismatch: `type mismatch: cannot assign <value> to <target>`
  (or stage1's exact wording — confirm at the site).
- SOUND: emit only when both operands (or target + value) are confidently typed; Unknown →
  skip. `make diff-checker` stays clean on the 22 examples.

## 3. Design decisions (record in the Phase-50 ADR)

- Operator errors are emitted during expression traversal in post-order (stage1
  `checkBinaryExpr` checks left, then right, then this operator) — integrate into the
  expression walker so operand-level diagnostics precede the operator diagnostic, matching
  stage1 order. `infer_expr_type` should return the correct result type (or Unknown) so the
  Phase-48 result-type contract still holds.
- Reconcile with Phase 48: today comparison/logical binops infer `Bool` optimistically;
  once operand types are available, an invalid comparison should both emit the operator
  error AND yield Unknown/nil result (mirroring stage1 returning nil), so a downstream
  condition check does not also fire.

## 4. Tasks (indicative)

- US-1: operator-typing checks in the binop path (all operator classes), byte-equal
  messages; fixtures per class.
- US-2: assignment type-mismatch in the `st_assign` path (confirm stage1 message at
  `checkAssignStmt`); fixture.
- US-3: reconcile binop result typing with operand validity (invalid → Unknown result).
- US-4: no-false-positive sweep + tests; docs + validate + push.

Ref stage1: `checkBinaryExpr` (1554–1621, the `c.diag.Errorf` operator sites),
`checkUnaryExpr` (1625), `checkAssignStmt` (1302).

## 5. Non-Goals

- Argument-type mismatch / method-call arity (Phase 51); return/match/contract (Phase 52).

## 6. Success Metrics

`make diff-checker` green (operator + assignment fixtures byte-equal, 22 examples clean);
all gates + `make validate` green.
