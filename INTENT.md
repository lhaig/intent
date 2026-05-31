# Intent Language -- Agent Instructions

You are generating code in **Intent**, a contract-based programming language that compiles to native binaries via Rust. Intent is designed for AI code assistants to write and humans to audit. The contract system is the product -- every function, entity, and module carries explicit verification.

## Why Intent Exists

Humans audit contracts, not implementations. When you generate Intent code, the human reads your `requires`, `ensures`, `invariant`, and `intent` blocks to verify correctness. The implementation is secondary -- if the contracts are right, the code is auditable. Write contracts first, then implement to satisfy them.

## File Structure

Every `.intent` file starts with a module declaration, followed by optional imports, then declarations.

```
module <name> version "<semver>";

import "<path>.intent";          // optional, for multi-file projects

// declarations: functions, entities, enums, intent blocks
```

## Types

### Primitive Types

| Type     | Description              | Rust Mapping |
|----------|--------------------------|-------------|
| `Int`    | 64-bit signed integer    | `i64`       |
| `Float`  | 64-bit floating point    | `f64`       |
| `String` | Owned UTF-8 string       | `String`    |
| `Bool`   | Boolean                  | `bool`      |
| `Void`   | Unit / no value          | `()`        |

### Parameterized Types

| Type              | Description                        |
|-------------------|------------------------------------|
| `Array<T>`        | Dynamic array of element type T    |
| `Result<T, E>`    | Success (Ok) or failure (Err) type |
| `Option<T>`       | Present (Some) or absent (None)    |
| `Future<T>`       | Pending async computation          |

### User-Defined Types

- **Entities** -- nominal struct types with fields, invariants, constructor, methods
- **Enums** -- sum types with unit or data-carrying variants

### Type Rules

- No implicit conversions. `Int` does not convert to `Float`.
- No type inference. All variable declarations require explicit type annotations.
- Arithmetic operators require matching numeric types (`Int` with `Int`, `Float` with `Float`).
- `+` on `String` values performs concatenation.

## Functions

```
function <name>(<params>) returns <Type>
    requires <bool_expr>       // zero or more preconditions
    ensures <bool_expr>        // zero or more postconditions
{
    // body
}
```

### Entry Point

Every program needs exactly one entry function:

```
entry function main() returns Int {
    // return value becomes the process exit code
    return 0;
}
```

### Contracts on Functions

- `requires` -- preconditions checked at function entry. Can reference parameters.
- `ensures` -- postconditions checked before return. Can reference parameters and `result` (the return value).

```
function abs(n: Int) returns Int
    requires true
    ensures result >= 0
    ensures (n >= 0) implies (result == n)
{
    if n < 0 { return 0 - n; }
    return n;
}
```

### Public Functions (Multi-File)

Mark functions with `public` to export them from a module:

```
public function add(a: Int, b: Int) returns Int
    ensures result == a + b
{
    return a + b;
}
```

## Variables

```
let x: Int = 42;                           // immutable
let mutable counter: Int = 0;              // mutable
counter = counter + 1;                     // assignment (mutable only)
```

All variables require a type annotation and an initializer.

## Control Flow

### If/Else

```
if condition {
    // ...
} else if other_condition {
    // ...
} else {
    // ...
}
```

Braces are always required. Condition must be `Bool`.

### While Loops

```
while condition {
    // body
    break;       // exit loop
    continue;    // skip to next iteration
}
```

With loop contracts (checked each iteration):

```
while i < len(arr)
    invariant i >= 0
    invariant i <= len(arr)
    decreases len(arr) - i
{
    i = i + 1;
}
```

- `invariant` -- boolean condition that must hold at each iteration
- `decreases` -- expression that must decrease each iteration (termination proof)

### For-In Loops

Over arrays:
```
for item in array_expr {
    print(item);
}
```

Over integer ranges (exclusive upper bound):
```
for i in 0..n {
    print(arr[i]);
}
```

## Arrays

