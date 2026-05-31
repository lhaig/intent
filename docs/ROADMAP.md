# Intent Language Roadmap

Milestones 1-3 from the original build plan are complete. The compiler handles
while/for loops, arrays, enums, pattern matching, Result/Option, try operator,
multi-file imports, loop invariants, quantifiers, and property-based test
generation. The language is usable for non-trivial programs.

A post-POC review (see [POST_POC_REVIEW.md](POST_POC_REVIEW.md)) identified
the architectural direction for what comes next. The key decisions are
documented in [ADR 0008](decisions/0008-intermediate-representation.md)
(introduce an IR) and [ADR 0009](decisions/0009-multi-target-codegen.md)
(multi-target code generation).

---

## Completed

### Milestone 1: Usable Language
- [x] While loops with break/continue
- [x] Print as built-in function (all primitive types)
- [x] Arrays (`Array<T>`, index access, `len()`, `push()`)
- [x] For loops (for-in over arrays, for-in over integer ranges)

### Milestone 2: Robust Language
- [x] Enums with unit and data-carrying variants
- [x] Pattern matching with exhaustiveness checking
- [x] `Result<T, E>` and `Option<T>` built-in types
- [x] Try operator (`?`) for error propagation
- [x] Multi-file imports with `import` declarations
- [x] Visibility modifiers (`public` / `private`)
- [x] Cross-file type checking and dependency resolution

### Milestone 3: Verification Depth (partial)
- [x] Loop invariants (`invariant` clause on while loops)
- [x] Loop termination (`decreases` clause)
- [x] Quantifiers in contracts (`forall`, `exists`)
- [x] Property-based test generation from contracts (`intentc test-gen`)

### Infrastructure
- [x] Lexer with full token set
- [x] Recursive-descent parser
- [x] Semantic checker (types, contracts, intents)
- [x] Rust code generator
- [x] Compiler pipeline (parse -> check -> codegen -> cargo)
- [x] CLI (`build`, `check`, `test-gen`, `lint`)
- [x] Linter (8+ rules)
- [x] `#![allow(...)]` in generated Rust to suppress cargo warnings
- [x] Examples (hello, bank_account, fibonacci, array_sum, enum_basic, shape_area, sorted_check, result_option, try_operator, multi_file)
- [x] Design document and grammar specification
- [x] Architecture decision records (ADRs 0000-0009)
- [x] Apache 2.0 license
- [x] GitHub Actions CI (test, build, vet, fmt-check)

### Formatter
- [x] Formatter (`intentc fmt`) -- fully implemented in `internal/formatter`

---

## Milestone 4: Intermediate Representation

**Goal:** Decouple contract semantics from Rust text generation. Enable
static verification and multi-target output.

See [ADR 0008](decisions/0008-intermediate-representation.md) for full rationale.

### Phase 4.1: Finish Tooling Foundations
- Implement `internal/formatter` package (currently referenced but missing)
- Better error messages: source snippets with underline markers
- Fix known checker bugs (missing return type checking, enum variant collisions)
- Fix codegen issues (fragile string concat detection)

### Phase 4.2: Design the Intent IR -- DONE
- [x] IR node types in `internal/ir/nodes.go` (~30 node types)
- [x] Contracts as first-class IR nodes (`Contract` with `Expr` + `RawText`)
- [x] `OldCapture` + `OldRef` as explicit pre-state nodes
- [x] Type information attached to every expression via `ExprType()`
- [x] `CallExpr` with resolved `CallKind` (function/constructor/variant/builtin/method)
- [x] `StringConcat` as dedicated node (replaces fragile AST detection)
- [x] `MatchPattern` with resolved enum name and field bindings

### Phase 4.3: Implement IR Lowering -- DONE
- [x] `internal/ir/lower.go` with `Lower()` and `LowerAll()` entry points
- [x] `old()` expressions lowered to `OldCapture` + `OldRef` nodes
- [x] String concat detected via checker type info
- [x] Call resolution using checker's entity/enum/function maps
- [x] Literal parsing (string to typed values)
- [x] 7 IR-level unit tests in `internal/ir/lower_test.go`

