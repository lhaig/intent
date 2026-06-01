# Pickup Notes — 2026-06-01

Quick handoff after laptop restart. Resume from here.

## Where we are

Last shipped: **Phase 28 — testgen multi-param iteration** (commit `71f91b8`). The Rust testgen path's two retirement prerequisites are now both in place (17.A.1 entity emission, 17.A.2 multi-param). Branch `main`, working tree clean.

## Immediate next step (recommended)

**Phase 29 — Retire the legacy Rust testgen path (17.A.4).** Both prerequisites are in place. Scope:

1. Audit current callers of the Rust testgen path:
   - `cmd/intentc/main.go` `handleTestGen` — `target := "rust"` is still the legacy default.
   - `internal/compiler/test_runner.go` — verify it doesn't import `testgen` directly for Rust emission (it should be using IR → rustbe, not testgen).
   - `internal/testgen/rustutil.go` (~520 lines), `internal/testgen/testgen.go`, and the `prngHelpers` constant — these are the legacy-path code to remove.
   - `internal/testgen/testgen_test.go` — coverage of the legacy path; remove or migrate.
2. Decide whether to (a) make `--target intent` the only path with `--target rust` erroring out with a clear migration message, or (b) flip the default and keep `--target rust` as an opt-in pending deletion. Recommend (a) since both prerequisites are in place.
3. Update INTENT.md, docs/ROADMAP.md, `cmd/intentc/main.go` usage text.
4. ADR-light note (or revision of ADR 0036 / 0037) recording the retirement.

This is the natural close-out of Phase 17.A and the cleanest stopping point before pivoting to bigger work.

## Other candidates (in priority order from earlier conversation)

- **Verify-aware stripping** (`--strip-contracts=verified`). ADR 0033 explicitly deferred this. Needs an ADR to pick the verify-state persistence mechanism (Z3-at-build vs. `.verify.json` cache vs. `@verified` annotation). User decision required before implementation.
- **Linter rewrite in Intent** — HARNESS.md §7's named self-hosting stepping stone. Big work; will surface language gaps (dynamic dispatch, regex) that need their own ADRs.
- **Phase 23 — VS Code Marketplace publish.** Blocked on user-supplied publisher account, PAT, and branded icon. PRD is at `ops/plans/phase-23-marketplace-publish.md`.
- **Phase 17.G** — WASM test runner. Needs design (test-failure protocol from WASM back to runner).
- **Phase 17.H** — Coverage / snapshot testing. Real PRD; needs design.

## Open ADRs from this session (for context)

- **ADR 0033** (`--strip-contracts` flag) — accepted; revised in-place to drop `--release`.
- **ADR 0034** (per-contract verify source positions) — accepted; shipped as Phase 24.
- **ADR 0035** (LSP `textDocument/references`) — accepted; shipped as Phase 26.
- **ADR 0036** (entity/method test-gen emission) — accepted; shipped as Phase 27.
- **ADR 0037** (multi-param iteration test-gen) — accepted; shipped as Phase 28.

## Memory state

The agent memory now carries three durable items:

- `project_intent_is_a_new_language` — every cross-cutting decision should cite prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs are written as decisions land, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code before editing, run `make validate` after each task, surface uncertainty instead of guessing.

## How to resume

1. `git log --oneline -8` to see the recent landing.
2. `aiki task` to see open tasks (parent for the previous sequence was closed; no in-progress work).
3. Pick from the list above and either ask for a recommendation or just say "do Phase 29."
