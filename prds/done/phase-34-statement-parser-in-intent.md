# Phase 34: Statement Parser in Intent

**Status:** Shipped (2026-06-03)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 33](phase-33-parser-toplevel-in-intent.md) (stage2 top-level parser)

## Goal

Replace Phase 33's raw-text body capture with a real statement-level AST. The stage2 parser now parses `let`, `return`, `if`/`else` (including `else if` chains), `while`, and expression statements inside function bodies. Expressions are still captured as raw token-joined text via depth-balanced bracket scanning — a real expression parser is Phase 35.

## Success Criteria

- [x] `Stmt` entity with `kind: Int` discriminator + per-kind fields (`expr_text`, `name`, `type_name`, `is_mutable`, `then_block`, `else_block`, `has_else`). Single-entity-with-tag mirrors how `Token.kind` works in the lexer.
- [x] `Block` entity holding `Array<Stmt>`. Mutually-recursive sizing safe because `Block` reaches `Stmt` only through `Array<Stmt>` (heap-allocated).
- [x] Statement kind constants: `st_let`, `st_return`, `st_if`, `st_while`, `st_expr` (1..5).
- [x] `FunctionDecl.body` is now a `Block` (was `body_text: String`).
- [x] `parse_block` replaces `consume_braced_body`. Consumes `{`, parses statements until `}`, returns `Block`.
- [x] `parse_statement` dispatches on the first token's kind.
- [x] `parse_let_stmt` handles `let [mutable] IDENT [: TYPE] = EXPR;`. Both annotated and inferred bindings supported.
- [x] `parse_return_stmt` handles `return [EXPR];`. Empty expr for `return;`.
- [x] `parse_if_stmt` handles `if EXPR { BLOCK } [else (if-stmt | { BLOCK })]`. `else if` folds into a single-statement else-block whose only statement is the nested if (LISP-style chain).
- [x] `parse_while_stmt` handles `while EXPR { BLOCK }`.
- [x] `parse_expr_stmt` captures anything else until `;`.
- [x] `capture_expr_until_semi` / `capture_expr_until_lbrace` scan tokens with depth tracking of `()`, `[]`, `{}` until the terminator at depth 0. Token texts joined with single spaces.
- [x] `empty_block()` helper avoids untyped `Block([])` literals at call sites that can't infer the element type.
- [x] 13 new in-language tests covering: empty body, return with/without value, let annotated, let mutable inferred, expression statement (assignment), if-only, if-else, else if chain, while, balanced parens in expression text, nested if in if-then, and a missing-semicolon error case.
- [x] Combined with Phase 32 lexer + Phase 33 top-level parser: 38/38 passing on rust + js.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 33 PRD](phase-33-parser-toplevel-in-intent.md) — top-level parser this builds on
- `selfhost/formatter/parser.intent` — the artefact
- `selfhost/formatter/README.md` — directory README + per-phase status

## Tasks (as shipped)

### 34.1 AST shape

**Files:** `selfhost/formatter/parser.intent`

Added a single `Stmt` entity tagged by `kind: Int` plus a `Block` entity wrapping `Array<Stmt>`. Considered Intent's payload-carrying enums, but every field access through a sum-typed `Stmt` would need a match arm and Intent's match arms are single expressions — Token's existing Int-discriminator + per-kind-field pattern is the precedent that already works. The header comment of `parser.intent` documents this trade-off so it can be revisited if Phase 35+ design pressure changes.

`Block` ↔ `Stmt` mutual reference is sound because the only path from `Block` back to `Stmt` is through `Array<Stmt>` (heap-allocated). Direct-field recursion is avoided.

### 34.2 `parse_block` + statement dispatch

`parse_block` consumes the opening `{`, parses statements until it sees `}` (or hits an error), and expects the closing `}`. `parse_statement` dispatches on the leading-token kind: `kw_let` / `kw_return` / `kw_if` / `kw_while` / anything-else.

### 34.3 Statement parsers

- `parse_let_stmt`: optional `mutable`, ident name, optional `: TYPE`, `=`, `capture_expr_until_semi`, `;`.
- `parse_return_stmt`: `return`, optional expression, `;`.
- `parse_if_stmt`: `if`, `capture_expr_until_lbrace`, `parse_block` (then), optional `else` followed by either `if` (recursive `parse_if_stmt` wrapped in a 1-statement Block) or `{ BLOCK }`.
- `parse_while_stmt`: `while`, `capture_expr_until_lbrace`, `parse_block`.
- `parse_expr_stmt`: `capture_expr_until_semi`, `;`.

### 34.4 Expression-text capture

Two helpers walk tokens with `(` / `[` / `{` depth counters:

- `capture_expr_until_semi`: terminates at `;` when all three depths are zero. Bails with a `fail()` if it sees a `}` at brace-depth 0 (that's the enclosing block's terminator, not the expression's).
- `capture_expr_until_lbrace`: terminates at `{` when paren-depth + bracket-depth are both zero. Only balances `()` and `[]` — brace-balancing would defeat the purpose since `{` itself is the terminator.

Captured token texts are joined with single spaces. Original whitespace fidelity is lost, but Phase 34 doesn't need it; Phase 37 (formatter) will reconstruct from the AST. Phase 35 replaces both helpers with a real expression parser.

### 34.5 `empty_block()` helper

Stmt constructors that don't use `then_block` / `else_block` still must pass a `Block` value. A bare `Block([])` literal triggers `empty array literal requires type annotation (element type cannot be inferred)` because the constructor signature can't disambiguate the empty array's element type at the call site. The free function `empty_block() returns Block` creates a typed `Array<Stmt>` then constructs the block — call sites stay readable.

### 34.6 Tests

13 new `test "..." { ... }` blocks. The `{` / `}` interpolation gotcha (Phase 32) still applies, so fixture sources concatenate `'{'.to_string()` / `'}'.to_string()`.

## Out of Scope (deferred)

- **Expression parsing** — Phase 35. Operators, calls, indexing, field access, lambdas, match expressions.
- **`match` statement, `for` loop, `break` / `continue`, `try` / `await`** — surface those when the parser actually needs them (later stage2 phases).
- **Real `TypeRef` AST** — types are still stored as strings with a `<...>` suffix for generic heads.
- **Source-fidelity inside expressions** — captured token texts are joined with single spaces.

## Validation

- `make validate` green (all examples pass, all Go unit tests pass).
- `intentc test selfhost/formatter/parser.intent` 38/38 on rust.
- `intentc test --target js selfhost/formatter/parser.intent` 38/38 on js.

## Actual size

~200 additional LOC in `selfhost/formatter/parser.intent` (entities + parse methods + helpers), plus 13 new tests (~160 LOC). No language / backend changes were needed — Phase 33's constructor-field-hoist fix already covers the common `self.f = f` initialiser pattern that the new entities use.

## Surfaced gaps (workarounds applied; no fix this phase)

| Gap | Workaround | Suggested follow-up |
|---|---|---|
| Bare `Block([])` literal can't infer element type. | `empty_block()` helper. | Future: array-literal element-type inference from constructor signature, or a `Block::empty()` static method. |
| `{` / `}` in string literals still trigger interpolation parsing. | Continue using `'{'.to_string()` concat. | Polish ADR for `${}` interp or `\{` escape (carried from Phase 32). |
| Stmt entity has wide unused-field payload for non-block statements. | Accept the bloat; mirrors Token's tradeoff. | Reconsider if/when Phase 35's expression AST forces a real sum-type pattern; may be worth an ADR proposing variant-typed entities for Phase 36. |
