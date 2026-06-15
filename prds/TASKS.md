# Norman Tasks

Live task list. The active phase is expanded into steps; completed phases are
collapsed into [TASKS-archive.md](TASKS-archive.md). Per-phase design detail
lives in the PRD files under `done/`, `active/`, and `backlog/`, and the rich
shipped summaries remain in [docs/ROADMAP.md](../docs/ROADMAP.md).

Resuming? Read [NEXT-STEPS.md](NEXT-STEPS.md) first.

## Phase 40A.2: Stage2 Comment Preservation (active)

Closes the comment-preservation half of the byte-equal self-format gate for the
self-hosted formatter (`selfhost/formatter/`). Sub-pieces C (source-order) and B
(paren stripping) and A.1 (leading-decl comments) already shipped (Phases 40C /
40B / 40A.1).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.2.1 | Trailing-EOF comments | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | commit 19d766e; +5 tests; 136/136 rust+js |
| 40A.2.2 | Body / between-statement comments (`Stmt.comments_before`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 141/141 rust+js |
| 40A.2.3 | Inline-after comments on statements (`let x = 1; // ...`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 146/146 rust+js; statements only |
| 40A.2.4 | Comprehensive synthetic comment round-trip (partial gate) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | 1 test exercising all 4 supported positions; 147/147. Real-file byte-equal gate moved to Phase 40A.3 (see below) |

**Phase 40A.2 complete** — comments now round-trip in 4 positions: leading-decl, between-statement, inline-after-statement, trailing-EOF.

## Phase 40A.3: Real-file byte-equal self-format (active/next)

A probe (2026-06-15) measured `format(parse(src)) == src` against the 4 stage2 source files: all diverge. Remaining comment positions to support, then the source files must be canonicalized (the gate is idempotence on canonical source, not on the hand-written originals).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.3.1 | Module-leading comments (before `module`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | ModuleDecl.comments_before; +2 tests; 149/149 |
| 40A.3.2 | Comments before entity fields | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comments_before |
| 40A.3.3 | Comments before entity methods / constructor | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | + impl methods; FunctionDecl.comments_before |
| 40A.3.4 | End-of-block comments (before `}`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | Block.trailing_comments from rbrace token; +2 tests; 155/155 |
| 40A.3.5 | Inline-after on fields | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comment_after; +4 tests total; 153/153 |
| 40A.3.6 | Canonicalize stage2 files + add real-file gate test | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | TODO | run formatter to de-align/normalize the 4 files (must still compile + self-parse), commit, then assert `format(parse(src)) == src` |

Note: column-aligned inline comments (e.g. `field x: T;       // ...`) can't survive a canonical formatter as-is; 40A.3.6 resolves this by normalizing the source to single-space (de-alignment), making the formatter a fixpoint.

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |
| 41 | Parser surface widening (`requires`/`ensures`, `match`, `for-in`, `try ?`) | _(PRD TBD)_ | TODO | follows 40A.2; unblocks stage2 over real examples |

## Completed Phases (11–40C) — see [TASKS-archive.md](TASKS-archive.md)

29 phases shipped (Phases 11–40C, excl. 40A which is active). Full index with
per-phase status, dates, and PRD links is in the archive.
