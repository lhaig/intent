# Phase 28: Multi-Param Iteration for `--target intent` Test Generation

**Status:** Shipped (2026-06-01; commits 2da9fec..HEAD)
**Milestone:** v1.2 — Self-Improvement Foundations (Phase 17.A.2 — last prerequisite for legacy Rust-testgen retirement)
**Decision:** [ADR 0037](../../docs/decisions/0037-testgen-multi-param-iteration.md)

## Goal

Extend `intentc test-gen --target intent` to emit nested `while` loops for free functions with 2+ Int parameters, replacing the current single-example-call fallback. Cap total Cartesian iterations at 1000 by trimming each param's range to `floor(1000^(1/N))` values where N is the param count.

This closes Phase 17.A.2 — the last blocker for retiring the legacy Rust testgen path (which Phase 17.A.4 will do in a follow-on phase once both prerequisites are in place).

## Success Criteria

- [x] `generateIntentTestForFunction` handles the `allInt && len(params) >= 2` case: emit nested `while` loops, one per param, with precondition guards + call + ensures asserts in the innermost body
- [x] Per-param iteration count is capped at `floor(1000^(1/N))` for N params; each param's `[lo, hi]` from `intRange` is trimmed to that count
- [x] Param names are used as loop variables (consistent with existing single-Int code)
- [x] Precondition guards (`if not (<req>) { continue; }`) appear in the innermost body, once per `requires` clause
- [x] Non-Int params still fall through to the single-example-call branch (no regression in current behaviour)
- [x] New unit tests cover: two-Int-param function with derived ranges, three-Int-param function (caps each at ~10), per-param cap math correctness, fallback for mixed Int+String params, regression on the existing single-Int path
- [x] Generated output for a real multi-Int example (e.g., a synthetic `min` / `max` function) parses cleanly
- [x] `make validate` green
- [x] No regression in any existing testgen test
- [x] Phase 17 PRD `17.A.2` entry updated to reflect "satisfied by Phase 28"

## Reference

- ADR 0037: `docs/decisions/0037-testgen-multi-param-iteration.md`
- Existing intent emission: `internal/testgen/intentgen.go`
- Single-Int loop code: `generateIntentTestForFunction` switch case `allInt && len(params) == 1`
- Default-args fallback: same function, `default:` branch
- Constraint analysis: `internal/testgen/constraints.go` — `AnalyzeConstraints`, `ParamConstraint`
- `intRange` helper for `[lo, hi]` derivation
- Existing testgen tests: `internal/testgen/intentgen_test.go`

## Tasks

### 28.1 Per-param cap derivation

**Files:** `internal/testgen/intentgen.go`

Add `perParamCap(n int) int` returning `floor(1000^(1/n))` for n ≥ 1. Hand-rolled rather than `math.Pow + math.Floor` to avoid float drift on a fixed-cap calculation; the small table for n ∈ [1, 8] suffices:

| N | cap per param |
|---|---|
| 1 | 1000 |
| 2 | 31 |
| 3 | 10 |
| 4 | 5 |
| 5 | 3 |
| 6 | 3 |
| 7 | 2 |
| 8+ | 2 |

**Acceptance:** Unit test `TestPerParamCap` asserts the table values.

### 28.2 Multi-Int loop emission

**Files:** `internal/testgen/intentgen.go`

Extend the switch in `generateIntentTestForFunction` with a new case `allInt && len(params) >= 2`:

