# Pickup Notes — 2026-06-03 (late evening, after Phase 32)

Handoff after the first stage2 Intent code ships.

## Where we are

Today's session shipped:
- **Phase 30** — package registry (commit `9898b53`)
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** (commit `dfd3063`)
- **Phase 31** — `Char` + string indexing + char predicates (commit `54f05b4`)
- **Phase 32** — Lexer in Intent (this session — about to commit)

Stage2 of the self-hosting plan now has its first real artefact: `selfhost/formatter/lexer.intent` (~400 lines) tokenises a useful subset of Intent source. 13 in-language tests all pass on rust + js.

## Language gaps surfaced by Phase 32 (worth tracking)

| Gap | Workaround used | Suggested follow-up |
|---|---|---|
| Rust backend `&mut self` propagation missed `self.<user_method>()` calls. | **Fixed in Phase 32** by conservatively treating any `self.method()` as mutating + adding LetStmt / ReturnStmt to the statement walker. | None — landed alongside Phase 32. |
| Intent string literals interpret `{...}` as interpolation, so a literal `{` in a string needs `'{'.to_string()` concatenation. | Used the char-to-string concat in test fixtures. | Future polish ADR could require `${}` for interpolation (freeing `{`) or accept `\{` as literal. Not blocking. |
| `let _:` isn't accepted as a discard binding. | Used expression statements (`self.advance();`) when the result is unused. | Future polish: support `_` as a let-binding name; cheap to add but not blocking. |
| `s.to_int(): Result<Int, String>` doesn't exist — needed by parser to interpret integer-literal token text. | n/a yet. | Next-phase ADR (`String → Int parse`); part of Phase 33 prep. |
| String interpolation parsing in Intent source isn't tokenised — the lexer treats `"hello {name}"` as a single tk_string token without splitting parts. | n/a yet (lexer scope limit). | Defer until parser phases force the issue; could be its own follow-up. |
| `*/` multi-line comments aren't skipped. | n/a yet. | Add to scan_whitespace_and_comments in Phase 33. |
| `Char` literals aren't tokenised by the stage2 lexer. | n/a yet. | Add scan_char_literal in Phase 33. |
| Float literals aren't tokenised. | n/a yet. | Add to scan_int_literal as a `.` continuation in Phase 33. |

## Immediate next step

**Phase 33 — Parser top-level in Intent.** With tokens in hand, the next move is parsing the top-level declarations: `module ... version "X";`, `import "...";`, function signatures, function bodies (probably defer to a later sub-phase). The parser will quickly need richer AST node types — at minimum a Program → ModuleDecl + ImportDecl[] + FunctionDecl[] tree. Likely gaps it'll surface:

1. **Sum types richer than enum** for AST node variants. Today's enum variants can carry data but pattern-matching is one-expression-per-arm, which gets awkward when AST node bodies want sequence-of-statements logic. Might be solvable with helper functions, might warrant a dedicated ADR.
2. **String → Int parse** (`s.to_int()` or equivalent) for converting tk_int.text → Int.
3. **Dynamic dispatch** for the visitor pattern that the formatter (Phase 37) will want. Today's traits are static-dispatch only.

Recommended approach: start *very* small. Parse only `module ... version "X";` — produce a `ModuleDecl` entity from one token sequence. Layer functions / imports next. Don't try to parse expressions yet (Phase 35).

## Other candidates (orthogonal, not on self-hosting critical path)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()` (needed by Phase 33), `s.index_of`, `s.replace`, Unicode-aware predicates.
- **Phase 17.G — WASM test runner**, **Phase 17.H — coverage**.
- **Phase 23 — VS Code Marketplace publish** (blocked on user-supplied publisher account, PAT, icon).
- **ADR 004x — Package registry signing.**

## Memory state

Four durable items hold (unchanged):
- `project_intent_is_a_new_language` — every cross-cutting decision cites prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs ship with decisions, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code, validate after each task, surface uncertainty.
- `project_self_hosting_priority` — bootstrapping Intent with itself is the near-term goal.

## How to resume

1. `git log --oneline -10` for recent landings.
2. `aiki task` for the open task list.
3. Recommended start: open `selfhost/formatter/lexer.intent` to remind yourself of the token shape, then begin Phase 33. The parser is the next phase per ADR 0040 §"Delivery phases."