```
let nums: Array<Int> = [1, 2, 3];         // literal
let first: Int = nums[0];                  // index access
let n: Int = len(nums);                    // length
let mutable items: Array<Int> = [1, 2];
items.push(3);                             // append (mutable only)
```

- All elements must be the same type.
- Index must be `Int`.
- `len()` returns `Int`.
- Arrays are passed by reference to functions automatically.

## Entities

Entities are struct types with fields, invariants, a constructor, and methods.

```
entity BankAccount {
    field owner: String;
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

    method withdraw(amount: Int) returns Bool
        requires amount > 0
        ensures (result == true) implies (self.balance == old(self.balance) - amount)
        ensures (result == false) implies (self.balance == old(self.balance))
    {
        if self.balance >= amount {
            self.balance = self.balance - amount;
            return true;
        }
        return false;
    }

    method get_balance() returns Int
        ensures result == self.balance
    {
        return self.balance;
    }
}
```

### Key Entity Rules

- Fields are declared with `field name: Type;`
- `invariant` conditions are checked after every constructor and method call
- `old(expr)` in `ensures` captures the value of `expr` before the method executed
- `self.field` accesses entity fields inside methods and constructors
- Instantiation: `let acc: BankAccount = BankAccount("Alice", 100);`
- Method calls: `acc.deposit(50);`
- Mutable entities: `let mutable acc: BankAccount = BankAccount("Alice", 100);` (required if methods mutate state)

## Generics

Intent supports user-defined generic entities and functions. Type parameters are declared with `<T>` syntax and instantiated with concrete types. Compilation uses monomorphization -- each unique instantiation (e.g., `Stack<Int>`, `Stack<String>`) produces a separate specialized version.

### Generic Entities

```
entity Stack<T> {
    field items: Array<T>;
    field count: Int;

    constructor()
        ensures self.count == 0
    {
        self.items = [];
        self.count = 0;
    }

    method push(item: T) returns Void
        ensures self.count == old(self.count) + 1
    {
        self.items.push(item);
        self.count = self.count + 1;
    }
}
```

### Generic Functions

```
function identity<T>(x: T) returns T
    ensures result == x
{
    return x;
}
```

### Instantiation

Type arguments are required (no type inference):

```
let mutable s: Stack<Int> = Stack<Int>();
s.push(42);

let x: Int = identity<Int>(42);
```

### Constraints

- No type bounds (e.g., `T: Comparable`) -- planned for future.
- No type inference for generic function calls -- explicit type args required.
- Multiple type parameters supported: `entity Pair<A, B> { ... }`

## Enums

### Simple Enums

```
enum Status {
    Pending,
    Running,
    Complete,
}
```

### Data-Carrying Variants

```
enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float),
    Point,
}
```

Variants with data use named fields. Unit variants have no parentheses.

### Constructing Variants

```
let c: Shape = Circle(5.0);
let r: Shape = Rectangle(3.0, 4.0);
let p: Shape = Point;
```

### Pattern Matching

```
let result: Float = match shape {
    Circle(r) => 3.14159 * r * r,
    Rectangle(w, h) => w * h,
    Point => 0.0
};
```

- Match must be exhaustive (cover all variants) or include a wildcard `_`.
- Match is an expression -- it returns a value.
- Bindings in patterns are positional, matching the field declaration order.
- Wildcard: `_ => default_value`

### Match as Statement

```
let code: Int = match s {
    Circle(r) => 1,
    Rectangle(w, h) => 2,
    _ => 0
};
```

## Error Handling

### Result<T, E>

```
function safe_divide(a: Int, b: Int) returns Result<Int, String>
    ensures result.is_ok() implies b != 0
    ensures result.is_err() implies b == 0
{
    if b == 0 {
        return Err("Division by zero");
    }
    return Ok(a / b);
}
```

- Construct with `Ok(value)` or `Err(error)`.
- Contracts can use `result.is_ok()` and `result.is_err()` predicates.

### Option<T>

```
function find(arr: Array<Int>, target: Int) returns Option<Int> {
    for i in 0..len(arr) {
        if arr[i] == target {
            return Some(i);
        }
    }
    return None;
}
```

