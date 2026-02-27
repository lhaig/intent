# 0020: HTTP, JSON, and Event Builtins (Phase 8)

**Date:** 2026-02-27
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 8)

## Context

The Attractor pipeline runner needs network I/O for LLM API calls, JSON parsing for API responses, event emission for observability, and timestamp tracking for performance measurement. Phase 7 established the builtin pattern (name-match in checker, IR lowerer, Rust codegen, JS codegen). Phase 8 extends this with 5 new builtins.

## Decision

Add five built-in functions:

| Function | Signature | Returns | Rust Mapping |
|----------|-----------|---------|--------------|
| `http_post` | `(url: String, body: String, content_type: String)` | `Result<String, String>` | `reqwest::blocking::Client` POST |
| `http_get` | `(url: String)` | `Result<String, String>` | `reqwest::blocking::get()` |
| `json_get` | `(json_str: String, key: String)` | `Result<String, String>` | `serde_json::from_str` + top-level key lookup |
| `emit_event` | `(event_type: String, payload: String)` | `Void` | `eprintln!("[EVENT] {}: {}", ...)` |
| `timestamp_ms` | `()` | `Int` | `SystemTime::now().duration_since(UNIX_EPOCH).as_millis() as i64` |

Key design choices:

1. **Synchronous-only HTTP via `reqwest::blocking`** -- avoids async/tokio complexity, matches Intent's synchronous execution model.
2. **`serde_json` for JSON field extraction** -- top-level keys only via `json_get(json_str, key)`, sufficient for extracting LLM response content.
3. **JS backend uses `curl` via `execSync`** -- Node.js has no synchronous HTTP client, curl is universally available.
4. **Scan-based Cargo.toml dependency detection** -- `buildCargoToml()` scans emitted Rust source for `reqwest::` / `serde_json::` substrings to conditionally add crate dependencies.
5. **Event emission via stderr** -- `emit_event` writes `[EVENT] type: payload` to stderr, keeping stdout clean for program output.
6. **Timestamp as Int** -- `timestamp_ms()` returns epoch milliseconds as Int (i64), sufficient for timing and checkpoint metadata.

## Alternatives Considered

- **Async HTTP with tokio** -- Would require an async runtime, `async`/`await` keywords, and significant compiler changes. Synchronous blocking is appropriate for a pipeline tool that makes sequential API calls.
- **Full JSON parsing to an Intent value type** -- Would require a JSON value type (object, array, number, string, bool, null). `json_get` returning strings for top-level keys is sufficient for the Attractor use case (extracting `content` from LLM responses).
- **Node.js `http` module for JS backend** -- The built-in `http` module is callback-based and cannot be used synchronously. `curl` via `execSync` is the simplest synchronous option.
- **Static Cargo.toml with all dependencies** -- Would slow down compilation for programs that don't use HTTP/JSON. Scan-based detection keeps simple programs fast.
- **Event emission via stdout** -- Would intermingle events with program output, making it impossible to pipe program output cleanly.

## Consequences

- Programs using HTTP builtins require internet connectivity and longer compile times (reqwest pulls in many dependencies).
- JSON extraction is limited to top-level string fields -- nested access requires multiple calls or manual parsing.
- JS HTTP is not production-grade (shell escaping concerns with curl via execSync).
- Cargo.toml scanning is simple but could false-positive on string literals containing "reqwest::" -- acceptable tradeoff for zero-config dependency management.
