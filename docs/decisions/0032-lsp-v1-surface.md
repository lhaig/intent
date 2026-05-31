# 0032: LSP v1 Surface

**Date:** 2026-05-30
**Status:** accepted; revised four times (Phase 19, Phase 20, Phase 21, Phase 25); Phase 18 commits 0a4ca14..4999b06, Phase 19 commits 888f80b..257ccfe, Phase 20 commits addd43b..757abfe, Phase 21 commits 7c44883..HEAD, Phase 25 (test-only — see Revised section)
**Phase:** Milestone 8 (Developer Experience)

## Context

Intent has zero editor integration today. Users edit `.intent` files in whatever editor they pick and discover errors only when they run `intentc check`, `intentc lint`, or `intentc verify` from the command line. Every other modern compiled language ships at least a diagnostics-and-hover LSP within the first year because the cost of "no editor feedback" compounds: typos are caught on save instead of on keystroke, type errors surface as a wall of CLI output instead of an underline, and contract documentation lives in the docs folder rather than in the editor's hover popup.

The compiler's internals are already structured to support an LSP server cheaply. The pipeline produces structured diagnostics (`internal/diagnostic/`), every checked program carries resolved types on every expression, contracts survive lowering as first-class IR nodes, and `intentc verify` already classifies each contract as verified / unverified / error / timeout. A v1 LSP can be a thin shell over the existing pipeline — no compiler refactoring needed.

What's contested is the **scope** of v1. The LSP specification is large; shipping a maximal v1 trades calendar time against marginal benefit, and the smallest useful subset is dramatically smaller than the maximal subset. This ADR scopes v1, names what is deliberately deferred, and sets the editor extension model. No code is written until this ADR is approved.

Milestone 8 in the roadmap (`docs/ROADMAP.md`) is gated on this ADR. The downstream PRD will be `ops/plans/phase-18-lsp-server.md` (numbering tentative; Phase 18 currently holds Phase 17 deferrals — the LSP phase number may shift).

## Options

### O1. v1 feature surface

- **A. Diagnostics + hover + go-to-definition only.** The "MVP triple" — every modern LSP starts here. Diagnostics include checker errors, lint warnings, and parser errors; hover shows types and contracts; go-to-definition jumps to declaration. Everything else (completion, find-references, rename, code actions, formatting, signature help, document symbols, semantic tokens, inlay hints) is out.
- **B. MVP triple + completion + document symbols.** Adds two features that materially improve perceived quality but require non-trivial new infrastructure (a completion model and a symbol-tree extractor).
- **C. Maximal surface.** Diagnostics, hover, go-to-definition, completion, find-references, rename, code actions, formatting, signature help, document symbols, semantic tokens, inlay hints. Multi-quarter scope.

### O2. Diagnostics — which sources

- **A. Checker errors + parser errors only.** Hardest-blocking signals only.
- **B. Checker + parser + lint warnings.** Adds the existing `intentc lint` rules as `DiagnosticSeverity.Warning`.
- **C. Checker + parser + lint + verification status.** Z3 verification surfaces per-contract status as a hint or info-level diagnostic ("verified" / "unverified" / "timed out").

### O3. Hover content

- **A. Type only.** `let x = ...` hover shows the resolved type. Function call hover shows the signature.
- **B. Type + signature + contracts.** Hovering a function name shows its signature and `requires`/`ensures` clauses verbatim. Hovering a method on an entity adds the entity's `invariant`s.
- **C. Type + signature + contracts + verification status.** Adds a one-line summary derived from the cached Z3 results: "3/3 contracts verified" or "1/3 unverified — see contract: ...".

### O4. Go-to-definition scope

- **A. Functions, methods, fields, entities, enums, traits — declared in the same file.** Single-file scope only.
- **B. Same-file + same-package files.** Cross-file within one package via the existing module registry.
- **C. Full import graph.** Cross-package via `intent.toml` resolution; includes `extern function`s (jumps to the declaration, not the Rust crate).

### O5. Editor extension model

- **A. VS Code only, hand-written extension.** A `intent-vscode` extension that bundles the LSP binary, registers the language ID, and spawns the server. Other editors are deferred until a user asks.
- **B. VS Code + a generic `intentc lsp` subcommand documented for other editors.** Same extension, plus a documented protocol for users to wire the LSP into Neovim, Helix, Emacs, etc. via their LSP clients. No first-party extension for those editors.
- **C. First-party extensions for VS Code, Neovim, Helix, JetBrains.** Maximal coverage; each editor has its own quirks and maintenance burden.

