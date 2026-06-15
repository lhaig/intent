# PRD (Research) — Verification Trust Manifest

**Status:** research / proposed
**Decision record:** [ADR 0048](../../docs/decisions/0048-verification-trust-manifest.md)
**Theme:** Verifiable Trust Loop (post-Phase 42)
**Date:** 2026-06-15

## 1. Introduction / Overview

Intent has invested heavily in the **writing** side of trust — mandatory contracts,
Z3 verification, `intent` blocks. The **reading** side is comparatively thin. When a
human reviews an AI-authored Intent change, there is no single artifact that answers
the only question that matters at review time: *"What, exactly, has been
guaranteed about this code — and what hasn't?"*

Trust is not "everything is proven." Trust is **knowing precisely what was not
proven.** A reviewer needs to see, at a glance: how many contracts exist, how many
Z3 actually proved, how many are runtime-only, how many depend on an *assumed*
lemma, and whether the targets agree. Most formal-methods tools hide this surface;
exposing it is the core of Intent's human-facing value and is currently
underdeveloped relative to the machinery that produces the guarantees.

This PRD proposes a **trust manifest**: a generated, human-readable (and
machine-readable) summary of the verification state of a module or build, with
"what was *not* verified" as a first-class, prominent section.

## 2. Goals

- Generate a per-module / per-build manifest summarising contract and verification
  status: proven, runtime-only, assumed, unknown/timeout, and cross-target
  agreement.
- Make the **unverified surface** prominent — assumptions, runtime-only checks, and
  solver timeouts are listed explicitly, not buried.
- Emit both a concise human summary (for PR review) and a structured form (for
  tooling / the agent interface).
- Keep it a read-only reporting artifact derived from existing verify state — no
  new proof obligations, no change to verification semantics.

## 3. User Stories

### US-001: One-screen trust summary
**As a reviewer of an AI-authored change**, I want a one-screen summary of what is
guaranteed about a module, so I can decide where to focus my review.

**Acceptance Criteria:**
- [ ] `intentc verify --manifest <file-or-package>` prints a summary like:
  `module bank_account: 12 functions · 34 contracts · 31 proven · 3 runtime-only ·
  0 assumed · 0 unknown · all 3 targets agree`.
- [ ] The summary distinguishes, per clause kind, counts of: proven by Z3,
  runtime-only (not submitted to / not decided by Z3), assumed (depends on an
  explicit assumption), and unknown (solver timeout / gave up).
- [ ] Exit code reflects only hard errors, not the presence of unproven clauses
  (the manifest reports state; it does not redefine pass/fail).

### US-002: The unverified surface, made loud
**As a reviewer**, I want the things that are *not* fully proven surfaced
explicitly, so I am never lulled by a green checkmark.

**Acceptance Criteria:**
- [ ] A dedicated "Not proven" section lists every runtime-only clause, every
  assumption, and every `unknown`, each with source position and reason.
- [ ] If a module is 100% proven with no assumptions, the manifest says so
  explicitly ("0 unverified obligations") rather than staying silent.

### US-003: Machine-readable manifest
**As tooling / an agent**, I want the manifest as structured data, so it can gate CI
or feed a review bot.

**Acceptance Criteria:**
- [ ] `--manifest --format json` emits a versioned structure (counts + per-clause
  records + target-agreement summary).
- [ ] A golden test pins the JSON shape for a sample module.
- [ ] Composes with the counterexample output (ADR 0046) and cross-target
  equivalence (ADR 0047) where those are present, rather than duplicating them.

## 4. Functional Requirements

- FR-1: The manifest is derived purely from existing verify results plus contract
  metadata; it introduces no new verification obligations.
- FR-2: Clause classification (proven / runtime-only / assumed / unknown) is
  explicit and exhaustive — every contract falls into exactly one bucket.
- FR-3: An "assumption" is a first-class, recordable concept (see Open Questions —
  this may require a small surface for declaring assumed lemmas).
- FR-4: Human and JSON outputs are two renderings of one underlying model.

## 5. Non-Goals

- Introducing new proof power or changing what Z3 can decide.
- Gating builds on coverage thresholds (a future policy layer could, but the
  manifest itself only reports).
- A graphical/web dashboard (text + JSON first; a viewer is a later, separate idea
  and overlaps with the retired webapp-target discussion).
- Defining the assumption-declaration syntax in full if one does not already exist
  — scope that as its own decision if needed (see Open Questions).

## 6. Technical Considerations

- This depends on verify state being queryable per-clause with status — the
  per-contract source position work (ADR 0034) is the natural substrate.
- "Assumed" obligations presuppose a way to mark a lemma as assumed. If Intent has
  no such construct today, the first cut can treat the assumed-count as always zero
  and flag the feature as a follow-on, rather than block the manifest on new syntax.
- Cross-target agreement in the manifest should reference, not re-run, the
  cross-target equivalence harness (ADR 0047) when available; degrade to "not
  checked" when it is not.
- Keep the JSON shape aligned with the counterexample schema so an agent consumes
  one coherent verification vocabulary.

## 7. Success Metrics

- For `bank_account.intent`, the manifest correctly reports the split of proven vs
  runtime-only contracts and names every unproven clause.
- A reviewer can answer "what is guaranteed here?" from the manifest alone without
  opening the implementation.
- JSON shape is stable under a golden test and shares vocabulary with ADR 0046/0047
  outputs.

## 8. Open Questions

- Does Intent need an explicit `assume`/assumed-lemma construct to make the
  "assumed" bucket meaningful, or is runtime-only a sufficient first classification?
  (Leaning: ship with proven/runtime-only/unknown first; add `assumed` when an
  assumption construct exists.)
- Should the manifest live per-file, per-package, or both? (Leaning: per-package
  with per-file drill-down.)
- Is there a place for this in the LSP (a "trust" view of the open file)? (Plausible
  follow-on; keep the model LSP-renderable.)
