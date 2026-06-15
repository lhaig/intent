# Phase 23: VS Code Marketplace Publishing

**Status:** Draft (operational — execution blocked on user-supplied credentials and a branded icon)
**Milestone:** Milestone 8 — Developer Experience (operational close-out)
**Decision:** No new ADR; this is operational follow-on from Phase 20 ([ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md) marked the engineering side publish-ready).

## Goal

Publish the Intent VS Code extension to the Visual Studio Marketplace so users can `code --install-extension` it (or install through the VS Code UI) without building from source. Phase 20 left the extension publish-ready *engineering-wise* — bundled `.vsix`, marketplace metadata in `package.json`, CHANGELOG, gallery banner, keywords, placeholder icon. The remaining blockers are operational (publisher account, Personal Access Token, branded icon) and one mechanical task (set up the publish workflow).

This PRD documents what needs to happen and in what order, so the user can drive it through end-to-end when they're ready to ship publicly. It is **not** code work — there is no Go change involved. The output is a publish recipe + a CI workflow + a few small repository changes.

## What "shipped" means

- The Intent extension is live at `https://marketplace.visualstudio.com/items?itemName=<publisher>.intent-vscode` (the URL is determined by the chosen publisher).
- `code --install-extension intent-lang.intent-vscode` (or whatever the publisher ends up as) succeeds without building from source.
- The extension page on the Marketplace shows the README, CHANGELOG, branded icon, and at least one screenshot demonstrating diagnostics / hover.
- A GitHub Actions workflow can publish a new version on a tagged release (Phase 23.5 — optional, can ship after the first manual publish).
- `editors/vscode/README.md` updates the "Install" section to lead with the marketplace install command rather than the from-source instructions.

## Success Criteria

- [ ] Publisher account created on <https://marketplace.visualstudio.com/manage>; `package.json` `publisher` field updated to match if it differs from the current `intent-lang` placeholder
- [ ] Personal Access Token (PAT) with `Marketplace > Acquire and manage` scope generated; `vsce login <publisher>` succeeds
- [ ] Branded 128×128 PNG icon replaces the placeholder at `editors/vscode/icon.png`. Source SVG (vector original) committed under `editors/vscode/icon.svg` for future re-rendering
- [ ] At least one screenshot saved under `editors/vscode/screenshots/` (PNG, 1280×720 or larger) and referenced from `editors/vscode/README.md`
- [ ] `vsce package` produces a valid `.vsix` without warnings (currently warns on missing icon contrast / readme images — both addressed)
- [ ] `vsce publish` (run manually the first time) successfully uploads the extension; marketplace listing renders correctly
- [ ] `editors/vscode/README.md` "Install" section updated to lead with `ext install <publisher>.intent-vscode` (and `code --install-extension`); from-source section moved below as "Build from source"
- [ ] Root `README.md` mentions the marketplace install option
- [ ] (Optional, future) GitHub Actions workflow at `.github/workflows/vscode-publish.yml` publishes on tagged release `editors/vscode/v*` using a repository secret `VSCE_PAT`
- [ ] `docs/ROADMAP.md` v1.2 entry adds Phase 23 SHIPPED with publish date

## Reference

- Existing extension: `editors/vscode/`
- Extension package metadata: `editors/vscode/package.json`
- CHANGELOG: `editors/vscode/CHANGELOG.md`
- Phase 20 PRD (where marketplace publish-readiness landed): `prds/done/phase-20-lsp-polish-production.md`
- ADR 0032 §Revised — 2026-05-31 (Phase 20): "Publishing is now blocked only on credentials (publisher account, PAT) and a branded icon — engineering side is publish-ready."
- VS Code marketplace publish docs: <https://code.visualstudio.com/api/working-with-extensions/publishing-extension>
- `vsce` CLI: <https://github.com/microsoft/vscode-vsce>

## Tasks

### 23.1 Publisher account + PAT

**Out of repo.** User work; nothing to commit until tasks below.

1. Create or claim a publisher at <https://marketplace.visualstudio.com/manage>. Common choices:
   - `intent-lang` (current placeholder; check availability)
   - `<your-github-username>` (simplest for solo maintainership)
   - `intent` (likely taken; check)
2. Note the resulting publisher ID — this is what `package.json` will use.
3. Generate a Personal Access Token at <https://dev.azure.com/<your-org>/_usersSettings/tokens>:
   - Organization: All accessible
   - Scopes (custom defined): **Marketplace > Acquire and manage**
   - Expiration: 1 year (renewable)
4. Save the PAT somewhere safe — you'll paste it once during `vsce login` and (later) into a GitHub Actions secret.

**Acceptance:** `vsce login <publisher>` succeeds locally (Personal Access Token accepted).

### 23.2 Update `package.json` publisher field

**Files:** `editors/vscode/package.json`

If the chosen publisher in 23.1 isn't `intent-lang`, update the `publisher` field to match. The `.vsix` filename will inherit from `name + version`, but the listing URL inherits from `publisher.name`.

**Acceptance:** `cd editors/vscode && npm run package` succeeds and the generated `.vsix` carries the expected publisher in its manifest. `vsce show <publisher>.intent-vscode` returns a 404 (extension not yet published) but doesn't error on the publisher field.

### 23.3 Branded icon

