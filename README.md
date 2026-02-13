# Intent Programming Language

Intent is a programming language designed for AI code assistants to write, that compiles to native binaries for humans to use. The toolchain is built in Go, transpiles to Rust, and produces native binaries via `cargo build`.

The language prioritizes **explicit contracts**, **declared intent**, and **verifiable correctness** over brevity. Every function carries preconditions and postconditions, every entity carries invariants, and intent blocks link natural-language goals to formal verification points.

## Prerequisites

- **Go** 1.21+
- **Rust** (with `cargo`) for native binary compilation

## Quick Start

```bash
# Build the compiler
make build

# Compile and run an example
./intentc build examples/hello.intent
./hello

# Type-check without building
./intentc check examples/bank_account.intent

# Run the linter
./intentc lint examples/bank_account.intent

# Emit Rust source (no cargo required)
./intentc build --emit-rust examples/fibonacci.intent
```

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

### Intent Blocks

```
intent "Safe withdrawal preserves non-negative balance" {
    goal: "BankAccount.withdraw never results in balance < 0";
    guarantee: "if withdraw returns false then balance is unchanged";
    verified_by: [BankAccount.invariant, BankAccount.withdraw.requires];
}
```

## CLI Commands

```
intentc build <file.intent>              Compile to native binary
intentc build --emit-rust <file.intent>  Emit generated Rust source
intentc check <file.intent>              Parse and type-check only
intentc lint <file.intent>               Run lint checks for style/best practices
```

## Project Structure

```
.
├── cmd/intentc/          CLI entry point
├── internal/
│   ├── ast/              AST node definitions
│   ├── checker/          Semantic analysis and type checking
│   ├── codegen/          Rust code generation
│   ├── compiler/         Pipeline orchestration (parse -> check -> codegen -> cargo)
│   ├── diagnostic/       Error/warning reporting
│   ├── lexer/            Tokenizer
│   ├── linter/           Style and best-practice warnings
│   └── parser/           Recursive-descent parser
├── examples/             Example .intent programs
├── testdata/             Test fixtures
├── docs/
│   ├── DESIGN.md         Full language design document
│   └── grammar.ebnf      Formal grammar specification
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
- [Build Plan](docs/PLAN.md) -- phased implementation plan with milestones
- [Roadmap](docs/ROADMAP.md) -- feature overview and status tracking
