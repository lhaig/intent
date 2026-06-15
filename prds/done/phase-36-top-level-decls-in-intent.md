# Phase 36: Top-Level Declarations in Intent + AST Split

**Status:** Shipped (2026-06-03)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 35](phase-35-expression-parser-in-intent.md) (stage2 expression parser)

## Goal

Stage2 parser handled only module / import / function decls at the top level after Phase 35. Phase 36 fills in the rest of the declaration grammar so the parser can ingest the structural surface of any Intent program: entities (with fields, constructors, methods, and invariant blocks), enums (unit + data variants), traits (method signatures), impls (trait + entity + method bodies), test declarations, extern declarations, and intent blocks. Also splits the AST entities out of `parser.intent` into a sibling `ast.intent` so the parsing logic and the AST shape can evolve independently.

## Success Criteria

- [x] `selfhost/formatter/ast.intent` exists, contains every AST entity (Param, Expr, Stmt, Block, FunctionDecl, ImportDecl, ModuleDecl, FieldDecl, EntityDecl, EnumDecl, EnumVariant, TraitDecl, TraitMethodSig, ImplDecl, IntentBlock, TestDecl, ExternDecl, Program), every kind constant (`st_*` / `ex_*`), and the AST-side helpers (`empty_block`, `empty_expr`, `empty_function_decl`, `empty_program`, `parse_int`).
- [x] `parser.intent` no longer declares the AST entities — it imports them from `ast.intent` (alongside the existing `lexer.intent` import).
- [x] Stage2 lexer (`lexer.intent`) gains 12 new keywords (`entity`, `enum`, `trait`, `impl`, `test`, `extern`, `intent`, `field`, `constructor`, `method`, `invariant`, `for`), each with `_marker` suffix on the kind function to avoid Intent's reserved-word handling when the lexer itself is parsed (precedent: Phase 33's `import`/`public`/`async`).
- [x] `Program` entity carries parallel arrays per declaration kind: `imports`, `functions`, `entities`, `enums`, `traits`, `impls`, `intent_blocks`, `tests`, `externs`.
- [x] Parser methods for each new declaration: `parse_entity_decl`, `parse_field_decl`, `parse_constructor_decl`, `parse_method_decl`, `parse_invariant_block`, `parse_enum_decl`, `parse_enum_variant`, `parse_trait_decl`, `parse_trait_method_sig`, `parse_impl_decl`, `parse_test_decl`, `parse_extern_decl`, `parse_intent_block`.
- [x] `parse_program` dispatches on the leading keyword for each top-level construct. `public` is consumed as a modifier and dispatches to entity/enum/trait/function with `is_public=true`.
- [x] `FunctionDecl` gets an `is_constructor: Bool` field so the same entity can represent free functions, methods, and constructors. Existing call sites updated.
- [x] 14 new in-language tests covering each new declaration kind plus a "mixed declarations in any order" test and a synthetic dogfood test that parses a small entity-with-tests fixture.
- [x] Combined: 74/74 passing on rust + js (60 from Phases 32-35 + 14 new).
- [x] `make validate` green.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 35 PRD](phase-35-expression-parser-in-intent.md) — expression parser this builds on
- `selfhost/formatter/ast.intent` — AST entities and helpers
- `selfhost/formatter/parser.intent` — Parser + parse methods + tests
- `selfhost/formatter/lexer.intent` — tokens with new Phase 36 keywords

## Design decisions (and why)

### Split into `ast.intent`

Phase 35 ended with `parser.intent` at 1404 LOC, mixing entity declarations and parsing logic. Phase 36 adds ~400 LOC of new entities + ~500 LOC of new parsers; without a split that would push `parser.intent` past 2300 LOC and make navigation tedious. Splitting now (before adding the new entities) keeps the diff narrower and the rationale clearer.

The split goes through cleanly because Intent allows cross-module entity references without qualification (the Phase 33 precedent: `Token` from `formatter_lexer` is used bare in `parser.intent`). The cost is that cross-module *function* calls do need qualification — `formatter_ast.empty_block()` rather than `empty_block()`. The bulk-qualify after the move was ~100 mechanical edits.

### `Program` uses parallel arrays per kind, not a single `Array<TopLevelDecl>`

Two viable shapes for the top-level decl list:

1. **Parallel arrays** (chosen): `imports: Array<ImportDecl>`, `functions: Array<FunctionDecl>`, `entities: Array<EntityDecl>`, etc. Source order across kinds is lost; order within each kind is preserved.
2. **Single discriminated list**: `decls: Array<TopLevelDecl>` where `TopLevelDecl { kind: Int, idx: Int }` indexes into the typed arrays.

