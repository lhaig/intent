# Phase 18: LSP Server (v1)

**Status:** Shipped (2026-05-30; commits 0a4ca14..4999b06)
**Milestone:** Milestone 8 — Developer Experience
**Decision:** [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md)
**Deferred:** Completion, find-references, rename, code actions, refactorings, formatting-via-LSP, semantic tokens, inlay hints, document/workspace symbols, signature help, incremental re-check, multi-root workspaces, TCP transport, Marketplace publishing.

## Goal

Ship the v1 LSP surface locked in ADR 0032: diagnostics (parser/checker/lint/Z3 verification), hover (type + signature + contracts + verification summary), and go-to-definition (same-file + same-package). The server lives in a new `internal/lsp/` package and is invoked via `intentc lsp` over stdio. A first-party VS Code extension under `editors/vscode/` registers the language and spawns the server. Other editors get a one-paragraph README pointing at `intentc lsp`.

End-state: a developer who installs `intentc` and the .vsix extension opens an `.intent` file and immediately sees type/parse/lint errors underlined as they type, with hovers that show contracts and verification status, and `Go to Definition` that jumps to same-file and same-package declarations.

## Why now

End users have zero editor support today. Phase 16/17 made the language testable and gave the harness a mechanical-truth surface. The remaining barrier to "Intent is a real language someone might pick up" is editor integration. Without LSP, every `.intent` file is a wall of text whose errors only surface on CLI invocation. With it, the language meets users where modern developers actually work.

## Success Criteria

- [x] `intentc lsp` subcommand starts a stdio LSP 3.17 server (initialize → initialized → ... → shutdown → exit lifecycle works)
- [x] Parser, checker, and lint diagnostics publish on `textDocument/didChange` (debounced ~150ms per document) with correct severities (parser/checker = Error, lint = Warning)
- [x] Z3 verification runs on `textDocument/didSave` asynchronously; per-contract status surfaces as `Information` (verified) / `Information` (unverified, with diagnostic on the contract location) / `Hint` (timeout/error); absence of Z3 degrades gracefully (no verification diagnostics)
- [x] `textDocument/hover` returns markdown content combining: resolved type for the symbol under the cursor; for functions/methods, the full signature + `requires`/`ensures` clauses + cached verification summary; for entities, fields + invariants
- [x] `textDocument/definition` resolves identifiers to declarations within the same file and across files in the same package; cross-package and `extern function` jumps land on the local declaration site
- [x] Server handles single-file and multi-file workspaces (re-uses `IsMultiFile` / `NewModuleRegistry`)
- [x] VS Code extension under `editors/vscode/` activates on `.intent` files, spawns `intentc lsp`, surfaces a clear error if `intentc` is not on PATH; settings `intent.binaryPath` and `intent.lsp.trace` work
- [x] `.vsix` package builds with `npm run package`; install instructions in `editors/vscode/README.md`
- [x] End-to-end smoke test in `internal/lsp/` drives a scripted LSP session (initialize → open file with error → observe diagnostics → hover → goto-def → shutdown) without spawning a real editor
- [x] No regressions: `make validate` green
- [x] No new external Go dependencies (ADR 0002 stance preserved)
- [x] `docs/ROADMAP.md` Milestone 8 LSP entry marked complete; ADR 0032 status updated with the implementation commit
- [x] Phase 18 PRD Status flipped to `Shipped` with date

## Reference

- Design: `docs/decisions/0032-lsp-v1-surface.md`
- Existing pipeline entry points to reuse: `internal/parser/parser.go`, `internal/checker/checker.go`, `internal/linter/linter.go`, `internal/verify/verifier.go`
- Multi-file machinery: `internal/compiler/registry.go` (ModuleRegistry), `internal/compiler/target.go` (`IsMultiFile`)
- Diagnostic infra: `internal/diagnostic/`
- LSP spec: protocol version 3.17, https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/

## Tasks

### 18.1 LSP server scaffold + CLI subcommand

**Files:** `internal/lsp/` (new package — `server.go`, `transport.go`, `protocol.go`), `cmd/intentc/main.go`

Skeleton:

