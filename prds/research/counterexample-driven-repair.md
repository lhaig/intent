# PRD (Research) — Counterexample-Driven Self-Repair

**Status:** research / proposed
**Decision record:** [ADR 0046](../../docs/decisions/0046-counterexample-driven-repair.md)
**Theme:** Verifiable Trust Loop (post-Phase 42)
**Date:** 2026-06-15

## 1. Introduction / Overview

Intent's value proposition is "an AI writes contract-bearing code, a human trusts
it because the contracts are proven." Today `intentc verify` runs Z3 and reports
**that** a `requires` / `ensures` / `invariant` clause could not be proven. It does
not surface **why** — specifically, it does not hand back the concrete input
assignment (the SMT *model*) that violates the clause.

That missing output is the single highest-leverage gap in the loop, because it is
the signal an AI author needs to *self-correct*. With a structured counterexample
("`balance = 5, amount = 10` violates `ensures balance == old(balance) - amount` at
line 12"), the write → verify → read-failure → fix → re-verify loop closes without
a human in it until the code is green. Z3 already computes a satisfying model when a
verification condition fails; this phase is about **extracting, mapping back to
source, and emitting that model in an AI-first, machine-readable form**.

This is a research PRD: it scopes the direction, the design space, and the open
questions, and is expected to spawn one or more implementation phases once the
approach is validated against the existing verifier.

## 2. Goals

- When `intentc verify` cannot prove a contract, extract the Z3 counterexample model
  and map each variable in it back to a named source-level binding (parameter,
  field, local, `old(...)` snapshot, `result`).
- Emit counterexamples in a **structured, machine-readable format** (JSON) designed
  to be consumed by an AI agent, alongside the existing human-readable diagnostic.
- Make the counterexample reproducible: where feasible, emit a concrete failing
  call (a synthesised input) the author can run against the compiled binary to
  observe the violation.
- Keep the human-facing text diagnostic at least as good as today; the JSON is
  additive, gated behind a flag/format selector.

## 3. User Stories

### US-001: Structured counterexample on verify failure
**As an AI author**, when a contract fails to verify, I want a machine-readable
description of the inputs that break it, so I can fix the implementation or the
contract without guessing.

**Acceptance Criteria:**
- [ ] `intentc verify --format json <file>` emits, per unproven clause, an object
  containing: the clause kind (`requires`/`ensures`/`invariant`/`loop_invariant`/
  `decreases`), the source position (file, line, col — reuse ADR 0034 positions),
  the clause text, and a `counterexample` map of `{ binding_name: value }`.
- [ ] Binding names are source-level, not Z3-internal (`amount`, `old(balance)`,
  `result` — never `x!3`).
- [ ] Clauses Z3 *proves* appear with `status: "verified"` and no counterexample;
  clauses it cannot decide appear as `status: "unknown"` (distinct from
  `"violated"`).
- [ ] The default (no `--format`) human text output is unchanged or improved.

### US-002: Reproducible failing input
**As an author**, I want a concrete input I can actually run, so the counterexample
is verifiable rather than abstract.

**Acceptance Criteria:**
- [ ] For functions whose parameters are of supported types (Int, Bool, String,
  and simple entities), the JSON includes a `repro` field with a literal argument
  vector that triggers the violation.
- [ ] When a faithful repro cannot be synthesised (unsupported type, model
  references an unbounded/symbolic value), `repro` is `null` with a `repro_reason`
  string — never a misleading partial repro.

### US-003: Loop in an agent harness
**As an agent harness**, I want a stable contract for the verify output so I can
drive a fix loop, so the format must be versioned and not break silently.

**Acceptance Criteria:**
- [ ] The JSON carries a top-level `schema_version`.
- [ ] A golden-file test in `internal/verify` (or equivalent) pins the JSON shape
  for a known-failing example.
- [ ] Documented in `INTENT.md` and the verify section of the README.

## 4. Functional Requirements

- FR-1: The verifier captures the Z3 model on `unsat`-of-the-negation / `sat`
  counterexample path (whatever the current encoding produces on failure).
- FR-2: A model-to-source mapping layer translates solver symbols to source
  bindings, including `old(...)` snapshots and `result`.
- FR-3: A JSON emitter behind `--format json`, additive to the human formatter.
- FR-4: A repro synthesiser for supported parameter types, with explicit
  null-and-reason fallback.
- FR-5: `status` distinguishes `verified` / `violated` (model found) / `unknown`
  (solver gave up / timeout).

## 5. Non-Goals

- Automatically *editing* the user's code to fix the violation (the agent does
  that; we only supply the signal).
- Counterexamples for properties Z3 returns `unknown` on — we report `unknown`,
  not a fabricated model.
- Repro synthesis for arbitrary nested generic/collection types in this phase
  (Int/Bool/String/simple-entity first; widen later).
- Changing the verification *encoding* itself (this phase reads what the existing
  encoding produces).

## 6. Technical Considerations

- Where does Z3 run today, and does the current code request a model on failure?
  The first implementation step is to confirm the solver invocation requests and
  retains the model (`get-model`) rather than only the sat/unsat verdict.
- `old(...)` and `result` already have an encoding for postconditions (see the
  per-contract position work, ADR 0034); the mapping layer must reuse those names.
- JSON output should compose with the existing diagnostic pipeline so LSP can later
  surface counterexamples inline (future; out of scope here but keep the shape
  LSP-friendly).
- Timeouts: Z3 `unknown` must be a first-class outcome, not collapsed into failure.

## 7. Success Metrics

- On a deliberately-buggy example (a `withdraw` that forgets the balance check),
  `intentc verify --format json` returns a counterexample whose `repro`, when run
  against the compiled binary, actually triggers the contract assertion.
- The JSON shape is stable under a golden test and documented with a
  `schema_version`.
- No regression to `intentc verify` human output or exit codes.

## 8. Open Questions

- Should the repro be emitted as an Intent `test "..." { }` block (so it drops
  straight into the suite) rather than a raw argument vector? (Leaning: offer
  both — raw vector in JSON, optional `--emit-repro-test`.)
- How much of the model do we surface when the violated clause depends on many
  bindings — full model, or just the bindings referenced by the clause? (Leaning:
  clause-referenced bindings by default, full model behind a verbosity flag.)
- Does this belong partly in the LSP surface (inline counterexample on the failing
  contract) in a follow-on phase? (Likely yes; keep the JSON LSP-shaped.)
