# Phase 37: Stage2 Lexer Extensions (Char + Float + Block Comments)

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 36](phase-36-top-level-decls-in-intent.md) (stage2 top-level parser + AST split)

## Goal

Stage2 needs to lex the full surface of real Intent source before the formatter (Phase 38) can be written. Phase 32-36's lexer covered identifiers, integers, double-quoted strings, single-line comments, and the common punctuation — enough to bootstrap the parser. Three syntactic forms remained: **char literals** (`'a'`, `'\n'`, `'\u{...}'`), **float literals** (`3.14`, `1.5e-3`), and **multi-line `/* ... */` comments** (with nesting). Phase 37 adds them to `selfhost/formatter/lexer.intent`, wires the new token kinds into the expression parser as primary expressions (`ex_float`, `ex_char`), and lands an end-to-end dogfood test exercising all three new tokens.

## Success Criteria

- [x] `lexer.intent` gains `tk_float` (= 4) and `tk_char` (= 5) kind constants; existing constants unchanged (no numeric renumbering).
- [x] `scan_int_literal` promotes to `tk_float` when the digits are followed by `.<digit>`; supports exponents `e`/`E` with optional sign — but only when a decimal portion was already consumed, so `1e5` stays as int + ident (no surprising re-tokenisation).
- [x] `1..5` continues to tokenise as `int dotdot int` (range op), not float.
- [x] `42.foo` continues to tokenise as `int dot ident` (method call), not float.
- [x] `scan_char_literal` recognises `'<char>'`, `'\<escape>'`, and `'\u{...}'`; preserves the raw lexeme (including quotes) in `Token.text`; emits `tk_error` on unterminated/empty/newline-in-literal.
- [x] `skip_whitespace_and_comments` handles `/* ... */` with **nested** support (depth counter), per the Intent grammar.
- [x] `scan_one` dispatches `'` to `scan_char_literal`.
- [x] `ast.intent` gains `ex_float` (= 13) and `ex_char` (= 14) kind constants; both store the raw lexeme in `str_value` (no float runtime parsing yet — the formatter only needs to round-trip).
- [x] `parser.intent` `parse_primary` handles `tk_float` → `ex_float` and `tk_char` → `ex_char`.
- [x] 14 new in-language tests (12 in `lexer.intent`, 4 in `parser.intent` — including the Phase 37 dogfood asserting end-to-end pipeline on a program with all three tokens).
- [x] Combined: 93/93 passing on rust + js (88 from Phases 32-36 + 5 net additions to the parser file; the lexer test count rose from 13 to 27).
- [x] `make validate` green.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 36 PRD](phase-36-top-level-decls-in-intent.md) — AST split this builds on
- `selfhost/formatter/lexer.intent` — `tk_float`, `tk_char`, `scan_char_literal`, block-comment skipping
- `selfhost/formatter/ast.intent` — `ex_float`, `ex_char`
- `selfhost/formatter/parser.intent` — `parse_primary` extended; Phase 37 dogfood test

## Design decisions (and why)

### Float promotion requires a `.<digit>` lookahead

Three options for disambiguating int vs. float:

1. **`.<digit>` lookahead** (chosen): `1.5` is a float; `1..5` (range) and `1.foo` (method call) stay as int + further tokens.
2. **Always consume `.` if it follows digits**: simpler, but would break the range operator and method-call syntax mid-stream.
3. **Bare exponent like `1e5` also becomes float**: most languages do this, but `e5` is a valid Intent identifier and the change would silently re-tokenise pre-existing source. The risk doesn't pay off for self-hosting — Intent code typically writes `1.0e5` anyway.

(1) is the conservative choice: float promotion only triggers when there's no syntactic ambiguity. Tests cover `1..5`, `42.foo`, and `1e5` explicitly to lock the chosen behavior.

### Raw lexeme in `ex_float.str_value` / `ex_char.str_value`

Two storage options:

1. **Raw lexeme as String** (chosen): `ex_float.str_value = "3.14"`, `ex_char.str_value = "'\\n'"`.
2. **Parsed `Float` / `Char` value**: would require an in-Intent `parse_float` (non-trivial: IEEE-754 round-tripping) and an in-Intent decoder for char escapes.

The formatter (Phase 38) needs to round-trip these literals byte-for-byte. Storing the raw lexeme defers the parsing question entirely; a later phase (or a semantic pass) can add a `parse_float`/`decode_char` if the linter/checker needs the actual value. This mirrors the Phase 32 decision to keep string literals as raw text with quotes intact.

### Nested block comments via depth counter

Intent's grammar specifies nested `/* */` comments (matching Rust). The depth-counter implementation is straightforward — five lines in `skip_whitespace_and_comments`. The alternative (flat block comments, where the first `*/` terminates regardless of nesting) is simpler to implement but inconsistent with the documented grammar and would surprise users coming from Rust.

### Unicode escape consumes content up to `}`

The `'\u{1F600}'` syntax has variable-length hex content. The lexer accepts whatever appears between `{` and `}` without validating it — the same way escape sequences in string literals are accepted as 2-char sequences without decoding. Semantic validation belongs in the checker, not the lexer.

### Test for `'\u{...}'` uses string concatenation, not inline `{`/`}`

Stage1's known gap: literal `{` / `}` inside a string literal trigger interpolation parsing. The unicode-escape test source is built via `"'\\u" + '{'.to_string() + "1F600" + '}'.to_string() + "'"`. This mirrors how Phase 32 onward writes test inputs containing braces.

## Surfaced gaps (deferred)

Same carried-over gaps as Phase 36 — none worsened, none resolved this phase:

- File I/O in stage2 (`read_file`) — needed for a *real* self-parse dogfood (parse `selfhost/formatter/lexer.intent` from disk). Currently dogfooded via inline synthetic source. Likely a stage1 extern in Phase 38 once the formatter needs to read source.
- String interpolation tokenisation — currently `"abc ${expr} def"` lexes as one `tk_string`. The formatter will need the interp segments split out (likely a `tk_string_start` / `tk_interp_start` / `tk_string_part` / `tk_interp_end` sequence). Deferred to Phase 38 if needed; the formatter can re-emit interpolated strings as opaque tokens for v1.
- Backend ADR — cross-module bare function calls (Phase 36).
- Backend ADR — auto-`&mut` for entity parameters (Phase 36).

## Out of scope

- A real `Float` value type in `Expr` (kept as raw String).
- Char-escape decoding (deferred to checker/runtime, not lexer).
- String interpolation token split (deferred to Phase 38 if needed).
- File I/O in stage2 (deferred to Phase 38).
- Beginning the formatter — Phase 38.

## Files touched

- `selfhost/formatter/lexer.intent` — `tk_float`, `tk_char` consts; `scan_char_literal`; `scan_int_literal` float promotion; `skip_whitespace_and_comments` block-comment support; `'` dispatch in `scan_one`; 14 new tests.
- `selfhost/formatter/ast.intent` — `ex_float`, `ex_char` consts + doc comment.
- `selfhost/formatter/parser.intent` — `parse_primary` extended; BNF comment updated; 5 new tests (4 unit + 1 dogfood).