```go
case allInt && len(params) >= 2:
    cap := perParamCap(len(params))
    // For each param, derive [lo, hi] and trim to lo + cap - 1.
    ranges := make([]paramRange, len(params))
    for i, p := range params {
        lo, hi := intRange(constraints[p.Name], -10, 10)
        if hi-lo+1 > cap {
            hi = lo + cap - 1
        }
        ranges[i] = paramRange{name: p.Name, lo: lo, hi: hi}
    }
    // Emit nested while loops in param order. The innermost body
    // emits the precondition guards, the call, and the ensures asserts.
    emitNestedIntLoops(&sb, ranges, func(inner *strings.Builder, indent string) {
        emitPreconditionGuard(inner, f.Requires, indent)
        if hasResult {
            fmt.Fprintf(inner, "%slet __r: %s = %s(%s);\n", indent,
                typeRefToIntent(f.ReturnType), f.Name, paramNamesCSV(params))
        } else {
            fmt.Fprintf(inner, "%s%s(%s);\n", indent, f.Name, paramNamesCSV(params))
        }
        emitEnsuresAssertsIndented(inner, f.Ensures, hasResult, indent)
    })
```

`emitNestedIntLoops` walks the ranges, opening each `let mutable <p>: Int = <lo>; while <p> <= <hi> { ... }` in turn, calling the innermost-body callback at full depth, then closing the loops in reverse with `<p> = <p> + 1; }`.

**Acceptance:** Unit test on a 2-Int-param function asserts the emitted shape: outer + inner loop with the call in the inner body.

### 28.3 Refactor helper signatures

**Files:** `internal/testgen/intentgen.go`

The existing single-Int case already uses `emitPreconditionGuard` and `emitEnsuresAssertsIndented` — both take an indent string. The new multi-loop case re-uses both. No signature changes needed; just call sites.

`paramNamesCSV` was added in Phase 27 for entity-method emission. Reusable here for the call site.

**Acceptance:** Same as 28.2 — the test validates the call.

### 28.4 Unit tests

**Files:** `internal/testgen/intentgen_test.go`

- `TestPerParamCap` — table-driven coverage of the cap function for N ∈ [1, 8].
- `TestGenerateIntentTwoIntParams` — function with two `Int` params and bounded `requires`; assert nested while loops + capped range + assertion against `result`.
- `TestGenerateIntentThreeIntParams` — three Int params; assert each cap-trimmed to ≤ 10 iterations.
- `TestGenerateIntentMixedIntStringParamFallsBack` — function with `Int, String`; assert it still uses the single-example-call branch (no while loop).
- Regression: ensure `TestGenerateIntentSingleIntParamFunction` still passes unchanged.

**Acceptance:** All new tests pass; existing tests unchanged.

### 28.5 Live-emit smoke test

**Files:** `internal/testgen/intentgen_test.go`

End-to-end: define a small multi-Int function inline, run `GenerateIntent`, parse the result. Assert it parses without error.

**Acceptance:** Smoke test passes.

### 28.6 Docs

**Files:** `docs/ROADMAP.md`, `prds/done/phase-17-testing-polish.md` (note 17.A.2 satisfied), `prds/done/phase-28-testgen-multi-param.md` (status flip), `INTENT.md` (if it mentions test-gen scope)

- ROADMAP: `### Phase 28: testgen Multi-Param Iteration -- SHIPPED (date)` under v1.2.
- Phase 17 PRD: update 17.A.2 status; note that 17.A.1 + 17.A.2 are both in place; 17.A.4 (Rust path retirement) is now deliverable.
- This PRD: status flip + checkbox ticks.

**Acceptance:** `make validate` green; no stale "multi-param deferred" claims.

## Out of Scope

- **Bool iteration.** ADR 0037 §O1.A; future ADR.
- **Enum iteration.** Same.
- **Float / String / Array param iteration.** No natural enumeration; PRNG required.
- **Phase 17.A.4 Rust-path retirement.** Separate phase, unblocked by this one.
- **Coverage measurement on generated tests.** Phase 17.H bucket; orthogonal.
- **Configurable iteration cap.** ADR 0037 §O2.C; defer until a real user case demands it.

## Suggested Order

1. **28.1 Per-param cap helper** — pure function with unit table
2. **28.2 Multi-Int loop emission** — main work
3. **28.3 Helper-signature refactor** — should be no-op given Phase 27's `paramNamesCSV`
4. **28.4 Unit tests** — lock the shape
5. **28.5 Smoke test** — end-to-end parse check
6. **28.6 Docs + PRD flip** — last
