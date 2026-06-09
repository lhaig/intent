# Pickup Notes — 2026-06-09 (after Phase 39)

Handoff after the **self-parse certification milestone**.

## Where we are

Recent landings (in order):
- **Phase 30** — package registry.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding**.
- **Phase 31** — `Char` primitive + string indexing.
- **Phase 32** — Lexer in Intent.
- **Phase 33** — Parser top-level in Intent.
- **Phase 34** — Statement parser in Intent.
- **Phase 35** — Expression parser in Intent.
- **Phase 36** — Top-level declarations + AST split.
- **Phase 37** — Stage2 lexer extensions (char + float + block comments).
- **Phase 38** — Stage2 formatter MVP. hello.intent round-trips byte-equal.
- **Phase 39** — **Self-parse certification.** All four stage2 files (lexer.intent, ast.intent, parser.intent, format.intent) parse cleanly with the stage2 parser. Parse + format roundtrip runs end-to-end on each. **115/115 stage2 tests** on rust + js. Stage1 fix: builtin I/O calls (`read_file`/`write_file`/`create_dir`/`file_exists`/`env_get`) now `cloneIfNeeded` their string arg.

Stage2 (`selfhost/formatter/`) is now four files + one test file:
- `lexer.intent` (~610 LOC, 27 tests) — full tokeniser.
- `ast.intent` (~400 LOC) — AST entities + kind constants.
- `parser.intent` (~1740 LOC, 66 tests) — Parser + tests.
- `format.intent` (~450 LOC) — formatter.
- `format_test.intent` (~190 LOC, 22 tests) — formatter unit + round-trip + self-parse + self-format dogfood.

**115 in-language tests pass on rust + js.** `make validate` green.

## Self-hosting status

| Capability | Status |
|---|---|
| Stage2 lexer + parser cover the structural surface | ✓ Phase 32-37 |
| Stage2 formatter exists and works on hello.intent byte-equal | ✓ Phase 38 |
| Stage2 parses all four of its own source files | ✓ Phase 39 |
| Stage2 parse + format pipeline runs end-to-end on its own source | ✓ Phase 39 |
| **Stage2 formatter output byte-equal to stage1 on stage2 source** | ✗ blocked on comment preservation, paren stripping, source-order tracking |
| Stage2 covers `requires` / `ensures` / `match` / generics | ✗ surface not needed by stage2 itself; needed for richer dogfood corpus |
| CLI integration `intentc fmt --self-hosted` | ✗ Phase 42 |

The structural grammar is closed under self-application — **stage2 parses itself**. The remaining work is fidelity (byte-equal) and breadth (corpus).

## Language gaps still open

Carried over (unchanged this phase):
- `{` / `}` in stage1 string literals trigger interpolation parsing → use `'{'.to_string()` concat.
- `let _:` rejected → expression statements.
- `version` is a reserved word → use `module_version` etc.
- Cross-module entity-type qualification (`module.Entity`) rejected.
- Constructor double-use of a `String` parameter triggers borrow error in the Rust backend.
- No `String.to_int()` / `parse_float` builtins.
- Cross-module free-function calls need module prefix (`formatter_ast.empty_block()`).
- Entity-typed method/function parameters are passed by value in the Rust backend.
- Stage2 lexer doesn't tokenise interpolated string parts.
- Local `String` re-use across expressions can trigger `borrow of moved value`. Worked around with twin locals.
- Stage2 parser doesn't handle `requires` / `ensures` / `match` / `for-in` / generics in function signatures / `try ?` operator. None of these are used by stage2 source itself.
- Stage2 lexer drops comments — needed for byte-equal self-format.

## Immediate next step

**Phase 40 — Byte-equal self-format.** Three independent sub-pieces, all needed before the dogfood corpus can include stage2 files:

**Sub-piece A: comment preservation.** The lexer currently skips comments in `skip_whitespace_and_comments`. Three options for plumbing them through:
1. **Token-attached** (probably): each non-comment token carries the `Array<String>` of comments that preceded it. Formatter emits them on the appropriate line.
2. **Separate comment-token stream**: emit `tk_comment` tokens; parser ignores them but threads them into the AST via a sidecar map keyed by source position.
3. **AST-attached**: each AST node carries a `leading_comments: Array<String>` field. Cleanest semantically; widest diff.

ADR decision needed before implementation.

**Sub-piece B: paren stripping.** Stage1 fmt strips `(x)` to `x` but preserves `(a + b) * c`. Two options:
1. **Precedence-aware emitter**: format_expr tracks the parent's precedence and elides `ex_paren` when the child's precedence wouldn't change meaning.
2. **AST canonicalisation pass**: pre-walk strips redundant `ex_paren` before formatting.

(1) is the conventional pretty-printer approach; (2) is cleaner for the formatter but adds an explicit transform pass.

**Sub-piece C: source-order tracking.** Phase 36's parallel-array `Program` loses cross-kind decl order. Two options:
1. Add `line: Int` (or `source_idx: Int`) to each top-level decl entity. Emitter walks the union sorted by line.
2. Move `Program` to a single `Array<TopLevelDecl>` discriminated union.

(1) is less invasive; (2) is structurally cleaner. Either way the formatter's emission order matches the input.

Recommended sequencing for Phase 40:
1. Sub-piece C first (smallest diff, immediate win on richer fixtures).
2. Sub-piece B second (precedence-aware emit, mostly contained to format_expr).
3. Sub-piece A last (largest change; comment preservation touches lexer + parser + AST + formatter).

After Phase 40: try byte-equal on the four stage2 files. Whatever still diverges goes into Phase 41 corpus (requires/ensures + match parser surface).

## Other candidates (orthogonal to stage2 work)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()`, `s.index_of`, `s.replace`, Unicode-aware predicates.
- **Phase 17.G — WASM test runner**, **17.H — coverage**.
- **Phase 23 — VS Code Marketplace publish**.
- **ADR 004x — Package registry signing**.
- **Backend ADR — Cross-module function call qualification** (Phase 36).
- **Backend ADR — Auto-`&mut` for entity parameters** (Phase 36).
- **Backend ADR — String multi-use auto-clone** (Phase 38).

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
3. Recommended start: **Phase 40 Sub-piece C** (source-order tracking) — smallest diff for a clear win. Add `line: Int` field to each top-level decl entity (FunctionDecl, EntityDecl, EnumDecl, etc.) populated by the parser from the leading keyword token. Then rewrite `format_program` to walk a sorted-by-line union of the per-kind arrays. The test pattern: round-trip `fibonacci.intent`'s decl order (function then function then entry function then multiple tests) and assert the formatter preserves it.
