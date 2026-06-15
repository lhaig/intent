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
| 40A.2.2 | Body / between-statement comments (`Stmt.comments_before`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | TODO | next; biggest remaining divergence |
| 40A.2.3 | Inline-after comments (`let x = 1; // ...`) | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | TODO | most involved; needs `Token.comment_after` |
| 40A.2.4 | Byte-equal self-format dogfood gate test | [phase-40a-comment-preservation.md](active/phase-40a-comment-preservation.md) | TODO | after .1–.3; assert `format(parse(src)) == src` on all 4 stage2 files |

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |
| 41 | Parser surface widening (`requires`/`ensures`, `match`, `for-in`, `try ?`) | _(PRD TBD)_ | TODO | follows 40A.2; unblocks stage2 over real examples |

## Completed Phases (11–40C) — see [TASKS-archive.md](TASKS-archive.md)

29 phases shipped (Phases 11–40C, excl. 40A which is active). Full index with
per-phase status, dates, and PRD links is in the archive.
