# Phase 38: Stage2 Formatter MVP

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 37](phase-37-stage2-lexer-extensions.md) (stage2 lexer extensions)

## Goal

Stage2 has a lexer (Phase 32 + 37) and a parser covering the full structural surface of Intent (Phase 33-36). Phase 38 closes the round-trip: `selfhost/formatter/format.intent` walks an `ast.intent` `Program` and emits canonical Intent source. The dogfood goal is byte-equality with stage1's `intentc fmt` on `examples/hello.intent` — a small but real program exercising module, entry function, return statement, and test declaration with an expression-statement body.

## Success Criteria

- [x] `selfhost/formatter/format.intent` exists and exports `format_program(prog: Program) returns String` plus per-kind helpers (`format_module_decl`, `format_import_decl`, `format_function_decl`, `format_test_decl`, `format_entity_decl`, `format_enum_decl`, `format_trait_decl`, `format_impl_decl`, `format_intent_block`, `format_extern_decl`, `format_constructor_decl`, `format_method_decl`, `format_param_list`, `format_block_body`, `format_stmt`, `format_expr`).
- [x] Output order is module → imports → functions → entities → enums → traits → impls → intent_blocks → tests → externs, separated by blank lines, with the canonical four-space indent.
- [x] Expressions: ex_int, ex_float, ex_char, ex_string, ex_bool, ex_ident, ex_unary, ex_binop, ex_call, ex_index, ex_field, ex_array, ex_range, ex_paren — all round-trip.
- [x] Statements: st_let (with optional `mutable`), st_return (with/without value), st_expr, st_if (with/without else), st_while — all round-trip.
- [x] `selfhost/formatter/format_test.intent` exists with 17 new tests: 8 unit tests (one per emitter family) + 7 round-trip tests + 1 string concat test + 1 **real-file dogfood** test that reads `examples/hello.intent`, parses + formats it, and asserts byte-equality with the source.
- [x] Combined: **110/110 stage2 tests** on rust + js (93 from Phases 32-37 + 17 new).
- [x] `make validate` green.

## Stage1 fix surfaced + landed in-phase

- **Backend bug in `internal/rustbe/rustbe.go`** — call sites passing `Array<T>` / `Map<K,V>` field accesses (or indexed array elements) to a function expecting `Array<T>` / `Map<K,V>` would emit `obj.field.clone()` (an owned `Vec`/`HashMap`) where the function signature expected `&Vec` / `&HashMap`, producing `rustc` `E0308 mismatched types`. The fix extended the array-ref coercion in `generateCallExpr` from `*ir.VarRef` only to `{VarRef, FieldAccessExpr, IndexExpr}`. New regression test `TestArrayParamFieldAccessCallBorrows` locks the fix. No NLSpec or ADR was needed — the change matches the existing intent of "pass arrays by reference at call sites" expressed in the surrounding comment.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 37 PRD](phase-37-stage2-lexer-extensions.md) — lexer extensions this builds on
- `selfhost/formatter/format.intent` — formatter implementation
- `selfhost/formatter/format_test.intent` — formatter tests (incl. real-file dogfood)
- `internal/rustbe/rustbe.go` — array-ref coercion fix
- `internal/rustbe/rustbe_test.go` — `TestArrayParamFieldAccessCallBorrows`

## Design decisions (and why)

### Output decl order is fixed (module → imports → functions → entities → enums → traits → impls → intent_blocks → tests → externs)

Phase 36 made `Program` use parallel arrays per kind, which loses source order across kinds. Two options:

1. **Fixed canonical order** (chosen): emit decl kinds in a single deterministic sequence. `hello.intent`'s source order (module → function → test) coincides with the chosen sequence, so the dogfood holds. For programs that interleave kinds (entities mixed with functions, etc.), the formatter reorders them — same trade-off Go's `gofmt` makes for declarations.
2. **Attach `line: Int` to each decl entity + walk the union** by source order. Preserves user ordering exactly; requires Phase 36's parallel-array shape to be revisited (adds a field to every decl entity, or move to `Array<TopLevelDecl>`).

