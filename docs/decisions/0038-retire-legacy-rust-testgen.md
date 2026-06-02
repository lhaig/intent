# 0038: Retire the Legacy Rust `testgen` Path

**Date:** 2026-06-02
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 17.A.4 — final phase of the testgen migration)

## Context

`intentc test-gen` has historically had two emission paths:

- **Legacy (`--target rust`, default).** Generates Rust property-style tests with PRNG-driven input synthesis, appended to the IR-emitted Rust source. The Rust crate compiles the whole thing as a `cargo test` target. ~1.9k LOC across `internal/testgen/{testgen.go,rustutil.go,values.go}`. Maintains a parallel expression converter (`rustutil.go`) duplicating much of `internal/rustbe`. Cannot run on the JS or WASM backends — it is hard-coded to Rust output.

- **Intent (`--target intent`).** Introduced in Phase 16 ([ADR 0029](0029-in-language-testing.md), task 16.8). Emits a sibling `_test.intent` file of Intent `test` blocks. `intentc test` consumes those blocks via the normal IR → backend pipeline, so the same generated tests run on rust, js, and wasm. Backend-agnostic and self-hosting-aligned: tests written *in Intent* are the substrate the language wants to bootstrap itself with.

The migration was staged across three phases so the Intent path could grow to cover the legacy path's surface before the legacy path was deleted:

| Phase | Task | Status | What it added to the Intent emitter |
|---|---|---|---|
| 17.A.1 | [ADR 0036](0036-testgen-entity-method-emission.md) | Shipped (Phase 27, 2026-05-31) | Entity + method emission |
| 17.A.2 | [ADR 0037](0037-testgen-multi-param-iteration.md) | Shipped (Phase 28, 2026-06-01) | Multi-Int-param Cartesian iteration |
| 17.A.4 | This ADR | Phase 29, 2026-06-02 | Removes the legacy path |

With both prerequisites in, the Intent emitter covers everything the legacy path could deterministically express. The legacy path's *random* PRNG component is not reproduced and is deliberately deferred — see "What is not migrated" below.

### Precedent

Compiler / toolchain retirements after a replacement reaches parity:

- **GHC.** Removed the old C-via-`Cmm` backend (`-fvia-C`) once the native code generator + LLVM backends covered the platforms it served. Hard removal in 7.x; users migrated to `-fasm` or `-fllvm`. The replacement was *backend-agnostic from the IR boundary outward* — exactly the structure here (Intent IR → backend, with testgen now living on the Intent side of that boundary rather than the Rust side).
- **Rust.** Removed `libgreen` / `libnative` once `libstd` subsumed them (pre-1.0). Removed the old `crate_type = "staticlib"`-via-`rustc-trans` paths when the LLVM backend covered them. Rust's edition system *also* shows the alternative — multi-year compatibility windows — but that's reserved for changes that affect *user-written code*, not internal emitter paths.
- **Go.** `go fix` was retired once `gofmt` + the import-rewrite tooling covered the migration cases. Hard cut at the major-tooling level.
- **TypeScript.** The pre-ES-modules namespace emit (`namespace Foo { ... }` → IIFE) was *kept* alongside ES module emit because users wrote namespace-using code that needs to keep compiling. Notably *different* from our case: nobody writes "legacy testgen output" by hand — it is fully generated, so retirement has no source-code migration cost.
- **Dafny.** Old `axiom`-based verification helpers were dropped once the SMT-based encoding handled them. Dafny is the closest precedent to our model: a contract-bearing language whose toolchain has a deterministic "every contract is checked" stance ([ADR 0037](0037-testgen-multi-param-iteration.md) §Precedent already cites this).

The TypeScript counter-example is the relevant control: a parallel path stays alive when *user code* depends on it. Here, no user code depends on the legacy emitter's Rust output — its output is regenerated on every build. So the case for hard retirement is strong.

## Decision

Delete the legacy `--target rust` testgen path. Make `--target intent` the only (and default) path. Have `--target rust` produce an explicit error with migration instructions rather than silently falling back.

### Migration message

```
Error: `intentc test-gen --target rust` was removed in Phase 29 (ADR 0038).
Use `intentc test-gen --emit <file.intent>` to write a sibling _test.intent file,
then run `intentc test --target rust <file.intent>` to execute it on the Rust backend.
```

The error is exit-code 1 so CI catches stale invocations.

### What is migrated

The Intent emitter covers:
- No-param free functions (single example call, assert each `ensures`).
- Single-Int-param free functions (exhaustive iteration over constraint-derived `[lo, hi]`).
- Multi-Int-param free functions (Cartesian iteration over per-param ranges, capped at `floor(1000^(1/N))` per param — ADR 0037).
- Entities with constructor-bearing types, with one auto-test per `(entity, method)` pair carrying contract clauses (ADR 0036).
- `old(...)` capture, `result` substitution, `self` rebinding (ADR 0036).
- Generic entities and constructor-less entities — emit a one-line skip comment.

### What is not migrated

The legacy path's **PRNG-driven random sampling** for non-Int param types (Float, String, Array<T>, Map, entity-typed params) is deliberately *not* reproduced. The Intent testing story (per [ADR 0029](0029-in-language-testing.md)) is verifier-aligned exhaustive coverage within bounds, not stochastic property-based testing. If randomized PBT is wanted, it is a *new* ADR proposing a `--target intent-prop` mode or equivalent — not a reason to keep the legacy emitter alive.

For function shapes the Intent emitter can't iterate today (multi-param-with-non-Int, Float params, etc.), it falls back to a single example call with default values — same as it did before Phase 29. That fallback is weaker than the legacy path's PRNG coverage, but the fallback gap was already accepted by ADR 0037 §"Trade-offs."

