# 0037: Multi-Param Iteration in `--target intent` Test Generation

**Date:** 2026-06-01
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 17.A.2 — last prerequisite for legacy Rust-testgen retirement)

## Context

`intentc test-gen --target intent` (Phase 16 / Phase 27) currently handles two function shapes deterministically:

- **No-param functions:** one example call, assert each `ensures`.
- **Single-Int-param functions:** nested `while` loop over the constraint-derived `[lo, hi]` range, asserting each `ensures` per iteration.

Anything else — multi-param functions, non-Int params — falls through to a single-example call with default values. That's the third branch of `generateIntentTestForFunction` in `internal/testgen/intentgen.go`:

```go
default:
    // Fallback: example call with default values; assert ensures.
    args := defaultArgsForParams(params)
    ...
```

Single-example coverage on multi-param functions is materially weaker than the legacy Rust testgen path, which uses PRNG-driven property tests over multi-param functions. This is the last gap in Intent emission before the Rust path can be retired (Phase 17.A.4). Phase 27 closed the entity / method gap (17.A.1). 17.A.2 closes multi-param iteration.

The design space is small but worth pinning down. Three axes shape the decision:

1. **Which param types support iteration?** Int has natural ranges from `requires` constraints. Bool has two values. Unit-variant enums have N values. Float / String / Array / Entity / data-carrying enums don't have a natural finite enumeration without escalation to PRNG or symbolic synthesis.

2. **What's the iteration cap?** Multi-param Cartesian iteration grows multiplicatively. A test that runs `10³ = 1000` iterations per second is fine; one that runs `10⁶` blocks the test suite. Phase 17 PRD picked 10³ as the cap.

3. **How is the cap enforced when individual param ranges are wide?** Auto-balance (shrink each param's range proportionally until the product fits) or hard-clip individual ranges?

### Precedent

Property-based testing tools — QuickCheck, Hypothesis, fast-check, AutoTest (Eiffel) — universally use randomized sampling over input domains, not Cartesian product iteration. Their multi-param story is "compose generators and sample." Random sampling has two advantages over Cartesian iteration: (a) it scales to unbounded domains, and (b) shrinking on failure finds minimal counter-examples.

Intent's existing single-Int strategy is *exhaustive within bounds* rather than random. That's a deliberate choice — exhaustive iteration is deterministic, reproducible, and aligns with the verifier's "every contract is checked" stance. The trade-off is that it can't cover unbounded or large domains; it's only useful when the `requires` clauses produce small bounds.

Extending to multi-param Cartesian iteration keeps the deterministic-bounded model. The legacy Rust testgen path is the closest thing to a "real" PBT generator Intent has had; retiring it without re-creating the PRNG model is a deliberate choice — Intent's testing story is verifier-aligned exhaustive, not random PBT. A future ADR could add randomized property tests as a parallel mode (`--target intent-prop`?), but that's not v1.

## Options

### O1. Eligible param types for iteration

**A. Multi-Int only.** [Chosen for v1.2.] Every param must be `Int` to participate in nested iteration. Otherwise fall back to single-example call (existing behaviour). Smallest scope; lowest risk; closes the gap Phase 17.A.2 names ("Int with `requires` constraints").

**B. Int + Bool.** Adds 2-value iteration over Bool params via an `Int`-index inner loop with a `Bool` projection. Real but awkward in source: `let mutable __b_idx: Int = 0; while __b_idx <= 1 { let p: Bool = __b_idx == 1; ... }`. Doable; doubles the testable param shapes; only modest extra complexity.

**C. Int + Bool + unit-variant enums.** Adds enum iteration via the same Int-index-plus-projection trick (`if __e_idx == 0 { let c: Color = Red; ... } else ...`). Material extra complexity for emission and the projection block; v1 falls back instead.

**D. All types** including Float, String, Array, data-carrying enums. Requires generators (PRNG); escalates the v1 scope past what's deliverable here. Rejected.

### O2. Cap on total Cartesian iterations

**A. Hard cap at 1000 (≈ 10³).** [Chosen.] Matches Phase 17 PRD's stated cap. Tests run in milliseconds on a modern machine.

**B. No cap.** A function with 4 params each over `[-100, 100]` = 200⁴ = 1.6 billion iterations. Out of the question for a test suite.

**C. Configurable cap via env var or flag.** Speculative; defer until a real user case demands it.

### O3. Auto-balancing strategy when product exceeds the cap

**A. Trim each param's range proportionally** until the product fits the cap. Preserves shape — every param is iterated; coverage is even across params. [Chosen.]

**B. Trim only the largest range.** Simpler but uneven coverage.

**C. Per-param fixed cap.** `min(range_size, ceil(cap^(1/N)))` per param. Equivalent to (A) in practice; simpler to implement. [Chosen — actually this is the simpler form of (A).]

Concrete rule for **(C)**: each param's iteration count is at most `floor(cap^(1/N))`. For cap=1000:
- N=1: 1000
- N=2: ~31
- N=3: ~10
- N=4: ~5
- N=5: ~3

Then trim each param's `[lo, hi]` to `[lo, min(hi, lo + per_param_cap - 1)]`. Documented in ADR as "we may not cover the full constraint range when multi-param iteration would otherwise blow up; the per-param cap matters more than the lower bound."

### O4. Fallback when one or more params aren't Int

**A. Fall back to single-example call** (existing default arm). [Chosen.] Preserves current behaviour for unsupported shapes.

**B. Skip the function.** Worse — loses coverage.

### O5. Variable naming and loop shape

Use the param names directly as loop variables. Existing single-Int code does this. The nested loops use the same pattern, with the precondition-guards-then-call-then-asserts block in the innermost loop body.

### O6. Precondition guards

Keep the existing `if not (<requires>) { continue; }` pattern per clause, emitted in the innermost loop body before the call.

### O7. Naming the test

`"auto: <function> contracts"` — same as the existing single-Int case. Unchanged.

## Decision

**O1.A (multi-Int only) + O2.A (cap 1000) + O3.C (per-param cap = floor(cap^(1/N))) + O4.A (single-example fallback) + O5 + O6 + O7.**

1. Extend `generateIntentTestForFunction` to handle the case `allInt(params) && len(params) >= 2`: emit nested `while` loops, one per param, with the precondition guards and assertion block in the innermost body.

2. Cap each param's iteration count at `floor(1000^(1/N))` where N = number of params. Trim the param's `[lo, hi]` from the existing `intRange` helper accordingly.

3. Use param names as loop variables; the precondition guards reference them by name.

4. Functions with any non-Int param continue to fall through to the single-example-call branch (existing behaviour, unchanged).

5. Bool, unit-variant enums, and other iterable types are documented as future-ADR enhancements. They're not blockers for Rust-path retirement (the Rust path is also Int-heavy in practice).

## Precedent (collected for the record)

| System | Multi-input strategy | Notes |
|---|---|---|
| QuickCheck (Haskell) | Random sampling per generator; shrinking on failure | Composable generators. No Cartesian iteration. |
| Hypothesis (Python) | Random sampling + shrinking + targeted property testing | Same shape as QuickCheck. |
| fast-check (JS/TS) | Same composable-generator model | Same. |
| AutoTest (Eiffel) | Bounded random over constructed objects | Adaptive iteration count; no fixed cap. |
| Pex (legacy Microsoft Research) | Z3-driven concrete input synthesis | Solves constraints to find inputs satisfying `requires`. |
| Dafny | Doesn't generate runtime tests | Proves at compile time. |

Intent's deterministic-bounded-iteration model is genuinely different from these. The closest fit is Pex's "synthesize inputs satisfying constraints" — but Pex used Z3; Intent's v1 uses static bounds analysis only. A future ADR could add Z3-driven synthesis (cross-cutting with ADR 0033's deferred verify-aware stripping); that's the natural escalation when single-bounds analysis runs out.

