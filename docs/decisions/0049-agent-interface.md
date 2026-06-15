# 0049: Agent Interface (MCP / Structured Tool API)

**Date:** 2026-06-15
**Status:** proposed (research)
**Phase:** Research — Verifiable Trust Loop (post-Phase 42)
**PRD:** [agent-interface.md](../../prds/research/agent-interface.md)

## Context

Intent's declared audience is AI code assistants, yet the toolchain is reachable only
as a human reaches it: a CLI that emits prose and an LSP that serves an editor. An
agent that wants to use Intent as designed — write, verify, read the counterexample,
fix, re-verify — must shell out to `intentc` and scrape stdout.

When considering "what new backend would most advance the goal," the conclusion was
that it is **not** another code-generation target. Native, JS, and WASM already prove
the multi-target thesis (ADR 0009); a fourth dilutes effort and adds codegen-bug
surface for no thesis gain. The most valuable new surface is an **agent-facing
interface** exposing `check` / `verify` / `counterexample` / `format` / `trust
manifest` as structured, callable tools — i.e. an MCP server. This ADR records that
prioritisation and the shape of the interface.

It is explicitly sequenced **after** ADR 0046 (counterexamples), because the
counterexample is the most valuable thing the interface carries; building it first
would expose thin wrappers around prose.

## Options

### O1. What the new surface should be
- **A. A fourth codegen target (e.g. Python/JVM).** Rejected — does not advance the
  trust thesis; multi-target is already demonstrated; adds maintenance and codegen-bug
  surface.
- **B. An agent-facing structured tool interface (MCP).** [Chosen.] Directly serves the
  language's stated audience and turns the existing CLI/verify capabilities into tools
  an agent integrates into its reasoning loop.

### O2. Transport
- **A. Bespoke protocol.** Rejected — reinventing what MCP standardises.
- **B. MCP over stdio first; HTTP/JSON-RPC later if a remote use case appears.**
  [Chosen.] Matches the emerging standard and this project's existing MCP integration
  pattern; stdio is the simplest correct first cut.

### O3. Source of truth for tool results
- **A. Re-implement formatting/diagnostics inside the server.** Rejected — invites
  logic drift between CLI and server.
- **B. Thin server; capabilities live in the compiler/verifier libraries and are
  surfaced identically by CLI `--format json` and MCP tools, sharing the ADR
  0046/0047/0048 schemas.** [Chosen.] One vocabulary, no drift.

### O4. Write capability and safety
- **A. Expose edit/mutate tools.** Rejected — the agent edits via its own file tools;
  a mutating surface widens the trust boundary for no need.
- **B. Read / verify / format only; code execution limited to the existing
  compile/run path for the user's own sources; no shell passthrough; optional and
  dependency-isolated from core `intentc`.** [Chosen.]

## Decision

Adopt **O1.B + O2.B + O3.B + O4.B**: an optional MCP server (`intentc mcp` or sibling
binary) exposing at minimum `check`, `verify`, `format`, and `trust_manifest` tools,
all returning the structured schemas defined by ADRs 0046/0047/0048 — a transport over
existing capabilities, not a new source of truth. Read/verify/format only, safe by
default, separable from core `intentc`. Sequenced after the counterexample work lands.

Recorded as **proposed (research)**.

## Consequences

**Enables:**
- An agent completes the full write → verify → read-counterexample → fix → re-verify
  loop through tool calls, with no CLI scraping — the workflow the language exists for.
- A single verification vocabulary spanning CLI JSON, LSP (future), and the agent
  interface.
- A natural, opt-in host for the LLM-backed intent-agreement oracle (ADR 0047) without
  putting an LLM dependency in core `intentc`.

**Trade-offs / risks:**
- Another component to maintain; mitigated by keeping it thin and schema-shared.
- MCP is young; betting on it is a judgement that it is the right agent-tooling
  standard (consistent with the project's existing MCP usage).

**Defers:**
- HTTP/remote transport and non-MCP JSON-RPC (add on demand).
- Hosting the intent-agreement oracle as a server tool (opt-in, out of core).
- Exposing the stage2 self-hosted formatter (ADR 0040 / Phase 42 line) as the `format`
  backend — defer until self-hosting is the default.
