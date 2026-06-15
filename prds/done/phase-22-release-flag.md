# Phase 22: `--strip-contracts` Flag

**Status:** Shipped (2026-05-31; commits f8e4473..HEAD)
**Milestone:** v1.2 — Self-Improvement Foundations
**Decision:** [ADR 0033](../../docs/decisions/0033-release-flag-strip-policy.md) (revised: dropped `--release`, kept only `--strip-contracts`)

## Goal

Land the one flag ADR 0033 specifies: `--strip-contracts`. Make the self-hosted-`intentc` north star (HARNESS.md §7) reachable on the performance axis without silently lowering the safety floor for existing users.

A nuance discovered during ADR drafting: `intentc build` already invokes cargo with `--release`. Cargo's `debug_assertions` cfg is therefore already off in every shipped Intent binary. Swapping `assert!` for `debug_assert!` in the rust emitter is sufficient to compile contract checks out — no separate `--release` flag needed at the Intent level. The earlier draft of this PRD included such a flag; this revision drops it.

What "done" looks like:

- `intentc build hello.intent` produces byte-identical output to today. No regressions.
- `intentc build --strip-contracts hello.intent` emits `debug_assert!()` instead of `assert!()` for every contract clause on the rust target, and cargo's existing `--release` drops `debug_assertions`, so the binary contains no contract-check overhead.
- `intentc build --target js --strip-contracts hello.intent` drops the `if (!(cond)) throw new Error("...")` lines for preconditions / postconditions / invariants from the emitted source.
- `intentc build --target wasm --strip-contracts hello.intent` accepts the flag and produces output identical to without it (the WASM backend doesn't emit contract checks today).
- First-use stderr warning when `--strip-contracts` is set: "warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties."
- User-written `assert(...)` / `assert_eq(...)` / `assert_close(...)` / `assert_panics(...)` calls in test bodies are unaffected by the flag — they're the runtime assertion API (ADR 0029), not contracts.

## Success Criteria

- [x] `intentc build` accepts `--strip-contracts` flag (any position)
- [x] Rust target: `--strip-contracts` causes the emitter to produce `debug_assert!(...)` for every place it previously produced `assert!(...)` for contracts (preconditions, postconditions, invariants, loop invariants, decreases checks)
- [x] JS target: `--strip-contracts` drops the `if (!(cond)) throw new Error("Precondition failed: ...")` (and the `Postcondition` / `Invariant` equivalents) entirely from emitted source
- [x] WASM target: `--strip-contracts` is accepted but is a no-op today (WASM backend doesn't emit contract checks); document as such
- [x] First-use stderr warning fires for `--strip-contracts`
- [x] `intentc verify` is unaffected by the new flag (verify reads the AST, not the emitted output)
- [x] All existing example builds and `make check-examples` / `make emit-examples` produce identical output to before this phase (the `intentc build hello.intent` no-flag path is untouched)
- [x] New tests cover: emit with `--strip-contracts` produces `debug_assert!` (rust); emit with `--strip-contracts` produces no `throw` for contracts (js); regression — no-flag path produces identical output to before; user-written `assert(...)` test-body calls are unaffected
- [x] `INTENT.md` documents the flag and the rust-toolchain interaction
- [x] `README.md` build section mentions the flag
- [x] `docs/ROADMAP.md` v1.2 entry adds Phase 22 SHIPPED with commit range
- [x] `make validate` green

## Reference

- ADR 0033: `docs/decisions/0033-release-flag-strip-policy.md` (revised in-place to drop `--release`)
- Build entry points: `cmd/intentc/main.go` (`handleBuild`), `internal/compiler/target.go` (`BuildToTarget`, `EmitToTarget`, `BuildProjectToTarget`, `EmitProjectToTarget`)
- Rust contract emission: `internal/rustbe/rustbe.go` — the 14 contract `assert!(...)` sites for preconditions, postconditions, invariants, loop invariants, decreases/termination
- JS contract emission: `internal/jsbe/jsbe.go` — the `if (!(cond)) throw new Error("Precondition/Postcondition/Invariant failed: ...")` sites
- WASM backend: `internal/wasmbe/wasmbe.go` — contracts are not currently emitted to WASM (verified by code reading; PRD documents as no-op)
- Cargo invocation (already passes `--release`, unchanged this phase): `internal/compiler/compiler.go` — `cargoBuild` at line 145
- Existing build tests: `internal/compiler/target_test.go`, `cmd/intentc/main_test.go`

## Tasks

### 22.1 BuildOptions plumbing

**Files:** `internal/backend/backend.go`, `internal/backend/rust.go`, `internal/backend/js.go`, `internal/backend/wasm.go`, `internal/compiler/target.go`, `internal/compiler/compiler.go`

Introduce a `backend.BuildOptions` struct carrying `StripContracts bool`. Thread it through:

- `Backend.Generate(mod, opts)` and `Backend.GenerateAll(prog, opts)`
- `BinaryBackend.GenerateBytes(mod, opts)` and `BinaryBackend.GenerateAllBytes(prog, opts)`
- `EmitToTarget(source, target, baseName, opts)` / `EmitProjectToTarget(...)`
- `BuildToTarget(source, target, baseName, opts)` / `BuildProjectToTarget(...)`
- `Build(source, outPath, opts)` / `BuildProject(entryPath, outPath, opts)` (cargo passthrough is unchanged this phase, but the option still threads through for the emitter)

For internal callers that don't care (LSP, existing tests, internal `Compile` results), zero-value `BuildOptions{}` preserves today's behaviour.

The rustbe and jsbe packages get their own `Options` types (a copy of the relevant fields) — keeps each backend's package self-contained. The `backend.RustBackend` / `backend.JSBackend` wrappers do the translation.

**Acceptance:** `go build ./...` clean; existing tests pass with the new struct present but zero-valued in all paths.

### 22.2 Rust emitter: `assert!` → `debug_assert!` under `--strip-contracts`

**Files:** `internal/rustbe/rustbe.go`, `internal/rustbe/rustbe_test.go`

Every `emitLinef("assert!(...)`...)` site that emits a *contract* check becomes conditional via a `contractAssertMacro()` helper that returns `"debug_assert!"` when `--strip-contracts` is set, `"assert!"` otherwise.

User-written `assert(...)` calls in Intent test bodies stay as `assert!(...)`. Distinguish by code path: contract emission sites carry the `"Precondition failed: ..."` / `"Postcondition failed: ..."` / `"Invariant failed: ..."` / `"Loop invariant failed ..."` / `"Decreases metric ..."` / `"Termination metric ..."` message strings.

**Acceptance:**
- New test: emit a contract-bearing example with `Options{StripContracts: true}`; the generated Rust contains `debug_assert!` and contains zero `assert!(...)` calls for contract messages
- Regression test: emit same example with `Options{}`; output is byte-identical to the current behaviour
- Test: emit an example with `test "..." { assert_eq(...); }`; verify the test-body assert is unaffected by `StripContracts`

### 22.3 JS emitter: drop contract throws under `--strip-contracts`

**Files:** `internal/jsbe/jsbe.go`, `internal/jsbe/jsbe_test.go`

Introduce a `g.emitContractCheck(kind, cond, msg)` helper that emits `if (!(cond)) throw new Error("<kind> failed: <msg>");\n` when stripping is off and emits nothing when stripping is on. Convert all 12 contract-throw sites to use the helper. Entity `__checkInvariants()` methods are emitted with empty bodies under strip — existing call sites stay valid; the inner check lines just don't appear.

**Acceptance:**
- New test: emit a contract-bearing example with `Options{StripContracts: true}`; the generated JS contains zero `throw new Error("Precondition failed`, zero `Postcondition failed`, zero `Invariant failed`
- Regression test: emit same example with `Options{}`; output is byte-identical to the current behaviour

### 22.4 WASM emitter: accept the flag, document as no-op

**Files:** `internal/backend/wasm.go` (option propagation), this PRD

Verify by code reading that WASM doesn't emit contract checks today. Propagate options through the interface for consistency, but the WASM backend ignores `StripContracts`. Document in a comment on `internal/backend/wasm.go`.

**Acceptance:** `intentc build --target wasm --strip-contracts examples/hello.intent` succeeds and produces output identical to `intentc build --target wasm examples/hello.intent`.

### 22.5 CLI flag wiring

**Files:** `cmd/intentc/main.go` (`handleBuild`), `cmd/intentc/main_test.go`

In `handleBuild`'s flag-parsing loop:

```go
case "--strip-contracts":
    opts.StripContracts = true
```

Update the usage text and `--help` output to document the flag.

**Acceptance:**
- New test in `main_test.go`: run `intentc build --strip-contracts examples/hello.intent` → exits 0
- Regression test: run `intentc build examples/hello.intent` (no flag) → identical behaviour to today

### 22.6 First-use stderr warning

**Files:** `cmd/intentc/main.go` (`handleBuild`)

When `opts.StripContracts` is true, emit a single line to stderr before the build kicks off:

```
warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.
```

**Acceptance:**
- New test in `main_test.go`: run `intentc build --strip-contracts examples/hello.intent` → stderr contains the warning string

### 22.7 Docs

**Files:** `INTENT.md`, `README.md` (root), `docs/ROADMAP.md`, `cmd/intentc/main.go` usage text

- `INTENT.md`: add a short section near "Compilation" covering `--strip-contracts`, the rust-toolchain interaction (cargo already runs `--release` so `debug_assert!` compiles away), the warning, and the recommendation that release builds keep contracts on unless verification has covered the hot paths.
- Root `README.md`: build-commands section mentions the flag with a one-line description.
- `docs/ROADMAP.md`: under v1.2, add `### Phase 22: --strip-contracts Flag -- SHIPPED (date)` with commit range.
- `cmd/intentc/main.go` usage string: add `--strip-contracts` to the documented options.

**Acceptance:** `make validate` green. All cross-references consistent.

### 22.8 PRD status flip

**Files:** `prds/done/phase-22-release-flag.md`

`Status: In Progress` → `Status: Shipped (YYYY-MM-DD; commits XXXXXX..HEAD)`. Tick all the success-criteria checkboxes once tasks land.

## Out of Scope

- **`--release` flag.** Dropped from this phase — cargo already runs in release mode (see ADR 0033 revision). A future `--dev` flag could opt into a non-release cargo passthrough for faster local iteration, but no current use case demands it.
- **Verification-aware stripping** (`--strip-contracts=verified` or similar). Designed in a separate ADR; depends on persistence of verify state. See ADR 0033's "Future follow-ups."
- **Granular per-kind stripping** (`--strip=preconditions`, etc.). ADR 0033 rejects this for v1 surface-area reasons.
- **JS / WASM release optimisation.** Those targets have no downstream toolchain integration; users wrap them in whatever pipeline they want.
- **Keeping invariants alive while stripping requires/ensures.** ADR 0033 defers this until a concrete example demands the distinction.
- **`intentc test --release`** — testing with stripped contracts is a different proposition (you'd be testing a different binary than what shipped). Out for v1.2.

## Suggested Order

1. **22.1 BuildOptions plumbing** — unblocks all downstream tasks; zero behaviour change.
2. **22.2 Rust emitter macro swap** — smallest backend change, biggest user-visible effect.
3. **22.3 JS emitter strip** — parallel to 22.2; completes the cross-target story.
4. **22.4 WASM no-op verification** — quick.
5. **22.5 CLI flag wiring** — locks the wire contract.
6. **22.6 Stderr warning** — quality polish.
7. **22.7 Docs + 22.8 status flip** — last; locks the public surface.