- `transport.go` — JSON-RPC 2.0 framing over `io.Reader` / `io.Writer`. Read `Content-Length: N\r\n\r\n` header + N bytes of body. Write the same on response. Hand-rolled; no external dep (ADR 0002).
- `protocol.go` — minimal Go types for the LSP messages the server actually handles. Don't model the whole spec; just `InitializeParams`, `InitializeResult`, `ServerCapabilities`, `TextDocumentIdentifier`, `TextDocumentItem`, `Position`, `Range`, `Location`, `Diagnostic`, `DiagnosticSeverity`, `Hover`, `MarkupContent`, etc. Use `json.RawMessage` for fields we don't touch.
- `server.go` — `Server` struct holding the document store, a context for cancellation, and the request dispatcher. `Run(in io.Reader, out io.Writer) error` is the entry point.
- Lifecycle handlers: `initialize`, `initialized`, `shutdown`, `exit`. Reply with the capabilities we actually support (textDocumentSync.Open/Change/Save/Close, hover, definition, publishDiagnostics).
- `intentc lsp` subcommand in `cmd/intentc/main.go`: takes no positional args, reads stdin, writes stdout. Add to the help text.

**Acceptance:**
- `go test ./internal/lsp/... -v` covers: jsonrpc framing roundtrip; initialize handshake returns expected capabilities; shutdown → exit closes the connection cleanly.
- `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./intentc lsp` returns a well-formed response (manually verifiable).

### 18.2 Document state + text sync

**Files:** `internal/lsp/document.go`, `internal/lsp/server.go`, `internal/lsp/document_test.go`

- `Document` struct: URI, current text, line offsets (computed lazily for position conversion), last parsed AST, last checker result, last lint diagnostics, last verification result (with a sequence number for race-protection on async verifier results).
- Server holds `map[DocumentURI]*Document` guarded by a mutex.
- Implement `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didSave`, `textDocument/didClose`. Use the LSP `Full` sync kind in v1 — each `didChange` carries the complete text. Avoids incremental sync bugs; matches ADR 0032 §O6 → A.
- Position conversion utility: LSP uses 0-indexed line + UTF-16 column; Intent compiler uses 1-indexed line/column treating bytes. Provide `(line, col) ↔ Position` helpers that handle this for ASCII (the common case). For now, assume ASCII inside identifiers; document the limitation. Multi-byte support deferred.

**Acceptance:**
- `go test ./internal/lsp/... -run Document` covers: open inserts into store; change replaces text; close removes entry; line offsets compute correctly for a multi-line input; position conversion round-trips for ASCII positions.

### 18.3 Diagnostics: parser + checker + lint

**Files:** `internal/lsp/diagnostics.go`, `internal/lsp/server.go`, `internal/lsp/diagnostics_test.go`

- On `didChange`, debounce ~150ms per document. Implementation: a per-document timer reset on every change; when it fires, re-parse + re-check + re-lint and publish.
- Translate `diagnostic.Diagnostic` (line, column, severity, message) to LSP `Diagnostic` (Range + Severity + Message). Severity mapping: `Error` → `1`, `Warning` → `2`. Range is a single-character range at the diagnostic position (we don't have spans today — file as follow-up).
- Publish via `textDocument/publishDiagnostics` notification.
- Parser errors clear before checker runs; checker errors clear before lint runs; final publication is the union.
- Always publish — including an empty array when the file is clean — so the client clears stale markers.

**Acceptance:**
- `go test ./internal/lsp/... -run Diagnostics` covers: parser error publishes Error severity at correct position; checker error publishes after parse-clean; lint warning publishes as Warning; clean file publishes empty diagnostics; debouncing collapses rapid edits to a single publish.

### 18.4 Z3 verification diagnostics (async, on save)

**Files:** `internal/lsp/verify.go`, `internal/lsp/server.go`, `internal/lsp/verify_test.go`

- On `didSave`, if Z3 is available (`exec.LookPath("z3")`), spawn a goroutine that runs `internal/verify` against the document's IR and publishes per-contract verification diagnostics when the result arrives.
- Per-document sequence counter: the goroutine captures the doc's current seq at launch; if a newer save has bumped seq before results arrive, discard them (don't publish stale data).
- Per-contract diagnostics: `verified` → no diagnostic; `unverified` → `Information` severity at the contract source location, message `unverified: <contract text>`; `error`/`timeout` → `Hint` severity with the verifier's error text.
- If Z3 is absent, do nothing — no diagnostic, no error to the client. The hover handler shows "verification: Z3 not available" in that case.
- Cancellation: when a new save arrives, the in-flight verifier's context is cancelled. Implementation: store a `context.CancelFunc` per document; replace + cancel on each new save.

**Acceptance:**
- `go test ./internal/lsp/... -run Verify` (uses `-short` skip when Z3 is not on PATH) covers: verified contract publishes nothing; unverified publishes Information at correct contract location; subsequent save cancels prior verifier; Z3-missing degrades silently.

