# Pickup Notes — 2026-06-02

Quick handoff after Phase 29. Resume from here.

## Where we are

Last shipped: **Phase 29 — Retire legacy Rust testgen path** (17.A.4). The legacy `--target rust` testgen path is gone — `intentc test-gen` is `--target intent` only, and `--target rust` errors with a migration message. ~1.9k LOC deleted across `internal/testgen/{testgen.go,rustutil.go,values.go,testgen_test.go}` plus `compiler.GenerateTests` / `GenerateTestsProject`. ADR 0038 records the decision. Branch `main`, working tree clean.

Phase 17.A (testgen migration) is now fully closed:
- 17.A.1 — entity/method emission (Phase 27)
- 17.A.2 — multi-param iteration (Phase 28)
- 17.A.4 — legacy Rust path retired (Phase 29)

The toolchain now has **one** test-emission path: Intent source. Every generated test runs on rust + js + wasm via the normal IR → backend pipeline. This is the alignment we need for self-hosting.

## Strategic context — bootstrapping intent with itself

The user's near-term goal is to start bootstrapping Intent with itself. Phase 29 was a load-bearing prerequisite (no parallel Rust-shaped toolchain branch to maintain). The remaining blockers, in roughly priority order:

1. **Package registry** (parked as task `znpxulo`, p1). Required for any non-trivial self-hosted module that wants to depend on libraries. Both Phase 13 TODOs (`internal/compiler/semver.go:275`, `cmd/intentc/main.go:811`) are stubs waiting for this. **No ADR yet — this is the next design decision.** See "Package registry options" below.
2. **Linter rewrite in Intent.** HARNESS.md §7's named self-hosting stepping stone. Will surface language gaps (dynamic dispatch, regex). Doable without a registry if the linter has zero external dependencies.
3. **Formatter rewrite in Intent.** Similar shape to the linter; smaller surface; might be a better first stepping stone.
4. **Compiler rewrite in Intent.** The big one. Likely requires the package registry to be useful (split lexer/parser/checker/IR/backends across packages).

## Package registry options (next design decision)

Three credible hosting models, none currently implemented:

| Model | How it works | Prior art | Pros | Cons |
|---|---|---|---|---|
| **Git-based (Go-style)** | Dependencies are `name = "github.com/user/repo"` + version; resolver clones & checks out tags. No central server. | Go modules, Nix flakes (partial) | Zero infra; existing GitHub auth; reproducible via lockfile. | Slower first install; rate limits; private deps need credentials. |
| **HTTP-API central registry (crates.io-style)** | Single registry hosts tarballs + metadata; signed publishes; namespace authority. | crates.io, npm, PyPI, Hex.pm | Fast resolution; signing model is well-trodden; trust roots. | Operating infra; spam/squatting policies; bus-factor on the registry. |
| **Hybrid (Deno-style)** | URL imports (any HTTPS) + an optional curated index. | Deno (jsr / deno.land/x), Nix overlays | No central authority; flexible. | Cache invalidation; harder to enforce signing. |

For a personal project at v1.x, **git-based is the obvious starting point** — zero infra, leverages existing source-control identity, and Go's experience shows it scales. A central index can be added later without invalidating git-based deps.

Open questions for the ADR:
- **Module identity:** `name + version` (crates.io) vs. `import URL` (Go/Deno)?
- **Version resolution:** SemVer (`^1.2`) vs. MVS (minimum-version-selection) vs. range solver?
- **Lockfile format:** TOML alongside `intent.toml`?
- **Cache layout:** `~/.intent/cache/<host>/<owner>/<repo>@<version>/`?
- **Security:** Checksum-required, signing-optional? Or signing-required at the registry layer?
- **Offline workflow:** Vendor directory (`intent vendor`) for fully-offline builds?

Recommend: write ADR 0039 — Package Registry Model. Pick git-based, define the manifest schema (`intent.toml [dependencies]` already exists from Phase 13), define the lockfile, define the cache layout, leave signing as a v2 problem. That ADR unblocks task `znpxulo`.

## Other candidates (priority order, less critical for bootstrapping)

- **Verify-aware stripping** (`--strip-contracts=verified`) — deferred by ADR 0033. Needs an ADR on verify-state persistence (Z3-at-build vs `.verify.json` vs `@verified` annotation). Useful independently of self-hosting.
- **Phase 17.G — WASM test runner.** Needs design (test-failure protocol from WASM back to runner). Not on the self-hosting critical path.
- **Phase 17.H — Coverage / snapshot testing.** Real PRD; needs design. Not on the self-hosting critical path.
- **Phase 23 — VS Code Marketplace publish.** Blocked on user-supplied publisher account, PAT, and branded icon. PRD: `ops/plans/phase-23-marketplace-publish.md`.

## Open ADRs from this session (for context)

- **ADR 0033** (`--strip-contracts` flag) — accepted; revised in-place to drop `--release`.
- **ADR 0034** (per-contract verify source positions) — accepted; shipped as Phase 24.
- **ADR 0035** (LSP `textDocument/references`) — accepted; shipped as Phase 26.
- **ADR 0036** (entity/method test-gen emission) — accepted; shipped as Phase 27.
- **ADR 0037** (multi-param iteration test-gen) — accepted; shipped as Phase 28.
- **ADR 0038** (retire legacy Rust testgen path) — accepted; shipped as Phase 29.

## Memory state

Three durable items still hold:
- `project_intent_is_a_new_language` — every cross-cutting decision should cite prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs are written as decisions land, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code before editing, run `make validate` after each task, surface uncertainty instead of guessing.

## How to resume

1. `git log --oneline -8` to see the Phase 29 landing.
2. `aiki task` to see open tasks (parent for Phase 29 should be closed by now; `znpxulo` is the parked package-registry task at p1).
3. The single biggest unblock for self-hosting is the package registry. Recommend starting `znpxulo` and writing ADR 0039 — the design space is small enough to nail down in one session.
