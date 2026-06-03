# Pickup Notes — 2026-06-03 (evening)

Handoff after the Phase 30 ship + the self-hosting kickoff (ADRs 0040, 0041 + Phase 31 PRD + selfhost scaffolding). Resume from here.

## Where we are

Shipped today: Phase 30 (package registry — git + MVS, commit `9898b53`, pushed to `origin/main`). Then sketched the multi-phase self-hosting plan as design artefacts:

- **ADR 0040** — Self-hosted formatter strategy (stage1/stage2 mental model from Zig; Intent-parses-Intent direction; full byte-parity target; multi-phase, gap-driven).
- **ADR 0041** — String indexing + `Char` type (the first language gap that blocks writing a lexer in Intent).
- **Phase 31 PRD** — `ops/plans/phase-31-string-primitives.md` (9 tasks; ~1.5-2k LOC; covers lexer / parser / checker / IR / 3 backends / Z3 verify / docs / feature-coverage example).
- **selfhost/** tree scaffolded (READMEs + empty `intent.toml`); no Intent code yet.

Branch `main`, working tree about to commit the docs. Nothing has changed in stage1 code yet — all design.

## Immediate next step

**Implement Phase 31** per `ops/plans/phase-31-string-primitives.md`. This is the prerequisite for *any* stage2 Intent code. Until it lands, the lexer can't be written; until the lexer lands, nothing else in the self-hosting plan can move.

Phase 31 surface (recap):
1. Lexer: `'a'` char-literal token + escape forms.
2. AST + parser: `CharLit` node + slice via `RangeExpr` inside `IndexExpr`.
3. Checker: `Char` primitive + relaxed `IndexExpr`/`len()` for String + char built-ins.
4. IR + Rust backend (precomputed `Vec<char>`).
5. IR + JS backend (`Array.from(s)`).
6. IR + WASM backend (per-string offset table).
7. Z3 verifier integration.
8. Feature-coverage example + differential `--all-targets` test.
9. Docs (INTENT.md, DESIGN.md, grammar.ebnf).

Acceptance: `make validate` green; `intentc test --all-targets examples/char_string_demo.intent` green; a verify test on a contract using char predicates passes.

## After Phase 31

Phase 32 — write the lexer in Intent (`selfhost/formatter/lexer.intent`). The first stage2 code. Phase 33+ are parser layers. See ADR 0040 §"Delivery phases" for the indicative sequence and `selfhost/formatter/README.md` for the per-phase status table.

## Open ADRs / threads from this session

- **ADR 0040** (strategy) — accepted; the meta-ADR for the self-hosting milestone.
- **ADR 0041** (Char type + string indexing) — accepted; Phase 31 implementation.
- **Open follow-ups** (per ADR 0040 §"Open questions"): char type details (resolved by 0041), sum types beyond enum (TBD when parser phase needs them), dynamic dispatch for visitor pattern (TBD), I/O streaming for stage2 binaries (TBD), stage1 retirement criteria (TBD, post-parity).

## Other candidates (orthogonal, not on the self-hosting critical path)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **Phase 17.G — WASM test runner.**
- **Phase 17.H — Coverage / snapshot testing.**
- **Phase 23 — VS Code Marketplace publish.** Blocked on user-supplied publisher account, PAT, branded icon.
- **ADR 004x — Signing for the package registry** (ADR 0039 §9 deferred).

## Memory state

Four durable items hold:
- `project_intent_is_a_new_language` — every cross-cutting decision should cite prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs are written as decisions land, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code before editing, run `make validate` after each task, surface uncertainty instead of guessing.
- `project_self_hosting_priority` — bootstrapping Intent with itself is the near-term goal; package registry was the major unblock (now done); first stage2 target is the formatter.

## How to resume

1. `git log --oneline -8` to see the recent landings (Phase 30 + the design pass).
2. `aiki task` for the open task list.
3. Recommended start: open `ops/plans/phase-31-string-primitives.md`, pick task 31.1 (lexer char-literal token), and begin. Each task in the PRD is sized to be a single focused commit.