### Hard cut vs. deprecation cycle

[ADR 0029 §"Risks"](0029-in-language-testing.md) hinted at a deprecation warning for one cycle. We are choosing a hard cut instead, for these reasons:

1. **No source-code migration.** The legacy path emits Rust; the replacement emits Intent. Users don't *write* either kind of output — they write `.intent` source files and contracts. Both emitters consume the same `.intent` input. So "migration" for a user is `--target rust` → `--target intent` on the CLI — one flag flip. A deprecation cycle protects against weeks-of-rewriting transitions; that's not the failure mode here.
2. **Maintenance burden of the parallel path is real.** `rustutil.go` carries a duplicate expression converter that has to keep pace with `internal/rustbe` (entity field access, enum variants, `old(...)`, `forall` / `exists`, generic substitution, etc.). Every backend change risks divergence. ADR 0029's "one cycle" deprecation period would mean another full release of double-maintenance.
3. **CI catches the migration immediately.** The hard error is loud — an unmigrated CI pipeline fails on first run with a one-line fix.
4. **Project scale.** Intent is a single-author personal project. The user count for "production CI depending on `--target rust`" is zero outside this repo. A deprecation cycle is overhead with no downstream payoff.

The ADR explicitly overrides ADR 0029's hint at a deprecation cycle.

## Consequences

### Code removed (~1.9k LOC)

- `internal/testgen/testgen.go` — the legacy Rust emitter (`Generate`, `generateFunctionTest`, `generateEntityTests`, `generateMethodTest`, `generateWorkflowTest`, `prngHelpers`, etc.).
- `internal/testgen/rustutil.go` — the legacy Rust expression converter (`ExprToRust`, `rustHelper`, `MapType`, `EscapeRustString`).
- `internal/testgen/values.go` — PRNG value-generation helpers (`GenerateIntValues`, `GenerateFloatValues`, `GenerateArrayIntValues`, `xorshift64`, etc.).
- `internal/testgen/testgen_test.go` — tests for the legacy path. The one survivor — `TestConstraintAnalysis` — moves to `internal/testgen/constraints_test.go` since `constraints.go` is shared with the Intent emitter.
- `internal/compiler/GenerateTests` and `internal/compiler/GenerateTestsProject` — orchestration entry points that only called the legacy emitter.

### Code retained

- `internal/testgen/constraints.go` — `ParamConstraint`, `AnalyzeConstraints`. Shared between paths historically; the Intent emitter uses it via `intRange`.
- `internal/testgen/intentgen.go` — the Intent emitter (`GenerateIntent`).
- `internal/testgen/intentgen_test.go` — Intent-emitter tests.
- `internal/testgen/constraints_test.go` — `TestConstraintAnalysis` migrated from the deleted file.
- `internal/compiler/GenerateIntentTests` — the surviving orchestration entry point.

### CLI surface

`cmd/intentc/main.go` `handleTestGen`:
- Default `target` flips from `"rust"` to `"intent"`.
- `--target rust` → exit 1 with the migration message above.
- `--target` with any value other than `intent` → exit 1 with `unknown test-gen target` error.
- Help text and example block updated to reference `.intent` output filenames.

### Build / test surface

- `make test-gen-examples` continues to run, now writing `_test.intent` files to `examples/`. The Makefile comment is updated to reflect the migration.
- `make clean` now removes `examples/*_test.intent` to avoid committing generated artifacts.
- `make validate` is unchanged (it never ran `test-gen-examples` and still doesn't).
- `go build ./...`, `go test ./internal/testgen`, and `make validate` all pass after the change.

### Docs

- `README.md`: `test-gen` CLI line drops the `[--target intent|rust]` qualifier.
- `INTENT.md`: `test-gen` example lines reference `_test.intent` (was `_test.rs`).
- `docs/ROADMAP.md`: Phase 29 entry added; Phase 17.A marked complete; Phase 17.E / 17.G / 17.H remain deferred.

### Self-hosting alignment

This phase is a step toward the user's stated goal of bootstrapping Intent with itself. Before Phase 29 the toolchain had two test-emission code paths — one in Intent (the `.intent` output) and one in Rust (the legacy emitter). Retiring the Rust path means *all* generated tests live in `.intent` files, executable on every backend. When the linter, formatter, or other tooling is rewritten in Intent (HARNESS.md §7), the testgen package is no longer a Rust-shaped exception that has to be rewritten separately — it already emits Intent.

## Follow-ups

- **Phase 17.E (workflow tests for entity sequences).** Not unblocked or blocked by Phase 29. Separate PRD when prioritized.
- **Phase 17.G (WASM test runner).** Still deferred. Needs its own ADR (test-failure protocol from WASM back to runner).
- **Phase 17.H (coverage / snapshot testing).** Still deferred. Needs its own PRD.
- **Randomized property mode (`--target intent-prop`?).** Open ADR slot if/when there's demand. Would coexist with `--target intent`, not replace it.
- **Package registry (orthogonal).** Tracked separately; unblocks Phase 13 TODOs and is the next major milestone toward self-hosting.

## References

- [ADR 0029](0029-in-language-testing.md) — In-language testing (introduced `--target intent`)
- [ADR 0036](0036-testgen-entity-method-emission.md) — Entity / method emission (Phase 27 / 17.A.1)
- [ADR 0037](0037-testgen-multi-param-iteration.md) — Multi-param iteration (Phase 28 / 17.A.2)
- Phase 17 PRD: `ops/plans/phase-17-testing-polish.md`
- Pickup notes: `ops/NEXT-STEPS.md` (2026-06-01) — recommended hard retirement once both prerequisites landed