### O6. Server-side scope: incremental or full re-check

- **A. Full re-check on every `didChange`.** Re-run lex → parse → check → IR on every keystroke (debounced to ~150ms). Simple, slow on large files.
- **B. Incremental file-scoped re-check; module-level invalidation.** Cache per-file ASTs and check results; re-lex and re-check only changed files, re-resolve cross-file references on demand. Faster but more state to manage and easier to get wrong.
- **C. Server-side compilation database.** A persistent on-disk cache keyed by file hash. Complex; only worth it for very large codebases.

### O7. Transport

- **A. stdio LSP.** The standard. Spawned per workspace by the client.
- **B. stdio with optional TCP fallback for debugging.** Standard plus a `--port` flag for `nc`-style debugging sessions.

## Decision

**O1 → A.** v1 ships the MVP triple: diagnostics, hover, go-to-definition. Rationale: this is the smallest surface where users feel a qualitative difference ("the editor catches my errors"). Each additional feature (completion, rename, etc.) is a real engineering investment that should only land after the triple is in users' hands and they tell us what's missing. Shipping the triple unblocks the editor story for everyone; shipping a maximal v1 risks shipping nothing for two more quarters.

**O2 → C.** Diagnostics ship parser errors, checker errors, lint warnings, and verification status. Rationale: the pipeline already computes all four. The marginal cost of plumbing them through LSP is small. Lint warnings without LSP are nearly invisible (users have to run `intentc lint` manually); verification status without LSP is invisible by definition. Surfacing both is the highest-leverage feature of the entire LSP and shipping diagnostics without them sells the LSP short. Severity mapping: parser/checker → `Error`, lint → `Warning`, verification "unverified" → `Information`, verification "timeout"/"error" → `Hint` with a code action to re-run with a higher timeout. Verification runs on save (not on every keystroke) because Z3 invocation is slow.

**O3 → C.** Hover shows type + signature + full contracts + verification summary. Rationale: this is Intent's headline feature. The whole point of a `requires`/`ensures` block is that the caller sees it; an IDE hover is the natural surface. Without contracts in hover, users have to jump to the definition to read them, which defeats the value of contract-driven design. Verification status is a one-line addendum cheap to add once contracts are already in the hover payload.

**O4 → B.** Same-file plus same-package. Rationale: the module registry already resolves cross-file references within a package (`internal/compiler/` orchestrates this for multi-file builds). Wiring the LSP to use the same resolution is a small additional step. Full cross-package import-graph resolution is deferrable: most jumps a user wants are within the package they're editing; package-boundary jumps can land in v1.1 when package use is more established. `extern function` definitions resolve to the Intent-side `extern function` declaration only — we do not jump into Rust source.

**O5 → B.** VS Code first-party extension plus a documented `intentc lsp` subcommand for other editors. Rationale: VS Code is where 60-70% of users live in 2026, and a Microsoft-quality extension is a one-time cost. Other editors have capable LSP clients of their own; users of those editors are well-served by a CLI subcommand plus a one-paragraph README. First-party extensions for Neovim/Helix/JetBrains can come later if users on those editors materialise. The extension model: `intentc lsp` starts a stdio LSP server; the VS Code extension registers the language ID `intent`, activates on `.intent` files, and spawns `intentc lsp` as the language server. No bundled binary inside the extension — the extension assumes `intentc` is on `PATH` (with a clear error if not).

**O6 → A.** Full re-check on every `didChange`, debounced ~150ms. Rationale: Intent files are small (typical `.intent` files are <500 lines). The lexer + parser + checker pipeline runs in tens of milliseconds even on the largest example. Premature incremental optimisation is the most common LSP pitfall and the source of half the bugs in mature LSPs. Start simple; profile when (and if) a user hits a real performance wall; revisit then. Z3 verification is *not* re-run on `didChange` — it runs on `didSave` only, and emits diagnostics asynchronously when results arrive.

**O7 → A.** stdio only. Rationale: every LSP client supports stdio; TCP is a debugging convenience that has never been worth the protocol surface in mature LSPs. If we hit a debugging wall, add it then.

### Summary table

