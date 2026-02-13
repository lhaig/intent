# 0003: Runtime Assertions for Contract Enforcement

**Date:** 2025-02-12
**Status:** accepted
**Phase:** POC

## Context

Contracts (requires/ensures/invariant) are a core language feature.
They need an enforcement mechanism to provide value.

## Options

- **Runtime assertions** -- Compile contracts to assert!() in Rust output.
- **Static verification (Z3/SMT)** -- Prove contracts at compile time.
- **Hybrid** -- Static where possible, runtime as fallback.

## Decision

Runtime assertions for the POC. All contracts compile to assert!() in the
generated Rust code. Static verification is deferred to Milestone 3.

## Consequences

- Contract violations are caught at runtime, not compile time. A program
  with a violated precondition will panic rather than fail to compile.
- Runtime assertions are simple to implement, correct, and debuggable.
  The generated assert!() messages include source location and context.
- Performance cost from assertion checks in hot paths. A future release
  mode could strip assertions.
- Static verification is a research problem. Shipping runtime checks now
  provides immediate value while the harder problem is explored later.
