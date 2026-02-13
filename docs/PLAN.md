# Intent Language -- Build Plan

## Thesis

AI coding assistants write code in languages designed for humans. Those languages optimize for human cognitive constraints -- concise syntax, implicit behavior, syntactic sugar. AI has no such constraints. It can produce and consume arbitrarily verbose, maximally explicit code without fatigue.

**Intent tests whether a language designed for AI authorship produces better software.** If every function carries contracts, every entity carries invariants, and every module carries declared intent linked to formal verification, then AI-generated code should be more correct, more auditable, and more verifiable than the same AI writing uncontracted code in human languages.

The contract system is the product. Everything else is infrastructure.

See [ADR 0000](decisions/0000-why-intent-exists.md) for the full rationale.

## Feature Filter

Every feature must serve one of two goals:

1. **Make real programs possible** (loops, collections, IO, error handling)
2. **Deepen the verification story** (richer contracts, static analysis, proofs)

Features that do neither are out of scope.

## Architecture Pattern

Every language feature follows the same 6-step pipeline. Each phase below touches the same files in the same order:

```
1. Lexer    internal/lexer/token.go + lexer.go     Add tokens/keywords
2. Parser   internal/parser/parser.go              Add parse rules, produce AST
3. AST      internal/ast/nodes.go                  Add node types
4. Checker  internal/checker/checker.go             Add type rules + validation
5. Codegen  internal/codegen/codegen.go             Emit Rust
6. Tests    *_test.go in each package               Cover happy + error paths
```

After each phase: `go test ./...` must pass, `./intentc build` on updated examples must produce working binaries.

## Codebase Stats

| File | Lines | Role |
|------|-------|------|
| lexer/token.go | 277 | Token types + keyword map |
| lexer/lexer.go | 289 | Tokenizer |
| parser/parser.go | 734 | Recursive-descent parser |
| ast/nodes.go | 372 | AST node definitions |
| checker/checker.go | 826 | Type checking + semantic analysis |
| checker/types.go | ~100 | Type system definitions |
| checker/scope.go | ~60 | Scope/symbol table |
| checker/intent.go | ~100 | Intent block verification |
| codegen/codegen.go | 721 | Rust code generator |
| compiler/compiler.go | 139 | Pipeline orchestration |
| linter/linter.go | 353 | Style/best-practice warnings |
| cmd/intentc/main.go | ~165 | CLI entry point |

---

## Milestone 1: Usable Language

**Goal:** Write real programs that compute, iterate, and produce output.

Without loops and print, Intent can't express useful algorithms. Without arrays, it can't work with data. These three features are the minimum to make the language worth using beyond toy examples.

### Phase 1.1: While Loops

**Why first:** Unblocks all iterative algorithms. The fibonacci example currently uses 7 nested `if` statements -- this is the most painful gap.

**Language design:**
```
while condition {
    body
}
```

No `break` or `continue` in this phase (keep it simple).

**Contract extension -- loop invariants (defer to Milestone 3).** For now, while loops are just control flow.

**Changes:**
- token.go: Add `WHILE` keyword
- lexer.go: Map `"while"` to WHILE
- nodes.go: Add `WhileStmt` struct (Condition Expression, Body *Block)
- parser.go: Parse while statement in `parseStatement()`
- checker.go: Check condition is Bool, check body
- codegen.go: Emit `while condition { body }` (direct Rust mapping)
- Update grammar.ebnf: Add `while_statement` production

**Tests:** Parse while loop, type-check condition, generate Rust, reject non-bool condition.

**Example update:** Rewrite `examples/fibonacci.intent` to use while loop instead of nested ifs.

**Verification:** `go test ./...` passes. `./intentc build examples/fibonacci.intent && ./fibonacci` exits with fib(7) = 13.

### Phase 1.2: Print Function

**Why:** Programs need observable output. Currently impossible to see what a program computes without checking exit codes.

**Language design:** Built-in function, not a keyword. Overloaded for all primitive types:
```
print("Hello, world!");
print(42);
print(3.14);
print(true);
```

