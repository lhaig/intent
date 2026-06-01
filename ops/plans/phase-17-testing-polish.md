# Phase 17: Testing Framework Polish

**Status:** Shipped (2026-05-30) — 17.B, 17.C, 17.D, 17.F complete. 17.A / 17.E / 17.G / 17.H deferred (future PRD) with rationale below.
**Milestone:** v1.2 — Self-Improvement Foundations (continues from Phase 16)
**Decisions:** [ADR 0030](../../docs/decisions/0030-cross-package-test-visibility.md) (visibility for 17.D), [ADR 0031](../../docs/decisions/0031-target-specific-annotation.md) (annotation for 17.B).

## Goal

Close the explicit scope reductions taken during Phase 16 and add the obvious developer-experience polish that a single working test session reveals as missing. Each section below is independent — Phase 17 can ship any subset.

## Why now (or not)

Phase 16 shipped a working in-language testing framework with **six documented scope reductions** (recorded in `ops/plans/phase-16-testing-framework.md`). Without a successor PRD, those reductions drift into "we said we'd do that someday" territory; with this Draft on file, they're discoverable and prioritisable when capacity opens up.

Phase 17 is **not urgent**. Phase 16's framework is operational end-to-end on rust+js, six examples carry tests, `make validate` includes `intentc test`, and agents have a real assertion surface to verify their own work. Phase 17 turns that "operational" into "complete."

Recommendation: revisit when (a) a Phase-16-era friction actively blocks something downstream, or (b) the project has a natural pause point for polish work. Don't break flow on future polish work to do this.

---

## Sections (each independent)

### 17.A — Full testgen → Intent migration

**Source:** 16.8 scope reduction (`feat(testgen): --target intent emission (phase 16 task 16.8, partial)`, commit e888c67).

**State today:** `intentc test-gen --target intent` handles standalone 0-param and 1-Int-param functions with deterministic ranges. Entities, methods, multi-param non-Int functions, and complex constraints all fall through to a single-example call or are emitted as a "TODO" comment. The legacy `intentc test-gen` (Rust emission) remains the default and handles the cases Intent emission doesn't.

**Scope to add:**

1. **Entity / method emission.** Walk `prog.Entities`, emit `test "auto: <Entity>.<method>"` blocks that construct the entity (using constructor + default args), call the method, and assert each `ensures`. Needs entity `eq` method to be present for `assert_eq` on the entity itself; otherwise asserts field-by-field via accessor methods.

2. **Multi-param iteration.** For functions with N parameters where each has derivable bounds (Int with `requires` constraints, Bool, small enums), emit nested `while` loops. Cap at ~10³ iterations per test to keep runs fast.

3. **Inline-to-source mode.** Add `--inline` (or rename to `--append`) so generated tests are appended to the source file directly instead of writing a sibling `_test.intent`. Removes the `public` friction documented in Phase 16. The format would need a clear `// === auto-generated tests ===` marker so regenerations can locate and replace previous blocks.

4. **Retire the legacy Rust path.** Once 17.A.1 and 17.A.2 are in place, `--target rust` becomes unreachable for new code. Remove `internal/testgen/rustutil.go`, the `prngHelpers` constant, and the entire Rust-emission code path. Audit for downstream consumers before removing.

**Open question:** does anybody still want the Rust-emission path? It's the only way to get property-style PRNG-driven tests today (the Intent emission only does deterministic ranges). If the answer is "yes, for fuzzing-style stress tests," then keep `--target rust` and just stop calling it the default.

### 17.B — Synthetic cross-target divergence demonstration

**Source:** 16.9 scope reduction (no cross-target divergence test exists yet).

**State today:** the `--all-targets` runner can detect divergence, and the `FormatResults` output classifies it as `DIFF`. But no test in the repo actually exercises this path because Rust and JS agree on every test we've written.

**Scope to add:**

1. Find or construct a small Intent program where Rust and JS observably differ.
   - **Candidate 1:** Integer overflow. `Int * Int` near `i64::MAX` panics on Rust (debug) but silently wraps to a lossy `Number` on JS.
   - **Candidate 2:** Integer division semantics. Already documented as identical on both targets — probably not divergent.
   - **Candidate 3:** Float precision. `assert_eq` is type-checked away here, but `assert_close` with a very tight epsilon could expose IEEE 754 rounding differences.

2. Add the example to `examples/`, with a test demonstrating that `--all-targets` correctly flags the divergence as `DIFF` and exits non-zero.

3. Document the chosen divergence in `INTENT.md` as a "known cross-target gotcha" so users aren't surprised.

**Open question:** does intentionally checking in a divergence-flagging test set the wrong tone? It says "Intent doesn't guarantee equivalence; we just detect non-equivalence." Maybe the better surface is a `@target_specific` annotation (mentioned in ADR 0029) that lets users opt-in to single-target tests for cases the runtime semantics genuinely differ.

### 17.C — Tests for the remaining 13 flat examples

**Source:** 16.9 partial coverage (6 of 19 flat examples tested).

