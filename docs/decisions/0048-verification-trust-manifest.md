# 0048: Verification Trust Manifest

**Date:** 2026-06-15
**Status:** proposed (research)
**Phase:** Research — Verifiable Trust Loop (post-Phase 42)
**PRD:** [verification-trust-manifest.md](../../prds/research/verification-trust-manifest.md)

## Context

Intent's machinery is weighted toward the *writing* side of trust — contracts, Z3,
`intent` blocks. The *reading* side has no dedicated artifact. A human reviewing an
AI-authored change cannot answer, at a glance, the question that governs how much they
need to read: *"What has actually been guaranteed about this code, and what hasn't?"*

Trust is not "everything is proven" — it is **knowing precisely what was not proven.**
A reviewer needs the split: contracts proven by Z3 vs runtime-only vs assumed vs
solver-timeout, plus whether the targets agree. ADR 0034 (per-contract source
positions) makes per-clause verify state addressable; this ADR records the decision to
turn that state into a human- and machine-readable **trust manifest** whose most
prominent section is the *unverified surface*.

## Options

### O1. What the manifest reports
- **A. Pass/fail only.** Rejected — that is the status quo and it is exactly what lulls
  a reviewer.
- **B. A per-clause classification — proven / runtime-only / assumed / unknown — plus
  cross-target agreement, with the unproven items listed explicitly.** [Chosen.] The
  unverified surface is the point; it must be loud, not summarised away.

### O2. Output shape
- **A. Human text only.** Insufficient for CI/bots.
- **B. Both a concise human summary and a versioned JSON, as two renderings of one
  model.** [Chosen.] Shares vocabulary with ADR 0046 (counterexamples) and ADR 0047 so
  agents see one coherent verification language.

### O3. Relationship to verification semantics
- **A. Let the manifest redefine build pass/fail (e.g. fail if < 100% proven).**
  Rejected — coverage policy is a separate concern; conflating it with reporting makes
  the manifest coercive and harder to adopt.
- **B. Read-only report; exit codes reflect only hard errors.** [Chosen.] The manifest
  introduces no new proof obligations and does not change what passes.

### O4. The "assumed" bucket
- **A. Require an `assume`/assumed-lemma construct before shipping the manifest.**
  Rejected — blocks a useful report on new syntax design.
- **B. Ship with proven / runtime-only / unknown first; add `assumed` once an
  assumption construct exists.** [Chosen.] Incremental; the bucket can read zero until
  the construct lands.

## Decision

Adopt **O1.B + O2.B + O3.B + O4.B**: `intentc verify --manifest` produces a per-clause
trust report (proven / runtime-only / [assumed] / unknown + cross-target agreement)
with an explicit, prominent "Not proven" section, in both human and versioned-JSON
form, derived purely from existing verify state. It is read-only and does not redefine
pass/fail. The `assumed` bucket is deferred until an assumption construct exists.

Recorded as **proposed (research)**. Depends on per-clause verify state (ADR 0034) and
shares schema with ADR 0046; references rather than re-runs the cross-target harness
(ADR 0047).

## Consequences

**Enables:**
- A reviewer can answer "what is guaranteed here?" from one artifact without reading
  the implementation — the human half of the trust thesis, finally tooled.
- CI gates and review bots can consume the JSON.
- A coherent verification vocabulary across counterexamples, integrity checks, and the
  manifest.

**Trade-offs / risks:**
- The manifest is only as honest as the underlying classification; runtime-only must
  never be silently counted as proven.
- Without an assumption construct, the "assumed" dimension is initially absent — the
  manifest must not imply zero-assumptions means more than it does.

**Defers:**
- An `assume`/assumed-lemma language construct (its own decision if pursued).
- Any coverage-threshold *policy* layer that gates builds.
- A graphical/web viewer (overlaps the retired webapp-target discussion; text + JSON
  first).
