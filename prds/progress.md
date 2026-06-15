# Norman Progress Log

Append-only execution log for crash recovery. Newest entries at the bottom.

---

## 2026-06-15 — Norman initialized + full migration from ops/

Switched from aiki (config removed by user) to norman. Initially set up lightweight,
then per user direction did a **full migration** of the project's planning layer:

- Moved all 31 phase PRDs out of `ops/plans/`: 29 shipped → `prds/done/`,
  `phase-40a-comment-preservation` (active) → `prds/active/`,
  `phase-23-marketplace-publish` (Draft, blocked on credentials) → `prds/backlog/`.
- Moved `ops/NEXT-STEPS.md` → `prds/NEXT-STEPS.md`; removed the now-empty `ops/`.
- Created `prds/TASKS.md` (live driver: active Phase 40A.2 + backlog + completed pointer)
  and `prds/TASKS-archive.md` (29-phase completed index).
- Rewrote all 72 cross-references (ROADMAP, both READMEs, HARNESS.md, ADRs, Go test
  comments, inter-PRD links) from `ops/plans/...` → `prds/{done,active,backlog}/...`.
- Rewrote `docs/HARNESS.md` workflow sections to describe the norman flow.
- ADRs (`docs/decisions/`) and `docs/ROADMAP.md` stay put — norman does not own them.
- Flipped `config.md` scaffolding `lightweight` → `full`.

Stale-but-correct: `phase-30` and `phase-31` PRD `Status:` lines still read "Planning"
though both shipped (per ROADMAP + git). Left PRD bodies untouched; noted in the archive.

Current work stream: **Phase 40A.2 — comment preservation** in the stage2 self-hosted
formatter (`selfhost/formatter/`).
- step (1) trailing-EOF comments — DONE (commit 19d766e, pre-norman).
- step (2) body/between-statement comments — NEXT.
- step (3) inline-after comments — pending.
- step (4) byte-equal self-format dogfood gate — after 1–3.

PATTERN: [selfhost] - Comment preservation follows a uniform template: add a
`comments_before: Array<String>` field to the AST node, drain it from the relevant
token's `comments_before` in the parser, and emit it via `format_comments_before` in
the formatter. The lexer already attaches comments to the next non-trivia token (and
trailing comments to the synthetic EOF token), so no lexer change is usually needed.

PATTERN: [shell] - This environment's Bash tool runs under zsh, which does NOT
word-split unquoted `$var`. Use `... | while read -r f; do ...; done` for file-list
loops, not `for f in $files`.
