# Intent for VS Code

Language support for the Intent contract-based programming language. Provides:

- Syntax highlighting for `.intent` files (keywords, types, annotations, strings, comments)
- Live diagnostics from the Intent compiler (parser, checker, lint, Z3 verification)
- Hover (types, signatures, contracts)
- Go-to-definition (same-file and same-package)

Powered by the `intentc lsp` server (see [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md)).

## Prerequisites

`intentc` must be installed and on `PATH`. The extension does not bundle the compiler — install it from https://github.com/lhaig/intent.

For verification diagnostics (the Z3-backed ones), `z3` must also be on `PATH`. If it isn't, parser/checker/lint diagnostics still work — only the verification layer degrades silently.

## Install (from source)

```bash
cd editors/vscode
npm install
npm run compile
npm run package      # produces intent-vscode-<version>.vsix
code --install-extension intent-vscode-<version>.vsix
```

Marketplace publishing is planned for v1.1; for v1 install from a locally-built `.vsix`.

## Settings

| Setting              | Default | Description                                                                                                                   |
| -------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `intent.binaryPath`  | `""`    | Override the path to the `intentc` binary. When empty, the extension looks up `intentc` on `PATH`.                            |
| `intent.lsp.trace`   | `"off"` | LSP protocol tracing. `"messages"` logs method names in the Output panel; `"verbose"` includes payloads. Useful for debugging. |

## Troubleshooting

**"Could not find 'intentc' on PATH"**
The extension can't locate the compiler. Install it, or set `intent.binaryPath` to its location.

**Hover or go-to-definition does nothing**
Place the cursor directly on an identifier (function call, entity name, etc.). V1 doesn't resolve methods, locals, or cross-package symbols — see the [LSP PRD](../../ops/plans/phase-18-lsp-server.md) for the v1 surface.

**Verification diagnostics absent**
Install `z3` and ensure it's on `PATH`. Save the file to trigger verification (Z3 only runs on save).

## V1 scope

See ADR 0032. Out of scope for v1: completion, find-references, rename, code actions, refactorings, formatting via LSP, semantic tokens, inlay hints, document/workspace symbols, signature help.
