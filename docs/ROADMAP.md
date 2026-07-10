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

Design lives in `prds/done/phase-16-testing-framework.md` (Shipped). 10 commits 95c545f..f02bf13.

### Phase 17: Testing Framework Polish -- SHIPPED (2026-05-30)
- [x] 17.B: divergence demo + `@target_specific` annotation (ADR 0031) — full pipeline (lexer/parser/AST/checker/IR/runner) with `SkipAnnotation` distinct from WASM-rejection skip
- [x] 17.C: tests for all 19 flat examples
- [x] 17.D: cross-package test discovery (ADR 0030)
- [x] 17.F: `--filter`, `--list`, `--quiet` DX flags
- [x] 17.A: testgen migration to `--target intent` — entity/method emission (Phase 27), multi-param iteration (Phase 28), legacy Rust path retired (Phase 29)
- [ ] 17.E / 17.G / 17.H: deferred (future PRDs) with documented rationale

Design + execution record: `prds/done/phase-17-testing-polish.md`.

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

### Phase 54: Multi-file Self-Hosted Checking -- SHIPPED (2026-07-03)
`intentc check --self-hosted` used to check a *single* file, so it flooded false positives on the compiler's own multi-file source (imported types → `unknown type`, module-name call qualifiers like `shared_lexer.foo()` → `undeclared variable`). This — valid code rejected — was the real self-hosting blocker for the checker, invisible to the single-file `diff-checker` gate. Fixed per [ADR 0058](decisions/0058-self-hosted-checker-cross-module-resolution.md): a Go harness (`stage2CheckPaths`) reuses the module registry to discover the import closure and passes the entry + all module paths to the stage2 binary; `check_main.merge_programs` flattens them into one `Program` (cross-module dedup, entry verbatim); `check_program_seeded` seeds imported module names so qualifiers resolve. `check_program` stays a thin single-file wrapper, so single-file + test output is byte-identical. New `make selfcheck-checker` gate — the stage2 checker matches stage1 on all core self-hosting modules. Deferred non-goal: multi-file *error*-position/path parity and stage1's per-module import scoping (the flat merge gives broader visibility) — fine for the valid-source self-hosting target ("No errors found"). See `prds/progress.md`.

### Phases 48 & 53: Self-Hosted Checker — Expression Inference & Type-Rule Checks -- SHIPPED (2026-07-02..10)
Phase 48 ([ADR 0056](decisions/0056-self-hosted-expression-inference.md)) added a **sound-but-incomplete** `infer_expr_type` — it returns a `Type` only when certain, else an `Unknown` sentinel, and each type-rule check fires only on a confident result, so every check stays corpus-safe as inference grows. On that engine it shipped the type-rule checks the corpus exercises: condition-boolean, `let`-mismatch, function/variant argument typing, assignment mismatch, `self`/field access, method-call arity + arg-types, binary-operator typing, contract well-typedness, the full match-expression checks, builtin argument typing, and the async-context checks for `await`/`await_*`/`timeout` ([ADR 0057](decisions/0057-self-hosted-checker-async-context.md) — the async flag threaded on the `Scope` rather than a ~40-site parameter). The originally-planned Phases 49-52 (type-carrying scope, operator/assignment typing, arg-typing/method-arity, return/match/contract typing) were **superseded** by these slices. The closing tail (2026-07-10) added: spawn/try operand name-recursion; the **async-test-no-await warning** (introducing warning-severity support to the checker + `intentc check`'s dual warning output); **full assert_eq comparable-set parity** (eq-method signature sub-checks + Map/Future rejection + generic recursion, via new `type_to_string`/`type_equal`); **unary operator typing**; **method-call return-type inference** (user-entity methods, with bare type-param substitution through the receiver's type args); **contract-clause name/type recursion** (`requires`/`ensures`/`invariant` clauses recursed for undeclared-var/argument/operator errors, with `result`/`old()` handled as contract keywords); plus the Phase 53 siblings **entity has-no-constructor** and **extern param/return unknown-type**. Gate: `make diff-checker` **110/110** byte-equal (valid corpus with zero false positives + invalid fixtures), ~311 in-language checker tests, all self-check / formatter / linter / emit-sweep gates + `make validate` green. **Phase 58** then added impl-block-method contracts and the immutable-target checks (assign / index-assign / push / set / remove, via per-binding mutability in the `Scope`). A residual set of **sound false negatives** — built-in-method return typing, extern FFI-bridgeability messages, the module-qualified has-no-constructor variant, and the `@target_specific("wasm")` warning — is catalogued in `prds/backlog/prd-phase-58-checker-parity-tail.md`; none emit a wrong diagnostic or fire on valid code. PRD: `prds/done/prd-phase-48-expression-inference.md`.

