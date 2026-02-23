# Phase 3: Array<String> on Entity Fields

## Summary

Phase 3 adds `Array<String>` fields to Attractor entities, unlocking three previously blocked features:
1. Edge selection Step 3 (suggested next IDs)
2. Checkpoint completed_nodes tracking
3. BFS-based graph reachability lint rule

## Changes Made

### Entity Updates

**Outcome** — added `field suggested_next_ids: Array<String>` to carry suggested next node IDs from stage execution results. Constructor updated to accept the new field.

**Checkpoint** — added `field completed_nodes: Array<String>` to track which nodes have been visited. The `advance()` method now pushes the current node onto `completed_nodes` before moving to the next node.

### New Functions

**`find_suggested_next_edge`** (edge_selection.intent) — Step 3 of the 5-step edge selection algorithm. Iterates `suggested_next_ids` and finds the first edge whose `to_node` matches any suggested ID.

**`select_edge`** — Updated signature to accept `suggested_next_ids: Array<String>` and `suggested_count: Int`. Step 3 is now wired in between preferred label match (Step 2) and highest weight (Step 4).

**`string_array_contains`** (validation.intent) — Helper that checks if a string array contains a target value. Used by BFS reachability.

**`all_nodes_reachable`** (validation.intent) — BFS-based reachability check using `Array<String>` as both a worklist queue and visited set. Returns true if all nodes are reachable from `start_id`.

### Files Modified

| File | Changes |
|------|---------|
| types.intent | Added `suggested_next_ids` to Outcome, `completed_nodes` to Checkpoint |
| attractor.intent | Same entity changes, plus new functions and updated main() |
| edge_selection.intent | Added `find_suggested_next_edge`, updated `select_edge` signature |
| validation.intent | Added `string_array_contains` and `all_nodes_reachable` |
| main.intent | Updated `select_edge` calls, added Step 3 / reachability / checkpoint tests |

## Verification Results

- `go test ./... -timeout 30s` — all tests pass
- `./intentc check examples/attractor/attractor.intent` — no errors
- `./intentc check examples/attractor/main.intent` — no errors
- `./intentc build --emit-rust examples/attractor/attractor.intent` — valid Rust with `Vec<String>` fields
- `make check-examples` — all examples pass

## No Compiler Changes Required

As predicted, `Array<String>` on entity fields already worked through the full pipeline (parser, checker, IR, rustbe, jsbe). This phase was purely Attractor example updates.
