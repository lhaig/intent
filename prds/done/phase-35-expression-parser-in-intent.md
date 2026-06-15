# Phase 35: Expression Parser in Intent

**Status:** Shipped (2026-06-03)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 34](phase-34-statement-parser-in-intent.md) (stage2 statement parser)

## Goal

Replace Phase 34's depth-balanced raw-text expression capture with a real precedence-climbing parser that produces an `Expr` AST. Stage2 can now structurally understand the inside of every statement — assignments, calls, method chains, field access, indexing, slicing, array literals, ranges, and binary/unary operators with proper precedence.

## Success Criteria

- [x] `Expr` entity with `kind: Int` discriminator + payload fields (`int_value`, `str_value`, `bool_value`, `name`) + `children: Array<Expr>`. Same Int-tagged shape as `Token` / `Stmt` / `Block`. Mutual-recursion safety: every back-edge from a child entity to `Expr` runs through `Array<Expr>` (heap-allocated).
- [x] Expression kind constants: `ex_void` (placeholder for absent expr), `ex_int`, `ex_string`, `ex_bool`, `ex_ident`, `ex_unary`, `ex_binop`, `ex_call`, `ex_index`, `ex_field`, `ex_array`, `ex_range`, `ex_paren`.
- [x] `Stmt.expr: Expr` replaces Phase 34's `expr_text: String`. All callers updated. `return;` uses `empty_expr()` (kind `ex_void`).
- [x] Pratt / precedence-climbing parser layered as `parse_assign > parse_or > parse_and > parse_eq > parse_cmp > parse_range > parse_add > parse_mul > parse_unary > parse_postfix > parse_primary`.
- [x] Assignment (`=`) is parsed as the lowest-precedence right-associative binop. Statement-level assignments (`y = y + 1;`) and chained ones (`a = b = c;`) both parse.
- [x] Range (`..`) is non-associative, single-occurrence between two `parse_add` operands.
- [x] Postfix loop: function call `e(args)`, indexing `e[expr]`, field access `e.field`. Method calls fall out naturally as `call(field(obj, "m"), args...)`. Qualified module paths (`formatter_lexer.tk_eof()`) parse the same way.
- [x] Primary expressions: int literal, string literal (quotes stripped via existing `unquote`), `true`/`false`, identifier, parenthesised expression, array literal `[a, b, c]` (including empty `[]`).
- [x] `parse_int(s: String) returns Int` free function converts a decimal-digit token text to an Int via codepoint arithmetic — no `String.to_int()` builtin required.
- [x] `empty_expr()` helper returns an `Expr` with kind `ex_void` for the `return;` case.
- [x] Phase 34's `capture_expr_until_semi` / `capture_expr_until_lbrace` removed; they're no longer reachable.
- [x] All Phase 34 tests updated to assert against the new Expr structure (`stmt.expr.kind`, `stmt.expr.children`, `stmt.expr.int_value`, etc.) instead of the raw `expr_text` String. 22 new Phase 35 tests covering literals, precedence, associativity, calls, method chains, indexing, slicing, ranges, array literals, parens, and the missing-expression error case.
- [x] Combined: 60/60 passing on rust + js.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 34 PRD](phase-34-statement-parser-in-intent.md) — statement parser this builds on
- `selfhost/formatter/parser.intent` — the artefact

## Tasks (as shipped)

### 35.1 Expr AST shape

**Files:** `selfhost/formatter/parser.intent`

Added a single `Expr` entity tagged by `kind: Int`. Children are stored in `Array<Expr>`, broken out per-kind by convention (documented in the entity comment): `ex_unary` → `[operand]`, `ex_binop` → `[lhs, rhs]`, `ex_call` → `[callee, arg1..argN]`, `ex_index` → `[object, index]`, `ex_field` → `[object]` (+ `name` holds the field), `ex_array` → `[elem1..elemN]`, `ex_range` → `[start, end]`, `ex_paren` → `[inner]`. Scalar payload kinds (`ex_int` / `ex_string` / `ex_bool` / `ex_ident`) use the appropriate top-level field and leave `children` empty.

The same Int-discriminator-plus-children pattern Token and Stmt use. No language gap surfaced.

### 35.2 Statement integration

`Stmt.expr_text: String` → `Stmt.expr: Expr`. The five statement parsers (`parse_let_stmt`, `parse_return_stmt`, `parse_if_stmt`, `parse_while_stmt`, `parse_expr_stmt`) all call `parse_expr()` instead of the depth-balanced text-capture helpers. `return;` synthesises an `empty_expr()` (kind `ex_void`) so callers can distinguish "no expression" from "expression that happens to be `0`".

### 35.3 Pratt-style parser