**Changes:**
- checker.go: Register `print` as a built-in function that accepts any single primitive type and returns Void
- codegen.go: Map `print(expr)` to `println!("{}", expr)` for primitives. For String type: `println!("{}", expr)`.
- No lexer/parser changes needed -- `print` is already a valid identifier and call expression

**Tests:** Check print accepts all primitive types, rejects entity types, generates correct println!.

**Example update:** Add print calls to examples so running them produces visible output.

**Verification:** `./intentc build examples/hello.intent && ./hello` prints "Hello, world!" to stdout.

### Phase 1.3: Arrays

**Why:** Required for any program that works with collections of data.

**Language design:**
```
let numbers: Array<Int> = [1, 2, 3];
let first: Int = numbers[0];
let size: Int = len(numbers);
numbers.push(4);
```

Type syntax: `Array<T>` where T is any existing type (primitives or entities).

**Changes:**
- token.go: No new tokens needed (`[`, `]`, `<`, `>` already exist for other purposes -- but `<` and `>` are comparison operators. Use angle brackets in type position only.)
- nodes.go: Add `ArrayType` (ElementType *TypeRef), `ArrayLitExpr` (Elements []Expression), `IndexExpr` (Object Expression, Index Expression)
- parser.go: Parse `Array<Type>` in type position, `[expr, ...]` as array literal, `expr[expr]` as index access
- checker/types.go: Add array type representation with element type tracking
- checker.go: Type-check array literals (all elements same type), index expressions (index must be Int), len() built-in
- codegen.go: Map `Array<T>` to `Vec<T>`, `[a, b, c]` to `vec![a, b, c]`, `arr[i]` to `arr[i as usize]`, `len(arr)` to `arr.len() as i64`, `arr.push(x)` to `arr.push(x)`

**Contract extensions:**
- `len()` usable in contracts: `requires len(items) > 0`
- Index bounds expressible: `requires i >= 0 and i < len(arr)`

**Tests:** Array literal type checking, index type checking, len() type, push() on arrays, reject mixed-type literals.

**Example:** New `examples/sort.intent` -- simple bubble sort with contracts:
```
function sort(arr: Array<Int>) returns Array<Int>
    ensures len(result) == len(arr)
{ ... }
```

**Verification:** `go test ./...` passes. Sort example compiles and produces sorted output.

### Phase 1.4: For Loops

**Why:** Iterating over arrays with index tracking is verbose with while. For loops are the natural next step after arrays.

**Language design:**
```
for item in array {
    print(item);
}

for i in 0..n {
    print(i);
}
```

Two forms: for-in over arrays, for-in over integer ranges.

**Changes:**
- token.go: Add `FOR`, `IN`, `DOTDOT` tokens
- lexer.go: Map keywords, recognize `..` operator
- nodes.go: Add `ForStmt` (VarName string, Iterable Expression, Body *Block) and `RangeExpr` (Start, End Expression)
- parser.go: Parse for statement
- checker.go: Infer loop variable type from iterable (array element type or Int for ranges)
- codegen.go: `for item in array` -> `for item in arr.iter()` or `for item in &arr`, `for i in 0..n` -> `for i in 0..n`

**Tests:** For-in over array, for-in over range, type inference for loop variable.

**Verification:** Rewrite sort example to use for loop. All tests pass.

---

## Milestone 2: Robust Language

**Goal:** Error handling, sum types, multi-file programs.

### Phase 2.1: Enums

**Language design:**
```
enum Direction {
    North,
    South,
    East,
    West,
}

enum Shape {
    Circle(radius: Float),
    Rectangle(width: Float, height: Float),
}
```

**Match expression:**
```
match shape {
    Circle(r) => return 3.14 * r * r;
    Rectangle(w, h) => return w * h;
}
```

**Changes:** Full pipeline. Codegen maps to Rust enums + match. Checker enforces exhaustive matching.

### Phase 2.2: Result Type and Error Handling

**Why:** Programs need to express failure without panicking. Contracts should distinguish "this can fail" from "this must succeed."

