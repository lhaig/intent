# Phase 15: Rust FFI / Crate Imports

**Status:** Shipped (2026-05-28; completes Milestone 7)
**Milestone:** Milestone 7 — Language Evolution (final feature)
**Decision:** [ADR 0028](../../docs/decisions/0028-rust-ffi-crate-imports.md)
**Validated under Phase 16:** FFI tests against `examples/ffi_blake3/` deferred — extern function tests require cargo with the named crate available, which is target-specific (rust only) and beyond the scope of the cross-target runner. The existing manual verification (running the example, observing the 64-char hex digest) remains the validation surface; an automated rust-target-only FFI test is filed as Phase 17 work.

## Goal

Implement the `extern function ... from "crate::path"` declaration described in ADR 0028, so Intent programs can call arbitrary safe-Rust crate functions on the Rust target with contract-guarded boundaries.

## Success Criteria

- [x] `extern function name(...) returns T from "crate::path" requires ... ensures ...;` parses and type-checks
- [x] Type-check rejects unsupported types in extern signatures (entity, enum non-Result/Option, Map, Future, Fn, traits) with a clear diagnostic
- [x] Rust backend emits `crate_root::path::name(args)` at call sites, with `requires`/`ensures` asserts surrounding the call
- [x] `[rust_dependencies]` section in `intent.toml` (version pin or local path) flows through to the generated `Cargo.toml`'s `[dependencies]`
- [x] JS and WASM backends reject any program containing an extern declaration with a clear codegen error
- [x] `intentc fmt` formats `extern function` declarations canonically
- [x] All existing tests pass (no regressions)
- [x] New tests cover: parser, checker (positive + each rejection case), Rust codegen, manifest deps roundtrip, JS/WASM rejection, formatter, linter
- [x] `examples/ffi_blake3/ffi_blake3.intent` builds via cargo and produces the expected 64-char hex digest

## Reference

- Design: `docs/decisions/0028-rust-ffi-crate-imports.md`
- Existing extern-like wiring: `internal/checker/checker.go` builtin dispatch and `internal/rustbe/rustbe.go` Cargo.toml generation
- Manifest parser: `internal/compiler/manifest.go`
- Cargo.toml builder: `internal/compiler/compiler.go:buildCargoToml`

## Tasks

### 15.1 Lexer + Parser + AST

**Files:** `internal/lexer/token.go`, `internal/lexer/lexer.go`, `internal/ast/nodes.go`, `internal/parser/parser.go`, `docs/grammar.ebnf`

New token: `EXTERN` keyword.

New AST node:

```go
type ExternFunctionDecl struct {
    Name       string
    Params     []*Param
    ReturnType *TypeRef
    RustPath   string            // e.g. "blake3::hash"
    Requires   []*Contract
    Ensures    []*Contract
    Line       int
    Column     int
}
```

`Program.ExternFunctions []*ExternFunctionDecl` (a new top-level slice; do not merge into `Functions` because most pipeline passes treat extern functions differently).

Parser:

- `extern function NAME ( PARAMS ) returns T from "STRING_LITERAL" [REQUIRES] [ENSURES] ;`
- The trailing `;` distinguishes the extern form from a normal function definition (which has `{ ... }`).
- `from "..."` is required, not optional.

Grammar entry in `docs/grammar.ebnf`.

**Acceptance:**
- `go test ./internal/lexer/... ./internal/parser/... -v` passes with new tests
- Parser test: positive case, missing-`from`, missing-`;`, body-where-`;`-expected

### 15.2 Checker

**Files:** `internal/checker/checker.go`, `internal/checker/checker_test.go`

- Register `ExternFunctionDecl` in the function symbol table so call sites resolve through the existing call-checking path.
- Validate the FFI signature: each parameter type and the return type must be one of the bridge-supported set (see ADR 0028). Walk nested types (`Array<X>` → recurse on X). Reject otherwise with `extern function '%s': type '%s' is not supported across the FFI boundary; supported types are Int, Float, Bool, String, Void, Array<T>, Result<T,E>, Option<T> with bridged inner types`.
- Validate `from` string is non-empty and contains at least one `::` (must have a path inside the crate).
- Contracts work the same way as normal functions; `result` in `ensures` is the FFI return value.
- No body to check.

**Acceptance:**
- `go test ./internal/checker/... -v` covers: valid extern; entity in param rejected; Map<K,V> rejected; missing `::` in from rejected; valid Result<String, String>; valid Array<Int>.

### 15.3 IR

**Files:** `internal/ir/nodes.go`, `internal/ir/lower.go`, `internal/ir/validate.go`

- `ir.ExternFunction{Name, RustPath, Params, ReturnType, Requires, Ensures}` carried alongside `ir.Function` on `ir.Module`.
- Lowerer copies declared types verbatim (no monomorphization — extern functions cannot be generic).
- Validation: `RustPath` non-empty and contains `::`.

**Acceptance:**
- `go test ./internal/ir/...` passes; new test for ExternFunction lowering.

### 15.4 Rust backend

**Files:** `internal/rustbe/rustbe.go`, `internal/rustbe/rustbe_test.go`

