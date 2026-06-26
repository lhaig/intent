# PRD — Phase 46: Checker Type Representation Foundation

## 1. Introduction / Overview

Phase 45 self-hosted the checker's structural + name-resolution + arity checks (no type
inference). Phase 46 builds the **type-system foundation** every remaining checker
diagnostic needs: a structured `Type` tree, a `parse_type(string)` that builds it from
the type strings the AST already carries, a resolver, and the first type check —
`unknown type 'X'`. Expression type inference and the type-rule checks follow in later
phases. Strategic frame: [ADR 0053](../../docs/decisions/0053-self-hosted-checker-type-foundation.md).

This is additive to `selfhost/checker/` and touches no front-end (per ADR 0053 D1).

## 2. Goals

- A `Type` entity + `parse_type(s: String) -> Type` parsing primitives, `Array/Map/
  Result/Option/Future<...>`, `Fn(...) -> R`, and bare names into a tree.
- A `type_is_known(t, entities, enums, type_params)` resolver recognising primitives,
  the built-in generics, `Fn`, entity/enum names, and in-scope generic type params,
  recursing into type args.
- The `unknown type 'X'` check over param / field / return / `let` type annotations,
  threading the enclosing decl's type params; integrated into `check_program`.
- `make diff-checker` extended with unknown-type fixtures; **no false positives** on the
  22 valid examples (resolver resolves every type they use).
- All existing gates stay green.

## 3. Design decisions

Per [ADR 0053](../../docs/decisions/0053-self-hosted-checker-type-foundation.md): D1
in-checker `Type` built from existing strings (no front-end change), D2 first slice =
resolver + unknown-type (no expression inference), D3 two-directional diff-checker gate,
D4 faithful gate-protected port.

## 4. User Stories / Tasks

### US-001 (46.1): ADR 0053 — type-representation foundation
**AC:** `docs/decisions/0053-...md` records D1-D4. Done.

### US-002 (46.2): `Type` entity + `parse_type`
**AC:** In `selfhost/checker/check.intent`, a `Type` entity (`name: String`,
`type_args: Array<Type>`, and the means to represent `Fn(params) -> ret` — e.g. a kind
flag + `fn_params: Array<Type>` + a return Type, or model `Fn` as name="Fn" with
type_args = params then return; pick one and document). `parse_type(s: String) -> Type`
parses, mirroring `parse_type_name` forms: bare `Int`; `Array<Map<String, Int>>`
(nested `<>`); `Fn(Int, Int) -> Int`. ≥6 unit tests (primitive, single generic, nested
generic, two-arg generic `Map<K,V>`, `Fn(..)->..`, an entity name). No front-end change.

### US-003 (46.3): resolver `type_is_known`
**AC:** `type_is_known(t: Type, entity_names: Array<String>, enum_names: Array<String>,
type_params: Array<String>) -> Bool` — true for primitives (Int/Float/String/Bool/Void/
Char), `Array` (1 arg), `Map` (2), `Result` (2), `Option` (1), `Future` (1), `Fn`
(params + ret all known), entity/enum names, and names in `type_params`; recurses into
type args (all must be known). Port stage1 `ResolveType` (types.go) semantics. ≥6 tests
incl. unknown base, nested-unknown (`Array<Widget>`), type-param-in-scope.

### US-004 (46.4): `unknown type 'X'` check
**AC:** A register/check-phase pass over param / field / return / `let` type annotations:
`parse_type` the annotation, build the known entity/enum-name lists + the enclosing
decl's `type_params`, and if not `type_is_known` → emit `unknown type 'BASE'` at the
declaration's position (BASE = the unresolved base name, matching stage1's `ref.Name`).
Verify emit order + exact anchor + which base name against `internal/checker/checker.go`
(the `unknown type '%s'` sites). ≥3 tests; integrated into `check_program`.

### US-005 (46.5): diff-checker fixtures + no-false-positives
**AC:** Add unknown-type fixture(s) to `selfhost/checker/check-fixtures/` (e.g. a param
typed `Widget`), each minimized to trigger only the unknown-type check. `make
diff-checker` passes — all 22 valid examples still `No errors found.` (resolver resolves
every corpus type) AND the new fixtures byte-equal vs stage1. If a valid example
false-positives (an unresolved corpus type), fix the resolver (don't allow-list).

### US-006 (46.6): docs + final validate + push
**AC:** Update `selfhost/checker/README.md` (type foundation + unknown-type),
ROADMAP Phase 46, NEXT-STEPS (Phase 47 = expression inference). `make validate` + all
four diff/selfcheck gates green. Commit + push.

## 5. Non-Goals

- Expression type inference, assignability, operator/condition typing, generic
  substitution, match exhaustiveness, contracts, method/builtin arity — Phase 47+.
- Changing the AST/parser/formatter to carry structured types (ADR 0053 D1: avoided).
- Multi-file `CheckAll`.

## 6. Technical Considerations

- `parse_type` must handle the exact forms `parse_type_name` produces (read it):
  nested `<...>`, comma-separated args, and `Fn(p1, p2) -> R`. Whitespace in `Map<K, V>`
  / `Fn(.., ..) -> R` (the formatter emits `, ` and ` -> `) must be parsed/normalised
  consistently with how the strings are stored.
- The `unknown type` message uses the **base** name (`ref.Name`), not the full string.
- Type-param scope: a generic `function id<T>(x: T)` / `entity Box<T>` makes `T` a known
  type within that decl — thread `decl.type_params` into the resolver.
- No `Map`/no recursive-helper-mutation gotchas: `Type` has `type_args: Array<Type>`
  (heap via Array, like `Expr.children`) — fine. `parse_type` is a recursive-descent
  over the string (reuse char/index scanning like the lexer).
- Never run stage1 `intentc fmt` on stage2 files; keep `check.intent` canonical.

## 7. Success Metrics

- `make diff-checker` green: 22 valid examples no-false-positives + unknown-type fixtures
  byte-equal. `make validate`, selfcheck 4 EQUAL, diff-formatter 22/22, diff-linter 26/26
  stay green.

## 8. Open Questions

- Exact `Fn` modelling in the `Type` tree (name+args+ret vs a dedicated flag) — decide in
  46.2; only needs to support resolution (all parts known) this phase.
- Does stage1 emit `unknown type` for a wrong-arity builtin generic (e.g. `Array<Int,Int>`)?
  Check in 46.3/46.4 and match; otherwise out of scope.
