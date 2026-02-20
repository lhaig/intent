# Attractor-in-Intent: Development Strategy

## Approach

Attractor (a DOT-based AI pipeline orchestration spec) serves as the primary driver for Intent language development. By implementing the Attractor spec in Intent, we organically discover language gaps that need to be filled — each gap becomes a concrete, motivated roadmap item.

This is preferable to designing language features in the abstract: every addition is justified by a real use case, and progress is measurable against the spec.

## Current Coverage

**Phase 1 (complete):** Structural and logical skeleton — ~40% of the spec by section count.

| Spec Area | Status | Files |
|-----------|--------|-------|
| Type model (entities, enums, invariants) | Done | types.intent |
| Edge selection (5-step priority algorithm) | Done | edge_selection.intent |
| Retry policy (exponential backoff, 5 presets) | Done | retry.intent |
| Graph validation (7 of 12 lint rules) | Done | validation.intent |
| Status helpers (match on StageStatus variants) | Done | attractor.intent |
| Contract system (requires/ensures/invariant/intent blocks) | Done | all files |
| Multi-file module system | Done | main.intent imports 4 modules |

Both single-file (`attractor.intent`) and multi-file (`main.intent`) versions compile to native binaries and run correctly.

### What works well

- Entity invariants enforce spec constraints (max_retries >= 0, weight >= 0, no self-loops)
- Constructor contracts match invariant preconditions
- `old()` postconditions on Checkpoint.advance() enforce monotonic progress
- Match expressions cover all enum variants exhaustively
- Intent blocks link natural-language goals to formal contracts
- The 5-step edge selection algorithm is faithfully modeled with bounds postconditions

### Known limitations in current implementation

- `edge_matches_condition()` hard-codes ~15 string patterns instead of parsing conditions dynamically
- `normalize_label()` is a stub (returns input unchanged)
- `suggested_next_ids` field omitted from Outcome (needs Array\<String\>)
- Context type entirely absent (needs Map\<K,V\>)
- Checkpoint only tracks count, not completed_nodes list or node_retries map

## Gap-to-Feature Mapping

Each gap found while implementing Attractor maps to an Intent language feature. Ordered by what unlocks the most Attractor coverage:

### Phase 2: String Standard Library

**Attractor need:** Condition expression parsing, label normalization, context key lookup.

**What's blocked without it:**
- `evaluate_condition()` can't parse `"outcome=success && context.tests_passed=true"`
- `normalize_label()` can't strip accelerator prefixes or lowercase
- `context.*` condition keys can't be matched
- Reachability lint rule needs string equality checks in arrays

**Intent feature:** String methods — `split(delim)`, `to_lowercase()`, `trim()`, `starts_with(prefix)`, `contains(substr)`, `len()`.

**Attractor deliverable:** General-purpose condition parser replacing the hard-coded string comparisons. Label normalization. 5 more validation lint rules.

### Phase 3: Array\<String\> on Entity Fields

**Attractor need:** `suggested_next_ids` in Outcome, `completed_nodes` in Checkpoint, BFS worklist for reachability.

**What's blocked without it:**
- Edge selection Step 3 (suggested next IDs) is skipped entirely
- Checkpoint can't track which nodes have been visited
- Graph reachability requires a worklist queue of node ID strings

**Intent feature:** Confirm and fix `Array<T>` where T is a primitive type (String, Int, Bool) in entity fields, function params, and return types.

**Attractor deliverable:** Full 5-step edge selection, reachability lint rule, Checkpoint with completed_nodes.

### Phase 4: Map\<K,V\> Type

**Attractor need:** Context (the entire state-passing mechanism between pipeline stages), node_retries in Checkpoint, context_updates in Outcome.

**What's blocked without it:**
- The Context object — the central data structure of the execution engine
- `context_updates` field in Outcome (merged after each node)
- `node_retries` tracking in Checkpoint
- HandlerRegistry (mapping type strings to handlers)

**Intent feature:** `Map<K, V>` type with `get(key)`, `set(key, value)`, `contains(key)`, `keys()`, `remove(key)`.

**Attractor deliverable:** Context propagation between stages, full Checkpoint, condition evaluation with context variables.

### Phase 5: Error Handling (Result\<T,E\>)

**Attractor need:** Retry loop wraps handler execution in try/catch, converting exceptions to FAIL outcomes. ArtifactStore raises on missing artifacts. Validation raises ValidationError.

**What's blocked without it:**
- `execute_with_retry()` can't catch handler failures
- `validate_or_raise()` can't signal errors to callers
- The pattern "try something, fall back on failure" is pervasive in the spec

**Intent feature:** `Result<T, E>` type, `?` propagation operator, `match` on Ok/Err variants, contracts on error paths.

**Attractor deliverable:** Retry loop with exception handling, validation error propagation, artifact store error returns.

### Phase 6: Traits / Interfaces

**Attractor need:** Handler interface (9 handler types all implement `execute(node, context, graph) -> Outcome`), LintRule interface, Transform interface.

**What's blocked without it:**
- No unified Handler dispatch — each handler is an isolated function
- Can't build a HandlerRegistry that resolves handlers by type string
- LintRule and Transform extensibility patterns are inexpressible

**Intent feature:** Trait definitions, trait implementations on entities, trait-based dispatch in codegen.

**Attractor deliverable:** Handler interface with implementations for start, exit, codergen, conditional, wait.human, tool, stack.manager_loop. HandlerRegistry dispatch.

### Phase 7: I/O Standard Library

**Attractor need:** Log writing, checkpoint persistence, artifact storage, DOT file parsing.

**What's blocked without it:**
- Every "write to {logs_root}" operation
- Checkpoint save/restore
- ArtifactStore file-backed storage
- Reading .dot files from disk

**Intent feature:** File read/write, JSON serialization, environment variables, stdout formatting.

**Attractor deliverable:** Persistent execution engine with log output and checkpoint recovery.

### Phase 8: Network I/O and Runtime

**Attractor need:** LLM API calls, HTTP server mode, SSE event streaming.

**What's blocked without it:**
- CodergenBackend can't call LLM APIs
- No HTTP server for pipeline management
- No event streaming for TUI/web frontends

**Intent feature:** HTTP client/server, async I/O, FFI to Rust networking crates.

**Attractor deliverable:** Full Attractor runtime — a working AI pipeline orchestrator.

## Features Not Driven by Attractor

Some spec requirements fall outside Intent's design goals:

- **Concurrency** (parallel handler fan-out) — explicitly a non-goal for Intent. The parallel handler would need to be implemented in the Rust runtime layer.
- **Timestamp/Duration type** — needed for checkpoints and timeouts but could be modeled as Int (epoch millis) until a proper type is added.
- **Random number generation** — needed for retry jitter only. Low priority.

## How to Use This Document

1. Pick the next phase based on what's most impactful for Attractor coverage
2. Implement the Intent language feature (lexer/parser/checker/codegen)
3. Immediately use it in the Attractor example to validate
4. Document any new gaps discovered during implementation
5. Update this document with progress

Each phase should produce both a language improvement and a visible expansion of the Attractor implementation, keeping the two projects in lockstep.

## Progress Log

| Date | What | Bugs/Gaps Found |
|------|------|-----------------|
| 2026-02-20 | Phase 1 complete: 8 codegen/checker bugs fixed, single-file and multi-file compile and run | enum defaults, &self/&mut self, String cloning, cross-module resolution, empty arrays, verified_by for constructors and functions |