### Phase 4.4: Refactor Rust Codegen to Consume IR -- DONE
- [x] New `internal/rustbe/` package consumes IR instead of AST
- [x] Byte-identical Rust output verified against legacy codegen for all 9 examples
- [x] Compiler pipeline switched: `checker.CheckWithResult()` -> `ir.Lower()` -> `rustbe.Generate()`
- [x] Legacy `codegen.Generate()` / `codegen.GenerateAll()` marked deprecated
- [x] `testgen` still uses `codegen.ExprToRust()` internally (to be migrated later)

### Phase 4.5: IR Validation and Testing -- DONE
- [x] `internal/ir/validate.go` with structural validation (nil types, duplicate captures, etc.)
- [x] 11 IR-level unit tests in `internal/ir/validate_test.go`
- [x] Round-trip integration tests for all example files in `internal/ir/integration_test.go`

---

## Milestone 5: Static Verification (Z3) -- COMPLETE

**Goal:** Prove contracts at compile time. Move from "runtime assertions" to
"verified correctness."

This is the milestone that differentiates Intent from "Rust + contract macros."

### Phase 5.1-5.2: SMT Translation and Z3 Integration -- DONE
- [x] `internal/verify/smt.go` translates IR contracts to SMT-LIB formulae
- [x] Intent types mapped to SMT sorts (Int -> Int, Bool -> Bool, Float -> Real)
- [x] `internal/verify/verifier.go` invokes Z3 solver via `exec.Command`
- [x] Per-contract status: "verified" / "unverified" / "error" / "timeout"
- [x] CLI command: `intentc verify <file.intent>`
- [x] Graceful degradation when Z3 is not installed
- [x] 7 tests in `internal/verify/verify_test.go`

### Phase 5.3: Verification Reporting -- DONE
- [x] Entity contract verification (constructor, method requires/ensures, invariants)
- [x] `verified_by` references checked semantically against Z3 results
- [x] Intent blocks report verification status with per-contract detail
- [x] Human-readable verification report via `internal/verify/report.go`
- [x] `VerifyResult.QualifiedName()` for entity-qualified contract names

### Phase 5.4: Quantifier Verification -- DONE
- [x] Loop invariant verification via inductive reasoning (assume inv + cond, prove inv preserved)
- [x] `TranslateLoopInvariant()` and `TranslateLoopInvariantForMethod()` in smt.go
- [x] Invariant verification in functions, constructors, and methods
- [x] `OldRef` handling in SMT translation

---

## Milestone 6: Multi-Target Code Generation -- COMPLETE

**Goal:** Intent programs run on more than just native binaries.

See [ADR 0009](decisions/0009-multi-target-codegen.md) for full rationale.

### Phase 6.1: Codegen Backend Interface -- DONE
- [x] `internal/backend/backend.go` defines `Backend` interface (`Name()`, `Generate()`, `GenerateAll()`)
- [x] `internal/backend/rust.go` wraps `rustbe` package
- [x] `internal/backend/js.go` wraps `jsbe` package

### Phase 6.2: WASM Target (via Rust) -- DONE
- [x] `intentc build --target wasm <file.intent>`
- [x] Leverages Rust's `wasm32-unknown-unknown` target via cargo
- [x] `internal/compiler/target.go` with `EmitToTarget()` / `BuildToTarget()`

### Phase 6.3: JavaScript Target -- DONE
- [x] `intentc build --target js <file.intent>`
- [x] `internal/jsbe/jsbe.go` direct JS emission from IR (~1000 lines)
- [x] ES6 classes for entities, object-based enums, contract checks via throw
- [x] Type mapping: Int->number, Float->number, Bool->boolean, String->string
- [x] 6 tests in `internal/jsbe/jsbe_test.go`

### Phase 6.4: Direct WASM Emission -- DONE
- [x] `internal/wasmbe/` package emits WASM binary directly from IR
- [x] WASM binary encoding: LEB128, sections, opcodes in `encoding.go`
- [x] Full expression/statement compilation: arithmetic, control flow, function calls
- [x] `internal/backend/wasm.go` implements `BinaryBackend` interface
- [x] No Rust toolchain dependency; instant WASM compilation
- [x] Validated with Node.js WebAssembly.validate() and runtime execution
- [x] 9 tests in `internal/wasmbe/wasmbe_test.go`

---

## Milestone 7: Language Evolution

**Goal:** Features that expand what Intent programs can express.

These are deferred until the IR and verification foundations are in place,
because each feature needs to work across all backends and with the verifier.

