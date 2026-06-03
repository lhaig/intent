# selfhost/formatter/

Stage2 Intent formatter — Intent-implemented `intentc fmt`. Lives alongside
stage1's Go formatter (`internal/formatter/`); will eventually replace it
once parity is reached.

**Status:** scaffold only. No working code yet — blocked on Phase 31 (string
primitives + `Char` type, per ADR 0041) before the lexer can be written.

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

## Planned layout (target)

```
selfhost/formatter/
  intent.toml             # package manifest (Phase 30 / ADR 0039)
  ast.intent              # AST entity declarations
  lexer.intent            # source → tokens                 (Phase 32)
  parser.intent           # tokens → AST                    (Phases 33-36)
  format.intent           # AST → formatted source string   (Phase 37)
  main.intent             # stdin → format → stdout
  tests/                  # in-language tests per layer
  README.md               # (this file)
```

## Phase-by-phase

| Phase | Scope | Status |
|---|---|---|
| **31** | Stage1 adds `Char` type, `s[i]`, `s[i..j]`, `len(s)`, char predicates ([ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md)). | Planning |
| **32** | Lexer in Intent: tokenise a useful subset of source. | Blocked on 31 |
| **33** | AST entity layout + parser for top-level decls. | Blocked on 32 |
| **34** | Statement-level parser. | Blocked on 33 |
| **35** | Expression parser with precedence. | Blocked on 33 |
| **36** | Entity / trait / impl / intent block parsing. | Blocked on 33-34 |
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

For now this directory is intentionally near-empty. As Phase 31 lands and
Phase 32 begins, code will fill in here, with corresponding tests under
`tests/`. PRs should reference the relevant phase number.
