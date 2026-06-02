# Intent Programming Language

Intent is a programming language designed for AI code assistants to write, that compiles to **multiple targets** for humans to use. The toolchain is built in Go and produces native binaries (via Rust), JavaScript, and WebAssembly from a single source file.

The language prioritizes **explicit contracts**, **declared intent**, and **verifiable correctness** over brevity. Every function carries preconditions and postconditions, every entity carries invariants, and intent blocks link natural-language goals to formal verification points. Programs can be tested in Intent itself (`test "..." { ... }` blocks) and edited in VS Code with full LSP support — diagnostics, hover, go-to-definition, completion, formatting, signature help, semantic-token highlighting, and Z3 verification status.

## Prerequisites

- **Go** 1.26+
- **Rust** (with `cargo`) for native binary compilation
- **Node.js** (optional) for JavaScript target builds and in-language tests on the JS target
- **Z3** (optional) for SMT-based contract verification

## Quick Start

```bash
# Build the compiler
make build

# Compile and run an example (native binary)
./intentc build examples/hello.intent
./hello

# Compile to JavaScript
./intentc build --target js examples/task_queue.intent
node task_queue.js

# Type-check without building
./intentc check examples/bank_account.intent

# Verify contracts with Z3
./intentc verify examples/bank_account.intent

# Format source code
./intentc fmt examples/bank_account.intent

# Run the linter
./intentc lint examples/bank_account.intent

# Emit generated source without building
./intentc build --emit examples/fibonacci.intent          # Rust source
./intentc build --target js --emit examples/hello.intent  # JS source
```

## Multi-Target Compilation

One `.intent` source file compiles to multiple targets with identical logic:

```bash
# Native binary (default)
intentc build task_queue.intent          # -> task_queue (executable)

# JavaScript
intentc build --target js task_queue.intent   # -> task_queue.js

# WebAssembly
intentc build --target wasm task_queue.intent # -> task_queue.wasm
```

All contracts (preconditions, postconditions, invariants) are enforced at runtime in every target. The same contract violation that crashes the Rust binary will throw an exception in JavaScript.

## Language Features

### Functions with Contracts

```
function fib(n: Int) returns Int
    requires n >= 0
    ensures result >= 0
{
    if n <= 0 { return 0; }
    if n == 1 { return 1; }
    // ...
}
```

### Entities with Invariants

```
entity BankAccount {
    field balance: Int;

    invariant self.balance >= 0;

    constructor(owner: String, initial_balance: Int)
        requires initial_balance >= 0
        ensures self.balance == initial_balance
    {
        self.owner = owner;
        self.balance = initial_balance;
    }

    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}
```

### Enums with Pattern Matching

```
enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float),
    Point,
}

let area: Float = match shape {
    Circle(r) => 3.14159 * r * r,
    Rectangle(w, h) => w * h,
    Point => 0.0
};
```

### Intent Blocks

```
intent "Safe withdrawal preserves non-negative balance" {
    goal "BankAccount.withdraw never results in balance < 0";
    guarantee "if withdraw returns false then balance is unchanged";
    verified_by BankAccount.invariant;
    verified_by BankAccount.withdraw.requires;
}
```

## CLI Commands

```
intentc build [--target rust|js|wasm] [--emit]
              [--strip-contracts] <file>                          Compile to binary or source
                                                                  (--strip-contracts drops contracts;
                                                                   see ADR 0033)
intentc check <file.intent>                                      Parse and type-check only
intentc verify <file.intent>                                     Verify contracts with Z3 SMT solver
intentc fmt [--check] <file.intent>                              Format source code
intentc lint <file.intent>                                       Run lint checks
intentc test-gen [--emit] <file>                                 Generate property-based test blocks
intentc test [--target t] [--all-targets] [--filter s]
             [--list] [--quiet] <file>                            Run in-language tests
intentc pkg init|add|remove|install|list                          Local package management
intentc lsp                                                       Start the LSP server (stdio)
```

## In-Language Testing

Write tests in Intent itself, alongside the code they exercise. `assert`, `assert_eq`, `assert_close`, and `assert_panics` are built-in; `intentc test` runs tests on rust + js + wasm targets and flags cross-target divergence.

```
test "deposit increases balance" {
    let mutable a: BankAccount = BankAccount("alice", 100);
    a.deposit(50);
    assert_eq(a.balance, 150);
}

@target_specific("rust")
test "FFI hash works on rust only" { ... }
```