**Files:** `editors/vscode/icon.svg` (new — vector source), `editors/vscode/icon.png` (replace placeholder)

Design or commission an icon. Constraints from VS Code marketplace docs:

- 128×128 PNG, sRGB, transparent or solid background
- Recognisable at 32×32 (the size the marketplace search results render)
- Should evoke "contract / proof / verification" — possible directions: a stylised contract seal, a checkmark over a code bracket, the `⊢` (turnstile) symbol from formal logic
- Commit the SVG source alongside the PNG so future rebuilds don't depend on whoever made the original

**Acceptance:** Icon is visually distinct from the placeholder; `vsce package` produces a `.vsix` with no warnings about contrast or sizing.

### 23.4 Screenshot for the marketplace listing

**Files:** `editors/vscode/screenshots/diagnostics-hover.png` (new), `editors/vscode/README.md` (reference the image)

The marketplace listing pulls images from the README. Take at least one screenshot showing the extension's value in 5 seconds of scanning:

- Open `examples/bank_account.intent` in VS Code with the extension installed
- Trigger a deliberate type error (e.g., `let x: Int = "oops";`) so the squiggly underline is visible
- Hover over a contract-bearing function (`deposit` or `withdraw`) to show the `requires` / `ensures` popover
- Capture at 1280×720 or larger; crop to remove personal information from window chrome

Reference the image in `editors/vscode/README.md` near the feature list:

```markdown
![Diagnostics and contract hover](screenshots/diagnostics-hover.png)
```

**Acceptance:** Screenshot is committed; README renders it both on GitHub and (after publish) on the marketplace listing page.

### 23.5 First manual publish

**Out of repo.** One-time operational step.

```bash
cd editors/vscode
npm run package     # produces intent-vscode-0.1.0.vsix
vsce publish        # uploads to marketplace
```

Watch for the email confirmation from Microsoft (the marketplace queues the upload for malware/policy review before going live; usually completes within an hour for established publishers, longer for first-time publishers).

If `vsce publish` errors with "Make sure you have the required permissions...", the PAT scope is wrong — regenerate with `Marketplace > Acquire and manage`.

**Acceptance:** Marketplace listing URL returns 200; `code --install-extension <publisher>.intent-vscode` succeeds on a clean VS Code install.

### 23.6 Update install docs

**Files:** `editors/vscode/README.md`, root `README.md`

In `editors/vscode/README.md`, replace the "Install (from source)" section with two sections:

```markdown
## Install (recommended)

From the VS Code Extensions view (`Cmd+Shift+X`), search for "Intent" and click Install. Or from the command line:

`code --install-extension <publisher>.intent-vscode`

## Build from source

(existing instructions)
```

In root `README.md`, add a one-line mention near the "Editor support" section that the extension is installable from the marketplace.

**Acceptance:** Both READMEs render correctly on GitHub. Marketplace URL is correct.

### 23.7 (Optional — defer until first publish succeeds) CI publish workflow

**Files:** `.github/workflows/vscode-publish.yml` (new)

Trigger on tag push matching `editors/vscode/v*` (or a less Git-tag-heavy convention you prefer). The workflow:

1. Checks out the repo
2. Sets up Node
3. Runs `cd editors/vscode && npm ci && npm run package`
4. Runs `vsce publish -p ${{ secrets.VSCE_PAT }}`

The repository secret `VSCE_PAT` must be set in repo settings — same PAT as task 23.1.

This is **optional for v1.2** — manual publishes work fine while the project has one maintainer. Add the workflow once releases happen often enough that automating saves real time.

**Acceptance:** Pushing a tag like `editors/vscode/v0.2.0` triggers the workflow; new version appears on marketplace within an hour.

### 23.8 Status flip + roadmap

**Files:** `prds/backlog/phase-23-marketplace-publish.md`, `docs/ROADMAP.md`

Once the manual publish in 23.5 succeeds, flip this PRD's Status to `Shipped (date)` and add an entry to `docs/ROADMAP.md` under v1.2 referencing the marketplace URL.

## Out of Scope

- **Open VSX publish.** The Open VSX registry (used by VSCodium, Gitpod, etc.) is a separate publish target. Worth doing eventually but not part of this phase — the v1 audience is VS Code proper.
- **JetBrains, Vim, Emacs, Helix extensions.** Other editors can wire `intentc lsp` directly (it's plain LSP 3.17). Per-editor packaging is future work, not v1.2.
- **Pre-release / preview channels.** Microsoft supports `vsce publish --pre-release` for beta builds. Defer until there's a real cadence to support.
- **Telemetry.** No usage telemetry; not adding any. If we later want it, it's a separate ADR (privacy / opt-in / what gets sent).
- **Automated screenshot generation.** The screenshot is hand-taken once; automating it isn't worth the CI surface yet.

## Suggested Order

1. **23.1 Publisher + PAT** — operational, no repo changes
2. **23.3 Branded icon** — biggest visual quality lift; do before publish
3. **23.4 Screenshot** — quick once you have the extension installed locally
4. **23.2 `package.json` publisher field** — only if the chosen publisher differs from `intent-lang`
5. **23.5 First manual publish** — the actual shipping moment
6. **23.6 Update install docs** — switches the front-door instructions
7. **23.8 Status flip + roadmap** — close the phase
8. **23.7 CI workflow** — optional; add when manual publishes feel like friction
