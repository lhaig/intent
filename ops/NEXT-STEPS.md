# Pickup Notes — 2026-06-09 (after Phase 40C)

Handoff after source-order tracking shipped.

## Where we are

Recent landings:
- **Phase 30-38** — see prior NEXT-STEPS.
- **Phase 39** — self-parse certification.
- **Phase 40C** — source-order tracking via per-decl `line: Int`. ADR 0042. Stage2 formatter now preserves source order across interleaved decl kinds.

Stage2 (`selfhost/formatter/`):
- `lexer.intent` (~610 LOC, 27 tests) — full tokeniser.
- `ast.intent` (~420 LOC) — AST entities with `line: Int` on all 9 top-level kinds.
- `parser.intent` (~1750 LOC, 66 tests) — Parser populating `line` from leading keyword tokens.
- `format.intent` (~530 LOC) — formatter with k-way merge by `line` in `format_program`.
- `format_test.intent` (~220 LOC, 24 tests) — unit + round-trip + self-parse + self-format + source-order tests.

**117 in-language tests pass on rust + js.** `make validate` green.

## Byte-equal self-format gate progress

Stage2 self-format byte-equal on its own source files needs three independent sub-pieces:

| Sub-piece | Status |
|---|---|
| C — source-order tracking | ✓ Phase 40C |
| B — paren stripping | next |
| A — comment preservation | blocked on B |

Once all three land, the dogfood gate becomes: parse each stage2 file → format → byte-compare → green.

## Immediate next step

**Phase 40B — paren stripping.** `intentc fmt` strips redundant parens (`(x)` → `x`) but preserves necessary ones (`(a + b) * c` stays). Stage2's `format_expr` currently emits every `ex_paren` verbatim.

Two implementation options to choose between:

1. **Precedence-aware emitter.** `format_expr` walks with a `min_precedence: Int` parameter representing the surrounding context's binding power. When emitting an `ex_binop`, the operands are formatted with `min_precedence` set to the binop's precedence. When emitting an `ex_paren(inner)`, look at `inner`: if `inner.precedence >= min_precedence` then strip the paren and emit `inner`'s content directly; otherwise keep the paren.
2. **AST canonicalisation pass.** A pre-walk over the program strips redundant `ex_paren` nodes before formatting. Requires knowing the precedence of each parent context — same logic, just precomputed.

Option (1) is the conventional pretty-printer approach and matches how stage1's Go formatter is structured. Option (2) is cleaner but introduces an AST mutation pass that other tooling (linter, etc.) would need to be aware of.

ADR 0043 should record the choice.

Test plan:
- Synthetic inputs covering: `(x)` → `x`, `(1 + 2) * 3` preserved, `1 + (2 * 3)` → `1 + 2 * 3`, chained `(a + b) + c` → `a + b + c`, mixed-precedence `(a == b) and (c == d)` → `a == b and c == d`.
- After 40B, retry self-format on stage2 files and confirm no new byte differences come from paren handling.

Sequence recommendation: **Phase 40B → 40A** (comment preservation, the largest of the three). After both ship, attempt byte-equal self-format dogfood on the four stage2 files; iterate on whatever still diverges.

## Other candidates (orthogonal to stage2 work)

Unchanged from prior NEXT-STEPS:
- Verify-aware stripping (ADR 0033 deferred).
- String surface follow-up ADR.
- Phase 17.G / 17.H — WASM test runner / coverage.
- Phase 23 — VS Code Marketplace publish.
- Backend ADRs surfaced in Phases 36 / 38 (cross-module fn qualification, auto-`&mut` for entity params, multi-use String auto-clone).

## Language gaps still open

Carried over from Phase 39 — unchanged this phase:
- `{` / `}` in stage1 string literals trigger interpolation parsing → use `'{'.to_string()` concat.
- `let _:` rejected → expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity-type qualification (`module.Entity`) rejected.
- Constructor double-use of a `String` parameter triggers borrow error in the Rust backend.
- No `String.to_int()` / `parse_float` builtins.
- Cross-module free-function calls need module prefix.
- Entity-typed method/function parameters passed by value in the Rust backend.
- Stage2 lexer doesn't tokenise interpolated string parts or preserve comments.
- Local `String` re-use across expressions can trigger `borrow of moved value`.
- Stage2 parser doesn't handle `requires` / `ensures` / `match` / `for-in` / generics in function signatures / `try ?` operator / `break`. None used by stage2 source itself; needed for richer corpus (fibonacci.intent etc.).

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
3. Recommended start: **Phase 40B paren stripping.** Write ADR 0043 documenting the choice between precedence-aware emitter (option 1, recommended) vs AST canonicalisation pass (option 2). Then implement `format_expr` with a `min_precedence: Int` parameter and an `expr_precedence(e: Expr) returns Int` helper. Begin with the binop precedence table from `parser.intent` (assign < or < and < eq < cmp < range < add < mul < unary < postfix < primary). Strip `ex_paren` when its child's precedence is at least the surrounding minimum.
