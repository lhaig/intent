# ADR 0023: Closures and First-Class Functions

## Status

Accepted

## Context

Intent listed closures and first-class functions as non-goals for the POC. However, higher-order function patterns (map, filter, apply, callbacks) are common in the Attractor pipeline and general programming. Adding closures aligns with Intent's goal of being a capable language for AI-generated code.

## Decision

### 1. Function Type Syntax: `Fn(T1, T2) -> R`

A new `Fn` keyword introduces function types in type annotations:

```intent
function apply(x: Int, f: Fn(Int) -> Int) returns Int {
    return f(x);
}
```

`Fn` is parsed as a type keyword (like `Int`, `Array`). The parser reads parameter types in parentheses followed by `->` and a return type.

### 2. Lambda Expression Syntax: `|params| -> R => expr`

Lambda expressions use pipe-delimited parameters with explicit types:

```intent
let double: Fn(Int) -> Int = |x: Int| -> Int => x * 2;
```

- Parameters are typed (consistent with Intent's no-inference policy)
- Return type is optional (inferred from body expression)
- Body is a single expression (no multi-statement lambdas)
- Captures variables from enclosing scope

### 3. Type System Integration

The `Type` struct gained three fields: `IsFunction bool`, `FnParams []*Type`, `FnReturn *Type`. Function types participate in equality checking and can be used in let bindings, function parameters, and return types.

### 4. Calling Function-Typed Variables

When the checker encounters a call expression (`f(x)`), it first checks if `f` is a variable with a function type before looking up the function table. This allows closures stored in variables to be called with the same syntax as regular functions.

### 5. Rust Codegen

- Lambda expressions emit Rust closures: `|x: i64| -> i64 { expr }`
- Function parameters with Fn types emit `impl Fn(i64) -> i64`
- Let bindings with Fn types omit the type annotation (Rust infers closure types)
- Closures are treated as Copy-like for the clone analysis (not cloned when passed)

### 6. JavaScript Codegen

- Lambda expressions emit arrow functions: `(x) => { return expr; }`
- Fn types map to `Function`

## Consequences

- Higher-order functions are now supported across the full pipeline
- Lambda expressions can capture variables from enclosing scope
- No multi-statement lambda bodies (deliberate simplicity)
- No returning closures from functions that capture mutable state (Rust lifetime limitations)
- All existing tests continue to pass
- New `closure_demo.intent` example demonstrates the feature
