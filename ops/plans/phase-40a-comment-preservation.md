# Phase 40A: Comment Preservation (Leading Decl Comments)

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece A — v1)
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Design decision:** [ADR 0044](../../docs/decisions/0044-stage2-comment-preservation.md)
**Prerequisite:** [Phase 40B](phase-40b-paren-stripping.md) (paren stripping)

## Goal

The stage2 lexer dropped comments in `skip_whitespace_and_comments` — they never reached the parser or AST. Byte-equal self-format on `selfhost/formatter/*.intent` (heavily commented) was impossible until comments survived round-trip. Phase 40A.1 captures **leading comments on top-level declarations** — the most common documentation pattern in stage2 source — and round-trips them.

Inline-after comments (`let x = 1; // explanation`) and comments inside function bodies are deferred to Phase 40A.2 (or rolled into a later phase). This phase lands the lexer/AST/formatter scaffolding; later sub-phases extend without restructuring.

## Success Criteria

- [x] `Token` entity gains `comments_before: Array<String>` field, defaulted to empty in the constructor body so existing 41 Token construction sites don't change signature.
- [x] `Lexer` entity gains `pending_comments: Array<String>` field.
- [x] `skip_whitespace_and_comments` captures both `//` and `/* */` comments verbatim (including delimiters) and pushes onto `pending_comments`.
- [x] `scan_all` drains `pending_comments` onto each non-trivia token's `comments_before` (including trailing comments onto the synthetic EOF token).
- [x] `drain_comments()` helper on Lexer swaps the buffer.
- [x] Nine top-level decl entities gain `comments_before: Array<String>`: `ImportDecl`, `FunctionDecl`, `EntityDecl`, `EnumDecl`, `TraitDecl`, `ImplDecl`, `IntentBlock`, `TestDecl`, `ExternDecl`.
- [x] Each `parse_*_decl` method accepts `leading_comments: Array<String>` and passes it to the decl constructor. The dispatcher in `parse_program` captures comments at iteration start (before consuming any modifier) and passes them down — `public` lookahead doesn't strand them.
- [x] `format_comments_before(cmts) returns String` helper emits each comment on its own line with a trailing newline.
- [x] `format_program` k-way merge dispatcher prepends each decl's `comments_before` to its emission.
- [x] **6 new tests** lock the behaviour: single-line before decl, multiple lines, block comment, `public` decl preserves comments, entity decl, interleaved decls with comments each.
- [x] Stage1 backend fix: `let x: T = self.field;` for non-Copy `T` now appends `.clone()` — symmetric with the prior IndexExpr fix. Regression test `TestLetBindingClonesFieldAccessOfNonCopyType`.
- [x] All prior tests pass — **131/131 stage2 tests on rust + js** (was 125 + 6 new).
- [x] `make validate` green.

## Reference

- [ADR 0044](../../docs/decisions/0044-stage2-comment-preservation.md) — design decision (token-attached vs. comment-token stream vs. AST-attached)
- [Phase 40B PRD](phase-40b-paren-stripping.md) — preceding sub-piece
- `selfhost/formatter/lexer.intent` — `Token.comments_before`, `Lexer.pending_comments`, `drain_comments`, comment-capturing `skip_whitespace_and_comments`
- `selfhost/formatter/ast.intent` — 9 decl entities widen
- `selfhost/formatter/parser.intent` — 9 `parse_*_decl` accept `leading_comments`; `parse_program` captures pre-modifier
- `selfhost/formatter/format.intent` — `format_comments_before`; k-way merge dispatch prepends each decl's comments
- `selfhost/formatter/format_test.intent` — 6 new comment-preservation tests
- `internal/rustbe/rustbe.go` — `let`-binding clone fix for FieldAccessExpr
- `internal/rustbe/rustbe_test.go` — `TestLetBindingClonesFieldAccessOfNonCopyType`

## Design decisions (and why)

### Token-attached leading comments

Documented in [ADR 0044](../../docs/decisions/0044-stage2-comment-preservation.md). Briefly:

- **Token-attached** keeps the lexer as the source of truth for comment position. Parser just reads `Token.comments_before` when it cares.
- Alternative **separate comment-token stream** would let the parser stay totally unchanged (just skip `tk_comment`), but pushes coordinated-walk complexity onto the formatter.
- Alternative **AST-attached** (every node gets a comments field) is the long-term right answer but widens every AST entity. Deferred.

### Default `comments_before` to empty in Token constructor body

