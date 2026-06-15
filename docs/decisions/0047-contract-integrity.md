# 0047: Contract Integrity — Vacuity, Intent-Agreement, Cross-Target Equivalence

**Date:** 2026-06-15
**Status:** proposed (research)
**Phase:** Research — Verifiable Trust Loop (post-Phase 42)
**PRD:** [contract-integrity.md](../../prds/research/contract-integrity.md)

## Context

A Z3 proof confirms the implementation satisfies the contract. It cannot confirm the
contract is the *right* contract. This is the structural hole in Intent's trust
thesis: a wrong, weak, or vacuous `ensures` produces a green "verified ✓" that
guarantees nothing — and an AI author is precisely the kind of writer most likely to
emit a contract the implementation trivially satisfies, because that is the path of
least resistance.

ADR 0003 chose runtime assertions and the project layered Z3 verification on top; ADR
0046 closes the AI's *convergence* loop. Neither addresses whether the thing being
proven is meaningful. This ADR records the decision to invest in **contract
integrity** — defences that make "verified" mean "verified the right thing" — and the
choice of which defences to pursue and how aggressive each should be.

Three candidate defences emerged:
1. **Vacuity detection** — flag contracts that do not constrain behaviour.
2. **Intent-agreement** — check formal clauses against their `intent "..."` English.
3. **Cross-target equivalence** — exploit the three backends as mutual oracles.

## Options

### O1. Vacuity detection technique
- **A. Syntactic only (catch literal `ensures true`).** Cheap but shallow; misses the
  common case of a postcondition provable without the body.
- **B. Semantic via havoc.** [Chosen.] Re-verify the clause against a body replaced by
  an arbitrary return of the right type; if it still proves, the clause does not
  depend on the implementation and is vacuous. Catches the real failure mode; reuses
  the existing Z3 path.

### O2. Severity of vacuity findings
- **A. Hard error.** Rejected as default — would break legitimately-weak contracts and
  fight adoption.
- **B. Suppressible warning, with an opt-in `--strict-contracts` to escalate.**
  [Chosen.] Reports the risk without dictating policy; ADR 0004's "separate advisory
  layer" precedent (linter) applies.

### O3. Intent-agreement authority
- **A. Build-blocking proof of agreement.** Rejected — there is no sound way to *prove*
  English matches maths; an LLM oracle treated as authoritative would reintroduce the
  very over-trust this theme guards against.
- **B. Advisory-only heuristic, oracle pluggable, fully skippable when no oracle is
  configured.** [Chosen.] Useful as a drift detector; never load-bearing. Core
  `intentc` must not gain a hard LLM/network dependency — the oracle is hosted
  elsewhere (e.g. the agent interface, ADR 0049).

### O4. Cross-target equivalence
- **A. Skip — trust per-target tests.** Rejected — multiple codegen-divergence bugs
  were found by hand during dogfooding; a standing harness turns those into caught
  regressions for free.
- **B. Derive properties from `requires` (valid-input space), run on native/JS/WASM,
  assert agreement; degrade gracefully when a toolchain is absent.** [Chosen.] Reuses
  property-based test-gen (Phase 3.3) and the multi-target pipeline. A differential
  trust signal unique to a multi-target verified language.

## Decision

Pursue all three defences as **independently-shippable** units under one theme:
**O1.B + O2.B + O3.B + O4.B**. Vacuity detection (havoc-based, warning-level,
`--strict-contracts` to escalate); intent-agreement (advisory, pluggable, skippable,
no core network dependency); cross-target equivalence (a `make` harness alongside
`make diff-formatter`). All additive — no change to existing verify/lint exit
semantics beyond new suppressible warnings.

Recorded as **proposed (research)**. Each defence can become its own implementation
phase; vacuity detection is the most self-contained and the natural first.

## Consequences

**Enables:**
- "Verified" gains teeth — a green check is harder to fake.
- Codegen divergences are caught automatically rather than during manual dogfooding.
- A drift signal between English intent and formal contracts.

**Trade-offs / risks:**
- Vacuity-by-havoc must be tuned to zero false positives on the example corpus, or it
  becomes noise that gets ignored.
- The intent-agreement oracle is a heuristic; presenting it as anything stronger would
  be actively harmful. It is explicitly advisory and explicitly optional.
- Cross-target equivalence must handle float/string-encoding differences (tolerance vs
  exact) per type — decided during implementation.

**Defers:**
- Any notion of contract *completeness* as a formal property (this is a heuristic
  floor, not a theorem).
- Auto-generating contracts from intent text (separate, larger research question).
- The decision on where the intent-agreement oracle physically lives (leaning: the
  agent interface, ADR 0049, to keep core `intentc` dependency-free).
