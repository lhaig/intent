# 0045: `args()` Builtin — Command-Line Arguments (Self-Hosting CLI Gap)

**Date:** 2026-06-15
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 42 — runnable self-hosted formatter)

## Context

[ADR 0040](0040-self-hosted-formatter-strategy.md) commits to stage2 tools
(Intent-in-Intent). Phase 42 makes the stage2 formatter a *runnable* CLI tool
(`selfhost/formatter/main.intent`) and wires it into `intentc fmt --self-hosted`.
A formatter CLI must know which file to format, but today's Intent cannot read
its command line:

| Operation needed by a CLI tool | Today |
|---|---|
| Read the invoked file path / flags | No syntax or builtin. Only `env_get(name)` exists (environment, not argv). |
| Iterate over passed arguments | No `argv` exposure. |

Per ADR 0040's gap-driven rule ("when a stage2 tool can't be expressed in today's
Intent, capture the language gap as an ADR + phase"), command-line access is the
next gap. It is not formatter-specific: the eventual self-hosted compiler, linter,
and package manager all need argv. This ADR specifies the minimum viable surface —
a single `args()` builtin. Flag parsing, subcommands, and stdin streaming are out
of scope; they compose on top of `args()` or get their own ADRs.

### Precedent

| Language | Argv surface | Index 0 | Notes |
|---|---|---|---|
| **Rust** | `std::env::args() -> Args` (iterator of `String`). | Program name. | `.collect::<Vec<String>>()` is the idiomatic materialization. |
| **Go** | `os.Args []string`. | Program name. | Flat slice; `flag` package builds on it. |
| **C** | `int argc, char **argv`. | Program name. | The original model everything else echoes. |
| **Python** | `sys.argv: list[str]`. | Script path. | Index 0 is the script, not the interpreter. |
| **Node/JS** | `process.argv: string[]`. | `node` binary; `[1]` is the script. | Two leading entries (runtime + script) before user args. |
| **Java** | `String[] args` param to `main`. | First *user* arg (no program name). | Program name absent entirely — inconsistent with the C lineage. |

### What Intent should pick

Intent favours a small, predictable surface. The chosen model is **Rust + Go**:
`args()` returns `Array<String>` where **`args()[0]` is the program name and
`args()[1]` is the first user argument**. This is the dominant convention (C, Rust,
Go, Python all put the program/script at index 0), so call sites read naturally and
port from those languages without an off-by-one trap.

Crucially, the two compiled backends are normalized to this contract:

- **Rust:** `std::env::args().collect::<Vec<String>>()` — already program-name-first.
- **JS (Node):** `process.argv.slice(1)` — drops the `node` binary so the script
  path lands at index 0 and the first user arg at index 1, matching Rust. (Plain
  `process.argv` would put the script at index 1 and break the shared contract.)

Returning a materialized `Array<String>` (not a lazy iterator) keeps it consistent
with how every other Intent collection builtin behaves and lets `len(args())` and
`args()[i]` work with the existing array machinery — no new iterator type.

## Decision

### 1. `args()` builtin

- Signature: `args() -> Array<String>`. Zero arguments; passing any is a checker
  error: `args() takes no arguments, got N`.
- Semantics: returns the process command-line arguments, program/script name at
  index 0, first user argument at index 1.

### 2. Backend lowering

| Target | Emit | argv available? |
|---|---|---|
| **Rust** | `std::env::args().collect::<Vec<String>>()` | Yes (native). |
| **JS (Node)** | `process.argv.slice(1)` | Yes (Node host). |
| **WASM** | Stub (pushes `0`); compiles but returns no real argv. | No — pure WASM has no process argv. |

WASM has no command line in the pure module model, so `args()` there is a compile-
only stub. The self-hosted formatter targets rust/js, so this is acceptable; a
WASI-based argv mapping can be a later ADR if a WASM CLI is ever needed.

## Consequences

- **Enables** `main.intent` and any future self-hosted CLI (compiler, linter, pkg)
  to read their invocation — the concrete unblock for Phase 42.
- **Minimal:** one builtin, three-layer plumbing (checker type rule → IR
  `resolveCallKind` → rust/js/wasm emit), mirroring `timestamp_ms`. No new type, no
  new syntax.
- **Index-0 contract is load-bearing:** the JS `.slice(1)` normalization must hold,
  or rust/js programs that index `args()` diverge. Covered by the Phase 42 emit
  checks and the differential corpus (`main.intent` indexes `args()[1]`).
- **No flag parsing in-language yet:** tools hand-roll arg handling over the flat
  array until a stdlib arg parser is justified by repeated need.