### Phase 47: Self-Hosted Checker — Builtin-Call Arity -- SHIPPED (2026-07-02)
Closed the builtin-call arity gap deferred from Phase 45. The self-hosted checker now arity-checks all 23 stage1 builtins (`print`, `assert`/`assert_eq`/`assert_close`/`assert_panics`, `len`, `read_file`/`write_file`/`create_dir`/`file_exists`, `env_get`, `args`, the HTTP/JSON/event builtins, `timestamp_ms`, `sleep`, `char_from_codepoint`, the `await_*`/`timeout` async builtins) against a name→count table, emitting stage1's exact message in its three shapes — `NAME() expects N argument(s)` (the assert family), `NAME() requires exactly N argument(s)`, and `NAME() takes no arguments` (`argument` singular at N=1). Strategic frame: [ADR 0055](decisions/0055-self-hosted-builtin-arity.md). Arity (argument count) is separable from argument *typing*, which needs the expression-inference engine (Phase 48) — so this is a bounded, no-inference, fully byte-equal parity step; the deferred per-argument type + `await_*` async-context checks are false negatives that never fire on the valid corpus. Checked first in `check_expr_names` (matching stage1 `checkCallExpr`'s builtin-before-variant/function precedence), anchored at the callee, early-returning on mismatch (a wrong-arity builtin's args are not also flagged undeclared). Gate results: `make diff-checker` **44/44** (22 valid examples clean + 22 invalid fixtures, incl. one per message shape, byte-equal), **188** in-language checker tests, all formatter/linter/self-check gates + full Go suite green. `Ok`/`Err`/`Some`/`None` remain on the variant-constructor path (unchanged). Method-call arity (needs the receiver type) and builtin argument typing move to Phase 48. PRD: `prds/done/prd-phase-47-builtin-arity.md`.

### Phase 46: Self-Hosted Checker — Type Foundation + `unknown type` -- SHIPPED (2026-07-02)
Built the self-hosted checker's type-system foundation and its first type-aware diagnostic. Strategic frame: [ADR 0053](decisions/0053-self-hosted-checker-type-foundation.md). A structured `Type` tree (`name` + `type_args` + an `Fn(...)->R` shape) with `parse_type(s)` that parses the flat type strings the AST already carries (`Array<Map<String, Int>>`, `Fn(Int) -> Int`), and a `type_is_known` resolver that faithfully ports stage1 `ResolveTypeWithParams` (primitives, the fixed-arity built-in generics incl. Map's unhashable-key rejection, `Fn`, entity/enum names, in-scope generic type params, recursing into args). Per ADR 0053 D1 this needs **no structured types in the AST** — the checker re-parses the strings it already receives, keeping the formatter gates untouched. The **`unknown type 'X'`** check is then wired into `check_program` across **every annotation site the corpus uses** — function params/returns, entity fields, entity methods, `let` statements, and enum-variant fields — each byte-equal with stage1 including subtle emit-order and anchor details (the name reported is the *outer* ref, so `Array<Widget>` → `unknown type 'Array'`; the `registerEnums`-before-`registerEntities` quirk means an entity-typed enum-variant field still errors). One additive front-end change was needed and is recorded in [ADR 0054](decisions/0054-additive-ast-positions-for-diagnostics.md): `FieldDecl` gained `line`/`column` (populated from the `field` keyword, matching stage1) — additive positions are permitted (distinct from ADR 0053 D1's ban on *structured types*: the formatter reconstructs type strings from a tree but never reads positions, so they are inert; precedent: Phase 45.7's `Expr` positions). Two latent bugs surfaced during the work and were fixed: the Rust backend now borrows an owned-temporary (e.g. a call result) passed to an `Array`/`Map` (`&Vec`) parameter instead of emitting E0308, and the `intentc test` runner now surfaces a swallowed Rust compile error instead of a bare "did not run". Gate results: `make diff-checker` **41/41** (22 valid examples with zero false positives + 19 invalid fixtures byte-equal), **183** in-language checker tests (rust+js), and `selfcheck-formatter` 4 EQUAL / `diff-formatter` 22/22 / `diff-linter` 26/26 / full Go suite held throughout. Deferred: expression type inference and all type-rule checks (assignability, operators, generics substitution, match exhaustiveness, contracts) to **Phase 47**; extern param/return `unknown type` (0 corpus usage) and method/builtin call arity remain small tracked gaps. PRD: `prds/done/prd-phase-46-checker-type-foundation.md`.

### Phase 45: Self-Hosted Checker (first slice) -- SHIPPED (2026-06-26)
The first *compiler* subsystem self-hosted in Intent: a `selfhost/checker/` sibling reusing the `../shared/` front-end, byte-equal with stage1 `intentc check` across the examples corpus + invalid fixtures (`make diff-checker` **34/34**: 22 valid examples produce zero errors — no false positives — and 12 invalid fixtures match byte-for-byte). Wired as `intentc check --self-hosted`. Strategic frame: [ADR 0052](decisions/0052-self-hosted-checker-strategy.md). The Go checker is ~4,281 LOC / ~167 diagnostics with a full type system; this first slice deliberately ships the checks needing **no type inference**: duplicate declaration (entity/enum/function/trait), duplicate enum variant, break/continue-outside-loop, return-in-test, undeclared-variable + variable-redefinition (via an `Array`-based scope stack — no `Map`, no recursive entity field), and function/variant call arity. Two front-end prerequisites surfaced and were landed gap-driven: real **break/continue statements** (45.4 — stage2 had lexed them as identifiers) and **source positions on `Expr`** (45.7 — so `undeclared variable` anchors at the identifier). The differential is two-directional — invalid fixtures byte-equal vs stage1 PLUS a no-false-positives sweep over the 22 valid examples (which surfaced and fixed an enum-variant-in-scope gap). Deferred to later phases: all type inference (assignability, operators, generics, traits, match exhaustiveness, contracts), method-call arity, builtin-call arity, and multi-file `CheckAll`. ~150 in-language checker tests; the formatter/linter gates (`make selfcheck-formatter` 4 EQUAL, `diff-formatter` 22/22, `diff-linter` 26/26) held throughout. PRD: `prds/done/prd-phase-45-self-hosted-checker.md`.

