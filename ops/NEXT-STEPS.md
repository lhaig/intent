# Pickup Notes — 2026-06-03 (after Phase 36)

Handoff after the fifth stage2 deliverable shipped.

## Where we are

Today's session shipped (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** — strategic frame + first language-gap design.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent.
- **Phase 34** — Statement parser in Intent.
- **Phase 35** — Expression parser in Intent.
- **Phase 36** — Top-level declarations + AST split (this session).

Stage2 (`selfhost/formatter/`) is now three files:
- `lexer.intent` (520 LOC) — tokenizer.
- `ast.intent` (395 LOC) — every AST entity + kind constant + AST-side helper.
- `parser.intent` (1707 LOC, ~700 LOC parser + ~1000 LOC tests) — `Parser` entity + parse methods + the `parse(source)` convenience + tests.

The parser handles the full structural surface of Intent programs: module / imports / functions / entities (with fields, constructors, methods, invariant blocks) / enums (unit + data-carrying variants) / traits / impls / tests / externs / intent blocks. Inside function bodies, statements (`let`, `return`, `if`/`else`, `while`, expression stmts) and expressions (literals, identifiers, calls, method chains, field access, indexing, slicing, ranges, array literals, full operator precedence) all parse to a real AST.

**74 in-language tests pass on rust + js.** `make validate` green.

## Language gaps surfaced in Phase 36

Two real backend gaps surfaced (workarounds applied; fixes deferred):

| Gap | Workaround in Phase 36 | Future fix |
|---|---|---|
| Cross-module free-function calls need module prefix (`formatter_ast.empty_block()`); entities don't. | Bulk-qualified ~100 call sites. | Either let bare free-function names resolve through imports (emitter change), or document as a deliberate namespacing rule. ADR needed if we want the former. |
| Entity-typed method/function parameters are passed by value (cloned) in the Rust backend. Mutations on the parameter don't propagate back. | Inlined the `public` declaration dispatch in `parse_program` instead of factoring a `parse_public_decl(prog)` helper that mutated `prog`. | Auto-`&mut` entity parameters when the body mutates them, analogous to Phase 32's `&mut self` work for methods. Needs an ADR + emitter change. |

Plus three reserved-word collisions on intuitive field/local names: `goal` → `goal_text`, `verified_by` → `verifications`, `result` → `exprs`. A future polish ADR could narrow Intent's reserved-word set in field/local positions.

Carried-over gaps still open (unchanged this phase):
- `{` / `}` in string literals trigger interpolation parsing → `'{'.to_string()` concat.
- `let _:` rejected → expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity-type qualification (`module.Entity`) is rejected — entities must be referenced bare from imported modules.
- Constructor double-use of a `String` parameter triggers borrow error in the Rust backend.
- No `String.to_int()` builtin → in-Intent `parse_int` via Phase 31 Char primitives.
- Stage2 lexer doesn't tokenize char literals (`'a'`), float literals, multi-line `/* */` comments, or interpolated string parts.

## Immediate next step

**Phase 37 — Stage2 lexer extensions, then begin the formatter.**

Phase 37 is a two-part deliverable:

**Part A: stage2 lexer extensions** (prerequisite for full self-parse dogfood and for the formatter to handle real Intent source). Add to `lexer.intent`:
- **Char literals**: `'a'`, `'\n'`, `'\t'`, `'\\'`, `'\''`, `'\u{1234}'`. Emit as new token kind `tk_char` carrying the raw literal text. Phase 31 already added `Char` at the type level; the stage2 lexer just needs to recognise the syntax.
- **Float literals**: `3.14`, `1.5e10`, `2.5e-3`. Emit as `tk_float`. The Phase 35 expression parser will need a corresponding `ex_float` Expr kind + a `parse_float` helper (modelled on `parse_int`).
- **Multi-line `/* ... */` comments**: extend `skip_whitespace_and_comments` to handle these. Nested comments per the Intent grammar.
- **String interpolation tokenisation** (stretch): currently the whole `"abc ${expr} def"` lexes as one `tk_string`. The formatter will eventually need the interp segments split out — could be a `tk_string_start` / `tk_string_part` / `tk_interp_start` / `tk_interp_end` sequence. May defer to a follow-up if Part B can land without it.

**Part B: begin the formatter** (`selfhost/formatter/format.intent`). Build an `AST → String` emitter. Initial scope:
- Module + import lines.
- Function declarations with parameter lists, return types, and (initially) raw-text body re-emission via re-tokenisation. A proper statement/expression formatter follows incrementally.
- Entity / enum / trait / impl / test / extern declarations.
- Per-line indentation by AST depth (entity body indented 4 spaces, statement body the same, etc.).

The dogfood goal: `intentc fmt` (current Go impl) and `selfhost/formatter` (stage2 Intent impl) produce byte-equal output on a small corpus. Initial corpus: `examples/hello.intent`, `examples/fibonacci.intent`, `selfhost/formatter/lexer.intent`. Full parity is a Phase 38+ milestone.

Recommended sequencing for Phase 37:
1. Part A first (char + float + multi-line comments). Stage2 tests around the new token kinds.
2. Wire `tk_char` / `tk_float` into the expression parser as new primary kinds.
3. Self-parse dogfood: `parse(read_file("selfhost/formatter/lexer.intent"))` succeeds — first real-world test.
4. Begin Part B.

Likely new gaps Phase 37 will surface:
- File I/O in stage2 (`read_file`). Probably needs a stage1 extern.
- A real `TypeRef` AST if the formatter needs to round-trip `Array<Int>`, `Result<T, E>`, etc. instead of the current `<...>`-suffix string.
- Source-order tracking on `Program` (or per-decl `line: Int`) if the formatter needs to interleave declarations of different kinds.

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
3. Recommended start: re-read `selfhost/formatter/ast.intent` to refresh on the AST shape (now split out, all entities + kind constants in one file), then begin Phase 37 with Part A (lexer extensions). Suggested order: char literals first (smallest surface, unblocks parser test for `'{'.to_string()` patterns), then float literals (the Phase 35 expression parser already has `parse_int`; modelling `parse_float` on top is straightforward), then multi-line comments. Self-parse dogfood after Part A lands.