**Language design:**
```
function divide(a: Int, b: Int) returns Result<Int, String>
    requires true
    ensures result.is_ok() implies b != 0
{
    if b == 0 {
        return Err("division by zero");
    }
    return Ok(a / b);
}
```

Built-in enum `Result<T, E>` with `Ok(T)` and `Err(E)` variants. Propagation with `?` operator.

**Depends on:** Phase 2.1 (enums).

### Phase 2.3: Multi-File Imports

**Language design:**
```
import math;
import collections.list;
```

Module resolution by file path relative to project root.

**Changes:** Compiler needs to resolve imports, parse multiple files, merge symbol tables before checking. Codegen emits Rust modules or a single flattened file.

---

## Milestone 3: Verification Depth

**Goal:** Make contracts more than runtime assertions.

### Phase 3.1: Loop Invariants

**Language design:**
```
while i < len(arr)
    invariant i >= 0
    invariant is_sorted(arr, 0, i)
{
    // ...
}
```

Codegen: assert invariant at loop entry and after each iteration.

**Depends on:** Phase 1.1 (while loops).

### Phase 3.2: Quantifiers in Contracts

**Language design:**
```
ensures forall i in 0..len(result) - 1: result[i] <= result[i + 1]
ensures exists i in 0..len(arr): arr[i] == target
```

Codegen: expand to runtime loops that check each element. This is expensive but correct.

**Depends on:** Phase 1.3 (arrays), Phase 1.4 (for loops).

### Phase 3.3: Property-Based Test Generation

Generate Rust `#[test]` functions from contracts. For each function with `requires`, generate random inputs satisfying preconditions and verify postconditions hold.

**Depends on:** Milestone 2 complete.

### Phase 3.4: Static Verification (Z3)

Connect contracts to Z3 SMT solver. Prove simple contracts at compile time. Report "verified" vs "unverified" status for each contract.

**Depends on:** Phase 3.2 (quantifiers make this meaningful).

---

## Milestone 4: Tooling

### Phase 4.1: Formatter (`intentc fmt`)

Canonical formatting. Parse -> AST -> pretty-print. Idempotent.

### Phase 4.2: Better Error Messages

Source snippets with underline markers. Levenshtein-based "did you mean?" suggestions.

### Phase 4.3: LSP Server

Diagnostics, go-to-definition, hover, completions. VS Code extension.

---

## Execution Notes

**Phase sizing:** Each phase is scoped to complete in a single session (~30 min of agent work). If a phase feels too large, split it.

**Token efficiency for automated execution:**
- Each phase description above contains the exact files to modify, the exact changes to make, and the exact verification steps. An agent should not need to explore.
- The architecture pattern (lexer -> parser -> AST -> checker -> codegen -> tests) is identical for every language feature. Once one phase is done, subsequent phases follow the template.
- Tests use the established pattern: parse source string, run pipeline stage, assert on output. See `internal/codegen/codegen_test.go` and `internal/checker/checker_test.go` for the pattern.
- Avoid re-reading unchanged files. The phase descriptions list which files are touched.

**Phase dependency graph:**
```
1.1 While ─────────────────────────────────┐
1.2 Print (independent)                     │
1.3 Arrays ──┬── 1.4 For ──┐               │
             │              │               │
             └──────────────┴── 2.1 Enums ──┤
                                    │       │
                              2.2 Result    │
                                    │       │
                              2.3 Imports   │
                                            │
3.1 Loop Invariants ◄───────────────────────┘
3.2 Quantifiers ◄── 1.3 + 1.4
3.3 Test Gen ◄── M2 complete
3.4 Z3 ◄── 3.2
```

Phases 1.1 and 1.2 have no dependencies and can be built in parallel.
Phase 1.3 is independent of 1.1 and 1.2.
Phase 1.4 depends only on 1.3.

**Priority order for serial execution:** 1.1 -> 1.2 -> 1.3 -> 1.4 -> 2.1 -> 2.2 -> 2.3 -> 3.1 -> 3.2 -> 4.1 -> 3.3 -> 4.2 -> 3.4 -> 4.3
