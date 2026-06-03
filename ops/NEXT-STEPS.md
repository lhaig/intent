# Pickup Notes — 2026-06-03 (after Phase 33)

Handoff after the second stage2 file shipped.

## Where we are

Today's session shipped (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** — strategic frame + first language-gap design.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent (this session).

Stage2 (`selfhost/formatter/`) now has two files (`lexer.intent` + `parser.intent`, ~900 LOC combined) and 25 passing in-language tests on rust + js. The parser produces a `Program` AST containing a `ModuleDecl`, an `Array<ImportDecl>`, and an `Array<FunctionDecl>` with function bodies captured as raw token-stream text.

## Language gaps surfaced and fixed in Phase 33

| Gap | Status |
|---|---|
| Rust backend: `ReturnStmt` value expressions weren't auto-cloned for non-Copy field-access / index-access. | **Fixed** — mirrored the `AssignStmt` clone logic into the return path. |
| Rust backend: entity-typed fields emit invalid `EntityName { /* default fields */ }` placeholders. | **Fixed** — hoist top-level `self.field = expr` assignments out of the constructor body and into the struct literal. Works for the common pattern of one-shot constructor assignments. |
| Lexer keywords missing for `import` / `public` / `async`. | **Added to stage2 lexer** with `_marker` suffix to avoid clashing with Intent's own reserved-word handling. |

## Language gaps still open (workarounds applied)

| Gap | Workaround | Suggested follow-up |
|---|---|---|
| `version` is reserved — can't use as a field name. | Renamed `ModuleDecl.version` → `module_version`. | Future polish ADR could narrow the reserved set. |
| `{` in string literals triggers interpolation. | Use `'{'.to_string()` concatenation. | Polish ADR for `${}` interp or `\{` escape. |
| `let _:` rejected. | Use expression statements (`self.advance();`). | Polish ADR for `_` binding. |
| Cross-module entity types can't be qualified (`module.Entity` is rejected). | Use bare `Token` from imported `formatter_lexer` module. | Documentation note — currently works because public entities are flat-namespaced. |
| Constructor double-use of `String` parameter triggers borrow error. | Restructure constructor to use the parameter once. | Future backend polish: auto-`.clone()` on second use. |
| `s.to_int()` parse missing. | Token-text Int parsing not yet needed — parser stores raw text. | Add when Phase 35 (expression parser) hits integer literals. |
| Multi-line `/* */` comments not skipped by stage2 lexer. | Stage2 lexer only handles `//` for now. | Add in a follow-up. |
| Char literals, float literals not in stage2 lexer. | n/a yet. | Add when parser phases need them. |
| String interpolation tokenisation not split into parts. | Whole quoted run is one tk_string. | Address when expression parsing reaches interpolated strings. |

## Immediate next step

**Phase 34 — Statement parser in Intent.** Parse the inside of function bodies into AST statement nodes: `let`, `return`, `if`/`else`, `while`, expression statements. Don't parse full expressions yet (Phase 35); use a stub that just captures the expression's raw text by depth-balancing brackets.

Recommended scope:
1. New AST entities in `parser.intent` (or a new `ast.intent` if the file grows too big): `LetStmt`, `ReturnStmt`, `IfStmt`, `WhileStmt`, `ExprStmt`, `Block`.
2. Wire `parse_function_decl`'s body capture into a `parse_block` that produces a real `Block` instead of raw text.
3. Statement-level parse methods + tests.

Likely new gaps Phase 34 will surface:
- An AST node shape problem if we want one `Statement` sum type containing all stmt kinds — Intent enums with single-expression match arms get awkward. We may end up with separate `LetStmt`/`ReturnStmt`/... entities tracked via Int-tagged unions, similar to Token.kind.
- `s.to_int()` if we start interpreting integer literals as actual Int values.

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
3. Recommended start: open `selfhost/formatter/parser.intent` to remind yourself of the AST shape and parser plumbing, then begin Phase 34. Statement nodes + a Block AST node + a parse_block method that replaces the current raw-text body capture.
