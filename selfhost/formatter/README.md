# selfhost/formatter/

Stage2 Intent formatter — Intent-implemented `intentc fmt`. Reuses the shared
front-end in [`../shared/`](../shared/) (lexer/ast/parser) and lives alongside
stage1's Go formatter (`internal/formatter/`). As of Phase 44 the linter moved to
[`../linter/`](../linter/) and the front-end to `../shared/` ([ADR 0051](../../docs/decisions/0051-selfhost-shared-restructure.md));
this directory now holds the formatter only.

**Status (2026-06-26):** Formatter **complete** — byte-for-byte parity with
`intentc fmt` on the full examples corpus (Phase 42, `make diff-formatter` 22/22) and
a self-format fixpoint on the stage2 source files (`make selfcheck-formatter` — now
over `../shared/{lexer,ast,parser}` + `format.intent`). Wired as
`intentc fmt --self-hosted`.

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
selfhost/
  shared/                 # lexer.intent, ast.intent, parser.intent (see ../shared/)
  formatter/              # THIS directory:
    format.intent         #   AST → source                  (Phase 38 — shipped)
    format_test.intent    #   formatter tests + hello.intent dogfood
    main.intent           #   entry (intentc fmt --self-hosted)  (Phase 42)
    difftest.sh           #   make diff-formatter harness    (Phase 42)
    difftest-lint.sh      #   make diff-linter harness (drives ../linter)  (Phase 43)
    selfcheck.sh          #   make selfcheck-formatter harness  (Phase 42/44)
    README.md             #   (this file)
  linter/                 # lint.intent, lint_main.intent, ... (see ../linter)
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
| **42** | CLI wiring (`args()` builtin, `main.intent`, `intentc fmt --self-hosted`) + differential harness (`make diff-formatter`) vs `intentc fmt` + parser-gap closing (invariants, contracts, intent blocks, implies, await, forall/exists, generics, Fn/lambdas, attributes). **Corpus 22/22, 0 divergences.** Self-format gate via built binary (`make selfcheck-formatter`). | **Shipped** ([PRD](../../prds/done/phase-42-formatter-cli-differential.md)) |
| **43** | **Self-hosted linter** — all 16 Go-linter rule families in `lint.intent` reusing the stage2 lexer/parser/AST; `lint_main.intent` + `intentc lint --self-hosted` shim; `make diff-linter` differential vs `intentc lint`. **Corpus + fixtures 26/26, byte-equal.** Added source-`column` tracking to the AST ([ADR 0050](../../docs/decisions/0050-self-hosted-linter-strategy.md)). | **Shipped** ([PRD](../../prds/done/prd-phase-43-self-hosted-linter.md)) |

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
intentc test --all-targets selfhost/formatter/lexer.intent       # lexer tests
intentc test --all-targets selfhost/formatter/parser.intent      # parser + lexer (imported)
intentc test --all-targets selfhost/formatter/format_test.intent # formatter + imported
intentc test --all-targets selfhost/formatter/lint_test.intent   # linter + imported (269)
```

Differential + self-format gates (run from the repo root):

```bash
make diff-formatter        # stage2 fmt vs intentc fmt over examples (22/22)
make selfcheck-formatter   # stage2 files are formatter fixpoints (4 EQUAL)
make diff-linter           # stage2 lint vs intentc lint over examples + fixtures (26/26)
```

PRs should reference the relevant phase number.