- Construct with `Some(value)` or `None`.
- Contracts can use `result.is_some()` and `result.is_none()` predicates.

### Try Operator (?)

The `?` operator propagates errors from `Result` returns:

```
function process(a: String, b: String) returns Result<Int, String> {
    let x: Int = parse(a)?;    // returns Err early if parse fails
    let y: Int = parse(b)?;
    return Ok(x + y);
}
```

- `?` can only be used in functions that return `Result<T, E>`.
- On `Err`, it returns the error immediately.
- On `Ok`, it unwraps to the inner value.

### Matching on Result/Option

```
let val: Int = match result_expr {
    Ok(v) => v,
    Err(e) => 0
};

let val: Int = match option_expr {
    Some(v) => v,
    None => -1
};
```

## Async/Await

Intent supports async/await for concurrent execution. Async functions return futures that can be awaited.

### Async Functions

Mark functions with `async` to make them asynchronous:

```
async function fetch_data(url: String) returns Result<String, String>
    requires len(url) > 0
{
    let response: String = http_get(url, "")?;
    return Ok(response);
}
```

### Await

Use `await` to suspend execution until a future resolves:

```
let data: Result<String, String> = await fetch_data("https://api.example.com");
```

### Spawn

Use `spawn` to create concurrent tasks. `spawn` takes an async function call and returns `Future<T>`:

```
let f: Future<Int> = spawn compute(42);
let result: Int = await f;
```

### Concurrent Execution

Spawn multiple tasks and await their results:

```
async entry function main() returns Int {
    let f1: Future<Int> = spawn delayed_add(3, 4);
    let f2: Future<Int> = spawn delayed_add(10, 20);
    let r1: Int = await f1;
    let r2: Int = await f2;
    print(r1);
    print(r2);
    return 0;
}
```

### Built-in Async Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `sleep(ms)` | `Int -> Future<Void>` | Async sleep |
| `await_all(futures)` | `Array<Future<T>> -> Array<T>` | Wait for all futures |
| `await_any(futures)` | `Array<Future<T>> -> T` | Wait for first to complete |
| `timeout(future, ms)` | `(Future<T>, Int) -> Result<T, String>` | Timeout wrapper |

### Contracts on Async Functions

Contracts work the same as synchronous functions:
- `requires` is checked at function entry (before any await)
- `ensures` is checked when the future resolves (after all awaits)

### Async Code Generation

| Intent | Rust | JavaScript |
|--------|------|------------|
| `async function f()` | `async fn f()` | `async function f()` |
| `await expr` | `expr.await` | `await expr` |
| `spawn expr` | `tokio::spawn(async move { expr })` | `Promise` wrapper |
| `Future<T>` | `impl Future<Output=T>` | `Promise<T>` |

## Contract System

Contracts are the core feature. Every function, entity, and loop should carry contracts that express what the code guarantees.

### Preconditions (`requires`)

State what must be true when calling. Caller's responsibility.

```
function get(arr: Array<Int>, i: Int) returns Int
    requires i >= 0
    requires i < len(arr)
```

### Postconditions (`ensures`)

State what the function guarantees. Implementation's responsibility. Use `result` to refer to the return value.

```
function sum(a: Int, b: Int) returns Int
    ensures result == a + b
```

### Entity Invariants

Always-true conditions on entity state, checked after every constructor and method.

```
entity Counter {
    field value: Int;
    invariant self.value >= 0;
}
```

### The `old()` Expression

In method `ensures` clauses, `old(expr)` captures the pre-mutation value:

```
ensures self.balance == old(self.balance) + amount
```

### Quantifiers

For contracts over collections:

```
// All elements are positive
requires forall i in 0..len(arr): arr[i] > 0

// At least one element equals target
ensures exists i in 0..len(result): result[i] == target
```

- `forall` -- universal quantifier, all elements must satisfy the predicate
- `exists` -- existential quantifier, at least one element must satisfy
- Domain must be an integer range `start..end`
- Quantifiers expand to runtime loops that check each element

### Loop Contracts

