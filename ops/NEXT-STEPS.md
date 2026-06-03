# Pickup Notes — 2026-06-03 (late evening)

Handoff after Phase 31 ship. Resume from here.

## Where we are

Shipped today (in order):
- **Phase 30** (commit `9898b53`) — package registry, git + MVS.
- **ADRs 0040 + 0041 + Phase 31 PRD + scaffolding** (commit `dfd3063`) — self-hosting strategic frame + first language-gap design.
- **Phase 31** (this session) — `Char` primitive + string indexing/slicing + `len(s)` for String + ASCII char predicates + `char_from_codepoint`. Three backends + Z3 verifier integration. About to commit.

The first language extension for stage2 self-hosting is in place. A hand-rolled lexer in Intent is now writable.

## Immediate next step

**Phase 32 — lexer in Intent (`selfhost/formatter/lexer.intent`).** With Phase 31's primitives, the lexer can iterate codepoint-by-codepoint, classify with `is_alpha`/`is_digit`/`is_whitespace`, and extract token text via slicing. Recommended approach:

1. Define a `Token` entity in `selfhost/formatter/ast.intent` (or a sibling file) — kind + literal + line + column. Probably best modelled as an enum since Intent already supports data-carrying variants.
2. Write `lex(source: String) returns Array<Token>` in `selfhost/formatter/lexer.intent`. Start with the smallest useful subset: idents, ints, strings, keywords, punctuation. Defer interpolation, multi-line strings, comments-aware lex modes.
3. Add an in-language test block exercising the lexer against a fixture string of every token kind.

Likely gaps Phase 32 will surface (each worth its own ADR if they actually block work):
- **`s.to_int() -> Result<Int, String>`** for parsing integer literals — straightforward extension.
- **Sum-type ergonomics richer than enum** if Token-kind enum gets unwieldy.
- **Stdin streaming** for the eventual `selfhost/formatter/main.intent` binary.

## Strategic context

Per `[[project-self-hosting-priority]]`, self-hosting is the priority. Phase 29 + 30 + 31 are the foundation:
- 29 retired the parallel Rust testgen path so the toolchain has one test-emission code path.
- 30 added the package registry so stage2 tools can declare and depend on packages.
- 31 added the language extensions needed to lex Intent source from Intent.

Phases 32–39 (per ADR 0040) build the stage2 formatter incrementally. Phase numbers are indicative — each phase may surface its own language gap, in which case the gap gets its own ADR before the implementation phase resumes.

## Other candidates (orthogonal, not on the self-hosting critical path)

- **Verify-aware stripping** (`--strip-contracts=verified`) — ADR 0033 deferred.
- **String surface follow-up ADR** — `s.to_int()` parse, `s.index_of(needle)`, `s.replace(...)`, Unicode-aware predicates. Each as needed by Phase 32+.
- **Phase 17.G — WASM test runner.**
- **Phase 17.H — Coverage / snapshot testing.**
- **Phase 23 — VS Code Marketplace publish.** Blocked on user-supplied publisher account, PAT, icon.
- **ADR 004x — Package registry signing.** ADR 0039 §9 deferred.

## Memory state

Four durable items hold:
- `project_intent_is_a_new_language` — every cross-cutting decision should cite prior-art precedent.
- `feedback_write_adrs_along_the_way` — ADRs are written as decisions land, with precedent tables.
- `feedback_minimise_mistakes_in_autonomous_runs` — re-read code before editing, run `make validate` after each task, surface uncertainty.
- `project_self_hosting_priority` — bootstrapping Intent with itself is the near-term goal.

## How to resume

1. `git log --oneline -10` for recent landings (Phases 30/31, ADRs 0039-0041).
2. `aiki task` for the open task list.
3. Recommended start: open `selfhost/formatter/README.md`, then sketch the Token entity layout and start Phase 32. The lexer is the smallest piece and surfaces the first round of gaps.
