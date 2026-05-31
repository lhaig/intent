# Phase 22: `--release` Flag and Contract Stripping

**Status:** Draft
**Milestone:** v1.2 — Self-Improvement Foundations
**Decision:** [ADR 0033](../../docs/decisions/0033-release-flag-strip-policy.md)

## Goal

Land the two flags ADR 0033 specifies: `--release` (cargo optimisation passthrough on the Rust target) and `--strip-contracts` (explicit opt-in to drop runtime contract checks). Make the self-hosted-`intentc` north star (HARNESS.md §7) reachable on the performance axis without silently lowering the safety floor for existing users.

What "done" looks like:

- `intentc build hello.intent` produces byte-identical output to today. No regressions.
- `intentc build --release hello.intent` compiles via `cargo build --release`. The emitted Rust still contains `assert!()` for every contract — release builds keep the safety net by default.
- `intentc build --release --strip-contracts hello.intent` emits `debug_assert!()` instead of `assert!()` for every contract clause, and the cargo `--release` profile drops `debug_assertions`, so the binary contains no contract-check overhead.
- `intentc build --strip-contracts hello.intent` (without `--release`) errors with a clear message: "`--strip-contracts` requires `--release` on the rust target — cargo's debug-assertions are on by default."
- `intentc build --target js --strip-contracts hello.intent` and the WASM equivalent drop the contract checks from the emitted source. `--release` is a no-op on those targets (no downstream toolchain to drive).
- First-use stderr warning when `--strip-contracts` is set: "warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties."

## Success Criteria

