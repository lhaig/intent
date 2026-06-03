# Pickup Notes — 2026-06-03

Quick handoff after Phase 30. Resume from here.

## Where we are

Last shipped: **Phase 30 — Package registry (git-based + MVS)** per ADR 0039. The two Phase-13 registry TODOs are gone; `intentc pkg install` now resolves git-sourced dependencies via MVS, fetches them into `~/.intent/cache/git/<host>/<owner>/<repo>@<rev>/`, and writes a committed `intent.lock` with sha256 tree-hash checksums. `intentc pkg upgrade` and `intentc pkg vendor` are also live. Branch `main`, working tree clean.

Combined Phase 29 + 30: the toolchain now has (a) one test-emission path that runs on every backend and (b) a working dependency-management story. Self-hosted tooling can declare its own packages and depend on them.

## Immediate next step (recommended)

**First self-hosted tool — the formatter.** Per HARNESS.md §7 and `[[project-self-hosting-priority]]` memory, this is the smallest stepping stone before tackling the linter or compiler. Rewriting the formatter in Intent will surface real language gaps (string manipulation, file I/O, possibly some dynamic dispatch) that need their own ADRs. Recommended approach:

1. Create `selfhost/formatter/` as a new Intent package.
2. Write the formatter in Intent against the existing Go-side AST shape (or define a JSON intermediate).
3. Wire it as an opt-in backend (`intentc fmt --self-hosted`) so the Go formatter stays as a fallback during the transition.
4. Per language-gap surfaced, write a focused ADR + phase.

Expected language gaps to bump into (worth ADRs in advance if obvious):
- String slicing and case-conversion ergonomics
- File I/O (currently exists via FFI on rust target; needs portable Intent-native form for self-hosting)
- Possibly union types or sum-type pattern-matching in places the formatter switches on AST node kind

## Other candidates (priority order)

- **Linter rewrite in Intent.** Same template as the formatter; similar surface area; can land in parallel.
- **Verify-aware stripping** (`--strip-contracts=verified`) — deferred by ADR 0033. Needs an ADR on verify-state persistence (Z3-at-build vs `.verify.json` vs `@verified` annotation). Useful but not on the self-hosting critical path.
- **Phase 17.G — WASM test runner.** Test-failure protocol from WASM back to runner. Off the critical path.
- **Phase 17.H — Coverage / snapshot testing.** Needs a PRD. Off the critical path.
- **Phase 23 — VS Code Marketplace publish.** Blocked on user-supplied publisher account, PAT, and branded icon. PRD: `ops/plans/phase-23-marketplace-publish.md`.

## Open follow-ups from Phase 30

- **ADR 004x — Signing.** ADR 0039 §9 deferred publisher signing. v1 lockfile schema reserves a `signature` field via `version = 1`; v2 lockfiles will populate it. Sigstore is the leading candidate. Not blocking until there are users to protect.
- **Optional central index.** A simple GitHub repo with a `packages.toml` listing well-known packages → git URLs would let users write `foo = "1.2.3"` without the full `git = "..."` URL. Layerable on top of ADR 0039 without changes. Defer until there's demand.

## Open ADRs from recent sessions (for context)

- **ADR 0033** (`--strip-contracts` flag) — accepted; revised in-place to drop `--release`.
- **ADR 0034** (per-contract verify source positions) — accepted; shipped as Phase 24.
- **ADR 0035** (LSP `textDocument/references`) — accepted; shipped as Phase 26.
- **ADR 0036** (entity/method test-gen emission) — accepted; shipped as Phase 27.
- **ADR 0037** (multi-param iteration test-gen) — accepted; shipped as Phase 28.
- **ADR 0038** (retire legacy Rust testgen path) — accepted; shipped as Phase 29.
- **ADR 0039** (package registry — git + MVS) — accepted; shipped as Phase 30.

## Memory state

Four durable items now hold:
- `project_intent_is_a_new_language` — every cross-cutting decision should cite prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs are written as decisions land, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code before editing, run `make validate` after each task, surface uncertainty instead of guessing.
- `project_self_hosting_priority` — bootstrapping Intent with itself is the near-term goal; package registry was the major unblock (now done).

## How to resume

1. `git log --oneline -8` to see the Phase 30 landing.
2. `aiki task` to see open tasks (Phase 30 parent + 8 subtasks all closed).
3. Recommended start: pick the formatter as the first self-hosted-tool candidate; sketch an ADR-light proposal for the directory layout and language-gap survey.
