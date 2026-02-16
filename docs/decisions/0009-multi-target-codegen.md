# 0009: Multi-Target Code Generation

**Date:** 2026-02-16
**Status:** accepted
**Phase:** Post-POC architecture

## Context

ADR 0001 chose Rust as the compilation target. ADR 0008 introduces an IR
that decouples contract semantics from any specific backend. This opens the
question: should Intent support multiple codegen targets?

The original premise (ADR 0000) is that Intent makes AI-generated code more
correct and auditable. That premise applies equally to backend services, CLI
tools, web frontends, and edge/serverless functions. Limiting Intent to
Rust-only output limits the addressable use cases.

Not everyone writes backend or CLI code. Frontend development (web apps,
browser-based tools, interactive UIs) is a massive category where AI coding
assistants are heavily used. If Intent can only produce native binaries, it
excludes this entire domain.

## Options

- **Rust-only.** Keep the current single backend. Frontend users would need
  to compile Rust to WASM separately, which is possible but adds friction
  and limits the Rust subset that works in browsers.

- **Rust + WASM.** Add a WASM backend (possibly via Rust's wasm32 target or
  direct WASM emission). Covers browser and edge use cases. WASM has a
  well-defined execution model that maps cleanly to Intent's semantics.

- **Rust + JavaScript/TypeScript.** Add a JS/TS backend for direct browser
  and Node.js targeting. Contracts compile to runtime assertions in JS.
  Wider reach but weaker type guarantees at the target level.

- **Multi-target via IR.** With the IR from ADR 0008, treat code generation
  as a pluggable backend interface. Ship Rust first, add targets as demand
  and resources allow. The IR design does not need to change per target --
  only the backend implementation.

## Decision

Multi-target via IR. The IR (ADR 0008) is designed to be target-agnostic.
Each codegen backend implements a common interface that consumes the IR and
emits target-specific code with appropriate contract enforcement.

Planned target priority:
1. **Rust** (current, production-ready) -- native binaries, CLI, backend
2. **WASM** (next) -- browser, edge, portable. Can leverage Rust's
   wasm32-unknown-unknown target initially, then move to direct WASM
   emission if needed
3. **JavaScript/TypeScript** (future) -- frontend, Node.js. Contracts
   compile to runtime checks. TypeScript output preserves type information
4. **Z3/SMT-LIB** (verification target, not execution) -- contract nodes
   translated to SMT formulae for static verification

Each target handles contract enforcement in its own idiom:
- Rust: `assert!()` macros (current approach)
- WASM: `unreachable` traps or imported assertion functions
- JS/TS: `throw` or `console.assert()` with contract metadata
- Z3: proof obligations, not runtime checks

## Consequences

- The codegen interface must be defined before building additional backends.
  This is a design task that should happen during IR implementation
  (ADR 0008).
- Each new backend is an independent effort that can be developed in
  parallel once the IR is stable.
- WASM as the second target gives frontend coverage with minimal new
  complexity (Rust already compiles to WASM; the initial WASM path can
  simply be `intentc build --target wasm` invoking
  `cargo build --target wasm32-unknown-unknown`).
- JS/TS codegen is the most complex additional backend because JavaScript's
  type system and execution model differ significantly from Intent's. This
  is why it is lower priority.
- The `intent` block `verified_by` checking remains the same regardless of
  target -- it operates at the IR level before any backend is invoked.
- This decision does NOT require all targets to be implemented before
  shipping. Rust remains the primary and default target. Additional
  targets are additive.
