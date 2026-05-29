# 0018: Trait System (Static Dispatch)

**Date:** 2026-02-24
**Status:** accepted
**Phase:** post-v1.0 (Attractor Phase 6)

## Context

The Attractor spec (Section 4) defines 8 handler types that all share an `execute(node, context) -> Result<Outcome, String>` interface. Without traits, each handler is a standalone entity with no compile-time guarantee that it implements the expected interface. The spec's handler dispatch pattern requires a shared contract across all handler implementations.

## Decision

Add a narrowly-scoped trait system with static dispatch only.

### Syntax

```intent
trait Handler {
    method execute(ctx: Context) returns Int
        requires ctx.get_value() >= 0
        ensures result >= 0;
}

impl Handler for StartHandler {
    method execute(ctx: Context) returns Int {
        return self.code + ctx.get_value();
    }
}
```

### Design Choices

1. **Static dispatch only** -- No trait objects (`dyn Trait`), no vtables. The caller always knows the concrete type. This keeps codegen simple and avoids runtime overhead.

2. **No default implementations** -- All trait methods are signatures only. Every impl block must provide all methods.

3. **No generic traits** -- Trait declarations cannot have type parameters.

4. **Trait contracts flow to implementations** -- `requires`/`ensures` clauses on trait method signatures apply to all implementations. Impl methods may add additional contracts.

5. **`for` keyword reuse** -- The `impl Trait for Entity` syntax reuses the existing `for` keyword (already used in `for-in` loops). The parser distinguishes by context.

6. **Impl methods merge into entity** -- For method resolution, trait impl methods are added to the entity's method table. This means `entity.method()` works regardless of whether the method was defined directly on the entity or via an impl block.

7. **implOrigins tracking** -- The checker records which methods come from trait impls (`"Entity.Method" -> "Trait"`) so the Rust backend can emit proper `impl Trait for Entity { }` blocks separate from `impl Entity { }`.

### Backend Strategies

- **Rust**: Direct mapping to Rust traits and impl blocks
- **JavaScript**: Traits as JSDoc `@interface` comments; impl methods as `Entity.prototype.method` assignments
- **WASM**: Trait declarations skipped; impl methods emitted as mangled standalone functions (`EntityName_MethodName`)

## Consequences

- All Attractor handlers can share a verified interface contract
- Method resolution unchanged for callers -- entity.method() works for both direct and trait methods
- No dynamic dispatch means handler registries (dispatching by string key) remain a future concern
- Future phases may add trait objects if dynamic dispatch is needed
