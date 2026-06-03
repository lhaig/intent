# selfhost/formatter/

Stage2 Intent formatter — Intent-implemented `intentc fmt`. Lives alongside
stage1's Go formatter (`internal/formatter/`); will eventually replace it
once parity is reached.

**Status (2026-06-03):** lexer + top-level parser + statement parser +
expression parser shipped (Phases 32-35). 60 in-language tests passing
on rust + js. Entity / trait / impl declarations are next (Phase 36).

## Big picture

See [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
for the full multi-phase plan. The short version:

- **Stage1** = Go formatter at `internal/formatter/` (current truth).
- **Stage2** = Intent formatter at `selfhost/formatter/` (this directory).
- Goal: stage2 reaches byte-for-byte parity with stage1 on a corpus of
  `.intent` files, then becomes the canonical formatter.
- Approach: **gap-driven**. Each phase first surfaces the language extension
  it needs, lands that extension in stage1 (via its own ADR + phase), then
  writes the stage2 Intent code that uses it.

## Current layout

```
selfhost/formatter/
  intent.toml             # package manifest (Phase 30 / ADR 0039)
  lexer.intent            # source → tokens                 (Phase 32 — shipped)
  parser.intent           # tokens → AST (top-level only)   (Phase 33 — shipped)
  README.md               # (this file)
```

AST entity declarations are inlined at the top of `parser.intent` for now —
will split into a sibling `ast.intent` if the file gets unwieldy.

## Target layout (eventual)

```
selfhost/formatter/
  intent.toml
  ast.intent              # AST entity declarations (split out once large)
  lexer.intent            # source → tokens
  parser.intent           # tokens → AST (statements + expressions)
  format.intent           # AST → formatted source string   (Phase 37)
  main.intent             # stdin → format → stdout
  tests/                  # in-language tests per layer
  README.md
```

## Phase-by-phase

| Phase | Scope | Status |
|---|---|---|
| **31** | Stage1 adds `Char` type, `s[i]`, `s[i..j]`, `len(s)`, char predicates ([ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md)). | **Shipped** (commit `54f05b4`) |
| **32** | Lexer in Intent: tokenise a useful subset of source. | **Shipped** (commit `859998f`; [PRD](../../ops/plans/phase-32-lexer-in-intent.md)) |
| **33** | AST entity layout + parser for top-level decls (module / imports / function signatures). | **Shipped** (commit `3d3fdef`; [PRD](../../ops/plans/phase-33-parser-toplevel-in-intent.md)) |
| **34** | Statement-level parser (`let`, `return`, `if`/`else`, `while`, expression statements, `Block`). | **Shipped** ([PRD](../../ops/plans/phase-34-statement-parser-in-intent.md)) |
| **35** | Expression parser with precedence (Pratt / precedence climbing). | **Shipped** ([PRD](../../ops/plans/phase-35-expression-parser-in-intent.md)) |
| **36** | Entity / trait / impl / intent / test / extern declarations + AST split. | Next |
| **37** | Formatter (AST → string), byte-parity on a corpus. | Blocked on 33-36 |
| **38** | Full-feature parser parity (async, pattern matching, generics). | Blocked on 33-37 |
| **39** | Differential test gate + CLI integration (`intentc fmt --self-hosted`). | Blocked on 37-38 |

Phase numbers are indicative and may shift as language gaps surface.

## How to invoke (target)

Once stage2 is up:

```bash
cd selfhost/formatter
intentc pkg install
intentc build main.intent           # produces ./main binary
./main < ../../examples/hello.intent
```

The CLI integration (`intentc fmt --self-hosted`) is a thin shim added in
Phase 39 once parity holds.

## How to develop

Each phase has its own PRD in `ops/plans/phase-N-*.md` and a ROADMAP entry
in `docs/ROADMAP.md`. In-language tests live alongside the code in
`lexer.intent` / `parser.intent` for now; will move to `tests/` once the
file count grows.

Run the stage2 tests on every backend with:

```bash
intentc test --all-targets selfhost/formatter/lexer.intent     # 13 tests
intentc test --all-targets selfhost/formatter/parser.intent    # 47 tests + the 13 lexer tests (imported) = 60
```

PRs should reference the relevant phase number.
