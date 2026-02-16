# Intent Programming Language

Intent is a programming language designed for AI code assistants to write, that compiles to **multiple targets** for humans to use. The toolchain is built in Go and produces native binaries (via Rust), JavaScript, and WebAssembly from a single source file.

The language prioritizes **explicit contracts**, **declared intent**, and **verifiable correctness** over brevity. Every function carries preconditions and postconditions, every entity carries invariants, and intent blocks link natural-language goals to formal verification points.

## Prerequisites

- **Go** 1.21+
- **Rust** (with `cargo`) for native binary compilation
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
intentc build [--target rust|js|wasm] [--emit] <file>   Compile to binary or source
intentc check <file.intent>                              Parse and type-check only
intentc verify <file.intent>                             Verify contracts with Z3 SMT solver
intentc fmt [--check] <file.intent>                      Format source code
intentc lint <file.intent>                               Run lint checks
intentc test-gen [--emit] <file.intent>                  Generate property-based tests
```

## Showcase

The `showcase/` directory demonstrates the same `task_queue.intent` source compiled to different targets:

- **Option A** (`examples/task_queue.intent`): CLI application compiled to native Rust binary
- **Option B** (`showcase/option-b/`): Browser dashboard using compiler-generated JavaScript
- **Option C** (`showcase/option-c/`): Node.js server with REST API using compiler-generated JavaScript

All three options use **unmodified compiler output** -- no hand-edited generated code.

## Project Structure

```
.
├── cmd/intentc/          CLI entry point
├── internal/
│   ├── ast/              AST node definitions
│   ├── backend/          Shared backend interfaces
│   ├── checker/          Semantic analysis and type checking
│   ├── codegen/          Legacy Rust code generation
│   ├── compiler/         Pipeline orchestration
│   ├── diagnostic/       Error/warning reporting
│   ├── formatter/        Source code formatter
│   ├── ir/               Intermediate representation
│   ├── jsbe/             JavaScript backend
│   ├── lexer/            Tokenizer
│   ├── linter/           Style and best-practice warnings
│   ├── parser/           Recursive-descent parser
│   ├── rustbe/           Rust backend (IR-based)
│   ├── testgen/          Property-based test generation
│   └── verify/           Z3 SMT verification
├── examples/             Example .intent programs
├── showcase/             Multi-target demos
├── testdata/             Test fixtures
├── docs/
│   ├── DESIGN.md         Full language design document
│   ├── grammar.ebnf      Formal grammar specification
│   └── ROADMAP.md        Feature overview and status
└── Makefile
```

## Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-v

# Type-check all examples
make check-examples

# Lint all examples
make lint-examples
```

## Documentation

- [Design Document](docs/DESIGN.md) -- full language specification
- [Grammar](docs/grammar.ebnf) -- formal EBNF grammar
- [Roadmap](docs/ROADMAP.md) -- feature overview and status tracking
- [Agent Instructions](AGENT.md) -- guide for AI code assistants writing Intent
