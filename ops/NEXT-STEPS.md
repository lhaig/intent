# Pickup Notes — 2026-06-09 (after Phase 38)

Handoff after the seventh stage2 deliverable shipped.

## Where we are

Recent landings (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** — strategic frame + first language-gap design.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent.
- **Phase 34** — Statement parser in Intent.
- **Phase 35** — Expression parser in Intent.
- **Phase 36** — Top-level declarations + AST split.
- **Phase 37** — Stage2 lexer extensions (char + float + block comments).
- **Phase 38** — **Stage2 formatter MVP**. `format.intent` round-trips `examples/hello.intent` byte-equal with stage1's `intentc fmt`. Stage1 Rust-backend bug fixed in-phase (array-ref coercion now covers field accesses + index exprs, not just bare variable references).

Stage2 (`selfhost/formatter/`) is now four files:
- `lexer.intent` (~610 LOC, 27 tests) — full tokeniser.
- `ast.intent` (~400 LOC) — AST entities + kind constants.
- `parser.intent` (~1740 LOC, 66 tests) — Parser + tests.
- `format.intent` (~400 LOC) — formatter.
- `format_test.intent` (~150 LOC, 17 tests) — formatter unit + round-trip + real-file dogfood.

**110 in-language tests pass on rust + js.** `make validate` green.

Stage1 fix this phase:
- `internal/rustbe/rustbe.go` — call sites passing `Array<T>` field accesses (and indexed array elements) now correctly emit `&obj.field` instead of `obj.field.clone()` (which mismatched the `&Vec<T>` parameter signature). Regression test `TestArrayParamFieldAccessCallBorrows` in `internal/rustbe/rustbe_test.go`.

## Language gaps still open

Carried over (unchanged this phase):
- `{` / `}` in stage1 string literals trigger interpolation parsing → use `'{'.to_string()` concat. Affects test-source construction in stage2 files.
- `let _:` rejected → expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity-type qualification (`module.Entity`) is rejected — entities must be referenced bare from imported modules.
- Constructor double-use of a `String` parameter triggers borrow error in the Rust backend.
- No `String.to_int()` builtin → in-Intent `parse_int` via Phase 31 Char primitives. No `parse_float` either — stage2 stores float lexemes as raw `String`.
- Cross-module free-function calls need module prefix (`formatter_ast.empty_block()`); entities don't. Backend ADR pending.
- Entity-typed method/function parameters are passed by value in the Rust backend. Backend ADR pending.
- Stage2 lexer doesn't yet tokenise interpolated string parts (`"abc ${expr} def"` is one `tk_string`).
- **Local `String` re-use across expressions can trigger `borrow of moved value`** — particularly when a `let x: String = pad;` is followed by another use of `pad`. Worked around in Phase 38 by computing the indent string twice (`pad_open` + `pad_close`). Backend should learn to clone-on-multi-use.

## Immediate next step

**Phase 39 — Widen the dogfood corpus.** Two sub-goals, take either order:

**Track A: parser surface for contracts** — extend `parser.intent` + `format.intent` to handle `requires` / `ensures` clauses on function signatures (used by `examples/fibonacci.intent` and many others). Lands `fibonacci.intent` as the second real-file dogfood fixture. Likely follow-ups: `decreases`, loop `invariant` clauses, and the `result` binding inside `ensures`.

**Track B: paren-stripping + source-order tracking** — match stage1's behaviour on programs that have user-written parens (`(a + b) * c` keeps the parens, `(x)` strips them) and programs that interleave decl kinds (entity followed by function followed by entity). Either: (a) precedence-aware paren strip pass on the AST + per-decl `line: Int`, or (b) move `Program` to `Array<TopLevelDecl>` discriminated union. Decision point this phase.

Other possible work:

- **`Char` literal escape-decoding** — currently stage2 emits the raw lexeme. A program that passes char literals through the formatter is byte-equal, but a program that constructs a `Char` value at runtime and asks the formatter to round-trip an in-memory Char would need a decode. Probably not needed for self-hosting.
- **`String` interpolation tokenisation** — `"abc ${expr} def"` is one `tk_string`. The formatter currently re-emits the literal verbatim, which works as long as nothing inside the interpolation needs re-formatting. Phase 39+ if the corpus exercises this.
- **Stage1 multi-use String backend fix** — would let us remove the `pad_open` / `pad_close` workaround. Small, contained, mostly cleanup.

## Other candidates (orthogonal to stage2 work)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()`, `s.index_of`, `s.replace`, Unicode-aware predicates.
- **Phase 17.G — WASM test runner**, **17.H — coverage**.
- **Phase 23 — VS Code Marketplace publish**.
- **ADR 004x — Package registry signing**.
- **Backend ADR — Cross-module function call qualification** (surfaced in Phase 36).
- **Backend ADR — Auto-`&mut` for entity parameters** (surfaced in Phase 36).

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
3. Recommended start: **Phase 39 Track A** — extend `parser.intent` to parse `requires` / `ensures` clauses (and the `result` keyword inside `ensures`), wire into `FunctionDecl` (or a sibling `Contract` entity), then teach `format.intent` to emit them. The fibonacci.intent dogfood unlocks immediately after.
