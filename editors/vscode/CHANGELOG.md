# Change Log

All notable changes to the Intent VS Code extension are documented in this file.

## 0.1.0 — 2026-05-31

Initial public release. v1 LSP feature surface per [ADR 0032](https://github.com/lhaig/intent/blob/main/docs/decisions/0032-lsp-v1-surface.md):

### Added

- Live diagnostics from the Intent compiler: parser errors, type-checker errors, lint warnings, and Z3 verification status. Verification runs asynchronously on save.
- Hover on functions, methods, entities, enums, traits, locals, parameters, `self`, fields, and field/method receivers. Hover surfaces the full `requires` / `ensures` contracts.
- Go-to-definition for top-level declarations and for locals, parameters, methods, and fields within the same file or same package.
- Document outline (`Cmd+Shift+O`) — functions, entities (with fields/constructors/methods nested), enums (with variants), traits.
- Format Document via `intentc fmt`.
- Signature help with active-parameter tracking for function and single-step method calls.
- Identifier completion: in-scope locals + top-level declarations + sibling-package declarations + Intent keywords + built-in type names.
- Semantic tokens — type-aware highlighting that distinguishes functions, methods, parameters, locals, properties, classes, enums, enum variants, traits, and built-in functions. Themes pick up the kinds and colour them according to user preference.
- TextMate grammar for offline (non-LSP) highlighting of keywords, types, strings, numbers, annotations, function definitions/calls, method calls, field access, and built-ins.
- Status bar item showing server state — running, starting, stopped, or error. Click to view logs.
- Commands: `Intent: Restart Server`, `Intent: Show Output`.
- Settings: `intent.binaryPath`, `intent.lsp.trace`.

### Known limitations (deferred to v1.1+)

- Member completion (`.field` / `.method` after `.`) returns the full identifier list rather than narrowing to receiver type.
- Cross-package go-to-definition returns null.
- Find references, rename, code actions, refactorings, inlay hints are not implemented.
- Z3 verification diagnostics anchor at line 1, column 1 (verifier results don't carry source positions yet).
- Multi-byte UTF-16 column handling: ASCII-only. Identifiers with non-ASCII characters may misalign.

### Requirements

- `intentc` on `PATH` (or set `intent.binaryPath`). Install from <https://github.com/lhaig/intent>.
- For verification diagnostics: `z3` on `PATH`. Without it, parser/checker/lint diagnostics still work; only the verification layer degrades silently.
