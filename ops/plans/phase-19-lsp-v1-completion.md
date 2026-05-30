# Phase 19: LSP v1 Completion

**Status:** Draft
**Milestone:** Milestone 8 — Developer Experience (Phase 18 follow-on)
**Decision:** [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md) (revised in this phase to expand the v1 surface)

## Goal

Close the gaps that make Phase 18's LSP feel half-built when a user actually edits an `.intent` file. Phase 18 shipped diagnostics, hover, and go-to-definition but limited each to a narrow surface — hover only worked on top-level declaration names, the outline view was empty, "Format Document" returned "no formatter available," and there was no completion or signature help. This phase fills those gaps so a user opening their first `.intent` file in VS Code gets the experience they expect from a modern LSP.

What "fully working v1" means after Phase 19:

- Hovering on **anything** — locals, parameters, method calls, field access, top-level decls — returns useful information.
- Go-to-definition jumps from any reference (call site, method call, field access, identifier reference) to its declaration within the same package.
- Typing an identifier prefix gets you suggestions. Typing `(` after a function name shows parameter hints.
- The outline view shows your file's structure.
- "Format Document" runs `intentc fmt`.

What stays out of v1 (real refactoring scope or polish, see Out of Scope):

- Rename, find-references (touches more than one file in non-trivial ways)
- Code actions / quick fixes (per-rule design)
- Refactorings (extract function, etc.)
- Semantic tokens, inlay hints (polish on top of working diagnostics)
- Cross-package goto-def (deferred to v1.1)
- Member completion (`.field`, `.method` after `.`) — needs receiver-type resolution at cursor, lands in v1.2
- Marketplace publishing (distribution, not capability)
- Per-contract Z3 source positions (anchors at (1,1) today; threading line/col through verify is a separate v1.1 piece)

## Why this is a separate phase, not Phase 18 extended

Phase 18 shipped end-to-end; commits and ADR status are locked in. Reopening the phase to expand its scope rewrites history. Phase 19 is the cleaner record: Phase 18's narrower scope was a deliberate calendar trade, Phase 19 expands it once we saw the rough edges.

## Success Criteria

- [ ] `internal/lsp/scope.go` walks AST with a scope stack so hover/goto-def/completion can resolve locals, parameters, and method receivers
- [ ] Hover works on: function calls (already), method calls, field access, let-bound locals, function parameters, `self` inside methods, entity/enum/trait references in type positions, top-level decls (already)
- [ ] Go-to-definition works on: function calls (already), method calls, field access, let-bound locals, function parameters, top-level decls (already)
- [ ] `textDocument/documentSymbol` returns a tree: top-level functions/entities/enums/traits/tests, with entity methods nested under their entity
- [ ] `textDocument/formatting` runs the `internal/formatter` package and returns a single `TextEdit` replacing the whole document
- [ ] `textDocument/signatureHelp` returns parameter info when the cursor is inside a function or method call's argument list; active-parameter index tracks which arg is being typed
- [ ] `textDocument/completion` returns identifier suggestions: top-level decl names + in-scope locals/params + Intent keywords/built-in type names. No member completion (deferred)
- [ ] VS Code extension activates the new capabilities (auto-discovered from server caps; no extension code change needed)
- [ ] End-to-end smoke test extended to drive every new method
- [ ] ADR 0032 revised with a dated "Revised: scope expanded" section listing the additions
- [ ] `docs/ROADMAP.md` Milestone 8 entry mirrors the expanded surface
- [ ] `INTENT.md` "Editor support" section updated
- [ ] `make validate` green
- [ ] No regressions in any Phase 18 test

## Reference

- Phase 18 design: `docs/decisions/0032-lsp-v1-surface.md`
- Phase 18 PRD: `ops/plans/phase-18-lsp-server.md`
- Existing LSP package: `internal/lsp/`
- Existing symbol resolver (name-based, top-level-only): `internal/lsp/symbol.go`
- Formatter to wire into LSP: `internal/formatter/`
- LSP 3.17 spec sections: completion (`textDocument/completion`), documentSymbol, formatting, signatureHelp

## Tasks

### 19.1 Scope walker

**Files:** `internal/lsp/scope.go` (new), `internal/lsp/scope_test.go` (new)

Walk the AST with a scope stack. At any (line, col), the walker can answer:

- "What's in scope here?" → list of identifiers with their kind (function, let, param, entity, etc.)
- "What declares the identifier `x` at this position?" → returns the declaring node + line/col

The walker:

- Maintains a stack of scopes: program-level, function-body, block (`if`/`while`/`for`), entity-method (with `self` injected).
- At each statement/expression, pushes/pops scope frames.
- Records the (line, col) range each scope covers so the lookup can pick the innermost frame containing the cursor.
- For method bodies, injects `self: <Entity>` plus a synthetic frame containing the entity's fields and methods.

Output: a `scopeResolver` with two methods:

```go
func (r *scopeResolver) resolve(prog *ast.Program, line, col int, name string) *declRef
func (r *scopeResolver) inScope(prog *ast.Program, line, col int) []*declRef
```

`declRef` is a richer version of `declHit` from Phase 18 that includes locals/params and tracks the *declaring* node.

**Acceptance:**
- `go test ./internal/lsp/... -run Scope` covers: resolve a let inside its block; resolve a let outside its block returns nil; resolve a function param inside the body; resolve `self` inside a method; the same identifier shadowed by an inner let returns the inner binding.

### 19.2 Hover + goto-def for locals, params, methods, fields

**Files:** `internal/lsp/hover.go`, `internal/lsp/definition.go`, `internal/lsp/hover_test.go`, `internal/lsp/definition_test.go`, `internal/lsp/symbol.go`

- Replace the name-only resolver call in hover/definition with the scope-aware path. For each kind of reference, the handler:
  - Top-level decl (already working) → existing path
  - Let-binding → resolve to the `LetStmt`; hover shows `let <name>: <type>`; goto-def jumps to the let line
  - Parameter → resolve to the `Param`; hover shows `<name>: <type>` with function context; goto-def jumps to the param position
  - `self` → hover shows `self: <Entity>`
  - Method call (`expr.foo(...)`) → resolve `foo` on the receiver's entity type via the checker's type info; hover shows the method signature + contracts; goto-def jumps to the method declaration
  - Field access (`expr.field`) → resolve `field` on the receiver's entity type; hover shows the field's declared type; goto-def jumps to the field declaration