- For each `ir.ExternFunction` in the module: emit nothing in the module body (no Rust declaration is needed — the crate provides it). Just track the crate root from the `from` path for Cargo.toml inclusion.
- At call sites for extern functions: emit `crate_root::path::function_name(args)`. Wrap in the standard `requires`/`ensures` pattern (asserts before and labeled-block after if `ensures` is present).
- Cargo.toml: union the crate roots used by extern functions with the existing sniffer set.
- Argument conversion: at FFI boundary, Intent String values are owned `String` already, Array<T> is `Vec<T>`, so no manual conversion is needed for the supported types — Rust accepts them directly.

**Acceptance:**
- `go test ./internal/rustbe/... -v` covers: extern call site emits the right path; Cargo.toml includes the crate; contracts surround the call.

### 15.5 Cargo.toml: [rust_dependencies] in intent.toml

**Files:** `internal/compiler/manifest.go`, `internal/compiler/manifest_test.go`, `internal/compiler/compiler.go`

- Manifest parser learns a new `[rust_dependencies]` section. Each entry is either a string (`name = "1.5"`) or an inline table (`name = { version = "0.13", features = ["std"] }`).
- `buildCargoToml` (or whichever code path reaches the Rust build) appends the resolved entries to the generated `Cargo.toml`'s `[dependencies]`. Order: tokio/futures/reqwest/serde_json sniffer entries first, then user rust_dependencies.
- Path is multi-file aware: `EmitProjectToTarget` and `BuildProjectToTarget` must surface the entry-package's `intent.toml` rust_dependencies into the Cargo.toml.

**Acceptance:**
- `go test ./internal/compiler/...` covers: parse rust_dependencies (both string and inline-table forms); buildCargoToml emits them; conflict with sniffer entries (user wins).

### 15.6 JS and WASM rejection

**Files:** `internal/compiler/target.go` (or wherever the per-target preflight runs)

- Walk the IR for `ExternFunction` declarations or call sites; if any exist and target is js or wasm, return: `extern function <names>: Rust FFI declarations are only supported on the rust target; use --target rust`.

**Acceptance:**
- Compiler test: extern + --target js → clean error, no output file written. Same for wasm.

### 15.7 Formatter and Linter

**Files:** `internal/formatter/formatter.go`, `internal/linter/linter.go`

- Formatter emits `extern function name(...) returns T\n    from "..."\n    requires ...\n    ensures ...;` with the same indentation rules as regular functions.
- Linter: warn if an extern function has no `requires` and no `ensures` — the whole point is contract-guarded boundaries.

**Acceptance:**
- `intentc fmt --check` clean on the new example; lint warning fires on a no-contract extern.

### 15.8 Example

**Files:** `examples/ffi_blake3.intent`, optionally extend `examples/attractor/intent.toml`

Pick a small, real crate. Default proposal: `blake3` (hash) — single function, simple signature, low compile-time cost.

```intent
module ffi_blake3 version "0.1.0";

extern function blake3_hash_hex(input: String) returns String
    from "blake3_intent::hash_hex"
    requires len(input) > 0
    ensures len(result) == 64;

entry function main() returns Int {
    let h: String = blake3_hash_hex("hello, intent");
    print(h);
    return 0;
}
```

Where `blake3_intent` is a 5-line wrapper crate published on crates.io (or we keep the wrapper inline as a path dep under `examples/ffi_blake3/`). Wrapping is needed because `blake3::hash` returns `blake3::Hash`, not a String — the type bridge requires we land in one of the supported Intent types.

**Open:** whether to publish a wrapper crate or vendor it as a local path dep. Recommendation: vendor it as `examples/ffi_blake3/blake3_intent/` so the example builds offline and doesn't depend on crates.io being available during CI.

**Acceptance:**
- `intentc build examples/ffi_blake3/ffi_blake3.intent` produces a binary that prints the 64-char hex digest.

### 15.9 ADR + Documentation

**Files:** `docs/decisions/0028-rust-ffi-crate-imports.md`, `docs/ROADMAP.md`, `INTENT.md`, `docs/DESIGN.md`, `docs/grammar.ebnf`

- Flip ADR 0028 status from `proposed` to `accepted` after implementation lands.
- ROADMAP Milestone 7 Rust FFI section: mark complete with [x] entries.
- `INTENT.md` and `docs/DESIGN.md` gain an FFI section with the syntax and the type-bridge table.
- `docs/grammar.ebnf` gains the `extern_function_decl` production.

**Acceptance:**
- All docs consistent with implementation; `go test ./... -timeout 60s` passes; `make clean` leaves no artifacts.

## Out of Scope

- C ABI extern, FFI with manual marshalling
- Importing Rust types, traits, or macros into Intent
- Block-style `extern from "crate" { ... }` (deferred; inline `from "..."` is enough for MVP)
- Map<K,V> across the FFI boundary
- Entity types across the FFI boundary
- WASM FFI via wasm-bindgen

## Suggested Order

1. 15.1 (lexer/parser/AST) — fastest grammar work, unblocks everything else
2. 15.2 (checker) — type bridge enforcement, the main correctness gate
3. 15.5 (manifest [rust_dependencies]) — independent, can land in parallel
4. 15.3 (IR) — small, mechanical
5. 15.4 (Rust backend) — call-site emit + crate root tracking
6. 15.6 (JS/WASM rejection) — preflight error path
7. 15.7 (formatter + linter)
8. 15.8 (blake3 example) — proves the end-to-end story works
9. 15.9 (docs + ADR status flip)
