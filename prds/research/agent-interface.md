# PRD (Research) — Agent Interface (MCP / Structured Tool API)

**Status:** research / proposed
**Decision record:** [ADR 0049](../../docs/decisions/0049-agent-interface.md)
**Theme:** Verifiable Trust Loop (post-Phase 42)
**Date:** 2026-06-15

## 1. Introduction / Overview

Intent's stated audience is AI code assistants. Yet the toolchain is reachable only
the way a human reaches it: a CLI emitting prose, and an LSP serving an editor. An AI
agent that wants to *use* Intent the way the design intends — write code, verify it,
read the counterexample, fix it, re-verify — has to shell out to `intentc` and
scrape stdout.

The highest-leverage "new backend" for Intent is therefore **not** another code
target (native/JS/WASM already prove the multi-target thesis). It is a **structured,
agent-facing interface** that exposes the toolchain's core capabilities — `check`,
`verify`, `counterexample`, `format`, and the `trust manifest` — as first-class
callable tools an agent integrates natively into its reasoning loop.

The natural shape for this is an **MCP (Model Context Protocol) server** (`intentc
mcp` or a sibling binary), since MCP is the emerging standard for exposing tools to
AI agents and is already how this project's own assistant tooling connects to
external capabilities. This PRD scopes that interface and is deliberately sequenced
**after** the counterexample work (ADR 0046), because counterexamples are the most
valuable thing the interface has to offer.

## 2. Goals

- Expose Intent's core read/verify capabilities to AI agents as structured tools,
  not scraped CLI text.
- Reuse the structured outputs defined by the counterexample (ADR 0046), contract
  integrity (ADR 0047), and trust manifest (ADR 0048) work — the interface is a
  transport, not a new source of truth.
- Keep the surface read-and-verify oriented and safe by default (no arbitrary code
  execution beyond compiling/running the user's own Intent sources in a controlled
  way).
- Ship as an optional, separable component — core `intentc` gains no hard runtime
  dependency on it.

## 3. User Stories

### US-001: Verify-and-explain as a tool call
**As an AI agent**, I want to call a `verify` tool on an Intent source and get back
structured results including counterexamples, so I can self-correct without parsing
CLI text.

**Acceptance Criteria:**
- [ ] An MCP server exposes a `verify` tool taking source (path or inline) and
  returning the structured verify result (per-clause status + counterexamples) from
  ADR 0046 — identical vocabulary, not a re-implementation.
- [ ] A `check` tool returns type/parse diagnostics structurally.
- [ ] A `format` tool returns canonical formatting (delegating to the existing
  formatter).

### US-002: Trust manifest as a tool call
**As a review bot**, I want to request the trust manifest for a module, so I can
gate a PR or summarise guarantees.

**Acceptance Criteria:**
- [ ] A `trust_manifest` tool returns the structured manifest (ADR 0048).
- [ ] The tool result shares the verification vocabulary used by `verify`.

### US-003: Safe, optional, documented
**As a maintainer**, I want this to be optional and safe, so it never compromises the
core toolchain.

**Acceptance Criteria:**
- [ ] The server is a separate subcommand/binary; building/using `intentc` without
  it is unaffected.
- [ ] Tools that compile or run user code do so within the existing build pipeline's
  boundaries; the interface adds no new way to execute arbitrary host commands.
- [ ] The tool surface, transport, and configuration are documented; at least one
  end-to-end test exercises a tool round-trip.

## 4. Functional Requirements

- FR-1: An MCP server subcommand exposing at minimum `check`, `verify`, `format`,
  and `trust_manifest` tools.
- FR-2: All tool results reuse the structured schemas from ADR 0046/0047/0048 —
  one verification vocabulary across CLI `--format json` and the agent interface.
- FR-3: The component is optional and dependency-isolated from core `intentc`.
- FR-4: Tool inputs accept both file paths and inline source where sensible.

## 5. Non-Goals

- Exposing write/edit tools that mutate the user's source — the agent edits via its
  own file tools; this interface is read/verify/format only.
- Becoming an LLM client (e.g. running the intent-agreement oracle from ADR 0047
  inside the server is a *possible host* for that capability, but this PRD does not
  bundle a model).
- Replacing the LSP — the LSP serves editors; this serves agents. They share schemas
  but not transport.
- A hosted/remote service — local stdio MCP first, matching how the project's other
  MCP integrations connect.

## 6. Technical Considerations

- Sequencing: this PRD is downstream of ADR 0046 (counterexamples) and benefits from
  0047/0048; building it first would mean exposing thin wrappers with little of the
  value. Recommend implementing after at least the counterexample work lands.
- Transport: stdio MCP is the simplest first cut and matches existing project MCP
  usage; HTTP transport can follow if a remote use case appears.
- Safety: the only code execution is the existing compile/run path for the user's
  own Intent sources. Do not add shell passthrough. Document the trust boundary.
- The server should be thin — capabilities live in the compiler/verifier libraries
  and are surfaced identically by CLI `--format json` and by MCP tools. Avoid logic
  drift between the two.

## 7. Success Metrics

- An agent can complete a full write → `verify` → read-counterexample → fix →
  re-`verify` loop entirely through tool calls, with no CLI text scraping.
- The `verify` tool's output is byte-identical in structure to `intentc verify
  --format json` (proving one shared vocabulary).
- Core `intentc` builds and tests are unaffected by the presence/absence of the
  server.

## 8. Open Questions

- Is MCP the right and only transport, or should there also be a plain JSON-RPC /
  HTTP mode for non-MCP agents? (Leaning: MCP-first; add others on demand.)
- Should the server host the intent-agreement oracle (ADR 0047) as an optional tool,
  given it is the natural place for an LLM-backed capability? (Plausible; keep it
  opt-in and out of core.)
- How is the stage2 self-hosted formatter exposed once it matures (ADR 0040 / Phase
  42 line) — as the `format` tool's backend behind a flag? (Defer until self-hosting
  is the default.)
