# Phase 21: LSP Member Completion

**Status:** Shipped (2026-05-31; commits 7c44883..HEAD)
**Milestone:** Milestone 8 — Developer Experience (Phase 19/20 follow-on)
**Decision:** [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md) (revised in this phase to include member completion)

## Goal

Close the last v1.1 LSP gap: when the user types `receiver.` (or `receiver.partial`), the completion list should be the fields and methods of the receiver's entity type — not the full identifier list that Phase 19 returns at every position.

Phase 19 explicitly deferred member completion on the assumption that "cursor receiver-type resolution" was new infrastructure. In practice that infrastructure already shipped in Phase 19 via `internal/lsp/symbol.go` (`receiverBeforeMember` + `resolveMemberOnReceiver`) to make hover and goto-def work on `expr.field` / `expr.method`. Phase 21 reuses it.

Concretely, after Phase 21:

- Typing `account.` in `examples/bank_account.intent` lists `owner`, `balance`, `deposit`, `withdraw`, `get_balance`.
- Typing `self.` inside a method or constructor lists the same.
- Typing `self.bal` narrows on the client side; the server still returns the full member list (LSP norm — the client filters by typed prefix).
- Typing `unknown.` (receiver doesn't resolve, or no entity type) returns an empty list rather than the full identifier list.
- The `.` is registered as a completion trigger character so the editor pings the server immediately on dot.

## Success Criteria

- [x] PRD scoped against existing infrastructure (no separate scope walker work)
- [x] `completion.go` detects member-completion context using a small variant of `receiverBeforeMember`
- [x] Receiver resolves through scope (`self`, locals, params); fields and methods of the entity become `CompletionField` / `CompletionMethod` items with type detail / signature
- [x] Unresolvable receivers return an empty list (no false positives)
- [x] `.` advertised in `CompletionOptions.TriggerCharacters`; existing identifier completion unaffected when the cursor is not in member position
- [x] Tests cover: member completion after a local-typed receiver; after `self` in a method; after `self` in a constructor; unresolvable receiver returns empty; identifier completion at column 1 still includes top-level decls (regression)
- [x] E2E test extended to drive a member completion request
- [x] ADR 0032 gets a `## Revised — 2026-05-31 (Phase 21)` section noting member completion shipped
- [x] `docs/ROADMAP.md` Phase 19 line updated to reflect that the v1.2 follow-on landed; new Phase 21 entry added
- [x] `INTENT.md` editor-support note no longer caveats member completion as missing
- [x] `editors/vscode/README.md` feature list mentions member completion
- [x] `make validate` green
- [x] No regressions in any Phase 19 / 20 test

## Reference

- ADR 0032 (LSP v1 surface): `docs/decisions/0032-lsp-v1-surface.md`
- Phase 19 PRD: `ops/plans/phase-19-lsp-v1-completion.md` (member completion called out as deferred)
- Existing completion handler: `internal/lsp/completion.go`
- Existing receiver-type infrastructure: `internal/lsp/symbol.go` — `receiverBeforeMember`, `resolveMemberOnReceiver`, `localTypeName`, `findEntityByName`
- Field / method lookup helpers: `internal/lsp/scope.go` — `findFieldOnEntity`, `findMethodOnEntity`
- CompletionItem kinds: `internal/lsp/protocol.go` (`CompletionField = 5`, `CompletionMethod = 2`)
- LSP 3.17 `textDocument/completion` + `triggerCharacters`

## Tasks

### 21.1 Detect member-completion context

**Files:** `internal/lsp/completion.go`

The existing `receiverBeforeMember` in `symbol.go` requires a non-empty member identifier at the cursor (it's tuned for hover/goto-def where the user is *on* a name). Completion is asked for at positions where the member may be empty — `account.` with cursor immediately after the dot is the common case.

Add a `memberCompletionContext(text string, line, col int) (receiver string, ok bool)` helper that:

- Walks back from the cursor over identifier characters (the partial member, possibly empty) and any whitespace.
- Looks for a `.` immediately before that. If not present, returns `("", false)`.
- Walks back over whitespace + an identifier to find the receiver.
- Returns the receiver identifier and `true`.

Receiver must be a simple identifier (no chained `a.b.c`) — chained access stays deferred. The receiver-type resolver only handles single-step access today, and chaining is a separate-shaped problem.

**Acceptance:** unit test on `memberCompletionContext` covering: `account.` returns `("account", true)`; `self.balance` returns `("self", true)` when cursor is on `balance`; `account.bal` partial returns `("account", true)`; `foo` (no dot) returns `("", false)`; `a.b.c` returns `("", false)` (chained — out of scope, treat as unresolvable rather than partially resolving).

### 21.2 Build member completion items

**Files:** `internal/lsp/completion.go`

Add `buildMemberCompletionItems(prog *ast.Program, scope *scopeResolver, line, col int, receiver string) []CompletionItem`:

- Resolve the receiver's entity:
  - `self` → `scope.enclosingMethod(line, col)` or `scope.enclosingConstructor(line, col)` → `*ast.EntityDecl`.
  - Other identifier → `scope.resolveLocal(line, col, receiver)` → `localRef`; use `localTypeName` then `findEntityByName` to get the entity.
- If no entity, return an empty slice.
- For each field on the entity: emit a `CompletionField` item with the field's type as detail.
- For each method on the entity: emit a `CompletionMethod` item with the method signature as detail (`(<params>) -> <return>`).

Fields and methods are the only members surfaced. Constructor is reachable via the entity name itself (handled by identifier completion), not via `.`. Trait method completion is out of scope; the receiver-type resolver doesn't model traits today.

**Acceptance:** unit tests on `buildMemberCompletionItems` covering: receiver is a typed local → fields and methods present with correct kinds and detail strings; receiver is `self` inside a method → same; receiver is `self` inside a constructor → same; receiver doesn't resolve → empty slice.

### 21.3 Wire member completion into the request handler

**Files:** `internal/lsp/completion.go`

In `handleCompletion` (and / or `buildCompletionItems`), call `memberCompletionContext` first:

- If in member position, return `buildMemberCompletionItems(...)` — *only* the member list. Do not concatenate keywords, top-level decls, or sibling-module decls. That's surprising in member position and would re-introduce the noise Phase 19's deferral was meant to eliminate.
- Otherwise, fall through to the existing identifier-completion path.

**Acceptance:** integration test on `buildCompletionItems` (which already takes `prog`, `scope`, `line`, `col`) covering: in member position, returns member-only list; out of member position, returns the existing identifier set.

### 21.4 Advertise `.` as a completion trigger character

**Files:** `internal/lsp/server.go`

Change the initialize response from `CompletionProvider: &CompletionOptions{TriggerCharacters: nil, ResolveProvider: false}` to `TriggerCharacters: []string{"."}`. Without this, clients only request completion on characters they consider word-start by default; a literal `.` is *not* a word-start character in most LSP clients, so the user would have to type one more character before completion fires. With the trigger character set, the editor calls `textDocument/completion` the instant the dot is typed.

**Acceptance:** server initialize test asserts `.` is in `completionProvider.triggerCharacters`.

### 21.5 E2E test extension

**Files:** `internal/lsp/e2e_test.go`

Open a file containing `entity Foo { field x: Int; method bar() returns Int { return 0; } }` plus a function body that does `let f: Foo = Foo(); f.<cursor>`. Issue `textDocument/completion` at that position and assert the response contains `x` (Field) and `bar` (Method), and does *not* contain `if` or other keywords.

**Acceptance:** `go test ./internal/lsp/... -run E2E` passes including the new assertion.

### 21.6 Docs + ADR revision + roadmap

**Files:** `docs/decisions/0032-lsp-v1-surface.md`, `docs/decisions/README.md`, `docs/ROADMAP.md`, `INTENT.md`, `editors/vscode/README.md`, `ops/plans/phase-19-lsp-v1-completion.md`, `ops/plans/phase-21-lsp-member-completion.md`

- ADR 0032: add `## Revised — 2026-05-31 (Phase 21)` mentioning member completion landed and updating the status line.
- Decisions README: re-touch the 0032 row with the latest revision date.
- ROADMAP.md: add `### Phase 21: LSP Member Completion -- SHIPPED (2026-05-31)` under v1.2, and amend the Phase 19 SHIPPED block's "Member completion deferred to v1.2" note with "(landed Phase 21)".
- INTENT.md "Editor support" section: drop the member-completion caveat.
- VS Code README: add member completion to the feature list.
- Phase 19 PRD: append a brief "Followed up by Phase 21" line near the Out of Scope note.
- Phase 21 PRD: status `In Progress` → `Shipped (date)` with commit range.

**Acceptance:** `make validate` green. All cross-references consistent. No stale "v1.2" or "deferred" mentions of member completion.

## Out of Scope

- Chained member access (`a.b.c`). Resolving past the first hop requires expression-typing, which the LSP doesn't have today. Phase 19 already excluded this for hover/goto-def; Phase 21 stays consistent.
- Trait-method completion on a `dyn Trait` or generic-parameter receiver. The receiver-type resolver doesn't model traits.
- Static method / associated function completion via `EntityName.`. Not a real Intent construct today.
- Function-call result completion (`foo().bar`). Requires expression typing; out for the same reason as chained access.
- Snippet templates (`method(${1:arg})`). Plain identifiers stay the v1.2 surface.

## Suggested Order

1. 21.1 Member-context detection (small, pure function, fully unit-testable)
2. 21.2 Member item builder (reuses scope + symbol infrastructure)
3. 21.3 Wire into `handleCompletion`
4. 21.4 Trigger character (one-line server change + capability test)
5. 21.5 E2E extension
6. 21.6 Docs + ADR revision (last — locks the wire contract)
