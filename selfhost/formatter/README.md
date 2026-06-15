# selfhost/formatter/

Stage2 Intent formatter — Intent-implemented `intentc fmt`. Lives alongside
stage1's Go formatter (`internal/formatter/`); will eventually replace it
once parity is reached.

**Status (2026-06-09):** Phases 32-40A shipped. 131 in-language
tests on rust + js. **Self-parse + self-format certified** (Phase
39); **source-order tracking** (Phase 40C, ADR 0042); **paren
stripping** (Phase 40B, ADR 0043); **leading-decl comment
preservation** (Phase 40A.1, ADR 0044). hello.intent remains the
byte-equal dogfood fixture. Stage2-source byte-equal needs Phase
40A.2 (inline + body comments).

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
  lexer.intent            # source → tokens                 (Phase 32 / 37 — shipped)
  ast.intent              # AST entity declarations          (Phase 36 / 37 — shipped)
  parser.intent           # tokens → AST (full grammar)     (Phases 33-37 — shipped)
  format.intent           # AST → source                    (Phase 38 — shipped)
  format_test.intent      # formatter tests + hello.intent dogfood
  README.md               # (this file)
```

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
| **32** | Lexer in Intent: tokenise a useful subset of source. | **Shipped** (commit `859998f`; [PRD](../../prds/done/phase-32-lexer-in-intent.md)) |
| **33** | AST entity layout + parser for top-level decls (module / imports / function signatures). | **Shipped** (commit `3d3fdef`; [PRD](../../prds/done/phase-33-parser-toplevel-in-intent.md)) |
| **34** | Statement-level parser (`let`, `return`, `if`/`else`, `while`, expression statements, `Block`). | **Shipped** ([PRD](../../prds/done/phase-34-statement-parser-in-intent.md)) |
| **35** | Expression parser with precedence (Pratt / precedence climbing). | **Shipped** ([PRD](../../prds/done/phase-35-expression-parser-in-intent.md)) |
| **36** | Entity / enum / trait / impl / intent / test / extern declarations + AST split. | **Shipped** ([PRD](../../prds/done/phase-36-top-level-decls-in-intent.md)) |
| **37** | Stage2 lexer extensions: char + float literals, nested `/* */` comments; `ex_char` / `ex_float` wired into expression parser. | **Shipped** ([PRD](../../prds/done/phase-37-stage2-lexer-extensions.md)) |
| **38** | Formatter MVP — `format.intent`. Hello.intent round-trips byte-equal with stage1. | **Shipped** ([PRD](../../prds/done/phase-38-stage2-formatter-mvp.md)) |
| **39** | Self-parse certification — all stage2 files parse + format without errors. | **Shipped** ([PRD](../../prds/done/phase-39-self-parse-certification.md)) |
| **40C** | Source-order tracking via per-decl `line: Int` (ADR 0042). | **Shipped** ([PRD](../../prds/done/phase-40c-source-order-tracking.md)) |
| **40B** | Paren stripping — precedence-aware emit (ADR 0043). | **Shipped** ([PRD](../../prds/done/phase-40b-paren-stripping.md)) |
| **40A.1** | Leading-decl comment preservation (ADR 0044). | **Shipped** ([PRD](../../prds/done/phase-40a-comment-preservation.md)) |
| **40A.2 / 40A.3** | Inline-after + body + real-file comments; byte-equal self-format on all 4 stage2 files. | **Shipped** ([PRD](../../prds/done/phase-40a-comment-preservation.md)) |
| **41** | Parser surface widening: contracts (`requires`/`ensures`/`decreases`), `match`, `for`-in, `try`. | **Shipped** ([PRD](../../prds/done/phase-41-parser-surface-widening.md)) |
| **42** | CLI wiring (`args()` builtin, `main.intent`, `intentc fmt --self-hosted`) + differential harness (`make diff-formatter`) vs `intentc fmt`. Corpus 16/22, 0 divergences; gap-closing ongoing. | **In progress** ([PRD](../../prds/active/phase-42-formatter-cli-differential.md)) |

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

Each phase has its own PRD in `prds/done/phase-N-*.md` and a ROADMAP entry
in `docs/ROADMAP.md`. In-language tests live alongside the code in
`lexer.intent` / `parser.intent` for now; will move to `tests/` once the
file count grows.

Run the stage2 tests on every backend with:

```bash
intentc test --all-targets selfhost/formatter/lexer.intent       # 27 tests
intentc test --all-targets selfhost/formatter/parser.intent      # 66 tests + 27 lexer (imported) = 93
intentc test --all-targets selfhost/formatter/format_test.intent # 38 formatter tests + 93 imported = 131
```

PRs should reference the relevant phase number.