**State today:** the following examples have tests: fibonacci, array_sum, sorted_check, bank_account, generic_stack, async_demo. The following 13 don't: hello, enum_basic, shape_area, result_option, try_operator, error_handling, io_demo, js_demo, map_demo, handler_trait, task_queue, verify_example, closure_demo.

**Scope to add:**

Mechanical work. For each example, add at least one `test "..."` block exercising its headline behaviour. Most will be 2-5 lines each. Total ~30-60 minutes of work plus running validate.

Some examples (io_demo, js_demo) have side effects (file I/O, JS-specific demos) that don't lend themselves to assertion-based tests — those should still get a `test "smoke"` that confirms the entry point runs without panicking, or they should be moved out of the flat-examples directory if they aren't really illustrative.

**Open question:** is there an example that doesn't deserve a test (e.g. hello.intent — print "Hello, Intent!" — what's there to assert beyond "doesn't panic")? If yes, document the omission rather than forcing a no-op test.

### 17.D — Cross-package test discovery

**Source:** Phase 13 / Phase 16 deferred (validation footer in `ops/plans/phase-13-packages.md`).

**State today:** `intentc test foo.intent` works for single-file or contiguous-multi-file inputs (the runner already handles `isMulti` via `NewModuleRegistry`). But test blocks across multiple packages — e.g. tests in `examples/packages/types_pkg/` that exercise `examples/packages/app_pkg/` — aren't discovered together.

**Scope to add:**

1. When the entry file imports packages, the runner should collect tests from all transitively-imported modules, not just the entry.
2. Cross-package test names should be prefixed with their module/package name in the report so `types_pkg::point_distance_works` and `geometry_pkg::point_distance_works` don't collide.
3. Tests in a dependency that fail should fail the parent run too.

**Open question:** should a package's tests be public by default, or require `public test`? Phase 16 explicitly rejected `public test` because tests "don't cross module boundaries." Cross-package discovery reverses that decision. Likely the right model: tests are always discoverable across packages within the same project's import graph, but never exported to downstream consumers of a package. Needs an ADR.

### 17.E — FFI test automation

**Source:** Phase 15 deferred (validation footer in `ops/plans/phase-15-rust-ffi.md`).

**State today:** `examples/ffi_blake3/` uses an extern crate (the `blake3_intent` shim) and is manually verified ("run it, observe the 64-char hex digest"). No automated test exists.

**Scope to add:**

1. Add `test` blocks to `examples/ffi_blake3/ffi_blake3.intent` exercising the `blake3_hash` extern function with known input/output pairs.
2. The runner already supports rust target, but FFI examples need cargo to fetch and build the `blake3_intent` crate. Add the crate path resolution to the runner's temp Cargo.toml generation.
3. Skip the FFI tests automatically on non-rust targets (the existing extern-rejection error covers this — runner should treat it as Skipped, not Failed).

**Open question:** how does CI handle this? Building an FFI crate requires network or vendored deps. Document the cargo-required path and accept that CI either has cargo + the crate cached, or skips FFI tests in CI.

### 17.F — `intentc test --filter <substring>` and friends

**Source:** Not a Phase 16 deferral — pure DX polish that the running framework reveals as missing.

**State today:** `intentc test foo.intent` runs every test in `foo.intent`. No way to run a single test or a subset.

**Scope to add:**

1. `--filter <substring>`: only run tests whose name contains `<substring>`.
2. `--list`: print declared test names and exit without running.
3. `--quiet`: print only the summary line, not per-test results.
4. Exit code stays the same: non-zero on any failure.

Small, well-scoped, no design questions. Could ship in a single commit.

### 17.G — Full WASM test runner

**Source:** 16.6 scope reduction (`feat(target): WASM rejects test declarations (phase 16 task 16.6)`, commit 994b08c).

**State today:** WASM rejects all test declarations with a clear error. Users target rust or js for tests.

**Scope to add (if pursued):**

1. Each test compiles to a no-arg WASM export named `__test_<sanitised>` returning `i32` (0 = pass, 1 = fail).
2. Assertion failures need to write a message to an exported buffer or trap via `unreachable` and signal failure to the JS-side host.
3. Runner gains a wasm path that loads the module via `WebAssembly.instantiate` (Node 18+ supports this natively), calls each test export in turn, captures pass/fail and assertion messages.
4. Cross-target equivalence (`--all-targets`) now runs WASM as a real participant instead of always skipping.

**Open question — should we even do this?** WASM is not Intent's primary test target. Users who care about WASM behaviour can write tests on Rust (the same source compiles to both) and observe that the Rust tests pass — WASM correctness becomes a property of the WASM backend's lowering, not the user's test. The marginal value of "tests literally run inside WASM" is small for a non-trivial engineering investment. **My recommendation: only pursue this if a user surfaces a real case where Rust tests pass but WASM behaviour diverges.**

### 17.H — Coverage and snapshot testing

**Source:** Listed in Phase 16's "Out of Scope" section as future candidates.

**State today:** No coverage reporting, no snapshot testing.