### Generics -- DONE
- [x] Type parameters on functions and entities (Phase 11)
- [x] Monomorphization in codegen (one concrete type per instantiation)
- [x] Contract expressions over generic types
- [x] Rust, JS, WASM backends emit monomorphized output

### Traits / Interfaces -- DONE
- [x] Behavioral contracts across types
- [x] Default method implementations
- [x] Trait-based static dispatch in codegen
- [x] ADR 0018 accepted

### Async / Concurrency -- DONE (Rust and JS; not WASM)
- [x] `async function`, `await`, `spawn` (Phase 12)
- [x] `Future<T>` as built-in generic; `await_all`, `await_any`, `timeout`, `sleep` builtins
- [x] Contracts on async functions (`requires` at entry, `ensures` at resolve)
- [x] Rust backend: `tokio` runtime, `JoinHandle`-based futures
- [x] JS backend: native async/Promise
- [x] WASM target rejects async with clear error (no runtime to host it)

### Package Management -- DONE (local path deps; registry deferred)
- [x] `intent.toml` manifest, semver constraints (Phase 13)
- [x] Local path dependencies, `intentc pkg` CLI
- [x] Cross-package type references (entities, enums, traits) on Rust and JS
- [ ] Real package registry (versioned remote fetch) — deferred; ADR 0027

### String Interpolation -- DONE
- [x] `"Balance: {self.balance}"` syntax across lexer, parser, checker, IR, backends
- [x] Rust backend: `format!()` macro generation
- [x] JS backend: template literal with `${}` interpolation
- [x] End-to-end compiler test in `compiler_test.go`

### String Standard Library -- DONE
- [x] Methods on String: `split(delim)`, `to_lowercase()`, `trim()`, `starts_with(prefix)`, `contains(substr)`, `len()` (ADR 0013)
- [x] Rust backend maps to `String`/`str` methods
- [x] JS backend maps to native string methods
- [x] Drove Attractor condition expression parsing and label normalization

### Map Type -- DONE
- [x] `Map<K, V>` with `get(key, default)`, `set(key, value)`, `contains(key)`, `keys()`, `remove(key)` (ADR 0016)
- [x] Rust backend maps to `HashMap<K, V>`; JS backend uses plain object literals
- [x] Float/Array/Map key types rejected at type-check time
- [x] Drove Attractor Context (state passing) and HandlerRegistry

### Rust FFI / Crate Imports -- DONE
- [x] `extern function NAME(...) returns T from "crate::path"` declaration (Phase 15, ADR 0028)
- [x] Type-bridge restricted to Int/Float/Bool/String/Void/Array<T>/Result<T,E>/Option<T>; entity, user enum, Map, Future, Fn rejected at type-check
- [x] Contracts surround the call (requires/ensures compile to asserts)
- [x] `[rust_dependencies]` in intent.toml (version pin or local path) flows through to generated Cargo.toml
- [x] JS and WASM targets reject extern at codegen with named-function error
- [x] Formatter + linter (warn on contractless extern)
- [x] `examples/ffi_blake3/` builds via cargo and prints the 64-char hex digest

---

## v1.2: Self-Improvement Foundations -- IN PROGRESS

**Goal:** Give the project (and the agents working on it) the mechanical-validation surface needed to push Intent forward without per-change human review. See [docs/HARNESS.md](HARNESS.md) for the agent harness contract.

### Phase 16: In-Language Testing Framework -- SHIPPED (2026-05-29)
- [x] `test "name" { ... }` blocks parse and type-check
- [x] `assert` / `assert_eq` / `assert_close` / `assert_panics` builtins
- [x] `intentc test` runner on Rust + JS; WASM rejects with clear error
- [x] `intentc test --all-targets` flags cross-backend divergence (rust + js; wasm skipped)
- [x] `intentc test-gen --target intent` (partial — see Phase 17 for the rest)
- [x] 6 of 19 flat examples carry tests; remaining 13 deferred to Phase 17
- [x] `make validate` includes `intentc test` over tested examples

Design lives in `ops/plans/phase-16-testing-framework.md` (Shipped). 10 commits 95c545f..f02bf13.

### Phase 17: Testing Framework Polish -- SHIPPED (2026-05-30)
- [x] 17.B: divergence demo + `@target_specific` annotation (ADR 0031) — full pipeline (lexer/parser/AST/checker/IR/runner) with `SkipAnnotation` distinct from WASM-rejection skip
- [x] 17.C: tests for all 19 flat examples
- [x] 17.D: cross-package test discovery (ADR 0030)
- [x] 17.F: `--filter`, `--list`, `--quiet` DX flags
- [ ] 17.A / 17.E / 17.G / 17.H: deferred (future PRDs) with documented rationale