| Feature | v1 | Deferred |
|---|---|---|
| Diagnostics (parser, checker, lint, verification) | yes | — |
| Hover (type, signature, contracts, verification summary) | yes | — |
| Go-to-definition (same-file + same-package) | yes | — |
| Cross-package go-to-definition | — | yes (v1.1) |
| Completion (identifiers, methods, fields) | — | yes (v1.1) |
| Find references | — | yes (v1.1) |
| Rename | — | yes |
| Code actions (quick fixes) | — | yes |
| Document symbols (outline view) | — | yes (v1.1) |
| Signature help (param tooltips) | — | yes (v1.1) |
| Semantic tokens (richer highlighting) | — | yes |
| Inlay hints | — | yes |
| Formatting via LSP | — | yes (use `intentc fmt` directly) |
| Refactorings | — | yes |
| Workspace symbols | — | yes |
| Server transport | stdio | TCP debug mode |
| Re-check model | full on `didChange` (debounced) | incremental |
| Editors with first-party extensions | VS Code | Neovim, Helix, JetBrains |

### Editor extension model

- The LSP server ships as `intentc lsp` — a subcommand of the existing CLI binary. No new binary, no new install path.
- The VS Code extension lives in `editors/vscode/` inside this repo (new directory). Published to the VS Code Marketplace as `intent-vscode`. Activates on `.intent` files. Spawns `intentc lsp` over stdio.
- The extension does not bundle the `intentc` binary. Users must have `intentc` on `PATH`. If absent, the extension shows a clear error pointing to the install instructions. Rationale: avoids binary-distribution complexity (the extension would otherwise need per-platform builds matching every supported target); keeps the extension small; matches `rust-analyzer`'s model where the toolchain ships separately from the editor extension.
- Settings the extension exposes (v1): `intent.binaryPath` (override the `intentc` lookup), `intent.lsp.trace` (off | messages | verbose) for debugging.
- Other editors get a one-paragraph README entry: "Wire `intentc lsp` into your LSP client as the server for `.intent` files."

### Compatibility

- LSP protocol version 3.17 (current). No backwards compatibility with older LSP clients required — modern editors all support 3.17 in 2026.
- The `intentc lsp` subcommand starts with no positional args, reads from stdin, writes to stdout per the LSP spec.
- Server lifecycle follows standard LSP: `initialize` → `initialized` → file open/edit/save/close cycles → `shutdown` → `exit`.

## Consequences

**Accepted trade-offs:**

- Users on Neovim/Helix/Emacs/JetBrains will see a less polished experience than VS Code in v1. Mitigation: the `intentc lsp` subcommand is documented; their existing LSP clients should work with it; community-contributed editor extensions are welcome.
- Completion is the single most-requested LSP feature in user-research surveys, and shipping without it is a notable omission. Mitigation: ship v1 fast, learn what users actually do, and prioritise completion in v1.1 with concrete user feedback rather than speculation.
- Full re-check on `didChange` will become a problem on Intent files past ~5000 lines. Mitigation: profile when this happens; the incremental machinery is a known follow-up.
- Verification status in hover requires running Z3 on save. If Z3 is not installed, the verification line is omitted (matches the existing `intentc verify` graceful-degradation pattern). The hover does not block on Z3 — verification is asynchronous; the hover shows "verification pending" or the latest cached result.

**Out of scope, explicitly:**

- **Refactorings** (rename across files, extract function, inline, change signature). Each refactoring is a substantial design exercise; v1 punts.
- **Code actions / quick fixes.** Tempting (especially for lint warnings like "remove unused variable"), but every code action is an explicit code-mutation contract that must be designed carefully. Deferred.
- **Formatting via LSP.** Users have `intentc fmt` as a CLI; integrating the formatter as an LSP `textDocument/formatting` provider is straightforward but uninteresting — punt until VS Code users ask. (When they do, the implementation is trivial because `internal/formatter` already produces canonical output.)
- **Workspace symbols, semantic tokens, inlay hints.** All "nice to have" features; each adds protocol surface and server-side state. Deferred.
- **Multi-root workspaces.** Single-workspace only in v1. VS Code's multi-root feature surfaces as multiple `initialize` requests; v1 treats each as a fresh workspace.
- **Cancellation tokens for long-running checker / verifier runs.** v1 fire-and-forget; if the checker is still running when a new `didChange` arrives, the old result is discarded. Stand-up risk: low (checker is fast).
- **Snippets, completion item details, completion documentation.** All depend on completion existing.

**Server architecture:**

