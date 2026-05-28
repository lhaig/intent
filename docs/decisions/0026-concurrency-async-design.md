# ADR 0026: Concurrency and Async -- Design Plan

## Status

Accepted

## Context

The Attractor pipeline example would benefit from concurrent handler execution, async HTTP calls, and timeout handling. Intent currently has no concurrency primitives. The design must integrate with the contract system -- the core value proposition.

## Design

### Execution Model: Async/Await with Futures

Intent adopts async/await semantics similar to Rust, mapping directly to Rust's async runtime (tokio) and JavaScript's Promise system.

### Syntax

**Async functions:**
```intent
async function fetch_data(url: String) returns Result<String, String>
    requires len(url) > 0
    ensures result.is_ok() implies len(result) > 0
{
    let response: String = http_get(url, "")?;
    return Ok(response);
}
```

**Await expressions:**
```intent
async entry function main() returns Int {
    let data: Result<String, String> = await fetch_data("https://api.example.com");
    match data {
        Ok(body) => print(body),
        Err(e) => print(e),
    };
    return 0;
}
```

**Spawn for concurrent execution:**
```intent
async function run_pipeline(nodes: Array<NodeAttr>) returns Array<Outcome> {
    let mutable futures: Array<Future<Outcome>> = [];
    for node in nodes {
        futures.push(spawn execute_handler(node));
    }
    return await_all(futures);
}
```

### New Types

| Type | Rust Mapping | JS Mapping | Description |
|------|-------------|------------|-------------|
| `Future<T>` | `impl Future<Output=T>` | `Promise<T>` | Pending async computation |

### New Keywords

- `async` -- marks a function as asynchronous
- `await` -- suspends until a Future resolves
- `spawn` -- creates a concurrent task, returns `Future<T>`

### Contract Integration

Contracts on async functions work the same as sync:
- `requires` checked at function entry (before any await)
- `ensures` checked when the future resolves (after all awaits)
- `old()` captures values at function entry

**Key insight:** Contracts are checked in the async function's logical scope, not at the call site. This means:
```intent
async function fetch(url: String) returns Result<String, String>
    requires len(url) > 0         // checked when fetch() is called
    ensures result.is_ok()        // checked when the future resolves
```

### Built-in Async Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `await_all(futures)` | `Array<Future<T>> -> Array<T>` | Wait for all futures |
| `await_any(futures)` | `Array<Future<T>> -> T` | Wait for first to complete |
| `timeout(future, ms)` | `(Future<T>, Int) -> Result<T, String>` | Timeout wrapper |
| `sleep(ms)` | `Int -> Future<Void>` | Async sleep |

### Implementation Phases

**Phase 1: Parser + AST**
- Add `async` keyword to lexer
- Parse `async function`, `await expr`, `spawn expr`
- Add `IsAsync bool` to `FunctionDecl`
- Add `AwaitExpr`, `SpawnExpr` AST nodes
- Add `Future<T>` to `TypeRef` resolution

**Phase 2: Checker**
- Validate `await` only in async functions
- Validate `spawn` target is an async function call
- Type-check: `await Future<T>` produces `T`
- Type-check: `spawn async_fn(args)` produces `Future<T>`
- Validate async entry function

**Phase 3: IR + Lowering**
- Add `AwaitExpr`, `SpawnExpr` IR nodes
- Mark async functions in IR
- Thread Future types through IR

**Phase 4: Rust Backend**
- Async functions emit `async fn name(...) -> T`
- `await expr` emits `expr.await`
- `spawn` emits `tokio::spawn(async move { ... })`
- Entry function wraps in `#[tokio::main]`
- Add `tokio` to generated Cargo.toml dependencies

**Phase 5: JS Backend**
- Async functions emit `async function name(...)`
- `await expr` emits `await expr`
- `spawn` emits `Promise` wrapper
- `await_all` emits `Promise.all()`

### Constraints

- No shared mutable state between spawned tasks (avoids data races)
- No channels or message passing in initial implementation
- No async closures initially
- Async functions must return `Result<T, E>` or `Option<T>` or `Void` (enforces error handling)
- WASM backend: async not supported initially (WASM has limited async support)

### Dependencies

- None strictly, but benefits from generics (generic `Future<T>` handling)

## Consequences

- Enables concurrent pipeline execution in Attractor
- Contracts work naturally with async (checked at logical boundaries)
- Rust backend gets efficient async via tokio
- JS backend maps cleanly to native Promises
- Foundation for channels and structured concurrency in future

## Implementation Notes (Phase 14)

