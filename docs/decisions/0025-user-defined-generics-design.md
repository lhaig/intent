# ADR 0025: User-Defined Generics -- Design Plan

## Status

Accepted

## Context

Intent currently supports built-in generic types (`Array<T>`, `Map<K,V>`, `Result<T,E>`, `Option<T>`) but not user-defined generics. Adding generics enables reusable data structures (`Stack<T>`, `Queue<T>`) and generic functions (`apply<T>(f: Fn(T) -> T, x: T) -> T`).

## Design

### Syntax

**Generic entities:**
```intent
entity Stack<T> {
    field items: Array<T>;
    field count: Int;

    invariant self.count >= 0;
    invariant self.count == len(self.items);

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

    method peek() returns Option<T>
        ensures (self.count > 0) implies result.is_some()
    {
        if self.count == 0 { return None; }
        return Some(self.items[self.count - 1]);
    }
}
```

**Generic functions:**
```intent
function identity<T>(x: T) returns T
    ensures result == x
{
    return x;
}
```

**Instantiation:**
```intent
let s: Stack<Int> = Stack<Int>();
s.push(42);
let val: Option<Int> = s.peek();
```

### Type System Changes

1. **TypeRef** gains `TypeParams []*TypeParam` on `EntityDecl` and `FunctionDecl` AST nodes.

2. **Type parameter scope:** During checking, type params (e.g., `T`) are added to a scope that field types, method signatures, and contracts can reference. `T` resolves to a placeholder type (`Type{Name: "T", IsTypeParam: true}`).

3. **Monomorphization strategy:** At IR lowering time, collect all concrete instantiations (e.g., `Stack<Int>`, `Stack<String>`) and emit a separate IR entity for each. This avoids generics in JS/WASM backends which don't support them.

4. **Type equality:** `Stack<Int>` != `Stack<String>`. Type params participate in `Equal()`.

### Implementation Phases

**Phase 1: Parser + AST (est. scope: lexer, parser, ast)**
- Add `TypeParam` AST node
- Parse `entity Name<T, U> { ... }` and `function name<T>(...)` syntax
- Parse `TypeRef` with type params in instantiation position (`Stack<Int>`)
- No semantic changes yet

**Phase 2: Checker (est. scope: checker/types.go, checker/checker.go)**
- Add `IsTypeParam bool` to `Type`
- Track type parameter scope during entity/function checking
- Validate type params are used in fields/signatures
- Validate concrete type arguments at instantiation sites
- Build substitution map (`T -> Int`) for type checking method bodies

**Phase 3: IR Monomorphization (est. scope: ir/lower.go)**
- Collect all unique instantiations during lowering
- For each `Stack<Int>`, emit `Entity{Name: "Stack__Int", Fields: [...]}`
- Substitute type params in method bodies, contracts, field types
- Handle nested generics (`Stack<Array<Int>>`)

**Phase 4: Backend updates (est. scope: rustbe, jsbe, wasmbe)**
- Rust: option A (monomorphized structs) or option B (`impl<T>` Rust generics)
- JS: monomorphized classes (no native generics)
- WASM: monomorphized types
- Update `mapType()`, `mangledEntityName()` for generic instantiations

**Phase 5: Formatter + Linter**
- Formatter emits `<T>` in declarations and instantiations
- Linter: warn on unused type parameters

### Constraints

- No type bounds (e.g., `T: Comparable`) in initial implementation
- No generic enums or traits initially (can extend later)
- No type inference for generic function calls -- explicit type args required: `identity<Int>(42)`, not `identity(42)`
- Monomorphization may increase code size for many instantiations

### Dependencies

None -- this is a standalone feature.

## Consequences

- Enables reusable generic data structures and algorithms
- Monomorphization keeps JS/WASM backends simple
- Foundation for generic traits and type bounds in future

## Implementation Notes (Phase 14)

Phase 11 landed the parser, checker, IR monomorphization, and backend output, but the IR's monomorphize helpers called `resolveTypeRef` (no `typeParams` argument), so references to a type parameter `T` inside a generic body failed to resolve. `Array<T>` collapsed to nil, the Rust backend emitted unit `()` for the field type, and generated code did not compile. The JS backend hid the bug because it does not type-check field declarations. Phase 14 added `resolveTypeRefWithParams` plus a `typeParamSet` helper in `internal/ir/lower.go` and threaded the type-param set through `monomorphizeEntity`, `monomorphizeConstructor`, `monomorphizeMethod`, and `monomorphizeFunction`. Test: `TestMonomorphizeUsesTypeParamsInBody` in `internal/ir/lower_test.go`.