Design + execution record: `ops/plans/phase-17-testing-polish.md`.

### LSP Server Scoping ADR -- DONE (2026-05-30)
Scoped in [ADR 0032](decisions/0032-lsp-v1-surface.md).

### Phase 18: LSP Server v1 -- SHIPPED (2026-05-30)
Implemented in commits 0a4ca14..4999b06.
- `intentc lsp` subcommand starts the stdio LSP 3.17 server
- Diagnostics: parser, checker, lint (debounced ~150ms), Z3 verification (async on save)
- Hover: signature + full requires/ensures + entity fields/invariants
- Go-to-definition: same-file + same-package via the workspace cache
- VS Code extension under `editors/vscode/` (.vsix builds via `npm run package`)
- End-to-end smoke test in `internal/lsp/e2e_test.go`

### Phase 22: `--strip-contracts` Flag -- SHIPPED (2026-05-31)
Implemented in commits f8e4473..HEAD. Adds a single `--strip-contracts` flag to `intentc build` ([ADR 0033](decisions/0033-release-flag-strip-policy.md)). On the rust target every contract `assert!(...)` becomes `debug_assert!(...)`; cargo's existing always-on `--release` profile then compiles the calls out. On JS the `if (!(cond)) throw new Error(...)` lines are omitted. WASM is a no-op. User-written `assert(...)` / `assert_eq(...)` in test bodies are unaffected — they're the runtime assertion API, not contracts. A one-line stderr warning fires on use, surfacing the safety trade in CI logs. The original ADR proposed a two-flag design (`--release` + `--strip-contracts`); during implementation we discovered `intentc build` already always passes `--release` to cargo, making the separate `--release` flag redundant. ADR revised in-place to drop it. Verification-aware stripping (`--strip-contracts=verified`) is the next ADR; this phase is the foundation.

### Phase 21: LSP Member Completion -- SHIPPED (2026-05-31)
Implemented in commits 7c44883..HEAD. Closes the last v1.1 LSP capability gap: typing `receiver.` returns the entity's fields + methods (with `CompletionField` / `CompletionMethod` kinds) instead of the full identifier list. `.` is now a completion trigger character so the popover fires on the dot. Receivers resolve through `self` in methods/constructors and typed locals/params; chained access and call-result chains remain out of scope (single-step receiver typing only). Phase 19 had deferred this on the assumption new infrastructure was needed; in practice Phase 19's `receiverBeforeMember` + `resolveMemberOnReceiver` were already enough, so Phase 21 is a small, self-contained follow-on. ADR 0032 revised again.

### Phase 20: LSP Polish + Production-Ready Extension -- SHIPPED (2026-05-31)
Implemented in commits addd43b..757abfe. ADR 0032 revised again. Closes the polish gap before Marketplace:
- Tier-1 TextMate grammar (function defs/calls, methods, fields, built-ins, string interpolation)
- Semantic tokens (`textDocument/semanticTokens/full`) — type-aware highlighting that beats TextMate's regex limits
- esbuild bundling (.vsix is 90 KB single-file vs the previous 300 KB tree)
- Status bar item + `Intent: Restart Server` / `Intent: Show Output` commands
- Marketplace metadata: CHANGELOG, placeholder icon, gallery banner, keywords
- Publishing blocked only on credentials (publisher account, PAT) + branded icon — engineering side publish-ready

### Phase 19: LSP v1 Completion -- SHIPPED (2026-05-30)
Implemented in commits 888f80b..257ccfe. ADR 0032 revised in-place. Adds the "feels half-built" gaps from Phase 18:
- Scope walker: hover/goto-def resolve locals, params, `self`, fields, methods (single-step receiver)
- `textDocument/documentSymbol` for the outline view
- `textDocument/formatting` runs `internal/formatter`
- `textDocument/signatureHelp` with active-parameter tracking
- `textDocument/completion` for identifiers (locals + top-level + sibling modules + keywords + built-in types)
- Member completion (.field/.method) deferred to v1.2 (landed in Phase 21)
- Find-references, rename, code actions, refactorings, semantic tokens, inlay hints, cross-package goto-def, Marketplace publishing — still v1.1+

