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
- Keep the proof-of-concept scope minimal: no generics, no collections, no enums, no traits.

### 1.2 Non-Goals (for POC)

- Standard library.
- Package management.
- Generic types or type parameters.
- Arrays, slices, maps, or other collection types.
- Enum or union types.
- Concurrency or async constructs.
- Pattern matching.
- Closures or first-class functions.
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

Intent uses a simple, nominal type system with no subtyping, no generics, and no type inference beyond literal types.

### 4.1 Primitive Types

| Intent Type | Rust Mapping | Description |
|-------------|-------------|-------------|
| `Int`       | `i64`       | 64-bit signed integer |
| `Float`     | `f64`       | 64-bit IEEE 754 floating-point |
| `String`    | `String`    | Owned UTF-8 string |
| `Bool`      | `bool`      | Boolean (`true` or `false`) |
| `Void`      | `()`        | Unit type, no value |

### 4.2 Entity Types

Entity types are user-defined nominal types, declared with the `entity` keyword (see Section 7). Each entity declaration introduces a new type whose name can be used as a type annotation. Entity types map to Rust structs.

### 4.3 Type Rules

- Arithmetic operators (`+`, `-`, `*`, `/`, `%`) require both operands to have the same numeric type (`Int` or `Float`). The result has that same type. Mixed `Int`/`Float` arithmetic is a compile-time error.
- The `+` operator is also defined for `String` operands (concatenation). The result is `String`.
- Comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`) require both operands to have the same type. Equality (`==`, `!=`) is defined for all types. Ordering (`<`, `>`, `<=`, `>=`) is defined for `Int` and `Float` only. The result is always `Bool`.
- Logical operators (`and`, `or`, `not`, `implies`) require `Bool` operands and produce `Bool`.
- Assignment requires the right-hand side type to match the variable's declared type exactly.
- Function call arguments must match parameter types exactly. The call expression's type is the function's return type.
- Method call arguments must match parameter types exactly. The call expression's type is the method's return type.
- Field access on an entity-typed expression produces the field's declared type.

### 4.4 No Implicit Conversions

There are no implicit type conversions. `Int` does not implicitly convert to `Float`, `Bool` does not implicitly convert to `Int`, and so on. All conversions, if needed, must be explicit (and in the POC, no conversion functions are provided -- this is a known limitation).

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
- Intent blocks

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

The Intent compiler (`intentc`) is a Go program that processes `.intent` source files through four phases:

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
[4. Code Generator]  -->  Rust source (.rs)
    |
    v
[cargo build]  -->  Native binary
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

## Appendix A: Future Directions

The following features are explicitly deferred from the POC but are candidates for future versions:

- **Arrays and collection types**: `Array<T>` with quantifier expressions in contracts (`forall`, `exists`).
- **Enums and pattern matching**: Sum types with exhaustive match.
- **Generics**: Parameterized types and functions.
- **Imports**: Cross-module references with dependency resolution.
- **Standard library**: Basic I/O, math, string manipulation.
- **Proof integration**: Connecting `verified_by` to formal proof tools.
- **Property-based testing**: Generating test cases from contracts.
- **Optimization levels**: Removing contract checks in release builds.
- **String interpolation**: `"Balance: {self.balance}"` syntax.
- **Traits/Interfaces**: Behavioral contracts across types.
