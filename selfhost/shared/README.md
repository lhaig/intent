# selfhost/shared/

The stage2 front-end shared by every self-hosted Intent tool (formatter, linter,
and the upcoming checker). Plain Intent source, built by stage1 `intentc`.

| File | Module | Purpose |
|------|--------|---------|
| `lexer.intent` | `shared_lexer` | source → tokens (preserves comments/positions) |
| `ast.intent` | `shared_ast` | AST entity declarations + kind constants + helpers |
| `parser.intent` | `shared_parser` | tokens → `Program` (full grammar) |

Tools import these via `../shared/…` (e.g. `import "../shared/parser.intent"`) and
reference them as `shared_lexer.*` / `shared_ast.*` / `shared_parser.*`.

This directory was split out of `selfhost/formatter/` in Phase 44
([ADR 0051](../../docs/decisions/0051-selfhost-shared-restructure.md)) once the
checker became the third tool to depend on the front-end. `lexer.intent`,
`ast.intent`, and `parser.intent` are self-format fixpoints — verify with
`make selfcheck-formatter` (never run stage1 `intentc fmt` on them).

Run the front-end's own in-language tests:

```bash
intentc test --all-targets selfhost/shared/parser.intent   # parser + lexer (imported)
```
