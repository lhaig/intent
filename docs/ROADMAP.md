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

### In Progress
- [ ] Formatter (`intentc fmt`) -- CLI wired up, formatter package not yet implemented

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

### Phase 4.5: IR Validation and Testing
- IR-level tests independent of any backend
- Contract consistency checks at the IR level
- Round-trip testing: source -> IR -> Rust -> compile -> run

---

## Milestone 5: Static Verification (Z3)

**Goal:** Prove contracts at compile time. Move from "runtime assertions" to
"verified correctness."

This is the milestone that differentiates Intent from "Rust + contract macros."

### Phase 5.1: SMT Translation
- Translate IR contract nodes to SMT-LIB formulae
- Start with arithmetic contracts: `requires x > 0`, `ensures result >= 0`
- Map Intent types to SMT sorts (Int -> Int, Bool -> Bool)

### Phase 5.2: Z3 Integration
- Invoke Z3 solver on generated SMT formulae
- Report per-contract status: "verified" / "unverified" / "timeout"
- New CLI command: `intentc verify <file.intent>`
- Contracts that Z3 proves can optionally skip runtime assertion generation

### Phase 5.3: Verification Reporting
- `verified_by` references checked semantically, not just syntactically
- Intent blocks report verification status: "all contracts verified" vs
  "2 of 5 contracts unverified"
- Human-readable verification report

### Phase 5.4: Quantifier Verification
- Translate `forall`/`exists` to SMT quantifiers
- Handle array bounds and index safety proofs
- Loop invariant verification via inductive reasoning

---

## Milestone 6: Multi-Target Code Generation

**Goal:** Intent programs run on more than just native binaries.

See [ADR 0009](decisions/0009-multi-target-codegen.md) for full rationale.

### Phase 6.1: Codegen Backend Interface
- Define common interface that all backends implement
- Factor out contract enforcement strategy per target
- Rust backend implements the interface (refactor from Phase 4.4)

### Phase 6.2: WASM Target (via Rust)
- `intentc build --target wasm <file.intent>`
- Leverage Rust's `wasm32-unknown-unknown` target
- Contracts compile to WASM traps or imported assertion functions
- Produces `.wasm` binary usable in browsers and edge runtimes

### Phase 6.3: JavaScript/TypeScript Target
- `intentc build --target js <file.intent>`
- Direct JS/TS emission from IR (no Rust intermediary)
- Contracts compile to `throw` statements with contract metadata
- TypeScript output preserves type annotations
- Enables frontend development with Intent contracts

### Phase 6.4: Direct WASM Emission
- Remove Rust intermediary for WASM target
- Emit WASM bytecode directly from IR
- Smaller output, faster compilation, no Rust toolchain dependency

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

### String Interpolation
- `"Balance: {self.balance}"` syntax
- Maps to format functions in each backend

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