### 18.5 Hover

**Files:** `internal/lsp/hover.go`, `internal/lsp/symbol.go`, `internal/lsp/hover_test.go`

- `textDocument/hover` handler. Resolve the AST node at the requested position by walking the cached AST and matching against `(line, col)`.
- Content composition (markdown):
  - Identifier of a function call → signature + `requires` + `ensures` from `FunctionDecl`. If verified, append `**Verification:** N/M contracts verified` line from the cached verifier result. If Z3 unavailable, append `**Verification:** Z3 not available`.
  - Identifier of a method call → same as function plus the entity name as context (e.g., `BankAccount.deposit(amount: Int) -> Void`).
  - Identifier of an entity → entity name + fields + invariants.
  - Identifier of a let-bound variable → resolved type from checker.
  - Identifier of an enum variant → enum name + variant signature.
  - `self` inside a method → `self: <Entity>`.
  - Default (cursor on a keyword, operator, or unresolvable position): no hover.
- All hovers wrap content in ```intent ``` fenced blocks so editors render with syntax highlighting.

**Acceptance:**
- `go test ./internal/lsp/... -run Hover` covers: hover on a function-call ident returns signature + contracts; hover on an entity ident returns fields + invariants; hover on a let-bound var returns type; hover on a keyword returns nil; verification summary line appears when verifier ran.

### 18.6 Go-to-definition (same-file + same-package)

**Files:** `internal/lsp/definition.go`, `internal/lsp/symbol.go`, `internal/lsp/definition_test.go`

- `textDocument/definition` handler. Resolve the identifier at the cursor to its declaration.
- Within the same file: walk the AST to find a `FunctionDecl`, `EntityDecl`, `EnumDecl`, `TraitDecl`, `MethodDecl`, `LetStmt`, or function `Param` whose name matches. Return its file URI + line/column as a `Location`.
- Cross-file within the same package: if `IsMultiFile` was true for the workspace, consult the `ModuleRegistry` to find the declaring file. The registry already does this work for the compiler; expose a lookup method if one doesn't exist.
- `extern function` calls jump to the local extern declaration (not the Rust crate). Same for trait method calls — jump to the trait declaration, not impls.
- Cross-package jumps deferred (per ADR 0032 §O4 → B); return `null` for those.

**Acceptance:**
- `go test ./internal/lsp/... -run Definition` covers: same-file function-call → declaration; same-file method-call → method declaration; same-file entity-ident in a type ref → entity decl; same-package function call → other file's decl; cross-package call → null; identifier with no resolution → null.

### 18.7 Workspace and multi-file handling

**Files:** `internal/lsp/workspace.go`, `internal/lsp/server.go`

- On `initialize`, capture `rootUri` from params. If absent (single-file edit outside a workspace), each open document is its own workspace.
- For each open document, determine workspace mode: if an `intent.toml` exists at or above the document's directory, multi-file mode applies; otherwise single-file.
- Cache the `ModuleRegistry` per workspace so cross-file resolution doesn't re-scan the directory on every keystroke. Invalidate the cache when an `intent.toml` changes or a file is added/removed (best-effort — full workspace invalidation on `didChangeWatchedFiles` is deferred; reload-on-restart is acceptable for v1).

**Acceptance:**
- `go test ./internal/lsp/... -run Workspace` covers: single-file workspace builds without a registry; multi-file workspace (with intent.toml) builds a registry; cached registry serves repeat lookups.

### 18.8 VS Code extension

**Files:** `editors/vscode/package.json`, `editors/vscode/src/extension.ts`, `editors/vscode/language-configuration.json`, `editors/vscode/syntaxes/intent.tmLanguage.json`, `editors/vscode/README.md`, `editors/vscode/.vscodeignore`, `editors/vscode/tsconfig.json`