### Phase 44: selfhost/shared Restructure -- SHIPPED (2026-06-26)
Pure refactor that split the shared stage2 front-end into `selfhost/shared/` and made `formatter/`, `linter/`, (and the upcoming `checker/`) siblings importing it via `../shared/…` — the split [ADR 0050](decisions/0050-self-hosted-linter-strategy.md) D1 deferred until a third stage2 tool (the checker) was about to land. Strategic frame: [ADR 0051](decisions/0051-selfhost-shared-restructure.md). Moved `lexer.intent`/`ast.intent`/`parser.intent` to `selfhost/shared/` (modules renamed `formatter_{lexer,ast,parser}` → `shared_*`, ~601 qualified refs); relocated the linter to `selfhost/linter/` (modules → `linter`/`linter_test`/`lint_main`); re-pointed every consumer to `../shared/…`; updated `selfcheck.sh`/`difftest.sh`/`difftest-lint.sh`, the Makefile, and the Go shims (`stage2LinterBinary` path + both shims' cache-staleness checks now also scan `selfhost/shared/`). A 44.2 spike first confirmed `../`-relative cross-directory imports build on rust+js (no stage1 change needed; `registry.go:509` resolves them). Zero behaviour change: `make selfcheck-formatter` 4 EQUAL, `make diff-formatter` 22/22, `make diff-linter` 26/26, full Go suite + stage2 suites (207 formatter / 188 linter) all green; `fmt`/`lint --self-hosted` byte-identical to before. PRD: `prds/done/prd-phase-44-selfhost-shared-restructure.md`.

### Phase 43: Self-Hosted Linter -- SHIPPED (2026-06-24)
The second self-hosted toolchain artefact (after the formatter), and the proof point named in HARNESS.md §7. Reimplements the Go linter (`internal/linter/`) in Intent, reusing the stage2 lexer/parser/AST in `selfhost/formatter/`, and drives it to **byte-for-byte parity with stage1 `intentc lint` across the entire examples corpus plus dedicated fixtures (`make diff-linter` = 26/26 PASS, 0 diverged)**. All **16 Go-linter rule families** are ported: missing-contracts (function/method/trait-method/extern), naming (snake_case functions/methods, PascalCase entities/enums/variants), empty-body, entity-without-invariant, intent-without-verified_by, unused variable / parameter / type-parameter, mutable-never-reassigned, and discarded-spawn. Strategic frame: [ADR 0050](decisions/0050-self-hosted-linter-strategy.md) (reuse the formatter dir; byte-equal parity *including* the column; all rules gated by the differential).
- **43.1 ADR 0050** — strategy: faithful port, reuse `selfhost/formatter/`, byte-equal-with-column, all-16-gated-by-diff. Corpus baseline measured at 76 warnings / 13 files / 8 of 16 families.
- **43.2 Column tracking** — added `column` (and `line` where missing) to the stage2 AST nodes the linter anchors on (decls, `Stmt`, `Param`, `EnumVariant`, `TraitMethodSig`), defaulted in the constructor body and assigned post-construction from the leading token. Diagnostics are emitted in append order (stage1 does not sort), so per-decl rule order is load-bearing.
- **43.3-43.9 Rules** — `lint.intent` (module `formatter_linter`): `LintDiag` model, stage1-order dispatch walk, `format_diags` (`warning[file:line:col]: message`), `is_snake_case`/`is_pascal_case` ported from the Go code, a functional used-name/assigned-name engine (stage2 has no assignment statement — `x = y` is a `st_expr` wrapping an `ex_binop "="`), and all 16 rules across `lint_function_decl` / `lint_entity_decl` / `lint_impl_decl` / `lint_enum_decl` / `lint_trait_decl` / `lint_extern_decl` / `lint_intent_block`. Type-param rules required additive parallel position arrays (`type_param_lines`/`columns`) + a token-aware `type_uses_param`. R3/R4 fire on every parseable trait-method/extern — correct because the stage2 parser only accepts contract-less forms.
- **43.10 `lint_main.intent`** — runnable entry; output byte-identical to `intentc lint` (verified on the high-warning files map_demo=18, task_queue=17, enum_basic=12).
- **43.11 `intentc lint --self-hosted`** — Go shim mirroring `fmt --self-hosted` (`INTENT_STAGE2_LINT` override + build/cache; stdout emitted verbatim, no fallback on stage2 failure).
- **43.12 Differential gate** — `make diff-linter` (`difftest-lint.sh` + `lint-fixtures/`) compares stage2 vs stage1 across corpus + fixtures; 26/26 PASS. R4 (extern) is unit-test-only: stage1 (`extern function … from "path"`) and stage2 (`extern "target" function …`) have incompatible extern syntax, so no shared fixture exists.
- 269 in-language linter tests on rust + js; the stage2 formatter's self-format fixpoint (`make selfcheck-formatter`) and corpus parity (`make diff-formatter` 22/22) are preserved throughout. PRD: `prds/done/prd-phase-43-self-hosted-linter.md`.

### Phase 42: Stage2 Formatter CLI Wiring + Differential Test -- SHIPPED (2026-06-16)
Turned the stage2 formatter from a test-only library into a runnable, CLI-wired tool, then drove it to **byte-for-byte parity with stage1 `intentc fmt` on the entire examples corpus (22/22, 0 divergences, 0 parse errors)**.
- **42.1 `args()` builtin** ([ADR 0045](decisions/0045-args-builtin.md)) — the command-line-argument primitive self-hosted tools need. `args() -> Array<String>`, program/script name at index 0 (Rust+Go convention). Rust `std::env::args().collect()`, JS `process.argv.slice(1)` (normalized so the script is index 0), WASM stub.
- **42.2 `main.intent`** — the stage2 formatter as a runnable program: reads `args()[1]`, parses, prints `format_program(...)` (exit 0/1/2/3). Surfaced and fixed a latent stage1 multi-file bug: an `entry` function in an *imported* module emitted a duplicate `main` — gated the entry wrapper on `f.IsEntry && g.isEntryFile` (rustbe + jsbe; +2 regression tests).
- **42.3 differential harness** — `selfhost/formatter/difftest.sh` + `make diff-formatter`. Canonicalize-first (stage1-`fmt` a copy, then check stage2 reproduces it byte-for-byte) = a true comparison against `intentc fmt`. **Zero true divergences throughout** — whenever the stage2 parser accepts a file, the formatter matches stage1 exactly, so every gap was pure parser coverage.
- **42.4 `intentc fmt --self-hosted`** — Go shim delegating to the stage2 binary (env `INTENT_STAGE2_FMT` override or auto-build-with-cache). Composes with `--check`; a stage2 parse error exits non-zero with no stage1 fallback. Exec layer unit-tested with fake binaries (no cargo).
- **Parser-gap closing (42.5–42.11)**, each round-tripping byte-equal and preserving self-format: entity `invariant <expr>;` clauses (+ constructor contracts + `intent "..." { verified_by: [...] }` blocks); `implies` operator; `await` (+ `spawn`, `async test`); `forall`/`exists` quantifiers; generic declaration type params `<T>` + generic instantiation `Stack<Int>()`; `Fn(...) -> T` types + lambdas `|x: T| -> R => e` (new `->` token); test attributes `@name("...")`.
- **Self-format verification — built-binary gate** (`selfcheck.sh` + `make selfcheck-formatter`): discovered that the in-language byte-equal gate cannot run reliably under `intentc test`, because `cargo test`/libtest executes each test on a small (~2 MB) thread stack that overflows on the deep recursive-descent parse of the 95 KB `parser.intent` (non-deterministic abort). The real stage2 binary (8 MB main thread) self-formats every file fine, so the authoritative gate drives the built binary. (No codegen stack change was needed — the "stack blocker" was a verification-method artifact.)

Result: **22/22 examples agree with `intentc fmt`**, the stage2 formatter remains a byte-equal fixpoint on all four of its own source files, and `intentc fmt --self-hosted` is a working CLI. Stage2 suite 203/203 on rust + js. PRD: `prds/done/phase-42-formatter-cli-differential.md`.

### Phase 41: Stage2 Parser Surface Widening -- SHIPPED (2026-06-15)
With byte-equal self-format banked (Phase 40), the stage2 parser still only handled the restricted subset its own source uses. Phase 41 widens it to the constructs it previously skipped or rejected, each round-tripping through parse + format (byte-equal self-format preserved throughout):
- **41.1 Contracts** — `requires` / `ensures` / `decreases` on functions and methods were *silently discarded* by a crude skip (and `skip_method_contracts`, now removed). `FunctionDecl` gains `requires_clauses` / `ensures_clauses` / `decreases_clauses`; the clauses lex as plain idents with no `;` terminator (parse_expr stops at the next clause keyword or `{`); the formatter emits them between the signature and the body in stage1's canonical layout (signature line, indented clause lines, `{` on its own line). `result` round-trips as an identifier.
- **41.2 match** — `match <scrutinee> { <pattern> => <body>, ... }`. New `match` keyword; `ex_match` kind + `MatchArm` entity (variant / bindings / is_wildcard / body) + `Expr.match_arms`. Patterns: `_`, `Variant`, `Variant(a, b)`. Because match is multi-line and indent-dependent, the let/return/expr statement formatters route their expression through a new `format_expr_indented(e, level)` that emits matches with arms at `level+1` and the closing brace at `level`; nested matches recurse.
- **41.3 for-in** — `for <var> in <iter> { ... }`. New `st_for` statement kind reusing `Stmt` fields (name = loop var, expr = iterable, then_block = body); `for` reuses the existing `kw_for_marker` token (shared with `impl ... for ...`, disambiguated by statement position); `in` lexes as a plain ident.
- **41.4 try** — postfix `inner?`. New `ex_try` kind in the postfix loop; precedence 10; lower-precedence inners keep their parens.

12 new tests (170/170 on rust + js). A stage1 Rust-backend gap surfaced and was worked around: an Intent identifier named `fn` (a Rust keyword) emits invalid Rust — the backend doesn't escape reserved words (renamed the param; noted in `prds/progress.md`). PRD: `prds/done/phase-41-parser-surface-widening.md`.

### Phase 40 / 40A.2 / 40A.3: Byte-Equal Self-Format -- SHIPPED (2026-06-15)
The milestone the whole Phase 40 line was building toward: the stage2 formatter **byte-equal self-formats all four of its own source files** (`selfhost/formatter/{lexer,ast,parser,format}.intent`) — `format(parse(src)) == src`. Delivered across two sub-phases after 40A.1.

**40A.2 — remaining comment positions on the statement/decl surface:** trailing-EOF comments (`Program.trailing_comments` from the synthetic EOF token); body / between-statement comments (`Stmt.comments_before` captured per-statement in `parse_block`); inline-after comments on statements (`stmt; // c`) — the lexer learns same-line detection (`saw_newline_since_token` + `pending_inline_after`, attaching to the previous token's new `Token.comment_after`; `scan_all` restructured to hold a prev-token local since post-push array-element mutation is unreliable in the stage1 backend), `Stmt.comment_after` from the `;` token, formatter appends with a single canonical space. Plus a comprehensive synthetic round-trip test.

**40A.3 — entity/module/block comment positions + the real blocker:** module-leading comments (`ModuleDecl.comments_before` from the module token); comments before entity fields / methods / constructors and impl methods (`FieldDecl.comments_before`, reusing `FunctionDecl.comments_before`); end-of-block comments before `}` (`Block.trailing_comments` from the rbrace token); inline-after on fields (`FieldDecl.comment_after`) and on declaration closing braces (`Block.brace_comment_after` + `brace_trail()` — this caught the formatter silently dropping one-liner doc-comments like `ex_void() ... } // placeholder`). The real byte-equal blocker turned out to be **generic types**: `parse_type_name` discarded type arguments and emitted the placeholder `Array<...>` (a Phase 33 shortcut), so it now reconstructs args verbatim with canonical spacing (`, ` after commas; nested `<>` via a depth counter; `>>` = two `>`). Finally the four source files were **canonicalized** (reformatted by the formatter, making it a fixpoint) — they still compile, self-parse, and pass the full suite. `self_format_one` asserts byte-equality. 158/158 on rust + js.

Key lesson (recorded in `prds/progress.md`): **idempotence is not losslessness** — the first canonicalization attempt would have permanently dropped inline doc-comments and corrupted every generic type into `Array<...>`; both were caught by validating the reformatted files before committing. ADR 0044; PRD: `prds/done/phase-40a-comment-preservation.md`.

### Phase 40A: Comment Preservation (Leading Decl Comments) -- SHIPPED (2026-06-09)
Third of three sub-pieces toward byte-equal self-format. The stage2 lexer used to drop comments in `skip_whitespace_and_comments`. Token entity gains `comments_before: Array<String>` (defaulted empty in the constructor body so existing 41 Token construction sites don't need updating); Lexer gains a `pending_comments` accumulator that captures both `//` and `/* */` verbatim and is drained onto each non-trivia token by `scan_all`. Nine top-level decl entities (ImportDecl, FunctionDecl, EntityDecl, EnumDecl, TraitDecl, ImplDecl, IntentBlock, TestDecl, ExternDecl) gain `comments_before: Array<String>`, populated by the parser. `parse_program` captures the iteration's first-token comments BEFORE consuming any `public` modifier so the `public entity Foo {...}` path doesn't strand comments on the `public` token; comments are passed to each `parse_*_decl` via a new `leading_comments` parameter. Formatter's `format_comments_before` helper emits each comment on its own line; the k-way merge dispatcher prepends each decl's comments to its emission. Stage1 fix in-phase: `let x: T = self.field` for non-Copy T now appends `.clone()` (extends the prior IndexExpr fix in `internal/rustbe/rustbe.go`); regression test `TestLetBindingClonesFieldAccessOfNonCopyType`. 6 new comment tests + 2 existing constructor calls updated for the new ImportDecl signature. Combined 131/131 on rust + js. ADR 0044 documents the choice of token-attached comments (over comment-token-stream and over AST-attached). v1 scope is **leading comments on top-level decls only**; inline-after comments (`let x = 1; // ...`) and comments inside function bodies are deferred to Phase 40A.2. PRD: `prds/done/phase-40a-comment-preservation.md`; ADR: `docs/decisions/0044-stage2-comment-preservation.md`.

### Phase 40B: Precedence-Aware Paren Stripping -- SHIPPED (2026-06-09)
Second of three sub-pieces toward byte-equal self-format. `format_expr` becomes a precedence-aware emitter: a `binop_precedence(op)` table (mirroring `parser.intent`'s Pratt parser), an `expr_precedence(e)` helper for arbitrary expressions, and a three-layer dispatch (`format_expr` → `format_expr_at(e, min_prec)` → `format_expr_inner(e)`). `ex_paren` is stripped at the `format_expr_at` layer: inner expression is forwarded with the same `min_prec` and a redundant paren around a sufficiently-binding inner expr disappears. Bump rule for associativity: left-assoc binops recurse into LHS at `min_prec = P` (accepts same-prec chains naturally) and into RHS at `P + 1` (keeps necessary `a - (b - c)` parens); right-assoc (`=`) swaps. Call args / index inner / array literal elements / paren content all recurse with `min_prec = 0` (fresh context — they're inside parens). 8 new tests lock the matrix: `(x)` → `x`, `(1 + 2) * 3` preserved, `1 + (2 * 3)` → `1 + 2 * 3`, `(a + b) + c` → `a + b + c`, `a - (b - c)` preserved, `(a == b) and (c == d)` → `a == b and c == d`, `not (a or b)` preserved, `f((x + y))` → `f(x + y)`. Combined 125/125 on rust + js. Conventional pretty-printer recipe (Wadler / Hughes); chosen over an AST-canonicalisation pre-pass to keep `ex_paren` in the AST for future linter / LSP diagnostics. ADR: `docs/decisions/0043-stage2-paren-stripping.md`; PRD: `prds/done/phase-40b-paren-stripping.md`.

### Phase 40C: Source-Order Tracking for Top-Level Decls -- SHIPPED (2026-06-09)
First of three sub-pieces toward byte-equal self-format. Phase 36's parallel-array `Program` lost cross-kind source order; the Phase 38 formatter emitted in a fixed canonical order (module → imports → functions → entities → ...) which happened to match `hello.intent` but reorders any program that interleaves decl kinds. This phase adds a `line: Int` field to all nine top-level decl entities (ImportDecl, FunctionDecl, EntityDecl, EnumDecl, TraitDecl, ImplDecl, IntentBlock, TestDecl, ExternDecl), populated by the parser from each decl's leading keyword token. `format_program` rewritten as an inline k-way merge: nine index pointers + a min-by-line pick at each step. Per-kind arrays stay sorted by line (parser appends in source order), so a single pass suffices. Two new tests lock the behaviour: a roundtrip on interleaved `function, test, function, test` source asserts the format preserves that order (vs grouping functions then tests as before), and a direct assertion that the parser populates `FunctionDecl.line` from the source line. Stage2 stays self-parseable — `format_program` uses `while not done { ... }` instead of `break` (which the stage2 parser doesn't yet handle) so the file continues to satisfy Phase 39's self-parse certification. Per-decl `line: Int` chosen over moving `Program` to a discriminated union (ADR 0042): the diff is mechanical (one field added to nine entities), the call-site impact is zero outside parser and formatter, and the discriminated-union restructure remains available as a future refactor. Combined 117/117 on rust + js. ADR: `docs/decisions/0042-stage2-source-order-tracking.md`; PRD: `prds/done/phase-40c-source-order-tracking.md`.

### Phase 39: Self-Parse Certification -- SHIPPED (2026-06-09)
The "intent with intent" milestone. Five new in-language tests (4 self-parse + 1 self-format roundtrip) certify that stage2 parses and formats all four of its own source files: `lexer.intent` (610 LOC), `ast.intent` (400 LOC), `parser.intent` (1740 LOC), `format.intent` (450 LOC). All four parse with `prog.error == ""` and the full parse + format pipeline runs without crashing — proof that the structural grammar implemented across Phases 32-38 is closed under self-application. Byte-equal self-format is gated on comment preservation + paren-stripping + source-order tracking (Phase 40+) and is explicitly out of scope here; the format-side assertion is `len(out) > 0`. Combined 115/115 on rust + js. The phase surfaced one stage1 Rust-backend gap, fixed in-line: the builtin I/O calls (`read_file`, `write_file`, `create_dir`, `file_exists`, `env_get`) skipped `cloneIfNeeded` on their string argument, so reading via an indexed-array element (`read_file(paths[0])`) produced an `E0507` move-out-of-Vec. Six-line patch in `rustbe.go` matches the convention already used for regular function calls; regression test `TestBuiltinIOClonesIndexedStringArg`. PRD: `prds/done/phase-39-self-parse-certification.md`.

### Phase 38: Stage2 Formatter MVP -- SHIPPED (2026-06-09)
Seventh stage2 deliverable — closes the AST → source round-trip. `selfhost/formatter/format.intent` walks `ast.intent`'s `Program` and emits canonical Intent source: module declaration, imports, functions (with `public` / `entry` / `async` modifiers, params, return type, body), constructors, methods, entities, enums (unit + data variants), traits, impls, intent blocks, tests, externs. Statements (let / return / if-else / while / expression statement) and expressions (int / float / char / string / bool / ident / unary / binop / call / index / field / array / range / paren) all round-trip. Decl-kind output order is fixed (module → imports → functions → entities → enums → traits → impls → intent_blocks → tests → externs) since `Program` lost cross-kind source order in Phase 36. Tests in a separate sibling `selfhost/formatter/format_test.intent` (the file count made the per-file test approach awkward at ~100 tests): 8 unit tests for individual emitters + 7 synthetic round-trip tests covering each statement kind + 1 char-literal escape unit + 1 **real-file dogfood** that `read_file`s `examples/hello.intent`, parses, formats, and byte-compares — passes from the repo root and silently skips elsewhere. Combined 110/110 on rust + js. Surfaced a stage1 Rust backend bug fixed in-phase: call sites passing struct-field or indexed-element `Array<T>` to a function with an `Array<T>` parameter emitted `obj.field.clone()` (owned `Vec`) where the function signature was `&Vec<T>`, producing `E0308`. Fix in `internal/rustbe/rustbe.go` extends array-ref coercion from `*ir.VarRef` only to `{VarRef, FieldAccessExpr, IndexExpr}`. Regression test `TestArrayParamFieldAccessCallBorrows` locks the fix. PRD: `prds/done/phase-38-stage2-formatter-mvp.md`.

### Phase 37: Stage2 Lexer Extensions (Char + Float + Block Comments) -- SHIPPED (2026-06-09)
Sixth stage2 deliverable. Fills the remaining syntactic gaps in `selfhost/formatter/lexer.intent` needed before the formatter can be written. Adds **char literals** (`'a'`, `'\n'`, `'\u{...}'` — raw lexeme preserved in `Token.text`), **float literals** (`3.14`, `1.5e10`, `2.5e-3` — promoted from `scan_int_literal` only when `.<digit>` follows, so `1..5` stays as range and `42.foo` stays as method call), and **multi-line `/* ... */` comments with nesting** (depth-counter in `skip_whitespace_and_comments`). New token kinds `tk_float` and `tk_char` wire into the expression parser as new primary kinds via `ex_float` and `ex_char` in `ast.intent`. Both AST nodes store the raw lexeme in `str_value` — formatter Phase 38 only needs to round-trip, so no in-Intent `parse_float` or char-escape decoder this phase. 14 new in-language tests (12 lexer + 4 parser) including a Phase 37 dogfood that parses a small program containing all three new token kinds end-to-end. Combined 93/93 on rust + js. Conservative call on bare-exponent literals: `1e5` stays as int + ident (no decimal required → no float promotion) — locks the chosen behavior with explicit tests, avoids silently re-tokenising any pre-existing Intent source. Real-file self-parse dogfood (reading `lexer.intent` from disk) deferred to Phase 38, when file I/O surfaces as a need for the formatter. PRD: `prds/done/phase-37-stage2-lexer-extensions.md`.

### Phase 36: Top-Level Declarations in Intent + AST Split -- SHIPPED (2026-06-03)
Fifth stage2 deliverable. Fills in the rest of the declaration grammar so the parser can ingest the structural surface of any Intent program: entities (with fields, constructors, methods, invariant blocks), enums (unit + data-carrying variants), traits (method signatures), impls (trait + entity + method bodies), test declarations, extern declarations, intent blocks. Also splits the AST entities out of `parser.intent` (which had grown to 1404 LOC at the end of Phase 35) into a new sibling `selfhost/formatter/ast.intent`. Stage2 lexer gains 12 new keywords (`entity`, `enum`, `trait`, `impl`, `test`, `extern`, `intent`, `field`, `constructor`, `method`, `invariant`, `for`), all with `_marker` suffix on the kind functions following the Phase 33 precedent. `FunctionDecl` extended with `is_constructor: Bool` so the same entity represents free functions, methods, and constructors. `Program` expanded to 10 parallel arrays per declaration kind. 14 new tests + the existing 60; combined 74/74 on rust + js. Key surfaced gaps (workarounds applied): (a) cross-module free-function calls require module qualification (`formatter_ast.empty_block()`); entity constructors don't — bulk-qualified ~100 call sites; (b) entity-typed method parameters are passed by value in the Rust backend so mutations don't propagate — inlined the `public` dispatch in `parse_program` instead of factoring a helper; (c) reserved-word collisions on intuitive field names (`goal` / `verified_by` / `result`) — renamed. Full self-parse dogfood test is gated on stage2 lexer extensions (char + float literals + multi-line comments) — Phase 36 substitutes a synthetic fixture exercising entity + constructor + method + test. PRD: `prds/done/phase-36-top-level-decls-in-intent.md`.

### Phase 35: Expression Parser in Intent -- SHIPPED (2026-06-03)
Fourth stage2 deliverable. Replaces Phase 34's depth-balanced raw-text expression capture with a real Pratt / precedence-climbing parser producing an `Expr` AST: Int-tagged kind (`ex_int`, `ex_string`, `ex_bool`, `ex_ident`, `ex_unary`, `ex_binop`, `ex_call`, `ex_index`, `ex_field`, `ex_array`, `ex_range`, `ex_paren`, plus `ex_void` for `return;`) with scalar payload fields and a `children: Array<Expr>` for sub-expressions. Mutual-recursion safety: every back-edge to `Expr` goes through `Array<Expr>` (heap-allocated). Grammar layered top-down: `assign > or > and > eq > cmp > range > add > mul > unary > postfix > primary`. Assignment (`=`) is parsed as the lowest-precedence right-associative binop so `y = y + 1;` and chained `a = b = c;` both work uniformly. Postfix loop handles call / index / field-access; method calls fall out naturally as `call(field(obj, "m"), args...)`. Qualified paths like `formatter_lexer.tk_eof()` parse without special-casing. `Stmt.expr_text: String` becomes `Stmt.expr: Expr`; all five statement parsers updated. `parse_int` implemented in-Intent using `Char.to_codepoint` + `Char.is_digit` — no `String.to_int()` builtin needed. 22 new tests + all Phase 34 tests rewritten to assert against the AST structure; combined 60/60 on rust + js. Zero Go / backend changes — Phase 31-33's groundwork (Char ergonomics, `&mut self` propagation, constructor-field-hoist) carried this phase end-to-end. PRD: `prds/done/phase-35-expression-parser-in-intent.md`.

### Phase 34: Statement Parser in Intent -- SHIPPED (2026-06-03)
Third stage2 Intent-in-Intent artefact. Replaces Phase 33's raw-text body capture with a real statement-level AST: `Block`, `Stmt` (Int-discriminated, mirroring `Token.kind`), and parse methods for `let` (annotated + inferred, `mutable` flag), `return` (with or without value), `if`/`else` (including `else if` chains folded into single-statement nested else-blocks), `while`, and bare expression statements. Expressions are still captured as raw token-joined text via depth-balanced bracket scanning — a real expression parser is Phase 35. `FunctionDecl.body` changed from `body_text: String` to `body: Block`. 13 new in-language tests; combined with Phase 32 lexer + Phase 33 top-level parser that's 38/38 passing on rust + js. One small gap surfaced and worked around in-language: empty `Block([])` literals at call sites can't infer the element type, fixed via a typed `empty_block()` helper. No backend / language changes were required this phase — Phase 33's constructor-field-hoist fix already handled the new entities cleanly. Rationale for sticking with a single Int-tagged `Stmt` entity over a sum-typed enum: Intent's match arms are single-expression, so each field access through a payload-carrying enum would force an awkward match wrapper; Token already makes the same tradeoff. PRD: `prds/done/phase-34-statement-parser-in-intent.md`.

### Phase 33: Parser Top-Level in Intent -- SHIPPED (2026-06-03)
Second stage2 Intent-in-Intent artefact. `selfhost/formatter/parser.intent` (~500 lines) builds on Phase 32's lexer and parses the top-level shape of an Intent program: module declaration, file imports + package imports (including dotted forms), and function declarations (modifiers + signature + body captured as raw token-stream text for now). Out of scope (Phase 34+): statements / expressions inside function bodies, entity / trait / impl declarations, real TypeRef AST (v1 stores the head + a `<...>` suffix for generics). 12 in-language tests covering each shape plus error-path cases; combined with Phase 32 lexer that's 25/25 passing on rust + js. Two real Rust-backend bugs surfaced and fixed: (a) `ReturnStmt` value expressions weren't being cloned for non-Copy field-access / index-access types, mirroring the existing `AssignStmt` logic; (b) entity-typed fields with no obvious default emitted `EntityName { /* default fields */ }` placeholders that don't compile — fixed by hoisting top-level `self.field = expr` assignments out of the constructor body and into the struct literal. Three lexer keywords added (`import`, `public`, `async`) with `_marker` suffix on the kind functions to avoid colliding with Intent's own reserved-word handling.

### Phase 32: Lexer in Intent (Stage2 first step) -- SHIPPED (2026-06-03)
First Intent-in-Intent code under [ADR 0040](decisions/0040-self-hosted-formatter-strategy.md)'s stage2 plan. `selfhost/formatter/lexer.intent` (~400 lines) tokenises a useful subset of Intent source: identifiers + keywords (small lookup table), integer literals, double-quoted strings (raw, no escape decoding), the common punctuation (paren/brace/bracket/`;` `,` `:` `.` `..` `=` `=>` `+` `-` `*` `/` `%` `==` `!=` `<` `>` `<=` `>=` `?` `|` `@`), whitespace skipping, and single-line `//` comments. 13 in-language tests covering empty source, identifiers, all keyword kinds, integer literals, every punctuation form, string literal escape-preserving, comments, line/column tracking, and a full small-program token sequence. All 13 pass on rust + js. Findings surfaced and addressed: (a) Rust backend `&mut self` propagation was incomplete — methods calling `self.<user_method>()` weren't tagged as mutating, fixed in this phase by treating any `self.method()` call as conservatively mutating and extending the statement walker to cover `let` / `return`; (b) Intent's string literals interpret `{...}` as interpolation markers, so embedding a literal `{` requires `'{'.to_string()` — documented as a gotcha and worth a future polish ADR; (c) `let _:` isn't accepted by the parser — workaround is to use expression statements for discarded results.

### Phase 31: String Indexing + `Char` Type -- SHIPPED (2026-06-03)
First language extension for the stage2 self-hosting plan ([ADR 0041](decisions/0041-string-indexing-and-char-type.md), strategic frame in [ADR 0040](decisions/0040-self-hosted-formatter-strategy.md)). Adds: `Char` primitive (dedicated type, codepoint-indexed; Rust+Dafny model), char literals `'a'` / `'\n'` / `'\u{1234}'` with surrogate + range validation, string indexing `s[i]: Char`, slicing `s[i..j]: String`, `len(s)` for String (codepoint count), ASCII char predicates (`is_digit` / `is_alpha` / `is_alphanumeric` / `is_whitespace` / `is_lowercase` / `is_uppercase`), `to_codepoint` / `to_string` / free builtin `char_from_codepoint(n) -> Result<Char, String>`, and comparison operators on Char. Three backends emit working code (rust: precomputed `Vec<char>` via `.chars()`, js: `Array.from(s)`, wasm: stubs the indexing op but accepts Char as i32). Z3 verifier encodes Char as bounded Int and predicates as integer comparisons; tautology contracts like `requires c.is_digit() ensures c.is_alphanumeric()` verify end-to-end. Feature-coverage example `examples/char_string_demo.intent` (11 test blocks) passes on rust + js via `intentc test --all-targets`. Implementation notes: discovered existing Intent String already had `.starts_with` / `.contains` / `.split` / `.trim` (so ADR 0040/0041 framing slightly understated what was already there; ADR 0041's contribution is char-level access). `char_from_codepoint` is exposed as a free builtin rather than `Char.from_codepoint` to avoid needing static-method syntax in v1.

### Phase 31+: Self-Hosted Formatter (Multi-Phase) -- PLANNING (2026-06-03)
Strategic frame in [ADR 0040](decisions/0040-self-hosted-formatter-strategy.md): build a stage2 Intent formatter (`selfhost/formatter/`) that parses and re-emits Intent source, eventually replacing the Go formatter at byte parity. Stage1 / stage2 mental model adapted from Zig precedent. Delivered gap-driven across multiple phases — each phase first lands the language extension it needs in stage1 (Go), then writes stage2 Intent code that uses it. First gap: string indexing + `Char` type ([ADR 0041](decisions/0041-string-indexing-and-char-type.md), `prds/done/phase-31-string-primitives.md`). Phase 32 lexer, 33-36 parser, 37 formatter, 38 full-feature parity, 39 differential test gate + CLI integration. Phase numbers indicative; will shift as gaps surface. Per-phase ADRs follow as needed (sum types, dynamic dispatch, I/O streaming, etc.).

### Phase 30: Package Registry — Git-Based + MVS -- SHIPPED (2026-06-03)
Implemented per [ADR 0039](decisions/0039-package-registry-git-mvs.md) (revises [ADR 0027](decisions/0027-package-management-design.md) §"Version Resolution"). Unblocks the two Phase-13 registry TODOs and the multi-package self-hosting path. New surface: `DependencySpec.Git` field; canonical `foo = { git = "<url>", version = "<min>" }` form; bare-version short form and `^`/`~` constraints accepted with deprecation warnings; `internal/compiler/fetcher.go` (Fetcher interface, GitFetcher.ListTags/Clone, TreeHash); `internal/compiler/resolver.go` (Resolver with MVS graph walk, max-of-minimums, cross-major hard error); `internal/compiler/lockfile.go` (intent.lock with sha256 tree-hash, deterministic output, Verify); cache layout extended with `~/.intent/cache/git/<host>/<owner>/<repo>@<rev>/` keyed by commit; `internal/compiler/loader.go` GitFsLoader composes Fetcher+Cache+root for production resolution. CLI gains `intentc pkg install [--refresh]`, `intentc pkg upgrade <name> [--major]`, `intentc pkg vendor`; existing `intentc pkg install` writes intent.lock for both git and path deps. Two integration tests in `internal/compiler/loader_test.go` exercise the full pipeline against local bare git fixtures (no network). NFC normalisation deliberately omitted from TreeHash to keep zero-external-deps; documented in ADR 0039 §7. Aligns with self-hosting per the same motivation as Phase 29.

### Phase 29: Retire Legacy Rust testgen Path (17.A.4) -- SHIPPED (2026-06-02)
Implemented per [ADR 0038](decisions/0038-retire-legacy-rust-testgen.md). With Phase 27 (entity/method emission) and Phase 28 (multi-param iteration) both shipped, `--target intent` covers the surface area the legacy Rust emitter handled. This phase removes the legacy path: `intentc test-gen` now defaults to `--target intent`; `--target rust` errors with a migration message pointing at `intentc test-gen --emit` + `intentc test --target rust`. Deleted `internal/testgen/{testgen.go,rustutil.go,values.go,testgen_test.go}` (~1.9k LOC) plus `compiler.GenerateTests` / `compiler.GenerateTestsProject`. Migrated `TestConstraintAnalysis` to `internal/testgen/constraints_test.go`. Makefile `test-gen-examples` now emits `_test.intent` siblings; `make clean` removes them. Closes Phase 17.A; the testgen package now has one emission path that produces target-agnostic Intent test blocks, executable on every backend via `intentc test`.

### Phase 28: testgen `--target intent` Multi-Param Iteration -- SHIPPED (2026-06-01)
Implemented in commit 2da9fec..HEAD per [ADR 0037](decisions/0037-testgen-multi-param-iteration.md). Free functions with N ≥ 2 Int params now emit nested `while` loops in source-order, capped at `floor(1000^(1/N))` iterations per param. Each loop range is trimmed centred around the original midpoint (`[-10, 10]` → `[-5, 4]` for N=3) so coverage isn't always anchored at the lower bound. Discovered a pre-existing correctness bug: `generateIntentTestForFunction` sorted params alphabetically, which broke the multi-param call site (`clamp(hi, lo, x)` instead of `clamp(x, lo, hi)`); fixed in this phase by using declaration order. Closes Phase 17.A.2 — the last prerequisite for retiring the legacy Rust testgen path (17.A.4 becomes deliverable). ADR 0037 cites precedent from QuickCheck, Hypothesis, fast-check, AutoTest, Pex, Dafny.

### Phase 27: testgen `--target intent` Entity/Method Emission -- SHIPPED (2026-05-31)
Implemented in commit 1e5e2f5..HEAD per [ADR 0036](decisions/0036-testgen-entity-method-emission.md). `intentc test-gen --target intent` now emits one auto-test per (entity, method) pair carrying contract clauses. Each test constructs the entity with default-valued args, binds method params as named locals so the ensures-clause references resolve, captures `old(<self.x>)` sub-expressions into `let __old_<i>` locals before the call, and asserts each ensures with `self → a`, `old(...) → __old_<i>`, `result → __r` substitutions. Generic entities and constructor-less entities get a one-line skip comment instead of silently disappearing from the output. Closes Phase 17.A.1 (one of two prerequisites for retiring the legacy Rust testgen path; 17.A.2 multi-param iteration for free functions still pending). ADR 0036 cites precedent from Dafny, JML, Pex, QuickCheck/Hypothesis, AutoTest (Eiffel).

### Phase 26: LSP textDocument/references (Find References) -- SHIPPED (2026-05-31)
Implemented in commit 1c94d3d..HEAD per [ADR 0035](decisions/0035-lsp-find-references.md). Adds the `textDocument/references` LSP method covering top-level decls (function, entity, enum, trait, test, extern) plus locals/params/`self`. Locals are scope-bound to their enclosing function frame; top-level references walk every AST in the workspace (including cross-package, free from Phase 25's plumbing). `includeDeclaration` honoured per LSP spec. Method/field references and same-name disambiguation across modules are documented v1 limitations — deferred to future PRDs. ADR 0035 cites precedent from rust-analyzer, gopls, tsserver, pyright, metals.

### Phase 25: Cross-Package Goto-Def (test-only) -- SHIPPED (2026-05-31)
Regression test `TestDefinitionCrossPackage` confirmed cross-package goto-definition already works in the LSP — the deferred claim from Phase 19/20 was a planning artefact. `workspace.siblingModules()` already returns every module discovered by `ModuleRegistry.AllModules()` (which walks `intent.toml`'s `[dependencies]` transitively); `resolveAcrossWorkspace` iterates them without gating on package boundary. Phase 25 added the test that locks the behaviour in and corrected the stale deferred-list claim in ADR 0032 (4th revision). No resolver code changed.

### Phase 24: Per-Contract Verify Source Positions -- SHIPPED (2026-05-31)
Implemented in commit c778e40..HEAD. Closes ADR 0032's "Z3 anchors at (1,1)" deferred item via [ADR 0034](decisions/0034-per-contract-source-positions.md). The AST already carried 1-indexed `Line` / `Column` on `ContractClause` and `DecreaseClause` from the parser; lowering threaded those through new fields on `ir.Contract` and `ir.DecreasesClause`; the verifier copies them into `verify.VerifyResult.Line` / `.Column`; and the LSP's `verifyResultsToDiagnostics` builds an LSP `Range` from the position (1-indexed parser → 0-indexed LSP). Toolchain-error rows with no source position (z3-not-found, translation errors) leave Line=0 and fall back to file-start anchoring. Console output of `intentc verify` is unchanged this phase; CLI `file:line:col:` prefixes are a separate future ADR. Precedent surveyed in ADR 0034 from Dafny, SPARK Ada, F*, Lean, Coq, Liquid Haskell, Eiffel, C# Code Contracts.

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
- Find-references, rename, code actions, refactorings, semantic tokens, inlay hints, Marketplace publishing — still v1.1+

### Phase 17.B Annotation Implementation -- DONE (2026-05-30)
Shipped per ADR 0031 in commit 4dacd6c. Lexer/parser/AST/checker/IR/runner all carry the `@target_specific("rust", ...)` annotation through; new `SkipAnnotation` classification keeps annotation skips distinct from the WASM-rejection skip. `examples/target_specific_demo.intent` demonstrates all four cases.

---

## Milestone 9: Self-Hosting -- COMPLETE (2026-07-09)

The Intent toolchain is written in Intent and compiles itself. Four tools are
self-hosted and byte-equal with their Go (stage1) counterparts:

- **Formatter** (`intentc fmt --self-hosted`) -- Phase 42, a fixpoint on its own source.
- **Linter** (`intentc lint --self-hosted`) -- Phase 43.
- **Checker** (`intentc check --self-hosted`) -- Phases 45-54, incl. the compiler's own
  multi-file source (ADR 0058).
- **Compiler** (`intentc build --emit --self-hosted`) -- IR lowering + Rust backend in
  Intent (Phase 55, single-file; Phase 56, multi-file with cross-module name mangling,
  type-origins, and call/method return-type inference; Phase 57, emitter hardening --
  byte-equal with stage1 on EVERY program in the repo, not just the example corpus).

The bootstrap is a proven fixpoint: **stage1** (Go) compiles the Intent compiler into
**stage2**, **stage2** compiles it into **stage3**, and stage3 re-emits the entire
toolchain byte-identical to stage1. Gates: `make diff-emit` (33/33 example corpus),
`make diff-emit-self` (4/4 the toolchain's own source), `make diff-emit-sweep` (71/0/0
every repo program, Phase 57), `make bootstrap-stage3` (4/4 the stage1->stage2->stage3
fixpoint), plus the per-tool `diff-*` / `selfcheck-*` gates. See ADR 0059 and
`prds/done/phase-55-*` / `phase-56-*` (Phase 57 was gap-closing; see `prds/progress.md`).

### Phase 59: Cross-package code generation -- DONE (2026-07-10)

An audit found the "cross-package codegen not fully supported" caveat was largely stale:
entities, methods, traits, qualified function calls, enums, generics, and nested type
references already emitted, compiled, and ran across a package boundary on Rust and JS.
Three real bugs were fixed ([ADR 0061](decisions/0061-cross-package-codegen.md)):
Rust enum data-variant construction dropped its fields under a mangling (G1); JS enum
construction used the unmangled name (G2); Rust multi-module generic monomorphization
emitted duplicate structs and un-monomorphized signature types (G3, fixed in both the Go
and the self-hosted Intent backend). New `diff-emit` fixtures `multimod_enum` /
`multimod_generic` keep them byte-equal stage1↔stage2 (`make diff-emit` 35/35;
`bootstrap-stage3` still a fixpoint). The blanket build warning is removed; DESIGN.md
§15.9 now carries the support matrix. Two syntax limitations remain, documented with
workarounds: module-qualified type-args/variants (`pkg.Generic<T>()`) and unqualified
imported function calls.

### Phase 60: Cross-package ergonomics completion -- DONE (2026-07-10)

Closed the remaining cross-package gaps from Phase 59 ([ADR 0061](decisions/0061-cross-package-codegen.md)):

- **Unqualified imported function calls (G6)** — `helper(...)` for an imported `helper`
  type-checked but emitted the calling module's (empty) prefix; a `funcOrigins` map on both
  backends now mangles it to the defining module's prefix.
- **Module-qualified generic/variant syntax** — `pkg.Generic<T>()`, `pkg.Variant()`, and
  `pkg.Enum.Variant()` now parse and lower (stage1 + stage2), rewriting to the bare form so
  monomorphization / variant construction happen in one place.
- **stage2 entry-only generic monomorphization** — the self-hosted backend now emits the
  monomorphization for a generic instantiated only in the entry module (assigns each
  instantiation to the first-using module in `lower_all`, deduped globally, decl resolved
  against the global registry). The `examples/packages` demo constructs its generic directly
  in the consumer again.

Gates: `make diff-emit` 36/36 (new `multimod_qualified` fixture), `bootstrap-stage3` 4/4,
`diff-emit-sweep` clean. Deferred (internal stage2 parity, not user-facing): generic *free
function* monomorphization in stage2, and the stage2 `funcOrigins` mangling for unqualified
imported calls.

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
- Out of scope (v1.1+): member completion (.field/.method), rename, code actions, refactorings, semantic tokens, inlay hints, Marketplace publishing, incremental sync, multi-byte UTF-16, per-contract Z3 source positions

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

The remaining Attractor-driven work is async handler integration (Phase 12 stretch goal, deferred — see `prds/done/phase-14-phase11-13-gaps.md`).

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
