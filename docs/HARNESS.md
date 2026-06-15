# Intent Project Harness — Agent Self-Improve Guide

This document is the entry point for an AI agent (or returning human) who wants to push Intent forward without explicit human instruction. It captures *how this repository expects work to happen*: where decisions live, what counts as "done," and which loops are mechanical truth.

This is distinct from:
- `INTENT.md` — how to **write** programs in Intent.
- `CLAUDE.md` / `AGENTS.md` (same file via symlink) — project conventions and how task tracking works: the **norman** skill, with state in `prds/` (see `prds/config.md`). Replaced the earlier aiki tooling.
- `~/.claude/CLAUDE.md` — global Claude Code conventions (commits, code style).

If you are an agent reading this for the first time: skim section 1, then keep this open as a reference. The mechanical-validation commands in section 3 are non-negotiable.

---

## 1. The Self-Improve Loop

This is how Intent advances:

```
Observe gap or candidate work
        ↓
Decision worth recording?  ─yes→  Write ADR  ─→  Add to docs/decisions/README.md index
        │                                                    │
        no                                                   │
        ↓                                                    ↓
Feature/fix scoped > 1 file?  ─yes→  Write PRD in prds/ (research→backlog), add row to prds/TASKS.md
        │                                    │
        no                                   ↓
        ↓                          Execute PRD task-by-task (PRD moves backlog→active)
Make change directly                         │
        ↓                                    ↓
Run validation (section 3)  ←──────────────  Run validation after each task
        ↓
Commit (conventional commit, no Claude co-author)
        ↓
Flip TASKS.md row to DONE; move PRD active→done; add Status block when complete
```

Critical rule: **iterate the harness, not the prompt or the agent**. When an agent makes the same mistake twice, the answer is a lint rule, an ADR clarification, or a doc update — not "try a different model."

---

## 2. Where to Find Work

In order of priority:

1. **`prds/TASKS.md`** — live task list. The active phase is expanded into steps; anything `TODO`/`ACTIVE` is candidate work.
2. **`docs/ROADMAP.md`** — current milestone view (v1.0 shipped, v1.1 shipped, Milestone 7 shipped, v1.2 in progress).
3. **`prds/active/` and `prds/backlog/`** — open PRDs. Active is in-flight; backlog is scoped and ready.
4. **`prds/NEXT-STEPS.md`** — handoff notes for resuming the current work stream.
5. **`docs/decisions/`** — ADRs with `Status: accepted` that have deferred items (e.g. ADR 0027's remote registry fetch).
6. **Mechanical signals**: lint warnings on `examples/`, failing examples, TODO comments, gaps surfaced by running validation.

If multiple options exist and you can't decide, prefer:
- Foundational over feature (testing > new builtin)
- Already-scoped over net-new (Draft PRD > greenfield)
- Closes a deferred decision over opens a new one

---

## 3. Validation Harness (Mechanical Truth)

These are the commands that determine whether a change is actually done. Do not claim a task complete without running them and observing pass.

| Command | When to run |
|---|---|
| `make build` | After any change to Go code under `cmd/`, `internal/` |
| `make test` | After any change touching the compiler pipeline |
| `make check-examples` | After parser, checker, or IR changes |
| `make lint-examples` | After linter changes or new lint rule |
| `make validate` | Before committing any non-trivial change. Includes `gofmt-check` matching the CI Format Check job exactly. |
| `make gofmt-check` | Standalone gofmt verification — pre-flight before `git push` if you skipped `make validate`. |
| `./intentc check <file.intent>` | When debugging a specific example |
| `./intentc verify <file.intent>` | When changes might affect Z3 contract verification |
| `./intentc fmt --check <file.intent>` | After grammar or formatter changes |
| `./intentc lint <file.intent>` | When verifying lint rule output |
| `./intentc test <file.intent>` (Phase 16+) | After in-language testing lands |
| `go test ./internal/<pkg>/... -v` | While developing a single package |
| `gosec ./...` (when installed) | Before commits that touch security-sensitive code |

Cross-backend changes (any IR or backend modification) must run `make check-examples` AND emit at least one example per target (`./intentc build --target js --emit ...`, same for `wasm`).

If a command is unavailable on your machine (e.g. no `cargo`), state that explicitly when reporting. Do not claim success based on what should work.

---

## 4. When to Write an ADR

Write one when:
- Choosing between two viable designs and committing to one (the alternatives matter to future readers).
- Changing a previously-decided semantic (then mark the new ADR as revising the old; example: Phase 14 revised ADR 0026).
- Adding a new top-level language construct.
- Establishing a project convention you want enforced.

Don't write one for: bug fixes that don't change semantics, small refactors, formatting changes, dependency upgrades, doc edits.

**ADR template (matches `docs/decisions/README.md`):**

```markdown
# NNNN: Title

**Date:** YYYY-MM-DD
**Status:** accepted | superseded by NNNN | deprecated
**Phase:** which milestone/phase prompted this

## Context
What situation are we in?

## Options
What did we consider?

## Decision
What did we choose and why?

## Consequences
Trade-offs accepted.
```

After writing: add a row to the index table in `docs/decisions/README.md`. The index is the discovery surface for every other agent and human.

---

## 5. When to Write a PRD

Write one when:
- The work spans more than one file or commit.
- Success can be expressed as a checklist of verifiable criteria.
- The work could be paused and resumed by someone else (or future-you in a different session).

Don't write one for: typo fixes, single-line bug fixes, doc nudges, dependency upgrades, formatter runs.

**PRD template (match `prds/done/phase-15-rust-ffi.md`):**

```markdown
# Phase N: Title

**Status:** Draft | In Progress | Shipped (YYYY-MM-DD)
**Milestone:** which milestone this belongs to
**Decision:** [ADR link] (if applicable)
**Deferred:** explicit items deferred from this phase (if any)

## Goal
One paragraph: what this phase delivers and why.

## Success Criteria
- [ ] verifiable criterion 1
- [ ] verifiable criterion 2
- [ ] No regressions in existing tests

## Reference
- ADR link
- Files that will change

## Tasks
### N.1 Subsystem name
**Files:** ...
Description of work.
**Acceptance:** specific test commands that prove this task done.

### N.2 ...

## Out of Scope
Things explicitly deferred or rejected.
```

After writing: the PRD is the source of truth for that work, tracked by a row in `prds/TASKS.md`. Move it `prds/backlog/` → `prds/active/` when you start and `prds/active/` → `prds/done/` when it ships. Update the checklist as you go. When complete, change `Status:` to `Shipped (date)`, flip the TASKS.md row to `DONE`, and update `docs/ROADMAP.md`.

---

## 6. Anti-Patterns

These come from both the project `CLAUDE.md` and from harness-engineering experience (OpenAI Frontier team, summarised below). They apply universally.

**Don't:**
- Claim a task is done because tests "should" pass — **run them**.
- Add backwards-compatibility shims, feature flags, or `// removed in NN` comments. Just change the code.
- Mock at the boundary you're testing. Run real Z3, real cargo, real node when feasible.
- Iterate the same prompt expecting different output. If an approach fails twice, change the approach or change the harness.
- Expand scope mid-PRD. STOP and add a follow-up PRD instead.
- Trust agent self-reports (including your own) without mechanical verification.
- Write multi-paragraph code comments. Code should be self-documenting; comments are for non-obvious WHY.
- Use `--no-verify`, `--no-gpg-sign`, or any hook-bypass without explicit permission.
- Skip or delete failing tests. Fix the underlying issue.
- Commit secrets, even in fixtures.

**Do:**
- Make decisions explicit (ADR), make criteria explicit (PRD), make completion mechanical (validation).
- Treat code as disposable, harness as durable. If agents repeatedly write bad patterns, add a lint rule.
- Run validation after each task within a PRD, not just at the end.
- Document deferred items explicitly. "Out of Scope" sections prevent scope creep on the next pass.

---

## 7. The Self-Hosting North Star

Intent is on a long road toward writing `intentc` in Intent. Today it's written in Go.

When you find a language gap that blocks an Intent subsystem from being written in Intent — missing dynamic dispatch, weak string performance (ADR 0011 calls this out), absence of a real module system for the compiler's own internals, missing regex — **file it**:

- If the gap is well-understood: write a PRD for the feature.
- If the gap requires design: write an ADR with options.
- If the gap is obvious-but-blocked-on-other-work: add it to the v1.2/v1.3 backlog in `docs/ROADMAP.md`.

Do not attempt self-hosting wholesale before the foundations are in place. The realistic stepping stones are: in-language testing (Phase 16), then `--release` flag for stripping unverified contracts, then LSP, then one small subsystem (the linter is the best candidate) rewritten in Intent as a proof point.

---

## 8. Reference: Harness Engineering Influences

The thinking in this document is influenced by OpenAI's "Harness Engineering" — Ryan Lopopolo and the Frontier team. The relevant patterns adopted here:

- **Decisions in repo as primary discovery surface** → ADRs.
- **Mechanical guardrails over prose specs** → `make validate`, lints, `intentc verify`, in-language tests (Phase 16).
- **Iterate the harness, not the prompt** → reflected in section 1 and the anti-patterns.
- **Agents should be able to run their own validation** → section 3.
- **Code disposable, harness durable** → PRD/ADR discipline; Phase 16 makes this explicit by making contracts and tests the source of truth, not the implementation.

Patterns *not* adopted, deliberately:
- One-shot regeneration of the whole project. Intent's contracts are already mechanically checkable in a way that markdown specs aren't; we don't need to throw the implementation away and rebuild from English each iteration.
- Background "AI slop" cleanup jobs. Intent is small enough that human review at PR time suffices.
- Daily standups with agents. Not applicable to a single-maintainer project; revisit when team grows.