- For receiver-type resolution: re-run `checker.CheckWithResult` (we do this anyway in the diagnostics path; cache via 19.2's scope resolver entry).

**Acceptance:**
- Hover tests for: local var, function param, method call, field access, `self`, top-level decl (regression).
- Goto-def tests for: local var, function param, method call jumps to method, field access jumps to field, top-level decl (regression).

### 19.3 Document symbols

**Files:** `internal/lsp/symbols.go` (new), `internal/lsp/symbols_test.go` (new), `internal/lsp/server.go`

- `textDocument/documentSymbol` handler returns `DocumentSymbol[]` — a tree mirroring the file's structure.
- Top-level entries: functions, entities, enums, traits, tests, extern functions.
- Each entity carries its methods (and constructor) as children.
- Each enum carries its variants as children.
- Each trait carries its method signatures as children.
- `SymbolKind` mapping: Function=12, Variable=13, Class=5 (entity), Enum=10, EnumMember=22, Interface=11 (trait), Method=6, Field=8, Property=7, Constructor=9.
- Range = full source range of the declaration (best effort from AST line/col); selectionRange = name range.

Add capability `documentSymbolProvider: true` to the initialize response.

**Acceptance:**
- `go test ./internal/lsp/... -run DocumentSymbol` covers: top-level functions appear; entity methods nest under their entity; enum variants nest under their enum; empty file returns empty slice.

### 19.4 Formatting via LSP

**Files:** `internal/lsp/format.go` (new), `internal/lsp/format_test.go` (new), `internal/lsp/server.go`

- `textDocument/formatting` handler. Parses the document text, runs `internal/formatter`, and returns a single `TextEdit` replacing the whole document.
- If the parser errored, return an empty `TextEdit[]` — don't write garbage on top of an unparseable file.
- If the formatted output equals the input, return an empty `TextEdit[]` — let the editor skip a no-op edit.

Add capability `documentFormattingProvider: true`.

**Acceptance:**
- `go test ./internal/lsp/... -run Formatting` covers: unformatted file returns one TextEdit with the formatted content; already-formatted file returns no edits; unparseable file returns no edits.

### 19.5 Signature help

**Files:** `internal/lsp/signature.go` (new), `internal/lsp/signature_test.go` (new), `internal/lsp/server.go`

- `textDocument/signatureHelp` handler. Walks back from the cursor to find the enclosing call: scan the text for the matching unclosed `(`, capture the identifier before it, count commas after the `(` and before the cursor to compute the active parameter index.
- Resolve the called function/method via the symbol resolver from 19.1/19.2.
- Return a `SignatureHelp` with one signature containing the function's full sig and one parameter entry per declared param.

Add capability `signatureHelpProvider: { triggerCharacters: ["(", ","] }`.

**Acceptance:**
- `go test ./internal/lsp/... -run SignatureHelp` covers: cursor inside `add(|)` returns `add`'s signature with active=0; cursor at `add(1, |)` returns active=1; cursor not inside a call returns null.

### 19.6 Completion

**Files:** `internal/lsp/completion.go` (new), `internal/lsp/completion_test.go` (new), `internal/lsp/server.go`

- `textDocument/completion` handler. Returns `CompletionItem[]`:
  - All in-scope locals/params at the cursor (from the scope walker)
  - All top-level decls in the open document (functions, entities, enums, traits, extern functions)
  - All top-level decls in sibling modules (workspace-aware)
  - Intent keywords (`if`, `else`, `while`, `for`, ...)
  - Built-in type names (`Int`, `Float`, `String`, `Bool`, `Void`, `Array`, `Map`, `Result`, `Option`, `Future`, `Fn`)
- `CompletionItemKind` per source: Variable=6 for let/param, Function=3, Class=7 for entity, Enum=13, Interface=8 for trait, Keyword=14, Module=9.
- No member completion (`.field` / `.method` after a `.`) in v1 — deferred. The cursor's preceding char check filters out positions where member completion would be expected.

Add capability `completionProvider: { triggerCharacters: [], resolveProvider: false }`.

**Acceptance:**
- `go test ./internal/lsp/... -run Completion` covers: top-level decls appear; in-scope let appears; out-of-scope let does not appear; keywords present; sibling module's public decls present; cursor after a `.` returns the same list (no member filtering — deferred — but the request still succeeds).

### 19.7 Server capabilities + E2E extension

**Files:** `internal/lsp/server.go`, `internal/lsp/e2e_test.go`

- Advertise the new capabilities in the `initialize` response.
- Extend the E2E test to drive `textDocument/documentSymbol`, `textDocument/formatting`, `textDocument/signatureHelp`, `textDocument/completion`, plus a hover on a local and a goto-def on a method call.

**Acceptance:**
- `go test ./internal/lsp/... -run E2E` passes with the extended flow.

### 19.8 Docs + ADR revision + roadmap + INTENT.md

**Files:** `docs/decisions/0032-lsp-v1-surface.md`, `docs/decisions/README.md`, `docs/ROADMAP.md`, `INTENT.md`, `ops/plans/phase-19-lsp-v1-completion.md`, `editors/vscode/README.md`

- ADR 0032: add a `## Revised — 2026-05-30 (Phase 19)` section listing the expanded surface. Status line: `accepted; revised in Phase 19; implemented in <range>`.
- Decisions README: update the 0032 row.
- ROADMAP.md: Milestone 8 LSP entry mirrors the expanded surface.
- INTENT.md: update "Editor support" section.
- VS Code README: update the feature list.
- Phase 19 PRD Status: `Draft` → `Shipped (date)`.

**Acceptance:**
- All docs consistent. `make validate` green.

## Out of Scope (truly deferred)

- Rename across files (refactoring; needs cross-package symbol index)
- Find references (uses the resolver but cross-package surface is non-trivial)
- Code actions / quick fixes (per-lint-rule design)
- Refactorings (extract function, inline, change signature, ...)
- Semantic tokens (richer-than-tmLanguage highlighting)
- Inlay hints (type / parameter inlays)
- Cross-package go-to-definition (v1.1)
- Member completion (`.field` / `.method`) (v1.2 — needs cursor receiver-type resolution)
- Marketplace publishing (v1.1)
- Per-contract Z3 diagnostic positions (anchors at (1,1) today; threading positions through verify is a separate v1.1 piece)
- Incremental text sync (Full sync stays in v1)
- Multi-byte UTF-16 column handling (ASCII-only stays in v1)
- Multi-root workspaces
- TCP transport
- `didChangeWatchedFiles` reactivity

## Suggested Order

1. **19.1 Scope walker** — unblocks 19.2, 19.5, 19.6
2. **19.2 Hover + goto-def upgrades** — uses 19.1; biggest user-visible UX lift
3. **19.4 Formatting via LSP** — independent and trivial; quick win
4. **19.3 Document symbols** — independent; second quick win
5. **19.5 Signature help** — uses 19.1
6. **19.6 Completion** — uses 19.1
7. **19.7 Capabilities + E2E** — locks the wire contract
8. **19.8 Docs + ADR revision** — status flip
