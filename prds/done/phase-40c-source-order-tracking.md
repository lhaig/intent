# Phase 40C: Source-Order Tracking for Top-Level Declarations

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece C)
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Design decision:** [ADR 0042](../../docs/decisions/0042-stage2-source-order-tracking.md)
**Prerequisite:** [Phase 39](phase-39-self-parse-certification.md) (self-parse certification)

## Goal

The stage2 formatter shipped in Phase 38 emits top-level decls in a fixed canonical order (module → imports → functions → entities → ...). For `hello.intent` the user's source order coincided with that canonical order, so byte-equal worked. For any program that interleaves decl kinds — entity, then function, then entity, then test — the formatter reorders them and diverges from `intentc fmt`'s output. This is the first of three sub-pieces (C → B → A) Phase 40 needs to land before byte-equal self-format on the stage2 files themselves becomes the dogfood gate.

Sub-piece C adds a `line: Int` field to every top-level decl entity, populated by the parser from the leading keyword token, and rewrites `format_program` as a k-way merge of the per-kind arrays sorted by `line`.

## Success Criteria

- [x] Nine top-level decl entities (`ImportDecl`, `FunctionDecl`, `EntityDecl`, `EnumDecl`, `TraitDecl`, `ImplDecl`, `IntentBlock`, `TestDecl`, `ExternDecl`) gain a `line: Int` field. `ModuleDecl` does not (always first by grammar).
- [x] Parser populates `line` from the leading keyword token in each `parse_*_decl` method. For functions, `line` is taken from the very first token (which may be `public` / `async` / `entry` or `function`); for the others, it's the kind keyword (entity / enum / trait / impl / test / extern / intent / import).
- [x] `empty_function_decl()` helper defaults `line: Int = 0`.
- [x] `format_program` walks a k-way merge of the nine per-kind arrays, picking the smallest current `line` at each step. Module decl is always first; other decls are emitted in source-line order regardless of kind.
- [x] Existing tests still pass — 93 stage2 parser tests + 17 prior formatter tests.
- [x] **Two new tests** lock the new behaviour:
  - Source-order roundtrip: parses a synthetic source with `function, test, function, test` interleaving and asserts the formatted output preserves that order (instead of `function, function, test, test`).
  - Line-field population: parses a small program and asserts the parser populated each `FunctionDecl.line` from the correct source line.
- [x] Combined **117/117 stage2 tests on rust + js**. `make validate` green.

## Reference

- [ADR 0042](../../docs/decisions/0042-stage2-source-order-tracking.md) — the design decision (per-decl `line` vs. discriminated union)
- [Phase 38 PRD](phase-38-stage2-formatter-mvp.md) — formatter MVP this builds on
- [Phase 39 PRD](phase-39-self-parse-certification.md) — self-parse certification
- `selfhost/formatter/ast.intent` — 9 entities gained `line: Int`
- `selfhost/formatter/parser.intent` — 9 parse methods populate `line`
- `selfhost/formatter/format.intent` — `format_program` rewritten as k-way merge
- `selfhost/formatter/format_test.intent` — 2 new tests

## Design decisions (and why)

### Per-decl `line: Int` vs discriminated union

Documented in **ADR 0042**. Briefly: adding one field to nine entities is mechanically simple and localised; moving `Program` to `Array<TopLevelDecl>` would touch every consumer of the per-kind arrays (parser tests, formatter, future tools) for a structural improvement that's purely aesthetic at this point. The parallel-array shape stays; future refactor remains available if the surface stabilises.

### Capture line inside each parse method, not in `parse_program`

The dispatcher in `parse_program` consumes `public` before dispatching to `parse_entity_decl(true)` etc. — so if we captured line in `parse_program`, we'd get the line of `public`. If we capture inside `parse_entity_decl` (from the `entity` keyword token), we get the line of `entity`. For typical one-line `public entity Foo { ... }` source these coincide; for the rare multi-line `public\nentity Foo { ... }` they differ by one. We chose the inside-each-method approach because (a) it keeps the line-capture logic local to the method that needs it, (b) it matches `parse_function_decl`'s pattern (which has to capture line *before* modifier consumption since modifiers are consumed inside, not in `parse_program`), and (c) the multi-line modifier case is vanishingly rare in real code.

### `format_program` uses an inline 9-way merge, not a generic priority queue

Writing a generic priority queue in stage2 Intent would require either a heap entity or sortable trait — non-trivial surface for a one-shot loop. With nine fixed kinds, the inline merge is ~150 LOC of straight-line `if` chains. The alternative (sort a single `Array<DeclRef>` of `(line, kind, idx)` tuples) would need an `Array.sort` builtin or an in-Intent sort implementation — Intent doesn't have either yet. The inline approach is the smallest path to working code; readability is fine because the structure is fully symmetric across the nine kinds.

### `while not done { ... }` instead of `break`

Stage2's parser doesn't handle `break` (see `parser.intent` line 17 comment listing missing features). `format_program` is a stage2 file and must remain parseable by stage2 itself (per Phase 39's self-parse certification). The cost of using a `done: Bool` flag instead of `break` is one extra local variable; the benefit is that `format.intent` continues to be self-parseable, preserving the Phase 39 milestone.

### `if/else` nested chain instead of else-if cascade

The dispatch on `best_kind` (`if best_kind == 0 { ... } else { if best_kind == 1 { ... } else { ... } }`) uses nested `else { if ... }` instead of `else if`. This is because the stage2 parser's `if`-statement handling treats `else if` as `else { if ... }` internally, and writing the source form explicitly avoids any ambiguity around how the parser folds the chain. Same convention as other stage2 files (see `parse_program` in `parser.intent`).

### `line: Int` over `line: Option<Int>`

Empty placeholders (`empty_function_decl`) default `line: Int = 0`. Using `0` as a sentinel for "no line" is acceptable because real Intent source can never produce a decl on line 0 (the parser numbers lines starting at 1). An `Option<Int>` field would be slightly more correct semantically but adds match-arm overhead at every read site. Sentinel-0 is the precedent-matching choice in stage2 (`Token.kind` uses `tk_eof()` similarly).

## Surfaced gaps

None new this phase. The `line` field is reused by future tooling (linter, LSP — `[[gap-cross-module-fn-qualify]]`) without further change.

## Out of scope (still pending for byte-equal self-format)

- **Sub-piece B: paren stripping** — `intentc fmt` strips `(x)` to `x` but preserves `(a + b) * c`. Stage2's `format_expr` currently emits `ex_paren` verbatim.
- **Sub-piece A: comment preservation** — the lexer drops comments; needed for byte-equal on any commented file. Largest of the three sub-pieces.
- **`requires` / `ensures` parser surface** — Phase 41.

## Files touched

- `docs/decisions/0042-stage2-source-order-tracking.md` — new ADR.
- `selfhost/formatter/ast.intent` — 9 entity constructors widen by 1 param (`line: Int`); doc comment on `FunctionDecl` explains the field.
- `selfhost/formatter/parser.intent` — 9 `parse_*_decl` methods capture `line` from leading keyword token.
- `selfhost/formatter/format.intent` — `format_program` rewritten as k-way merge by `line`.
- `selfhost/formatter/format_test.intent` — 2 new tests (interleaved-order roundtrip + line-field assertion); 2 existing tests updated for new constructor signatures.
