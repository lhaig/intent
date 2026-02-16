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

### Generics
- Type parameters on functions and entities
- Monomorphization in codegen (one concrete type per instantiation)
- Contract expressions over generic types

### Traits / Interfaces
- Behavioral contracts across types
- Default method implementations
- Trait-based dispatch in codegen

### String Interpolation -- DONE
- [x] `"Balance: {self.balance}"` syntax across lexer, parser, checker, IR, backends
- [x] Rust backend: `format!()` macro generation
- [x] JS backend: template literal with `${}` interpolation
- [x] End-to-end compiler test in `compiler_test.go`

### Rust FFI / Crate Imports
- Call Rust crate functions from Intent (Rust backend only)
- Type-safe bridge declarations with contracts on the boundary
- Unlocks the Rust ecosystem for Intent programs

---

## Milestone 8: Developer Experience

### LSP Server
- Diagnostics (parse errors, type errors, lint warnings)
- Go-to-definition for functions, entities, methods, fields
- Hover information (types, contracts, verification status)
- Editor integration (VS Code extension)

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
