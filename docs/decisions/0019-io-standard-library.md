# 0019: I/O Standard Library (Phase 7)

**Date:** 2026-02-27
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 7)

## Context

The Attractor pipeline runner needs persistent execution capabilities: saving and restoring checkpoints, writing status files, reading DOT graph files from disk, and configuring paths via environment variables. None of these are possible without file I/O and environment access.

Intent already has a pattern for built-in functions (print, len) that are recognized by name in the checker with no lexer/parser changes. The same pattern extends naturally to I/O operations.

## Decision

Add five built-in functions and one built-in method:

| Function | Signature | Returns | Rust Mapping |
|----------|-----------|---------|--------------|
| `read_file` | `(path: String)` | `Result<String, String>` | `std::fs::read_to_string().map_err(\|e\| e.to_string())` |
| `write_file` | `(path: String, content: String)` | `Result<Void, String>` | `std::fs::write().map_err(\|e\| e.to_string())` |
| `create_dir` | `(path: String)` | `Result<Void, String>` | `std::fs::create_dir_all().map_err(\|e\| e.to_string())` |
| `file_exists` | `(path: String)` | `Bool` | `std::path::Path::new(&p).exists()` |
| `env_get` | `(name: String)` | `Option<String>` | `std::env::var(n).ok()` |
| `.to_string()` | method on Int/Float/Bool | `String` | `.to_string()` (native Rust) |

Error-producing I/O functions return `Result<T, String>` rather than panicking, consistent with Intent's existing error handling philosophy (ADR 0017). `file_exists` returns `Bool` directly since existence checks are inherently non-failing. `env_get` returns `Option<String>` since a missing variable is not an error.

Each built-in is wired through four layers: checker (type validation), IR lowerer (call kind registration), Rust codegen, and JS codegen. WASM backend uses its existing default fallback for unknown builtins.

`to_string()` is implemented as a method (not a free function) because it operates on a specific value and chains naturally: `count.to_string()`. It is restricted to Int, Float, and Bool -- String already is a String, and entity/enum conversion requires user-defined formatting.

## Alternatives Considered

- **Standard library module with imports** -- Would require `import "io"` or similar. Intent's import system currently only supports `.intent` file imports, not built-in module resolution. Built-in functions avoid this complexity.
- **Synchronous vs async I/O** -- All operations are synchronous. Async would require a runtime concept that doesn't exist. Synchronous is appropriate for a compiler/pipeline tool.
- **`to_string()` as a free function** -- `to_string(42)` vs `42.to_string()`. Method syntax is more natural and consistent with String methods (ADR 0013). It also matches Rust's native `.to_string()`.
- **Returning `Option<String>` from `read_file`** -- Using `Result` gives callers the error message, which is important for debugging (permission denied vs file not found).
- **`create_dir` vs `create_dir_all`** -- Mapped to `create_dir_all` (recursive) since creating nested paths is the common case and the non-recursive variant is rarely what callers want.

## Consequences

- Attractor can now implement checkpoint save/restore, DOT file parsing from disk, and environment-based configuration.
- The JS backend uses IIFEs for Result-returning I/O functions to wrap try/catch in expression position.
- `to_string()` enables string interpolation patterns like `"count: " + n.to_string()`.
- No new keywords were introduced; `read_file`, `write_file`, etc. are identifiers recognized by the checker, so existing programs are unaffected.
- WASM backend stubs these functions (returns default values), consistent with existing stub pattern for non-portable operations.