Option (1) matches the Phase 33 pattern (`imports` + `functions` were already parallel arrays). Option (2) preserves source order, which the formatter (Phase 37) will need. Going with (1) for Phase 36 to keep the diff focused on declaration parsing; Phase 37 will revisit by either attaching `line: Int` / `column: Int` to each declaration entity (cheap) or migrating to a discriminated sequence (more invasive). The choice can be made then with concrete formatter requirements in hand.

### One `FunctionDecl` entity for free functions, methods, and constructors

Three options for the AST shape:

1. **Single `FunctionDecl` with flags** (chosen): adds `is_constructor: Bool` to distinguish; `return_type == ""` for constructors. Methods are flagged externally (they live in `EntityDecl.methods`).
2. **Separate `FunctionDecl` / `MethodDecl` / `ConstructorDecl` entities**: structurally identical but typed differently.
3. **Single `FunctionDecl` without flags**: distinguish by which array it appears in (top-level vs. entity.methods vs. entity.ctor).

(1) keeps the AST narrow (fewer entities for `parser.intent` and a future formatter to handle) and makes the constructor/method distinction explicit without polluting Program-level arrays. The trade-off — every `FunctionDecl` instance carries an `is_constructor` flag even when it doesn't apply — is consistent with the Token / Stmt / Expr "carry unused fields" pattern we've used throughout stage2.

### `_marker` suffix on every Phase 36 keyword kind function

Following Phase 33's precedent: when this lexer file is itself parsed by Intent, bare names like `kw_entity` could collide with Intent's reserved-word handling. The `_marker` suffix is a known workaround that costs one extra word per keyword and avoids the issue entirely. Applied to all 12 new keywords for consistency, even though some (e.g. `for`) probably wouldn't bite given they're not used as identifiers in our codebase.

### Field renames around reserved words

Several intuitive field names collided with Intent's reserved words:

- `goal` → `goal_text` on `IntentBlock` (`goal` is reserved inside `intent { ... }` blocks).
- `verified_by` → `verifications` on `IntentBlock` (`verified_by` is the clause keyword inside `intent` blocks).
- Local variable `result` → `exprs` inside `parse_invariant_block` (`result` is the postcondition placeholder identifier).

These are documented in the entity comment. A future polish ADR could narrow Intent's reserved-word set so field/local names can be more natural, but Phase 36 just absorbs the renames.

### `public` dispatch is inlined, not factored into a helper

