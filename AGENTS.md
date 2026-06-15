# AGENTS.md — intent/

(`CLAUDE.md` is a symlink to this file, so all agents read the same guidance.)

Task tracking and execution for this project use the **norman** skill, backed by
`prds/`. This replaced the earlier aiki tooling, which has been removed — do NOT
run `aiki ...` commands.

## Working with norman

- State lives in `prds/`: `TASKS.md` (live task list), `TASKS-archive.md`
  (completed-phase index), `config.md`, `progress.md`, and PRDs under
  `done/`, `active/`, `backlog/`, `research/`.
- Resuming work? Read `prds/NEXT-STEPS.md` first, then `prds/TASKS.md`.
- Drive execution with the norman skill: `norman` / `continue norman` to work the
  next task, `norman plan` / `norman prd` to scope new work, `norman verify` to
  validate against a PRD. See the skill for the full mode list.
- `prds/progress.md` is the append-only crash-recovery log; add an entry as work
  lands.

## Project conventions

- Scoped work (>1 file, or pausable/resumable) gets a PRD in `prds/` and a row in
  `prds/TASKS.md`; move the PRD `backlog/` → `active/` → `done/` as it progresses.
- Decisions worth recording go in `docs/decisions/` as ADRs.
- Run the validation harness before claiming done: `make build`, `make test`,
  `make validate` (full table in `docs/HARNESS.md` §3). Stage2 formatter work also
  runs `./intentc test --all-targets selfhost/formatter/format_test.intent`.
- Conventional commits, no Claude co-author, no emojis (per the global CLAUDE.md).

See `docs/HARNESS.md` for the full agent harness contract, `docs/ROADMAP.md` for
the milestone view, and `INTENT.md` for how to write Intent programs.
