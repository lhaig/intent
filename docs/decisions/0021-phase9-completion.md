# ADR 0021: Phase 9 Completion -- Lint Rules, HandlerRegistry, Map Key Rejection, json_path

## Status

Accepted

## Context

Phases 1-8 achieved ~90% Attractor spec coverage. Phase 9 closes remaining gaps: missing WARNING lint rules, enum-based HandlerRegistry dispatch, Map<Float,V> compile-time rejection, and a `json_path` builtin for nested JSON extraction.

## Decisions

### 1. WARNING Lint Rules (validation.intent)

Five new warning functions follow the existing validation pattern: iterate nodes/edges, check a condition, push a `Diagnostic(rule, Warning, message, node_id)`.

- `retry_target_exists` -- validates retry_target references existing nodes
- `goal_gate_has_retry` -- goal_gate nodes must have retry_target
- `prompt_on_llm_nodes` -- codegen nodes must have non-empty prompt
- `type_known` -- node_type must be in known set
- `condition_syntax` -- non-empty edge conditions must contain "="

A `validate_warnings` aggregate calls all five and concatenates results.

### 2. HandlerRegistry (enum-based dispatch)

Rather than trait objects (not supported), we use a `HandlerKind` enum with `resolve_handler()` and `dispatch_handler()` functions. This provides static dispatch via if/else on enum variants -- type-safe and compatible with Intent's existing feature set.

### 3. Map<Float,V> Key Type Rejection

Float, Array, and Map are rejected as Map key types in `types.go` `ResolveType`. Rust's HashMap requires `Eq + Hash`, which floats don't satisfy. The checker returns nil for invalid key types, which triggers a type resolution error.

### 4. json_path Builtin

`json_path(String, String) -> Option<String>` follows the same 4-layer pattern as other builtins (checker, IR, rustbe, jsbe). The path uses dot-separated notation with array index support (e.g., `"choices.0.message.content"`).

### 5. Rust Codegen Improvements (discovered during E2E)

Multi-file Rust codegen had several naming issues exposed by Phase 9's more complex cross-module usage:

- **Type origin tracking**: Entity/enum names in function signatures now use the defining module's prefix via a `typeOrigins` map, not the current module's prefix.
- **Module declaration name mapping**: Cross-module function calls now resolve the declaration name (e.g., `attractor_validation`) to the filename-based prefix (e.g., `validation_`).
- **Enum variant qualification**: Unit enum variants used as values (not in match arms or constructors) now emit qualified names via a fallback in the IR lowerer.
- **Intra-module function name prefixing**: Function calls within a module now get the module's `namePrefix`.
- **Impl block entity names**: `impl Trait for Entity` now uses mangled entity names.

## Consequences

- Attractor spec coverage increases from ~90% to ~95% by section count
- All 12 of 12 lint rules are now implemented (7 structural + 5 warning)
- Handler dispatch is complete (resolve + dispatch pattern)
- Map key type safety enforced at compile time
- Nested JSON extraction available for LLM API responses
- Multi-file Rust codegen is improved but still has pre-existing ownership/borrowing issues that prevent native binary compilation of the full attractor example; these are tracked for a future codegen improvement phase
