# Phase 40B: Precedence-Aware Paren Stripping

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece B)
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Design decision:** [ADR 0043](../../docs/decisions/0043-stage2-paren-stripping.md)
**Prerequisite:** [Phase 40C](phase-40c-source-order-tracking.md) (source-order tracking)

## Goal

The Phase 38 formatter preserved every `ex_paren` node verbatim — convenient for the MVP but it diverges from stage1's `intentc fmt` which strips redundant parens. Phase 40B adds precedence-aware emit so that `(x)` becomes `x`, `1 + (2 * 3)` becomes `1 + 2 * 3`, and `(a == b) and (c == d)` becomes `a == b and c == d`, while necessary parens like `(1 + 2) * 3` and `a - (b - c)` stay.

This is the second of three sub-pieces (C → B → A) before byte-equal self-format on stage2 files becomes the dogfood gate.

## Success Criteria

- [x] `format_expr` becomes a precedence-aware emitter taking an internal `min_precedence: Int` parameter.
- [x] `binop_precedence(op: String) returns Int` helper exposes the same precedence table as `parser.intent`'s Pratt parser (assign < or < and < eq < cmp < range < add < mul < unary < postfix < primary).
- [x] `binop_is_right_assoc(op: String) returns Bool` returns true only for `=` (assignment).
- [x] `expr_precedence(e: Expr) returns Int` exposes each expression's intrinsic binding power.
- [x] `ex_paren` is stripped at the `format_expr_at` dispatcher: inner forwarded with the same `min_precedence`, so a redundant paren around a sufficiently-binding inner expr disappears.
- [x] LHS / RHS recursion uses the bump rule for associativity (left-assoc: `lhs_min = P, rhs_min = P+1`; right-assoc: swap).
- [x] Call args, index inner, array literal elements, and parenthesised content recurse with `min_prec = 0` (fresh context).
- [x] **8 new tests** cover the matrix: primary, lower-prec necessary, higher-prec redundant, left-assoc chain strips left paren, left-assoc right paren kept, logical operators across precedences, unary operand parens kept, call arg parens unwrap.
- [x] All prior tests pass — 117 → 125 on rust + js.
- [x] `make validate` green.

## Reference

- [ADR 0043](../../docs/decisions/0043-stage2-paren-stripping.md) — design decision
- [Phase 40C PRD](phase-40c-source-order-tracking.md) — preceding sub-piece
- `selfhost/formatter/format.intent` — precedence-aware `format_expr` / `format_expr_at` / `format_expr_inner`
- `selfhost/formatter/format_test.intent` — 8 new paren-stripping tests + the `expr_roundtrips_to` helper

## Design decisions (and why)

### Precedence-aware emit, not AST canonicalisation pass

Documented in [ADR 0043](../../docs/decisions/0043-stage2-paren-stripping.md). Briefly: the formatter is the single source of truth for "what binds how tightly"; pulling paren-stripping into a pre-formatter pass would add an AST mutation step that other tooling (linter, LSP) would need to be aware of and would still need a precedence table somewhere. Keep `ex_paren` in the AST — only the formatter elides it.

### Three-layer dispatch: `format_expr` → `format_expr_at` → `format_expr_inner`

The original `format_expr(e)` becomes a one-line shim `format_expr_at(e, 0)`. The dispatcher `format_expr_at(e, min_prec)` handles paren stripping (special-cases `ex_paren`) and wraps in parens when `expr_precedence(e) < min_prec`. `format_expr_inner(e)` emits the content using its own recursive `format_expr_at` calls. Why three layers:

- `format_expr(e)` keeps the call site for top-level emits trivial — anything that doesn't have a surrounding context just passes the expression.
- `format_expr_at(e, min_prec)` centralises the "should this wrap?" decision in one place. Without this layer, every `format_expr_inner` branch would need to check `min_prec` itself.
- `format_expr_inner(e)` is where each kind picks the right `min_prec` for its children — that logic is intrinsically per-kind.

### Args/index/array elements use `min_prec = 0`, not the postfix precedence

When recursing into the inside of `f(args)`, `obj[idx]`, or `[elem1, elem2]`, the surrounding context is "inside parens" — a fresh re-evaluation that won't fold with anything outside. So min_prec=0. A `let x = f(a + b)` should emit `a + b` inside the parens, not `(a + b)`.

### `ex_unary` operand recurses with `min_prec = 9`, not 10

Unary's intrinsic precedence is 9 (between postfix-10 and `mul`-8). The operand needs to bind at least as tightly as 9 to avoid being misread. `-(1 + 2)` keeps parens; `-x` doesn't. `-x.field` doesn't either since field-access is prec 10 > 9.

### Defensive `ex_paren` branch in `format_expr_inner`

`format_expr_at` strips `ex_paren` before calling `format_expr_inner`, so `format_expr_inner` should never see an `ex_paren`. But defensive coding: if someone calls `format_expr_inner` directly (debugging, a future caller), we emit literally to avoid silently losing the grouping. The branch is a safety net, not normal control flow.

### Test helper `expr_roundtrips_to(rhs_in, rhs_out)`

Each paren test embeds an expression inside a minimal function and asserts the round-tripped output. The helper takes `rhs_in` (the source's expression) and `rhs_out` (the formatter's expected expression). For tests where the input is already canonical, both args are the same; for tests where the formatter normalises (strips parens), they differ. This keeps test sources synthetic and lets us assert formatting changes precisely.

## Surfaced gaps (deferred)

- **Comment preservation (Phase 40A)** — lexer still drops comments. Byte-equal self-format on stage2 files still gated on this.
- **Contract clause emission** — formatter doesn't handle `requires` / `ensures` / `decreases` on function signatures yet. Phase 41 widens the parser surface; formatter emit follows.
- **Method receiver `&mut self` / `self`** — stage1 parser distinguishes these via Phase 32+'s `&mut self` work; stage2 parser doesn't yet emit a flag. Not blocking for stage2 self-format (stage2 source uses methods uniformly).

## Out of scope

- "Stylistic" parens that the user didn't write but a reader might want for readability. Stage1 fmt doesn't add these either.
- Comment-aware paren handling (e.g. `(/* comment */ x)`). The lexer doesn't preserve comments yet.

## Files touched

- `docs/decisions/0043-stage2-paren-stripping.md` — new ADR.
- `selfhost/formatter/format.intent` — added `binop_precedence`, `binop_is_right_assoc`, `expr_precedence`, `format_expr_at`, `format_expr_inner`. `format_expr` becomes a one-line shim to `format_expr_at(e, 0)`.
- `selfhost/formatter/format_test.intent` — 8 new paren-stripping tests + `expr_roundtrips_to` helper.