Phase 12 implemented this ADR but several codegen choices were ambiguous and produced uncompilable output. Phase 14 (`ops/plans/phase-14-phase11-13-gaps.md`) locked in the following decisions:

### Rust target: source-aware `Future<T>` lowering (revised by Phase 10 Attractor work)

The original Phase 14 rule "`Future<T>` always maps to `JoinHandle<T>`" was too coarse — it broke when async functions were called *directly* (without `spawn`) and when the same value was used by multiple spawn sites. The final lowering is:

- **Async function signatures:** an Intent `async function f() returns Future<T>` emits Rust `async fn f() -> T`. The Future wrapping is implicit in `async fn`; emitting `-> JoinHandle<T>` here was wrong because the body returns `T`, not a JoinHandle. The Rust backend's `fnReturnType` helper peels the `Future<>` wrapper for async-fn signatures only.
- **`spawn fn(args)`** lowers to `tokio::spawn(fn(args))`. Because the async fn already returns `impl Future<Output=T>`, `tokio::spawn` takes that future directly. Wrapping in `async move { fn(args).await }` (the earlier Phase 14 form) forced a by-move capture of every argument variable in the enclosing scope, which broke the common pattern of using the same variable across multiple spawn sites.
- **`await expr`:** the IR's `AwaitExpr` carries an `IsJoinHandle` flag set during lowering. It is `true` when the inner expression is structurally a `SpawnExpr` *or* a `VarRef` whose binding was a spawn (tracked via `joinHandleVars` on the lowerer).
  - `IsJoinHandle = true` → `(expr).await.expect("spawned task panicked")`. JoinError is treated as a panic; this matches typical Rust idiom for cooperatively-cancelled tasks.
  - `IsJoinHandle = false` → bare `(expr).await`. The inner is an `impl Future<Output=T>` from a direct async-fn call (or a builtin like `sleep`), and `.await` yields `T` directly with no JoinError layer.
- **`sleep` builtin** emits the bare Sleep future: `tokio::time::sleep(Duration::from_millis(ms as u64))`. The source-level `await sleep(...)` adds the single `.await`. (The earlier Phase 14 form wrapped sleep in `tokio::spawn(...)` to make `Future<T> = JoinHandle<T>` uniform; this is no longer needed once `IsJoinHandle` carries the distinction in the IR.)
- **`await_all` / `await_any`** unwrap each per-task `JoinError`: `futures::future::join_all(handles).await.into_iter().map(|r| r.expect("spawned task panicked")).collect::<Vec<_>>()` and `select_all(handles).await.0.expect("spawned task panicked")`. They return concrete values (`Array<T>`, `T`), not Futures, so they are not subject to the `IsJoinHandle` rule at the await site.
- **`timeout`** returns `Result<T, String>` (the Intent type) — `tokio::time::timeout(...).await.map_err(|_| "timeout".to_string())`.
- **`cloneIfNeeded`** skips `Future` types: `tokio::task::JoinHandle` is non-Clone and represents unique ownership of a spawned task, so its values must be moved, never cloned.
- **`Cargo.toml` generation** in `internal/compiler/compiler.go:buildCargoToml` sniffs for `tokio::` and `futures::` in the emitted source and adds `tokio = { version = "1", features = ["full"] }` and `futures = "0.3"` accordingly.

### Checker

- `spawn` accepts either a direct call `f(args)` or a module-qualified call `module.f(args)`. The earlier "spawn requires a function call" was too strict — multi-file Attractor code spawns across module boundaries.
- `Ok()` / `Err()` / `Some()` variant inference peels a `Future<>` wrapper from the enclosing function's declared return type, so an async function declared `returns Future<Result<T,E>>` can still write `return Ok(...)` in its body.

### JS target

- Entry function's `IsAsync` is honored: emitted as `async function __intent_main()` and invoked as `__intent_main().then(code => process.exit(code)).catch(err => { console.error(err); process.exit(1); });`.
- Functions/methods with `ensures` clauses use a labeled `__body: { ... }` block with `__result = X; break __body;` rewriting on `ReturnStmt`. This mirrors the Rust backend's labeled-block pattern and ensures postconditions are not bypassed by explicit early returns.

### WASM target: async rejected at compile time

The original ADR said "WASM backend: async not supported initially." Phase 14 made this concrete: `internal/compiler/target.go` rejects WASM builds whose IR contains any `IsAsync` function with a clear error message naming the offending functions and suggesting `--target rust` or `--target js`. This replaces the prior behavior of silently emitting invalid WASM bytecode.
