# PRD — Phase 45: Self-Hosted Checker (first slice)

## 1. Introduction / Overview

Begin reimplementing Intent's semantic checker (`internal/checker/`) in Intent itself,
as a new `selfhost/checker/` tool reusing the `selfhost/shared/` front-end (Phase 44).
The Go checker is ~4,281 LOC / ~167 diagnostics with a full type system — far too large
for one phase. This phase ships a **first slice** of checks that need **no expression
type inference**: structural checks, name resolution (via an Array-based scope stack),
and call arity. Strategic frame: [ADR 0052](../../docs/decisions/0052-self-hosted-checker-strategy.md).

Stage1's Go checker stays the production checker; the stage2 checker is wired as
`intentc check --self-hosted` and grown to parity over later phases.

## 2. Goals

- Stand up `selfhost/checker/check.intent` (module `checker`) reusing `../shared/…`,
  with a `CheckDiag` model, a dispatch walk, and `error[file:line:col]: message`
  rendering matching stage1's `diagnostic.Format`.
- Implement the first-slice checks (D1 in ADR 0052): duplicate decl, duplicate enum
  variant, break/continue outside loop, return-in-test, undeclared variable + variable
  redefinition (scope stack), and function/variant/builtin call arity.
- Ship `check_main.intent` + `intentc check --self-hosted` (Go shim, mirrors fmt/lint).
- Commit `make diff-checker`: invalid fixtures (byte-equal vs stage1 errors) + no false
  positives on the 22 valid examples.
- Keep the formatter/linter gates green throughout.

## 3. Design decisions

Recorded in [ADR 0052](../../docs/decisions/0052-self-hosted-checker-strategy.md):
D1 first-slice scope (structural + name-resolution + arity, no type inference),
D2 type inference deferred, D3 scope stack via parallel Arrays (no Map), D4
two-directional differential gate, D5 faithful gate-protected port.

## 4. User Stories / Tasks

### US-001 (45.1): ADR 0052 — checker strategy
**AC:** `docs/decisions/0052-self-hosted-checker-strategy.md` records D1-D5. Done.

### US-002 (45.2): Checker scaffold + first structural check
**AC:** `selfhost/checker/check.intent` (module `checker`, imports `../shared/…`) with a
`CheckDiag` entity (line, column, message), `public function check_program(prog) ->
Array<CheckDiag>` dispatching in stage1 register-then-check order, and `format_diags`
producing `error[file:line:col]: message` (lines `\n`-joined, NO trailing newline — see
§6). Wire ONE check end-to-end: **duplicate top-level declaration** (`entity/enum/
function/trait '%s' already defined`), with ≥2 in-language tests. Verify the exact
stage1 emit order and message text against `internal/checker/`.

### US-003 (45.3): Structural no-symbol-table checks
**AC:** duplicate enum variant (`duplicate variant name '%s' in enum '%s'`),
`break statement outside loop`, `continue statement outside loop`, and the test-body
return rule — each with ≥2 tests, exact messages + anchors verified against the Go
source. (break/continue/return need a loop-depth counter + in-test flag during the
statement walk — no symbol table.)

### US-004 (45.4): Array-based scope stack / symbol table
**AC:** a `Scope` (parallel `Array`s: names + kinds, parent chain) with `define` /
`resolve` (lexical: current→parent) / `resolve_local`. A global scope populated exactly
as stage1's register passes (entities, enums, functions, traits, **and builtins**).
Unit-tested directly (define/resolve/shadowing). No user-facing check yet.

### US-005 (45.5): Undeclared-variable + variable-redefinition checks
**AC:** using 45.4, walk function/method bodies building child scopes (params, `let`
bindings, `self` in methods, for-loop var, match bindings, lambda params) and emit
`undeclared variable '%s'` on unresolved identifiers and `variable '%s' already defined
in this scope` on a redeclaration. CRITICAL: **no false positives on the 22 valid
examples** (the differential gate enforces this). ≥4 tests incl. builtins-not-flagged,
self-in-method, shadowing.

