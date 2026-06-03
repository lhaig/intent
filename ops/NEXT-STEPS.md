# Pickup Notes — 2026-06-03 (after Phase 34)

Handoff after the third stage2 deliverable shipped.

## Where we are

Today's session shipped (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** — strategic frame + first language-gap design.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent.
- **Phase 34** — Statement parser in Intent (this session).

Stage2 (`selfhost/formatter/`) now has two files (`lexer.intent` + `parser.intent`, ~1100 LOC combined) and **38 passing in-language tests on rust + js**. The parser produces a `Program` AST containing a `ModuleDecl`, an `Array<ImportDecl>`, and an `Array<FunctionDecl>` — and each `FunctionDecl.body` is now a real `Block` of `Stmt` nodes (`let` / `return` / `if`-`else` / `while` / expression statements). Expressions inside statements are still captured as raw token-joined `String` text (depth-balanced bracket scan); a real expression parser is Phase 35.

## Language gaps surfaced and fixed in Phase 34

None — no language or backend changes were needed this phase. Phase 33's constructor-field-hoist fix already covers the new `Stmt` / `Block` entities cleanly.

## Workarounds applied this phase (no fix)

| Gap | Workaround | Suggested follow-up |
|---|---|---|
| Bare `Block([])` literal can't infer element type at the call site. | `empty_block()` free function returns a typed `Array<Stmt>` then constructs the `Block`. | Future: array-literal element-type inference from constructor signature, or a `Block::empty()` static method (needs static-method syntax in v1, currently absent). |
| `Stmt` carries unused payload fields for non-block stmts. | Accept the bloat; mirrors Token's tradeoff. | Reconsider with an ADR if/when Phase 35's expression AST forces a real sum-type pattern. |

Carried-over gaps from Phase 32 / 33 still open (workarounds applied, fixes deferred):
- `{` / `}` in string literals trigger interpolation parsing → use `'{'.to_string()` concat.
- `let _:` rejected → use expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity types can't be qualified (`module.Entity`) → use bare entity names from imports.
- Constructor double-use of a `String` parameter triggers borrow error → restructure to single-use.
- `s.to_int()` parse missing → not needed yet (parser stores expression text raw).
- Multi-line `/* */` comments not skipped by stage2 lexer.
- Char / float literals not in stage2 lexer.
- String interpolation tokenisation not split into parts.

## Immediate next step

**Phase 35 — Expression parser in Intent.** Replace `capture_expr_until_semi` / `capture_expr_until_lbrace` with a real Pratt / precedence-climbing parser. Build an `Expr` AST node (likely the same Int-tagged shape as `Stmt` and `Token`) covering:

- Literals: int, string, bool, char (when stage2 lexer supports it).
- Identifier references + qualified paths (`module.func`, `self.field`).
- Function calls + method calls.
- Indexing (`a[i]`) + slicing (`a[i..j]`).
- Field access (`x.f`).
- Unary operators (`-x`, `not x`).
- Binary operators with correct precedence: `*` / `/` / `%`, `+` / `-`, `<` / `>` / `<=` / `>=`, `==` / `!=`, `and`, `or`.
- Parenthesised expressions.
- Array literals (`[a, b, c]`).
- Range expressions (`a..b`).

Out of scope (probably Phase 36 or later): lambdas, `match` expressions, interpolated strings (needs lexer support), `await` / `try`, struct/entity literals.

Recommended scope:
1. New `Expr` AST entity (kind-tagged) + kind constants (`ex_int`, `ex_ident`, `ex_call`, `ex_binop`, `ex_index`, …).
2. Pratt-style `parse_expr` with a `parse_unary` and `parse_primary`.
3. Replace the two `capture_expr_until_*` helpers in the statement parsers with `parse_expr(stopper)` calls — the stopper is `;` for statements and `{` for if/while conditions.
4. Stmt fields like `expr_text: String` become `expr: Expr` (or similar). Update tests.
5. New tests covering each operator + precedence.

Likely new gaps Phase 35 will surface:
- `s.to_int()` if we promote integer-literal token text to actual `Int` values.
- Reserved-word tightness — `Expr` as a struct field name might collide.
- Float literals at the lexer level.
- Possibly mutually-recursive entity depth limits if `Expr` recurses through too many fields by value (unlikely — `Array<Expr>` for argument lists keeps recursion behind the heap).

## Other candidates (orthogonal)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()`, `s.index_of`, `s.replace`, Unicode-aware predicates.
- **Phase 17.G — WASM test runner**, **17.H — coverage**.
- **Phase 23 — VS Code Marketplace publish**.
- **ADR 004x — Package registry signing**.

## Memory state

Four durable items hold (unchanged):
- `project_intent_is_a_new_language`.
- `feedback_write_adrs_along_the_way`.
- `feedback_minimise_mistakes_in_autonomous_runs`.
- `project_self_hosting_priority`.

## How to resume

1. `git log --oneline -10` for recent landings.
2. `aiki task` for the open task list.
3. Recommended start: re-read `selfhost/formatter/parser.intent` to refamiliarise with the Stmt/Block shape, then begin Phase 35. Build `Expr` first, then thread it through the statement parsers in place of `String` expression text. Mirror the Phase 34 pattern (Int discriminator + per-kind fields).
