# Phase 26: LSP `textDocument/references` (Find References)

**Status:** Shipped (2026-05-31; commits 1c94d3d..HEAD)
**Milestone:** v1.2 — Self-Improvement Foundations (LSP capability addition)
**Decision:** [ADR 0035](../../docs/decisions/0035-lsp-find-references.md)

## Goal

Implement `textDocument/references` on the LSP server: given a cursor on a symbol, return every place that symbol is referenced across the workspace. Cover top-level decls (functions, entities, enums, enum variants, traits, tests) and locals/params per ADR 0035. Method and field references stay deferred.

What "done" looks like:

- `references` capability advertised in the `initialize` response.
- `textDocument/references` returns `Location[]` for every symbol kind ADR 0035 covers.
- `context.includeDeclaration` honoured per LSP spec.
- Cross-package references work for free (same workspace plumbing as Phase 25's goto-def).
- VS Code's "Find All References" command (`Shift+F12`) returns useful results.

## Success Criteria

- [x] `references.go` (new) handles `textDocument/references` requests
- [x] `referencesProvider: true` in `initialize` capabilities
- [x] Top-level function references: every call site + the declaration (if `includeDeclaration`)
- [x] Entity references: every type-position occurrence + every constructor call + declaration
- [x] Enum references: every type-position occurrence + every variant-construction + declaration
- [x] Enum-variant references: every match arm + every construction + declaration
- [x] Trait references: every type-position occurrence (impl headers, generic bounds) + declaration
- [x] Test references: declaration site only (tests aren't referenced from code)
- [x] Local / param / self references: every `VarRef` within the enclosing function body
- [x] Cross-package references: traverses every AST in `workspace.siblingModules()` plus the open document's AST
- [x] Same-name disambiguation: references for `add` in module A do not include references to `add` in module B
- [x] Unit tests cover: function refs, entity refs, enum-variant refs, local refs, `includeDeclaration` true/false, cross-package, same-name disambiguation
- [x] E2E test: VS Code-style request roundtrip returns expected Locations
- [x] `make validate` green
- [x] No regression in existing LSP tests

## Reference

- ADR 0035: `docs/decisions/0035-lsp-find-references.md`
- Existing resolver: `internal/lsp/symbol.go` — `resolveAtPosition`, `declHit`, `localRef`
- Scope walker: `internal/lsp/scope.go`
- Workspace surface: `internal/lsp/workspace.go` — `siblingModules`
- LSP protocol shape: `internal/lsp/protocol.go`
- Existing goto-def handler (closest precedent for shape): `internal/lsp/definition.go`
- LSP 3.17 spec for `textDocument/references`

## Tasks

### 26.1 Protocol additions

**Files:** `internal/lsp/protocol.go`, `internal/lsp/server.go`

Add:

- `ReferenceContext { IncludeDeclaration bool }`
- `ReferenceParams { TextDocument, Position, Context ReferenceContext }`
- `ReferencesProvider bool` in `ServerCapabilities`

Advertise `ReferencesProvider: true` in the `initialize` response.

**Acceptance:** Server initialize test asserts `referencesProvider: true` in the capabilities; `ReferenceParams` decodes a sample request from VS Code-shaped JSON.

### 26.2 Reference-walker per symbol kind

**Files:** `internal/lsp/references.go` (new), `internal/lsp/references_test.go` (new)

A reference walker traverses an `*ast.Program` and collects positions matching a target symbol. The target is one of:

- A `declHit` (top-level decl resolved from the cursor)
- A `localRef` (local binding within the cursor's enclosing function)

For each kind:

- **Function**: walk every `CallExpr`; if the call target's resolved declaration matches the target by path/line/column, record the call site's position. Also walk extern-function call sites the same way.
- **Entity**: walk every `TypeRef` whose name matches and resolves to the target entity; also every `EntityName(...)` call site that constructs it.
- **Enum**: walk every `TypeRef` whose name matches; plus every variant construction (`EnumName.Variant(...)`).
- **Enum variant**: walk every match arm pattern + every variant construction.
- **Trait**: walk every type-position occurrence (impl headers, generic bounds, parameter types).
- **Test**: just the declaration site.
- **Local / param / self**: scope-bound — walk only the enclosing function's body, collecting `VarRef`s whose name matches and resolve to the same binding.

Same-name disambiguation: when two declarations share a name across modules, the walker checks whether the use-site resolves to the target's declaration (via path + line + column tuple). Locals are bounded to their function, so no cross-binding ambiguity.

**Acceptance:**
- Unit tests on the walker covering each symbol kind, both with `includeDeclaration: true` and `false`
- A same-name test: two functions named `add` in different modules; references for one don't include uses of the other

### 26.3 Handler wiring

**Files:** `internal/lsp/references.go`, `internal/lsp/server.go`

`handleReferences`:

1. Resolve the symbol at the cursor via `resolveAtPosition` (same as hover/definition).
2. If no resolution: respond with empty `Location[]`.
3. Else dispatch to the walker for the resolved kind across every AST (`prog` + `siblings`).
4. If `context.IncludeDeclaration`, prepend the declaration's name range to the result.
5. Sort results by URI then by start position for stable client display.
6. Respond with the `Location[]`.

**Acceptance:**
- New LSP test: send `textDocument/references` for a function with two call sites; receive 3 locations (decl + 2 sites) when `includeDeclaration: true`, 2 sites when `false`
- Empty result for non-identifier cursors

### 26.4 Cross-package references test

**Files:** `internal/lsp/references_test.go`

A multi-package fixture: `app_pkg` imports `types_pkg` and calls `types_pkg.Point(...)` from `main.intent` plus one more location. `references` on `Point` (from either the app side or the types-side declaration) returns both call sites and the declaration.

**Acceptance:** Test passes; symlink handling uses `EvalSymlinks` like Phase 25's `TestDefinitionCrossPackage`.

### 26.5 E2E test extension

**Files:** `internal/lsp/e2e_test.go`

Extend the E2E test to issue a `textDocument/references` request for a top-level function and assert non-empty response.

**Acceptance:** Existing E2E test passes with the new step.

### 26.6 Docs

**Files:** `docs/ROADMAP.md`, `INTENT.md`, `editors/vscode/README.md`, `docs/decisions/0032-lsp-v1-surface.md`

- ROADMAP: add `### Phase 26: LSP textDocument/references -- SHIPPED (date)` under v1.2.
- INTENT.md "Editor support": list "Find references" as supported.
- VS Code README: add "Find references (Shift+F12)" to the feature list; remove the "find-references — v1.1+" caveat.
- ADR 0032: 5th revision section noting find-references shipped per ADR 0035; remove `find-references` from out-of-scope lists.

**Acceptance:** `make validate` green; no stale "find-references deferred" claims.

### 26.7 PRD status flip

**Files:** `prds/done/phase-26-lsp-find-references.md`

`Status: In Progress` → `Status: Shipped (date; commits XXXX..HEAD)`. Tick the success-criteria boxes.

## Out of Scope

- **Method / field references.** Needs receiver-type disambiguation per use site; deferred per ADR 0035.
- **Trait-method enumeration / impl-site lookup.** Different shape from references on the trait *name*; deferred.
- **`textDocument/documentHighlight`.** Same shape but bounded to the open file; could be added easily later by reusing the walker.
- **`workspace/symbol` search.** Different mechanism (typeahead over the workspace's symbol set); separate phase.
- **Performance optimisation (indexed search).** Brute-force scan is fine at Intent's current scale.

## Suggested Order

1. **26.1 Protocol shape** — pure data plumbing
2. **26.2 Walker** — per-kind logic; the bulk of the work
3. **26.3 Handler wiring** — connects walker to LSP
4. **26.4 Cross-package test** — locks the workspace behaviour
5. **26.5 E2E** — locks the wire contract
6. **26.6 Docs + 26.7 status flip** — last