- New package: `internal/lsp/`. Owns the LSP protocol handling (jsonrpc framing, request/response routing) and integrates with the existing compiler pipeline.
- The server holds a workspace state: open documents (by URI), their parsed AST, their checker result, their lint diagnostics, their last verification result. Re-check rebuilds this state for the affected file.
- No external Go LSP library — the protocol surface is small enough that a hand-written jsonrpc loop is preferable to a dependency. Existing pattern: ADR 0002 (Go toolchain, zero external dependencies) applies. Re-evaluate if the protocol surface grows past the v1 set.

**Editor extension architecture:**

- VS Code extension is TypeScript; uses `vscode-languageclient` to wire the LSP. Standard pattern.
- The extension package is small (~50 lines of TypeScript) because all language smarts live in `intentc lsp`. The extension is essentially a registration shim.
- CI builds the `.vsix` on each release; publish via `vsce publish` from a release script.

**Relationship to other ADRs:**

- ADR 0002 (Go toolchain, no external dependencies): preserved. LSP server is hand-written jsonrpc; no Go LSP library.
- ADR 0003 (runtime assertions over static proofs): the LSP surfaces verification status, which is the only Z3 user-facing surface today. The LSP makes this surface visible per-keystroke instead of per-CLI-run.
- ADR 0008 (IR): the LSP consumes the AST and checker result, not the IR. Hover content comes from the checker's resolved types; verification status comes from the verifier's cached results. The IR is irrelevant to LSP.
- ADR 0014 (legacy codegen removed): no impact.
- ADR 0029 (in-language testing): tests are a top-level declaration. LSP go-to-definition jumps to test declarations the same way it jumps to functions.
- ADR 0030, 0031 (testing polish): no impact; tests are syntactically similar to functions for navigation purposes.

**Follow-up work expected:**

- `editors/vscode/` directory layout, build, publish flow — defined in the implementation PRD.
- A PRD-per-deferred-feature when each is prioritised: completion, find-references, rename, code actions, formatting-via-LSP.
- A `protocol.md` reference documenting which LSP requests/notifications the server handles, for users wiring it into non-VS Code editors.
- An end-to-end smoke test: spawn `intentc lsp`, open a file, edit it, observe diagnostics arrive — pre-commit hook for the LSP package once it lands.

## Revised — 2026-05-30 (Phase 19)

Phase 18 shipped the MVP triple (diagnostics, hover, goto-def) but limited each to a narrow surface — hover and goto-def only resolved top-level declaration names, the outline view was empty, "Format Document" returned "no formatter available," and there was no completion or signature help. Use-testing showed those deferrals made the LSP feel half-built in real code. Phase 19 expands v1 (commits 888f80b..257ccfe).

**Now in v1:**

- **Hover and goto-def** resolve locals, parameters, `self` inside methods/constructors, entity fields, and entity methods, in addition to the Phase 18 top-level decls. Member access like `account.deposit()` and `self.balance` works for single-step receivers.
- **`textDocument/documentSymbol`** returns the outline tree — top-level functions / entities (with field+constructor+method children) / enums (with variants) / traits (with method signatures) / tests / extern functions.
- **`textDocument/formatting`** runs `internal/formatter` and returns a single TextEdit replacing the document. Idempotent files produce no edits; unparseable files produce no edits (parse-error diagnostics already cover that case).
- **`textDocument/signatureHelp`** returns parameter info for the enclosing call. Handles bare functions, extern functions, and single-step method calls (`receiver.method`). Active parameter index tracks comma count.
- **`textDocument/completion`** returns identifier suggestions: in-scope locals/params + own program's top-level decls + sibling modules' decls + Intent keywords + built-in type names.

**Still out of v1, deferred:**

- Member completion (`.field` / `.method` after `.`) — needs cursor-context receiver-type resolution; lands in v1.2 with chained-receiver hover/goto-def.
- Find references, rename, code actions, refactorings — each requires its own design pass.
- Cross-package go-to-definition — same-package only stays in v1.
- Semantic tokens, inlay hints — polish, not capability.
- Marketplace publishing — distribution; v1.1.
- Per-contract Z3 source positions — verify diagnostics still anchor at (1,1); threading positions through `internal/verify` is its own piece. (Landed in Phase 24 / ADR 0034.)
- Incremental text sync, multi-byte UTF-16, multi-root workspaces, TCP transport, `didChangeWatchedFiles` — unchanged from the original deferred list.