```
while i < n
    invariant i >= 0
    invariant i <= n
    decreases n - i
{
    i = i + 1;
}
```

### Logical Operators in Contracts

| Operator   | Meaning              |
|------------|----------------------|
| `and`      | Logical AND          |
| `or`       | Logical OR           |
| `not`      | Logical NOT          |
| `implies`  | Logical implication  |

```
ensures (result == false) implies (self.balance == old(self.balance))
```

## Intent Blocks

Intent blocks link natural-language goals to formal contracts. The compiler verifies all `verified_by` references resolve to actual contracts.

```
intent "Account safety" {
    goal "Maintain non-negative balances for all accounts";
    constraint "Withdrawals cannot exceed current balance";
    guarantee "Deposits always increase balance by exact amount";
    verified_by BankAccount.invariant;
    verified_by BankAccount.deposit.requires;
    verified_by BankAccount.deposit.ensures;
    verified_by BankAccount.withdraw.requires;
}
```

### verified_by Reference Paths

- `EntityName.invariant` -- references entity invariants
- `EntityName.method_name.requires` -- method preconditions
- `EntityName.method_name.ensures` -- method postconditions
- `EntityName.constructor.requires` -- constructor preconditions
- `EntityName.constructor.ensures` -- constructor postconditions
- `function_name.requires` -- function preconditions
- `function_name.ensures` -- function postconditions

## Packages

Intent supports packages for sharing code across projects. A package is an Intent project with an `intent.toml` manifest.

### Creating a Package

Run `intentc pkg init` to create an `intent.toml` in the current directory:

```toml
[package]
name = "my-package"
version = "1.0.0"
description = "My reusable Intent library"

[dependencies]
```

### Adding Dependencies

```bash
intentc pkg add graph-types 1.0.0              # exact version
intentc pkg add utils "^2.0.0"                 # compatible range
intentc pkg remove graph-types                 # remove a dependency
```

Or edit `intent.toml` directly. Local path dependencies for development:

```toml
[dependencies]
my-lib = { path = "../my-lib" }
```

### Importing Packages

Package imports use identifier syntax (not quoted strings):

```
import graph_types;                    // import whole package
import graph_types.validation;         // import specific submodule

let g: graph_types.Graph = graph_types.Graph("node1");
```

This differs from local file imports which use quoted paths:

```
import "helpers.intent";               // local file import
```

### Example Usage

Given a dependency `math_utils` declared in `intent.toml`:

```toml
[dependencies]
math_utils = { path = "../math_utils" }
```

Use it in your code:

```
module main version "1.0.0";

import math_utils;

entry function main() returns Int {
    let result: Int = math_utils.add(3, 4);
    print(result);
    return 0;
}
```

### Version Constraints

| Constraint | Meaning |
|------------|---------|
| `"1.0.0"` | Exact version |
| `"^1.0.0"` | Compatible updates (>=1.0.0, <2.0.0) |
| `"^0.1.0"` | Compatible updates (>=0.1.0, <0.2.0) |
| `"^0.0.1"` | Exact version (>=0.0.1, <0.0.2) |
| `"~1.0.0"` | Patch updates only (>=1.0.0, <1.1.0) |
| `">=1.0.0"` | Minimum version |
| `"<2.0.0"` | Maximum version (exclusive) |

The caret operator (`^`) follows standard semver conventions for pre-1.0 versions: `^0.y.z` treats the minor version as the compatibility boundary, and `^0.0.z` pins to the exact patch version.

Resolved packages are cached at `~/.intent/cache/`. Run `intentc pkg install` to resolve and fetch all dependencies.

## Rust FFI (Crate Imports)

The Rust target can call functions from any Cargo crate via `extern function` declarations. Crates are listed in `intent.toml`'s `[rust_dependencies]` section.

```intent
extern function blake3_hash_hex(input: String) returns String
    from "blake3_intent::hash_hex"
    requires len(input) > 0
    ensures len(result) == 64;
```

The Rust path after `from` names the function exactly as the crate exposes it (first segment is the crate name, the rest is the path inside the crate). Contracts work as on regular functions and compile to runtime asserts around the call.

