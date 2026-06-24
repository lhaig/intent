# Pickup Notes — 2026-06-24 (Phase 43 COMPLETE: self-hosted linter at full parity)

## Where we are

**Phase 43 (Self-Hosted Linter) — COMPLETE.** The Intent-implemented (stage2)
linter is a runnable, CLI-wired tool that matches stage1's Go `intentc lint`
**byte-for-byte across the entire examples corpus plus dedicated fixtures**:
`make diff-linter` → **26/26 PASS, 0 diverged, 0 parse-err**. All **16 Go-linter
rule families** are ported. It reuses the stage2 lexer/parser/AST that the formatter
already established, and is wired as `intentc lint --self-hosted`. Stage2 linter
suite: 269 in-language tests on rust + js. The formatter gates are preserved
throughout (`make diff-formatter` 22/22, `make selfcheck-formatter` 4 EQUAL).

This is the **second self-hosted toolchain artefact** after the formatter — the
proof point named in HARNESS.md §7. Strategy: [ADR 0050](../docs/decisions/0050-self-hosted-linter-strategy.md).

Shipped this phase (43.1-43.13):
- **43.1 ADR 0050** — strategy: faithful port, reuse `selfhost/formatter/`, byte-equal
  *including column*, all-16-rules gated by the differential.
- **43.2 column tracking** — `column` (and `line` where missing) on the stage2 AST
  nodes the linter anchors on (decls, `Stmt`, `Param`, `EnumVariant`, `TraitMethodSig`).
- **43.3 scaffold** — `lint.intent` (module `formatter_linter`): `LintDiag`, dispatch
  walk in stage1 order, `format_diags`, `is_snake_case`/`is_pascal_case`.
- **43.4 used-name engine** — functional `collect_used_names` / `collect_assigned_names`.
- **43.5-43.9 rules** — all 16 families across the `lint_*` dispatch functions, in
  stage1's exact per-decl order; type-param rules added additive position arrays +
  a token-aware `type_uses_param`.
- **43.10 `lint_main.intent`** — runnable entry, output byte-identical to `intentc lint`.
- **43.11 `intentc lint --self-hosted`** — Go shim mirroring `fmt --self-hosted`.
- **43.12 `make diff-linter`** — differential harness + `lint-fixtures/` (26/26).
- **43.13 docs** — this file, ROADMAP, both selfhost READMEs.

## Key learnings (full notes in progress.md)

- **Diagnostics are emitted in APPEND order — stage1 does not sort.** So stage2 must
  match stage1's dispatch order across decl kinds AND each rule's order within a decl
  (and each rule's recursion behaviour: `checkUnusedVariables` recurses,
  `checkMutableNeverReassigned` does NOT, `checkSpawnWithoutAwait` recurses). Verify
  per-rule against `internal/linter/linter.go`; don't assume uniformity.
- **Stage2 has no assignment statement** — `x = y` is a `st_expr` wrapping an
  `ex_binop "="`. The used-name engine treats the LHS plain-ident as a write, not a read.
- **A rule keyed off a construct the stage2 parser can't represent** (trait-method /
  extern contracts): check whether the parser can even PARSE it. It can't (both require
  `;` after the return type), so every parseable trait-method/extern is contract-less →
  "always fire" R3/R4 is byte-equal-correct, no AST enrichment needed.
- **R4 (extern) can't be differentially gated** — stage1 (`extern function … from
  "path"`) and stage2 (`extern "target" function …`) have incompatible extern syntax;
  no shared fixture parses in both. R4 is unit-test-only.
- **Additive AST fields are safe** — new position/metadata fields (column, type-param
  positions) defaulted in the constructor body + assigned post-construction don't touch
  the formatter, so `selfcheck-formatter` / `diff-formatter` stay green.
- **Intent has no `break`** — early-exit loops use a `let mutable running: Bool` guard.

## Candidate next directions (self-hosting path; not yet chosen)

1. **Self-host the compiler — the big one.** Formatter + linter both reuse the stage2
   lexer/parser/AST; the compiler (checker + IR + a backend) is the next and largest
   target. Likely wants a split-package architecture over the package registry
   (Phase 30). Start by scoping which subsystem goes first (the checker is the natural
   entry — it's pure AST analysis, like the linter but deeper). ADR-worthy.
2. **Promote stage2 fmt/lint toward canonical.** Both hold byte-equal parity on the
   corpus. Consider widening the corpus (more/larger real `.intent` files) to harden
   them, then make `--self-hosted` the default and begin retiring the stage1 Go paths.
3. **Stage2 parser surface gaps surfaced this phase** — extern contracts (the `from`
   clause), trait-method contracts. Closing these would let R3/R4 be fully
   differentially gated and is prerequisite for self-hosting any code that uses FFI
   with contracts. Small, well-scoped parser-widening tasks (Phase 41 pattern).
4. **(Optional) `selfhost/shared/` restructure** — once a third stage2 tool (the
   compiler) lands, split the shared lexer/parser/AST out of `selfhost/formatter/`
   (deferred per ADR 0050 D1; needs its own ADR + module renames).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md`.
2. `continue norman` finds nothing queued — pick a direction above (recommend #1, the
   checker, to keep compounding the stage2 toolchain toward a self-hosted compiler),
   scope it as Phase 44, add a TASKS.md row, then proceed.
3. Validate with `make validate`, `make diff-formatter`, `make selfcheck-formatter`,
   `make diff-linter`.