**Implementation note:** the scope walker in `internal/lsp/scope.go` uses "find enclosing function/method body, then check params + lets before the cursor" rather than a full scope-stack walker. Trade-off documented in the file: lets inside inner blocks remain visible everywhere later in the function, not just within their block. Covers the common case (locals declared at function top, used throughout); v1.1 can tighten this when a real example exposes the limitation.

## Revised — 2026-05-31 (Phase 20)

Phase 20 adds polish + semantic tokens to v1. After Phase 19 the LSP was functionally complete but the editor experience was rough — function calls didn't render distinctly from variables (TextMate regexes can't tell), there was no outline integration, no format command, no way to see if the server was alive, and the `.vsix` shipped a raw npm tree. Phase 20 closes those gaps (commits addd43b..757abfe).

**Now in v1:**

- **Semantic tokens** (`textDocument/semanticTokens/full`) — server walks the AST and emits per-identifier kind: function, method, parameter, variable, property, class, enum, enumMember, interface, decorator. Modifiers: declaration, async, defaultLibrary. Built-ins (`print`, `len`, `assert*`, `Ok`, `Err`, `Some`, `None`, etc.) carry the defaultLibrary modifier. Themes pick up the kinds and color accordingly.
- **Tier-1 TextMate grammar** — function definitions/calls, method calls, field access, built-in functions, string interpolation. Acts as the offline fallback (when LSP isn't connected) and the bedrock that semantic tokens layer on top of.
- **VS Code extension polish**: esbuild bundling (.vsix is 90 KB single-file vs the previous 300 KB tree), status bar item showing server state (running / starting / stopped / error), `Intent: Restart Server` and `Intent: Show Output` commands.
- **Marketplace metadata**: CHANGELOG.md, 128×128 placeholder icon, gallery banner color, keywords. Publishing is now blocked only on credentials (publisher account, PAT) and a branded icon — engineering side is publish-ready.

**Still out of v1, deferred to v1.1+:** member completion (`.field`/`.method` after `.`), find-references, rename, code actions, refactorings, Marketplace publish (credentials), incremental semantic tokens (`/delta` and `/range`), multi-byte UTF-16, multi-root workspaces.

## Revised — 2026-05-31 (Phase 21)

Phase 21 closes the only outstanding v1.1 capability gap: member completion. Phase 19 had deferred it under the assumption that "cursor receiver-type resolution" was a separate piece of work; in practice that infrastructure already shipped in Phase 19's `internal/lsp/symbol.go` (`receiverBeforeMember`, `resolveMemberOnReceiver`) so hover and goto-def could resolve `expr.field` / `expr.method`. Phase 21 reuses it.

**Now in v1:**

- **`textDocument/completion` in member position** — when the cursor follows `<receiver>.` (optionally with a partial member identifier already typed), the response is the receiver entity's fields + methods only. `CompletionField` items carry the declared type as detail; `CompletionMethod` items carry a `(<params>) -> <return>` signature.
- **`.` advertised as a completion trigger character** — LSP clients fire `textDocument/completion` the instant the user types the dot, without waiting for a word-start character.
- Receivers resolved through scope: typed locals (`let x: Entity`), typed parameters, and `self` inside methods and constructors. Impl-block methods on the entity are included alongside the entity's own methods.
- Unresolvable receivers (untyped, non-entity type, chained access, call-result chains) return an empty list rather than the full identifier set — surfacing keywords after `.` is worse noise than nothing.

**Still out of v1, deferred (unchanged):** find-references, rename, code actions, refactorings, Marketplace publish (credentials), incremental semantic tokens, multi-byte UTF-16, multi-root workspaces, and now also chained member access (`a.b.c`) and trait-method completion on `dyn`/generic receivers (both need richer expression typing than the LSP carries today).

## Revised — 2026-05-31 (Phase 25)

Phase 25's regression test confirmed that **cross-package goto-definition already works** — the Phase 19/20 "deferred" claim was wrong. The workspace surface (`internal/lsp/workspace.go` `siblingModules`) invokes `compiler.ModuleRegistry.AllModules()`, which transitively discovers every module reachable through `intent.toml`'s `[dependencies]`. The resolver in `internal/lsp/symbol.go` (`resolveAcrossWorkspace`) iterates them all without gating on package boundary. Cross-package goto-def fell out for free from the Phase 19 workspace plumbing; the deferral was a planning artefact from before that work landed.

Phase 25 adds a regression test (`TestDefinitionCrossPackage`) so the behaviour stays correct, and updates this ADR's deferred list to drop the stale claim. No resolver or workspace code changed — only test + docs.
