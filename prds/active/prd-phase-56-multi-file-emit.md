# PRD — Phase 56: Multi-File Emit → Full Bootstrap (stage2 emits its own source)

**Status:** ACTIVE (kickoff 2026-07-09). Post-endgame of Phase 55: the single-file
Rust emitter self-hosts the full corpus (diff-emit 31/31). This phase extends the
stage2 compiler to MULTI-FILE emit and drives toward the true capstone — the
self-hosted compiler emitting its OWN (multi-module) source into a working stage3.

> **Read first after any compaction.** Phase 55 closed the single-file bootstrap loop
> (front-end Phases 42-54 + IR/backend Phase 55, all byte-equal with stage1). "Full
> coverage" self-hosting means the stage2 compiler can emit a MULTI-FILE program —
> ultimately its own source — byte-equal with stage1's `LowerAll`/`GenerateAll`. Work
> it in thin, byte-equal-gated slices exactly like Phase 55.

## Why / end state

The compiler's own source is multi-module (`selfhost/shared/*` + `selfhost/compiler/*`).
Phase 55's `intentc build --emit --self-hosted` rejected multi-file input. Reproducing
stage1's multi-file lowering (`ir.LowerAll`) and generation (`rustbe.GenerateAll`) —
cross-module name mangling, per-module emission, once-at-end `use` injection — is the
last piece before stage2 can regenerate itself.

**Done when:** the stage2 compiler emits Rust byte-equal with stage1 for multi-file
programs across a growing corpus, culminating in the `selfhost/**` source itself, and a
stage3 binary built from that emit matches stage2. Gated by `make diff-emit` (multi-file
entries added to the existing corpus) and, for the capstone, a stage2-vs-stage3 check.

## Design decision — resolve mangling at LOWERING, not in the backend

Stage1 builds a `moduleManglings` map inside `GenerateAll` and threads
`namePrefix`/`structPrefix`/`typeOrigins` through the entire generator. The stage2
backend is FREE FUNCTIONS threading `funcs` (gotcha b), so threading per-module prefix
state through the whole `generate_expr` tree would be invasive. Instead we **resolve the
mangling during lowering** (`lower_all`): a non-entry module's declarations get their
final mangled names in the IR, and a `mod.fn(args)` call is lowered to a pre-mangled
`irex_call` named `mod_fn`. The backend then needs no per-module prefix state — the
OUTPUT is identical (byte-equal is what the gate enforces), the churn far smaller. This
matches the existing stage2 deviation from stage1's structure.

## Slices (byte-equal-gated; ✅ = done, in `make diff-emit`)

- ✅ **56.1 — functions + module-qualified calls** (`examples/multi_file`, diff-emit 32/32).
  - Go: `stage2CompilePaths(entry)` returns the import closure in TOPOLOGICAL order
    (deps first, ENTRY last — contrast `stage2CheckPaths`, entry first for diagnostics);
    `--emit --self-hosted` passes it; dropped the multi-file rejection.
  - `lower.intent`: `LowerScope` +`module_names`/`module_prefixes` (the global qualifier→
    prefix map, EMPTY in single-file so single-file emit is byte-unchanged), threaded via
    the 5 copy helpers; `path_module_name`; `LowerCtx`; `lower` → `lower_module` + new
    `lower_all`; the ex_field-callee site detects a module qualifier and emits a
    pre-mangled `irex_call`. Non-entry module functions are name-prefixed + demoted from
    entry in `lower_function`.
  - `rustbe.intent`: extracted `generate_module_body`, added `generate_all` (multi-file
    header, global funcs table for cross-module borrow/arity, once-at-end HashMap
    injection).
  - `compile_main.intent`: N==1 single-file path unchanged; N>1 → `lower_all`/`generate_all`.

- **56.2 — cross-module entities/enums/traits** (structPrefix + typeOrigins). A non-entry
  module's `struct`/`enum`/`impl` names get the capitalised file-base prefix, and type
  references across modules resolve to the defining module's prefix (stage1
  `typeOrigins`). Extend `lower_module` to prefix type decls + `LowerCtx`/`lower_member`
  threading; add an attractor-style multi-file example to the corpus.

- **56.3 — emit the compiler's own source.** Feed `selfhost/shared/*` +
  `selfhost/compiler/*` through `--emit --self-hosted`, byte-equal with stage1. Expect
  gaps: the decl-name→file-base mangling (`shared_parser` qualifier → `parser_` prefix,
  the moduleManglings second pass), `use std::collections::HashMap;` (Map-heavy), and
  any construct combination the corpus didn't exercise.

- **56.4 — stage3 bootstrap.** Compile the stage2-emitted Rust into a stage3 binary and
  verify it matches stage2 (byte-equal emit / functional parity). Closes the full triangle.

## Gates per slice

`make diff-emit` (add the new multi-file entry) + `make selfcheck-formatter` +
`make selfcheck-checker` + `intentc test selfhost/compiler/{lower,ir}_test.intent`. If
`selfhost/shared/*` is touched: also `make diff-checker diff-formatter diff-linter` +
`go test ./...` + `make validate`. Changes to `cmd/intentc/*.go`: `go test ./cmd/intentc/...`.

## Non-goals (for now)

Multi-file ERROR-diagnostic parity (deferred per ADR 0058); the js/wasm backends in
Intent (a separate front); cross-package (`intent.toml`) emit beyond simple imports.
