# 0058: Self-Hosted Checker — Cross-Module Resolution via Harness Discovery + Stage2 Merge

**Date:** 2026-07-03
**Status:** accepted
**Phase:** 54 — multi-file self-hosted checking

## Context

`intentc check --self-hosted` ran the stage2 (Intent) checker on a **single
file** (`cmd/intentc/main.go` `handleCheck` → `runStage2Checker`), while stage1
routes any file with imports through `compiler.CheckProject` →
`checker.CheckAll`, which discovers the import closure and type-checks every
module with import-scoped symbol visibility. `make diff-checker` (86/86) only
tests single-file examples + fixtures, so this gap was invisible.

Run against the compiler's OWN source, the stage2 checker flooded **false
positives** (valid code rejected): `check.intent` → 84, `parser.intent` → 461+
`unknown type` (imported entities/enums), plus ~1172 `undeclared variable`
(imported module names used as call qualifiers, e.g. `shared_lexer.foo()`). A
checker that rejects valid code cannot replace stage1, so this — not the
remaining error-diagnostic gaps (missing diagnostics on *invalid* input) — was
the real self-hosting blocker.

## Decision

Resolve imports for `--self-hosted` by **discovering the closure in the Go
harness and merging it in the stage2 driver**, keeping `check_program` a
single-`Program` checker. This mirrors stage1's split: the Go `ModuleRegistry`
is stage1's front-end too (`CheckAll` receives pre-loaded modules), so module
discovery living in Go is consistent — not a shortcut around self-hosting.

1. **Harness discovery** (`stage2CheckPaths`): for a multi-file entry, reuse
   `NewModuleRegistry` / `DiscoverDependencies` / `TopologicalSort` to get the
   closure, and pass the entry first (its path is used for diagnostic output)
   followed by every other module path to the stage2 binary. Single-file entries
   (or any discovery failure) pass the entry alone — unchanged behaviour.
2. **Stage2 merge** (`check_main.merge_programs`): parse each path and flatten
   the modules into one `Program` so `type_is_known` / the global scope see
   cross-module entities, enums, and traits. Cross-module name collisions are
   **deduped, first-seen kept** (e.g. `empty_string_array()` is defined
   identically in several shared modules; stage1 permits it via module scoping, a
   flat merge would otherwise report a false duplicate). The **entry module is
   merged verbatim** (no dedup) so genuine within-module duplicates it contains
   are still reported exactly as stage1 would; `impls`/`externs` are appended.
3. **Module-name seeding** (`check_program_seeded`): the driver seeds the
   imported modules' names into the global scope so a qualifier like
   `shared_lexer` in `shared_lexer.foo()` resolves as a known name (Unknown type →
   method-resolution is soundly skipped) instead of `undeclared variable`.
   `check_program(prog)` is now a thin wrapper (`check_program_seeded(prog, [])`),
   so single-file and in-language-test callers are byte-identical to before.
4. **Gate** (`make selfcheck-checker`): diffs `intentc check` vs `intentc check
   --self-hosted` over the nine core self-hosting source modules. NOT wired into
   `make validate` — the stage2 checker (compiled Intent) is slow (tens of
   seconds) on the large merged programs, so it runs as its own gate. The
   single-file corpus stays under `make diff-checker`.

## Consequences

### Benefits
- `intentc check --self-hosted` now matches stage1 byte-for-byte on all nine core
  compiler source modules (previously hundreds of false positives) — the actual
  self-hosting milestone, now measured by a gate.
- Reuses the robust, tested Go module registry rather than reimplementing import
  discovery, topological sort, and path resolution in Intent.
- Zero change to `check_program`'s contract: single-file / test behaviour and the
  86/86 `diff-checker` corpus are untouched.

### Costs / non-goals
- **Valid-source parity, not full multi-file parity.** The flat merge gives
  broader type visibility than stage1's per-module import scoping and does not
  reproduce per-module error POSITIONS/paths; for *invalid* multi-file input the
  two can diverge. The self-hosting target (valid source → "No errors found") is
  unaffected. The dedup assumes colliding cross-module names are compatible
  (true for the sole real collision, `empty_string_array`).
- Imported modules' function bodies are not re-checked as thoroughly as stage1's
  `CheckAll` (a sound under-report on valid source).
- Performance: the stage2 checker is slow on large merged programs; hence the
  separate, non-`validate` gate.

## References
- [ADR 0056](0056-self-hosted-expression-inference.md) — the sound-but-incomplete
  strategy; this fixes the opposite failure (false positives on valid code).
- `cmd/intentc/main.go` — `stage2CheckPaths`, `runStage2Checker`.
- `selfhost/checker/check_main.intent` — `merge_programs`.
- `selfhost/checker/check.intent` — `check_program_seeded`.
- `internal/compiler/compiler.go` — `CheckProject`; `internal/checker/checker.go`
  — `CheckAll` (the stage1 multi-file path this mirrors).