### Phase 17.B Annotation Implementation -- DONE (2026-05-30)
Shipped per ADR 0031 in commit 4dacd6c. Lexer/parser/AST/checker/IR/runner all carry the `@target_specific("rust", ...)` annotation through; new `SkipAnnotation` classification keeps annotation skips distinct from the WASM-rejection skip. `examples/target_specific_demo.intent` demonstrates all four cases.

### Package Registry Remote-Fetch -- DEFERRED (no user demand)
Closes the ADR 0027 deferred item. Manifest, semver, cache, and resolver are in place; only the network-fetch step is stubbed. **Deferred until there are real users publishing Intent packages.** No point building a registry no one is going to push to.

---

## Milestone 8: Developer Experience

### LSP Server -- v1 SHIPPED (2026-05-30)
Scoped in [ADR 0032](decisions/0032-lsp-v1-surface.md) (revised in Phase 19); implemented as Phase 18 + Phase 19. v1 surface in production:
- Diagnostics (parser, checker, lint warnings, Z3 verification status)
- Hover and goto-def (top-level decls, locals, params, self, fields, methods)
- Document symbols (outline view)
- Formatting (`textDocument/formatting` via `intentc fmt`)
- Signature help (active-parameter tracking)
- Identifier completion (locals + top-level + sibling-package decls + keywords + types)
- `intentc lsp` stdio subcommand; first-party VS Code extension (`editors/vscode/`)
- Out of scope (v1.1+): member completion (.field/.method), find-references, rename, code actions, refactorings, semantic tokens, inlay hints, cross-package go-to-definition, Marketplace publishing, incremental sync, multi-byte UTF-16, per-contract Z3 source positions

### REPL / Playground
- Interactive expression evaluation with contract checking
- Web-based playground for sharing Intent snippets

### Linter Enhancements
- Complexity warnings (deeply nested control flow)
- Unreachable code detection
- Contract completeness hints

### Optimization Levels
- `--release` flag to strip unverified contract assertions
- Keep verified contracts as documentation, remove runtime checks
- Configurable per-contract: critical invariants always checked

---

## Driving Example: Attractor

The Attractor pipeline orchestration spec (`examples/attractor/`) serves as the primary driver for language development. By implementing the spec in Intent, we discover which features matter most in practice.

**Current state:** Phase 1 complete — type model, edge selection, retry policy, graph validation all compile and run. See `examples/attractor/STRATEGY.md` for the full gap analysis and phased plan.

**Features driven by Attractor (all complete):**
1. [x] String standard library (split, lowercase, trim, starts_with) — unlocks condition parsing
2. [x] Array\<String\> on entity fields — unlocks suggested_next_ids, reachability
3. [x] Map\<K,V\> type — unlocks Context (the central state-passing mechanism)
4. [x] Result\<T,E\> and error handling — unlocks retry loop exception handling
5. [x] Traits/Interfaces — unlocks Handler dispatch

The remaining Attractor-driven work is async handler integration (Phase 12 stretch goal, deferred — see `ops/plans/phase-14-phase11-13-gaps.md`).

---

## Priority Order

The milestones are ordered by strategic importance:

1. **Milestone 4 (IR)** -- architectural foundation for everything else
2. **Milestone 5 (Z3)** -- the differentiator; without this, Intent is
   "Rust with mandatory asserts"
3. **Milestone 6 (Multi-target)** -- broadens addressable use cases
4. **Milestone 7 (Language)** -- deferred until foundations are solid
5. **Milestone 8 (DX)** -- important but not blocking

Milestones 4 and 5 are the critical path. If Intent can statically verify
contracts at compile time and target multiple platforms, it occupies a
unique position that no existing tool matches.

---

## Dependency Graph

```
Milestone 4 (IR)
    |
    +---> Milestone 5 (Z3 Verification)
    |         |
    |         +---> Milestone 7 (Language Evolution)
    |                   |
    +---> Milestone 6 (Multi-Target)
    |         |
    |         +---> Milestone 8 (DX / LSP)
    |
    +---> Phase 4.1 (Tooling fixes -- can start immediately)
```

Phase 4.1 (formatter, bug fixes) has no dependencies and can start now.
Milestones 5 and 6 both depend on Milestone 4 but are independent of each
other and can be developed in parallel once the IR is stable.
