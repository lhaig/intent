# 0039: Package Registry — Git-Based Resolution with MVS

**Date:** 2026-06-03
**Status:** accepted; revises [ADR 0027](0027-package-management-design.md)
**Phase:** v1.2 — Self-Improvement Foundations (next unblock for self-hosting)

## Context

[ADR 0027](0027-package-management-design.md) (Phase 13, 2026-03-20) shipped the manifest, local-path dependencies, semver constraint matching (`=`, `^`, `~`, `>=`, `<`), and a cache at `~/.intent/cache/<name>/<version>/`. Two stubs remained, both blocked on the same missing piece — a *remote-fetch* registry path:

- `internal/compiler/semver.go:275` — `ConstraintBaseVersion` returns the constraint's base version (e.g. `^1.0.0` → `1.0.0`) because no registry exists to resolve it to the actual latest compatible version.
- `cmd/intentc/main.go:942` — `intentc pkg install` warns "no registry available to fetch this version" and writes nothing.

The user's near-term goal is to bootstrap Intent with itself ([[project-self-hosting-priority]]). Self-hosted tooling (linter, formatter, eventually the compiler) will be split across packages and require versioned dependencies. The current Phase-13 implementation cannot fetch a remote dep — so multi-package self-hosting is blocked here.

This ADR commits to a v1 registry model. The decision was made interactively (2026-06-03) across three axes; the choices and the prior art are recorded below.

### Decision axes

| Axis | Options considered | v1 choice |
|---|---|---|
| **Hosting model** | git-based (Go modules); central HTTP registry (crates.io); hybrid URL imports (Deno) | **git-based** |
| **Version resolution** | MVS (Go); SemVer ranges + SAT solver (Cargo/npm); exact pins only | **MVS** |
| **Scope of v1 ADR** | Manifest + lockfile; cache layout + vendor; signing + checksum policy; private/auth flow | **All four** |

### Precedent

| System | Hosting model | Resolution | Lockfile | Notable |
|---|---|---|---|---|
| Go modules (2018→) | git-based; module path *is* import URL | MVS (Cox 2016) | `go.sum` is checksums only; `go.mod` records resolved versions | The chosen precedent. `GOPROXY` adds optional caching/auth in front. Pseudo-versions handle untagged commits. |
| Cargo / crates.io | central registry (tarballs) | SemVer ranges + backtracking solver | `Cargo.lock` records full resolution | The dominant "modern" packaging UX; trades operational complexity for fast resolution. |
| npm / yarn / pnpm | central registry | SemVer ranges + solver | `package-lock.json` / `yarn.lock` / `pnpm-lock.yaml` | npm pre-2020 had non-deterministic installs even with lockfiles; pnpm fixed this. Resolver complexity is a known cost. |
| Bundler / RubyGems | central registry | SemVer ranges + Molinillo solver | `Gemfile.lock` records full resolution | Cleanest lockfile format among the surveyed systems; copying its layout. |
| Hex.pm (Elixir) | central registry | SemVer ranges + solver | `mix.lock` | Erlang VM ergonomics; not directly comparable. |
| Deno | URL imports (any HTTPS) + optional curated index (`jsr`) | URL-pinned with optional version specifier | `deno.lock` records checksums | The "no central authority" reference design; cache invalidation pain is well documented. |
| Nix flakes | content-addressed; multiple input fetchers (git, github, tarball) | lockfile-driven | `flake.lock` | Most rigorous reproducibility model; substantial implementation overhead. |
| Dart pub / pubspec | central registry; git fallback | SemVer ranges + solver | `pubspec.lock` | Demonstrates that "central + git fallback" can coexist. |

The closest fit is **Go modules**: zero infrastructure for the v1 maintainer, leverages GitHub identity, well-documented resolution algorithm with proven multi-year track record, and a small (auditable) lockfile format.

### Why MVS over SemVer ranges

Russ Cox's MVS proposal (research.swtch.com/vgo-mvs, 2016) reframed dependency resolution around three properties Cargo / npm sacrificed for caret-range convenience:

