# 0029: In-Language Testing Framework

**Date:** 2026-05-29
**Status:** accepted
**Phase:** v1.2 (Phase 16)

## Context

Intent has three forms of correctness today: Go-side unit tests of the compiler, runtime contract assertions injected into generated code, and Z3 static verification of contracts. What's missing is a way to express **tests in Intent itself** — assertions of program behaviour, written in `.intent` source, runnable on every backend.

This matters for three reasons. First, Intent's "harness engineering" posture (see `docs/HARNESS.md`) requires agents to validate their own work mechanically; reading compiler output and trusting it is the anti-pattern. Second, Intent's multi-target story (Rust + JS + WASM from the same IR) is the language's strongest claim, but nothing exercises the "same source → same behaviour" guarantee end-to-end. Third, `intentc test-gen` already generates property tests, but it emits Rust source — runnable only on the Rust target and not composable with hand-written assertions.

Phase 16 fills this gap. This ADR records the design decisions made at approval time.

## Options

### O1. Test syntax

- **A. `test "name" { ... }` block as a top-level declaration.** Peer of `function`, `entity`, `enum`, `trait`, `extern function`. No params, no return type.
- **B. `@test function name() { ... }` annotation.** Reuses the function machinery.
- **C. Naming convention: `function test_foo() { ... }`.** Cheapest but most magic.

### O2. Float equality

- **A. `assert_close(actual, expected, epsilon)` builtin with explicit per-call tolerance.** No global flag. `assert_eq` rejects `Float` at type-check.
- **B. `assert_eq` on Float uses a global `--epsilon` flag.** One default applies to all float comparisons in a run.
- **C. Both.** Provide both mechanisms.

### O3. Entity equality in `assert_eq`

- **A. Explicit `method eq(other: T) returns Bool` required.** Type-check fails if absent, with a clear diagnostic pointing the user to add the method.
- **B. By-field default.** Two entities are equal iff all fields compare equal pairwise.
- **C. Identity equality.** Two entity values are equal iff they reference the same object (Rust uses `==` on derived `PartialEq`, JS uses object identity).

### O4. Cross-target equivalence in v1

- **A. Yes — `--all-targets` runs each test on every supported target and fails on divergence.** The differentiator.
- **B. No — single-target only in v1, equivalence is Phase 17.** Smaller v1 surface.

### O5. testgen output format

- **A. Migrate to Intent test blocks.** Single harness for generated and hand-written tests; legacy `--emit-rust` removed.
- **B. Keep Rust emission as the primary, add Intent emission as a parallel `--emit-intent` flag.** Backwards-compatible but doubles the surface.

## Decision

**O1 → A.** `test "name" { ... }` is a new top-level declaration. Most readable, distinct from functions (which have return types and can be called), and matches Intent's general style of explicit constructs over annotations.

**O2 → A.** `assert_close(actual, expected, epsilon)` builtin, no global flag. Rationale: per-call tolerance is self-documenting at the call site, doesn't hide test precision behind global state, and forces users to think about what tolerance means for the specific computation under test. `assert_eq` on `Float` is a type-check error directing users to `assert_close`.

**O3 → A.** Explicit `eq` method required. Rationale: Intent's design ethos is "no implicit behaviour, no hidden conversions." Entities have invariants and can carry derived state; by-field equality could silently disagree with what the type's author considers semantically equal. Mirrors Rust's `PartialEq` derive-vs-manual choice but makes it explicit (no auto-derive).

**O4 → A.** Cross-target equivalence in v1. Rationale: this is Intent's strongest differentiator and the actual selling point of the multi-target story. The runner already needs per-target compile + execute + stdout capture; the additional work is diffing captured outputs across targets — estimated 1-2 days inside a multi-week phase. Deferring it would ship a "Intent has unit tests" v1, which is uninteresting. Shipping with it gives "Intent guarantees identical behaviour across Rust, JS, WASM" as a checkable property.

**O5 → A.** Migrate testgen to emit Intent test blocks. Rationale: one harness for everything is the harness-engineering principle. Generated tests should look and run identical to hand-written ones. The migration is destructive (anyone depending on `--emit-rust` breaks), so the legacy path stays under a deprecation warning for one release cycle and is removed at the end of Phase 16.

## Consequences

**Accepted trade-offs:**

- `assert_eq` cannot be used for floats. New users hitting this get a clear diagnostic; the alternative (silent precision loss with hidden global epsilon) is worse.
- Entity authors must write an `eq` method to use `assert_eq` on their type. Boilerplate cost, but predictable and explicit. Future ADR could add a `derive` mechanism if the boilerplate becomes painful.
- Cross-target equivalence comparison is currently stdout-based. Two backends could produce identical stdout but different internal state. Mitigation: encourage tests to print computed values; revisit if it bites.
- `intentc test-gen --emit-rust` removal breaks any downstream consumer. Mitigation: deprecation warning for one cycle; documented in changelog at removal.
- Test declarations grow the language surface and the grammar. Documented in `docs/grammar.ebnf` at task 16.1.

**Follow-up work expected:**

- `assert_close` may want a sibling `assert_relative_close(actual, expected, rel_tolerance)` once real numerics work begins. Defer.
- `assert_panics` requires a `Fn() -> Void` lambda; verifies the existing closure infrastructure handles zero-arg lambdas cleanly. If not, that's a bug surfaced by Phase 16.
- Cross-target equivalence on async tests is undefined for now (WASM rejects async; Rust and JS could be compared). Phase 16 runner handles this by treating "target rejects program" as equivalent to "test skipped on that target" rather than as a divergence.
- Future Phase 17 candidates: snapshot testing, parallel execution, coverage reporting, `@skip`/`@only` attributes if the test surface grows past what `intentc test --filter <substring>` (out of scope here) can handle.

**Relationship to other ADRs:**

- ADR 0003 (runtime assertions): asserts in tests compile through the same machinery — already in place.
- ADR 0014 (legacy codegen removed): testgen's Rust emission is a residual user of the old code-as-string approach; Phase 16 migrates it to the IR-based pattern.
- ADR 0023 (closures): `assert_panics` exercises zero-arg closures; failures here surface as ADR 0023 bugs.
- ADR 0026 (async, revised by Phase 14): WASM-rejects-async carries forward to tests.
- ADR 0027 (package management): tests are not exported across packages — they live with their module of definition.
