# 0046: Counterexample-Driven Self-Repair

**Date:** 2026-06-15
**Status:** proposed (research)
**Phase:** Research — Verifiable Trust Loop (post-Phase 42)
**PRD:** [counterexample-driven-repair.md](../../prds/research/counterexample-driven-repair.md)

## Context

`intentc verify` runs Z3 over the contracts (`requires` / `ensures` / `invariant` /
loop invariants / `decreases`) and reports whether each verification condition holds.
On failure it reports *that* a clause is unproven. It does not surface the concrete
input assignment — the SMT model — that violates the clause.

That model is the single most valuable signal for the language's stated author: an AI.
The design thesis is "AI writes contract-bearing code, a human trusts the proven
contracts." For that to work the AI must be able to *converge* on correct code. Today
it writes, sees "unproven," and guesses. A concrete counterexample ("`balance = 5,
amount = 10` violates `ensures balance == old(balance) - amount`") turns the verifier
into a closed-loop teaching signal: write → verify → read the failing input → fix →
re-verify, with no human required until the code is green.

Z3 already computes a satisfying model on the failure path; the work is to extract it,
map solver symbols back to source-level bindings (including `old(...)` and `result`),
and emit it in a form an AI consumes reliably — i.e. structured JSON, not prose.

This sits at the top of a four-part "Verifiable Trust Loop" research theme (ADRs
0046–0049). It is sequenced first because it delivers the most value alone and because
0049 (the agent interface) is largely a transport for what this ADR produces.

## Options

### O1. Output format for counterexamples
- **A. Human prose only (status quo).** Rejected — unparseable by an agent; the whole
  point is machine consumption.
- **B. Structured JSON behind `--format json`, human text unchanged.** [Chosen.]
  Additive, non-breaking, agent-first, and reusable by the agent interface (0049) and
  trust manifest (0048).
- **C. Replace human output with JSON.** Rejected — breaks human/CLI use and existing
  expectations.

### O2. Variable naming in the model
- **A. Pass through Z3-internal symbols (`x!3`).** Rejected — meaningless to author or
  agent.
- **B. Map to source-level bindings, including `old(...)` and `result`.** [Chosen.]
  Reuses the per-contract position machinery (ADR 0034). The mapping is the substance
  of the work.

### O3. Reproducibility
- **A. Report the abstract model only.** Weak — abstract values are hard to act on.
- **B. Synthesise a concrete runnable repro for supported parameter types, with an
  explicit null-and-reason fallback otherwise.** [Chosen.] A repro that actually
  triggers the runtime assertion makes the counterexample verifiable, not just
  plausible. Never emit a misleading partial repro.

### O4. Solver `unknown` handling
- **A. Collapse `unknown` into failure.** Rejected — conflates "proven false" with
  "couldn't decide," which misleads the author into "fixing" correct code.
- **B. First-class `unknown` status distinct from `violated`.** [Chosen.]

## Decision

Adopt **O1.B + O2.B + O3.B + O4.B**: extract the Z3 model on verification failure,
map it to source bindings, and emit per-clause structured results via `intentc verify
--format json` with `status ∈ {verified, violated, unknown}`, a clause-referenced
`counterexample` map, and a best-effort runnable `repro` (null with reason when not
synthesisable). Human output stays and improves. The JSON carries a `schema_version`
and is pinned by a golden test, because downstream consumers (agent interface, trust
manifest) depend on its stability.

This is recorded as **proposed (research)**; it becomes **accepted** with an assigned
implementation phase once the approach is validated against the current verifier (in
particular, confirming the solver invocation retains a model on failure).

## Consequences

**Enables:**
- A self-correcting AI authoring loop — the core mechanism the language exists to
  support.
- A shared verification JSON vocabulary that ADR 0048 (manifest) and ADR 0049 (agent
  interface) build on rather than reinvent.
- Future LSP inline counterexamples on the failing contract (kept in mind via an
  LSP-shaped JSON; out of scope here).

**Trade-offs / risks:**
- Model extraction and source mapping are only as good as the current encoding
  exposes; the first task is to confirm `get-model` is available and faithful.
- Repro synthesis is type-limited initially (Int/Bool/String/simple entities); other
  types degrade to `repro: null` with a reason rather than a wrong repro.

**Defers:**
- Counterexamples for `unknown` results (none possible — reported as `unknown`).
- LSP surfacing and `--emit-repro-test` (emitting the repro as an Intent `test` block).
