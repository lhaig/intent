# Phase 32: Lexer in Intent (Stage2 First Step)

**Status:** Shipped (2026-06-03; commit `859998f`)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 31](phase-31-string-primitives.md) ([ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md))

## Goal

First Intent-in-Intent code. Tokenise a useful subset of Intent source from within Intent, using only the language primitives stage1 provides. The lexer becomes both an artefact (the stage2 toolchain has a real piece) and a forcing function (any language gap that blocks lexing has to be designed properly via an ADR before progress resumes).

## Success Criteria

- [x] `selfhost/formatter/lexer.intent` exists and type-checks via `intentc check`.
- [x] Tokenises identifiers + small keyword table (`module`, `version`, `function`, `entry`, `returns`, `let`, `mutable`, `return`, `if`, `else`, `while`, `true`, `false`, `and`, `or`, `not`, `Int`, `String`, `Bool`, `Void`).
- [x] Tokenises integer literals (digit runs).
- [x] Tokenises double-quoted string literals (raw text preserved, no escape decoding; `\\` consumed as a 2-char sequence so `\"` doesn't terminate early).
- [x] Tokenises punctuation: `( ) { } [ ] ; , : . + - * / % ? | @`.
- [x] Tokenises two-char operators: `== != <= >= .. =>` and disambiguates from the single-char forms.
- [x] Skips whitespace (` \t \n \r`) and single-line `//` comments.
- [x] Tracks line + column on every token; lines bump on `\n`.
- [x] Emits a synthetic `tk_eof()` terminator so consumers don't need bounds checks.
- [x] Top-level `lex(source) returns Array<Token>` convenience.
- [x] 13 in-language tests covering empty source, identifiers (with underscores + digits), every keyword kind, integer literals, all punctuation singletons, multi-char operators, string literal text preservation, escaped-backslash handling, comment skipping, line+column tracking, and a full small-program token sequence.
- [x] All 13 tests pass on rust + js (`intentc test --all-targets`).

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame (stage1/stage2 model from Zig precedent)
- [ADR 0041](../../docs/decisions/0041-string-indexing-and-char-type.md) — Phase 31 primitives used here
- `selfhost/formatter/lexer.intent` — the artefact
- `selfhost/formatter/README.md` — directory README + per-phase status

## Tasks (as shipped)

### 32.1 Design + Token/Lexer entities

**Files:** `selfhost/formatter/lexer.intent`

Token kinds modelled as plain `Int` with named accessor functions (`tk_*` / `kw_*`) rather than an enum with data-carrying variants. Rationale: Intent enums use single-expression-per-arm match for discrimination, which would force every kind check into a match expression. Int kinds keep call sites concise.

`Token` carries `kind: Int`, `text: String`, `line: Int`, `column: Int`. `Lexer` carries `source: String`, `position: Int` (codepoint index), `line: Int`, `column: Int`, with `position`/`line`/`column` mutated by methods.

### 32.2 `scan_identifier_or_keyword`

Loop while `is_ident_continue(peek())`, slice `source[start..position]`, look up via `keyword_kind(text)` (linear scan over the small keyword table). Falls back to `tk_ident` when no keyword matches.

### 32.3 `scan_int_literal`

Loop while `peek().is_digit()`, slice for the text. Float literals not in v1 — the parser hasn't needed them yet.

### 32.4 `scan_string_literal`

Consume opening `"`, accumulate until matching `"`. `\\` followed by any char counted as a 2-char sequence (so `\"` doesn't terminate). No escape decoding — text field carries the raw literal including surrounding quotes. Newline inside literal → `tk_error`. EOF before close → `tk_error`.

### 32.5 `scan_one` punctuation dispatch

Single-char punct + lookahead for two-char operators. Inline char literals (`'{'`, `'}'`) used in source — string literals containing literal braces would trigger Intent's string-interpolation grammar.

### 32.6 `skip_whitespace_and_comments`

Peek-loop handling space/tab/CR/LF and `//` single-line comments. Multi-line `/* */` comments deferred to Phase 34+.

### 32.7 `scan_all` + top-level `lex`

`scan_all` calls `skip_whitespace_and_comments` → snapshots line/column → `scan_one` → pushes token → repeats. Emits synthetic `tk_eof()` at the end. Top-level `lex(source)` constructs a `Lexer` and returns the `Array<Token>`.

### 32.8 In-language tests

13 `test "..." { ... }` blocks. Coverage matches the success criteria. Tests with literal braces in their fixture source use `'{'.to_string() + ...` concatenation to avoid Intent's string-interpolation grammar.

### 32.9 Gap surfacing + docs

Findings:
1. **Rust backend `&mut self` propagation** — fixed in this phase. `self.<user_method>()` calls weren't being detected as mutating; `LetStmt` and `ReturnStmt` weren't being walked by `stmtMutatesSelf`. Both fixed conservatively (any `self.method()` is treated as mutating; let/return now visited).
2. **Literal `{` in string literals** — workaround via char-to-string concat; future polish ADR could require `${}` or accept `\{` escape.
3. **`let _:` not accepted** — workaround via expression statements; future polish.

ROADMAP entry added; NEXT-STEPS rewritten with the gap-tracking table.

## Out of Scope (deferred)

- Multi-line `/* */` comments.
- Char literal tokens in stage2 lexer.
- Float literal tokens.
- String interpolation token splitting (whole quoted run = one `tk_string`).
- Keywords beyond the v1 set (e.g. `entity`, `enum`, `trait`, `impl`, `match`, `for`, `in`, `forall`, `exists`, etc.) — added as parser phases need them.

## Validation

- `make validate` green.
- `intentc test --all-targets selfhost/formatter/lexer.intent` green on rust + js.
- WASM target skipped — pre-existing stub, not Phase 32.

## Actual size

~400 LOC across the single file. About 200 LOC of scanner code, 200 LOC of token-kind constants + tests.
