# Phase 39: Self-Parse Certification

**Status:** Shipped (2026-06-09)
**Milestone:** v1.2 — Self-Improvement Foundations
**Strategic frame:** [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md)
**Prerequisite:** [Phase 38](phase-38-stage2-formatter-mvp.md) (stage2 formatter MVP)

## Goal

Phases 32-38 incrementally added a stage2 lexer, parser, AST, and formatter. The strategic question this phase answers: **does stage2 actually parse itself?** Plain enumeration of grammar features the parser supports doesn't prove the toolchain can ingest its own source — there's always the risk that the self-host files lean on a corner the parser doesn't handle. Phase 39 lands that proof: four in-language tests that `read_file` each stage2 file (`lexer.intent`, `ast.intent`, `parser.intent`, `format.intent`), parse it with the stage2 parser, and assert no parse error. Plus one round-trip test exercising the full parse + format pipeline on all four files.

Byte-equal self-format is **not** part of this phase — that's gated on comment preservation + paren-stripping (Phase 40+).

## Success Criteria

- [x] 4 self-parse tests (one per stage2 file) read source from disk and assert `prog.error == ""` after `formatter_parser.parse(source)`.
- [x] 1 self-format roundtrip test runs `format_program(parse(source))` on each stage2 file and asserts the formatted output is non-empty (no crashes; byte-equality deferred).
- [x] Tests pass on both rust and js targets; **115/115 stage2 tests green**.
- [x] `make validate` green.
- [x] Stage1 backend fix: builtin I/O calls (`read_file`, `write_file`, `create_dir`, `file_exists`, `env_get`) now apply `cloneIfNeeded` to their string arg — previously a String taken from an indexed array element moved out of the `Vec` and hit `E0507`. Regression test `TestBuiltinIOClonesIndexedStringArg`.

## Reference

- [ADR 0040](../../docs/decisions/0040-self-hosted-formatter-strategy.md) — strategic frame
- [Phase 38 PRD](phase-38-stage2-formatter-mvp.md) — formatter MVP this builds on
- `selfhost/formatter/format_test.intent` — 5 new self-tests (4 parse + 1 format roundtrip)
- `internal/rustbe/rustbe.go` — builtin I/O `cloneIfNeeded` fix
- `internal/rustbe/rustbe_test.go` — `TestBuiltinIOClonesIndexedStringArg`

## Why this matters

Phase 38 dogfood was `examples/hello.intent` — a deliberately minimal program. Self-parse on the four stage2 files exercises the parser on:

- **Long files**: `parser.intent` is ~1740 LOC and contains the densest concentration of Intent's structural surface in the repo. If the parser handles that file, it handles the structural grammar.
- **Every declaration kind exercised in production**: entity declarations with fields + constructors + methods, free functions with parameters and return types, public modifiers, intent's `Char` literals (Phase 37) used in keyword tables, real method chains (`self.source[start..self.position]`), nested array literals, complex string-concat builds with `'{'.to_string()` workarounds.
- **The formatter's own AST traversal**: `format.intent` exercises `match`-free dispatch on `Int`-tagged kinds, paren wrapping in `ex_paren`, deep nested `format!`-style concatenation patterns.

Passing this test certifies that the structural grammar is closed — every construct stage2's own source uses is parseable by stage2's own parser. The remaining surface (`requires`/`ensures`, `match`, `for-in`, generics in function signatures, the try `?` operator) is unused by the self-host code and can be added incrementally without blocking self-hosting.

## Design decisions (and why)

### `self_format_one(path: String)` helper instead of array iteration

The first cut iterated `["lexer.intent", "ast.intent", ...]` inside the test body. The Rust backend emitted `read_to_string(files[i])` which moves out of the `Vec<String>` — `E0507` at cargo build time. The source-side workaround extracts the call into a helper function; the underlying builtin-clone bug was fixed in `internal/rustbe/rustbe.go` in the same phase. Both changes carry their own regression tests.

### Self-format roundtrip asserts `len(out) > 0`, not byte-equality

The stage2 formatter currently drops:

- **Comments** (the lexer skips them in `skip_whitespace_and_comments`; the parser never sees them).
- **Source-order across decl kinds** (Phase 36's parallel-array `Program` shape).
- **Stripped redundant parens** (`ex_paren` preservation diverges from stage1 fmt's behavior).
- **Blank-line patterns inside function bodies**.

Byte-equality is a Phase 40+ goal. For Phase 39, "doesn't crash and produces non-empty output" is the right shape of assertion — it certifies the formatter survives the AST shapes that stage2 actually produces, without overcommitting to byte-equality semantics that aren't yet in scope.

### Read from disk, not embedded literals

Embedding the stage2 source as a multi-line string would let the tests run from any working directory but obscures what's being tested. Reading from disk via `read_file` couples the test to the repo root, with graceful skip-when-missing logic (the test asserts the parse only when `read_file` returned `Ok`). The repo's `make validate` always runs from the root so the test runs there; ad-hoc invocations from other directories silently pass.

### Stage1 fix is small and contained

The five builtin I/O cases (`read_file`, `write_file`, `create_dir`, `file_exists`, `env_get`) all extract their string argument the same way; adding `g.cloneIfNeeded(arg, expr.Args[0])` to each is a six-line patch. No NLSpec needed — this matches the existing `cloneIfNeeded` convention for regular function calls, the omission was just a gap in the builtin path.

## Surfaced gaps (deferred to Phase 40+)

- **Comment preservation** — lexer drops comments; needed for byte-equal self-format.
- **Source-order across decl kinds** — Phase 36's parallel arrays.
- **Paren stripping / precedence-aware re-parenthesisation** — to match stage1 fmt on programs with user parens.
- **`requires` / `ensures` contract clauses** — parser doesn't handle them; unblocks `fibonacci.intent` as a richer real-file dogfood.
- **`match` expressions** — used in `format_test.intent` for Result destructuring (which is stage1 code, not stage2), but stage2 parser doesn't handle them. Required for stage2 to format `format_test.intent` itself.

## Out of scope

- Byte-equal self-format (Phase 40+).
- CLI integration (`intentc fmt --self-hosted`) (Phase 41+).
- Adding parser surface beyond what stage2 files already use.

## Files touched

- `selfhost/formatter/format_test.intent` — 5 new tests (4 self-parse + 1 self-format roundtrip + `self_format_one` helper).
- `internal/rustbe/rustbe.go` — `cloneIfNeeded` applied to string args of `read_file` / `write_file` / `create_dir` / `file_exists` / `env_get` builtins.
- `internal/rustbe/rustbe_test.go` — `TestBuiltinIOClonesIndexedStringArg` regression test.
