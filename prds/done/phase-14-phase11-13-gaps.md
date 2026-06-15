# Phase 14: Phase 11-13 Gaps and Codegen Fixes

**Status:** Shipped (2026-05-28; closes audit gaps from Phases 11-13)
**Milestone:** v1.1 follow-up
**Touches decisions:** [ADR 0026](../../docs/decisions/0026-concurrency-async-design.md) (async semantics revised — see "Implementation Notes (Phase 14)" section)
**Validated under Phase 16:** Phase 14's async-on-WASM rejection is exercised by the runner's WASM-skip path on `examples/async_demo.intent`. Phase 14's source-aware `Future<T>` lowering is exercised by the two async tests on rust that succeed end-to-end (cargo + tokio). No new artifacts needed.

## Goal

Close the gaps found by a code-vs-PRD audit of Phases 11-13. Phases 11 (generics), 12 (async), and 13 (packages) were implemented in commit `8399b35` but never had their PRD checklists verified. Tests pass because they target compile/type-check stages, not actual runtime, and a several backends emit invalid code that only surfaces when the generated source is compiled or executed.

## Background: What Actually Works Today

- Phase 11 (generics): parser, checker, IR monomorphization, Rust + JS + WASM emit. `examples/generic_stack.intent` builds on all three targets and runs correctly on Node (`2`, `false`). WASM bytecode validates.
- Phase 12 (async): lexer, parser, checker (Future<T>, await ctx, await_all, await_any, timeout, sleep), IR, formatter, Rust + JS emit. **Generated code does not run.**
- Phase 13 (packages): manifest parser (TOML, no external deps), semver parser/constraints, cache module, registry resolution, all `intentc pkg ...` CLI commands, multi-package example builds. Rust output runs correctly. **JS output has a name-mangling bug.**
- ADRs 0025, 0026, 0027 all marked Accepted. `INTENT.md`, `docs/DESIGN.md`, `docs/grammar.ebnf` updated for all three features.

## Success Criteria

- [x] `examples/async_demo.intent` runs successfully on JS (`node async_demo.js` prints `7` and `30`)
- [x] `examples/async_demo.intent` produces Rust that should compile with `cargo build` (no double-`.await`, no `JoinHandle`/inner-type mismatch). *Full `cargo build` not run locally — cargo unavailable on dev machine; structural correctness verified by unit tests.*
- [x] `examples/async_demo.intent` is rejected on WASM with a clear error (Option A from 14.5)
- [x] `examples/packages/app_pkg/main.intent` runs successfully on JS (`node main.js` prints `25`)
- [x] `intentc fmt --check examples/generic_stack.intent` passes (canonical form committed)
- [x] `intentc fmt --check examples/async_demo.intent` passes
- [x] PRD checklists in `phase-11-generics.md`, `phase-12-async.md`, `phase-13-packages.md` marked complete (Phase 12 Attractor stretch goal marked deferred)
- [x] `intentc fmt --check examples/*.intent` passes for every example (also fixed `closure_demo.intent` drift in passing)
- [x] All existing tests pass

## Reference

- `prds/done/phase-11-generics.md`, `prds/done/phase-12-async.md`, `prds/done/phase-13-packages.md`
- ADR 0024 (`docs/decisions/0024-js-multifile-codegen-fix.md`) — prior cross-module name resolution work in JS backend
- ADR 0026 (`docs/decisions/0026-concurrency-async-design.md`) — async design

## Gaps

### 14.1 JS async codegen: entry function is not async (P0)

**File:** `internal/jsbe/jsbe.go`

When `async entry function main()` appears in source, the emitted JS is:

```js
function __intent_main() {           // missing 'async'
  let f1 = (async () => { return delayed_add(3, 4); })();
  let f2 = (async () => { return delayed_add(10, 20); })();
  let r1 = await f1;                 // SyntaxError: await is only valid in async functions
  ...
}
const __exitCode = __intent_main();  // not awaited; would be a Promise
process.exit(__exitCode);
```

Required fixes:
1. If the entry function is `async` (or contains `await`), emit `async function __intent_main()` and `__intent_main().then(c => process.exit(c))` (or `await` at top level if module mode is acceptable).
2. The `IsAsync` flag on `ir.Function` already exists; the entry path in `internal/jsbe/jsbe.go` around the `__intent_main` emit needs to honor it.

