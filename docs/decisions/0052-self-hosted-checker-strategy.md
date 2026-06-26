# 0052: Self-Hosted Checker Strategy

**Date:** 2026-06-26
**Status:** accepted
**Phase:** 45 — Self-Hosted Checker (first slice; later phases widen it)

## Context

Per `[[project-self-hosting-priority]]`, the end goal is a fully self-hosted Intent
compiler. The **formatter** ([ADR 0040](0040-self-hosted-formatter-strategy.md), Phases
38-42) and **linter** ([ADR 0050](0050-self-hosted-linter-strategy.md), Phase 43) are
done and byte-equal with their stage1 counterparts. Phase 44 ([ADR 0051](0051-selfhost-shared-restructure.md))
restructured the stage2 toolchain into `selfhost/shared/` + per-tool siblings, so the
checker can start as a clean `selfhost/checker/` importing `../shared/…`.

The **checker** (`internal/checker/`) is the first *compiler* subsystem and is large:
~4,281 LOC, ~167 distinct diagnostics, a full type system (Type struct, lexical scope
stack, symbol table, generic substitution). Self-hosting it is a multi-phase effort.
This ADR records the strategy and scopes the **first slice** (Phase 45).

### Stage2 AST readiness (measured during Phase 44 planning)

The stage2 AST stores types as flat `String`s (no structured `TypeRef`), `Expr` carries
no type field, and there is no symbol table and no `Map` type (only `Array`, linear
scans). Therefore **type-inference checks are out of reach** without major AST
enrichment, while **structural and name-resolution checks are feasible today**, reusing
the linter's `Array<String>` machinery.

## Decision

Build a stage2 checker in `selfhost/checker/` (module `checker`), reusing
`../shared/{lexer,ast,parser}`. Stage1's Go checker remains the production checker; the
stage2 checker is grown to parity across phases (the formatter/linter pattern).

### D1 — First-slice scope: structural + name-resolution + arity (no type inference)
Phase 45 ports the checks that need **no expression type inference**:
- **Structural (no symbol table):** duplicate top-level declaration (`entity/enum/
  function/trait '%s' already defined`), duplicate enum variant (`duplicate variant
  name '%s' in enum '%s'`), `break statement outside loop` / `continue statement
  outside loop`, and the test-body return rule.
- **Name resolution (Array-based scope stack):** `undeclared variable '%s'` and
  `variable '%s' already defined in this scope`, via a lexical scope chain populated
  exactly as stage1 does (globals = entities/enums/functions/traits + builtins; then
  params, `let` bindings, `self`, loop/match/lambda bindings).
- **Arity (using the registry):** function-call arity (`function '%s' expects %d
  arguments, got %d`), variant arity, and builtin arity. **Method-call arity is
  deferred** — it needs the receiver's entity type (type inference).

### D2 — Type inference and the remaining ~140 diagnostics are deferred
A structured `TypeRef`/`Type` entity and a richer symbol table are prerequisites and
are their own later phases. Phase 45 deliberately ships ~25-30% of the checker with
zero type-inference machinery.

### D3 — Scope stack via parallel `Array`s (no `Map` in stage2)
Stage2 has no `Map`. The symbol table is built from parallel `Array`s with linear
lookup (the linter's `name_in` pattern), one scope per lexical level with a parent
chain. Name resolution must populate scopes identically to stage1 to avoid false
positives — the valid-corpus gate (D4) is the forcing function.

### D4 — Two-directional differential gate (`make diff-checker`)
The valid examples corpus produces **zero** checker errors, so a one-directional
corpus diff would be vacuous. The gate is therefore both:
- **No false positives:** stage2 produces no errors on the 22 valid `examples/*.intent`
  (matching stage1's none) — this protects name-resolution especially.
- **Invalid fixtures:** a set of intentionally-erroneous fixtures (one per check) where
  stage1 `intentc check` emits errors and stage2 reproduces them **byte-equal**.

`intentc check` output (verified, `cmd/intentc/main.go:220-266`): a valid file prints
`No errors found.\n` to **stdout** (exit 0); an invalid file prints `diag.Format()` —
`error[file:line:col]: message` lines joined by `\n`, no trailing newline — to
**stderr** (exit 1). Errors are emitted in walk order (the `diagnostic` package does
not sort). The stage2 `check_main` prints to stdout only, so the `--self-hosted` shim
and the differential reconcile the stderr/stdout split + exit code (the formatter/
linter-shim pattern; pinned empirically in 45.2/45.7/45.8).

### D5 — Faithful port, gate-protected
A faithful reproduction of stage1, not a redesign. Per-check emit order and scope
population are verified against `internal/checker/`. The formatter/linter gates stay
green throughout.

## Consequences

### Benefits
- Third self-hosted artefact; first compiler subsystem. The scope-stack / symbol-table
  machinery built here is the foundation every later checker phase (and the eventual
  self-hosted type system) reuses.
- Proves the `selfhost/shared/` front-end generalises to a third, deeper consumer.

### Costs
- Double maintenance (stage1 Go checker + stage2 Intent checker) for the checker's long
  road to parity.
- Name resolution is the subtle part: stage2 must populate scopes exactly as stage1
  (including builtins, `self`, module-qualified names) or it false-positives on the
  valid corpus. The no-false-positives gate catches this but it must be designed in.

### Non-goals (this phase)
- Type inference, assignability, operator typing, generics, traits, match
  exhaustiveness, contracts, async/await, method-call arity — all later phases.
- Multi-file / cross-package checking (`CheckAll`) — single-file (`Check`) first.

## References
- [ADR 0050](0050-self-hosted-linter-strategy.md) / [ADR 0040](0040-self-hosted-formatter-strategy.md) — the self-hosting pattern this follows.
- [ADR 0051](0051-selfhost-shared-restructure.md) — the `selfhost/shared/` layout the checker is a sibling of.
- `internal/checker/` — the behaviour being ported (checker.go, scope.go, types.go).
- `cmd/intentc/main.go:220-266` — `intentc check` output contract.
- PRD: `prds/active/prd-phase-45-self-hosted-checker.md`.
