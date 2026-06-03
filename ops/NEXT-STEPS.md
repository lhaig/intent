# Pickup Notes — 2026-06-03 (after Phase 35)

Handoff after the fourth stage2 deliverable shipped.

## Where we are

Today's session shipped (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** — strategic frame + first language-gap design.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent.
- **Phase 34** — Statement parser in Intent.
- **Phase 35** — Expression parser in Intent (this session).

Stage2 (`selfhost/formatter/`) now has two files (`lexer.intent` + `parser.intent`, ~1900 LOC combined) and **60 passing in-language tests on rust + js**. The parser produces a fully structured AST:
- `Program { module_decl, imports, functions, error }`
- `FunctionDecl { ..., body: Block }`
- `Block { stmts: Array<Stmt> }`
- `Stmt { kind, expr: Expr, name, type_name, is_mutable, then_block, else_block, has_else }`
- `Expr { kind, int_value, str_value, bool_value, name, children: Array<Expr> }`

Expressions are fully parsed (literals, identifiers, calls, method chains, field access, indexing, slicing, ranges, array literals, unary + binary operators with proper precedence, parens). Assignments are parsed as right-associative `=` binops at the lowest precedence.

## Language gaps surfaced and fixed in Phase 35

None — zero Go / backend changes were needed. Earlier phases' groundwork (Char ergonomics from Phase 31, `&mut self` propagation from Phase 32, constructor-field-hoist from Phase 33) carried this phase end-to-end. The framework is mature enough that several stage2 phases can ship without language changes.

## Workarounds applied this phase

| Gap | Workaround | Suggested follow-up |
|---|---|---|
| No `String.to_int()` builtin. | In-Intent `parse_int(s)` walks codepoints using `Char.to_codepoint` + `Char.is_digit`. | Optional ADR: add `String.to_int(): Result<Int, String>` and `String.parse_int(): Int` (panics on bad input) to the surface. Marginal value vs. the in-Intent helper. |

Carried-over gaps still open (no fix this phase, no new bite):
- `{` / `}` in string literals trigger interpolation parsing → use `'{'.to_string()` concat.
- `let _:` rejected → use expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity types can't be qualified (`module.Entity`) → use bare entity names from imports.
- Constructor double-use of a `String` parameter triggers borrow error → restructure to single-use.
- Multi-line `/* */` comments not skipped by stage2 lexer.
- Char / float literals not in stage2 lexer.
- String interpolation tokenisation not split into parts.
- Bare `Block([])` / `Expr` literals can't infer element type → use `empty_block()` / `empty_expr()` helpers.

## Immediate next step

**Phase 36 — Top-level declarations in Intent (entity / enum / trait / impl / intent / test / extern).** Stage2 parser currently only handles module / import / function declarations at the top level. Phase 36 covers the rest of the declaration grammar so the parser can ingest real Intent files end-to-end (including its own source).

Recommended scope:

1. **Split AST into `selfhost/formatter/ast.intent`.** `parser.intent` is now 1404 LOC and pure-AST entities mixed in with parsing logic obscure the structure. The split is the natural prelude to adding new declaration kinds. Carries the cross-module entity-types gap (workaround: import bare names, same as Phase 33 does with `Token`).

2. **New declaration AST entities:**
   - `EntityDecl { name, is_public, fields: Array<FieldDecl>, methods: Array<FunctionDecl>, constructor: Constructor (optional), invariants: Array<Expr> }`
   - `FieldDecl { name, type_name, default_expr: Expr (optional) }`
   - `EnumDecl { name, is_public, variants: Array<EnumVariant> }`
   - `EnumVariant { name, params: Array<Param> }`
   - `TraitDecl { name, methods: Array<TraitMethodSig> }`
   - `ImplDecl { trait_name, entity_name, methods: Array<FunctionDecl> }`
   - `IntentBlock { goal: String, verified_by: Array<String> }`
   - `TestDecl { name: String, body: Block }`
   - `ExternDecl { name, params, return_type, target_path }`

3. **Update `Program`** to hold `Array<EntityDecl>`, `Array<EnumDecl>`, etc., or a single `Array<TopLevelDecl>` with a kind discriminator. The latter is simpler.

4. **New parse methods** per declaration form. Reuse `parse_block`, `parse_expr`, `parse_function_decl` heavily.

5. **Tests** covering each declaration kind + the round-trip of `parser.intent` itself (parse the stage2 parser source with the stage2 parser — first dogfood checkpoint).

Likely new gaps Phase 36 may surface:
- Type-argument parsing: `Array<T>`, `Result<T, E>`, `Map<K, V>` currently stored as `"Array<...>"` string. A real `TypeRef` AST might be needed for the formatter to round-trip types correctly. Could be deferred to Phase 37.
- Cross-module imports of new entity types from `ast.intent`.
- Constructor parsing — `constructor(args) { body }` syntax inside an `entity` block.

## Other candidates (orthogonal)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()`, `s.index_of`, `s.replace`, Unicode-aware predicates.
- **Phase 17.G — WASM test runner**, **17.H — coverage**.
- **Phase 23 — VS Code Marketplace publish**.
- **ADR 004x — Package registry signing**.

## Memory state

Durable items (unchanged this phase):
- `project_intent_is_a_new_language`.
- `feedback_write_adrs_along_the_way`.
- `feedback_minimise_mistakes_in_autonomous_runs`.
- `project_self_hosting_priority`.
- `feedback_document_and_push_after_each_phase`.

## How to resume

1. `git log --oneline -10` for recent landings.
2. `aiki task` for the open task list.
3. Recommended start: open `selfhost/formatter/parser.intent` to refresh on the AST shape (especially `Stmt` and `Expr`), then begin Phase 36. Suggested first move: split AST entities + kind constants into `ast.intent` (read-only refactor; tests should be unaffected) before adding new declaration kinds. After the split, `parser.intent` keeps the `Parser` entity + parse methods + helpers + tests; `ast.intent` holds the entity declarations + kind constants.
