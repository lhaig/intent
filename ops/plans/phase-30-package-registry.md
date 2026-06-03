# Phase 30: Package Registry — Git-Based + MVS

**Status:** Planning (ADR 0039 accepted 2026-06-03)
**Milestone:** v1.2 — Self-Improvement Foundations
**Decision:** [ADR 0039](../../docs/decisions/0039-package-registry-git-mvs.md)
**Revises:** [ADR 0027](../../docs/decisions/0027-package-management-design.md) §"Version Resolution"

## Goal

Deliver a working git-based package registry with MVS resolution, a committed `intent.lock`, content-addressed cache, and an offline `pkg vendor` flow. Unblock Phase 13's two outstanding TODOs (`semver.go:275`, `main.go:942`) and make multi-package self-hosted tooling possible (linter, formatter, eventually compiler).

## Success Criteria

- [ ] `intent.toml [dependencies]` accepts `foo = { git = "github.com/lhaig/foo", version = "1.2.3" }` and resolves it to a cloned, checksummed package at `~/.intent/cache/git/github.com/lhaig/foo@<rev>/`.
- [ ] MVS resolver replaces `ConstraintBaseVersion`: given a graph of `intent.toml` files, produces a deterministic build list with `max(minimum_version)` per package and the highest patch tag at that major+minor.
- [ ] `intent.lock` is written by `intentc pkg install`, committed to source control, format-versioned (`version = 1`), and verified on every `intentc build`.
- [ ] Sha256 tree-hash checksum (sorted-path, length-prefixed) is computed for each cached package and recorded in the lockfile. Mismatch on load is a hard error.
- [ ] `intentc pkg vendor` copies the resolved set to `./vendor/`; subsequent `intentc build` reads from `./vendor/` when present.
- [ ] `intentc pkg upgrade <name>` updates the minimum in `intent.toml` and re-resolves; `--major` opens cross-major upgrades; bare `intentc pkg install` never crosses majors.
- [ ] Private repos work via the system git toolchain credentials (SSH, system credential helper, `gh auth`); `intentc` never prompts for or stores credentials.
- [ ] Legacy bare-version form (`foo = "1.0.0"`) keeps parsing but emits a deprecation warning pointing at `git = ...` or `path = ...`.
- [ ] `^` / `~` constraints accepted by parser but interpreted as `>=` minima; warning emitted suggesting plain minimum-version syntax.
- [ ] `internal/compiler/semver.go:275` and `cmd/intentc/main.go:942` TODOs removed.
- [ ] `make validate` green; all existing `examples/packages/*` continue to build.
- [ ] ADR 0027 status line updated to "revised by ADR 0039."

## Reference

- [ADR 0039](../../docs/decisions/0039-package-registry-git-mvs.md) — design
- [ADR 0027](../../docs/decisions/0027-package-management-design.md) — Phase 13 baseline (manifest, cache layout, constraint parser being retired)
- Phase 13 implementation: `internal/compiler/manifest.go`, `semver.go`, `cache.go`, `cmd/intentc/main.go` `handlePkg*`
- Existing manifests under `examples/packages/`, `examples/attractor/`, `examples/ffi_blake3/`
- Russ Cox, "Minimal Version Selection" (research.swtch.com/vgo-mvs)

## Tasks

### 30.1 Manifest schema extension

**Files:** `internal/compiler/manifest.go`

- Add `Git string` to `DependencySpec`; mutually exclusive with `Path` (extend `Validate()`).
- `parseDependencyValue`: recognize `git = "..."` in inline tables; produce a `DependencySpec{Git: ..., Version: ...}`.
- Soft-deprecation warning on bare `name = "1.2.3"` form (emit via diagnostics, not stderr) — point at `git = ...` or `path = ...`.
- Soft-deprecation warning on `^1.2.3` / `~1.2.3` constraints — point at minimum-version semantics.

**Acceptance:** New unit tests cover (a) inline table with `git + version`, (b) `git` + `path` rejected, (c) `git` without `version` rejected, (d) bare version still parses but raises a warning.

### 30.2 Fetcher

**Files:** `internal/compiler/fetcher.go` (new)

- `Fetcher` wraps `git` invocations. Methods: `ListTags(url) ([]Tag, error)`, `Clone(url, rev, dest) error`.
- `ListTags` uses `git ls-remote --tags <url>`, parses `refs/tags/vMAJOR.MINOR.PATCH`, drops non-semver tags, returns sorted descending.
- `Clone` shallow-clones to a temp dir, `git checkout <rev>`, moves to final cache path atomically.
- `TreeHash(dir) ([]byte, error)`: walks the directory in NFC-normalised sorted-path order, hashing `len(path) || path || len(content) || content` per file; excludes `.git/`, `.DS_Store`, build artefacts.
- All operations honour user's existing git credentials — no special auth handling.

**Acceptance:** Unit tests with a local bare git repo fixture (no network): `ListTags` enumerates correctly, `Clone` populates a directory at the right rev, `TreeHash` is stable across identical trees, differs on any byte change.

### 30.3 MVS resolver

**Files:** `internal/compiler/resolver.go` (new); `internal/compiler/semver.go` (retire `ConstraintBaseVersion`).