### Bridge types

Only these types may appear in an `extern function` signature:

| Intent type | Rust mapping |
|---|---|
| `Int` | `i64` |
| `Float` | `f64` |
| `Bool` | `bool` |
| `String` | owned `String` |
| `Void` | `()` |
| `Array<T>` | `Vec<T>` where `T` is bridged |
| `Result<T, E>` | `Result<T, E>` with both sides bridged |
| `Option<T>` | `Option<T>` with `T` bridged |

Entity types, user-defined enums, `Map<K, V>`, `Future<T>`, `Fn(...)` and trait references are rejected. If you need to expose a Rust type that does not bridge cleanly (e.g. `blake3::Hash`), write a tiny wrapper crate that returns a bridgeable shape — see `examples/ffi_blake3/`.

### intent.toml

```toml
[package]
name = "my_project"
version = "0.1.0"

[rust_dependencies]
blake3 = "1.5"
intent_zstd = { version = "0.13", features = ["std"] }
my_wrapper = { path = "./wrapper" }
```

Relative `path` entries are resolved against the directory containing `intent.toml`. User pins override the compiler's default sniffer entries (for example, pinning `tokio` here replaces the default `tokio = { version = "1", features = ["full"] }`).

### Target compatibility

`extern function` declarations are Rust-only. Building any program containing one with `--target js` or `--target wasm` fails at codegen with a clear error.

See ADR 0028 for the full design rationale.

## Multi-File Projects

### Imports

```
module main version "0.1.0";

import "math.intent";           // relative to project root (directory of entry file)
import "utils/helpers.intent";  // subdirectories supported
```

### Visibility

Only `public` declarations are accessible from other modules:

```
public function add(a: Int, b: Int) returns Int { ... }
public entity Point { ... }
public enum Color { ... }
```

Functions, entities, and enums without `public` are module-private.

### Qualified Calls

Call imported functions with `module.function()` syntax:

```
import "math.intent";

let result: Int = math.add(3, 4);
```

The module name is derived from the filename (e.g., `math.intent` -> `math`).

## Tests

Intent has first-class tests. A top-level `test "name" { ... }` declaration carries no parameters and no return type. The body may call any function in the module and asserts using the built-in `assert*` builtins.

```intent
test "fib(10) is 55" {
    assert_eq(fib(10), 55);
}

test "fib is non-negative across the contract range" {
    let mutable n: Int = 0;
    while n <= 15 {
        assert(fib(n) >= 0);
        n = n + 1;
    }
}
```

`async test "name" { ... }` is allowed on Rust and JS targets; WASM rejects tests entirely.