```
parse_expr   = parse_assign
parse_assign = parse_or ( '='  parse_assign )?                (right-assoc, single)
parse_or     = parse_and ( 'or'  parse_and )*
parse_and    = parse_eq  ( 'and' parse_eq  )*
parse_eq     = parse_cmp ( ('=='|'!=')             parse_cmp   )*
parse_cmp    = parse_range ( ('<'|'>'|'<='|'>=')   parse_range )*
parse_range  = parse_add ( '..' parse_add )?                  (non-assoc, single)
parse_add    = parse_mul ( ('+'|'-')         parse_mul   )*
parse_mul    = parse_unary ( ('*'|'/'|'%')   parse_unary )*
parse_unary  = ('-' | 'not') parse_unary | parse_postfix
parse_postfix = parse_primary ( '(' args ')' | '[' expr ']' | '.' ident )*
parse_primary = INT_LIT | STR_LIT | 'true' | 'false' | IDENT
            | '(' parse_expr ')' | '[' array_body ']'
```

Each layer is a small while-loop (or single-shot `if` for the non-associative cases). Operator helpers `is_cmp_op` / `is_mul_op` group the multi-token checks where the inline `or` chain would get noisy. The postfix loop centralises call / index / field — method calls fall out naturally as `call(field(obj, "m"), args...)`.

### 35.4 `parse_int` in-Intent

Phase 35 needs to turn the lexer's tk_int token text (e.g. "42") into an actual `Int` value to populate `Expr.int_value`. There's no `String.to_int()` builtin in stage1, so a small in-Intent helper walks codepoints and multiplies through, leaning on Phase 31's `Char.to_codepoint()` and `Char.is_digit()`. The lexer guarantees runs of digit chars only (no leading sign), so error handling is minimal.

### 35.5 Test rewrite

All Phase 34 tests that asserted on `stmt.expr_text == "..."` now assert against the parsed `Expr` structure: `stmt.expr.kind == ex_binop()`, `stmt.expr.name == ">"`, `stmt.expr.children[0].name == "x"`, etc. The Phase 34 `expression with balanced parens captures correctly` test (which checked the surface-level token-joined string) is replaced by `call expression with arguments parses as ex_call` which checks the actual AST. The `else if` test source originally used `return -1;` (would parse as `unary("-", int(1))` and that's correct), but switching to `return 0 - 1;` makes the test deterministic on the literal-vs-unary-minus question for now.

22 new tests cover: `parse_int` unit, integer literal, bool literals, string literal, unary minus / not, precedence (`1 + 2 * 3`, comparison-tighter-than-and), parens overriding precedence, right-associative assignment, field access, method call, indexing, slicing as `index(_, range(_, _))`, array literal (including empty), no-arg call, nested call, left-associative method chain, qualified module-path call, equality / inequality / `<=`, and a missing-expression error case.

### 35.6 Helper functions

Two small public test-helpers (`single_let_rhs(prog)` and `wrap_in_let(rhs_text)`) keep the Phase 35 test fixtures terse. They're `public function`s so they live alongside the production helpers; this is fine in stage2 because the file isn't published as a library yet.

### 35.7 No language / backend changes needed

Worth noting: this phase added ~600 LOC of new Intent code (parser + tests) and zero LOC of Go-side work. The constructor-field-hoist fix (Phase 33), the `&mut self` propagation (Phase 32), and Phase 31's `Char` ergonomics all carried this phase. The framework is now mature enough that several phases worth of stage2 work can ship without language-surface changes.

## Out of Scope (deferred)

- **Lambdas** (`(n) => n + 1`) — Phase 36+. Requires `=>` already in lexer; need parser hooks.
- **`match` expressions** — Phase 36+. Will likely need a new expression kind plus arm parsing.
- **Interpolated string parts** — needs lexer support for tokenising `${expr}` segments separately. Carried-over Phase 32 gap.
- **Char literals / float literals** in expressions — lexer doesn't tokenise them yet. Carried-over.
- **`await` / `try` / `break` / `continue` / `for`** — Phase 36+.
- **Bit operators (`&`, `|`, `^`, `<<`, `>>`)** — not in Intent's surface grammar yet.
- **Splitting AST into `selfhost/formatter/ast.intent`** — file is 1404 LOC; Phase 36 (entity / trait parsing) is the natural split point.

## Validation

- `make validate` green (Go unit tests + examples).
- `intentc test selfhost/formatter/parser.intent` 60/60 on rust.
- `intentc test --target js selfhost/formatter/parser.intent` 60/60 on js.

## Actual size

~600 added LOC in `selfhost/formatter/parser.intent`: ~150 LOC of new entity / kind constants / helpers, ~200 LOC of parse methods, ~250 LOC of tests (incl. rewritten Phase 34 tests). No Go / backend changes.

## Surfaced gaps (workarounds applied; no fix this phase)

None new. Carried-over gaps from earlier phases (multi-line comments, char/float literals in stage2 lexer, interpolation tokenisation) still apply but didn't bite this phase.
