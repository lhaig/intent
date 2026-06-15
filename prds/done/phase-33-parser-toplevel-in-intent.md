# Phase 33: Parser Top-Level in Intent

**Status:** Shipped (2026-06-03; commit `3d3fdef`)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 32](phase-32-lexer-in-intent.md) (stage2 lexer)

## Goal

Second Intent-in-Intent file. Parse the top-level shape of an Intent program from the Phase 32 token stream: module declaration, file imports + package imports, function declarations (modifiers + signature + body captured as raw token-stream text). Statement and expression parsing inside function bodies is deferred to Phase 34+.

## Success Criteria

- [x] `selfhost/formatter/parser.intent` exists and type-checks via `intentc check`.
- [x] AST entities defined: `Param`, `ModuleDecl`, `ImportDecl`, `FunctionDecl`, `Program`.
- [x] `Parser` entity holds `tokens: Array<Token>`, `position: Int`, `error: String`. Helpers: `peek_kind`, `peek_text`, `peek_n`, `advance`, `check`, `check_consume`, `expect`, `fail`.
- [x] `parse_module_decl` recognises `module IDENT version "X";`.
- [x] `parse_import_decl` handles `import "path";` (file) and `import name;` / `import name.sub.sub;` (package, including dotted).
- [x] `parse_function_decl` recognises modifiers (`public` / `async` / `entry` in any order) + `function` + name + parameter list + `returns` Type + crudely-skipped contracts + body block.
- [x] `parse_type_name` handles primitives + idents + a single generic-arg list (consumed and replaced by `<...>` suffix on the stored name).
- [x] `consume_braced_body` depth-tracks `{ }` and rebuilds raw body text from token texts.
- [x] `parse_program` orchestrates module → loop on imports + functions → returns `Program` with `error` field populated on failure.
- [x] Falls back with a clear error for unsupported top-level constructs (entity / enum / trait — Phase 34+).
- [x] Top-level `parse(source)` convenience.
- [x] 12 in-language tests covering: minimal module, file imports, package imports, dotted package imports, function with no params, function with params, entry function, public function, multiple functions, generic return-type head, unsupported-top-level error, missing-semicolon error.
- [x] Combined with Phase 32: 25 tests pass on rust + js.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 32 PRD](phase-32-lexer-in-intent.md) — token surface this builds on
- `selfhost/formatter/parser.intent` — the artefact
- `selfhost/formatter/README.md` — directory README + per-phase status

## Tasks (as shipped)

### 33.1 Restructure

**Files:** `selfhost/formatter/lexer.intent`

Removed the placeholder `entry function main` from `lexer.intent` (the test runner synthesises one when the file is the entry of `intentc test`). AST entities kept inline in `parser.intent` rather than a separate `ast.intent` — fewer multi-file moving parts for Phase 33's scope.

### 33.2 Parser entity + state

**Files:** `selfhost/formatter/parser.intent`

`Parser` holds `tokens` + `position` + `error`. Helpers: `peek_kind`, `peek_text`, `peek_n(n)` for lookahead, `advance`, `check(kind)`, `check_consume(kind)`, `expect(kind, what)` for required matches with diagnostic, `fail(message)` that records first error and advances past the offending token (suppresses cascade noise).

### 33.3 `parse_module_decl`

Recognises `module IDENT version STRING;`. `unquote(text)` helper strips the surrounding quotes from the string-literal token.

### 33.4 `parse_import_decl`

Two shapes: file import (`import STRING;`) and package import (`import IDENT (. IDENT)*;`). Dotted package paths get joined with `.` separators in the stored `path` field.

### 33.5 `parse_function_decl`

Loop over optional modifiers in any order, then `function`, name, `(` params `)`, `returns` Type, crudely-skipped contracts, body block.

`parse_type_name` handles primitives (`Int`, `String`, etc.), bare identifiers, and a single generic-argument list. Generic args are consumed by depth-tracking `<` and `>` and replaced by `<...>` suffix on the stored name string. A real `TypeRef` AST is Phase 34+.

`consume_braced_body` depth-tracks `{` and `}` so nested braces don't terminate early; rebuilds the body text by joining each consumed token's `text` with single spaces. Phase 34 replaces this with real statement parsing.

### 33.6 `parse_program`

Orchestrates: `parse_module_decl` then loop on imports + functions until `at_end()` or `error != ""`. `is_at_function_start` recognises a function-decl prefix (`function` / `entry` / `public` / `async`). Unsupported top-level constructs (entity/enum/trait) trigger `fail` with a clear message.

### 33.7 In-language tests

12 `test "..." { ... }` blocks covering each shape + error paths. Tests with literal braces in fixture source use `'{'.to_string() + ...` concat (same pattern as Phase 32).

### 33.8 Gap surfacing + docs

Two real Rust-backend bugs surfaced and fixed:

1. **`ReturnStmt` value cloning.** `return self.tokens[self.position];` failed to compile because indexing `Vec<Token>` is a move. Fix: mirror the existing `AssignStmt` clone logic into the return path. Field-access and index-access return values get auto-`.clone()` when the type isn't Copy.

2. **Constructor entity-typed field defaults.** `EntityName { /* default fields */ }` placeholder doesn't compile. Fix: hoist top-level `self.field = expr` assignments out of the constructor body and into the struct literal directly via new `splitConstructorInits` helper. Common pattern (`field x: T; ctor(x: T) { self.x = x; }`) now compiles cleanly; fields without a direct assignment fall back to `defaultValue`.

Lexer additions:
- `import`, `public`, `async` keywords with `_marker` suffix on kind functions to avoid clashing with Intent's own reserved-word handling when this file itself is parsed.

Workarounds documented (not fixed):
- `version` is reserved → renamed `ModuleDecl.version` → `module_version`.
- Cross-module entity types must be unqualified (`Token`, not `formatter_lexer.Token`).
- Constructor double-use of a `String` parameter triggers borrow error → restructured constructor to use the param once.

ROADMAP entry added; NEXT-STEPS rewritten with the gap-tracking table.

## Out of Scope (deferred)

- **Statements and expressions inside function bodies** — Phase 34 (statements), Phase 35 (expressions).
- **Entity / enum / trait / impl / intent / extern / test declarations.**
- **Real `TypeRef` AST** — Phase 34+ replaces the string-with-`<...>`-suffix approximation.
- **Source-fidelity body text** — current capture rejoins token texts with single spaces, losing original whitespace.

## Validation

- `make validate` green.
- `intentc test selfhost/formatter/parser.intent` 12/12 on rust.
- `intentc test --target js selfhost/formatter/parser.intent` 12/12 on js.
- Combined 25/25 with Phase 32 lexer.

## Actual size

~500 LOC in `selfhost/formatter/parser.intent`, plus ~120 LOC of backend fixes in `internal/rustbe/rustbe.go`, plus 15 LOC of lexer keyword additions.
