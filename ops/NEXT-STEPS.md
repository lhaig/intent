# Pickup Notes — 2026-06-09 (after Phase 37)

Handoff after the sixth stage2 deliverable shipped.

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
- **Phase 37** — Stage2 lexer extensions: char + float literals + multi-line nested `/* */` comments; wired `tk_char`/`tk_float` into the expression parser as `ex_char` / `ex_float` (raw-lexeme storage).

Stage2 (`selfhost/formatter/`) is now three files:
- `lexer.intent` (~610 LOC, 27 in-language tests) — full tokeniser for the structural surface of Intent (idents, keywords, ints, floats, chars, strings, all punctuation, both comment forms).
- `ast.intent` (~400 LOC) — every AST entity + kind constant (`ex_*`, `st_*`) + AST-side helper.
- `parser.intent` (~1740 LOC, ~700 LOC parser + ~1040 LOC tests, 66 in-language tests) — `Parser` entity + parse methods + the `parse(source)` convenience + tests.

**93 in-language tests pass on rust + js.** `make validate` green.

The lexer now handles everything the formatter (Phase 38) will need to *tokenise* real Intent source. The parser handles the full structural surface of Intent programs at the statement and expression level — see Phase 35-36 PRDs for the layered grammar.

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
- No file I/O in stage2 — stage1 extern needed for `read_file` (Phase 38 will likely surface this).

## Immediate next step

**Phase 38 — Begin the formatter (`selfhost/formatter/format.intent`).**

Build an `AST → String` emitter that round-trips Intent source. Initial scope (incremental from a v1 minimum):
- Module declaration + import lines.
- Function declarations: signature (modifiers, name, params, return type) emitted from AST; body re-emitted as raw text via re-tokenisation for v1. Per-statement formatting follows incrementally as the formatter is dogfooded against real files.
- Entity / enum / trait / impl / test / extern / intent-block declarations.
- Per-line indentation by AST depth (entity body = 4 spaces, statement body = same, etc.).

**Dogfood goal:** `intentc fmt` (current Go impl) and `selfhost/formatter` (stage2 Intent impl) produce **byte-equal** output on a small corpus. Initial corpus: `examples/hello.intent`, `examples/fibonacci.intent`, `selfhost/formatter/lexer.intent`. Full parity is a Phase 39+ milestone.

Likely gaps Phase 38 will surface:
- **File I/O in stage2** (`read_file`) — needs a stage1 extern. ADR + small implementation.
- **A real `TypeRef` AST** if the formatter needs to round-trip `Array<Int>`, `Result<T, E>`, etc. instead of the current `<...>`-suffix string. (Currently `FunctionDecl.return_type: String` carries the head + generic suffix.)
- **Source-order tracking on `Program`** (or per-decl `line: Int`) if the formatter needs to interleave declarations of different kinds in their original order. Phase 36 used parallel arrays per kind, losing cross-kind source order.
- **String interpolation tokenisation** — needed when the formatter wants to re-indent or otherwise touch interpolated strings; can be deferred if v1 emits them opaquely.
- **Decl-internal whitespace / blank-line preservation** — beyond v1 scope, but the choice between "canonicalise whitespace" vs. "preserve user style" needs an ADR before Phase 39.

Recommended sequencing for Phase 38:
1. Stage1 extern for `read_file` (+ tests). Smallest unblock.
2. Skeleton `format.intent`: `format_program(prog: Program) returns String` returning module + imports + decl headers in source order (using parallel arrays in roundtrip order or via a Phase 38 source-order list — design choice this phase makes).
3. Per-decl-kind formatters, body re-emission via re-tokenisation.
4. Differential test harness: stage2-formatted output vs. `intentc fmt` output, byte-compared, on the corpus.

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
3. Recommended start: **Phase 38 step 1** — add a stage1 extern for `read_file` (Go side, simple wrapper around `os.ReadFile`). Then write `selfhost/formatter/format.intent` with a `format_program` entry point that round-trips module + imports + decl headers on a small fixture, before scaling to the byte-equal corpus.
