# PRD — Phase 58: Self-Hosted Checker — remaining parity tail

## 1. Overview

Phases 48/53 drove the self-hosted (stage2) checker to byte-equal parity with
stage1 across the valid corpus plus every invalid fixture (`make diff-checker`
100/100). This PRD collects the **remaining** stage1 diagnostics that the stage2
checker does not yet reproduce. Every one is a **sound false negative** — the
checker never emits a WRONG diagnostic; it only, in these narrow cases, emits
FEWER than stage1 on certain INVALID inputs. None fire on any valid program, so
none affect self-hosting or a release. They are catalogued here so the parity gap
is explicit rather than silent (ADR 0056: Unknown → skip is always safe).

## 2. Deferred items

### Inference / name-resolution (ADR 0056 sound-but-incomplete)
- **Method-call return-type inference.** `infer_expr_type` returns Unknown for a
  method call; typing it needs generic type-param substitution through the
  receiver's type args AND stage1's full built-in-method return-type table.
  Consequence: checks keyed off a method-call result (let-mismatch, assert_eq,
  arg-typing) skip on method-call operands.
- **Contract-clause name/type recursion.** `requires`/`ensures` clauses are checked
  for boolean-typedness but not recursed for undeclared-variable / argument /
  operator errors. Needs `old()` and `result` handling (no stage2 AST nodes for
  them) plus quantifier bound-var scope, without false-positives on valid contracts.

### Needs new machinery
- **impl-block-method contracts.** Contracts on impl-block methods are not checked
  (trait+impl clause interaction).
- **immutable-target assignment / push.** No mutability tracking in the `Scope`, so
  assigning/pushing to an immutable target is not flagged.

### Narrow edges
- **assert_eq on a function-typed operand** — stage1 emits "does not support
  function types"; stage2 inference returns Unknown for lambdas/function refs, so
  it is skipped.
- **Module-qualified `entity 'X.Y' has no constructor`** — the non-qualified form
  is done (Phase 53); the module-qualified variant needs the module-qualified
  constructor-call path.
- **Extern FFI-bridgeability messages** — extern param/return `unknown type` is done
  (Phase 53); the "not bridgeable across the FFI boundary" messages for known-but-
  unbridgeable types (Map/entity/etc.) are not ported.
- **`@target_specific("wasm")` annotation warning** — the second checker warning
  (checkTestAnnotations); needs annotation semantic checking in stage2.

## 3. Non-goals

Return-type-mismatch on `return` statements — stage1's `checkReturnStmt` does not
compare the return value to the declared type, so there is nothing to port.

## 4. Success metric

Each item, when picked up, ships as an independent byte-equal-gated slice with an
invalid fixture in `check-fixtures/` (or a shared-parser/formatter/linter change +
the full gate suite), exactly like the Phase 48/53 slices — `make diff-checker`
stays green throughout.
