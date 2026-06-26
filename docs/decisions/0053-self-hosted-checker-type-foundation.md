# 0053: Self-Hosted Checker — Type Representation Foundation

**Date:** 2026-06-26
**Status:** accepted
**Phase:** 46 — checker type-system foundation (first slice; expression inference follows)

## Context

Phase 45 ([ADR 0052](0052-self-hosted-checker-strategy.md)) shipped the self-hosted
checker's first slice — structural + name-resolution + arity checks, all needing **no
type inference**. The remaining ~140 checker diagnostics are type-inference-heavy and
blocked on a structured type representation the stage2 AST does not have: types are flat
`String`s (`Param.type_name`, `FunctionDecl.return_type`, `FieldDecl.type_name`, …) and
`Expr` has no type field. This ADR records how the self-hosted checker will represent
and resolve types, and scopes Phase 46 (the foundation) before expression inference.

## Decision

### D1 — Represent types as an in-checker `Type` tree built from the existing strings
The checker gains a `Type` entity (`name: String`, `type_args: Array<Type>`, plus the
`Fn(...) -> R` shape) and a `parse_type(s: String) -> Type` that parses the type
**strings the AST already carries** (e.g. `"Array<Map<String, Int>>"`, `"Fn(Int) ->
Int"`) into a tree.

Chosen over enriching the parser/AST to carry structured `TypeRef` nodes (the
alternative): that would change `ast.intent`'s type fields from `String` to a `TypeRef`
entity and force the **formatter** to reconstruct the exact type string from the tree,
risking the byte-equal self-format + `diff-formatter` gates. The formatter does not need
structured types — only the checker does. So the checker parses the strings it already
receives, leaving the shared front-end (and its gates) untouched. The stage2 parser
already tokenises and reconstructs these exact type-string forms (`parse_type_name`), so
re-parsing them into a tree is well-bounded.

### D2 — First slice: resolver + `unknown type` check (no expression inference)
Phase 46 ships:
- the `Type` entity + `parse_type`,
- a **resolver** that recognises the valid type forms — primitives (`Int`, `Float`,
  `String`, `Bool`, `Void`, `Char`), the built-in generics (`Array<T>`, `Map<K,V>`,
  `Result<T,E>`, `Option<T>`, `Future<T>`), `Fn(...) -> R`, user entity/enum names, and
  in-scope generic type parameters — recursing into type arguments,
- the **`unknown type 'X'`** check over every type annotation (param / field / return /
  `let`), threading the enclosing declaration's generic type params so `T` resolves.

Expression type inference, assignability, operator typing, generics substitution, match
exhaustiveness, contracts, etc. are **deferred** to Phase 47+. They need the scope to
carry types (not just names) and inference over every `Expr` kind — a larger build on
this foundation.

### D3 — Gate: two-directional `make diff-checker` (unchanged shape)
- **No false positives:** the 22 valid examples use only resolvable types
  (Array/Map/Result/Option/Future/Fn/entities/enums/type-params, nested), so the
  resolver must resolve them ALL — `intentc check --self-hosted` must stay
  `No errors found.` on every example. This is the forcing function for resolver
  correctness.
- **Invalid fixtures:** programs referencing an unknown type, byte-equal vs stage1's
  `unknown type '<base>'` (the message uses the unresolved **base** name, anchored at
  the declaration). Crafted to trigger only the unknown-type check.

### D4 — Faithful, gate-protected
A faithful port of stage1's `ResolveType` semantics (`internal/checker/types.go`) and
the `unknown type` emit sites (`checker.go`). The formatter/linter gates and the Phase
45 checks stay green throughout.

## Consequences

### Benefits
- The `Type` tree + `parse_type` + resolver are the foundation every later type-rule
  check and all expression inference reuse — the single biggest unlock toward a fully
  self-hosted checker.
- Zero front-end churn (no parser/formatter/AST change) — the gates that protect the
  formatter and linter are untouched.

### Costs
- The resolver must be comprehensive (every type form the corpus uses) or it
  false-positives — but the no-false-positives gate catches that directly.
- `parse_type` re-parses type strings the parser already parsed once (minor duplication;
  acceptable to avoid front-end churn).
- Double maintenance continues (stage1 Go checker vs stage2) on the long road to parity.

### Non-goals (this phase)
- Expression type inference and all type-rule checks (Phase 47+).
- Changing the AST to carry structured types (deliberately avoided — see D1).

## References
- [ADR 0052](0052-self-hosted-checker-strategy.md) — checker strategy / first slice.
- `internal/checker/types.go` — `ResolveType` / `Type` semantics being ported.
- `internal/checker/checker.go` — `unknown type '%s'` emit sites.
- `selfhost/shared/parser.intent` — `parse_type_name` (the type-string forms re-parsed).
- PRD: `prds/active/prd-phase-46-checker-type-foundation.md`.
