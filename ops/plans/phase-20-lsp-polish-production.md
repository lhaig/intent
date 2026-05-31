# Phase 20: LSP Polish + Production-Ready Extension

**Status:** Draft
**Milestone:** Milestone 8 — Developer Experience (Phase 19 follow-on)
**Decision:** [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md) (revised again in this phase to include semantic tokens)

## Goal

Take the working-but-rough LSP + VS Code extension from Phase 18/19 to "this looks like a real production language extension you'd find on the Marketplace." Three thrusts:

1. **Tier 1 TextMate grammar** — function names, method calls, field access, built-in functions, string interpolation expressions render distinctly from plain identifiers. The current grammar paints everything past keyword/type/string/comment the same color, which makes function bodies look flat.

2. **Semantic tokens** — `textDocument/semanticTokens/full` provider in the LSP. The server walks the AST + checker results and tells the editor *which kind of thing* each identifier is — function vs variable vs method vs parameter vs entity vs trait vs enum variant. Beats TextMate's regex limits because it uses the actual resolved types. This is where rust-analyzer / typescript get their nice coloring.

3. **Extension polish** — esbuild bundling (50KB single file vs 300KB tree of npm modules), status bar showing server health, "Intent: Restart Server" command, CHANGELOG.md, icon placeholder, publisher placeholder. After this, publishing to Marketplace is a credential + icon-design task on the user's side.

Out of scope: the actual marketplace publish (separate PRD when the user has a publisher account + PAT + icon designed).

## Success Criteria

- [ ] TextMate grammar highlights: function definitions, function calls, method calls, field access, built-in functions (`print`/`len`/`assert`/`Ok`/`Err`/`Some`/`None`), string interpolation expression bodies
- [ ] `textDocument/semanticTokens/full` capability advertised and implemented
- [ ] Server emits tokens for: functions, methods, parameters, locals, entity names, enum variants, trait names, fields, annotations, async modifier, default-library modifier on built-ins
- [ ] VS Code extension picks up semantic tokens automatically (vscode-languageclient handles the negotiation)
- [ ] Extension bundles with esbuild — `.vsix` is single-file `extension.js`, ~50KB or less
- [ ] Status bar item shows "Intent ✓" when server is up, "Intent ⚠" when down
- [ ] "Intent: Restart Server" command appears in Command Palette
- [ ] `editors/vscode/CHANGELOG.md` documents 0.1.0
- [ ] `editors/vscode/icon.png` exists (128×128 placeholder — user replaces with branded version)
- [ ] `package.json` carries marketplace metadata: galleryBanner color, keywords
- [ ] All Phase 18/19 LSP tests still pass; new tests cover the semantic-tokens walker
- [ ] ADR 0032 revised again to include semantic tokens in v1 surface
- [ ] ROADMAP + INTENT.md reflect new state
- [ ] Phase 20 PRD status flipped to `Shipped`

## Tasks

### 20.1 Tier 1 TextMate grammar

**Files:** `editors/vscode/syntaxes/intent.tmLanguage.json`

Add patterns for:
- Function definition heads: `function NAME` / `entry function NAME` / `async function NAME` / `public function NAME` — capture NAME as `entity.name.function.intent`
- Function call sites: `NAME(` — capture NAME as `entity.name.function.intent`
- Method calls: `.NAME(` — capture NAME as `entity.name.function.method.intent`
- Field access: `.NAME` (not followed by `(`) — capture NAME as `variable.other.property.intent`
- Built-in functions: keyword-style for `print`, `len`, `assert`, `assert_eq`, `assert_close`, `assert_panics`, `Ok`, `Err`, `Some`, `None`, `await_all`, `await_any`, `timeout`, `sleep`
- String interpolation: inside `"..."`, the `{...}` substring's contents are scoped as expression (and would re-apply the language patterns recursively in a full implementation; v1 just colors the braces and embedded identifiers distinctly)
- Annotation arguments: inside `@name(...)`, string args are still strings (already handled)

