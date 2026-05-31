# Intent for VS Code

Language support for the Intent contract-based programming language. Provides:

- Syntax highlighting for `.intent` files (keywords, types, annotations, strings, comments)
- Live diagnostics from the Intent compiler (parser, checker, lint, Z3 verification)
- Hover on top-level decls, locals, params, `self`, fields, and methods (signature + contracts + type info)
- Go-to-definition: same-file and same-package, including locals/params/methods/fields
- Document symbols (outline view) with entity members and enum variants nested under their parents
- Format Document via `intentc fmt`
- Signature help with active-parameter tracking
- Identifier completion (in-scope locals + top-level decls + sibling-package decls + keywords + built-in types)

Powered by the `intentc lsp` server (see [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md)).

## Prerequisites

`intentc` must be installed and on `PATH`. The extension does not bundle the compiler — install it from https://github.com/lhaig/intent.

For verification diagnostics (the Z3-backed ones), `z3` must also be on `PATH`. If it isn't, parser/checker/lint diagnostics still work — only the verification layer degrades silently.

## Install (from source)

```bash
cd editors/vscode
npm install
npm run package      # esbuild + vsce → intent-vscode-<version>.vsix (~90 KB)
code --install-extension intent-vscode-<version>.vsix --force
```

The `.vsix` is a single-file esbuild bundle (no `node_modules` tree).

## Marketplace publishing (todo)

Before `vsce publish` will work this extension needs:

1. A Microsoft publisher account at <https://marketplace.visualstudio.com/manage>. The `publisher` field in `package.json` (currently `intent-lang`) must match.
2. A Personal Access Token with "Marketplace > Acquire and manage" scope (`vsce login <publisher>`).
3. A branded icon to replace `icon.png` (currently a placeholder).

Once those are in place, publishing is `vsce publish` from this directory.

## Settings

| Setting              | Default | Description                                                                                                                   |
| -------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `intent.binaryPath`  | `""`    | Override the path to the `intentc` binary. When empty, the extension looks up `intentc` on `PATH`.                            |
| `intent.lsp.trace`   | `"off"` | LSP protocol tracing. `"messages"` logs method names in the Output panel; `"verbose"` includes payloads. Useful for debugging. |

## Troubleshooting

**"Could not find 'intentc' on PATH"**
The extension can't locate the compiler. Install it, or set `intent.binaryPath` to its location.

**Hover or go-to-definition does nothing**
Place the cursor directly on an identifier (function call, entity name, local, parameter, field, method, etc.). V1 resolves single-step receivers (`account.deposit()`, `self.balance`) but not chains (`a.b.c.foo()`). Cross-package go-to-definition lands a v1.1.

**Verification diagnostics absent**
Install `z3` and ensure it's on `PATH`. Save the file to trigger verification (Z3 only runs on save).

## V1 scope

See [ADR 0032](../../docs/decisions/0032-lsp-v1-surface.md) (revised in Phase 19). Out of scope for v1: member completion (`.field`/`.method` after `.`), find-references, rename, code actions, refactorings, semantic tokens, inlay hints, cross-package go-to-definition, Marketplace publishing, multi-root workspaces.
