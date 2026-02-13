# 0001: Rust as Compilation Target

**Date:** 2025-02-12
**Status:** accepted
**Phase:** POC

## Context

Intent needs a compilation target. The compiler must emit code in some
lower-level language or representation that can be built into an executable.

## Options

- **LLVM IR** -- Maximum control, but enormous complexity for a POC.
- **C** -- Portable, but manual memory management undermines safety goals.
- **Rust** -- Memory safety, performance, mature toolchain (cargo).
- **Go** -- GC-based, poor fit for ownership/mutation semantics.
- **Direct machine code** -- Maximum effort, no ecosystem leverage.

## Decision

Rust. The generated code gets memory safety, zero-cost abstractions, and
a mature build/link toolchain for free. Rust's ownership model maps
naturally to Intent's entity mutation patterns. Cargo handles dependency
resolution and linking without additional work.

## Consequences

- Requires the Rust toolchain (rustc + cargo) installed on the target system.
- Cannot run on systems without cargo.
- Generated code is readable Rust, which aids debugging the compiler.
- We inherit Rust's compile times, though generated code is small.
- Future optimization passes can target Rust idioms rather than raw IR.