See `examples/target_specific_demo.intent` and ADR [0029](docs/decisions/0029-in-language-testing.md) / [0031](docs/decisions/0031-target-specific-annotation.md).

## Editor Support

`intentc lsp` is a full LSP 3.17 server. The VS Code extension under `editors/vscode/` activates on `.intent` files. Other editors wire `intentc lsp` into their LSP clients.

Features (see [ADR 0032](docs/decisions/0032-lsp-v1-surface.md)):

- Live diagnostics (parser, checker, lint, Z3 verification on save)
- Hover with full `requires` / `ensures` contracts
- Go-to-definition for top-level decls, locals, params, methods, and fields (same-file + same-package)
- Document outline, format-on-command via `intentc fmt`
- Signature help, identifier completion
- Semantic tokens (type-aware highlighting) + TextMate grammar fallback

Build the .vsix from source:

```bash
cd editors/vscode
npm install
npm run package
code --install-extension intent-vscode-0.1.0.vsix --force
```

## Showcase

The `showcase/` directory demonstrates the same `task_queue.intent` source compiled to different targets:

- **Option A** (`examples/task_queue.intent`): CLI application compiled to native Rust binary
- **Option B** (`showcase/option-b/`): Browser dashboard using compiler-generated JavaScript
- **Option C** (`showcase/option-c/`): Node.js server with REST API using compiler-generated JavaScript
- **Option D** (`showcase/option-d/`): Browser WASM demo with direct binary emission (155 bytes)

All four options use **unmodified compiler output** -- no hand-edited generated code.

## Project Structure

```
.
├── cmd/intentc/          CLI entry point
├── internal/
│   ├── ast/              AST node definitions
│   ├── backend/          Shared backend interfaces
│   ├── checker/          Semantic analysis and type checking
│   ├── compiler/         Pipeline orchestration (includes test runner, package manager)
│   ├── diagnostic/       Error/warning reporting
│   ├── formatter/        Source code formatter
│   ├── ir/               Intermediate representation
│   ├── jsbe/             JavaScript backend
│   ├── lexer/            Tokenizer
│   ├── linter/           Style and best-practice warnings
│   ├── lsp/              LSP server (Phase 18-20)
│   ├── parser/           Recursive-descent parser
│   ├── rustbe/           Rust backend (IR-based, includes FFI)
│   ├── testgen/          Property-based test generation
│   ├── verify/           Z3 SMT verification
│   └── wasmbe/           WebAssembly backend (direct binary)
├── editors/vscode/       VS Code extension (.vsix bundles via esbuild)
├── examples/             Example .intent programs (including in-language tests)
├── showcase/             Multi-target demos
├── testdata/             Test fixtures
├── ops/plans/            Phase PRDs (the source of truth for in-progress work)
├── docs/
│   ├── DESIGN.md         Full language design document
│   ├── HARNESS.md        Agent self-improve harness contract
│   ├── grammar.ebnf      Formal grammar specification
│   ├── ROADMAP.md        Feature overview and status
│   └── decisions/        ADRs (architectural decisions, 33 and counting)
└── Makefile
```

## Testing

```bash
# Run the Go test suite (compiler internals)
make test

# Run in-language tests (intentc test) over every tested example
make test-examples

# Full mechanical-truth gate (gofmt + build + test + check + lint + fmt-check + test-examples)
make validate

# Lint all examples
make lint-examples
```

`make validate` is the gate. A green local validate predicts a green CI run — see [HARNESS.md](docs/HARNESS.md) for the agent harness contract.

## Documentation

- [Design Document](docs/DESIGN.md) — full language specification
- [INTENT.md](INTENT.md) — guide for writing `.intent` programs (intended audience: AI assistants)
- [Grammar](docs/grammar.ebnf) — formal EBNF grammar
- [Roadmap](docs/ROADMAP.md) — feature overview and status tracking
- [HARNESS.md](docs/HARNESS.md) — agent self-improve harness contract (validation, ADR/PRD conventions)
- [Architectural Decisions](docs/decisions/README.md) — index of ADRs
- [Reproduction Guide](docs/REPRODUCE.md) — reproduce the compiler with your own AI assistant
- [Agent Instructions](AGENTS.md) — multi-agent workflow and orchestration guidelines
