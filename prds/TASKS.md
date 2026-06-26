# Norman Tasks

Live task list. The active phase is expanded into steps; completed phases are
collapsed into [TASKS-archive.md](TASKS-archive.md). Per-phase design detail
lives in the PRD files under `done/`, `active/`, and `backlog/`, and the rich
shipped summaries remain in [docs/ROADMAP.md](../docs/ROADMAP.md).

Resuming? Read [NEXT-STEPS.md](NEXT-STEPS.md) first.

## Phase 40A.2: Stage2 Comment Preservation — COMPLETE (2026-06-15)

Closes the comment-preservation half of the byte-equal self-format gate for the
self-hosted formatter (`selfhost/formatter/`). Sub-pieces C (source-order) and B
(paren stripping) and A.1 (leading-decl comments) already shipped (Phases 40C /
40B / 40A.1).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.2.1 | Trailing-EOF comments | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | commit 19d766e; +5 tests; 136/136 rust+js |
| 40A.2.2 | Body / between-statement comments (`Stmt.comments_before`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 141/141 rust+js |
| 40A.2.3 | Inline-after comments on statements (`let x = 1; // ...`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 146/146 rust+js; statements only |
| 40A.2.4 | Comprehensive synthetic comment round-trip (partial gate) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | 1 test exercising all 4 supported positions; 147/147. Real-file byte-equal gate moved to Phase 40A.3 (see below) |

**Phase 40A.2 complete** — comments now round-trip in 4 positions: leading-decl, between-statement, inline-after-statement, trailing-EOF.

## Phase 40A.3: Real-file byte-equal self-format — COMPLETE (2026-06-15)

**Byte-equal self-format achieved on all 4 stage2 files** (`lexer.intent`,
`ast.intent`, `parser.intent`, `format.intent`): `format(parse(src)) == src`.
A probe drove discovery of each remaining divergence; the 4 files were then
canonicalized (reformatted by the formatter) so it is a fixpoint on them.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.3.1 | Module-leading comments (before `module`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | ModuleDecl.comments_before; +2 tests |
| 40A.3.2 | Comments before entity fields | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comments_before |
| 40A.3.3 | Comments before entity methods / constructor | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | + impl methods; FunctionDecl.comments_before |
| 40A.3.4 | End-of-block comments (before `}`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | Block.trailing_comments from rbrace token; +2 tests |
| 40A.3.5 | Inline-after on fields | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comment_after; +4 tests |
| 40A.3.7 | Inline-after on declaration closing `}` | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | Block.brace_comment_after; fixed dropped one-liner doc-comments; +2 tests |
| 40A.3.8 | Generic type-arg round-trip (`Array<String>`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | parse_type_name reconstructs args (was `<...>` placeholder) — the real byte-equal blocker; +1 test |
| 40A.3.6 | Canonicalize stage2 files + real-file gate test | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | reformatted all 4 files (still compile + self-parse + 158/158); self_format_one asserts byte-equality; probe confirmed firstdiff -1 + idempotent on all 4 |

**Phase 40 complete.** Byte-equal self-format gate met (sub-pieces 40C source-order,
40B paren-stripping, 40A.1/40A.2/40A.3 comments). Stage2 formatter is a fixpoint on its
own source.

## Phase 41: Stage2 Parser Surface Widening — COMPLETE (2026-06-15)

Widened the stage2 parser beyond its self-hostable subset so it can format arbitrary Intent. Each sub-feature round-trips through parse + format; byte-equal self-format on the stage2 files preserved throughout. See [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 41.1 | Contracts: `requires` / `ensures` / `decreases` on functions + methods | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | FunctionDecl.{requires,ensures,decreases}_clauses; formatted between signature and `{`; +4 tests |
| 41.2 | `match` expressions over Result/Option | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | ex_match + MatchArm; level-aware format_match via format_expr_indented; +3 tests |
| 41.3 | `for ... in ...` loops | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | st_for (reuses Stmt name/expr/then_block); +3 tests |
| 41.4 | `try ?` operator | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | ex_try postfix; +2 tests; 170/170 rust+js |

## Phase 42: Stage2 Formatter CLI Wiring + Differential Test — 12 tasks completed 2026-06-16 (corpus 22/22 vs `intentc fmt`; see [TASKS-archive.md](TASKS-archive.md))

## Phase 43: Self-Hosted Linter (stage2) — 13 tasks completed 2026-06-24 (all 16 rule families; `make diff-linter` 26/26 byte-equal vs `intentc lint`; see [TASKS-archive.md](TASKS-archive.md))

## Phase 44: selfhost/shared Restructure — 5 tasks completed 2026-06-26 (shared/ + formatter/ + linter/ siblings; all gates green; see [TASKS-archive.md](TASKS-archive.md))

## Phase 45: Self-Hosted Checker (first slice) — ACTIVE

First compiler subsystem self-hosted in Intent: a `selfhost/checker/` sibling reusing
`../shared/`. First slice = structural checks + name-resolution (Array scope stack) +
call arity; NO type inference (deferred). Two-directional `make diff-checker` gate
(invalid fixtures byte-equal vs stage1 + no false positives on the valid corpus). See
[prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) and ADR 0052.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 45.1 | ADR 0052 — self-hosted checker strategy | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | DONE (2026-06-26) | docs/decisions/0052; D1 first-slice scope, D2 type-inference deferred, D3 Array scope stack, D4 two-dir diff, D5 faithful port |
| 45.2 | Checker scaffold + duplicate-decl check | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | DONE (2026-06-26) | selfhost/checker/check.intent + check_test.intent; CheckDiag, format_diags (error[f:l:c], no trailing \n), dispatch order enums→entities→traits→functions (matches checker.go:113-116), dup-decl all 4 kinds; 8 tests, 114 rust+js; gates green |
| 45.3 | Structural checks: dup enum variant + return-in-test | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | DONE (2026-06-26) | dup-variant in enum loop (register phase); return-in-test recursive over test bodies (check phase) with double-quoted name; two-phase register→check structure; +6 tests (120); gates green |
| 45.4 | Stage2 break/continue statement support | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | DONE (2026-06-26) | kw_break/kw_continue (lexer) + st_break/st_continue (ast) + parser + formatter emit; error_handling.intent still byte-equal (diff-formatter 22/22), selfcheck 4 EQUAL; parser 108, format_test 210, go test green |
| 45.5 | break/continue outside loop check | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | loop-depth walk; `break/continue statement outside loop`; needs: 45.4 |
| 45.6 | Array-based scope stack / symbol table | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | parallel-Array Scope (define/resolve/resolve_local), globals + builtins; needs: 45.2 |
| 45.7 | Undeclared-variable + redefinition checks | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | uses 45.6 + 45.4 (break/continue now statements, not ident-uses); NO false positives on valid corpus; needs: 45.4,45.6 |
| 45.8 | Call arity (function/variant/builtin) | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | method-arity deferred (needs receiver type); needs: 45.6 |
| 45.9 | check_main.intent + `intentc check --self-hosted` shim | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | stdout/stderr/exit contract; Go shim mirrors fmt/lint; needs: 45.7,45.8 |
| 45.10 | Differential gate `make diff-checker` + fixtures | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | invalid fixtures byte-equal + no-false-positives on 22 valid examples; needs: 45.9 |
| 45.11 | Docs + final validate + push | [prd-phase-45-self-hosted-checker.md](active/prd-phase-45-self-hosted-checker.md) | TODO | checker README + selfhost README + ROADMAP + NEXT-STEPS; needs: 45.10 |

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |

## Completed Phases (11–40) — see [TASKS-archive.md](TASKS-archive.md)

Phases 11–40 shipped (incl. Phase 40 byte-equal self-format). Full index with
per-phase status, dates, and PRD links is in the archive. Next: Phase 41 (parser
surface widening) — see [NEXT-STEPS.md](NEXT-STEPS.md).
