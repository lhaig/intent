# Phase 12: Concurrency and Async

**Status:** Shipped (v1.1, 2026-03-25; Rust/JS targets only — WASM rejects with clear error per Phase 14)
**Milestone:** v1.1 — Attractor Integration & Feature Expansion
**Decision:** [ADR 0026](../../docs/decisions/0026-concurrency-async-design.md) (revised by Phase 14 with source-aware `Future<T>` lowering)
**Deferred:** Attractor handlers async migration (stretch goal — tracked separately)
**Validated under Phase 16:** 2 hand-written async tests added to `examples/async_demo.intent` exercise `spawn` + `await` end-to-end on both rust (tokio) and js (native Promises). Both targets agree; WASM correctly rejects async tests with the documented error.

## Goal

Add async/await semantics to Intent with `Future<T>` type, `spawn` for concurrent execution, and contract integration. Maps to Rust's tokio runtime and JavaScript's native async/Promise system.

## Success Criteria

- [x] `async function fetch() returns Result<String, String>` parses and type-checks
- [x] `await expr` suspends async function, producing inner type
- [x] `spawn async_fn(args)` returns `Future<T>`
- [x] Contracts on async functions: `requires` at entry, `ensures` at resolve
- [x] Built-ins: `await_all`, `timeout`, `sleep`
- [x] Rust backend emits `async fn`, `.await`, `tokio::spawn` (corrected in [Phase 14](phase-14-phase11-13-gaps.md) 14.3 and 14.4)
- [x] JS backend emits `async function`, `await`, `Promise.all()` (corrected in [Phase 14](phase-14-phase11-13-gaps.md) 14.1 and 14.2)
- [x] All existing tests pass (no regressions)
- [x] `examples/async_demo.intent` compiles and runs on Rust and JS (JS verified end-to-end; Rust source verified, full `cargo build` not run locally — cargo unavailable on dev machine)
- [ ] Attractor example can be updated with async handlers (stretch goal) — **deferred**, tracked separately

> Verified retroactively under [Phase 14](phase-14-phase11-13-gaps.md).

## Reference

- Design: `docs/decisions/0026-concurrency-async-design.md`
- Existing error handling: `internal/checker/checker.go` (try operator)
- Existing builtins: `internal/checker/checker.go` (checkCallExpr)

## Prerequisites

- Phase 11 (Generics) is beneficial but not strictly required. `Future<T>` can be handled as a built-in generic type like `Option<T>`.

## Tasks

### 12.1 Lexer + Parser + AST

**Files:** `internal/lexer/token.go`, `internal/lexer/lexer.go`, `internal/ast/nodes.go`, `internal/parser/parser.go`

New tokens:
- `ASYNC` keyword
- `AWAIT` keyword
- `SPAWN` keyword

New AST nodes:
```go
type AwaitExpr struct {
    Expr   Expression
    Line   int
    Column int
}

type SpawnExpr struct {
    Expr   Expression  // must be a call to an async function
    Line   int
    Column int
}
```

Extend `FunctionDecl`:
```go
IsAsync bool  // true for async functions
```

Parser changes:
- `async function name(...)` -- parse `async` before `function`
- `await expr` -- new prefix expression in `parsePrimary`
- `spawn expr` -- new prefix expression in `parsePrimary`
- `Future<T>` in type position -- handled by existing generic type parsing

**Acceptance:**
- `go test ./internal/lexer/... ./internal/parser/... -v` passes
- Tests for: async function parsing, await expression, spawn expression, Future type

### 12.2 Checker

**Files:** `internal/checker/types.go`, `internal/checker/checker.go`

Type system:
- Add `Future<T>` as built-in generic type in `ResolveType` (like `Option<T>`)
- `await Future<T>` produces `T`
- `spawn async_fn(args)` produces `Future<ReturnType>`

Validation:
- `await` only valid inside `async` functions (track `isAsync` context like `currentFunc`)
- `spawn` argument must be a call expression to an async function
- Async functions can return any type (contracts are evaluated at resolve time)
- Contracts on async functions: same as sync (requires at entry, ensures at resolve)

