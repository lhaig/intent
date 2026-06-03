# git_dep_demo

Illustrative example of the Phase 30 / [ADR 0039](../../../docs/decisions/0039-package-registry-git-mvs.md)
git-source dependency syntax.

The `intent.toml` declares a `git = "..."` dependency. Running

    intentc pkg install

against this manifest exercises the full registry pipeline:

1. **Resolve** — MVS picks the highest tag at the declared major with version
   greater than or equal to the minimum.
2. **Fetch** — `git clone` populates `~/.intent/cache/git/<host>/<owner>/<repo>@<rev>/`.
3. **Lock** — `intent.lock` is written next to `intent.toml` with the
   resolved version, full commit rev, and sha256 tree-hash checksum.
4. **Verify** — subsequent runs re-hash the cache and refuse to proceed if
   the lockfile checksum doesn't match.

The URL in the example is a placeholder. The reproducible registry test
lives in `internal/compiler/loader_test.go` (see `TestPkgInstallEndToEnd`)
and runs against a local bare git fixture so CI doesn't need network
access.
