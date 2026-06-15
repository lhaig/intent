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

## Phase 42: Stage2 Formatter CLI Wiring + Differential Test — ACTIVE

Make the stage2 formatter a runnable CLI tool, wire it into `intentc fmt
--self-hosted`, and stand up a committed differential-test harness vs `intentc
fmt` over `examples/*.intent`. The harness drives gap-closing: each example the
stage2 parser can't yet handle is a small parse+format task (Phase 41 pattern).
Baseline: 12/22 examples already byte-equal (= agree with `intentc fmt`). See
[phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 42.1 | `args()` builtin (Array<String>) + ADR; checker, IR, rust/js/wasm backends | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | ADR 0045; +2 checker tests; rust=env::args, js=process.argv.slice(1), wasm=stub; emit verified rust+js |
| 42.2 | `main.intent` runnable formatter (reads args()[1], prints format_program) | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | builds+runs rust+js; hello byte-equal modulo 1 trailing newline; exit codes 0/1/2/3. Fixed stage1 bug: entry fn in imported module no longer dupes main (rustbe+jsbe; +2 tests) |
| 42.3 | Differential-test harness: `difftest.sh` + `make diff-formatter` | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | canonicalize-first (compares vs intentc fmt output); 13/22 PASS, 0 diverge, 9 parse-err; exits 1 as gate; bash 3.2 compatible |
| 42.4 | `intentc fmt --self-hosted` Go shim (delegates to stage2 binary) | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | env override INTENT_STAGE2_FMT + auto-build-with-cache; composes with --check; parse error exits non-zero, no fallback; +13 tests (fake-binary, no cargo); byte-equal w/ native fmt verified e2e |
| 42.5 | Gap: entity `invariant <expr>;` (+ constructor contracts + intent blocks) | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | fixed invariant form+position (was wrong block form); folded in constructor contracts + intent-block `intent "d" {...verified_by:[...]}` (needed by the 3 files); bank_account/js_demo/task_queue PASS; 16/22; byte-equal preserved; 171/171 rust+js |
| 42.6 | Gap: `forall`/`exists` quantifier expressions | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | sorted_check |
| 42.7 | Gap: `implies` operator | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | try_operator |
| 42.8 | Gap: generic type params on declarations `<T>` | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | generic_stack |
| 42.9 | Gap: `Fn(...) -> T` types + lambdas | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | closure_demo |
| 42.10 | Gap: `async` functions + `await` | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | async_demo |
| 42.11 | Gap: attributes `@name(args)` | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | TODO | target_specific_demo |
| 42.12 | char_string_demo: compare vs stage1 output; confirm or file follow-up | [phase-42-formatter-cli-differential.md](active/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | RESOLVED by 42.3 harness design: canonicalize-first comparison makes char_string_demo PASS — no real stage2 divergence (the raw fixture was simply non-canonical) |

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |

## Completed Phases (11–40) — see [TASKS-archive.md](TASKS-archive.md)

Phases 11–40 shipped (incl. Phase 40 byte-equal self-format). Full index with
per-phase status, dates, and PRD links is in the archive. Next: Phase 41 (parser
surface widening) — see [NEXT-STEPS.md](NEXT-STEPS.md).
