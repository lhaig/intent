# 0043: Stage2 Formatter Paren Stripping via Precedence-Aware Emit

**Date:** 2026-06-09
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece B)

## Context

Stage1's Go formatter strips redundant parens. Given `(1 + 2) * 3`, the parens stay because `*` binds tighter than `+` and removing them would change meaning. Given `(1 + 2);` the parens go because nothing surrounds them; given `1 + (2 * 3)` the parens go because `*` already binds tighter without them.

Stage2's parser preserves user-written parens as `ex_paren` AST nodes. The Phase 38 formatter emits every `ex_paren` verbatim — convenient for the MVP, but it diverges from stage1 fmt on any program where the user wrote redundant parens. Byte-equal self-format on `selfhost/formatter/parser.intent` (which has many `(x + y)` expressions in tests) is impossible until the formatter strips parens the same way stage1 does.

This ADR records the choice of *how* to strip redundant parens.

## Decision

`format_expr` becomes precedence-aware: it takes an extra `min_precedence: Int` parameter representing the binding power required by the surrounding context. When emitting an `ex_paren(inner)` node, the formatter doesn't emit parens unconditionally — it asks whether `inner` would already parse correctly at `min_precedence` and elides the parens when the answer is yes.

A separate helper `expr_precedence(e: Expr) returns Int` returns the binding power of an expression (using the same precedence table as `parser.intent`'s Pratt parser).

The check at the recursion boundary is: when formatting a sub-expression at `min_precedence = M`, if the sub-expression's intrinsic precedence is below `M`, wrap it in parens; otherwise emit unwrapped. `ex_paren` nodes are stripped at this layer: their content is forwarded to `format_expr(inner, min_precedence)` and the deeper logic decides whether parens are needed.

## Considered alternatives

| Option | Description | Trade-off |
|---|---|---|
| **(1) Precedence-aware emit** [**chosen**] | `format_expr(e, min_prec)`. Single pass, paren decisions made at recursion sites. | Conventional pretty-printer approach. Localised to `format_expr` and a precedence helper. ~50-line addition. |
| **(2) AST canonicalisation pass** | Pre-walk strips redundant `ex_paren` nodes before formatting. | Cleaner separation; formatter stays paren-blind. But adds an AST mutation pass that other tooling (linter, future LSP) would need to know about, and the precedence logic still has to live somewhere. |
| **(3) Always strip everything, then re-add by re-parsing** | Format without parens, re-parse to verify, add parens back where the re-parse diverges. | Worst of both — slow, two-pass, and the re-parse heuristic is fragile. Rejected. |

### Why (1) over (2)

(1) follows the standard pretty-printer recipe documented in most compiler textbooks (Wadler, Hughes, the Go fmt source). The formatter is the single source of truth for "what binds how tightly," which matches stage1's structure. (2) is cleaner if other tools also need paren-stripped ASTs, but right now only the formatter needs that — and even a future linter would want the user's parens preserved for diagnostics ("redundant parens here") rather than silently stripped. Keep `ex_paren` in the AST; let only the formatter elide it.

### Associativity matters

The check `child_precedence < min_precedence` works for non-associative operators but is wrong for associative ones. For a left-associative operator at precedence P:

- `(a op b) op c` is the natural left grouping. Recursing into LHS with `min_prec = P` accepts a same-precedence binop child without paren — correct.
- `a op (b op c)` is right grouping; if `op` is left-associative, this changes semantics. Recursing into RHS with `min_prec = P+1` rejects a same-precedence binop child — paren retained — correct.

For right-associative operators (just `=` / assignment in Intent), the bumps are swapped.

The chosen min-precedence-on-recursion rule is therefore:

| Operator class | LHS recurse with | RHS recurse with |
|---|---|---|
| Left-associative binop at prec P | `min_prec = P` | `min_prec = P + 1` |
| Right-associative binop at prec P | `min_prec = P + 1` | `min_prec = P` |
| Unary at prec P | operand: `min_prec = P` | — |
| Postfix (call / index / field) at prec P | callee: `min_prec = P` | inside-parens (call args, index expr): `min_prec = 0` (top-level context, full re-evaluation) |

The "+1 bump" trick is the same one used in textbook recursive-descent pretty-printers (Hughes' "A new approach to combining algorithms," and similar treatments in the Wadler papers).

## Precedence table

Mirrors `parser.intent`'s Pratt parser:

| Precedence | Operators | Associativity |
|---|---|---|
| 1 | `=` (assign) | right |
| 2 | `or` | left |
| 3 | `and` | left |
| 4 | `==`, `!=` | left |
| 5 | `<`, `<=`, `>`, `>=` | left |
| 6 | `..` | non |
| 7 | `+`, `-` (binary) | left |
| 8 | `*`, `/`, `%` | left |
| 9 | unary `-`, `not` | prefix |
| 10 | call, index, field-access (postfix) | left |
| 11 | primary (literals, idents, parens) | — |

`min_precedence = 0` is the top-level context: nothing requires parens to bind here.

## Consequences

### Positive

- Stage2 formatter matches stage1 on paren stripping for the common cases (`(x)` → `x`, `(1 + 2) * 3` preserved, `1 + (2 * 3)` → `1 + 2 * 3`).
- `format_expr` precedence parameter is reusable for any future output target (a stage2 lint diagnostic that wants to render an expression, etc.).
- `ex_paren` remains in the AST so a future linter / LSP can still see user-written parens for diagnostics.

### Negative

- Every `format_expr` call site now needs to think about the surrounding context's `min_precedence`. Most callers will pass 0 (top-level) but call/index/field sites must pass 10.
- A bug in the precedence table or the bump rule could change program meaning (e.g. emitting `a - b - c` for source `a - (b - c)`). Tested via explicit roundtrip assertions in `format_test.intent`.

### Neutral

- `ex_paren` nodes stay in the parser output. Other consumers of the AST are unaffected.

## Out of scope

- Stripping parens added around assignment statements like `let x = (1 + 2);`. The let statement formatter is independent of the expression formatter and doesn't currently produce wrapping parens.
- Adding "stylistic" parens that the user didn't write but a strict reader might want. Stage1 fmt doesn't do this either.

## Reference

- ADR 0040 — strategic frame for self-hosted formatter
- ADR 0042 — source-order tracking
- Phase 38 PRD — formatter MVP (paren-blind emit it replaces)
- `parser.intent` — Pratt parser whose precedence table this mirrors
