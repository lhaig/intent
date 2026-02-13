# Intent Language Roadmap

The POC is complete: lexer, parser, checker, codegen, compiler pipeline, CLI, linter, and examples all work. The compiler produces native binaries from `.intent` source via Rust.

This roadmap organizes future work into four tracks. Items within each track are roughly ordered by priority -- earlier items unblock later ones.

---

## Track 1: Language Features

### Loops
- `while` loop (condition + body)
- `for` loop over ranges (`for i in 0..n`)
- Loop invariants in contracts (`invariant` clause on loops)
- Currently there is no iteration; the fibonacci example uses nested `if` statements

### Arrays / Lists
- `Array<T>` type with fixed or dynamic size
- Index access (`arr[i]`), `len()`, `push()`, `pop()`
- Quantifier expressions in contracts: `forall`, `exists`
- Rust mapping to `Vec<T>`

### Enums and Pattern Matching
- Sum types: `enum Direction { North, South, East, West }`
- `match` expression with exhaustiveness checking
- Enum variants with associated data

### Error Handling
- `Result<T, E>` type built into the language
- `?` propagation operator or explicit match
- Contracts on error paths

### Import System
- Multi-file programs with `import` declarations
- Dependency resolution across modules
- Visibility modifiers (`public` / `private`)

### Generics
- Type parameters on functions and entities
- Trait bounds (depends on traits)

### Traits / Interfaces
- Behavioral contracts across types
- Default method implementations
- Trait-based dispatch in codegen

### String Interpolation
- `"Balance: {self.balance}"` syntax
- Maps to Rust `format!()` with positional args

---

## Track 2: Tooling

### Formatter (`intentc fmt`)
- Canonical formatting for `.intent` source
- Consistent indentation, spacing, line breaks
- Idempotent output

### LSP Server
- Diagnostics (parse errors, type errors, lint warnings)
- Go-to-definition for functions, entities, methods, fields
- Hover information (types, contracts)
- Editor integration (VS Code extension)

### REPL / Playground
- Interactive expression evaluation
- Web-based playground for sharing snippets

### Better Error Messages
- Source snippets with underline markers
- "Did you mean?" suggestions for typos
- Multi-line error context

---

## Track 3: Verification

### Static Contract Verification
- Prove contracts hold at compile time instead of relying on runtime assertions
- Start with simple cases: arithmetic bounds, null checks
- SMT solver integration (Z3)

### Formal Verification
- Connect `verified_by` references to actual proof obligations
- Generate proof obligations from contracts
- Integration with external provers

### Property-Based Test Generation
- Generate test cases from `requires`/`ensures` contracts
- Boundary value analysis from contract expressions
- Output as Rust `#[test]` functions

### Optimization Levels
- `--release` flag to strip contract assertions
- Configurable per-contract: keep critical invariants, remove debug contracts

---

## Track 4: Code Quality

### Suppress Unnecessary Parentheses in Generated Rust
- Codegen currently wraps all binary expressions in `()`
- Track operator precedence to emit parens only when needed

### Linter Enhancements
- Complexity warnings (deeply nested `if` chains)
- Unreachable code detection
- Contract completeness hints (e.g., "withdraw has requires but no ensures")

### Test Coverage
- Track which contracts are exercised by test inputs
- Report untested code paths

---

## Completed

- [x] Lexer with full token set
- [x] Recursive-descent parser
- [x] Semantic checker (types, contracts, intents)
- [x] Rust code generator
- [x] Compiler pipeline (parse -> check -> codegen -> cargo)
- [x] CLI (`build`, `check`, `lint`)
- [x] Linter (8 rules: empty body, missing contracts, naming, unused vars/params, mutable-never-reassigned, entity without invariant, empty verified_by)
- [x] `#![allow(...)]` in generated Rust to suppress cargo warnings
- [x] Examples (hello, bank_account, fibonacci)
- [x] Design document and grammar specification
