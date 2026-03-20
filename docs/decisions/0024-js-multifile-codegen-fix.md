# ADR 0024: JavaScript Multi-File Codegen Fix

## Status

Accepted

## Context

The JavaScript backend's multi-file code generation produced incorrect code for the Attractor example. Three categories of cross-module name resolution were broken:

1. Entity constructor calls used raw names (e.g., `new GraphMeta(...)`) instead of prefixed class names (e.g., `new TypesGraphMeta(...)`)
2. Enum variant constructors used raw enum names (e.g., `StageStatus.Success()`) instead of prefixed names (e.g., `TypesStageStatus.Success()`)
3. Intra-module function calls within non-entry modules lacked the module prefix (e.g., `node_exists()` instead of `validation_node_exists()`)
4. Cross-module function calls used declaration names instead of file-based names

## Decision

Applied the same cross-module name resolution patterns from the Rust backend to the JS backend:

1. **typeOrigins map**: Maps entity/enum names to their defining module's class prefix (e.g., `"GraphMeta" -> "Types"`). Used by new `mangledClassName()` method.
2. **Declaration name mappings**: `moduleManglings` now also maps declaration names (e.g., `"attractor_validation"`) to file-based prefixes (e.g., `"validation_"`).
3. **Intra-module function prefix**: `CallFunction` now emits `g.namePrefix + expr.Function` to prefix intra-module calls.
4. **Cross-module function resolution**: `generateMethodCallExpr` resolves module prefix via `moduleManglings`.

## Consequences

- Attractor example runs successfully in both Rust (native) and JavaScript (Node.js)
- All existing JS backend tests continue to pass
- Single-file JS codegen is unaffected (prefixes are empty for single-file)