1. **Deterministic without a lockfile.** Given a fixed set of `go.mod` files, MVS produces exactly one build list. No version drift across developers running `install` minutes apart.
2. **No backtracking solver.** Linear-time graph walk; trivially auditable; no edge cases (e.g. Cargo's `version 1.x` peer-dep wars).
3. **High-fidelity builds by default.** Upgrades are *explicit* (`go get foo@v1.5`); transitive deps don't silently move under you because someone else updated their `^1.x`.

The trade-off MVS accepts: you sometimes ship a slightly-older transitive than the latest compatible. In return, the build is reproducible without trust in any solver implementation. For a contracts-and-verifier language whose first-order value is *reproducibility of behavior*, MVS is the philosophically aligned choice. [ADR 0033 §"strip policy"](0033-release-flag-strip-policy.md) made the same kind of trade — "explicit, auditable, reproducible" over "smart, automatic, sometimes wrong."

[ADR 0027 §"Version Resolution"](0027-package-management-design.md) listed `^`, `~`, `>=`, `<` operators. That direction is **revised** by this ADR (see "Revisions to ADR 0027" below).

## Decision

### 1. Hosting model — git-based

Dependencies are identified by a git URL + a semver tag. The resolver clones the repository, checks out the tag, validates a checksum, and caches the tree. No central registry, no publish step, no infrastructure to operate. A central index can be layered on later without invalidating git-based deps.

### 2. Identity — short name in imports, full URL in manifest

Two-layer naming, mirroring Cargo more than Go (and *unlike* Go's "the module path is the import path"):

- **Manifest entry** uses the full git URL: `foo = { git = "github.com/lhaig/foo", version = "1.2.3" }`.
- **Source imports** use a short name: `import foo;` or `import foo.submodule;`.

The short name is the key in `[dependencies]`. This preserves the existing Phase-13 import syntax — no breakage to current `examples/packages/*` manifests.

Rationale: full URLs in import statements (the Go approach) bind source code to a hosting choice (`github.com/...`). Short names in imports keep Intent sources host-agnostic — if a dep moves from GitHub to a self-hosted git server, only `intent.toml` changes, not every `import` line.

### 3. Manifest schema extension

The existing `DependencySpec` is extended with a `Git` source. The schema becomes:

```toml
# intent.toml
[package]
name = "my_app"
version = "0.1.0"

[dependencies]
foo = { git = "github.com/lhaig/foo", version = "1.2.3" }   # git source
bar = { path = "../bar" }                                    # local path (unchanged)
baz = "1.0.0"                                                # legacy: short form, kept for compatibility
```

The bare-string form (`baz = "1.0.0"`) remains parseable for ADR-0027-era manifests but emits a soft warning starting in Phase 30 ("declare a `git` source or `path`; bare version strings are deprecated and will be removed in v2").

`DependencySpec` Go struct gains:
```go
type DependencySpec struct {
    Version string // semver minimum (interpretation changes — see §6)
    Path    string // local path (unchanged)
    Git     string // git source URL — new; mutually exclusive with Path
}
```

`Validate()` enforces: at most one of `Path` / `Git` set; if `Git` is set, `Version` is required.

### 4. Lockfile — `intent.lock`

A new file in the project root, generated by `intentc pkg install` and committed to source control. Layout follows Cargo / Bundler conventions:

```toml
# intent.lock — DO NOT EDIT BY HAND
version = 1                       # lockfile format version
generated = "2026-06-03T09:14:22Z"

[[package]]
name = "foo"
version = "1.2.3"
source = "git+https://github.com/lhaig/foo"
rev = "a7c4f9e8b3..."             # full commit hash at the tag
checksum = "sha256:8f3e..."       # sha256 of the tree (sorted-file-list scheme; §7)
dependencies = ["bar 1.4.0"]      # transitive deps in this lockfile

[[package]]
name = "bar"
version = "1.4.0"
source = "git+https://github.com/lhaig/bar"
rev = "f9c1..."
checksum = "sha256:..."
dependencies = []
```

Properties:
- **Generated**, never edited. `intentc pkg install` writes; `intentc pkg upgrade` updates.
- **Committed to source control.** Like `Cargo.lock` / `Gemfile.lock`; unlike `go.sum`-only model, it records resolved versions explicitly so reproducing a build doesn't require fetching every transitive manifest.
- **Lockfile format `version = 1`.** Reserves room for v2 changes (e.g. signing fields) without breaking v1 consumers.
- **`rev` is mandatory.** Even when `version` is set, the lockfile pins the commit hash. Catches retagged releases.
- **`checksum` is mandatory.** Sha256 of a normalised tree hash (§7), not the git tree object — so the same checksum works for non-git mirrors later.

### 5. Cache layout extension

Phase 13's cache lives at `~/.intent/cache/<name>/<version>/`. Phase 30 will extend this without breaking it:

```
~/.intent/cache/
  <name>/<version>/                     # legacy layout; kept for ADR-0027 bare-version manifests
  git/<host>/<owner>/<repo>@<rev>/      # new: git-sourced packages, keyed by commit hash
    intent.toml
    *.intent
  .checksums/                           # new: parallel checksum store
    git/<host>/<owner>/<repo>@<rev>.sha256
```

Keying git deps by **commit hash** (not version tag) means the same `(host, owner, repo, rev)` triple is content-addressed in the cache — even if a tag is retroactively moved.

### 6. Version resolution — Minimum Version Selection

The `Version` field in `DependencySpec` is reinterpreted from "constraint" to "minimum acceptable":

- `version = "1.2.3"` means `>= 1.2.3` (same major).
- Existing `^1.2.3` / `~1.2.3` constraints are accepted and **mapped to `>= 1.2.3`**. The constraint-solver semantics from ADR 0027 are retired; the parser still accepts the operators for back-compat but ignores the variant (`^` vs `~`) and uses the base version as the minimum.

Resolution algorithm (MVS):
1. Read the project's `intent.toml` and collect direct deps as `(name → minimum_version)`.
2. For each direct dep, fetch its `intent.toml` (clone + checkout if not cached).
3. Walk transitive deps; for each `name`, the build list version is `max(minimum_version)` across all paths that mention it. Same-major-version compatibility is required — a transitive ask for `foo >= 2.0.0` when the app declares `foo >= 1.5.0` is a hard error (no diamond dependency tolerance, consistent with ADR 0027 §"Constraints").
4. Resolve each `(name, minimum_version)` to the highest patch version available at that major+minor floor (greedy tag enumeration: query git tags via `git ls-remote --tags`, filter to `vMAJOR.MINOR.x`, pick the highest `x`).
5. Write `intent.lock` with the resolved set; populate the cache.

Cross-major upgrades are explicit (Go's `go get` model): `intentc pkg upgrade foo --major` bumps the minimum to the latest of the next major; ordinary `intentc pkg install` never crosses a major boundary.

### 7. Checksum policy

Each cached package gets a **tree-hash checksum**, recorded in the lockfile and verified on every load:

- **Algorithm:** sha256 of the concatenation of, in NFC-normalised sorted-path order, `len(path) || path || len(content) || content` for every file in the package tree (excluding `.git/`, build artifacts, and OS junk like `.DS_Store`).
- **Why not the git tree object hash?** Two reasons: (a) it would tie the design to git forever; the same content fetched from a non-git mirror (HTTP tarball, IPFS, etc.) should hash identically. (b) git tree hashes use sha1; sha256 git is still rolling out and not assumed.
- **Verification:** every `intentc build` re-hashes the cached tree and compares against the lockfile entry. Mismatch is a hard error ("cache poisoning detected; run `intentc pkg install --refresh`").

Signing is **deferred** — see §9.

### 8. Vendor flow

`intentc pkg vendor` copies the resolved dependency set into `./vendor/` for fully-offline builds:

```
my-project/
  intent.toml
  intent.lock
  main.intent
  vendor/                       # gitignored by convention, but optional to commit
    foo-1.2.3/
      intent.toml
      *.intent
    bar-1.4.0/
      ...
```

When `./vendor/` exists, `intentc build` reads from it instead of `~/.intent/cache/`. This is the offline/CI / air-gapped story. Mirrors `go mod vendor` and `cargo vendor`.

### 9. Signing — deferred to ADR 004x

Sha256 checksums + lockfile commits give *integrity* but not *authenticity*. A compromised GitHub account could push a malicious tag matching the lockfile checksum format. Defending against that needs publisher signing (sigstore, gpg, minisign, etc.) and a key-distribution story. This ADR deliberately does **not** define a signing model — it would slow down v1, and the unsigned-but-checksummed model is a strict improvement on the current "no registry at all" state.

A follow-up ADR will revisit signing when there are users to protect. The lockfile schema reserves a `signature` field via `version = 1` versioning; v2 lockfiles will populate it.

### 10. Authentication — credential delegation to git

Private repositories are accessed using whatever credentials the local `git` binary already has (HTTPS basic auth via the system credential helper; SSH via `~/.ssh`; GitHub CLI auth via `gh auth`). No bespoke credential store in `intentc`.

Concrete behaviour:
- `git = "github.com/private-org/foo"` invokes `git clone https://github.com/private-org/foo.git`.
- If the user's git is configured with an SSH key, that flow works without intervention.
- If 2FA / SSO is required, the user authenticates via their git toolchain *outside* `intentc`, then re-runs install.
- `intentc` never prompts for credentials, never writes a token to disk.

Rationale: cloning the credential-management problem from `git` into `intentc` is a perpetual source of bugs and security incidents in other ecosystems (npm tokens, Cargo credentials, etc.). Delegating to the system git toolchain is what Go modules does and it has aged well.

## Revisions to ADR 0027

ADR 0027 §"Version Resolution" is **revised** by this ADR:

| ADR 0027 (Phase 13) | ADR 0039 (Phase 30) |
|---|---|
| Constraint operators: `=`, `^`, `~`, `>=`, `<` | Minimum-version semantics; `^` / `~` accepted but reinterpreted as `>=` minima |
| Constraint solver in `semver.go` | MVS graph walk; constraint solver retired |
| `ConstraintBaseVersion()` returns the constraint base version | Replaced by `ResolveMinimumVersion()` returning the highest patch tag at the declared major+minor |
| "No diamond dependency resolution; one version per package" | Retained — cross-major conflicts remain hard errors |

ADR 0027's status will be updated to "revised by [ADR 0039](0039-package-registry-git-mvs.md)" when Phase 30 lands.

## Consequences

### Code surface (Phase 30 — implementation phase)

The implementation work to deliver this ADR is *not* in this ADR — it's tracked as Phase 30. Surface-level estimate:

- `internal/compiler/manifest.go` — add `Git` field; back-compat for bare-version short form; soft warning.
- `internal/compiler/semver.go` — retire `ConstraintBaseVersion`; add `ResolveMinimumVersion`; MVS graph walker.
- `internal/compiler/lockfile.go` — new file; serialize/deserialize `intent.lock`.
- `internal/compiler/fetcher.go` — new file; `git clone` wrapper, tag enumeration via `git ls-remote --tags`, tree-hash checksum.
- `internal/compiler/cache.go` — extend with `git/<host>/<owner>/<repo>@<rev>/` layout.
- `cmd/intentc/main.go` — rewire `pkg install` to call MVS resolver; add `pkg upgrade`, `pkg vendor`, `pkg lock` subcommands; remove "no registry available" warning.
- Wire `intent.lock` into the gitignore / template generated by `pkg init` (it *should* be committed).

### Removed / replaced TODOs

- `internal/compiler/semver.go:275` — removed (function retired in favor of MVS resolver).
- `cmd/intentc/main.go:942` — removed (cache population now happens via the fetcher).

### Migration

No existing `intent.toml` in the repo uses `^` or `~` constraints, so the resolver semantics flip is silent in practice. The bare-version short form (`baz = "1.0.0"`) remains parseable but emits a soft deprecation warning starting in Phase 30.

### Backend-agnostic

Like Phase 29's [ADR 0038](0038-retire-legacy-rust-testgen.md), this design has *no* per-backend code paths. The cache and lockfile are agnostic to whether the consumer is the Rust, JS, or WASM backend.

### Self-hosting unblocks

With this ADR shipped (Phase 30), the linter / formatter / compiler rewrites in Intent can:
- Declare their own `intent.toml` with versioned deps on standard-library modules.
- Reproduce their build deterministically from `intent.toml` + `intent.lock`.
- Run offline via `intentc pkg vendor`.

This is the design that lets Phase 31+ start moving tooling into Intent.

## Follow-ups

- **Phase 30 (PRD).** Implement everything in this ADR. Estimated ~1.5k LOC across manifest/lockfile/fetcher/cache/CLI.
- **ADR 004x — Signing / authenticity.** When the user base is non-zero, design publisher signing. Sigstore is the leading candidate (free, transparency log, OIDC-based identity).
- **Phase 31+ — First self-hosted tool.** Most likely candidate: the formatter (smallest surface, no language gaps).
- **Optional: a central index.** A simple GitHub repo with a `packages.toml` listing well-known packages → git URLs would let users write `foo = "1.2.3"` without the full `git = "..."` URL. Layerable on top of this ADR without changes; deferred until there's demand.

## References

- [ADR 0027](0027-package-management-design.md) — Package management design (Phase 13)
- [ADR 0038](0038-retire-legacy-rust-testgen.md) — Phase 29 testgen retirement (same self-hosting motivation)
- Russ Cox, "Minimal Version Selection" (research.swtch.com/vgo-mvs, 2016)
- Go modules reference: go.dev/ref/mod
- Cargo book: doc.rust-lang.org/cargo
- Bundler manual: bundler.io/v2.5/man/bundle-install.1.html
- `ops/NEXT-STEPS.md` (2026-06-02) — design-space notes for this ADR
