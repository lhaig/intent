# 0028: Rust FFI / Crate Imports (Milestone 7 final feature)

**Date:** 2026-05-28
**Status:** accepted
**Phase:** Milestone 7 completion (implemented as Phase 15)

## Context

Milestone 7 is one feature short of complete. Intent already covers the common needs that drove its built-in roster (HTTP, JSON, file I/O, async, regex via string methods) but the Rust ecosystem is enormous and a closed list of builtins cannot grow fast enough to cover every domain. FFI gives users an explicit escape hatch: they can call any function from any Cargo crate from Intent code, with Intent contracts guarding the boundary.

Two non-goals frame the scope:

- **Not C ABI extern.** Intent's "FFI" is *crate-to-crate*. We are calling regular safe Rust functions from another Cargo crate. There is no `unsafe`, no manual marshalling, no concern for ABI mismatch. The unit of import is a Rust function in a crate.
- **Not a general-purpose Rust embedding.** Users cannot import Rust types, traits, or macros. Only functions whose signature is expressible in Intent's type vocabulary.

## Decision

Add a top-level `extern function` declaration that names a Rust crate and the function path within it, plus type-bridge rules and a `[rust_dependencies]` section in `intent.toml`.

### Syntax

```intent
extern function blake3_hash(input: String) returns String
    from "blake3::hash"
    requires len(input) > 0
    ensures len(result) == 64;

extern function compress(data: String, level: Int) returns Result<String, String>
    from "intent_zstd::compress_level"
    requires level >= 1
    requires level <= 22;
```

- `extern function <name>(...) returns <T>` is the Intent-visible signature. `name` is the symbol Intent code uses.
- `from "<crate_path>::<function>"` is the Rust path the Rust backend emits at the call site. The first segment is the crate name; everything after `::` is the path inside the crate.
- Contracts (`requires` / `ensures`) work exactly as on a regular Intent function. They compile to `assert!()` calls *before* and *after* the FFI call. `result` refers to the value returned by the Rust function.
- No body. The declaration ends with `;` instead of `{ ... }`.

### `intent.toml` rust_dependencies section

```toml
[package]
name = "my_project"
version = "0.1.0"

[rust_dependencies]
blake3 = "1.5"
intent_zstd = { version = "0.13", features = ["std"] }
```

`buildCargoToml` reads this section and emits the entries verbatim into the generated `Cargo.toml`'s `[dependencies]`. The existing crate-sniffing (`tokio::`, `futures::`, `reqwest::`, `serde_json::`) is preserved for the builtin path.

### Type bridges

| Intent type | Rust mapping at FFI boundary |
|---|---|
| `Int` | `i64` |
| `Float` | `f64` |
| `Bool` | `bool` |
| `String` | `String` (owned) |
| `Void` | `()` |
| `Array<T>` | `Vec<T>` where `T` is a bridged type |
| `Result<T, E>` | `Result<T, E>` where `T` and `E` are bridged |
| `Option<T>` | `Option<T>` where `T` is bridged |

Unsupported in extern signatures (rejected at type-check time with a clear error):

- `entity` types — the Rust struct names are mangled (`TypesPoint`) and not stable
- `enum` types other than `Result` / `Option` — variant shape isn't portable
- `Map<K, V>` — would force `HashMap` import in every signature; defer
- `Future<T>`, `Fn(...)`, traits — no clean ABI

This list can grow later; the MVP is conservative on purpose.

### Backend behavior

- **Rust backend:** emits a direct call `crate_root::path::function(args)` where `crate_root` is the first segment of the `from` string. Cargo.toml gains the corresponding `[dependencies]` entry from `[rust_dependencies]` in `intent.toml`.
- **JS backend:** rejects at codegen time with a clear error: `extern function <name>: Rust FFI declarations are only supported on the rust target; use --target rust`.
- **WASM backend:** same rejection as JS, with the additional note that wasm-bindgen FFI is out of scope.

### Contracts at the FFI boundary

This is the safety story. The user writes contracts that they trust the Rust crate to satisfy. The compiler does *not* statically verify the Rust crate matches the contract — that is the user's responsibility, exactly the same as for any other function whose body the verifier can't see (e.g., builtins). At runtime, `requires` asserts before the call catch bad inputs; `ensures` asserts after the call catch contract drift in the upstream crate (e.g., after a dependency upgrade).

This is the headline value: Intent users get to consume the Rust ecosystem *with a contract layer they own*.

## Alternatives Considered

- **C-ABI extern (`#[no_mangle]`, `extern "C"`).** Maximum portability but requires manual marshalling, `unsafe`, and a much larger design (header generation, layout compatibility, etc.). Crate-to-crate calls are 95% of what users actually want and are dramatically simpler.
- **Annotation-based syntax (`@rust(crate=…)` on a regular function).** Reusing the function declaration grammar avoids a new keyword, but it confuses readers: an annotated function looks like normal Intent code yet has no body. A dedicated `extern function` keyword makes the special status visible at a glance.
- **Block-style `extern from "crate" { … }`.** More compact for many imports from one crate. Defer — the inline `from "…"` form is sufficient for the MVP, and a block form can be added later without invalidating existing code.
- **Auto-derive contracts from Rust crate docs.** Tempting but out of scope. Contracts are the user's responsibility.
- **Support `Map<K, V>` and `entity` types in MVP.** Both require non-trivial conversions and stable name mangling. Defer until a real use case forces it.

## Consequences

- The Rust ecosystem becomes an explicit extension surface for Intent programs. Anything a Cargo crate provides — compression, hashing, regex, sqlite, etc. — is one declaration away.
- Programs using `extern function` no longer build on JS or WASM. The error message is clear and the failure is at codegen, not runtime.
- The `intent.toml` schema grows. `[rust_dependencies]` is additive; existing manifests are unaffected.
- The contract layer at the FFI boundary becomes the user's documented responsibility. ADR text and the per-extern `requires`/`ensures` must be the source of truth for what the consumer expects from the upstream crate.
- Compile times grow with each crate dependency, just as with the existing `http_get` / `serde_json` builtins.
- The compiler does not check Rust function signatures against the declared Intent signature at compile time. A signature mismatch surfaces as a cargo build error, which is acceptable.

## Implementation Notes (Phase 15)

Implemented as `prds/done/phase-15-rust-ffi.md`. Adjustments from the original design:

- `from` is a contextual identifier in the parser, not a global keyword — too common a name to reserve everywhere.
- `[rust_dependencies]` learned a `path = "..."` form alongside `version = "..."`. Required for the `examples/ffi_blake3/` demo because Intent's bridge does not accept `blake3::Hash` and a tiny wrapper crate (vendored at `examples/ffi_blake3/blake3_intent/`) exposes a `String -> String` shim. Relative paths in the manifest are resolved to absolute paths anchored at the intent.toml directory before being written into the generated `Cargo.toml`.
- `IsMultiFile` now also returns true when an intent.toml sits alongside the entry file. Without this, single-file builds bypassed the manifest entirely and `[rust_dependencies]` were silently ignored.
- The Rust backend's existing `cloneIfNeeded` / array-by-reference call-site conventions apply unchanged to extern call sites. The single new path is `lookupExtern` which short-circuits the call-site emit to use the verbatim `from "..."` path with no module-name mangling.
