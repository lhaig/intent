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

## 2. Status

### Shipped (2026-07-10) — the high-value items
Each landed as a gate-verified slice (`make diff-checker` byte-equal, selfcheck-checker green):
- **Method-call return-type inference** — user-entity methods, with bare type-param
  substitution through the receiver's type args (Phase 48).
- **Contract-clause name/type recursion** — `requires`/`ensures`/`invariant` clauses
  recursed through `check_expr_names`, with `result`/`old()` handled as contract keywords (Phase 48).
- **impl-block-method contracts** — the trait's + impl's requires/ensures checked in the impl method scope.
- **immutable-target checks** — per-binding mutability in the `Scope` + the five stage1
  diagnostics (assign / index-assign to an immutable var/array; push/set/remove on an immutable array/map).

### Deferred — the low-value residual (all sound false negatives, corpus-invisible)
A line was drawn under Phase 58 here: the following are each low-value and deferred.
- **Built-in-method return typing** — `infer_expr_type` returns Unknown for built-in
  receiver methods (Array/Map/String/Char/Result/Option); faithfully porting stage1's full
  method-return table is high-effort and false-positive-prone for near-zero gain.
- **Extern FFI-bridgeability messages** — the "not bridgeable across the FFI boundary"
  messages for known-but-unbridgeable extern types (Map/entity/…); extern param/return
  `unknown type` already ships (Phase 53).
- **Module-qualified `entity 'X.Y' has no constructor`** — the non-qualified form ships
  (Phase 53); the module-qualified variant needs the module-qualified constructor-call path.
- **`@target_specific("wasm")` annotation warning** — needs annotation semantic checking in stage2.
- **assert_eq on a function-typed operand** — inference returns Unknown for lambdas/function
  refs, so it is naturally skipped (essentially a non-issue).

## 3. Non-goals

Return-type-mismatch on `return` statements — stage1's `checkReturnStmt` does not
compare the return value to the declared type, so there is nothing to port.

## 4. Success metric

Each item, when picked up, ships as an independent byte-equal-gated slice with an
invalid fixture in `check-fixtures/` (or a shared-parser/formatter/linter change +
the full gate suite), exactly like the Phase 48/53 slices — `make diff-checker`
stays green throughout.
