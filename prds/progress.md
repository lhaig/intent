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

PATTERN: [repo] - `intent/CLAUDE.md` is a symlink to `intent/AGENTS.md` — edit the
real target (AGENTS.md); writing through the symlink is refused.

---

## 2026-06-15 — Phase 40A.2 step (2): body/between-statement comments

Delegated to a Sonnet worker (first real norman worker run); advisor (Opus) wrote the
brief, reviewed the diff, and re-ran tests independently before committing. `Stmt`
gains `comments_before` (defaulted via a new module-local `empty_string_array()` in
ast.intent); `parse_block` captures each statement's first-token comments and assigns
them onto the Stmt; `format_stmt` is split into a comment-emitting wrapper +
`format_stmt_inner`. 5 new tests. 141/141 rust+js, independently re-verified. Code
review PASS — edits match the step-1 template exactly.

---

## 2026-06-15 — Phase 40A.2 step (3): inline-after comments on statements

Delegated to a Sonnet worker; advisor designed the lexer same-line-detection
mechanism, reviewed the diff, re-verified tests. Token gains `comment_after`;
Lexer gains `saw_newline_since_token` + `pending_inline_after`; `scan_all` rewritten
to hold the previous token in a local (so `comment_after` is set before push —
array-element mutation post-push is unreliable in the stage1 backend) and attach the
same-line comment to it. Stmt gains `comment_after`, captured from the `;` token in
parse_let/return/expr; the format_stmt wrapper appends ` <comment>` (canonical single
space). 5 new tests. 146/146 rust+js, independently re-verified. Code review PASS.

PATTERN: [stage1-backend] Mutating an already-pushed Array element (`arr[i].field = x`)
is unreliable; hold the element in a local, mutate, then push.

---

## 2026-06-15 — Phase 40A.2 step (4) + byte-equal probe finding; re-scoped 40A.3

Wrote a throwaway probe (compiled by stage1) that ran `format(parse(src)) == src` on
the four stage2 files. ALL diverge: first diff at index 0 (file-header comment before
`module` is dropped) and large length deltas (parser −6566 chars) from comments in
non-statement positions (entity method-docs, fields, end-of-block) plus column-aligned
inline comments a canonical formatter can't reproduce. So the "byte-equal gate" is a
mini-phase, not a test.

Delivered the achievable part as step (4): a comprehensive synthetic round-trip test
exercising all four supported comment positions together (leading-decl + inline-after +
leading-body + trailing-EOF). 147/147 rust+js. Probe deleted (not committed).

Re-scoped the remaining real-file byte-equal work as **Phase 40A.3** in TASKS.md
(module-leading, entity field/method, end-of-block, inline-after-field comments, then
canonicalize the source files + add the real-file gate). Phase 40A.2 is complete.

PATTERN: [stage1-test-io] `intentc test` runs the test binary from a temp cwd, so
`read_file` needs ABSOLUTE paths (relative paths silently skip); `print` inside a test
is suppressed — `write_file` to an absolute path to surface diagnostics.

---

## 2026-06-15 — Remove aiki block from CLAUDE.md / AGENTS.md

Replaced the ~500-line `<aiki>` instruction block in `AGENTS.md` (the real target of
the `CLAUDE.md` symlink) with concise norman-oriented project guidance: where state
lives (`prds/`), how to drive norman, and the project conventions (PRD lifecycle,
ADRs, validation harness, commit style). Dropped aiki/JJ-workspace-specific machinery
(workspace isolation, aiki task IDs, `aiki task run` delegation). Fixed the
`docs/HARNESS.md` "distinct from" list to note CLAUDE.md/AGENTS.md are the same file.
