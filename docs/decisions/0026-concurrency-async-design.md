# ADR 0026: Concurrency and Async -- Design Plan

## Status

Proposed

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