**Acceptance:**
- Open `examples/bank_account.intent` — function names, method calls, and field accesses visibly distinct from plain identifiers
- `intentc fmt --check` over examples still clean (grammar shouldn't affect compiler)

### 20.2 Semantic tokens (LSP server)

**Files:** `internal/lsp/semantic_tokens.go` (new), `internal/lsp/semantic_tokens_test.go` (new), `internal/lsp/server.go`, `internal/lsp/protocol.go`

Approach:
- Define `SemanticTokensLegend` with the token types/modifiers we emit
- Server advertises `semanticTokensProvider` in initialize result
- New handler `textDocument/semanticTokens/full` walks the AST and produces a flat `[]uint32` of delta-encoded tokens per LSP spec
- The walker collects tokens in source order, then converts to delta form

Token types emitted:
- `function` — function names at decl and call sites
- `method` — method names at decl and call sites
- `parameter` — function/method/constructor params
- `variable` — let-bindings
- `property` — entity fields
- `class` — entity names
- `enum` — enum names
- `enumMember` — enum variants
- `interface` — trait names
- `decorator` — annotations (@target_specific)

Token modifiers:
- `declaration` — when the token is the decl site (vs a use site)
- `async` — async functions
- `defaultLibrary` — built-in functions (print, len, assert*, Ok, Err, Some, None)

**Acceptance:**
- `go test ./internal/lsp/... -run SemanticTokens` covers: walker emits one token per declaration; call sites get function tokens; method calls get method tokens; entity names get class tokens; deltas encode correctly
- Manual smoke: open an .intent file in VS Code with semantic-token-aware theme; function calls, method calls, locals visibly distinct

### 20.3 esbuild bundling

**Files:** `editors/vscode/package.json`, `editors/vscode/esbuild.js` (new), `editors/vscode/.vscodeignore`

- Add `esbuild` as a devDependency
- New `esbuild.js` script that bundles `src/extension.ts` into `out/extension.js` as a single CommonJS file, externalizing `vscode` (provided at runtime by VS Code) and including all other deps
- Update `package.json` scripts: `compile` → esbuild build, `package` → esbuild + vsce package
- Update `.vscodeignore` to drop `node_modules/**` again (esbuild inlines everything) and trim unused
- Verify the .vsix is single-file and ~50KB

**Acceptance:**
- `npm run package` produces a .vsix without bundled node_modules
- Installed .vsix activates and works identically to the un-bundled version

### 20.4 Status bar + Restart command

**Files:** `editors/vscode/src/extension.ts`, `editors/vscode/package.json`

- StatusBarItem with text "Intent ✓" when client is running, "Intent ⚠" when not, "Intent ⟳" while starting
- Click handler shows the "Intent" output channel
- Command `intent.restartServer` registered in `contributes.commands`, stops and re-spawns the language client
- "Intent: Restart Server" appears in the Command Palette

**Acceptance:**
- Open VS Code with `.intent` file → bottom-right shows "Intent ✓"
- `Cmd+Shift+P` → "Intent: Restart Server" → status flips to ⟳ → ✓ (server restarts, diagnostics re-publish)

### 20.5 Marketplace metadata

**Files:** `editors/vscode/CHANGELOG.md` (new), `editors/vscode/icon.png` (placeholder, 128×128), `editors/vscode/package.json`

- CHANGELOG.md with a 0.1.0 entry describing the v1 surface
- icon.png — generate a simple placeholder (solid color + "I" letter) so the marketplace listing renders something. User replaces with branded icon later.
- package.json gains: `icon: "icon.png"`, `galleryBanner: { color: "...", theme: "dark" }`, expanded `keywords`
- Note in README to set up a Microsoft publisher account before `vsce publish`

**Acceptance:**
- `vsce package` produces a .vsix that opens to show the icon in VS Code's Extensions panel
- README points at the publisher-account requirement

### 20.6 Docs + ADR revision + roadmap

**Files:** `docs/decisions/0032-lsp-v1-surface.md`, `docs/decisions/README.md`, `docs/ROADMAP.md`, `INTENT.md`, `editors/vscode/README.md`, this PRD

- ADR 0032: third revision section (Phase 20) noting semantic tokens are now in scope and shipped
- Decisions README: update status line
- ROADMAP: Phase 20 entry, Milestone 8 LSP entry updated
- INTENT.md "Editor support" section updated
- VS Code README: status bar, restart command, semantic tokens listed
- Flip Phase 20 PRD status to Shipped

## Out of Scope

- Actual marketplace publish — separate PRD when publisher account + PAT + final icon are ready
- Open-VSX publish — same constraint, separate PRD
- First-party Neovim / Helix / JetBrains extensions
- Token-modifier-aware themes — users pick their preferred theme; we just emit the tokens
- Incremental semantic tokens (`/delta`) — `/full` only in v1
- Range-scoped semantic tokens (`/range`) — `/full` only in v1
- `themes/` contributions — not shipping a custom Intent theme; existing themes pick up our tokens
- Find-references, rename, code actions, refactorings, cross-package goto-def, member completion — Phase 19's deferrals stay deferred (each gets its own PRD)
- Per-contract Z3 diagnostic positions — still anchored at (1,1) until a separate PRD threads positions through `internal/verify`

## Suggested Order

1. **20.1** TextMate grammar — independent, biggest immediate visual improvement
2. **20.2** Semantic tokens — bigger task but the headline feature
3. **20.3** esbuild bundling — quick win; do before status bar so the extension is clean by the time we touch it again
4. **20.4** Status bar + restart command
5. **20.5** Marketplace metadata
6. **20.6** Docs + ADR revision
