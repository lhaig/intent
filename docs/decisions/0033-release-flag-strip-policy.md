# 0033: `--release` Flag and Contract Strip Policy

**Date:** 2026-05-31
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (HARNESS.md §7 stepping stone)

## Context

Intent compiles every `requires`, `ensures`, and `invariant` clause into a runtime `assert!()` call in the emitted Rust (see `internal/rustbe/rustbe.go` — `emitLinef("assert!(%s, ...")` at lines 595, 613, 624, 658, 697, 717, 844, 863, 879, 1212, 1233, 1242, 1244, 1647). These asserts run on every call, every method invocation, every loop iteration. They are the dev-time safety net — they catch contract violations the type system missed and the verifier didn't (or couldn't) prove.

There is currently no way to drop them. `intentc build hello.intent` produces a binary that re-checks every contract at every call site for the life of the program. For a hot-path program — a server, a data pipeline, or eventually a self-hosted `intentc` (HARNESS.md §7's north star) — this is real overhead. The `intentc` Go binary today gets to skip this tax because Go has no contract layer; if we rewrote `intentc` in Intent without a way to strip its internal contracts in release, the self-hosted compiler would be measurably slower than the bootstrap.

`intentc build` also doesn't pass any optimization level to cargo. The default cargo profile (`cargo build` without `--release`) keeps debug symbols, no LTO, no inlining, no codegen-units tuning. Release-grade output requires both: optimisation flags through to cargo, and a way to drop contract overhead.

HARNESS.md §7 names a `--release` flag as the next stepping stone after in-language testing and LSP, with the casual phrasing "stripping unverified contracts." That phrasing is genuinely ambiguous on the *which contracts* question; this ADR picks the cut.

The hardest design choice: **which contracts count as scaffolding worth stripping?** Three product stories pull in different directions.

### Three product stories

**Story A — Performance.** Contracts are dev-time scaffolding. Like Rust's `debug_assert!`, C's `assert()` under `NDEBUG`, D's `-release`. Release strips everything; trust the dev build to have caught violations. Simple, fast, no proof dependency.

**Story B — Verification reward.** Z3 verification today produces a report and nothing else — no executable consequence. If release stripped only *Z3-verified* contracts and kept unverified ones asserting, verification becomes economically valuable: prove more → ship faster. Runtime net stays exactly where the proof is weakest. SPARK Ada does this; Microsoft Research's C# Code Contracts gestured at it.

**Story C — Strip-unverified.** Literal reading of the HARNESS.md line. Drops unproven contracts as aspirational noise; keeps proven ones running. This is the opposite of standard engineering practice (you'd want the net where the proof is weakest) and is rejected on those grounds.

### The "safe" lens

Rust ships `assert!()` in release builds. Only `debug_assert!()` strips. The author of the assertion picks the level at write time, knowing whether they want it to ship. Rust gets away with this because its type system catches huge classes of bugs at compile time — its dev builds have a higher floor of correctness than Intent's do today. Intent's checker is weaker than Rust's; the verifier is opt-in and slow; and most user contracts are not Z3-proven. Treating Intent's `--release` as "strip everything by default" silently lowers the safety floor for every existing user.

A safer baseline: `--release` enables optimisation only. Stripping is a *second* flag that the user explicitly types. CI logs make the choice visible. Forward-compatible with Story B as a future opt-in mode.

## Options

### O1. What `--release` should mean

**A. Strip-everything-by-default (one flag).** `--release` strips all contract asserts AND passes `--release` to cargo. Closest to D's `-release` flag.

- Pros: One flag, one motion. Matches "release = fast" mental model. No new flag surface.
- Cons: Silent safety regression for existing users. Hides the contract-strip decision behind a flag named for optimisation. Loses runtime safety net for every contract, including unverified ones where it's most needed.

**B. Two flags: `--release` for optimisation, `--strip-contracts` for stripping.** [Chosen.]

- `--release` alone: passes `--release` through to cargo. Contracts still `assert!()`. Identical runtime safety to dev builds; just faster code.
- `--release --strip-contracts`: explicit opt-in to drop contracts. Loud flag name.

Pros: No silent behaviour change. Stripping is named and visible in CI logs. Zero new dependencies (no Z3 at build time, no verify cache, no annotation system). Forward-compatible: a future `--strip-contracts=verified` mode is purely additive.

Cons: Two flags instead of one. Users wanting the full perf win type a longer command. Doesn't yet capture the "verification reward" product story (deferred to a future ADR).

**C. Granular per-kind stripping.** Mirror Ada's pragma: `--strip-contracts=preconditions,invariants` etc.

- Pros: Maximum flexibility — keep input validation, drop output checks.
- Cons: Massive flag surface for v1. Most users want all-or-nothing. Deferred until a real use case demands the granularity.

**D. Verification-aware stripping.** `--release` strips only Z3-proven contracts.

- Pros: Makes verification economically rewarding. Soundest model.
- Cons: Multi-week implementation. Requires either invoking Z3 during every build (slow, hard CI dep), reading a `.verify.json` cache (invalidation rules), or trusting a `@verified` annotation (needs a tool that cross-checks the claim). Not feasible as a v1.2 stepping stone; belongs in a separate ADR after the simpler flag lands.

### O2. How to emit the stripped form in Rust

**A. Swap `assert!` for `debug_assert!`.** The cargo `--release` profile defines `cfg(debug_assertions) = false`, so `debug_assert!` expands to nothing. No conditional codegen on the Intent side; cargo does the work.

- Pros: Trivial. The Rust ecosystem expects this idiom. Single `assert!` → `debug_assert!` swap in the emitter.
- Cons: Conflates `--strip-contracts` with cargo's debug-assertions setting. A user running `intentc build --strip-contracts` (without `--release`) would still get the asserts because cargo's debug-assertions stays on. Acceptable: `--strip-contracts` requires `--release` to take effect; we surface that as a clear error.

**B. Conditionally omit the assert in the emitter.** When `--strip-contracts` is set, simply don't emit the line.

- Pros: Decouples from cargo's debug-assertions flag. The emitted Rust source itself shows the strip happened (useful for `--emit` debugging).
- Cons: Two emitter modes. Easier to introduce divergence between dev and release output paths than the swap-macro approach.

**C. Wrap in `cfg!` block.** Emit `if cfg!(debug_assertions) { assert!(...); }`.

- Equivalent to A in behaviour but more verbose generated code. No advantage.

### O3. Cross-target behaviour

Intent emits Rust, JS, and WASM. `--strip-contracts` is only meaningful where the target actually compiles to a release binary.

**A. Apply across all targets.** JS backend drops the `if (!cond) throw ...` calls. WASM backend drops them too.

**B. Rust-only.** Other targets keep contracts always; users wanting to drop them work at the JS minifier / WASM optimizer layer.

**C. Apply where the target has a release notion, error otherwise.** Rust: respects `--release` + `--strip-contracts`. JS: `--strip-contracts` strips, `--release` is a no-op (no JS toolchain integration today). WASM: same as JS. [Chosen.]

C reflects what the targets actually do: only the Rust backend has a downstream optimisation toolchain (cargo). The other backends emit source; the user wraps them in whatever pipeline they want.

### O4. Backwards compatibility

**A. No flag → behaviour unchanged.** `intentc build hello.intent` still emits `assert!()`-laden code, still calls plain `cargo build`. [Chosen.]

This is non-negotiable. Every existing test, every example, every user invocation must produce identical output without an explicit flag.

## Decision

**O1.B + O2.A + O3.C + O4.A.**

1. Add **two new flags** to `intentc build`:

   - `--release` — pass `--release` through to the cargo invocation (Rust target). Contracts remain in place. On JS and WASM targets, `--release` is currently a no-op (no downstream toolchain integration); reserved for future use.

   - `--strip-contracts` — emit `debug_assert!()` instead of `assert!()` for all contract clauses (Rust target). Equivalent transformations on JS (drop `if (!cond) throw ...`) and WASM (drop the trap). Requires `--release` on the Rust target — emitting `debug_assert!` without `--release` produces a binary that still asserts because cargo's debug-assertions default is on; `intentc build` errors with a clear message rather than silently doing nothing.

2. **Default behaviour is unchanged.** `intentc build hello.intent` and `intentc build --emit hello.intent` produce identical output to today. Existing tests, examples, CI behaviour, and user invocations are untouched.

3. **Rust emitter swaps `assert!` for `debug_assert!`** when `--strip-contracts` is set. No conditional emitter mode beyond the macro choice. cargo's `--release` profile drops `debug_assertions`, which drops the calls.

4. **JS and WASM emitters drop the contract checks entirely** when `--strip-contracts` is set. No `debug_assert` equivalent exists in those targets, so the simplest correct behaviour is to omit the check.

5. **Verification-aware stripping is deferred to a future ADR.** A future `--strip-contracts=verified` mode (or similar) can read Z3 results and strip only proven contracts. That mode is additive — it doesn't change today's `--strip-contracts` semantics. The decision on *how* verification state reaches the build (Z3-at-build-time / persisted cache / annotation) is the substance of that future ADR.

6. **Loud opt-in messaging.** The first time `--strip-contracts` is used in a build, `intentc` prints a single-line warning to stderr: `warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.` Suppressible via `--quiet`.

## Consequences

**Accepted trade-offs:**

- Users wanting the full perf win type two flags (`--release --strip-contracts`) instead of one. Mitigated by the warning making the trade visible — users who don't know they want it shouldn't type it.
- No verification reward in this ADR. Z3 verification remains advisory. The "prove more → ship faster" loop lands in a follow-on ADR.
- Documentation must surface that `--strip-contracts` requires `--release` on the Rust target. CLI help text and `INTENT.md` will both call this out.

**Things this enables:**

- A self-hosted `intentc` can be compiled with `--release --strip-contracts` once we trust its dev-time contract suite — making the self-hosted compiler competitive with the Go bootstrap on real workloads.
- Existing examples and the bank-account demo can ship `--release --strip-contracts` build instructions in their READMEs once the user understands the trade.
- Future v1.2/v1.3 work (verify-aware stripping, granular per-kind stripping, debug builds with extra assertions) is additive — no rework of `--release` semantics needed.

**Things this defers:**

- Whether `--strip-contracts` should keep entity `invariant` checks alive while dropping `requires`/`ensures` (a hybrid stance — invariants are the load-bearing safety property). Deferred until we have a real example where the distinction matters.
- Whether the JS target should learn a `--release` notion that drives a downstream bundler/minifier. Out of scope; user's pipeline owns that today.
- Whether `intentc verify` should write a persistent `.verify.json` cache the build can consume. Belongs in the verification-aware-stripping ADR.

**Open follow-ups for the implementing PRD:**

- Where does the flag plumbing live in the compiler API? `compiler.BuildToTarget(source, target, baseName)` is the current entry — likely grows a `BuildOptions` struct.
- Cargo invocation site: `internal/compiler/target.go` for single-file, and the multi-file equivalents. Need a `--release` passthrough.
- Test coverage: the build path doesn't have a unit-testable cargo step today; the PRD should add at least one end-to-end test that emits Rust with `--strip-contracts` and checks the generated source uses `debug_assert!`.
