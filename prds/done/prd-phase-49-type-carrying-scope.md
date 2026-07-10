# PRD — Phase 49: Type-Carrying Scope (ident / field / call-return inference)

## 1. Introduction / Overview

Phase 48's `infer_expr_type` returns Unknown for identifiers, field access, and calls
because the checker's `Scope` carries only **names**, not types. Phase 49 adds a
**type-carrying scope** so those expressions infer, which is the keystone that unlocks the
remaining type-rule checks (Phases 50–52). Strategic frame:
[ADR 0056](../../docs/decisions/0056-self-hosted-expression-inference.md) (write a Phase-49
ADR for the scope-representation decision).

## 2. Goals

- A scope structure mapping `name -> Type`, seeded and threaded alongside the existing
  name-only `Scope` (or extended additively) — the byte-equal name-resolution paths must
  stay untouched.
- Seed: function/method **params** (`parse_type(param.type_name)`), **`self`** (the
  enclosing entity type), **`let`** bindings (declared type, else inferred RHS), **for-loop**
  vars (the iterable's element type when inferable).
- Extend `infer_expr_type` (still SOUND — Unknown when not in scope): `ex_ident` (scope
  lookup), `ex_field` (entity field type via the entity registry), `ex_call` (function
  return type via the function registry), and `ex_index` (Array/Map element type) where
  confident.
- No new diagnostic required — this is inference completeness. `make diff-checker` stays
  green; the Phase-48 checks (condition-boolean, let-mismatch) now catch more cases (add
  fixtures where new coverage appears, verified byte-equal).

## 3. Design decisions (record in the Phase-49 ADR)

- Keep the name-only `Scope` for undeclared-variable resolution; add a **parallel typed
  environment** (name + Type arrays, mirroring the flattened-scope pattern — no `Map`, no
  recursive field) rather than mutating `Scope`'s signature everywhere. Thread it through
  `check_body_stmts` next to `cur_scope`.
- Inference stays sound: a name not in the typed env, or a field/call whose type can't be
  resolved, yields Unknown (never a guess).
- Reuse `Type` / `parse_type` / `type_is_known` (Phase 46) and build the function-return
  and entity-field registries the way the arity registries are built.

## 4. Tasks (indicative)

- US-1: typed-env entity + define/lookup helpers; seed params + `self` in the fn/method
  scope builders.
- US-2: thread the typed env through `check_body_stmts` (and its recursive if/while/for
  calls); seed `let` and for-loop bindings.
- US-3: extend `infer_expr_type` for `ex_ident` / `ex_field` / `ex_call` / `ex_index`
  (function-return + entity-field registries).
- US-4: fixtures + tests for newly-caught condition-boolean / let-mismatch cases
  (e.g. `let b: Bool = someIntVar;`), byte-equal; 22 examples stay clean.
- US-5: docs + validate + push.

Ref stage1: `checkIdentifier`, `checkFieldAccessExpr`, `checkCallExpr` (1655 return type),
`checkForInStmt` (1414 element type), `checkConstructor`/`checkMethod` scope seeding
(1108/1161).

## 5. Non-Goals

- The type-rule checks that consume the typed scope — operator typing + assignment
  mismatch (Phase 50), argument typing + method arity (Phase 51), return/match/contract
  (Phase 52).

## 6. Success Metrics

`make diff-checker` green with any new byte-equal fixtures + 22 examples clean; all
formatter/linter/self-check gates + `make validate` green.
