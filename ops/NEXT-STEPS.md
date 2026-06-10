# Pickup Notes — 2026-06-09 (after Phase 40A.1)

Handoff after leading-decl comment preservation shipped.

## Where we are

Recent landings:
- **Phase 30-38** — see prior NEXT-STEPS.
- **Phase 39** — self-parse certification.
- **Phase 40C** — source-order tracking. ADR 0042.
- **Phase 40B** — precedence-aware paren stripping. ADR 0043.
- **Phase 40A.1** — leading-decl comment preservation. ADR 0044.

Stage2 (`selfhost/formatter/`):
- `lexer.intent` (~650 LOC, 27 tests) — comment-capturing tokeniser.
- `ast.intent` (~440 LOC) — every top-level decl has `line: Int` + `comments_before: Array<String>`.
- `parser.intent` (~1770 LOC, 66 tests) — `parse_program` captures pre-modifier; each `parse_*_decl` accepts `leading_comments`.
- `format.intent` (~660 LOC) — k-way merge by `line`, precedence-aware paren stripping, leading-comment emission.
- `format_test.intent` (~340 LOC, 38 tests).

**131 in-language tests pass on rust + js.** `make validate` green.

## Byte-equal self-format gate progress

| Sub-piece | Status |
|---|---|
| C — source-order tracking | ✓ Phase 40C |
| B — paren stripping | ✓ Phase 40B |
| A.1 — leading-decl comments | ✓ Phase 40A.1 |
| A.2 — inline + body comments | next |

After 40A.2: byte-equal self-format on `selfhost/formatter/lexer.intent` / `ast.intent` / `parser.intent` / `format.intent` becomes the dogfood gate test.

## Immediate next step

**Phase 40A.2 — finish comment preservation.** Three remaining sub-problems, each with a small surface:

1. **Trailing-EOF comments.** The lexer already captures them onto the synthetic EOF token's `comments_before`. The formatter doesn't emit them yet. Add a "trailing block" to `format_program`: after the k-way merge, if `prog` carried trailing comments (could go on a new `Program.trailing_comments: Array<String>` field populated by parser from the EOF token), emit them.
2. **Comments inside function/method bodies, between statements.** Each `Stmt` needs `comments_before: Array<String>`. Lexer already attaches to tokens; parser needs to thread comments into stmt nodes the same way it does for top-level decls.
3. **Inline-after comments** (`let x = 1; // ...`). Trickier — Token needs a `comment_after: Option<String>` (or similar); parser captures after consuming the terminating `;`. Format hooks emit at end-of-stmt.

Recommended sequencing inside 40A.2:
- (1) first — smallest diff, immediate win on stage2 files that end with a trailing comment.
- (2) second — moderate diff, statement-comment is the biggest piece of remaining divergence on stage2 source.
- (3) third — most involved, includes the design of "where does an inline comment attach."

Once 40A.2 lands, add a byte-equal dogfood test:

```intent
test "byte-equal self-format on stage2 lexer.intent" {
    let r = read_file("selfhost/formatter/lexer.intent");
    let src = match r { Ok(s) => s, Err(_) => "" };
    if src != "" {
        let prog = formatter_parser.parse(src);
        assert_eq(prog.error, "");
        let out = formatter_format.format_program(prog);
        assert_eq(out, src);
    }
}
```

Anything that still diverges goes into Phase 41 (parser surface widening) — `requires` / `ensures` / `match` / `for-in` / `try ?`.

## Phase 41 outlook

Once 40A.2 lands and any structural divergence from comments is closed, Phase 41 widens the parser surface. The most impactful additions (by order they unblock real stage1 examples):

- `requires` / `ensures` clauses on function signatures + `result` keyword. Unblocks `examples/fibonacci.intent`.
- `match` expressions over `Result` / `Option`. Unblocks `examples/io_demo.intent`, `selfhost/formatter/format_test.intent`.
- `for in` loops. Cleaner stage2 iteration code.
- `try ?` operator. Result-propagation in stage2.

These are independent — can be picked up in any order.

## Other candidates (orthogonal to stage2 work)

Unchanged:
- Verify-aware stripping (ADR 0033 deferred).
- String surface follow-up ADR.
- Phase 17.G / 17.H — WASM test runner / coverage.
- Phase 23 — VS Code Marketplace publish.
- Backend ADRs surfaced (cross-module fn qualification, auto-`&mut` for entity params, multi-use String auto-clone).

## Language gaps still open

- `{` / `}` in stage1 string literals trigger interpolation parsing.
- `let _:` rejected → expression statements.
- `version` is a reserved word.
- Cross-module entity-type qualification rejected.
- Constructor double-use of a `String` parameter triggers borrow error.
- No `String.to_int()` / `parse_float` builtins.
- Cross-module free-function calls need module prefix.
- Entity-typed method/function parameters passed by value in the Rust backend.
- Stage2 lexer doesn't tokenise interpolated string parts; **comments now captured (Phase 40A.1) — leading-decl only**.
- Local `String` re-use across expressions can trigger `borrow of moved value`.
- Stage2 parser doesn't handle `requires` / `ensures` / `match` / `for-in` / `try ?` / `break`. Phase 41+.

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
3. Recommended start: **Phase 40A.2 sub-step (1)** — trailing-EOF comments. Smallest diff. Add `Program.trailing_comments: Array<String>`; populate in `parse_program` after the loop from the EOF token's `comments_before`; emit at the end of `format_program`. Then (2) — statement-level comments — and (3) — inline-after.