Built-in async functions:
- `await_all(Array<Future<T>>) -> Array<T>`
- `await_any(Array<Future<T>>) -> T`
- `timeout(Future<T>, Int) -> Result<T, String>`
- `sleep(Int) -> Future<Void>`

**Acceptance:**
- `go test ./internal/checker/... -v` passes with tests for:
  - Valid async function declaration
  - `await` outside async function (error)
  - `await` on non-Future type (error)
  - `spawn` on non-async function (error)
  - Future type resolution
  - Contract checking on async functions

### 12.3 IR + Lowering

**Files:** `internal/ir/nodes.go`, `internal/ir/lower.go`, `internal/ir/validate.go`

New IR nodes:
```go
type AwaitExpr struct {
    Expr Expr
    Type *checker.Type  // the unwrapped type (T from Future<T>)
}

type SpawnExpr struct {
    Expr Expr
    Type *checker.Type  // Future<T>
}
```

Extend `Function`:
```go
IsAsync bool
```

Lowering:
- Lower `ast.AwaitExpr` to `ir.AwaitExpr`
- Lower `ast.SpawnExpr` to `ir.SpawnExpr`
- Preserve `IsAsync` flag on functions
- Validate async nodes in `validate.go`

**Acceptance:**
- `go test ./internal/ir/... -v` passes
- IR validation accepts async nodes

### 12.4 Rust Backend

**Files:** `internal/rustbe/rustbe.go`

Code generation:
- Async functions: `async fn name(...) -> T`
- `await expr`: `expr.await`
- `spawn expr`: `tokio::spawn(async move { expr })`
- Entry function: `#[tokio::main] async fn main()`
- `await_all`: generate `futures::future::join_all(futures).await`
- `timeout`: `tokio::time::timeout(Duration::from_millis(ms), future).await`
- `sleep`: `tokio::time::sleep(Duration::from_millis(ms)).await`

Dependencies:
- Add `tokio = { version = "1", features = ["full"] }` to generated Cargo.toml
- Add `futures` crate if `await_all` is used

Contract generation:
- `requires` assertions before first `await` (at function entry)
- `ensures` assertions use labeled block pattern (same as sync)

**Acceptance:**
- `go test ./internal/rustbe/... -v` passes
- `intentc build --emit-rust examples/async_demo.intent` produces valid Rust
- `cargo build` on generated code succeeds (requires tokio dependency)

### 12.5 JS Backend

**Files:** `internal/jsbe/jsbe.go`

Code generation:
- Async functions: `async function name(...)`
- `await expr`: `await expr`
- `spawn expr`: wrap in immediately-invoked async: `(async () => { return expr; })()`
- `await_all`: `await Promise.all(futures)`
- `await_any`: `await Promise.race(futures)`
- `timeout`: custom wrapper with `Promise.race([future, sleep_reject])`
- `sleep`: `new Promise(resolve => setTimeout(resolve, ms))`

**Acceptance:**
- `go test ./internal/jsbe/... -v` passes
- `intentc build --target js --emit examples/async_demo.intent` produces valid JS
- `node examples/async_demo.js` runs correctly

### 12.6 Example + Formatter + Docs

**Files:** `examples/async_demo.intent`, `internal/formatter/formatter.go`, docs

Example (`examples/async_demo.intent`):
```intent
module async_demo version "0.1.0";

async function delayed_add(a: Int, b: Int) returns Int
    requires a >= 0
    requires b >= 0
    ensures result == a + b
{
    await sleep(100);
    return a + b;
}

async entry function main() returns Int {
    let f1: Future<Int> = spawn delayed_add(3, 4);
    let f2: Future<Int> = spawn delayed_add(10, 20);
    let r1: Int = await f1;
    let r2: Int = await f2;
    print(r1);
    print(r2);
    return 0;
}
```

Formatter:
- Format `async function` declarations
- Format `await expr` and `spawn expr`

Docs:
- Update ADR 0026 status to "Accepted"
- Update DESIGN.md, INTENT.md, grammar.ebnf
- Run `gofmt -w` on all changed Go files

**Acceptance:**
- All docs updated
- `go test ./... -timeout 30s` passes
- `make clean` leaves no artifacts
