# Pickup Notes — 2026-06-15 (Phase 40 complete: byte-equal self-format)

The stage2 formatter now **byte-equal self-formats all four of its own source files**.
`format(parse(src)) == src` for `selfhost/formatter/{lexer,ast,parser,format}.intent`.

## Where we are

- **Phase 40 complete.** Byte-equal self-format gate met:
  - 40C source-order tracking, 40B paren-stripping (prior).
  - 40A.1 leading-decl comments; 40A.2 trailing-EOF / body / inline-after-statement
    comments; 40A.3 module-leading / entity field+method / end-of-block / inline-after-
    field / inline-after-decl-brace comments; generic type-arg round-trip
    (`parse_type_name`); and canonicalization of the 4 source files.
- The 4 stage2 files were **reformatted to canonical form** in 40A.3.6 (one-liner bodies
  expanded, inline comments normalized to a single space, intra-body blank lines removed,
  blank line between header and module removed). They still compile + self-parse.
- **158 in-language tests** pass on rust + js. `self_format_one` asserts byte-equality
  (from repo root; skips under the `intentc test` temp cwd — the synthetic comprehensive
  gate is always-on).
- Tooling: norman (`prds/`); aiki removed.

## Immediate next step: Phase 41 — parser surface widening

The stage2 parser still can't handle several constructs (it skips or rejects them), which
keeps stage2 source written in a restricted subset. Widen it, in rough impact order:

1. `requires` / `ensures` clauses + `result` keyword. Currently `skip_method_contracts` /
   the top-level equivalent DISCARD contract clauses — so the formatter would drop them.
   Needed before any contract-bearing Intent can round-trip. Unblocks `examples/fibonacci.intent`.
2. `match` expressions over `Result` / `Option`. Unblocks `examples/io_demo.intent` and lets
   stage2 stop hand-rolling Int-tagged dispatch.
3. `for in` loops.
4. `try ?` operator.

Each needs: lexer keyword (if missing) → parser production → AST node → formatter emit →
tests. Then re-run the byte-equal probe pattern to confirm no regressions.

Note: contract clauses (1) are the highest priority — they're a known silent-drop in the
current formatter (only safe today because the stage2 files contain none).

## Other candidates (orthogonal)

- Verify-aware stripping (ADR 0033 deferred).
- Phase 23 — VS Code Marketplace publish (blocked on credentials; `prds/backlog/`).
- ADR for the Phase 40A.3 comment-preservation design (the comment-position taxonomy +
  canonicalization-for-fixpoint decision) — currently only in ADR 0044's orbit; worth its own.

## Language / backend gaps still open

- Mutating an already-pushed Array element (`arr[i].field = x`) is unreliable in the stage1
  Rust backend — hold a local, mutate, push.
- Local `String` re-use across expressions / loop iterations can trigger `borrow of moved
  value`; call helpers (e.g. `indent_string`) fresh each iteration.
- `let _:` rejected → bare expression statements to discard a Result.
- Stage2 parser doesn't handle `requires`/`ensures`/`match`/`for-in`/`try ?`/`break` (Phase 41).
- `intentc test` runs from a temp cwd: `read_file` needs absolute paths or it skips;
  `print` inside a test is suppressed.

## How to resume

1. `git log --oneline -15`.
2. `prds/TASKS.md` for the open task list (norman).
3. Recommended start: **Phase 41 step 1** — `requires`/`ensures` clauses + `result`. They're
   the highest-value parser gap and a current silent-drop risk in the formatter.
