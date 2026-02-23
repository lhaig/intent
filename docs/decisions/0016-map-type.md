# 0016: Map<K,V> Type (Phase 4)

**Date:** 2026-02-23
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 4)

## Context

Phase 3 completed Array<String> entity fields. Phase 4 adds `Map<K,V>` as a first-class generic type, unlocking three critical Attractor features:

1. **Context object** -- the central state-passing mechanism between pipeline stages (`Map<String, String>`)
2. **Checkpoint.node_retries** -- tracks retry counts per node (`Map<String, Int>`)
3. **Outcome.context_updates** -- inter-node data flow merged after each stage execution

## Decision

### Fully generic Map<K,V>

`Map<K,V>` supports any key/value type combination. The parser already handled `Map<K,V>` syntax correctly via recursive `parseTypeRef()`. Changes were needed in the checker (type resolution, method type-checking, builtins) and all three backends (Rust, JS, WASM).

### get(key, default) returns V

Two-arg `get` with a default value, matching the Attractor spec's `context.get(key, default)` pattern. This avoids forcing Option pattern matching at every call site. A future `get(key) -> Option<V>` can be added when Option handling is more ergonomic.

### Empty map literal reuses []

`let mutable m: Map<String, Int> = [];` -- the checker infers the Map type from the `let` type annotation, the same mechanism used for empty Array literals. This avoids parser changes (no `{}` ambiguity with blocks) and provides a consistent empty-container syntax.

### Backend targets

- **Rust:** `std::collections::HashMap<K,V>` with auto-imported `use` statement
- **JavaScript:** ES6 `Map` object
- **WASM:** Stub handling (pointer-based, consistent with existing approach)

### Method mapping

| Method | Rust | JS |
|--------|------|----|
| `get(key, default)` | `m.get(&k).cloned().unwrap_or(d)` | `(m.has(k) ? m.get(k) : d)` |
| `set(key, value)` | `m.insert(k, v)` | `m.set(k, v)` |
| `contains(key)` | `m.contains_key(&k)` | `m.has(k)` |
| `keys()` | `m.keys().cloned().collect::<Vec<_>>()` | `Array.from(m.keys())` |
| `remove(key)` | `m.remove(&k)` | `m.delete(k)` |
| `len(m)` | `(m.len() as i64)` | `(m.size)` |

### Mutability enforcement

`set()` and `remove()` require `let mutable`, matching the existing Array `push()` pattern. The checker validates this at compile time.

## Alternatives Considered

- **`get(key) -> Option<V>`** -- More type-safe but forces pattern matching at every read. The Attractor codebase reads context values dozens of times with sensible defaults. Deferred to a future phase.
- **Map literal syntax `{k: v}`** -- Would require parser changes to disambiguate from blocks. Empty `[]` is sufficient for now; populated maps use `set()`.
- **`Map<Float, V>` rejection** -- Rust's HashMap requires `K: Eq + Hash`, which floats don't satisfy. A proper type constraint system is needed to reject this at compile time. Deferred.

## Consequences

- PipelineContext entity now works with `Map<String, String>` for state propagation.
- Checkpoint tracks per-node retry counts via `Map<String, Int>`.
- Outcome carries context_updates for inter-node data flow.
- The condition evaluator can look up context variables dynamically.
- `Map<Float, V>` compiles but would fail at Rust compilation -- acceptable for POC.
- Phase 5 (Error Handling / Result<T,E>) is the clear next step.