- `package.json`: registers language ID `intent`, file extension `.intent`, activation event `onLanguage:intent`. Declares contributed settings `intent.binaryPath` (string, default empty) and `intent.lsp.trace` (enum: off | messages | verbose, default off). Dependency on `vscode-languageclient` (npm) — TypeScript extension dependency only; not a Go dependency.
- `extension.ts`: on activation, resolves `intentc` from `intent.binaryPath` or `PATH`; spawns `intentc lsp` over stdio via `LanguageClient`; on missing binary, shows an information-message with install instructions.
- `language-configuration.json`: basic bracket/comment configuration (`//` line, `/* */` block, `{}`, `[]`, `()`).
- `syntaxes/intent.tmLanguage.json`: minimal TextMate grammar for keyword highlighting — keep small; not the source of truth for diagnostics. Covers: keywords (`module`, `function`, `entity`, `enum`, `trait`, `impl`, `intent`, `goal`, `constraint`, `guarantee`, `verified_by`, `requires`, `ensures`, `invariant`, `let`, `mutable`, `if`, `else`, `return`, `while`, `for`, `in`, `break`, `continue`, `decreases`, `forall`, `exists`, `match`, `import`, `public`, `async`, `await`, `spawn`, `extern`, `test`, type names, `true`/`false`, `self`, `result`, `old`); string literals; line and block comments; numeric literals.
- `README.md`: install via `npm install && npm run package` to produce `intent-vscode-<version>.vsix`; install via `code --install-extension intent-vscode-<version>.vsix`.
- `.vscodeignore`: exclude build artifacts.

**Acceptance:**
- `cd editors/vscode && npm install && npm run package` produces a `.vsix` file.
- Manual smoke: install the .vsix in VS Code; open `examples/hello.intent`; observe syntax highlighting and a working hover.

### 18.9 End-to-end smoke test

**Files:** `internal/lsp/e2e_test.go`

In-process client that exercises the server through real LSP frames:

1. `initialize` → assert capabilities response.
2. `textDocument/didOpen` with a known-broken file → assert diagnostic published with expected position.
3. `textDocument/didChange` correcting the file → assert empty diagnostics published.
4. `textDocument/hover` on a function-call position → assert markdown content contains the signature.
5. `textDocument/definition` on a function-call → assert location matches the declaration line.
6. `shutdown` → `exit` → assert clean termination.

Use an in-memory pipe (`net.Pipe()` or `io.Pipe()`) for the transport — no external process spawned.

**Acceptance:**
- `go test ./internal/lsp/... -run E2E -v` passes deterministically.

### 18.10 Docs + ADR status flip + roadmap

**Files:** `docs/decisions/0032-lsp-v1-surface.md`, `docs/decisions/README.md`, `docs/ROADMAP.md`, `INTENT.md`, `editors/vscode/README.md`, `ops/plans/phase-18-lsp-server.md`

- ADR 0032 status: from `accepted (design); implementation pending Milestone 8` to `accepted; implemented in <commit>`.
- Decisions README index: update the row.
- `docs/ROADMAP.md` Milestone 8 LSP entry: mark complete with the commits.
- `INTENT.md` gains a short "Editor support" section pointing to `intentc lsp` and the VS Code extension.
- This PRD's Status flips from `Draft` to `Shipped (date)` with an Approval Record entry.

**Acceptance:**
- All docs consistent. `make validate` green.

## Out of Scope

- Completion, find-references, rename
- Code actions / quick fixes (would naturally pair with lint warnings — deferred)
- Refactorings (rename across files, extract function, etc.)
- Formatting via LSP (users use `intentc fmt` from the CLI; trivial to add later)
- Document/workspace symbols (outline view)
- Signature help (parameter hints during call)
- Semantic tokens (richer-than-tmLanguage highlighting)
- Inlay hints (type/parameter inlays)
- Incremental text sync (Full sync is good enough for v1)
- Multi-root workspaces
- TCP transport (stdio only)
- VS Code Marketplace publishing (v1.1 follow-up)
- First-party Neovim / Helix / JetBrains extensions (users wire `intentc lsp` themselves via their LSP clients)
- Cross-package go-to-definition (deferred to v1.1)
- Multi-byte UTF-16 column handling beyond ASCII (deferred — most `.intent` files are ASCII)
- `didChangeWatchedFiles` reactivity (workspace cache invalidates on restart)

## Suggested Order

1. **18.1** scaffold — unblocks everything; one commit
2. **18.2** document state — required by all message handlers
3. **18.3** parser + checker + lint diagnostics — biggest user-visible payoff per LOC
4. **18.7** workspace + multi-file — needed before 18.6 (cross-file goto-def)
5. **18.5** hover — same AST walker pattern; lets us validate position resolution before goto-def
6. **18.6** go-to-definition — reuses 18.5's symbol resolution
7. **18.4** Z3 verification — slowest, fully async, can land after the synchronous diagnostics are stable
8. **18.8** VS Code extension — once the server is functional, the extension is mostly wiring
9. **18.9** E2E smoke test — locks the wire format before declaring done
10. **18.10** docs + status flip

Land each task as its own atomic commit. Run `make validate` after each. The LSP package tests (`go test ./internal/lsp/...`) become the per-task acceptance gate.
