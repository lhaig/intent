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

## Phase 43: Self-Hosted Linter (stage2) — ACTIVE

Faithful port of the 16 Go-linter rule families into Intent, reusing the stage2
lexer/parser/AST. Byte-equal parity with stage1 `intentc lint` (including `:col`),
gated by `make diff-linter`. See [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md)
and ADR 0046. Corpus baseline: 76 warnings / 13 files / 8 of 16 families.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 43.1 | ADR 0046 — self-hosted linter strategy | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-23) | docs/decisions/0046; D1 reuse-formatter-dir, D2 byte-equal-w/-col, D3 all-16-gated |
| 43.2 | Column tracking in stage2 AST + parser | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | +column on 9 decls; +line+column on Stmt/Param/EnumVariant/TraitMethodSig; defaulted-in-body + post-construction assign; +4 tests; gates green |
| 43.3 | Linter core scaffold + diagnostic model | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | lint.intent + lint_test.intent; LintDiag, dispatch (fns→externs→entities→enums→traits→impls→intents), format_diags, is_snake/pascal_case, R5; +18 tests; gates green |
| 43.4 | Used-name + assigned-name engine | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | collect_used_names/from_stmt/from_expr + collect_assigned_names + name_in; functional (returns arrays) due to pass-by-value; +5 engine tests; gates green |
| 43.5 | Complete lint_function_decl (R10,R1,R5,R14,R12,R13,R16) | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | stage1 order; reusable check_unused_params/variables/mutable + find_discarded_spawns; advisor revise fixed R12/R13 split, R13 top-level, R16 recursion, R15 deferred; +tests; gates green |
| 43.6 | Complete lint_entity_decl + lint_impl_decl | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | entity R6/R9/ctor-R14 + methods R10/R2/R5/R14/R12/R13; impl R10/R5/R14/R12; R15e deferred to 43.8; needs: 43.4 |
| 43.7 | Complete lint_enum/trait/extern/intent dispatch | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | R7,R8,R6-trait(quirk),R5+R3,R4,R11; no engine needed; needs: 43.3 |
| 43.8 | Type-param position enrichment + R15 (fn) + R15e (entity) | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | Array<TypeParam>{name,line,column} in ast+parser, then byte-equal R15; needs: 43.5,43.6 |
| 43.9 | Runnable `lint_main.intent` | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | rust+js; exit codes + summary mirror stage1; needs: 43.5-43.8 |
| 43.10 | `intentc lint --self-hosted` Go shim | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | mirrors `fmt --self-hosted`; needs: 43.9 |
| 43.11 | Differential harness + fixtures + `make diff-linter` | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | 76/76 corpus + golden fixtures byte-equal; needs: 43.9 |
| 43.12 | Docs (ROADMAP/NEXT-STEPS/README) + final validate | [prd-phase-43-self-hosted-linter.md](active/prd-phase-43-self-hosted-linter.md) | TODO | needs: 43.11 |

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |

## Completed Phases (11–40) — see [TASKS-archive.md](TASKS-archive.md)

Phases 11–40 shipped (incl. Phase 40 byte-equal self-format). Full index with
per-phase status, dates, and PRD links is in the archive. Next: Phase 41 (parser
surface widening) — see [NEXT-STEPS.md](NEXT-STEPS.md).