**Acceptance:**
- `node examples/async_demo.js` prints `7\n30` and exits 0
- New test in `internal/jsbe/jsbe_test.go` asserting `async function __intent_main` appears when the entry function is async

### 14.2 JS async codegen: dead postcondition after `return` (P2)

**File:** `internal/jsbe/jsbe.go` (function-emit logic)

In the generated JS for `delayed_add`, the `if (!((__result === (a + b))))` check is placed after `return (a + b);`, so it is dead code. The current pattern works in the no-`return` case because the `__result = ...` path falls through to the postcondition checks, but emitting both `return X;` and `__result === ...` after means the postcondition is unenforced.

Required fix: when the lowered body of an `ensures`-bearing function contains an explicit `return`, either lower it to `__result = ...; break 'body;` style (matching the Rust backend's labeled block) or wrap the body so postconditions execute on every exit. The Rust backend already uses the labeled block approach (`'body: { ... break 'body X; }`) — the JS backend should do the equivalent (assign to `__result` and use a labeled block with `break`, or use a function-scoped try/finally).

**Acceptance:**
- Postcondition violation inside an async function actually throws at runtime
- New JS backend test covering an async function with `ensures`

### 14.3 Rust async codegen: double `.await` on sleep builtin (P0)

**File:** `internal/rustbe/rustbe.go` (built-in dispatch around line 1416)

The `sleep` builtin emits `tokio::time::sleep(...).await`. Then `await sleep(100)` in source adds another `.await`, producing:

```rust
tokio::time::sleep(std::time::Duration::from_millis(100i64 as u64)).await.await;
```

Required fix: the builtin should emit `tokio::time::sleep(Duration::from_millis(ms as u64))` (a `Sleep` future) and rely on the surrounding `AwaitExpr` lowering to add `.await`. Same review needed for `await_all`, `await_any`, and `timeout` — confirm whether they add their own `.await` and whether the surrounding `AwaitExpr` is even allowed on them per ADR 0026.

**Acceptance:**
- Generated Rust for `async_demo.intent` contains a single `.await` after `sleep(...)`
- New `internal/rustbe/rustbe_test.go` case asserting no `.await.await` substring
- `cargo build` of generated Rust succeeds (gated behind a build tag if cargo is not available in CI)

### 14.4 Rust async codegen: `spawn` produces JoinHandle but is treated as inner type (P0)

**File:** `internal/rustbe/rustbe.go` (type mapping for `Future<T>` and `SpawnExpr` lowering)

Currently `spawn delayed_add(3, 4)` emits:
```rust
tokio::spawn(async move { delayed_add(3i64, 4i64) })   // spawns Future<Future<i64>>
```
and the destination type is `tokio::task::JoinHandle<i64>`. Then:
```rust
let r1: i64 = f1.await;
```
This will not compile: `JoinHandle::await` resolves to `Result<i64, JoinError>`, not `i64`.

Required fix: pick one of two strategies (decide and document):

- **Option A (recommended):** Emit `tokio::spawn(async move { delayed_add(...).await })` and wrap `f.await` in `.unwrap()` (or propagate the `JoinError` as part of the contract surface). Future<T> maps to `tokio::task::JoinHandle<T>`.
- **Option B:** Emit `let f1 = delayed_add(3,4);` (a bare Future, not spawned on the runtime) and treat `await` as `.await`. Spawning becomes opt-in via a separate builtin. This changes ADR 0026 semantics.

**Acceptance:**
- Generated Rust for `async_demo.intent` builds with `cargo build`
- New backend test that compiles the emitted Rust (skippable if cargo missing)
- ADR 0026 updated with the chosen semantics

### 14.5 WASM async codegen produces invalid bytecode (P1)

**File:** `internal/wasmbe/*`

`intentc build --target wasm examples/async_demo.intent` writes a `.wasm` file that fails `WebAssembly.validate`:

```
Compiling function #1 failed: local.set[0] expected type i32, found i64.const of type i64
```

Two acceptable outcomes:

- **Option A (recommended):** Detect `IsAsync` / `AwaitExpr` / `SpawnExpr` in the IR during WASM backend pre-check and emit a clear error: "async functions are not supported on the wasm target; use --target rust or --target js."
- **Option B:** Implement a real async-on-WASM lowering. Out of scope for a gap-fix phase — defer to a future phase.

ADR 0026 should be updated to state which targets support async.

**Acceptance:**
- `intentc build --target wasm examples/async_demo.intent` either produces a valid `.wasm` or fails with the documented error message (no silent invalid output)
- Test added asserting the chosen behavior

### 14.6 JS cross-package name-mangling mismatch (P0)

**File:** `internal/jsbe/jsbe.go` (multi-file emit path; see ADR 0024 for the prior cross-module fix)

For `examples/packages/app_pkg/main.intent` importing `types_pkg`:

```js
class TypesPoint { ... }                            // definition uses module name 'types'
function types_distance_squared(a, b) { ... }       // function uses module name 'types'
function __intent_main() {
  let p1 = new Types_pkgPoint(0.0, 0.0);            // call site uses package name 'types_pkg'
  let d = types_pkg_distance_squared(p1, p2);       // call site uses package name 'types_pkg'
}
```

Both sides must agree. The Rust backend is consistent (`TypesPoint` / `types_distance_squared` on both sides), so the bug is JS-specific.

Required fix: in the JS multi-file emitter, when lowering a cross-package call site, use the same `module`-name-derived mangling that the entity/function definition uses. ADR 0024 covered the cross-module case for entities/enums/functions within a single package; this is the cross-*package* extension of the same logic.

**Acceptance:**
- `node examples/packages/app_pkg/main.js` prints `25` and exits 0
- New JS backend test (or compiler integration test) covering multi-package codegen end-to-end
- The names emitted by definitions and call sites match for any cross-package symbol

### 14.7 Formatter drift on Phase 11 and 12 examples (P2)

**Files:** `examples/generic_stack.intent`, `examples/async_demo.intent`, possibly `internal/formatter/formatter.go`

Currently both files fail `intentc fmt --check`:

- `examples/generic_stack.intent`: source has `(result == true) implies (self.count == 0)`; formatter would strip the redundant parens. Also a blank-line difference at end of file.
- `examples/async_demo.intent`: trailing newline missing.

Decide and act:
- If the formatter's behavior is canonical, run `intentc fmt` on both files and commit. This is the simplest path.
- If the original source style is desirable (e.g. parens around `implies` operands for clarity), update the formatter to preserve them.

**Acceptance:**
- `intentc fmt --check` passes on every example in `examples/` (except multi-file where each file is checked individually)
- `make check-examples` exits 0

### 14.8 Verify Phase 11-13 PRD checklists (P2)

**Files:** `prds/done/phase-11-generics.md`, `prds/done/phase-12-async.md`, `prds/done/phase-13-packages.md`

Once 14.1-14.7 are landed, walk the success-criteria checkboxes in each of the three prior PRDs and mark them. For criteria that are not satisfied (e.g. Phase 12 "Attractor example can be updated with async handlers" stretch goal), explicitly note "deferred" with a link to a follow-up issue or this phase's PRD.

**Acceptance:**
- All three PRDs have their checklists in a final state (checked or annotated as deferred)
- `docs/ROADMAP.md` Milestone 7 entries for Generics, Traits, Async reflect actual implementation status

## Out of Scope

- Real package registry / `pkg install` for versioned dependencies (the current "no registry" warning is intentional per CLAUDE.md note on Phase 13)
- Async-on-WASM full implementation (we will reject it cleanly instead)
- Attractor migration to async handlers (Phase 12 stretch goal — track separately if still wanted)
- Async on traits / async constructors (not in ADR 0026 scope)

## Suggested Order

1. 14.6 (JS cross-package mangling) — small, fully isolated, unblocks the multi-package example
2. 14.1 (JS async entry) + 14.2 (dead postcondition) — both in JS backend, easy to test with `node`
3. 14.3 (double `.await`) + 14.4 (JoinHandle) — Rust backend; verify with `cargo build` if available
4. 14.5 (WASM async error) — simplest path is the explicit-error variant
5. 14.7 (formatter drift) — mechanical
6. 14.8 (PRD checkbox audit) — final cleanup
