# Pickup Notes — 2026-06-09 (after Phase 40B)

Handoff after paren stripping shipped.

## Where we are

Recent landings:
- **Phase 30-38** — see prior NEXT-STEPS.
- **Phase 39** — self-parse certification.
- **Phase 40C** — source-order tracking. ADR 0042.
- **Phase 40B** — precedence-aware paren stripping. ADR 0043.

Stage2 (`selfhost/formatter/`):
- `lexer.intent` (~610 LOC, 27 tests) — full tokeniser.
- `ast.intent` (~420 LOC) — AST entities with `line: Int` on all 9 top-level kinds.
- `parser.intent` (~1750 LOC, 66 tests) — Parser populating `line`.
- `format.intent` (~630 LOC) — formatter with k-way merge by line + precedence-aware paren stripping.
- `format_test.intent` (~280 LOC, 32 tests) — unit + round-trip + self-parse + self-format + source-order + paren-strip tests.

**125 in-language tests pass on rust + js.** `make validate` green.

## Byte-equal self-format gate progress

| Sub-piece | Status |
|---|---|
| C — source-order tracking | ✓ Phase 40C |
| B — paren stripping | ✓ Phase 40B |
| A — comment preservation | next |

After 40A lands, the dogfood gate becomes: parse each stage2 file → format → byte-compare → green.

## Immediate next step

**Phase 40A — comment preservation.** Largest of the three sub-pieces. The lexer currently skips comments in `skip_whitespace_and_comments` — they never reach the parser or AST. To round-trip a commented file byte-equal we need to plumb comments through.

### Design decision pending (ADR 0044)

Three plumbing options:

1. **Token-attached comments.** Each non-comment token carries an `Array<String>` of comments that immediately preceded it (with their leading whitespace). The parser threads these through; on `tok.advance()`, captured comments attach to whatever decl/stmt is being built. Formatter emits them in front of the corresponding decl/stmt.
2. **Separate comment-token stream.** Lexer emits `tk_comment` tokens; parser ignores them but stores a sidecar `Array<Token>` of all comments in source order, indexed by source line. Formatter walks both streams in sync.
3. **AST-attached comments.** Each AST node (decl, stmt) gets a `leading_comments: Array<String>` field. Lexer emits comments as tokens; parser captures them into the field on the node being built.

Trade-offs:

- **(1)** is the cleanest plumbing — comments flow with tokens, no separate stream. But forces every parser method to be comment-aware, and "comments before the next decl/stmt" is the only granularity we can express. Inline-after comments (`let x = 1; // explanation`) need extra handling.
- **(2)** is the lowest-touch on the parser (it just skips comment tokens). But the formatter has to do a coordinated walk of two streams which is fiddly.
- **(3)** is the most semantically precise — comments live where they belong. But the AST surface widens significantly.

Recommendation: **option (1)** as the v1, with leading-comments only. Inline-after comments can be a Phase 40A.2 extension or roll into Phase 41+. Option (3) is the long-term right answer but the diff is large.

### Test plan

- Synthetic: round-trip a small file with a comment before a function decl and a comment between two decls.
- Real-file: `examples/hello.intent` doesn't have comments — still byte-equal.
- Stretch: after 40A, attempt byte-equal on `selfhost/formatter/lexer.intent` (heavy comments). Whatever still diverges goes into Phase 41 corpus.

### Recommended sequencing

1. Write ADR 0044 selecting one of the three options.
2. Lexer change — emit comments somehow (either as tokens or as side-channel data).
3. Parser change — accept the comments without breaking existing tests.
4. AST change — add the comment field (if applicable).
5. Formatter change — emit comments at the right positions.
6. Roundtrip tests for the leading-comment case.

## Other candidates (orthogonal to stage2 work)

Unchanged from prior NEXT-STEPS:
- Verify-aware stripping (ADR 0033 deferred).
- String surface follow-up ADR.
- Phase 17.G / 17.H — WASM test runner / coverage.
- Phase 23 — VS Code Marketplace publish.
- Backend ADRs surfaced in Phases 36 / 38 (cross-module fn qualification, auto-`&mut` for entity params, multi-use String auto-clone).

## Language gaps still open

Carried over — unchanged this phase:
- `{` / `}` in stage1 string literals trigger interpolation parsing.
- `let _:` rejected → expression statements.
- `version` is a reserved word.
- Cross-module entity-type qualification rejected.
- Constructor double-use of a `String` parameter triggers borrow error.
- No `String.to_int()` / `parse_float` builtins.
- Cross-module free-function calls need module prefix.
- Entity-typed method/function parameters passed by value in the Rust backend.
- Stage2 lexer doesn't tokenise interpolated string parts or preserve comments.
- Local `String` re-use across expressions can trigger `borrow of moved value`.
- Stage2 parser doesn't handle `requires` / `ensures` / `match` / `for-in` / generics in fn signatures / `try ?` / `break`. Needed for richer corpus (fibonacci.intent etc.).

## Memory state

Durable items (unchanged this phase):
- `project_intent_is_a_new_language`.
- `feedback_write_adrs_along_the_way`.
- `feedback_minimise_mistakes_in_autonomous_runs`.
- `project_self_hosting_priority`.
- `feedback_document_and_push_after_each_phase`.

## How to resume

1. `git log --oneline -10`.
2. `aiki task` for the open task list.
3. Recommended start: **Phase 40A — write ADR 0044** selecting the comment-plumbing approach (token-attached / separate stream / AST-attached). Then start with the lexer: change `skip_whitespace_and_comments` to either accumulate comments or emit `tk_comment` tokens. Smallest first step that exposes the comments to downstream consumers.