The natural design — `parse_public_decl(prog: Program)` mutates `prog.functions` / `prog.entities` / etc. — fails at the Rust backend layer. The stage1 Rust emitter passes entity-typed parameters by value (clone), so mutations on `prog` inside the helper don't propagate back to the caller. This is a real backend gap that's worth a future ADR (auto-`&mut` propagation for entity params, similar to Phase 32's `&mut self` work for methods).

Workaround: inline the `public` dispatch directly in `parse_program` so the mutations happen on the outer scope's `prog`. The code is slightly more verbose but the behavior is correct.

### Cross-module function calls require qualification

Confirmed during this phase: Intent imports make entity *constructors* available unqualified (`Token(...)`, `Block(...)`) but free *functions* still need their module prefix (`formatter_ast.empty_block()`, `formatter_lexer.tk_eof()`). This appears to be a stage1 emitter choice rather than a deep language constraint — the type checker resolves bare names via imports, but the Rust emitter renames free functions with their module prefix.

Phase 36 absorbs the ~100 qualifications mechanically. A future polish phase could either (a) auto-import-through bare free-function names, or (b) document this as a deliberate namespacing rule. Either is fine; the current behavior is workable.

### Dogfood scope: synthetic fixture, not full self-parse

NEXT-STEPS proposed "parse parser.intent's own source as a dogfood test." That's blocked by carry-over Phase 32 gaps: stage2 lexer doesn't tokenize char literals (`'{'`), float literals, or interpolated string parts. `parser.intent` uses `'{'.to_string()` extensively in its test fixtures, so a literal self-parse would die on the first char literal.

Phase 36 substitutes a synthetic dogfood test that exercises the full surface (entity + constructor + method + test, with statements + expressions inside). Real self-parse is gated on stage2 lexer extensions and stays a Phase 37+ goal.

## Tasks (as shipped)

### 36.1 Split AST into `ast.intent`

**Files:** `selfhost/formatter/ast.intent` (new), `selfhost/formatter/parser.intent`

Moved every AST entity declaration, every kind constant, and the helpers `empty_block` / `empty_expr` / `parse_int` (plus a new `empty_function_decl` + `empty_program` for Phase 36) into `ast.intent`. `parser.intent` keeps the `Parser` entity, parse methods, the `parse(source)` top-level convenience, and tests. Dead `join_with_space` helper deleted (was used by the Phase 33 raw-text capture, removed in Phase 35).

### 36.2 New lexer keywords

**Files:** `selfhost/formatter/lexer.intent`

Added 12 new keyword constants + 12 new `keyword_kind()` entries. Existing 13 lexer tests still pass unchanged.

### 36.3 New AST entities

**Files:** `selfhost/formatter/ast.intent`

`FieldDecl`, `EntityDecl`, `EnumVariant`, `EnumDecl`, `TraitMethodSig`, `TraitDecl`, `ImplDecl`, `IntentBlock`, `TestDecl`, `ExternDecl`. `FunctionDecl` extended with `is_constructor`. `Program` expanded to 10 arrays + `error`. New `empty_function_decl()` and `empty_program(module_decl)` helpers.

### 36.4 New parser methods

**Files:** `selfhost/formatter/parser.intent`

13 new parse methods (see Success Criteria). Each is small (15-40 lines) and follows the same shape as the Phase 34/35 statement parsers: expect leading keyword, parse fields with `expect` / `check_consume` helpers, return the structured AST entity.

### 36.5 Updated `parse_program`

`parse_program` now dispatches on the first token of each top-level declaration. The dispatch is a nested if-chain (Intent's match arms are single-expression; an if-chain is more readable for side-effecting dispatch). `public` is consumed and the next-keyword case is dispatched inline (the helper-method approach broke at the Rust backend layer — see design decisions).

### 36.6 Test rewrite + new tests

Test "parser reports unexpected token at top level" updated: it previously checked that `entity Foo { ... }` was rejected (Phase 33 behavior). Phase 36 accepts entities, so the test now checks a still-unsupported construct (bare integer literal at top level). 14 new Phase 36 tests cover each new declaration kind + a "mixed declarations in any order" + a synthetic dogfood. Combined: 74/74 on rust + js.

### 36.7 Bulk-qualify cross-module function calls

After the split, every call to `empty_block()`, `empty_expr()`, `parse_int()`, `st_*()`, `ex_*()`, etc. needs the `formatter_ast.` prefix. Done via a perl one-liner targeting just the call-expression form (`\b<fn>\(`) to avoid mangling identifiers inside strings or comments. ~100 substitutions.

## Out of Scope (deferred)

- **Contract parsing** (`requires` / `ensures` / `decreases`) — Phase 36 still uses the Phase 33 crude ident-skip heuristic between `returns Type` and `{` for both top-level functions and entity methods. A real contract parser is Phase 38+, after match expressions and lambdas are in.
- **Generic type parameters** in declarations (`entity Stack<T> { ... }`) — captured as part of the existing "type-arg list collapsed to `<...>`" handling. A real `TypeRef` AST is Phase 37+ work.
- **`async` modifier on methods** — stage1 grammar allows it, but stage2 method parsing doesn't recognise it yet. Easy follow-up.
- **`match` expressions, lambdas, interpolated strings, char/float literals** — carried-over Phase 32-35 gaps.
- **Full self-parse dogfood** — gated on stage2 lexer extensions (char + float literals + multi-line comments).
- **Source-order tracking across declaration kinds** — Phase 37 will revisit when the formatter's needs are concrete.

## Validation

- `make validate` green (Go unit tests + examples).
- `intentc test selfhost/formatter/parser.intent` 74/74 on rust.
- `intentc test --target js selfhost/formatter/parser.intent` 74/74 on js.

## Actual size

- `ast.intent`: 395 LOC (entities + kind constants + helpers).
- `parser.intent`: 1707 LOC (~700 LOC parser + ~1000 LOC tests).
- `lexer.intent`: 520 LOC (13 new keyword constants + 12 new `keyword_kind` entries).
- Net diff: +~1100 LOC added, with significant reuse of the Phase 34-35 parse helpers.

## Surfaced gaps (workarounds applied; no fix this phase)

| Gap | Workaround | Suggested follow-up |
|---|---|---|
| Cross-module free-function calls need module prefix (`formatter_ast.empty_block()`); entities don't. | Bulk-qualify all call sites. | ADR + emitter change: either let bare free-function names resolve through imports, or document this as a deliberate namespacing rule. |
| Entity-typed method parameters are passed by value in the Rust backend, so mutations don't propagate. | Inline the `public` dispatch in `parse_program` rather than factor a helper. | ADR + emitter change to auto-`&mut` entity parameters when the method body mutates them (similar to Phase 32's `&mut self` work). |
| `goal` / `verified_by` / `result` reserved-word collisions on entity fields and local variables. | Rename to `goal_text` / `verifications` / `exprs`. | Future polish ADR could narrow Intent's reserved-word set for field/local positions. |
| Real self-parse blocked by stage2 lexer char-literal / float-literal / multi-line-comment gaps. | Synthetic fixture dogfood test exercising entity + constructor + method + test. | Stage2 lexer extension phase, then full self-parse as a milestone. |
