# Intent Programming Language -- Design Document

**Version:** 0.1.0 (Proof of Concept)
**Status:** Draft Specification

---

## Table of Contents

1. [Overview](#1-overview)
2. [Design Philosophy](#2-design-philosophy)
3. [Lexical Structure](#3-lexical-structure)
4. [Type System](#4-type-system)
5. [Module System](#5-module-system)
6. [Functions](#6-functions)
7. [Entities](#7-entities)
8. [Intent Blocks](#8-intent-blocks)
9. [Statements](#9-statements)
10. [Expressions](#10-expressions)
11. [Contract System](#11-contract-system)
12. [Compilation Model](#12-compilation-model)
13. [Rust Mapping](#13-rust-mapping)
14. [Complete Example](#14-complete-example)
15. [Package Management](#15-package-management)
16. [Test Declarations and Annotations](#16-test-declarations-and-annotations)
17. [Extern Functions (Rust FFI)](#17-extern-functions-rust-ffi)
18. [In-Language Testing](#18-in-language-testing)
19. [Editor Support](#19-editor-support)

---

## 1. Overview

Intent is a programming language designed for AI code assistants to write, that compiles to native binaries for humans to use. The toolchain is built in Go, transpiles to Rust, and produces native binaries via `cargo build`.

The language prioritizes **explicit contracts**, **declared intent**, and **verifiable correctness** over brevity or ergonomic shortcuts. Every function carries preconditions and postconditions. Every data type carries invariants. Every module carries intent blocks that declare the programmer's goals in natural language and link them to formal verification points.

The compilation pipeline is:

```
.intent source -> Lexer -> Parser -> Semantic Checker -> Rust Code Generator -> cargo build -> Native Binary
```

The Go toolchain handles lexing, parsing, semantic analysis, and Rust code generation. The Rust compiler (`rustc`, invoked via `cargo build`) handles optimization, borrow checking, and native code generation.

### 1.1 Goals

- Provide a language where AI assistants can express rich semantic information that would be too verbose for humans to maintain.
- Compile to efficient native binaries through Rust as an intermediate representation.
- Verify at compile time that all declared intents reference valid contracts.
- Generate runtime assertions for all preconditions, postconditions, and invariants.
- Keep the proof-of-concept scope minimal: built-in generic containers only, no user-defined generics.

### 1.2 Implemented Since POC

The following features were originally non-goals but have been implemented:

- **Traits** with static dispatch and `impl` blocks (Phase 6).
- **User-defined enums** beyond `Result<T,E>` and `Option<T>` (Phase 5).
- **Map<K,V>** type with compile-time key type validation (Phase 9).
- **I/O standard library** -- file operations, environment access, `to_string` (Phase 7).
- **HTTP and JSON builtins** -- `http_post`, `http_get`, `json_parse`, `json_get`, `json_path` (Phase 8).
- **String interpolation** (Phase 9).
- **Quantifier expressions** -- `forall` and `exists` in contracts.
- **Result<T,E> error handling** with `match` and try operator `?` (Phase 5).
- **Multi-target codegen** -- Rust and JavaScript backends via IR layer (Phase 4).
- **Multi-file compilation** with cross-module imports (Phase 8).
- **Closures and first-class functions** -- `Fn(T) -> R` types, lambda expressions (Phase 10).
- **User-defined generics** -- `entity Stack<T>`, `function identity<T>()`, monomorphization-based compilation (Phase 11).
- **Async/await concurrency** -- `async function`, `await`, `spawn`, `Future<T>` (Phase 12).
- **Package management** -- `intent.toml` manifests, semver constraints, local path dependencies, package cache, `intentc pkg` CLI (Phase 13).

### 1.3 Remaining Non-Goals

- Operator overloading.
- Inheritance or subtyping.

---

## 2. Design Philosophy

AI code assistants do not share human cognitive constraints. A human programmer benefits from concise syntax, implicit behavior, and syntactic sugar because these reduce the cognitive load of reading and writing code. An AI assistant has no such constraint. It can produce and consume arbitrarily verbose, maximally explicit code without fatigue.

Intent exploits this asymmetry. The language is optimized for **machine authorship** and **human auditability**:

- **Maximal explicitness.** Every function declares its preconditions and postconditions. Every entity declares its invariants. There are no implicit conversions, no default values, no hidden control flow.

- **Rich contracts.** The `requires` and `ensures` clauses on functions and methods form a contract system that documents behavior and generates runtime checks. The `invariant` clauses on entities ensure data consistency across all mutations.

- **Declared intent.** The `intent` block lets the programmer (or AI assistant) state in natural language *what* a piece of code is supposed to accomplish and *why*, then link those statements to specific contracts via `verified_by` references. The compiler verifies that all such references resolve to actual contracts.

- **Compilation to a trusted backend.** By transpiling to Rust, Intent inherits Rust's memory safety, performance, and mature toolchain without reimplementing those concerns.

- **Auditability over convenience.** A human reviewer can read Intent source and see every assumption, every guarantee, and every stated goal. The compiler ensures these declarations are structurally consistent. The generated Rust code can be inspected to confirm the mapping.

---

## 3. Lexical Structure

### 3.1 Source Encoding

Intent source files use UTF-8 encoding. The file extension is `.intent`.

### 3.2 Comments

Intent supports two comment forms:

```
// Line comment: extends to end of line

/* Block comment:
   can span multiple lines */
```

Block comments do not nest.

### 3.3 Keywords

The following identifiers are reserved keywords:

```
module    version   function  entry     returns
requires  ensures   let       mutable   return
if        else      true      false     entity
invariant constructor method  self      result
old       intent    goal      constraint guarantee
verified_by and     or        not       implies
while     for       in        break     continue
enum      match     import    trait     impl
forall    exists    try       Ok        Err
Some      None      async     await     spawn
```

### 3.4 Identifiers

Identifiers begin with a letter (a-z, A-Z) or underscore, followed by zero or more letters, digits (0-9), or underscores.

```
identifier = (letter | '_') (letter | digit | '_')*
```

### 3.5 Literals

**Integer literals** are sequences of decimal digits, optionally preceded by a minus sign at the expression level (unary negation).

```
42
0
1000000
```

**Float literals** are decimal numbers containing a dot, with digits on both sides.

```
3.14
0.0
100.5
```

**String literals** are enclosed in double quotes. The following escape sequences are supported:

```
\"  -> double quote
\\  -> backslash
\n  -> newline
\t  -> tab
\r  -> carriage return
```

Example: `"Hello, world!\n"`

**Boolean literals** are the keywords `true` and `false`.

### 3.6 Operators and Punctuation

```
+   -   *   /   %
==  !=  <   >   <=  >=
=
(   )   {   }
;   ,   .
```

The word operators `and`, `or`, `not`, and `implies` are keywords, not symbolic operators.

### 3.7 Whitespace

Spaces, tabs, carriage returns, and newlines are whitespace. Whitespace separates tokens but is otherwise insignificant. Intent is not indentation-sensitive.

---

## 4. Type System

Intent uses a nominal type system with no subtyping and no type inference beyond literal types. It supports built-in generic container types (`Array<T>`, `Map<K,V>`, `Result<T,E>`, `Option<T>`) and user-defined generic entities and functions.

### 4.1 Primitive Types

| Intent Type | Rust Mapping | Description |
|-------------|-------------|-------------|
| `Int`       | `i64`       | 64-bit signed integer |
| `Float`     | `f64`       | 64-bit IEEE 754 floating-point |
| `String`    | `String`    | Owned UTF-8 string, codepoint-indexed (ADR 0041) |
| `Bool`      | `bool`      | Boolean (`true` or `false`) |
| `Char`      | `char`      | Unicode scalar value (ADR 0041, Phase 31) |
| `Void`      | `()`        | Unit type, no value |

### 4.2 Generic Container Types

Intent provides four built-in generic types and supports user-defined generic entities and functions via monomorphization.

| Intent Type | Rust Mapping | JS Mapping | Description |
|-------------|-------------|------------|-------------|
| `Array<T>` | `Vec<T>` | `Array` | Ordered, growable sequence |
| `Map<K,V>` | `HashMap<K,V>` | `Map` | Key-value associative container |
| `Result<T,E>` | `Result<T,E>` | tagged object | Success-or-error wrapper |
| `Option<T>` | `Option<T>` | tagged object | Present-or-absent wrapper |

**Map methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `get(key, default)` | `(K, V) -> V` | Returns value for key, or default if absent |
| `set(key, value)` | `(K, V) -> Void` | Inserts or updates key-value pair (requires `mutable`) |
| `contains(key)` | `(K) -> Bool` | Returns true if key exists |
| `keys()` | `() -> Array<K>` | Returns all keys as an array |
| `remove(key)` | `(K) -> Void` | Removes key-value pair (requires `mutable`) |

The `len()` builtin works on both `Array<T>` and `Map<K,V>`.

Empty maps are initialized with `[]` and a type annotation: `let mutable m: Map<String, Int> = [];`

**Result<T,E> constructors and methods:**

| Constructor/Method | Signature | Description |
|-------------------|-----------|-------------|
| `Ok(value)` | `(T) -> Result<T,E>` | Wraps a success value |
| `Err(error)` | `(E) -> Result<T,E>` | Wraps an error value |
| `is_ok()` | `() -> Bool` | Returns true if Ok |
| `is_err()` | `() -> Bool` | Returns true if Err |
| `?` (try operator) | postfix | Unwraps Ok or propagates Err to enclosing function |

Result values are destructured with `match`:

```
let val: Int = match result {
    Ok(v) => v,
    Err(e) => 0,
};
```

**Option<T> constructors and methods:**

| Constructor/Method | Signature | Description |
|-------------------|-----------|-------------|
| `Some(value)` | `(T) -> Option<T>` | Wraps a present value |
| `None` | `Option<T>` | Represents absence |
| `is_some()` | `() -> Bool` | Returns true if Some |
| `is_none()` | `() -> Bool` | Returns true if None |

Option values are destructured with `match`:

```
let val: Int = match opt {
    Some(v) => v,
    None => -1,
};
```

### 4.3 Entity Types

Entity types are user-defined nominal types, declared with the `entity` keyword (see Section 7). Each entity declaration introduces a new type whose name can be used as a type annotation. Entity types map to Rust structs.

### 4.4 Type Rules

- Arithmetic operators (`+`, `-`, `*`, `/`, `%`) require both operands to have the same numeric type (`Int` or `Float`). The result has that same type. Mixed `Int`/`Float` arithmetic is a compile-time error.
- The `+` operator is also defined for `String` operands (concatenation). The result is `String`.
- Comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`) require both operands to have the same type. Equality (`==`, `!=`) is defined for all types. Ordering (`<`, `>`, `<=`, `>=`) is defined for `Int` and `Float` only. The result is always `Bool`.
- Logical operators (`and`, `or`, `not`, `implies`) require `Bool` operands and produce `Bool`.
- Assignment requires the right-hand side type to match the variable's declared type exactly.
- Function call arguments must match parameter types exactly. The call expression's type is the function's return type.
- Method call arguments must match parameter types exactly. The call expression's type is the method's return type.
- Field access on an entity-typed expression produces the field's declared type.

### 4.5 No Implicit Conversions

There are no implicit type conversions. `Int` does not implicitly convert to `Float`, `Bool` does not implicitly convert to `Int`, and so on. All conversions, if needed, must be explicit (and in the POC, no conversion functions are provided -- this is a known limitation).

### 4.6 Future Type

| Intent Type | Rust Mapping | JS Mapping | Description |
|-------------|-------------|------------|-------------|
| `Future<T>` | `impl Future<Output=T>` | `Promise<T>` | Pending async computation |

`Future<T>` is the return type of `spawn` expressions. A `Future<T>` value represents an asynchronous computation that will eventually produce a value of type `T`. Use `await` to obtain the result.

### 4.7 Trait Types

Traits define shared method interfaces that multiple entities can implement:

```
trait Handler {
    method execute(ctx: Context) returns Int
        requires ctx.get_value() >= 0
        ensures result >= 0;
}
```

Entities implement traits via `impl` blocks:

```
impl Handler for StartHandler {
    method execute(ctx: Context) returns Int {
        return self.code + ctx.get_value();
    }
}
```

**Key rules:**

- Traits contain method signatures only (no bodies, no default implementations).
- `impl` blocks must implement all methods declared in the trait -- no missing, no extra.
- Parameter count, parameter types, and return types must match exactly.
- Contracts (`requires`/`ensures`) on trait methods apply to all implementations. Impl methods may add additional contracts.
- Trait methods are merged into the entity's method table for resolution -- callers use `entity.method()` syntax regardless of whether the method is defined directly or via a trait impl.
- **Static dispatch only** -- no trait objects, no dynamic dispatch, no vtables. The caller always knows the concrete type at compile time.
- Impl blocks must be in the same module as either the trait or the entity.

---

## 5. Module System

Every Intent source file must begin with a module declaration:

```
module <name> version "<semver>";
```

Where:
- `<name>` is an identifier naming the module.
- `<semver>` is a semantic version string in the form `"MAJOR.MINOR.PATCH"` (e.g., `"1.0.0"`, `"0.2.3"`).

Example:

```
module calculator version "1.0.0";
```

### 5.1 Module Rules

- The module declaration must be the first non-comment construct in the file.
- Each file contains exactly one module declaration.
- The module name is used in the generated Rust code as documentation and in diagnostic messages.
- The version string is recorded but not semantically checked in the POC (no import system exists yet).

### 5.2 Top-Level Declarations

After the module declaration, the file contains zero or more top-level declarations in any order:

- Function declarations
- Entity declarations
- Enum declarations
- Trait declarations
- Impl blocks (trait implementations for entities)
- Intent blocks
- Import statements (`import "<path>.intent";`)

---

## 6. Functions

### 6.1 Declaration Syntax

```
function <name>(<parameters>) returns <Type> {
    <body>
}
```

Or with the `entry` modifier:

```
entry function main() returns Int {
    <body>
}
```

### 6.2 Parameters

Parameters are a comma-separated list of `name: Type` pairs:

```
function add(a: Int, b: Int) returns Int {
    return a + b;
}
```

A function may have zero parameters:

```
function greet() returns String {
    return "Hello";
}
```

### 6.3 Entry Point

A program must have exactly one function marked with the `entry` keyword. This function must be named `main` and must return `Int`. The returned integer becomes the process exit code.

```
entry function main() returns Int {
    return 0;
}
```

The entry function maps to a Rust `fn main()` that calls the Intent `main` function and uses `std::process::exit()` with the returned value.

### 6.4 Contracts

Functions may declare preconditions (`requires`) and postconditions (`ensures`) between the parameter list/return type and the opening brace:

```
function divide(a: Int, b: Int) returns Int
    requires b != 0
    ensures result * b + (a % b) == a
{
    return a / b;
}
```

**Requires clauses:**
- Appear after `returns <Type>` and before `{`.
- Each `requires` keyword is followed by a boolean expression.
- Multiple `requires` clauses may appear; all must hold.
- The expression may reference function parameters.
- At runtime, compiled to an `assert!()` at function entry.

**Ensures clauses:**
- Appear after all `requires` clauses (if any) and before `{`.
- Each `ensures` keyword is followed by a boolean expression.
- Multiple `ensures` clauses may appear; all must hold.
- The expression may reference function parameters and the keyword `result`, which refers to the function's return value.
- At runtime, compiled to an `assert!()` evaluated just before the function returns.

### 6.5 The `result` Keyword

The keyword `result` is valid only inside `ensures` clauses. It refers to the value that the function will return. In the generated Rust code, the function body is wrapped in a labeled block whose value is captured into a local variable, and the `ensures` assertions check that variable before it is returned.

### 6.6 Return Statements

A function body must contain at least one `return` statement if the return type is not `Void`. The expression in the `return` statement must have the same type as the declared return type.

For `Void` functions, `return;` (with no expression) is permitted, and falling off the end of the function body is equivalent to `return;`.

### 6.7 Async Functions

Functions can be marked `async` to enable asynchronous execution:

```
async function fetch_data(url: String) returns Result<String, String>
    requires len(url) > 0
{
    let response: String = http_get(url, "")?;
    return Ok(response);
}
```

**Async rules:**
- The `async` keyword precedes `function` (and optionally `entry`).
- Contracts work the same as synchronous functions: `requires` is checked at function entry, `ensures` is checked when the future resolves.
- `await expr` suspends execution until the future resolves, producing the inner value.
- `spawn expr` creates a concurrent task from an async function call, returning `Future<T>`.

**Example with spawn and await:**

```
async entry function main() returns Int {
    let f1: Future<Int> = spawn delayed_add(3, 4);
    let f2: Future<Int> = spawn delayed_add(10, 20);
    let r1: Int = await f1;
    let r2: Int = await f2;
    return 0;
}
```

**Built-in async functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `await_all(futures)` | `Array<Future<T>> -> Array<T>` | Wait for all futures |
| `await_any(futures)` | `Array<Future<T>> -> T` | Wait for first to complete |
| `timeout(future, ms)` | `(Future<T>, Int) -> Result<T, String>` | Timeout wrapper |
| `sleep(ms)` | `Int -> Future<Void>` | Async sleep |

---

## 7. Entities

Entities are Intent's user-defined data types, analogous to structs or classes. They have fields, invariants, constructors, and methods.

### 7.1 Declaration Syntax

```
entity <Name> {
    <members>
}
```

Where `<members>` is a sequence of field declarations, invariant declarations, constructor declarations, and method declarations, in any order.

### 7.2 Fields

Fields are declared as:

```
<name>: <Type>;
```

Example:

```
entity Point {
    x: Float;
    y: Float;
}
```

Fields are always private to the entity in the POC (there is no visibility system). They can be read via field access expressions (e.g., `p.x`) and modified within the entity's own methods.

### 7.3 Invariants

Invariants declare boolean conditions that must hold after every constructor and method execution:

```
invariant <expression>;
```

The expression may reference the entity's fields using `self.<field>` syntax.

Example:

```
entity PositiveCounter {
    value: Int;

    invariant self.value >= 0;
}
```

Multiple invariants may be declared; all must hold simultaneously. Invariants are compiled to a `__check_invariants(&self)` method in Rust, called at the end of every constructor and every method body.

### 7.4 Constructors

Constructors create new instances of an entity:

```
constructor(<parameters>)
    [requires <expression>]
    [ensures <expression>]
{
    <body>
}
```

Rules:
- An entity may have at most one constructor in the POC.
- Constructor parameters follow the same syntax as function parameters.
- The constructor body must assign all fields using `self.<field> = <expression>;` syntax.
- `requires` and `ensures` clauses work the same as for functions.
- In `ensures` clauses, `self.<field>` refers to the post-construction field values.
- After the constructor body executes (and ensures clauses are checked), all invariants are checked.

Example:

```
entity BankAccount {
    balance: Int;
    owner: String;

    invariant self.balance >= 0;

    constructor(initial_balance: Int, account_owner: String)
        requires initial_balance >= 0
        ensures self.balance == initial_balance
    {
        self.balance = initial_balance;
        self.owner = account_owner;
    }
}
```

### 7.5 Methods

Methods operate on entity instances:

```
method <name>(<parameters>) returns <Type>
    [requires <expression>]
    [ensures <expression>]
{
    <body>
}
```

Rules:
- Methods implicitly have access to `self`, the entity instance.
- Fields are accessed via `self.<field>`.
- Methods may modify fields via `self.<field> = <expression>;` if the field needs to be mutated.
- `requires` and `ensures` clauses may reference parameters, `self.<field>`, `result`, and `old()` expressions.
- After the method body executes (and ensures clauses are checked), all invariants are checked.

Example:

```
method deposit(amount: Int) returns Void
    requires amount > 0
    ensures self.balance == old(self.balance) + amount
{
    self.balance = self.balance + amount;
}
```

### 7.6 The `old()` Expression

The `old(<expression>)` construct is valid only inside `ensures` clauses of methods. It captures the value of the expression as it was at method entry, before any mutations.

In the generated Rust code, each `old(expr)` is compiled to a local variable that saves the value of `expr` at the beginning of the method body.

Example:

```
method withdraw(amount: Int) returns Bool
    requires amount > 0
    ensures (result == true) implies (self.balance == old(self.balance) - amount)
    ensures (result == false) implies (self.balance == old(self.balance))
{
    if self.balance >= amount {
        self.balance = self.balance - amount;
        return true;
    } else {
        return false;
    }
}
```

### 7.7 Entity Instantiation

Entities are instantiated by calling the constructor with the entity name:

```
let account: BankAccount = BankAccount(1000, "Alice");
```

This maps to the Rust constructor function (e.g., `BankAccount::new(1000, "Alice".to_string())`).

### 7.8 Method Calls

Methods are called with dot syntax:

```
account.deposit(500);
```

This maps to a Rust method call on the struct.

---

## 8. Intent Blocks

Intent blocks are the language's signature feature. They allow the programmer (or AI assistant) to declare the purpose of a section of code in natural language and link those declarations to formal contracts.

### 8.1 Syntax

```
intent "<description>" {
    goal "<text>";
    constraint "<text>";
    guarantee "<text>";
    verified_by <entity>.<member>.<clause>;
}
```

All clauses within an intent block are optional and may appear in any order, with any number of repetitions.

### 8.2 Clauses

**`goal`**: A natural-language statement of what the code aims to accomplish.

```
goal "Ensure all bank account balances remain non-negative";
```

**`constraint`**: A natural-language statement of a restriction or limitation.

```
constraint "Withdrawals must not exceed the current balance";
```

**`guarantee`**: A natural-language statement of what the code promises to deliver.

```
guarantee "Deposits always increase the balance by the exact deposit amount";
```

**`verified_by`**: A reference to a specific contract in the codebase. The compiler resolves this reference and emits an error if it does not exist. The syntax is a dot-separated path:

```
verified_by BankAccount.balance_non_negative;       // references an invariant
verified_by BankAccount.deposit.requires;            // references a method's requires clause
verified_by BankAccount.deposit.ensures;             // references a method's ensures clause
verified_by BankAccount.constructor.requires;        // references the constructor's requires clause
verified_by BankAccount.withdraw.ensures;            // references a method's ensures clause
```

The resolution rules for `verified_by` paths:

- `<Entity>.<invariant_name>`: References a named invariant. Since invariants in the POC grammar are unnamed, they are referenced by a compiler-assigned name based on order (e.g., `invariant_0`, `invariant_1`). Alternatively, the path `<Entity>.invariant` references the presence of any invariant on that entity.
- `<Entity>.<method_name>.requires`: References the requires clause(s) of a method.
- `<Entity>.<method_name>.ensures`: References the ensures clause(s) of a method.
- `<Entity>.constructor.requires`: References the constructor's requires clause(s).
- `<Entity>.constructor.ensures`: References the constructor's ensures clause(s).
- `<function_name>.requires`: References a top-level function's requires clause(s).
- `<function_name>.ensures`: References a top-level function's ensures clause(s).

### 8.3 Compiler Verification

During semantic analysis, the compiler resolves every `verified_by` reference. If a reference points to a declaration that does not exist (e.g., a method that has no `ensures` clause, or an entity that does not exist), the compiler emits an error. This ensures that all declared intents are backed by actual contracts in the code.

### 8.4 Compilation

Intent blocks do not produce executable code. They are compiled to:

1. **Structured comments** in the generated Rust source, preserving the intent documentation.
2. **A `#[cfg(test)]` module** containing compile-time checks (as `const` assertions or test functions) that verify the structural relationships still hold.

---

## 9. Statements

### 9.1 Let Bindings

Let bindings introduce new variables:

```
let <name>: <Type> = <expression>;
```

By default, variables are immutable. To declare a mutable variable:

```
let mutable <name>: <Type> = <expression>;
```

The type annotation is required (no type inference in the POC). The initializer expression is required (no uninitialized variables).

Examples:

```
let x: Int = 42;
let mutable counter: Int = 0;
let name: String = "Alice";
```

### 9.2 Assignment

Assignment modifies a mutable variable or an entity field through `self`:

```
<name> = <expression>;
self.<field> = <expression>;
```

Assignment to an immutable variable is a compile-time error. Assignment to `self.<field>` is only valid within methods and constructors of the owning entity.

### 9.3 Return Statement

```
return <expression>;
return;
```

The expression form is required when the enclosing function/method has a non-`Void` return type. The bare form is valid only in `Void` functions/methods.

### 9.4 If/Else Statement

```
if <expression> {
    <statements>
}

if <expression> {
    <statements>
} else {
    <statements>
}

if <expression> {
    <statements>
} else if <expression> {
    <statements>
} else {
    <statements>
}
```

The condition expression must have type `Bool`. There is no ternary operator and no `if` expression form. Braces are always required around the bodies.

### 9.5 Expression Statement

Any expression followed by a semicolon is a valid statement:

```
do_something();
account.deposit(100);
```

The result of the expression is discarded.

### 9.6 While Loop

```
while <expression> {
    <statements>
}
```

The condition expression must have type `Bool`. The body executes repeatedly while the condition is true.

### 9.7 For-In Loop (Range)

```
for <name> in <start>..<end> {
    <statements>
}
```

Iterates `<name>` from `<start>` (inclusive) to `<end>` (exclusive). Both bounds must be `Int` expressions. The loop variable is implicitly `Int` and immutable within the body.

### 9.8 Break and Continue

```
break;
continue;
```

`break` exits the innermost enclosing `while` or `for` loop. `continue` skips to the next iteration. Both are compile-time errors outside of loops.

---

## 10. Expressions

### 10.1 Precedence and Associativity

From lowest to highest precedence:

| Precedence | Operators         | Associativity | Description         |
|------------|-------------------|---------------|---------------------|
| 1          | `implies`         | Right         | Logical implication |
| 2          | `or`              | Left          | Logical disjunction |
| 3          | `and`             | Left          | Logical conjunction |
| 4          | `not`             | Prefix        | Logical negation    |
| 5          | `==` `!=`         | Left          | Equality            |
| 6          | `<` `>` `<=` `>=` | Left          | Ordering            |
| 7          | `+` `-`           | Left          | Additive            |
| 8          | `*` `/` `%`       | Left          | Multiplicative      |
| 9          | `-` (unary)       | Prefix        | Negation            |
| 10         | `.` `()`          | Left          | Access, call        |

### 10.2 Primary Expressions

**Integer literal**: `42`
**Float literal**: `3.14`
**String literal**: `"hello"`
**Boolean literal**: `true`, `false`
**Identifier**: `x`, `counter`
**Self**: `self` (valid only in methods and constructors)
**Result**: `result` (valid only in `ensures` clauses)
**Parenthesized**: `(expr)`

### 10.3 Arithmetic Expressions

```
a + b       // addition (Int+Int->Int, Float+Float->Float, String+String->String)
a - b       // subtraction (Int, Float)
a * b       // multiplication (Int, Float)
a / b       // division (Int: truncating, Float: IEEE 754)
a % b       // modulo (Int only)
-a          // unary negation (Int, Float)
```

### 10.4 Comparison Expressions

```
a == b      // equality (all types)
a != b      // inequality (all types)
a < b       // less than (Int, Float)
a > b       // greater than (Int, Float)
a <= b      // less or equal (Int, Float)
a >= b      // greater or equal (Int, Float)
```

### 10.5 Logical Expressions

Intent uses word operators for logical operations, avoiding symbol overloading:

```
a and b         // logical AND
a or b          // logical OR
not a           // logical NOT
a implies b     // logical implication (equivalent to: (not a) or b)
```

All operands must be `Bool`. The result is `Bool`.

The `implies` operator is right-associative: `a implies b implies c` parses as `a implies (b implies c)`.

### 10.6 Function Calls

```
function_name(arg1, arg2, ...)
```

Arguments are evaluated left to right. The number and types of arguments must match the function's parameter list exactly.

### 10.7 Entity Construction

```
EntityName(arg1, arg2, ...)
```

Syntactically identical to a function call. The semantic checker distinguishes entity construction from function calls by looking up the name: if it resolves to an entity, it is construction; if it resolves to a function, it is a function call.

### 10.8 Method Calls

```
expr.method_name(arg1, arg2, ...)
```

The expression before the dot must have an entity type. The method must be defined on that entity. The `self` parameter is implicit and not listed in the argument list.

### 10.9 Field Access

```
expr.field_name
```

The expression before the dot must have an entity type. The field must exist on that entity.

### 10.10 The `old()` Expression

```
old(expr)
```

Valid only in `ensures` clauses of methods. See Section 7.6.

---

## 11. Contract System

The contract system is Intent's core mechanism for verifiable correctness. It consists of three components: preconditions, postconditions, and invariants.

### 11.1 Preconditions (`requires`)

Preconditions state what must be true when a function, method, or constructor is called. They are the caller's responsibility.

```
function sqrt(n: Float) returns Float
    requires n >= 0.0
{
    // ...
}
```

At runtime, preconditions are checked at function entry. A precondition violation causes a panic with a descriptive message including the source location and the condition text.

### 11.2 Postconditions (`ensures`)

Postconditions state what the function, method, or constructor guarantees upon completion. They are the implementation's responsibility.

```
function abs(n: Int) returns Int
    ensures result >= 0
    ensures (n >= 0) implies (result == n)
    ensures (n < 0) implies (result == -n)
{
    if n < 0 {
        return -n;
    }
    return n;
}
```

At runtime, postconditions are checked just before the function returns. The function body is compiled so that its return value is captured into a local variable named `__result`. The `ensures` expressions are evaluated with `result` mapped to `__result`. If any postcondition fails, the program panics.

### 11.3 Invariants

Invariants state what must always be true about an entity's state. They are checked after every constructor and method execution.

```
entity SortedPair {
    first: Int;
    second: Int;

    invariant self.first <= self.second;
}
```

At runtime, invariants are compiled into a `__check_invariants(&self)` method. This method is called:
1. At the end of every constructor body (after `ensures` checks).
2. At the end of every method body (after `ensures` checks).

A failing invariant causes a panic with a descriptive message.

### 11.4 Contract Evaluation Order

For a method call, the evaluation order is:

1. Capture `old()` values (save pre-state).
2. Check `requires` clauses (preconditions).
3. Execute the method body.
4. Check `ensures` clauses (postconditions), with `result` and `old()` values available.
5. Check invariants.

For a constructor call:

1. Check `requires` clauses (preconditions).
2. Execute the constructor body.
3. Check `ensures` clauses (postconditions).
4. Check invariants.

For a top-level function call:

1. Check `requires` clauses (preconditions).
2. Execute the function body.
3. Check `ensures` clauses (postconditions), with `result` available.

### 11.5 Contract Expressions

Contract expressions (in `requires`, `ensures`, and `invariant` clauses) are boolean expressions using the same expression syntax as the rest of the language. Additionally:

- `result` is available in `ensures` clauses.
- `old(expr)` is available in `ensures` clauses of methods.
- `self.field` is available in entity contracts.

Contract expressions should be side-effect-free. In the POC, this is not enforced by the compiler, but calling functions with side effects in contract expressions produces undefined behavior of the contract system (the side effects will execute at check time).

---

## 12. Compilation Model

### 12.1 Pipeline

The Intent compiler (`intentc`) is a Go program that processes `.intent` source files through six phases:

```
Source (.intent)
    |
    v
[1. Lexer]  -->  Token stream
    |
    v
[2. Parser]  -->  Abstract Syntax Tree (AST)
    |
    v
[3. Semantic Checker]  -->  Annotated AST (types resolved, contracts validated)
    |
    v
[4. IR Lowering]  -->  Intermediate Representation (target-independent)
    |
    v
[5. Backend]  -->  Rust source (.rs) or JavaScript (.js)
    |
    v
[cargo build]  -->  Native binary (Rust path)
```

### 12.2 Phase 1: Lexing

The lexer converts UTF-8 source text into a stream of tokens. Each token carries:
- Token type (keyword, identifier, literal, operator, punctuation)
- Lexeme (the source text of the token)
- Source position (line and column)

The lexer handles:
- Skipping whitespace and comments.
- Recognizing keywords vs. identifiers.
- Parsing integer and float literals.
- Parsing string literals with escape sequences.
- Recognizing all operators and punctuation.
- Emitting an EOF token at end of input.

### 12.3 Phase 2: Parsing

The parser consumes the token stream and produces an AST. The parser is a recursive-descent parser (LL(1) with minor lookahead for disambiguation).

Key parsing decisions:
- The module declaration is parsed first.
- Top-level declarations are parsed in a loop until EOF.
- Function declarations are distinguished from entity declarations by keyword.
- Intent blocks are recognized by the `intent` keyword.
- Expression parsing uses precedence climbing for correct operator precedence.
- Entity construction vs. function call is syntactically ambiguous and resolved in the semantic phase.

### 12.4 Phase 3: Semantic Analysis

The semantic checker walks the AST and performs:

1. **Symbol resolution**: Build a symbol table of all top-level functions, entities, and their members.
2. **Type checking**: Verify that all expressions have consistent types per the rules in Section 4.3.
3. **Contract validation**: Ensure `result` is only used in `ensures` clauses, `old()` is only used in method `ensures` clauses, `self` is only used in entity members.
4. **Intent verification**: Resolve all `verified_by` references to actual contracts. Emit errors for unresolvable references.
5. **Entry point validation**: Ensure exactly one `entry function main() returns Int` exists.
6. **Mutability checking**: Ensure only `let mutable` variables are assigned to after declaration.

### 12.5 Phase 4: Rust Code Generation

The code generator walks the annotated AST and emits Rust source code. The mapping is detailed in Section 13.

### 12.6 Phase 5: Rust Compilation

The generated Rust source is placed in a Cargo project structure:

```
output/
  Cargo.toml
  src/
    main.rs
```

The compiler invokes `cargo build --release` to produce a native binary. Compiler errors from `rustc` are reported to the user (they indicate bugs in the Intent compiler's code generator, not in the Intent source).

---

## 13. Rust Mapping

This section specifies the exact mapping from Intent constructs to Rust code. This mapping is normative: a conforming Intent compiler must produce semantically equivalent Rust code (though the exact formatting may differ).

### 13.1 Module Declaration

```intent
module calculator version "1.0.0";
```

Maps to:

```rust
// Module: calculator
// Version: 1.0.0
```

The module declaration becomes a comment. No Rust module system features are used in the POC.

### 13.2 Primitive Types

| Intent | Rust |
|--------|------|
| `Int`  | `i64` |
| `Float`| `f64` |
| `String`| `String` |
| `Bool` | `bool` |
| `Void` | `()` |

### 13.3 Functions Without Contracts

```intent
function add(a: Int, b: Int) returns Int {
    return a + b;
}
```

Maps to:

```rust
fn add(a: i64, b: i64) -> i64 {
    return a + b;
}
```

### 13.4 Functions With `requires`

```intent
function divide(a: Int, b: Int) returns Int
    requires b != 0
{
    return a / b;
}
```

Maps to:

```rust
fn divide(a: i64, b: i64) -> i64 {
    assert!(b != 0, "Precondition failed: b != 0");
    return a / b;
}
```

Each `requires` clause becomes an `assert!()` at the beginning of the function body, before any other code.

### 13.5 Functions With `ensures`

```intent
function abs(n: Int) returns Int
    ensures result >= 0
{
    if n < 0 {
        return -n;
    }
    return n;
}
```

Maps to:

```rust
fn abs(n: i64) -> i64 {
    let __result: i64 = 'body: {
        if n < 0 {
            break 'body -n;
        }
        break 'body n;
    };
    assert!(__result >= 0, "Postcondition failed: result >= 0");
    return __result;
}
```

Key details:
- The entire function body is wrapped in a labeled block `'body: { ... }`.
- Every `return expr;` in the Intent source becomes `break 'body expr;` in the Rust code.
- The block's value is captured into `__result`.
- Each `ensures` clause becomes an `assert!()` after the block, with `result` replaced by `__result`.
- The actual Rust `return` happens after all postcondition checks.

### 13.6 Functions With Both `requires` and `ensures`

```intent
function divide(a: Int, b: Int) returns Int
    requires b != 0
    ensures result * b + (a % b) == a
{
    return a / b;
}
```

Maps to:

```rust
fn divide(a: i64, b: i64) -> i64 {
    assert!(b != 0, "Precondition failed: b != 0");
    let __result: i64 = 'body: {
        break 'body a / b;
    };
    assert!(__result * b + (a % b) == a, "Postcondition failed: result * b + (a % b) == a");
    return __result;
}
```

Precondition checks come before the body block. Postcondition checks come after.

### 13.7 Entry Function

```intent
entry function main() returns Int {
    return 0;
}
```

Maps to:

```rust
fn __intent_main() -> i64 {
    return 0;
}

fn main() {
    let __exit_code = __intent_main();
    std::process::exit(__exit_code as i32);
}
```

The Intent `main` is renamed to `__intent_main`. A Rust `main` function is generated that calls it and uses the return value as the exit code.

### 13.8 Entities

```intent
entity Point {
    x: Float;
    y: Float;
}
```

Maps to:

```rust
#[derive(Clone, Debug)]
struct Point {
    x: f64,
    y: f64,
}
```

### 13.9 Entity With Invariants

```intent
entity PositiveCounter {
    value: Int;

    invariant self.value >= 0;
}
```

Maps to:

```rust
#[derive(Clone, Debug)]
struct PositiveCounter {
    value: i64,
}

impl PositiveCounter {
    fn __check_invariants(&self) {
        assert!(self.value >= 0, "Invariant failed: self.value >= 0");
    }
}
```

### 13.10 Entity Constructor

```intent
entity BankAccount {
    balance: Int;

    invariant self.balance >= 0;

    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
    }
}
```

Maps to:

```rust
#[derive(Clone, Debug)]
struct BankAccount {
    balance: i64,
}

impl BankAccount {
    fn __check_invariants(&self) {
        assert!(self.balance >= 0, "Invariant failed: self.balance >= 0");
    }

    fn new(initial: i64) -> BankAccount {
        assert!(initial >= 0, "Precondition failed: initial >= 0");
        let mut __self = BankAccount {
            balance: 0,  // default initialization
        };
        __self.balance = initial;
        assert!(__self.balance == initial, "Postcondition failed: self.balance == initial");
        __self.__check_invariants();
        return __self;
    }
}
```

Key details:
- The constructor maps to an associated function `new()` that returns `Self`.
- A temporary `__self` is created with default values for all fields.
- The constructor body assigns fields via `__self.<field>`.
- `ensures` clauses reference `__self` for `self` references.
- `__check_invariants()` is called after the ensures checks.

### 13.11 Entity Methods

```intent
method deposit(amount: Int) returns Void
    requires amount > 0
    ensures self.balance == old(self.balance) + amount
{
    self.balance = self.balance + amount;
}
```

Maps to:

```rust
fn deposit(&mut self, amount: i64) {
    let __old_self_balance = self.balance;
    assert!(amount > 0, "Precondition failed: amount > 0");
    self.balance = self.balance + amount;
    assert!(self.balance == __old_self_balance + amount, "Postcondition failed: self.balance == old(self.balance) + amount");
    self.__check_invariants();
}
```

Key details:
- Methods map to Rust methods with `&mut self` (or `&self` if the method does not mutate any fields; in the POC, all methods use `&mut self` for simplicity).
- Each `old(expr)` in ensures clauses generates a local variable `__old_<mangled>` at the beginning of the method, before any precondition checks, that captures the current value of the expression.
- `old(self.balance)` maps to `__old_self_balance`, which is assigned `self.balance.clone()` (or just `self.balance` for Copy types) at method entry.
- In the ensures assertion, `old(self.balance)` is replaced with `__old_self_balance`.
- `__check_invariants()` is called after postcondition checks.

For methods with non-`Void` return types, the same labeled block pattern from Section 13.5 is used:

```rust
fn withdraw(&mut self, amount: i64) -> bool {
    let __old_self_balance = self.balance;
    assert!(amount > 0, "Precondition failed: amount > 0");
    let __result: bool = 'body: {
        if self.balance >= amount {
            self.balance = self.balance - amount;
            break 'body true;
        } else {
            break 'body false;
        }
    };
    assert!((__result == true) || (self.balance == __old_self_balance), "Postcondition failed: ...");
    self.__check_invariants();
    return __result;
}
```

### 13.12 Entity Instantiation

```intent
let account: BankAccount = BankAccount(1000);
```

Maps to:

```rust
let account = BankAccount::new(1000);
```

### 13.13 Method Calls

```intent
account.deposit(500);
```

Maps to:

```rust
account.deposit(500);
```

### 13.14 Let Bindings

```intent
let x: Int = 42;
let mutable counter: Int = 0;
```

Maps to:

```rust
let x: i64 = 42;
let mut counter: i64 = 0;
```

### 13.15 If/Else

```intent
if x > 0 {
    return x;
} else {
    return -x;
}
```

Maps to:

```rust
if x > 0 {
    return x;
} else {
    return -x;
}
```

The mapping is direct. (Inside labeled blocks, `return` becomes `break 'body`.)

### 13.16 Logical Operators

| Intent      | Rust       |
|-------------|------------|
| `a and b`   | `a && b`   |
| `a or b`    | `a \|\| b`  |
| `not a`     | `!a`       |
| `a implies b` | `!a \|\| b` |

### 13.17 String Operations

```intent
let greeting: String = "Hello, " + name;
```

Maps to:

```rust
let greeting: String = format!("{}{}", "Hello, ", name);
```

String concatenation with `+` maps to `format!()` in Rust to handle owned `String` values correctly.

### 13.18 Intent Blocks

```intent
intent "Banking safety" {
    goal "Ensure account balances never go negative";
    guarantee "Deposits always increase balance by exact amount";
    verified_by BankAccount.invariant;
    verified_by BankAccount.deposit.ensures;
}
```

Maps to:

```rust
// Intent: Banking safety
// Goal: Ensure account balances never go negative
// Guarantee: Deposits always increase balance by exact amount
// Verified by: BankAccount.invariant
// Verified by: BankAccount.deposit.ensures

#[cfg(test)]
mod __intent_banking_safety {
    // Intent verification: all verified_by references were resolved at compile time.
    // - BankAccount.invariant: verified
    // - BankAccount.deposit.ensures: verified
}
```

Intent blocks produce structured comments and an empty `#[cfg(test)]` module. The actual verification happens at Intent compile time (semantic analysis), not at Rust compile time. The test module serves as documentation and can be extended in future versions with property-based tests.

---

## 14. Complete Example

The following is a complete Intent program demonstrating all major features:

```intent
module bank version "0.1.0";

intent "Account safety" {
    goal "Maintain non-negative balances for all accounts";
    goal "Track all balance changes accurately";
    constraint "Withdrawals cannot exceed current balance";
    guarantee "Deposits always increase balance by exact amount";
    guarantee "Failed withdrawals leave balance unchanged";
    verified_by BankAccount.invariant;
    verified_by BankAccount.deposit.requires;
    verified_by BankAccount.deposit.ensures;
    verified_by BankAccount.withdraw.requires;
    verified_by BankAccount.withdraw.ensures;
}

entity BankAccount {
    balance: Int;
    owner: String;

    invariant self.balance >= 0;

    constructor(initial_balance: Int, account_owner: String)
        requires initial_balance >= 0
        ensures self.balance == initial_balance
    {
        self.balance = initial_balance;
        self.owner = account_owner;
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
        } else {
            return false;
        }
    }

    method get_balance() returns Int
        ensures result == self.balance
    {
        return self.balance;
    }
}

function print_balance(account_name: String, balance: Int) returns Void {
    // In POC, no actual print -- this is a placeholder
    return;
}

entry function main() returns Int {
    let mutable account: BankAccount = BankAccount(1000, "Alice");
    account.deposit(500);
    let success: Bool = account.withdraw(200);
    if success {
        let balance: Int = account.get_balance();
        print_balance("Alice", balance);
    }
    return 0;
}
```

This program compiles to a Rust source file containing:

- A `BankAccount` struct with fields `balance: i64` and `owner: String`.
- A `__check_invariants` method asserting `self.balance >= 0`.
- A `new` constructor with precondition and postcondition checks.
- `deposit`, `withdraw`, and `get_balance` methods with full contract checks.
- Structured comments preserving the intent block.
- A `main` function that exercises the account.

When compiled and run, the program exits with code 0. If any contract is violated at runtime, the program panics with a descriptive assertion failure message.

---

## 15. Package Management

Intent supports package management for code reuse across projects. Packages are Intent projects with an `intent.toml` manifest at the root.

### 15.1 Manifest Format (`intent.toml`)

```toml
[package]
name = "graph-validation"
version = "1.0.0"
description = "Graph structure validation rules"

[dependencies]
graph-types = "0.2.0"
utils = { path = "../utils" }
core = { version = "1.0.0", path = "../core" }
```

The `[package]` section declares package metadata. The `[dependencies]` section lists dependencies as either a version string or an inline table with `version` and/or `path` keys.

### 15.2 Package Imports

Package imports use identifier syntax (not string paths):

```intent
import graph_types;                    // import whole package
import graph_types.validation;         // import specific submodule
```

Package names use underscores (matching Intent's identifier rules). The compiler resolves package names via the `intent.toml` dependencies.

**Local imports** (unchanged):

```intent
import "handlers.intent";             // relative file path
```

The parser distinguishes package imports (identifier tokens) from module imports (string literals).

### 15.3 Version Constraints

Semver-compatible resolution with the following constraint operators:

| Constraint | Meaning |
|------------|---------|
| `"1.0.0"` | Exact version |
| `"^1.0.0"` | Compatible updates (>=1.0.0, <2.0.0) |
| `"^0.1.0"` | Compatible updates (>=0.1.0, <0.2.0) |
| `"^0.0.1"` | Exact version (>=0.0.1, <0.0.2) |
| `"~1.0.0"` | Patch updates only (>=1.0.0, <1.1.0) |
| `">=1.0.0"` | Minimum version |
| `"<2.0.0"` | Maximum version (exclusive) |

The caret operator (`^`) follows standard semver conventions for pre-1.0 versions: `^0.y.z` treats the minor version as the compatibility boundary, and `^0.0.z` pins to the exact patch version. This reflects the expectation that 0.x releases may introduce breaking changes in any minor bump.

Only one version of each package is allowed in the dependency tree. The compiler validates that no conflicting constraints exist.

### 15.4 Local Path Dependencies

For development, dependencies can reference local directories:

```toml
[dependencies]
my-lib = { path = "../my-lib" }
```

Local path dependencies are resolved directly from the filesystem and are never cached.

### 15.5 Package Cache

Resolved packages are cached at `~/.intent/cache/<name>/<version>/`. Cache invalidation uses SHA256 checksums over source file contents. When source files change, the cached entry is invalidated and recompiled.

### 15.6 CLI Commands

```bash
intentc pkg init                        # create intent.toml in current directory
intentc pkg add <name> <version>        # add dependency with version constraint
intentc pkg add <name> --path <dir>     # add local path dependency
intentc pkg remove <name>               # remove dependency from intent.toml
intentc pkg install                     # resolve and fetch all dependencies
intentc pkg list                        # show dependency tree
intentc build                           # unchanged -- auto-resolves packages
```

### 15.7 Resolution Flow

1. Read `intent.toml` from the project root
2. For each dependency, check `~/.intent/cache/`
3. If not cached (or checksum mismatch), resolve from local path
4. Validate version constraints (detect conflicts)
5. Build dependency order via topological sort
6. Compile dependencies first, then the project

### 15.8 Package Layout

Packages use a **flat layout**: `.intent` source files are placed directly in the package root alongside `intent.toml`. For example:

```
my_project/
  types_pkg/
    intent.toml
    types.intent
  app_pkg/
    intent.toml
    main.intent
```

There is no `src/` subdirectory convention. This keeps the structure simple and avoids unnecessary nesting, especially for small packages.

### 15.9 Known Limitations

**Cross-package code generation** works for the common cases on the Rust and JavaScript targets: entities (with constructors, contracts, invariants, and methods), traits (`impl` in a dependency, method called from the consumer), free functions called via a qualified name (`pkg.fn(...)`), enums (declaration, `match`, and unit/data-variant construction), generic entities and functions instantiated with explicit type arguments, and nested cross-package type references. See [ADR 0061](decisions/0061-cross-package-codegen.md) for the full support matrix and the fixes behind it.

Two limitations remain (each produces a loud parse/compile error, not silent-wrong output, and has a workaround):

- **Module-qualified type-argument / variant syntax** — `pkg.Generic<T>(...)` and `pkg.Enum.Variant(...)` do not parse. Construct with the bare form (`Generic<T>(...)`) or via a factory function exported from the dependency.
- **Unqualified calls to imported functions** — `helper(...)` for an imported `helper` type-checks but is emitted unmangled. Qualify the call as `pkg.helper(...)`.

WebAssembly builds additionally reject `test` declarations ([ADR 0029](decisions/0029-in-language-testing.md)), independent of packaging.

---

## 16. Test Declarations and Annotations

### 16.1 Overview

Intent supports a small set of annotations that attach to specific declarations and influence semantic checking or runtime behavior. In v1, exactly one annotation is recognized: `@target_specific(...)`, which restricts a test declaration to a subset of compilation targets. See Section 18 for the full `test` declaration grammar; this section documents the annotation surface itself.

Only `@target_specific` is recognized in v1. Any other annotation name is a parse error -- the compiler does not silently accept unknown annotations. This conservative posture leaves room for future annotations without committing to an open extension point.

### 16.2 Syntax

```
TestDecl  := [Annotation] ["async"] "test" StringLiteral Block
Annotation := "@target_specific" "(" StringLiteral { "," StringLiteral } ")"
```

The annotation appears on its own line immediately before the `test` keyword:

```intent
@target_specific("rust")
test "uses Rust FFI" {
    let n: Int = native_double(21);
    assert_eq(n, 42);
}
```

Multiple targets may be listed:

```intent
@target_specific("rust", "js")
test "skipped on wasm" {
    // ...
}
```

### 16.3 Valid Arguments

The annotation accepts the literal strings `"rust"`, `"js"`, and `"wasm"`. Any other string is a type-check error.

- `"rust"` and `"js"` are honored at runtime: the test executes only when `intentc test` is targeting one of the listed backends.
- `"wasm"` is accepted by the parser and checker but produces a warning: the WebAssembly backend rejects test declarations during codegen, so `@target_specific("wasm")` cannot meaningfully execute. The warning encourages authors to remove the redundant argument.

### 16.4 Semantics

The runner consults the annotation when scheduling tests:

- **Single-target mode** (`intentc test --target rust` or no `--all-targets` flag): tests whose annotation excludes the current target are silently filtered out of the run.
- **Multi-target mode** (`intentc test --all-targets`): excluded tests are reported as `SKIP` rows with the reason `target_specific`, so divergence is visible without polluting the failure count.

Annotations are confined to `test` declarations. Attaching `@target_specific` to a function, entity, or other declaration form is a parse error.

### 16.5 References

- ADR 0031 -- target-specific test annotations (Phase 17).
- See Section 18 for the broader test-declaration grammar.

---

## 17. Extern Functions (Rust FFI)

### 17.1 Overview

Extern function declarations bridge Intent into the surrounding Rust crate ecosystem. An `extern function` has no Intent body; it names a Rust function (by fully qualified path) that the Rust backend will call at runtime. This lets Intent code depend on hand-written Rust modules or third-party crates without giving up Intent's contract surface.

Extern declarations are a **Rust-only** feature. The JavaScript and WebAssembly backends reject any program containing `extern function` at codegen time with a target-specific diagnostic.

### 17.2 Syntax

```
ExternDecl := "extern" "function" Identifier "(" [Params] ")" "returns" Type
              [ContractList]
              "from" StringLiteral ";"
```

The trailing semicolon is mandatory. There is no body.

```intent
extern function native_sqrt(x: Float) returns Float
    requires x >= 0.0
    ensures result >= 0.0
    from "crate::math::sqrt";

extern function native_lookup(key: String) returns Option<String>
    from "regex_cache::lookup";
```

Contracts (`requires` / `ensures`) compile to Intent-side runtime checks that wrap the foreign call -- the Rust function is invoked between the precondition and postcondition assertions, exactly as for a regular Intent function.

### 17.3 Bridged Type Set

Only a fixed set of types may appear in extern parameter or return positions. The Rust backend knows how to marshal these types across the FFI boundary:

| Type            | Notes                                       |
|-----------------|---------------------------------------------|
| `Int`           | maps to `i64`                               |
| `Float`         | maps to `f64`                               |
| `Bool`          | maps to `bool`                              |
| `String`        | maps to Rust `String`                       |
| `Void`          | return position only                        |
| `Array<T>`      | `T` must itself be bridgeable               |
| `Result<T, E>`  | both type arguments must be bridgeable      |
| `Option<T>`     | `T` must be bridgeable                      |

The following types are **rejected** in extern signatures:

- User-defined `entity` and `enum` types.
- `Map<K, V>` (the runtime representation is not stable across the boundary).
- `Future<T>` (async functions cannot be `extern`).
- `Fn(...) -> T` (closures and lambdas).
- Trait types.

Generic extern functions are not supported in v1.

### 17.4 Project Configuration

Extern callers typically need additional Rust crates pulled into the generated Cargo project. The `intent.toml` manifest accepts a `[rust_dependencies]` table that is forwarded verbatim into the generated `Cargo.toml`:

```toml
[package]
name = "my_app"
version = "0.1.0"

[rust_dependencies]
regex = "1.10"
serde_json = { version = "1", features = ["preserve_order"] }
```

The Rust backend writes these entries into the `[dependencies]` table of the synthesized `Cargo.toml` before invoking `cargo build`. Non-Rust backends ignore the table.

### 17.5 Target Restrictions

- **Rust** -- extern declarations compile to direct calls against the named path.
- **JavaScript** -- codegen rejects any program containing an extern declaration. The diagnostic names the offending function.
- **WebAssembly** -- codegen rejects extern declarations with the same message.

If you need cross-target portability, gate extern callers behind a wrapper that the Rust target uses but other targets stub out at the source level.

### 17.6 References

- ADR 0028 -- Rust FFI via `extern function` (Phase 15).

---

## 18. In-Language Testing

### 18.1 Overview

Intent files may declare top-level `test` blocks that are collected and run by `intentc test`. Tests are first-class declarations: they live alongside functions and entities, are type-checked under the same rules, and may exercise contracts, traits, async code, and FFI.

### 18.2 Syntax

```
TestDecl     := [Annotation] ["async"] "test" StringLiteral Block
```

Two example shapes:

```intent
test "addition is commutative" {
    assert_eq(2 + 3, 3 + 2);
}

async test "fetches a value" {
    let body: String = await fetch_body("https://example.test/data");
    assert(body.len() > 0);
}
```

A test declaration takes **no parameters** and has **no return type**. Both forms are parse errors. The body is a regular block; statements inside it follow the usual rules with one exception: a `return` statement inside a test body is a checker error, because a test has no meaningful return value.

The test name is the string literal that follows `test`. Names are used by the runner for filtering, reporting, and skip messages; duplicate names within a single file produce a warning.

### 18.3 Assert Builtins

Four builtins are available inside test bodies. They are ordinary call expressions and may also appear outside tests, but they are intended for assertions:

| Builtin                         | Behavior                                                        |
|---------------------------------|-----------------------------------------------------------------|
| `assert(condition)`             | fails the test if `condition` is `false`                        |
| `assert_eq(a, b)`               | fails if `a != b`; both arguments must have the same type       |
| `assert_close(a, b, epsilon)`   | float comparison; fails if `abs(a - b) > epsilon`               |
| `assert_panics(closure)`        | fails if the supplied `Fn() -> Void` closure does not panic     |

On failure, each builtin emits a structured diagnostic that the runner formats into the test report.

### 18.4 Discovery and Visibility

Tests are top-level declarations, but they are **not exported across package boundaries**. A test in package `A` is invisible to package `B` even if `B` imports `A`. Tests cannot be called from non-test code; they exist solely for the runner.

`intentc test` performs **cross-package discovery within a project**: starting from the entry package, it walks the import graph and collects all test declarations from every package transitively reachable, then runs them as a single suite. This means a downstream package's `intentc test` covers all tests from its dependencies, surfacing breakage from updated libraries immediately.

### 18.5 Runner Flags

- `--target <name>` runs tests for a single backend (defaults to `rust`).
- `--all-targets` runs every supported backend and reports cross-target divergence.
- `--filter <pattern>` selects tests whose name matches the substring.
- `--list` prints discovered tests without executing them.
- `--quiet` suppresses passing output.

In `--all-targets` mode, the runner cross-references results: a test that passes on Rust but fails on JS is reported as a **divergence**, not a failure, with both transcripts attached.

### 18.6 Target Annotations

Tests may be scoped to a subset of targets with `@target_specific(...)`. See Section 16. In summary:

- Single-target runs silently skip excluded tests.
- Multi-target runs report excluded tests as `SKIP` rows with reason `target_specific`.

### 18.7 References

- ADR 0029 -- in-language `test` declarations and assert builtins (Phase 16).
- ADR 0030 -- cross-package test discovery (Phase 17).
- ADR 0031 -- `@target_specific` annotation (Phase 17).

---

## 19. Editor Support

Intent ships an editor toolchain alongside the compiler. The integration point is a Language Server Protocol implementation embedded in `intentc`; editor plugins are thin clients around it.

### 19.1 Language Server

```
intentc lsp
```

Launches an LSP 3.17 server on stdio. The server reuses the compiler's lexer, parser, checker, linter, and Z3 verification path, so editor feedback matches command-line `intentc check`, `intentc lint`, and `intentc verify` exactly.

The v1 surface is:

- **Diagnostics** -- parser, checker, linter, and Z3 verification results streamed as `textDocument/publishDiagnostics`.
- **Hover** -- full contract bodies on top-level declarations; types on locals, parameters, `self`, fields, and methods.
- **Go to definition** -- same-file and same-package navigation for identifiers, fields, methods, and imports.
- **Document outline** -- `textDocument/documentSymbol` for declarations and nested members.
- **Formatting** -- `textDocument/formatting` driven by the same code path as `intentc fmt`.
- **Signature help** -- `textDocument/signatureHelp` with active-parameter tracking during call expressions.
- **Completion** -- identifier completion across the current scope and imports.
- **Semantic tokens** -- `textDocument/semanticTokens/full` with token types `function`, `method`, `parameter`, `variable`, `property`, `class`, `enum`, `enumMember`, `interface`, `decorator`, and modifiers `declaration`, `async`, `defaultLibrary`.

### 19.2 VS Code Extension

A reference client lives at `editors/vscode/`. It bundles via esbuild, registers `.intent` files, launches `intentc lsp`, and exposes a status bar entry plus a `Restart Server` command. The extension is the canonical example for porting Intent support to other LSP-capable editors.

### 19.3 References

- ADR 0032 -- LSP server architecture (Phases 18-20).

---

## Appendix A: Future Directions

The following features are candidates for future versions:

- **Generic type bounds**: Constraints on type parameters (e.g., `T: Comparable`).
- **Generic type inference**: Inferring type arguments from context (e.g., `identity(42)` instead of `identity<Int>(42)`).
- **Proof integration**: Connecting `verified_by` to formal proof tools.
- **Optimization levels**: Removing contract checks in release builds.

### Previously Future, Now Implemented

- ~~Quantifier expressions~~: `forall` and `exists` -- implemented.
- ~~Traits/Interfaces~~: Trait system with static dispatch -- implemented (Phase 6).
- ~~Standard library~~: I/O, HTTP, JSON builtins -- implemented (Phases 7-8).
- ~~String interpolation~~: Implemented (Phase 9).
- ~~Imports~~: Multi-file compilation with `import` -- implemented (Phase 8).
- ~~Property-based testing~~: Test generation from contracts via `intentc test-gen` -- implemented.
- ~~Closures and first-class functions~~: `Fn(T) -> R` types, lambda expressions -- implemented (Phase 10).
- ~~Full pattern matching~~: Exhaustive `match` on all enum types -- implemented (Phases 5-6).
- ~~Concurrency/async~~: `async function`, `await`, `spawn`, `Future<T>` -- implemented (Phase 12).
- ~~Package management~~: `intent.toml` manifests, semver constraints, package imports, package cache -- implemented (Phase 13).