### US-006 (45.6): Call arity (function / variant / builtin)
**AC:** function-call arity (`function '%s' expects %d arguments, got %d`), variant
arity (`variant '%s' expects %d arguments, got %d`), and builtin arity (the hardcoded
per-builtin counts from the Go source). Method-call arity is DEFERRED (needs receiver
type). ≥2 tests per family; exact messages.

### US-007 (45.7): `check_main.intent` + `intentc check --self-hosted`
**AC:** `selfhost/checker/check_main.intent` (`entry main`): args → read_file → parse →
`check_program` → output matching stage1's stdout/stderr/exit contract (§6). Go shim
`stage2CheckerBinary` + `runStage2Checker` in `cmd/intentc/main.go` mirroring the
fmt/lint shims (INTENT_STAGE2_CHECK override, build/cache, staleness scans
`selfhost/checker/` + `selfhost/shared/`). +Go tests.

### US-008 (45.8): Differential gate `make diff-checker` + fixtures
**AC:** `selfhost/checker/difftest-check.sh` + Makefile `diff-checker`. For every valid
`examples/*.intent`: stage2 produces no errors (matches stage1). For each
`selfhost/checker/check-fixtures/*.intent` (one per first-slice check): stage2 byte-
equals stage1 `intentc check`. Exits non-zero on any divergence.

### US-009 (45.9): Docs + final validate + push
**AC:** `selfhost/checker/README.md`, `selfhost/README.md`, ROADMAP Phase 45,
NEXT-STEPS. `make validate` + all four `diff`/`selfcheck` gates green. Commit + push.

## 5. Non-Goals

- Type inference, assignability, operators, generics, traits, match exhaustiveness,
  contracts, async/await — later phases.
- Method-call arity (needs receiver type inference).
- Multi-file / `CheckAll` cross-package checking — single-file `Check` first.
- Replacing stage1's Go checker as default (`intentc check` stays Go).

## 6. Technical Considerations

- **`intentc check` output contract (verified, main.go:220-266):** valid file →
  `No errors found.\n` on stdout, exit 0. Invalid file → `diag.Format()` (`error[file:
  line:col]: message`, `\n`-joined, NO trailing newline) on stderr, exit 1. Not sorted
  (walk order). `check_main` prints to stdout only; the `--self-hosted` shim must route
  errors to stderr + set the exit code, and the differential must compare the right
  streams (capture `2>&1` for stage1, account for the trailing-newline difference).
  Pin this empirically in 45.2/45.7 (the fmt/lint shims are the template).
- **Scope population must match stage1 exactly** (builtins, `self`, module-qualified
  names) or undeclared-variable false-positives on the valid corpus. The
  no-false-positives gate is the forcing function.
- **No `Map` / no `break` in stage2** — symbol table = parallel `Array`s + linear
  lookup; early-exit loops use a `running: Bool` guard (Phase 43 patterns).
- **Never run stage1 `intentc fmt` on a stage2 file**; keep `check.intent` canonical.
- **Emit/dispatch order** is load-bearing (errors unsorted) — verify per-check against
  `internal/checker/` (register passes, then body checks).

## 7. Success Metrics

- `make diff-checker`: 0 errors on all 22 valid examples + byte-equal on every fixture.
- `intentc check --self-hosted` byte-identical to `intentc check` on corpus + fixtures.
- `make validate`, `make selfcheck-formatter` (4 EQUAL), `make diff-formatter` (22/22),
  `make diff-linter` (26/26) stay green.

## 8. Open Questions

- Exact builtin set to seed the global scope (so builtins aren't flagged undeclared) —
  enumerate from the Go checker in 45.4/45.5.
- Whether `check_main` should print errors to stdout and let the shim re-route to
  stderr, or match exit codes only — decided in 45.7 against the verified contract.