- [ ] `intentc build` accepts `--release` and `--strip-contracts` flags (any order, repeatable rejection)
- [ ] Rust target: `--release` passes `--release` to the cargo invocation; without the flag, cargo runs in default profile
- [ ] Rust target: `--strip-contracts` causes the emitter to produce `debug_assert!(...)` for every place it previously produced `assert!(...)` for contracts (preconditions, postconditions, invariants, loop invariants, decreases checks)
- [ ] JS target: `--strip-contracts` drops the `if (!(cond)) throw new Error("Precondition failed: ...")` (and the `Postcondition` / `Invariant` equivalents) entirely from emitted source
- [ ] WASM target: `--strip-contracts` is accepted but is a no-op today (WASM backend doesn't emit contract checks); document as such
- [ ] `--strip-contracts` without `--release` on the rust target errors with a clear message (does NOT silently strip-then-keep-on at runtime)
- [ ] First-use stderr warning fires for `--strip-contracts`; suppressible via `--quiet` (or matches existing quiet flag convention)
- [ ] `intentc verify` is unaffected by the new flags (verify reads the AST, not the emitted output)
- [ ] All existing example builds, tests, and `make check-examples` / `make emit-examples` produce identical output to before this phase (the `intentc build hello.intent` no-flag path is untouched)
- [ ] New tests cover: emit with `--strip-contracts` produces `debug_assert!` (rust); emit with `--strip-contracts` produces no `throw` for contracts (js); `--strip-contracts` without `--release` on rust errors; `--release` alone on rust still emits `assert!`; build path passes `--release` to cargo when set
- [ ] `INTENT.md` documents both flags and the rust-toolchain interaction
- [ ] `README.md` build section mentions the flags
- [ ] `docs/ROADMAP.md` v1.2 entry adds Phase 22 SHIPPED with commit range
- [ ] `make validate` green

## Reference

- ADR 0033: `docs/decisions/0033-release-flag-strip-policy.md`
- Build entry points: `cmd/intentc/main.go` (`handleBuild`), `internal/compiler/target.go` (`BuildToTarget`, `EmitToTarget`, `BuildProjectToTarget`, `EmitProjectToTarget`)
- Rust contract emission: `internal/rustbe/rustbe.go` — the 14 `assert!(...)` emitLinef sites (preconditions, postconditions, invariants, loop invariants, decreases)
- JS contract emission: `internal/jsbe/jsbe.go` — the `if (!(cond)) throw new Error("Precondition/Postcondition/Invariant failed: ...")` sites
- WASM backend: `internal/wasmbe/wasmbe.go` — contracts are not currently emitted to WASM (verified; PRD documents as no-op)
- Cargo invocation site: `internal/compiler/compiler.go` (`Build`, `BuildProject`) — wherever `cmd := exec.Command("cargo", ...)` lives
- Existing build tests: `internal/compiler/target_test.go`, `cmd/intentc/main_test.go`

## Tasks

### 22.1 BuildOptions plumbing

**Files:** `internal/compiler/target.go`, `internal/compiler/compiler.go`, `internal/backend/` (or wherever the backend interface lives)

Introduce a `BuildOptions` struct carrying `Release bool` and `StripContracts bool`. Thread it through:

- `EmitToTarget(source, target, baseName string, opts BuildOptions) error`
- `BuildToTarget(source, target, baseName string, opts BuildOptions) error`
- `EmitProjectToTarget(entryPath, target, baseName string, opts BuildOptions) error`
- `BuildProjectToTarget(entryPath, target, baseName string, opts BuildOptions) error`

For backwards compatibility with internal callers (LSP, tests, examples), provide zero-value defaults: existing call sites that don't pass options get `BuildOptions{}` which preserves today's behaviour.

The backend `Generate` / `GenerateAll` methods grow an options parameter — choose the smallest-surface change:

- Either extend `backend.Backend` interface with `GenerateWithOptions(mod, opts)` (and keep `Generate(mod)` as a thin wrapper for callers that don't need options).
- Or attach options to a backend instance via a constructor.

Prefer the former — fewer state-carrying objects, easier to reason about.

**Acceptance:** `go build ./...` clean; existing tests pass with the new struct present but zero-valued in all paths.

### 22.2 Rust emitter: `assert!` → `debug_assert!` under `--strip-contracts`

**Files:** `internal/rustbe/rustbe.go`, `internal/rustbe/rustbe_test.go`

Every `emitLinef("assert!(...)`, ...)` site that emits a *contract* check (preconditions, postconditions, invariants, loop invariants, decreases checks — see the 14 sites identified in ADR 0033 context) becomes conditional: when `g.opts.StripContracts` is true, emit `debug_assert!(...)` instead.

User-written `assert(...)` calls in Intent test bodies stay as `assert!(...)`. Distinguish by code path: contract emission sites are the ones already emitting `"Precondition failed: ..."` / `"Postcondition failed: ..."` / `"Invariant failed: ..."` / `"Loop invariant failed ..."` / `"Decreases metric ..."` / `"Termination metric ..."` messages. Hand-written assertions (line 1647 area) come from test bodies and are not contract checks — leave them alone.

Add a `StripContracts` field to the rustbe generator struct; populate from the passed-through options.

**Acceptance:**
- New test: emit `examples/hello.intent` with `BuildOptions{StripContracts: true}`; the generated Rust contains `debug_assert!` and contains zero `assert!(...)` calls for contract messages
- Regression test: emit same example with `BuildOptions{}`; output is byte-identical to the current behaviour
- Test: emit an example with a `test "..." { assert_eq(...); }`; verify the test-body assert is unaffected by `StripContracts`

### 22.3 JS emitter: drop contract throws under `--strip-contracts`

**Files:** `internal/jsbe/jsbe.go`, `internal/jsbe/jsbe_test.go`

Skip the `if (!(cond)) throw new Error("Precondition failed: ...")` emission entirely when `g.opts.StripContracts` is true. Same for `Postcondition failed:` and `Invariant failed:`. Same for the per-entity `__checkInvariants()` method body — when stripping, either drop the method (preferred — simpler) or emit it as a no-op (only if some caller depends on the symbol).

`__checkInvariants()` is only called from generated code (constructors, methods with invariant-bearing entities). Dropping all the `this.__checkInvariants();` call sites along with the method definition is cleaner than emitting an empty method.

**Acceptance:**
- New test: emit `examples/bank_account.intent` with `BuildOptions{StripContracts: true}`; the generated JS contains zero `throw new Error("Precondition failed`, zero `Postcondition failed`, zero `Invariant failed`
- Regression test: emit same example with `BuildOptions{}`; output is byte-identical to the current behaviour
- Test: `__checkInvariants()` is absent from the stripped output and all call sites are gone

### 22.4 WASM emitter: accept the flag, document as no-op

**Files:** `internal/wasmbe/wasmbe.go` (likely no code change; just propagation), this PRD

Verify by code reading that WASM doesn't emit contract checks today. Propagate the options through the interface for consistency, but the WASM backend ignores `StripContracts`. Document this in a comment at the top of `wasmbe.go`.

**Acceptance:** `intentc build --target wasm --strip-contracts examples/hello.intent` succeeds and produces output identical to `intentc build --target wasm examples/hello.intent`.

### 22.5 Cargo `--release` passthrough

**Files:** `internal/compiler/compiler.go` (wherever `Build` / `BuildProject` invokes cargo)

When `BuildOptions.Release` is true, the cargo invocation becomes `cargo build --release` instead of `cargo build`. The output binary path changes from `target/debug/<name>` to `target/release/<name>` — verify the copy-out step uses the correct path. (The single-file wasm-via-rust path at `internal/compiler/target.go:341` already hardcodes `--release` — leave that alone; that's a separate concern.)

**Acceptance:**
- New test (probably integration, possibly skipped on `cargo` absence): build `examples/hello.intent` with `--release` and verify the resulting binary is at the release path and runs correctly
- Regression test: build same example without `--release`; identical behaviour to today

### 22.6 CLI flag wiring + error on bad combination

**Files:** `cmd/intentc/main.go` (`handleBuild`), `cmd/intentc/main_test.go`

In `handleBuild`'s flag-parsing loop:

```
case "--release":
    opts.Release = true
case "--strip-contracts":
    opts.StripContracts = true
```

After flag parsing, before dispatching to `Build*ToTarget`:

```
if opts.StripContracts && !opts.Release && target == "rust" {
    fmt.Fprintln(os.Stderr, "Error: --strip-contracts requires --release on the rust target")
    fmt.Fprintln(os.Stderr, "       (cargo's debug-assertions are on by default; debug_assert! would still run)")
    os.Exit(1)
}
```

Also update the usage text and `--help` output (visible from `intentc help` / `intentc --help`).

**Acceptance:**
- New test in `main_test.go`: run `intentc build --strip-contracts examples/hello.intent` → exit code 1, stderr contains the error message
- New test: run `intentc build --release examples/hello.intent` → exits 0 (regression — `--release` alone is fine)
- New test: run `intentc build --release --strip-contracts examples/hello.intent` → exits 0

### 22.7 First-use stderr warning

**Files:** `cmd/intentc/main.go` (`handleBuild`)

When `opts.StripContracts` is true and the user hasn't passed `--quiet` (or whatever the existing quiet convention is — check `handleTest`'s `--quiet` flag for precedent), emit a single line to stderr:

```
warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.
```

The warning fires once per `intentc` invocation, before the build kicks off.

**Acceptance:**
- New test in `main_test.go`: run `intentc build --release --strip-contracts examples/hello.intent` → stderr contains the warning string
- New test: same with `--quiet` (if the flag exists) → stderr does NOT contain the warning

### 22.8 Docs

**Files:** `INTENT.md`, `README.md` (root), `docs/ROADMAP.md`, `cmd/intentc/main.go` usage text

- `INTENT.md`: add a section near "Compilation" or similar covering `--release`, `--strip-contracts`, the rust-toolchain interaction, the warning, and the recommendation that release builds keep contracts on unless verification has covered the hot paths.
- Root `README.md`: build-commands section mentions both flags with one-line descriptions.
- `docs/ROADMAP.md`: under v1.2, add `### Phase 22: --release Flag and Contract Stripping -- SHIPPED (date)` with commit range.
- `cmd/intentc/main.go` usage string: add `--release` and `--strip-contracts` to the documented options.

**Acceptance:** `make validate` green. All cross-references consistent.

### 22.9 PRD status flip

**Files:** `ops/plans/phase-22-release-flag.md`

`Status: Draft` → `Status: Shipped (YYYY-MM-DD; commits XXXXXX..HEAD)`. Tick all the success-criteria checkboxes once tasks land.

## Out of Scope

- **Verification-aware stripping** (`--strip-contracts=verified` or similar). Designed in a separate ADR; depends on persistence of verify state. See ADR 0033's "Future follow-ups."
- **Granular per-kind stripping** (`--strip=preconditions`, etc.). ADR 0033 rejects this for v1 surface-area reasons.
- **JS / WASM release optimisation**. Those targets have no downstream toolchain integration; users wrap them in whatever pipeline they want.
- **Keeping invariants alive while stripping requires/ensures.** ADR 0033 defers this until a concrete example demands the distinction.
- **`intentc test --release`** — testing with stripped contracts is a different proposition (you'd be testing a different binary than what shipped). Out for v1.2.
- **Cargo profile customisation beyond `--release`**. No `--profile`, no LTO controls, no codegen-units flag. cargo's `--release` defaults suffice for v1.

## Suggested Order

1. **22.1 BuildOptions plumbing** — unblocks all downstream tasks; zero behaviour change.
2. **22.2 Rust emitter macro swap** — smallest backend change, biggest user-visible effect.
3. **22.5 Cargo `--release` passthrough** — independent of contract stripping; could ship alone if 22.2 hits a snag.
4. **22.6 CLI flag wiring + error guard** — locks the wire contract.
5. **22.3 JS emitter strip** — parallel to 22.2; less critical for v1.2 (no JS hot-path users today) but completes the cross-target story.
6. **22.4 WASM no-op verification** — quick.
7. **22.7 Stderr warning** — quality polish.
8. **22.8 Docs + 22.9 status flip** — last; locks the public surface.
