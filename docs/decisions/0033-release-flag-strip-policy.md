# 0033: `--strip-contracts` Flag and Contract Strip Policy

**Date:** 2026-05-31
**Status:** accepted; revised 2026-05-31 (dropped redundant `--release` flag — see "Revision: --release dropped")
**Phase:** v1.2 — Self-Improvement Foundations (HARNESS.md §7 stepping stone)

## Context

Intent compiles every `requires`, `ensures`, and `invariant` clause into a runtime `assert!()` call in the emitted Rust (see `internal/rustbe/rustbe.go` — the 14 `emitLinef("assert!(%s, ...")` sites for preconditions, postconditions, invariants, loop invariants, and decreases/termination metrics). These asserts run on every call, every method invocation, every loop iteration. They are the dev-time safety net — they catch contract violations the type system missed and the verifier didn't (or couldn't) prove.

There is currently no way to drop them. `intentc build hello.intent` produces a binary that re-checks every contract at every call site for the life of the program. For a hot-path program — a server, a data pipeline, or eventually a self-hosted `intentc` (HARNESS.md §7's north star) — this is real overhead. The `intentc` Go binary today gets to skip this tax because Go has no contract layer; if we rewrote `intentc` in Intent without a way to strip its internal contracts in release, the self-hosted compiler would be measurably slower than the bootstrap.

**Crucial discovery during ADR drafting:** `intentc build` already always invokes cargo with `--release` (`internal/compiler/compiler.go:170` — `exec.Command("cargo", "build", "--release")`). That means cargo's `debug_assertions` cfg is already disabled in every Intent binary that ships today. The implication: if Intent's emitter swaps `assert!(...)` for `debug_assert!(...)`, the calls compile away automatically with no other build-pipeline change required. A separate `--release` flag at the Intent level would be redundant — it would either be a no-op marker (silly) or it would silently break existing users by removing the cargo `--release` they're already getting (worse).

HARNESS.md §7 names "the `--release` flag" as the next stepping stone after in-language testing and LSP. The first draft of this ADR took that name literally and proposed a two-flag design (`--release` + `--strip-contracts`). On closer reading of the build pipeline, the `--release` half was unnecessary; the work that flag was supposed to do is already happening. This revision drops `--release` from the v1.2 surface.

The hardest design choice that remains: **which contracts count as scaffolding worth stripping?** Three product stories pull in different directions.

### Three product stories

**Story A — Performance.** Contracts are dev-time scaffolding. Like Rust's `debug_assert!`, C's `assert()` under `NDEBUG`, D's `-release`. Release strips everything; trust the dev build to have caught violations. Simple, fast, no proof dependency.

**Story B — Verification reward.** Z3 verification today produces a report and nothing else — no executable consequence. If release stripped only *Z3-verified* contracts and kept unverified ones asserting, verification becomes economically valuable: prove more → ship faster. Runtime net stays exactly where the proof is weakest. SPARK Ada does this; Microsoft Research's C# Code Contracts gestured at it.

**Story C — Strip-unverified.** Literal reading of the HARNESS.md line. Drops unproven contracts as aspirational noise; keeps proven ones running. This is the opposite of standard engineering practice (you'd want the net where the proof is weakest) and is rejected on those grounds.

### The "safe" lens

Rust ships `assert!()` in release builds. Only `debug_assert!()` strips. The author of the assertion picks the level at write time, knowing whether they want it to ship. Rust gets away with this because its type system catches huge classes of bugs at compile time — its dev builds have a higher floor of correctness than Intent's do today. Intent's checker is weaker than Rust's; the verifier is opt-in and slow; and most user contracts are not Z3-proven. Treating strip-on-release as the default silently lowers the safety floor for every existing user.

A safer baseline: stripping is its own explicit flag the user types. CI logs make the choice visible. Forward-compatible with Story B as a future opt-in mode.

## Options

### O1. Which contracts to strip when stripping is requested

**A. Strip all contracts.** [Chosen.] When `--strip-contracts` is set, every `requires` / `ensures` / `invariant` / loop-invariant / decreases assertion vanishes from the emitted output. Simplest. Matches Rust's `debug_assert!` model, D's `-release`, C's `NDEBUG`.

**B. Verification-aware stripping.** Strip only Z3-proven contracts. Soundest but requires verify state at build time (Z3-during-build, persisted cache, or annotation). Deferred to a future ADR. Layers cleanly on top of A — `--strip-contracts=verified` is purely additive.

**C. Granular per-kind stripping.** `--strip-contracts=preconditions,invariants` etc. Most flexible, most CLI surface. Deferred; almost no real example demands the granularity today.

### O2. Is opt-in needed at all, or strip by default in "release"?

**A. Strip-by-default in some implicit "release" mode.** [Rejected.] No real mechanism for this — Intent has no equivalent of cargo's `--release` vs default profile distinction. We'd have to invent one, and silently flipping safety semantics based on an environment variable or build flag is unsafe.

**B. Single explicit flag: `--strip-contracts`.** [Chosen.] The user types it; the CLI surfaces a one-line warning the first time the flag fires; the choice shows up in CI logs. No silent behaviour change for any existing build.

### O3. How to emit the stripped form per target

**A. Rust target: swap `assert!` for `debug_assert!`.** Cargo's existing always-on `--release` profile defines `cfg(debug_assertions) = false`, so `debug_assert!` expands to nothing. Single-macro swap in the emitter; cargo does the rest. [Chosen for rust.]

**B. JS target: omit the `if (!(cond)) throw new Error(...)` line entirely.** No JS analogue of `debug_assert!` exists; the cleanest representation of "this check is gone" is to not emit the source. Entity `__checkInvariants` methods become empty no-ops (we leave them in place so existing call sites still resolve). [Chosen for js.]

**C. WASM target: no change.** The WASM backend doesn't emit contract checks today (`internal/wasmbe/wasmbe.go` — only handles `result` references in `ensures`, no general contract assertion path). `--strip-contracts` is accepted and propagated but has no effect on emitted bytes. [Chosen for wasm; documented as such.]

### O4. Should the existing cargo always-release behaviour stay?

**A. Keep it.** [Chosen.] `intentc build` continues to invoke `cargo build --release`. Behaviour for users not using `--strip-contracts` is byte-identical to today. The "I want a non-release cargo build for faster local iteration" use case is real but separate — file as a future `--dev` flag if it comes up.

**B. Change default to non-release; add `--release` flag.** Silent perf regression for every existing user. Out.

### O5. Backwards compatibility

**A. No flag → behaviour unchanged.** `intentc build hello.intent` and `intentc build --emit hello.intent` produce identical output to today. Every existing test, every example, every user invocation. [Chosen; non-negotiable.]

## Decision

**O1.A + O2.B + O3 (A rust / B js / C wasm) + O4.A + O5.A.**

1. Add a **single new flag** to `intentc build`:

   - `--strip-contracts` — drop runtime contract checks from emitted output. On the rust target, the emitter swaps `assert!(...)` for `debug_assert!(...)` at every contract-check site (preconditions, postconditions, invariants, loop invariants, decreases/termination metrics). On the js target, the `if (!(cond)) throw new Error("...")` line is omitted entirely. On the wasm target the flag is accepted but has no effect today (the WASM backend doesn't emit contract checks).

2. **No `--release` flag is added in this phase.** `intentc build` already runs cargo with `--release`; adding a CLI flag named `--release` would either be a no-op marker (visible noise) or silently strip the cargo passthrough from existing users (regression). If a future use case demands a non-release cargo path for faster local iteration, a future `--dev` flag (or a `--cargo-profile=dev` knob) can introduce it cleanly.

3. **Default behaviour is unchanged.** `intentc build hello.intent` and `intentc build --emit hello.intent` produce identical output to today.

4. **Verification-aware stripping is deferred to a future ADR.** A future `--strip-contracts=verified` mode (or similar) can read Z3 results and strip only proven contracts. That mode is additive — it doesn't change today's `--strip-contracts` semantics. The decision on *how* verification state reaches the build (Z3-at-build-time / persisted cache / annotation) is the substance of that future ADR.

5. **Loud opt-in messaging.** When `--strip-contracts` is used, `intentc` prints a single-line warning to stderr: `warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.` Suppressible via `--quiet` (or whatever the existing quiet convention turns out to be — the implementing PRD checks).

6. **User-written assertion builtins are not affected.** `assert(...)`, `assert_eq(...)`, `assert_close(...)`, and `assert_panics(...)` from test bodies are part of the runtime assertion API (ADR 0029), not contracts. They continue to emit `assert!(...)` on rust and the equivalent on js even when `--strip-contracts` is set. The emitter distinguishes by code path — contract emissions carry "Precondition failed: ..." / "Postcondition failed: ..." / "Invariant failed: ..." / "Loop invariant failed ..." / "Decreases metric ..." / "Termination metric ..." messages; assertion builtins carry "assertion failed".

## Revision: --release dropped

**2026-05-31** — Initial draft of this ADR proposed a two-flag design: `--release` to pass `--release` through to cargo, and `--strip-contracts` to drop runtime checks. On implementation we discovered `internal/compiler/compiler.go:170` already always invokes `cargo build --release`, so cargo's `debug_assertions` cfg is already off in every shipped Intent binary. A separate `--release` flag at the Intent level was therefore redundant in any safe interpretation:

- As a no-op marker, it would add CLI surface for nothing.
- As an opt-in for cargo `--release`, it would silently slow down every existing user's build (cargo's release would only run with the flag).
- As a gate for `--strip-contracts`, the gate adds friction without value — cargo is already where it needs to be.

The cleaner design is a single `--strip-contracts` flag whose semantic is fully captured by the emitter (swap macro on rust, omit on js). The cargo invocation continues unchanged. This revision drops `--release` from the v1.2 surface; the title and decision sections of this ADR were rewritten to reflect the single-flag landing.

## Consequences

**Accepted trade-offs:**

- No verification reward in this ADR. Z3 verification remains advisory. The "prove more → ship faster" loop lands in a follow-on ADR.
- `intentc build` continues to always pass `--release` to cargo, so there's no fast-local-iteration story. Users wanting one type `cargo build` themselves against the emitted source. A future `--dev` flag could change this.
- Documentation must surface that `--strip-contracts` relies on cargo's release profile to actually compile the calls out. CLI help and `INTENT.md` will both call this out.

**Things this enables:**

- A self-hosted `intentc` can be compiled with `--strip-contracts` once we trust its dev-time contract suite — making the self-hosted compiler competitive with the Go bootstrap on real workloads.
- Examples and the bank-account demo can document `--strip-contracts` in their READMEs once the user understands the trade.
- Future v1.2/v1.3 work (verify-aware stripping, granular per-kind stripping, debug builds with extra assertions, a `--dev` cargo passthrough) is additive — no rework of `--strip-contracts` semantics needed.

**Things this defers:**

- Whether `--strip-contracts` should keep entity `invariant` checks alive while dropping `requires`/`ensures` (a hybrid stance — invariants are the load-bearing safety property). Deferred until we have a real example where the distinction matters.
- Whether the JS target should learn a release notion that drives a downstream bundler/minifier. Out of scope; user's pipeline owns that today.
- Whether `intentc verify` should write a persistent `.verify.json` cache the build can consume. Belongs in the verification-aware-stripping ADR.

**Open follow-ups for the implementing PRD:**

- Where does the flag plumbing live in the compiler API? `compiler.BuildToTarget(source, target, baseName)` is the current entry — likely grows a `BuildOptions` struct (single `StripContracts bool` field for now; future opt-ins are additive fields).
- Test coverage: a unit test per backend that emits a contract-bearing example with `StripContracts: true` and checks the generated source has no contract assertions (`debug_assert!` only on rust; no `throw new Error` on js).
- CLI tests in `cmd/intentc/main_test.go` for the new flag and the warning behaviour.
