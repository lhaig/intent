# Pickup Notes — 2026-06-15 (Phase 40 + Phase 41 complete)

## Where we are

- **Phase 40 (byte-equal self-format)** — the stage2 formatter byte-equal self-formats
  all four of its own source files: `format(parse(src)) == src` for
  `selfhost/formatter/{lexer,ast,parser,format}.intent`. Those files were canonicalized
  (reformatted to the formatter's fixpoint) in 40A.3.6.
- **Phase 41 (parser surface widening)** — the stage2 parser now handles the constructs it
  previously skipped/rejected, each round-tripping through parse + format:
  - 41.1 `requires`/`ensures`/`decreases` (were silently discarded).
  - 41.2 `match` (`ex_match` + `MatchArm`; multi-line via `format_expr_indented(e, level)`).
  - 41.3 `for ... in ...` (`st_for`).
  - 41.4 `try ?` (`ex_try`).
- **170 in-language tests** pass on rust + js for `selfhost/formatter/format_test.intent`.
  The byte-equal gate (`self_format_one`) asserts equality from repo root; it skips under
  the `intentc test` temp cwd, so re-verify byte-equality with a throwaway probe that
  reads the files via ABSOLUTE paths (see progress.md for the pattern).
- Tooling: norman (`prds/`); aiki removed.

## No task is currently queued

Phase 41 is the last completed phase; nothing is `ACTIVE`/`TODO` in `prds/TASKS.md`
(backlog #23 marketplace-publish is `BLOCKED` on credentials). Pick a direction below and
add it as a Phase 42 row in TASKS.md before/at the start of the next run.

## Candidate next directions (self-hosting path; not yet chosen)

1. **Wire the stage2 formatter into the CLI + differential-test against `intentc fmt`** on
   the real `examples/*.intent` corpus. It round-trips its own source and now handles
   contracts/match/for/try, so this is the natural next proof. Expect remaining parser gaps
   to surface: async functions, lambdas (`|x| => ...`), traits with default methods,
   string-interpolation parts, enums-with-data in match patterns, nested generics edge
   cases. Each gap is a small parse+format add (same pattern as Phase 41).
2. **Rewrite the linter in Intent** (HARNESS.md §7) — next-smallest tool after the formatter.
3. **Rewrite the compiler in Intent** — the big one; likely wants the package registry
   (shipped, Phase 30) for a split-package architecture.

Recommendation: (1) first — it's incremental, immediately validates the formatter against
real code, and surfaces the exact remaining gaps to prioritize.

## Useful patterns (from this work; full notes in progress.md)

- Adding a construct = (lexer keyword if needed) → AST kind/entity → parser production →
  formatter emit → round-trip test. Reuse `Stmt`/`Expr` fields where possible (e.g. `st_for`).
- Idempotence is NOT losslessness — verify reformatted files still compile + pass the suite
  before canonicalizing committed source.
- Avoid Rust keywords (`fn`, `match`, `loop`, `type`, ...) as Intent identifier/param names —
  the stage1 Rust backend doesn't escape them (JS is unaffected, so it shows as a rust-only
  build failure).
- Multi-line/indent-dependent expressions need `format_expr_indented(e, level)`, not the
  level-agnostic `format_expr`.

## How to resume

1. `git log --oneline -15`.
2. Read this file, then `prds/TASKS.md` (norman; aiki removed 2026-06-15).
3. `continue norman` will resume the workflow but find nothing queued — pick a direction
   above (recommend #1), add a Phase 42 task row, then proceed. Or just tell the agent
   "continue norman — start the formatter CLI wiring / differential test."
