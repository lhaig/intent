# Research PRDs

Forward-looking, exploratory PRDs that scope a direction, its design space, and its
open questions **before** an implementation phase is committed. A research PRD is not
yet on the schedule; when it is picked up it moves (or spawns a phase) into
`../active/`, gets a phase number in `../TASKS.md`, and its ADR flips from
`proposed (research)` to `accepted`.

## Decision flow

Every research PRD here pairs with an Architecture Decision Record under
`../../docs/decisions/`. The PRD says *what* and *how*; the ADR records *why this
direction and which option*, with the option space considered. The lifecycle:

```
idea → research PRD (here) + ADR (proposed)
     → picked up → phase in TASKS.md + PRD moves to active/ + ADR → accepted
     → shipped → PRD to done/ + ADR stays accepted (revised in place if changed)
```

## Theme: Verifiable Trust Loop (post-Phase 42)

Four linked proposals that strengthen both halves of Intent's trust thesis — the AI
*writing* loop and the human *reading* loop — rather than widening the language. They
share one structured verification vocabulary so each builds on the last instead of
reinventing it.

| PRD | Decision (ADR) | What it does | Sequence |
|-----|----------------|--------------|----------|
| [counterexample-driven-repair.md](counterexample-driven-repair.md) | [0046](../../docs/decisions/0046-counterexample-driven-repair.md) | Emit Z3 counterexamples (failing inputs + repro) as structured JSON so an AI can self-correct | 1st — highest leverage; others build on its schema |
| [contract-integrity.md](contract-integrity.md) | [0047](../../docs/decisions/0047-contract-integrity.md) | Defend against "verified but wrong": vacuity warnings, intent↔contract agreement, cross-target equivalence | Independent; vacuity is the natural first slice |
| [verification-trust-manifest.md](verification-trust-manifest.md) | [0048](../../docs/decisions/0048-verification-trust-manifest.md) | A per-clause "what's guaranteed / what isn't" report for human review | After per-clause state is queryable; reuses 0046 schema |
| [agent-interface.md](agent-interface.md) | [0049](../../docs/decisions/0049-agent-interface.md) | Expose check/verify/counterexample/manifest as MCP tools for agents | After 0046 lands (it's the transport for 0046's output) |

### Why these, and not more backends or syntax

The language core is sufficient to demonstrate the thesis. The under-built part is the
compiler's *relationship with its two readers*: making verification **talk back** to
the AI (0046), be **honest** with the human (0047, 0048), and be **reachable** by
agents (0049). Adding a fourth codegen target or more syntax scales the language
without scaling the thesis; see ADR 0049 for that rejection in full.