These are bigger features that deserve their own PRDs if pursued. Mentioning here only so they don't get forgotten. **Recommend: defer to future PRD.**

---

## Success Criteria

- [x] **17.B:** `examples/divergence_demo.intent` demonstrates a real Rust-vs-JS divergence (large Int multiplication: Rust panics, JS produces a lossy Number). `intentc test --all-targets` correctly flags it as `DIFF` and exits non-zero. The `@target_specific(...)` annotation (ADR 0031) is implemented end-to-end (lexer AT token, parser annotation prefix, AST/IR carry-through, checker target validation + wasm warning + duplicate rejection, runner with `SkipAnnotation` kind distinct from WASM-rejection skip). `examples/target_specific_demo.intent` exercises all four cases (unannotated, rust-only, js-only, rust-or-js). Commit 4dacd6c.
- [x] **17.C:** every flat example in `examples/*.intent` carries at least one test (19/19). `make validate` runs `intentc test` on every tested example.
- [x] **17.D:** tests in imported modules and packages auto-discover via the entry file. JS multi-file emission now produces a unified `__intent_tests` registry; JS call sites apply the module's namePrefix when calling local functions. `examples/packages/types_pkg/types.intent` has tests that run via `intentc test examples/packages/app_pkg/main.intent`. ADR 0030 records the visibility decision.
- [x] **17.F:** `--filter <substring>`, `--list`, `--quiet` flags ship on `intentc test`. CLI handler, runner support, and `FormatList` / `FormatResultsQuiet` helpers all in place.
- [ ] 17.A — partial: 17.A.1 entity/method emission landed in Phase 27 (ADR 0036); 17.A.2 multi-param iteration landed in Phase 28 (ADR 0037). 17.A.4 (retire the legacy Rust testgen path) is the remaining piece — both prerequisites are now in place.
- [ ] 17.E — deferred (future PRD) (FFI test automation).
- [ ] 17.G — deferred (future PRD) if a real user case appears (full WASM test runner).
- [ ] 17.H — deferred (future PRD)+ (coverage / snapshot).

## Out of Scope

- Coverage reporting (future PRD)
- Snapshot/golden testing (future PRD)
- Test-only entities or visibility tier (no separate `@test` annotation)
- IDE integration with the test runner (Milestone 8 LSP work)
- Parallel test execution within a single run

## Resolved Decisions (2026-05-30)

1. **Legacy Rust testgen path** — Retained for now; removal deferred (future PRD). Rationale: the Rust path's PRNG-driven stress testing has no Intent-native equivalent. Designing Intent-level stress primitives (a `random()` builtin or seeded iteration) is its own piece of work that should land *before* the Rust path retires. **17.A deferred (future PRD).**
2. **Real WASM runner (17.G)** — **Not pursued.** Phase 16's WASM-rejects-tests is documented behaviour, not a bug. The marginal value of "tests literally run inside WASM" is low because the same Intent source compiles to all targets — if Rust tests pass, WASM correctness becomes a property of the WASM backend's lowering, not the user's test. **17.G deferred (future PRD)+ unless a real user case appears.**
3. **Cross-package test visibility** — Tests are auto-discoverable to the runner across the entire import graph; never exported as callable symbols. No `public test` keyword. See [ADR 0030](../../docs/decisions/0030-cross-package-test-visibility.md).
4. **Divergence demo vs `@target_specific` annotation** — Both. The annotation is the user-facing surface for legitimate target-specific tests (e.g. JS-only DOM logic, Rust-only FFI checks). The demo is a small artifact in `examples/` proving the runner detects unintentional divergence. See [ADR 0031](../../docs/decisions/0031-target-specific-annotation.md).
5. **Phase 17 scope** — **17.B + 17.C + 17.D + 17.F.** Sections 17.A, 17.E, 17.G, 17.H deferred (future PRD).

## Future Deferrals

The following Phase 17 sections are deferred. Each remains a documented future-work item rather than an open obligation. (Phase 18 itself is the LSP server, per `phase-18-lsp-server.md`; the items below get their own PRDs when prioritized.)

- **17.A** (full testgen migration): blocked on Intent-native stress-testing design. File new PRD when that design is ready.
- **17.E** (FFI test automation): low audience, requires cargo + named extern crates. Pick up when there's demand.
- **17.G** (WASM runner): cost/value unclear. Pick up only if a user case surfaces.
- **17.H** (coverage / snapshot): each warrants its own PRD; file when prioritised.

## Approval Record

- 2026-05-30 — Scope locked to 17.B + 17.C + 17.D + 17.F. ADRs 0030 and 0031 written and approved. Execution proceeding.
- 2026-05-30 — Shipped. 17.C, 17.D, 17.F fully implemented; 17.B partial (demo shipped, annotation deferred). Commits 582c570 (17.F), 5c36404 (17.C), 1f9e554 (17.D), 3c22710 (divergence demo).
- 2026-05-30 — 17.B closed out. `@target_specific` annotation implemented per ADR 0031 in commit 4dacd6c. 17.A / 17.E / 17.G / 17.H remain deferred per the scope decision.