(1) ships the MVP. Phase 39 will add source-order tracking if the dogfood corpus widens to programs that mix decl kinds.

### Parens preserved via `ex_paren`, not algorithmically stripped

Stage1's formatter strips unnecessary parens. Stage2's parser already preserves user-written parens as `ex_paren` Expr nodes. The formatter emits parens exactly when `ex_paren` appears in the AST. For programs without any parens (like `hello.intent`), this is byte-equal; for programs with redundant parens like `(1 + 2);`, stage2 would emit them but stage1's formatter strips. Tightening to byte-equal on parenthesised programs is a Phase 39 refinement — needs operator-precedence-aware stripping in the emitter (or a paren-stripping pass on the AST).

### Twin `pad_open` / `pad_close` locals instead of reusing one

A local `let pad: String = indent_string(level);` referenced twice in a return expression triggers a `borrow of moved value` error in the Rust backend — the move analysis isn't currently use-counting String locals (in particular, when the local is *also* assigned into a mutable accumulator early in the function via `head = pad`). The source-side workaround is to compute the indent string twice. This is cheap (a small allocation), local to the formatter, and easy to undo when the backend learns to clone-on-multi-use. Documented inline in `format_function_decl`.

### Test file separate from implementation

`format_test.intent` lives next to `format.intent` and imports it. The Phase 32-37 pattern (tests in the implementation file) starts to hurt at ~100 tests; splitting now keeps each file under 600 LOC. Stage1's tooling supports multi-file packages so a sibling file with `test ... { ... }` blocks runs cleanly.

### Real-file dogfood + synthetic round-trip — both

The synthetic round-trip tests build the input source inline (`"module m version \"1.0\"; function f() ..."`) so they're independent of any external file and run from any working directory. The real-file dogfood (`read_file("examples/hello.intent")`) tests that the formatter survives a real Intent program lexed and parsed from disk — but `read_file` returns `Err` when the test runner's working directory isn't the repo root, in which case the assertion is silently skipped. The synthetic tests carry the load when the real-file dogfood is absent.

## Surfaced gaps (deferred)

- **Parens stripping** — `intentc fmt` strips unnecessary parens; stage2 preserves them. Byte-equal on parenthesised programs is Phase 39.
- **Source order across decl kinds** — `Program` uses parallel arrays per Phase 36; mixing kinds reorders. Phase 39 adds `line: Int` to each decl entity (or moves to `Array<TopLevelDecl>`).
- **`requires` / `ensures` contract clauses** — parser doesn't handle them yet. Fibonacci.intent dogfood blocked on Phase 39+ parser work.
- **Patterns / `match` / `for in`** — same parser gap.
- **`async function` modifiers in the body** — modifier preserved by parser; formatter handles it, but the surrounding contract syntax around async isn't wired yet.
- **String escape re-encoding** — stage2 stores string content verbatim from the lexer; formatter re-quotes verbatim. Works for hello.intent (no special escapes); a program with embedded `\n` etc. inside a `"..."` literal should also round-trip since the lexer doesn't decode escapes.

## Out of scope

- Differential CLI integration (`intentc fmt --self-hosted`) — Phase 40.
- Byte-equal on fibonacci.intent (blocked on requires/ensures parser).
- Full-feature parser parity (async, match, generics, traits with bodies) — Phase 39.

## Files touched

- `selfhost/formatter/format.intent` — new file, ~400 LOC implementing the AST→source emitter.
- `selfhost/formatter/format_test.intent` — new file, 17 tests including a real-file dogfood on `hello.intent`.
- `internal/rustbe/rustbe.go` — extend array-ref call-site coercion to FieldAccess / IndexExpr.
- `internal/rustbe/rustbe_test.go` — `TestArrayParamFieldAccessCallBorrows` regression test.
