# 0015: Array<String> on Entity Fields (Phase 3)

**Date:** 2026-02-23
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 3)

## Context

Phase 2 added string methods, enabling dynamic condition parsing. Phase 3 needed `Array<String>` fields on entities to unlock three blocked Attractor features:

1. **Edge selection Step 3** -- the Attractor spec (Section 3.3) defines a 5-step priority algorithm for selecting the next edge. Step 3 checks `outcome.suggested_next_ids` against edge targets. This was skipped in Phase 1 because `Outcome` had no way to carry a list of node ID strings.

2. **Checkpoint.completed_nodes** -- the spec requires tracking which nodes have been visited, not just a count. This enables resumable execution and debugging visibility.

3. **Graph reachability lint rule** -- the spec (Section 7) requires validating that all nodes are reachable from the start node. BFS requires a worklist queue, which is naturally `Array<String>` of node IDs.

## Decision

### No compiler changes needed

Investigation confirmed that `Array<String>` on entity fields already worked through the full pipeline: parser accepts `Array<T>` field types, checker resolves them, IR lowers them, and all backends (Rust, JS, WASM) emit the correct target types (`Vec<String>`, `Array`, etc.). Empty array literals (`[]`) propagate their type correctly in both `let` declarations and `self.field = []` assignment contexts.

Phase 3 was implemented as purely Attractor example updates.

### Explicit count parameters alongside arrays

`select_edge` and `find_suggested_next_edge` take both `suggested_next_ids: Array<String>` and `suggested_count: Int` as separate parameters, rather than using `len(suggested_next_ids)` internally.

This follows the established pattern throughout the Attractor codebase (e.g., `nodes: Array<NodeAttr>, node_count: Int`). The reasons:

- **Contract expressibility** -- `requires suggested_count >= 0` and `ensures result < edge_count` are clearer with named parameters. The `len()` builtin is a runtime call, not a contract-friendly expression in the current checker.
- **Consistency** -- every array-processing function in the codebase uses this pattern. Mixing styles would create confusion about when to pass counts and when not to.
- **Forward compatibility** -- when `len()` becomes usable in contracts (a future checker enhancement), all functions can be updated uniformly.

### BFS with Array<String> instead of waiting for Set<T>

The reachability check uses `Array<String>` as both a worklist queue and a visited set. This is O(n^2) for the visited-check step (linear scan on each insertion) versus O(n) with a proper `Set<T>`.

We accepted this because:

- Pipeline graphs are small (typically 5-50 nodes). O(n^2) is negligible at this scale.
- `Set<T>` would require a new type with hashing semantics -- a much larger language addition than what Phase 3 needed.
- `Map<K,V>` (Phase 4) could later provide `Set<T>` as `Map<T, Bool>`, so a dedicated Set type would likely be redundant.
- The BFS implementation serves as a concrete motivating example for future collection type decisions.

### select_edge signature change

`select_edge` gained two new parameters: `suggested_next_ids: Array<String>` and `suggested_count: Int`. This is a breaking change to the function signature.

We chose to pass the suggested IDs as a flat array rather than passing the entire `Outcome` entity because:

- **Avoids borrow issues** -- Rust codegen for entity references involves `&self` borrowing. Passing the Outcome entity into `select_edge` alongside other data derived from the same Outcome would create complex borrow checker interactions.
- **Decoupling** -- `select_edge` doesn't need to know about `Outcome`'s other fields. It only needs the status string, preferred label, and suggested IDs -- all of which are already extracted by the caller.
- **Testability** -- flat parameters are easier to construct in tests without building a full Outcome entity.

## Alternatives Considered

- **Wait for Map<K,V> to implement reachability** -- Would have delayed the lint rule until Phase 4. The Array<String> approach works now and validates the feature.
- **Add len() to contract expressions** -- Would allow removing explicit count parameters. This is a checker enhancement that belongs in its own ADR when implemented.
- **Pass Outcome entity to select_edge** -- Cleaner API but creates Rust borrow checker complexity. The flat-parameter approach is pragmatic.

## Consequences

- The full 5-step edge selection algorithm from the Attractor spec is now implemented.
- Checkpoint tracks both count and node list, enabling future resumable execution.
- Graph reachability validation works for small graphs; may need optimization if Intent is used for very large graphs (unlikely for Attractor pipelines).
- The explicit-count-alongside-array pattern is now even more established. Any future move to `len()`-in-contracts should update all functions uniformly.
- Phase 4 (Map<K,V>) is the clear next step, with no Array<String> blockers remaining.