- `Resolve(manifest *Manifest, fetcher Fetcher) (*ResolvedSet, error)`:
  1. Seed work list with direct deps.
  2. For each `(name, source, min_version)`: locate or fetch the package's `intent.toml`; record transitive deps.
  3. Walk the graph; for each `name` track the max minimum-version seen across all paths.
  4. Cross-major conflict → hard error with the conflicting parents listed.
  5. For each resolved `(name, min_version)`: enumerate tags via `Fetcher.ListTags`; pick highest `vMAJOR.MINOR.PATCH` with `MAJOR.MINOR ≥ min_version.MAJOR.MINOR` and `MAJOR == min_version.MAJOR`.
- `ResolvedSet` is the input to lockfile writing.
- Retire `ConstraintBaseVersion` — no callers remain after Phase 30.

**Acceptance:** Table-driven tests on a synthetic dep graph: simple chain, diamond (different majors → error; same major → max minimum), version-bumping transitive ask, missing tag (no compatible release → error with available tags listed).

### 30.4 Lockfile

**Files:** `internal/compiler/lockfile.go` (new)

- `Lockfile` struct mirrors the schema in [ADR 0039 §4](../../docs/decisions/0039-package-registry-git-mvs.md#4-lockfile--intentlock): `Version int`, `Generated time.Time`, `Packages []LockedPackage`.
- `ReadLockfile(path) (*Lockfile, error)` — strict; refuses unknown version numbers.
- `WriteLockfile(path, *Lockfile) error` — deterministic output (sorted packages by name; stable timestamp format).
- `Verify(lock *Lockfile, cache Cache) error` — re-hashes each cached package; mismatch → error naming the package + `intentc pkg install --refresh` hint.

**Acceptance:** Unit tests round-trip a lockfile through Write/Read, verify deterministic ordering, reject `version = 2`, and catch a tampered checksum.

### 30.5 Cache layout extension

**Files:** `internal/compiler/cache.go`

- Add `GitPath(host, owner, repo, rev) string` returning `~/.intent/cache/git/<host>/<owner>/<repo>@<rev>/`.
- Add `StoreGit(host, owner, repo, rev, srcDir, checksum)` — moves a fetched tree into place atomically, writes checksum side file.
- Keep legacy `~/.intent/cache/<name>/<version>/` path resolving for back-compat with bare-version manifests.
- Add `Refresh()` that wipes git-source entries (used by `pkg install --refresh`).

**Acceptance:** Unit test populates a fake git cache entry, verifies path layout matches ADR 0039 §5, verifies `Refresh()` removes only git-source entries.

### 30.6 CLI rewire

**Files:** `cmd/intentc/main.go`

- `handlePkgInstall`: replace the "no registry available" warning path with: read `intent.lock` (if present, verify and short-circuit); else run resolver, write lockfile, populate cache, print resolved set.
- Add `handlePkgUpgrade`: bump the minimum in `intent.toml` for the named dep; `--major` opens cross-major; re-resolve.
- Add `handlePkgVendor`: copy resolved set from cache into `./vendor/`; print summary.
- Add `handlePkgLock`: re-resolve and write lockfile without populating cache (CI / lockfile-refresh use case).
- Remove the `cmd/intentc/main.go:942` "no registry available" branch.
- Help text: document new subcommands; flag deprecation warning for bare-version syntax.

**Acceptance:** Smoke test via a real git repo fixture (small test package): `intentc pkg install` resolves and caches; subsequent run is a no-op against lockfile; `intentc pkg vendor` populates `./vendor/`.

### 30.7 Migration + examples

**Files:** `examples/packages/*/intent.toml`, `examples/attractor/intent.toml`, `examples/ffi_blake3/intent.toml`, `Makefile`

- Existing `examples/packages/*` use `path` deps — no change required.
- Add a new `examples/packages/git_dep_demo/` showing `git = "..."` syntax (uses a known stable mirror or a same-repo subdirectory to avoid external dependency in tests).
- Add a `Makefile` target `test-pkg-install: build` that runs `intentc pkg install` against the demo.

**Acceptance:** New demo type-checks and resolves; `make test-pkg-install` passes in CI.

### 30.8 Docs + ADR 0027 update

**Files:** `docs/decisions/0027-package-management-design.md`, `docs/ROADMAP.md`, `README.md`, `INTENT.md`, `ops/NEXT-STEPS.md`

- ADR 0027 status line — already updated by this ADR commit; verify final wording.
- ROADMAP — add Phase 30 entry above Phase 29.
- README — document `git = ...` source and `pkg vendor`.
- INTENT.md — mention `intent.lock` (it should be committed).
- NEXT-STEPS — refresh after Phase 30 ships.

**Acceptance:** All four docs reflect Phase 30 reality; no dangling references to "registry deferred."

## Validation

- `make validate` green at every commit.
- `make test-pkg-install` green (new target).
- Smoke test on a real `examples/packages/git_dep_demo/` resolving against a live git source.
- ADR 0039 link-checks in the index page.

## Out of scope

- Publisher signing (sigstore / gpg / minisign) — deferred to ADR 004x.
- Central index of well-known packages — layerable on top of this design later.
- Workspace / monorepo manifests (multi-`intent.toml`) — separate ADR if needed.
- `intentc pkg publish` — there's no central registry to publish to in this design; users tag their git repo, which is the publish step.

## Estimated size

~1.5k LOC across 5 new files (`fetcher.go`, `resolver.go`, `lockfile.go`, plus tests), modest changes to 3 existing files (`manifest.go`, `cache.go`, `main.go`). Comparable in surface area to Phase 13.
