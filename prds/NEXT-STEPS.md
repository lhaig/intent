# Pickup Notes — 2026-06-16 (Phase 42 COMPLETE: stage2 formatter at full parity)

## Where we are

**Phase 42 (Stage2 Formatter CLI Wiring + Differential Test) — COMPLETE.** The
Intent-implemented (stage2) formatter is now a runnable, CLI-wired tool that
matches stage1's Go `intentc fmt` **byte-for-byte on the entire examples corpus:
22/22 PASS, 0 divergences, 0 parse errors** (`make diff-formatter`). It also
remains a byte-equal fixpoint on all four of its own source files
(`make selfcheck-formatter`). Stage2 suite: 203/203 on rust + js.

Shipped this phase:
- **42.1 `args()` builtin** (ADR 0045) — `args() -> Array<String>`.
- **42.2 `main.intent`** — runnable stage2 formatter; also fixed a latent stage1
  bug (duplicate `main` when an imported module declares `entry`).
- **42.3 `make diff-formatter`** — differential harness vs `intentc fmt`
  (canonicalize-first; zero true divergences throughout).
- **42.4 `intentc fmt --self-hosted`** — Go shim delegating to the stage2 binary.
- **42.5–42.11 parser-gap closing** — invariants/constructor-contracts/intent-blocks,
  `implies`, `await`(+`spawn`,`async test`), `forall`/`exists`, generics `<T>` +
  generic instantiation, `Fn(...)->T` + lambdas, `@name(...)` attributes.
- **`make selfcheck-formatter`** — the authoritative built-binary self-format gate.

## Key learnings (full notes in progress.md)

- **Verify self-format on the LARGE stage2 files via the built binary, NOT an
  in-language `intentc test` probe.** `cargo test`/libtest runs each test on a
  ~2 MB thread stack, which overflows on the deep parse of the 95 KB
  `parser.intent` and aborts (non-deterministic). The real binary (8 MB main
  thread) is fine. Use `make selfcheck-formatter`.
- **Never run stage1 `intentc fmt` on a stage2 source file** — stage1/stage2 have
  diverging comment/paren behavior; it can break the stage2 fixpoint.
- **Adding a field to a stage2 AST entity:** default it in the constructor BODY
  (via a helper call like `empty_string_array()`), don't change the constructor
  signature — avoids call-site churn and the rust struct-literal scope trap.
- **Differential = stage2-output vs stage1-OUTPUT**, not vs the raw file
  (difftest.sh canonicalizes each example with `intentc fmt` first).

## Candidate next directions (self-hosting path; not yet chosen)

1. **Make `intentc fmt --self-hosted` the default / promote stage2 to canonical.**
   Parity holds on the corpus; consider widening the corpus first (more/larger
   real `.intent` files) to harden it before flipping the default.
2. **Rewrite the linter in Intent** (HARNESS.md §7) — next-smallest tool after the
   formatter; reuses the stage2 lexer/parser/AST.
3. **Rewrite the compiler in Intent** — the big one; likely wants a split-package
   architecture over the package registry (Phase 30).
4. **(Optional robustness) Big-stack generated binaries** — run generated rust
   (entry main + the `intentc test` runner) on a large-stack thread so deep
   recursion / large in-language tests never hit the libtest 2 MB limit. Not
   blocking today (the formatter binary is fine), but the self-hosted compiler
   will be deeper still. ADR-worthy if pursued.

## How to resume

1. `git log --oneline -15`, then read this file + `prds/TASKS.md`.
2. `continue norman` finds nothing queued — pick a direction above (recommend #2,
   the linter, to keep compounding the stage2 toolchain), scope it as Phase 43,
   add a TASKS.md row, then proceed.
3. Validate with `make build`, `make test`, `make diff-formatter`,
   `make selfcheck-formatter`.
