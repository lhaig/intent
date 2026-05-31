# Phase 25: Cross-Package Goto-Def (Already Working — Test Coverage + Docs)

**Status:** Shipped (2026-05-31; commits to follow)
**Milestone:** v1.2 — Self-Improvement Foundations (LSP polish carry-over from Phase 19)

## Goal

Confirm by test that cross-package goto-definition already works in the LSP, lock the behaviour in a regression test, and remove the stale "deferred to v1.1" caveat from ADR 0032's out-of-scope list.

Phase 19's PRD listed "Cross-package go-to-definition (v1.1)" under deferred items. That deferral was based on an incorrect assumption about how the workspace surface exposes sibling-module ASTs. In fact:

- `workspace.siblingModules(...)` invokes `compiler.NewModuleRegistry(entry)` and `reg.AllModules()`.
- `ModuleRegistry.AllModules()` returns *every* module discovered during `DiscoverDependencies`, including those reached transitively through `intent.toml`'s `[dependencies]` section.
- `resolveAcrossWorkspace` iterates `siblings` linearly looking for the requested name; it doesn't gate on which package each sibling belongs to.

So the resolver already searches cross-package siblings. The only thing missing was a test that locks the behaviour in.

## Success Criteria

- [x] New LSP test `TestDefinitionCrossPackage` covers: an app package imports a types package via `path = "../types_pkg"`; goto-def on a type used from the imports package returns a Location pointing at the type's declaration in the dependency package
- [x] Test handles the macOS `/var` → `/private/var` symlink (the registry walks real paths; `t.TempDir` uses the alias)
- [x] ADR 0032's "Still out of v1, deferred" lists no longer mention cross-package goto-def
- [x] `docs/ROADMAP.md` v1.2 entry adds Phase 25 SHIPPED noting "discovered to already work; regression test added"
- [x] `make validate` green
- [x] No code change in `internal/lsp/symbol.go` or `internal/lsp/workspace.go` — the existing implementation already handles cross-package; only test + docs change.

## Reference

- Phase 19 PRD: `ops/plans/phase-19-lsp-v1-completion.md` (where the deferral was claimed)
- ADR 0032: `docs/decisions/0032-lsp-v1-surface.md` (deferred lists updated)
- LSP resolver: `internal/lsp/symbol.go` — `resolveAcrossWorkspace`
- LSP workspace: `internal/lsp/workspace.go` — `siblingModules`
- ModuleRegistry: `internal/compiler/registry.go` — `AllModules`, `DiscoverDependencies`
- Existing same-package test: `internal/lsp/definition_test.go` — `TestDefinitionCrossFileSamePackage`
- Real example with cross-package imports: `examples/packages/app_pkg/` + `examples/packages/types_pkg/`

## Tasks

### 25.1 Regression test

**Files:** `internal/lsp/definition_test.go`

`TestDefinitionCrossPackage` builds a two-package workspace in `t.TempDir()`, opens the app package's `main.intent`, and asserts goto-def on a type imported from the dependency package returns a Location in the dependency package's directory.

**Acceptance:** `go test ./internal/lsp/... -run DefinitionCrossPackage` passes.

### 25.2 Docs update

**Files:** `docs/decisions/0032-lsp-v1-surface.md`, `docs/ROADMAP.md`, `INTENT.md` (if it mentions cross-package goto-def as deferred)

- ADR 0032: remove "cross-package goto-def" from both out-of-scope lines (the Phase 19 revision section and the Phase 20 revision section). Add a brief "Revised — 2026-05-31 (Phase 25)" note clarifying that cross-package goto-def shipped as a side effect of Phase 19's workspace plumbing; Phase 25 added a regression test.
- ROADMAP: add `### Phase 25: Cross-Package Goto-Def -- SHIPPED (date)` under v1.2.
- INTENT.md: editor-support section already mentions "same-file and same-package"; update to drop the qualifier if cross-package is now in scope.

**Acceptance:** `make validate` green. No remaining "cross-package goto-def deferred" claims in repo docs.

## Out of Scope

- **Goto-def into transitive dependencies of dependencies** beyond what the registry already discovers. The registry walks the full dependency graph; if your `intent.toml` chains `A → B → C`, goto-def from A on a symbol in C should work for free. Not separately tested in this phase.
- **Goto-def into the Rust dependencies** declared in `[rust_dependencies]`. Those aren't `.intent` source; out of scope by definition.
- **Cross-package find-references**. That's the next subtask in the parent task sequence; handled separately.

## Notes

This phase exists because Phase 19's deferral claim was wrong. The intended convention going forward (per [[feedback-write-adrs-along-the-way]]):

- When a deferred item turns out to already work, document the discovery in a small PRD like this one and update the deferred-list with the correct status.
- Don't quietly remove the caveat — explain how the deferral was wrong, so future readers don't repeat the analysis.
