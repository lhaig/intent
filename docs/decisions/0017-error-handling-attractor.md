# 0017: Error Handling Patterns in Attractor Examples

**Date:** 2026-02-24
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 5)

## Context

The Attractor spec requires error handling in two key areas:
- **Section 3.5**: Handler execution with retry — `execute_with_retry()` must handle handler failures and decide whether to retry or propagate.
- **Section 7.3**: Graph validation — `validate_or_raise()` must signal validation failures to callers with descriptive error messages.

Intent already has `Result<T,E>` and `Option<T>` types with `match` expressions, `Ok`/`Err`/`Some`/`None` constructors, `is_ok()`/`is_err()` methods, and the `?` try operator. These were implemented in earlier compiler work but had not yet been used in the Attractor examples.

## Decision

Use `Result<T,E>` for all fallible operations instead of sentinel values or flag-based error signaling.

### Pattern: match + is_err() for flow control

Match arms in Intent are expression-only (they cannot contain statements like `continue` or `return`). This means:

```
// This does NOT work:
match handler_result {
    Ok(o) => { if o.is_success() { return Ok(o); } },  // error: statements in match arm
    Err(e) => { continue; },                              // error: statements in match arm
};
```

Instead, we use `is_err()`/`is_ok()` for flow control decisions, and `match` only to extract the inner value:

```
let is_error: Bool = handler_result.is_err();
if is_error {
    continue;  // flow control with if statement
}
let outcome: Outcome = match handler_result {
    Ok(o) => o,       // value extraction only
    Err(e) => ...,    // fallback value
};
```

### Pattern: early-return Err for validation

`validate_graph()` returns `Err("descriptive message")` on the first failing check, avoiding the need to accumulate errors:

```
if not has_exactly_one_start(nodes, count) {
    return Err("Graph must have exactly one start node");
}
```

This maps directly to the Attractor spec's `validate_or_raise` pattern.

## Alternatives Considered

1. **try/catch**: Intent compiles to Rust, which uses `Result` not exceptions. Adding try/catch would require a fundamentally different error model.

2. **Sentinel values** (return -1 for errors): Already used for index searches. But for operations with rich error context (retry, validation), sentinels lose the error message.

3. **Error accumulation** (collect all errors): More complex and not needed for the current use cases. Can be added later if validation diagnostics need to report multiple issues.

## Consequences

- All new fallible functions in Attractor examples use `Result<T,E>`
- The `match` + `is_err()` pattern becomes the idiomatic way to handle Result in loops
- The `?` try operator is used for clean error propagation in non-loop contexts
- No compiler changes were needed for Phase 5