Adding a 5th constructor parameter to Token would force updating all 41 construction sites in `lexer.intent`. Instead, the constructor signature stays `(kind, text, line, column)` and the body sets `self.comments_before = empty_string_array()`. `scan_all` then mutates the field via `tok.comments_before = self.drain_comments()` after construction. Helper function `empty_string_array()` sidesteps a stage1 Rust-backend constructor-hoist quirk where `let mutable empty: Array<String> = [];` inside the constructor body lands AFTER the struct literal initialiser.

### Capture comments in `parse_program` before consuming `public`

In the current grammar, `parse_program` consumes the optional `public` modifier before dispatching to `parse_entity_decl` / `parse_enum_decl` / `parse_trait_decl`. The lexer attaches comments to the *first* token — which is `public` when the modifier is present. If the parse method captures its own comments (from its first-consumed token like `entity`), comments on `public` are stranded.

Fix: `parse_program` captures `self.tokens[self.position].comments_before` at the start of each iteration, before any `self.advance()` or `self.check(public)`. The captured comments are then passed down via a new `leading_comments: Array<String>` parameter on each `parse_*_decl` method.

Trade-off: nine `parse_*_decl` method signatures widen by one parameter; nine dispatch sites in `parse_program` pass `cmts`. Mechanical but localised — the alternative (use a Parser field as a backchannel) would be smaller in diff but more magical at the call sites.

### Constructors / methods inside entity bodies get empty comments

`parse_constructor_decl` and `parse_method_decl` don't accept a `leading_comments` parameter — they're called from inside `parse_entity_decl` and `parse_impl_decl`, where the enclosing decl already carries the leading comments. The methods/constructors themselves are formatted as part of their enclosing decl's body; per-method leading comments are out of scope for v1 (would require Phase 40A.2 — comments inside bodies). They pass an empty array to the FunctionDecl constructor.

### Block comments stored verbatim, including delimiters

A `/* multi\nline */` comment is stored as one entry in the `Array<String>`: `"/* multi\nline */"`. The formatter emits it verbatim. This preserves the user's internal formatting (indentation inside the block, asterisks on lines, etc.) without trying to re-canonicalise.

### Backend fix: clone `let x: T = self.field` for non-Copy T

`drain_comments` does `let cmts: Array<String> = self.pending_comments;` which moved out of `self`, hitting `E0507`. The existing `cloneIfNeeded` logic in `internal/rustbe/rustbe.go` handled `*ir.IndexExpr` non-Copy lets but not `*ir.FieldAccessExpr`. Six-line patch extends the IndexExpr branch to FieldAccessExpr. Regression test `TestLetBindingClonesFieldAccessOfNonCopyType` locks it.

## What now byte-equals (and what doesn't)

After Phase 40A.1, `examples/hello.intent` continues to round-trip byte-equal. Stage2 source files **do not** yet byte-equal because they have:

- **Inline-after comments** (`let x = 1; // ...`) — deferred to Phase 40A.2.
- **Comments inside function bodies** between statements.
- **Comments inside expressions** like `f(x, /* hint */ y)`.
- **Trailing-end-of-file comments** are captured (onto the synthetic EOF token) but the formatter doesn't emit them yet — also Phase 40A.2.

The parse + format roundtrip on stage2 source files continues to succeed (no crashes); byte-equal needs the deferred work.

## Surfaced gaps (deferred to Phase 40A.2 / Phase 41+)

- Inline-after comments need a `Token.comment_after: Option<String>` (or similar) and parser hooks at statement / expression boundaries.
- Comments inside function bodies need either statement-attached or `Block.trailing_comments` capture.
- Comments inside expressions need expression-attached fields — largest surface change.

## Files touched

- `docs/decisions/0044-stage2-comment-preservation.md` — new ADR.
- `selfhost/formatter/lexer.intent` — `Token.comments_before`, `Lexer.pending_comments`, comment-capturing `skip_whitespace_and_comments`, `drain_comments`, `scan_all` drain logic, `empty_string_array()` helper.
- `selfhost/formatter/ast.intent` — 9 decl entities gain `comments_before: Array<String>`; `empty_function_decl` updated.
- `selfhost/formatter/parser.intent` — 9 `parse_*_decl` accept `leading_comments`; `parse_program` captures pre-modifier and passes through dispatcher.
- `selfhost/formatter/format.intent` — `format_comments_before` helper; k-way merge dispatcher prepends comments before each decl emit.
- `selfhost/formatter/format_test.intent` — 6 new comment-preservation tests; 2 existing tests updated for new Import constructor signature.
- `internal/rustbe/rustbe.go` — clone-on-let for non-Copy FieldAccessExpr.
- `internal/rustbe/rustbe_test.go` — regression test.