## Consequences

**Accepted trade-offs:**

- Float, String, Array, entity, and data-carrying-enum params still fall through to single-example. Documented as future work.
- The per-param cap means a function with N=4 params each over `[-10, 10]` only iterates each param over `[-10, -6]` (5 values × 4 params = 625 iterations) instead of the full range. The cap UX is "test depth shrinks with breadth."
- Bool and unit-variant enums are *not* iterated in this phase, even though the design space allows it. Future ADR.

**Things this enables:**

- Multi-param functions get deterministic exhaustive coverage within their bounds, replacing the single-example-call fallback.
- Phase 17.A.4 (retire the legacy Rust testgen path) becomes deliverable — both 17.A.1 (Phase 27) and 17.A.2 (this phase) are now in place.
- Intent's testing story stays verifier-aligned: exhaustive deterministic iteration with explicit bounds, not random PBT.

**Things this defers:**

- Bool iteration (the projection trick is straightforward but adds emission complexity; the cap means even moderate Bool inclusion eats range from Ints).
- Unit-variant enum iteration (same projection trick).
- Data-carrying enum / entity / Float / String / Array generators (these need PRNG or Z3 synthesis).
- Random / property-based mode as a parallel `--target intent-prop` (future ADR; would re-create the PRNG capabilities the Rust path provided).
- Coverage measurement of generated tests (Phase 17.H; orthogonal).

**Open follow-ups:**

- Phase 17.A.4 — retire the legacy Rust testgen path. Unblocked by this phase; needs its own PRD to do the actual removal (file/module deletions, `intentc test-gen --target rust` error message change).
- Random / PRNG-driven generator ADR — separate; would also unlock unbounded-domain testing.
- Bool + enum iteration extension — small follow-on ADR if a real user case appears.
