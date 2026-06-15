# PRD (Research) — Contract Integrity: Vacuity, Intent-Agreement, Cross-Target Equivalence

**Status:** research / proposed
**Decision record:** [ADR 0047](../../docs/decisions/0047-contract-integrity.md)
**Theme:** Verifiable Trust Loop (post-Phase 42)
**Date:** 2026-06-15

## 1. Introduction / Overview

Intent's central claim is that a human can trust AI-written code by reading its
contracts instead of its implementation. That claim has one structural hole: **a
proof only confirms the implementation matches the contract — it cannot confirm the
contract is the *right* contract.** A wrong, weak, or vacuous `ensures` produces a
green "verified ✓" that means nothing. This is precisely the failure mode an AI
author is most prone to: it is far easier to emit a contract the implementation
trivially satisfies than one that actually pins down the intended behaviour.

This PRD attacks that hole with three complementary, independently-shippable
defences:

1. **Vacuity / triviality warnings** — detect contracts that do not constrain
   anything (e.g. `ensures true`, a postcondition Z3 discharges without reference to
   the body, an `invariant` implied by field types alone).
2. **Contract ↔ intent agreement check** — Intent already links natural-language
   `intent "..."` blocks to `verified_by` clauses. Add a check that the formal
   clauses plausibly capture the English goal (LLM-assisted, advisory).
3. **Cross-target behavioural equivalence** — Intent compiles one source to native,
   JS, and WASM. Derive property tests from the contracts and run them on all
   targets, asserting identical behaviour — a differential trust signal no
   single-target language can offer, and a free catch for codegen divergence.

Each is a separate user story and could become its own implementation phase; they
are grouped here because they share one goal: **make "verified" mean "verified the
right thing."**

## 2. Goals

- Surface contracts that are vacuously true or that fail to constrain their
  declared output, as lint-level diagnostics (warnings, not hard errors).
- Provide an advisory check that flags formal contracts which appear to diverge
  from their linked `intent "..."` natural-language description.
- Stand up a cross-target differential harness that generates properties from
  contracts and asserts native/JS/WASM agree, surfacing any divergence.

## 3. User Stories

### US-001: Vacuity warnings
**As a reviewer**, I want to be warned when a contract does not actually constrain
behaviour, so a green checkmark cannot lie to me.

**Acceptance Criteria:**
- [ ] A new lint/verify diagnostic fires when an `ensures` clause is logically
  `true`, or is provable without using the function body (the model is identical
  with the body replaced by an arbitrary return of the right type).
- [ ] Fires when an entity `invariant` is implied by field types alone (e.g.
  `invariant count >= 0` on a field already typed as an unsigned/Nat-like type if
  such exists, or a tautology over the declared types).
- [ ] Each warning names the clause, its position, and *why* it is considered
  vacuous; warnings are suppressible per-clause with an explicit annotation.
- [ ] Zero false positives on the existing `examples/*.intent` corpus (tune the
  detector against the corpus; document any clause that is legitimately weak).

### US-002: Contract ↔ intent agreement (advisory)
**As an author**, I want to know when my formal `ensures` clauses don't seem to
match the `intent "..."` sentence they claim to satisfy, so the English and the
maths can't drift apart.

**Acceptance Criteria:**
- [ ] For each `intent "<desc>" { verified_by: [...] }` block, an advisory check
  reports whether the referenced clauses plausibly discharge `<desc>`.
- [ ] The check is **advisory only** — it never fails a build or blocks verify; it
  emits an informational diagnostic with a confidence note.
- [ ] The mechanism is pluggable and offline-degradable: if no LLM/oracle is
  configured, the check is skipped with a one-line notice (never a hard dependency
  on a network service in `intentc`).
- [ ] Documented clearly as a heuristic aid, not a proof.

### US-003: Cross-target behavioural equivalence
**As a maintainer**, I want one command that proves the three backends agree on
contract-bearing programs, so codegen bugs surface automatically.

**Acceptance Criteria:**
- [ ] A harness derives property inputs from a function's `requires` (valid-input
  space) and runs the compiled native + JS (+ WASM where runnable) outputs on the
  same inputs, asserting identical results and identical contract-violation
  behaviour.
- [ ] Reports per-function `AGREE` / `DIVERGE (target A: x, target B: y)` with a
  summary and non-zero exit on any divergence.
- [ ] Wired into a `make` target alongside the existing `make diff-formatter`.
- [ ] Works whether or not cargo/node are installed (degrade: skip the unavailable
  target with a logged notice rather than failing).

## 4. Functional Requirements

- FR-1: Vacuity detection reuses the Z3 path — a clause is vacuous if it is valid
  independent of the function body / under a havoc'd body.
- FR-2: The intent-agreement check is an isolated, optional module with a clean
  "no oracle configured → skip" path; no network dependency baked into core verify.
- FR-3: The cross-target harness reuses the existing property-based test generation
  (Phase 3.3) to source inputs and the multi-target build pipeline to run them.
- FR-4: All three are additive — no change to existing verify/lint exit semantics
  except the new (suppressible) warnings.

## 5. Non-Goals

- Proving the contract is "complete" in any formal sense — vacuity detection is a
  heuristic floor, not a completeness theorem.
- Making the intent-agreement check authoritative or build-blocking (it is advisory
  by design; over-trusting an LLM oracle would reintroduce the very problem we are
  guarding against).
- Auto-generating contracts from intent text (separate, larger research question).
- Exhaustive cross-target equivalence for all types in one phase (start with the
  property-testable subset).

## 6. Technical Considerations

- Vacuity-by-havoc: the standard technique is to verify the clause against a body
  replaced by `havoc` (arbitrary return), and if it still proves, the clause does
  not depend on the implementation. Confirm the encoding supports this cheaply.
- The intent-agreement oracle must be swappable (local model, hosted model, or
  none). Keep `intentc` itself free of a hard network/LLM dependency — this is a
  language toolchain, not an AI client. Consider exposing it via the agent
  interface (see [agent-interface PRD](agent-interface.md) / ADR 0049) rather than
  embedding a client.
- Cross-target divergence is exactly the class of bug already hit during dogfooding
  (multiple codegen fixes in prior phases); a standing harness turns those into
  caught-early regressions.

## 7. Success Metrics

- Vacuity detector flags a deliberately-vacuous `ensures true` and a havoc-provable
  postcondition, with zero false positives on the example corpus.
- The cross-target harness catches an injected codegen divergence (e.g. an
  off-by-one deliberately added to the JS backend for one function).
- The intent-agreement check produces sensible advisories on hand-written
  mismatches and is cleanly skippable with no oracle configured.

## 8. Open Questions

- Should vacuity warnings ever be promoted to hard errors under a strict mode, or
  always remain warnings? (Leaning: warnings, with an opt-in `--strict-contracts`.)
- Where should the intent-agreement oracle live — a built-in `intentc` subcommand
  with a pluggable backend, or purely a capability exposed through the agent/MCP
  interface? (Leaning: the latter, to keep core toolchain dependency-free.)
- For cross-target equivalence, how are floating-point and string-encoding
  differences across targets handled — tolerance bands (cf. existing
  `assert_close`) vs exact match? (Decide per type during implementation.)