Tests cannot be `public` (they don't cross module boundaries) and cannot contain `return` statements (the body is implicitly `Void`).

### Assertion builtins

| Builtin                                        | Signature                                   | Behaviour                                                                 |
|------------------------------------------------|---------------------------------------------|---------------------------------------------------------------------------|
| `assert(cond)`                                 | `Bool -> Void`                              | Fails the test if `cond` is `false`.                                      |
| `assert_eq(actual, expected)`                  | `(T, T) -> Void` where T is comparable      | Fails if `actual != expected`. Entities need a `method eq(other: T) returns Bool`. |
| `assert_close(actual, expected, epsilon)`      | `(Float, Float, Float) -> Void`             | Fails if `\|actual - expected\| > epsilon`. **Use this instead of `assert_eq` for `Float`** — `assert_eq` rejects `Float` at type-check. |
| `assert_panics(fn)`                            | `Fn() -> Void -> Void`                      | Fails if `fn` does not panic.                                             |

### Running tests

```
intentc test foo.intent                    # runs on rust target (default)
intentc test --target js foo.intent        # runs on js target (requires node)
intentc test --all-targets foo.intent      # runs on rust + js + wasm, flags cross-target divergence
```

Cross-target divergence (e.g. a test that passes on Rust but fails on JS) is reported as `DIFF` and produces a non-zero exit code.

Additional runner flags: `--filter <substring>` (only run tests whose name contains substring), `--list` (print declared test names without running), `--quiet` (summary line only).

### Target-specific tests

Some tests are legitimately target-specific — a Rust-only FFI check, a JS-only DOM test, behaviour that differs between targets by design. Prefix the test with `@target_specific("rust", "js", ...)` so the runner skips it on excluded targets instead of flagging cross-target divergence:

```intent
@target_specific("rust")
test "FFI hash matches known value" {
    assert_eq(blake3_hash_hex("hello"), "ea8f163...");
}

@target_specific("rust", "js")
test "runs everywhere except wasm" {
    assert(1 + 1 == 2);
}
```

In `--all-targets` mode the runner emits `SKIP` rows for excluded targets with the annotation as the reason. In single-target mode (e.g. `--target js`), excluded tests are silently filtered out. `@target_specific("wasm")` is accepted but warned — WASM rejects all test declarations. See ADR 0031.

## Built-in Functions

| Function        | Signature                    | Description              |
|-----------------|------------------------------|--------------------------|
| `print(expr)`   | Any primitive type -> Void   | Print value to stdout    |
| `len(arr)`      | Array<T> -> Int              | Array length             |
| `arr.push(val)` | (mutable Array<T>, T) -> Void| Append to array          |

## Release builds

`intentc build --strip-contracts <file.intent>` drops runtime contract checks from the emitted output (Phase 22 / [ADR 0033](docs/decisions/0033-release-flag-strip-policy.md)).

- **Rust target:** every contract `assert!(...)` becomes `debug_assert!(...)`. `intentc build` already passes `--release` to cargo, and cargo's release profile sets `debug_assertions = false`, so the calls compile out entirely. Binary size drops; hot-path overhead goes to zero on the contract layer.
- **JS target:** the `if (!(cond)) throw new Error("Precondition/Postcondition/Invariant failed: ...")` lines are omitted from emitted source.
- **WASM target:** flag is accepted but has no effect today (WASM backend doesn't emit contract checks).

User-written assertion builtins in test bodies (`assert(...)`, `assert_eq(...)`, `assert_close(...)`, `assert_panics(...)`) are **not** affected by `--strip-contracts` — they're the runtime assertion API (ADR 0029), not contracts.

When `--strip-contracts` is set, `intentc` prints a one-line warning to stderr:

```
warning: --strip-contracts removes runtime contract checks; run 'intentc verify' to confirm safety properties.
```

The safe pattern: run `intentc verify` over a release codebase before shipping with `--strip-contracts`. Verification confirms the contracts that would have asserted at runtime are statically proven; once that pass is clean, stripping is sound. Without verification, stripping trades runtime safety for performance — make the trade deliberately.

A future ADR will scope verification-aware stripping (`--strip-contracts=verified`) that only drops Z3-proven contracts, leaving unproven ones as a runtime net.

## Editor support

`intentc lsp` starts a stdio LSP 3.17 server. The VS Code extension under `editors/vscode/` activates on `.intent` files and spawns the server. Build it with `cd editors/vscode && npm install && npm run package`, then install via `code --install-extension intent-vscode-<version>.vsix`. The extension assumes `intentc` is on `PATH`; override with the `intent.binaryPath` setting.

Other editors (Neovim, Helix, Emacs, JetBrains) can wire `intentc lsp` into their LSP clients directly; the protocol is plain LSP 3.17 over stdio.

V1 surfaces (see [ADR 0032](docs/decisions/0032-lsp-v1-surface.md), revised through Phase 21):
- Diagnostics — parser, checker, lint, Z3 verification (verification runs on save, async)
- Hover — signature + contracts on top-level decls; type info on locals, params, `self`, fields, and methods
- Go-to-definition — same-file, same-package, and cross-package (via `intent.toml` `[dependencies]`), including locals, params, methods, fields
- Find references — every use of a top-level decl or a local across the workspace; `includeDeclaration` honoured. Method and field refs are deferred.
- Document symbols — outline view (top-level decls + entity members + enum variants)
- Formatting — runs `intentc fmt` via `textDocument/formatting`
- Signature help — parameter info inside function and single-step method calls
- Completion — identifier suggestions (locals + top-level decls + sibling-package decls + keywords + built-in types); plus member completion on `receiver.` returning the entity's fields and methods (single-step receivers; `self` resolved in methods and constructors)
- Semantic tokens — full-document type-aware highlighting (`textDocument/semanticTokens/full`)

Out of scope for v1 (filed as v1.1+ follow-ups): chained member access (`a.b.c`), find-references on methods/fields, rename, code actions, refactorings, inlay hints, Marketplace publishing.

## Operators

### Arithmetic (Int or Float, both operands same type)
`+`, `-`, `*`, `/`, `%` (modulo is Int only)

### String
`+` (concatenation)

### Comparison (same type operands)
`==`, `!=`, `<`, `>`, `<=`, `>=`

### Logical (Bool operands)
`and`, `or`, `not`, `implies`

### Precedence (low to high)
1. `implies` (right-associative)
2. `or`
3. `and`
4. `not` (prefix)
5. `==`, `!=`
6. `<`, `>`, `<=`, `>=`
7. `+`, `-`
8. `*`, `/`, `%`
9. `-` (unary negation)
10. `.`, `()`, `[]`, `?`

## Compilation

```bash
# Native binary (default target: Rust)
intentc build <file.intent>                         # compile to native binary
intentc build --emit <file.intent>                  # emit generated Rust source

# JavaScript target
intentc build --target js <file.intent>             # generate JavaScript file
intentc build --target js --emit <file.intent>      # emit JS to stdout

# WebAssembly target
intentc build --target wasm <file.intent>           # compile to .wasm via Rust

# Other commands
intentc check <file.intent>                         # type-check without compiling
intentc verify <file.intent>                        # verify contracts with Z3 SMT solver
intentc fmt <file.intent>                           # format source to canonical style
intentc fmt --check <file.intent>                   # check formatting (exit 1 if not formatted)
intentc lint <file.intent>                          # lint for style issues
intentc test-gen <file.intent>                      # generate property-based tests
intentc test-gen --emit <file.intent>               # write tests to _test.rs file
```

Requires Go (to build the compiler) and Rust/Cargo (to compile generated code). Z3 is optional (for `verify` command).

Multi-file projects are detected automatically when the entry file contains `import` declarations.

### Multi-Target Output

All targets enforce the same contracts at runtime. A precondition failure throws an `Error` in JavaScript, panics in Rust, and traps in WebAssembly.

| Intent Contract | Rust Output | JavaScript Output |
|----------------|-------------|-------------------|
| `requires expr` | `assert!(expr, "Precondition failed: ...")` | `if (!(expr)) throw new Error("Precondition failed: ...")` |
| `ensures expr` | `assert!(expr, "Postcondition failed: ...")` | `if (!(expr)) throw new Error("Postcondition failed: ...")` |
| `invariant expr` | `assert!(expr, "Invariant failed: ...")` | `if (!(expr)) throw new Error("Invariant failed: ...")` |
| `old(expr)` | `let __old_N = expr;` before body | `const __old_N = expr;` before body |
| `match` | Rust `match` with patterns | Chain of `if (__scrutinee._tag === "...")` |
| `entity` | `struct` + `impl` | `class` with `__checkInvariants()` |
| `enum` | Rust `enum` | Object with factory functions + `_tag` field |

## Code Generation Summary

### Rust Target

| Intent                  | Rust                                    |
|-------------------------|-----------------------------------------|
| `Int`                   | `i64`                                   |
| `Float`                 | `f64`                                   |
| `String`                | `String`                                |
| `Bool`                  | `bool`                                  |
| `Array<T>`              | `Vec<T>`                                |
| `Result<T, E>`          | `Result<T, E>`                          |
| `Option<T>`             | `Option<T>`                             |
| `requires expr`         | `assert!(expr, "Precondition failed")`  |
| `ensures expr`          | `assert!(expr, "Postcondition failed")` |
| `invariant expr`        | `assert!(expr, "Invariant failed")`     |
| `forall i in a..b: p`   | runtime loop checking p for each i     |
| `match`                 | Rust `match`                            |
| `Ok(v)`, `Err(e)`      | `Ok(v)`, `Err(e)`                       |
| `Some(v)`, `None`       | `Some(v)`, `None`                       |
| `expr?`                 | `expr?` (in Result-returning functions) |
| `entity`                | `struct` + `impl`                       |
| `enum`                  | Rust `enum`                             |
| `print(x)`             | `println!("{}", x)`                     |

### JavaScript Target

| Intent                  | JavaScript                              |
|-------------------------|-----------------------------------------|
| `Int`                   | `number`                                |
| `Float`                 | `number`                                |
| `String`                | `string`                                |
| `Bool`                  | `boolean`                               |
| `Array<T>`              | `Array`                                 |
| `requires expr`         | `if (!(expr)) throw new Error(...)`     |
| `ensures expr`          | `if (!(expr)) throw new Error(...)`     |
| `invariant expr`        | `if (!(expr)) throw new Error(...)`     |
| `forall i in a..b: p`   | runtime loop checking p for each i     |
| `match`                 | `if/else if` chain on `_tag`            |
| `entity`                | `class` with `__checkInvariants()`      |
| `enum`                  | Object with factory functions            |
| `print(x)`             | `console.log(x)`                        |

## Guidelines for AI Code Generation

1. **Write contracts first.** Before implementing a function body, write the `requires` and `ensures` clauses. The human audits contracts, not your implementation.

2. **Every function should have contracts.** At minimum, use `requires true` and `ensures true` as placeholders if no meaningful contract applies. Prefer meaningful contracts.

3. **Entities need invariants.** If an entity has state that should be constrained, declare `invariant` clauses. The compiler checks them after every mutation.

4. **Use intent blocks for high-level goals.** Group related contracts under natural-language descriptions. Link every goal to a specific `verified_by` reference.

5. **Prefer `Result<T, E>` over panics.** Functions that can fail should return `Result` and use contracts on the Ok/Err paths.

6. **Use quantifiers for collection contracts.** `forall` and `exists` express properties over arrays that would be verbose as manual loops.

7. **All types are explicit.** Never omit type annotations. `let x: Int = 5;` not `let x = 5;`.

8. **Use `old()` in method postconditions.** When a method mutates entity state, the `ensures` clause should relate new state to old state.

9. **Exhaustive matching.** Match expressions must cover all enum variants or include a wildcard `_`.

10. **Multi-file organization.** Separate concerns into modules. Mark the public API with `public`. Keep internal helpers private.

## Complete Example

A contract-verified array utility module:

```
module array_utils version "1.0.0";

intent "Array search correctness" {
    goal "find_index returns the correct position of a target element";
    goal "is_sorted verifies array ordering";
    guarantee "find_index returns -1 only when target is absent";
    verified_by find_index.ensures;
    verified_by is_sorted.ensures;
}

public function find_index(arr: Array<Int>, target: Int) returns Int
    requires len(arr) > 0
    ensures (result >= 0) implies (result < len(arr))
    ensures (result >= 0) implies (arr[result] == target)
{
    for i in 0..len(arr) {
        if arr[i] == target {
            return i;
        }
    }
    return -1;
}

public function is_sorted(arr: Array<Int>) returns Bool
    requires len(arr) > 0
    ensures (result == true) implies forall i in 0..len(arr) - 1: arr[i] <= arr[i + 1]
{
    if len(arr) == 1 {
        return true;
    }
    let mutable i: Int = 0;
    while i < len(arr) - 1
        invariant i >= 0
        invariant i <= len(arr) - 1
        decreases len(arr) - 1 - i
    {
        if arr[i] > arr[i + 1] {
            return false;
        }
        i = i + 1;
    }
    return true;
}

public function sum(arr: Array<Int>) returns Int
    requires len(arr) > 0
    ensures result >= 0
{
    let mutable total: Int = 0;
    for x in arr {
        total = total + x;
    }
    return total;
}
```
