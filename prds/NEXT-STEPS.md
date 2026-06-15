# Pickup Notes — 2026-06-15 (Phase 42 in progress: CLI wiring done, gap-closing ongoing)

## Where we are

**Phase 42 (Stage2 Formatter CLI Wiring + Differential Test)** — the CLI-wiring
half is complete and the differential harness is live; parser-gap closing is in
progress. See `prds/active/phase-42-formatter-cli-differential.md` and
`prds/TASKS.md`.

Done this run:
- **42.1 `args()` builtin** (ADR 0045) — `args() -> Array<String>`; rust
  `std::env::args().collect()`, js `process.argv.slice(1)`, wasm stub.
- **42.2 `main.intent`** — runnable stage2 formatter (`args()[1]` → parse →
  `format_program` → stdout; exit 0/1/2/3). Also fixed a latent stage1 bug:
  an `entry` fn in an imported module no longer emits a duplicate `main`
  (gated on `f.IsEntry && g.isEntryFile`; rustbe + jsbe).
- **42.3 differential harness** — `selfhost/formatter/difftest.sh` +
  `make diff-formatter`. Canonicalize-first: stage1-`fmt` a copy, then assert
  stage2 reproduces it byte-for-byte (= agrees with `intentc fmt`).
- **42.4 `intentc fmt --self-hosted`** — Go shim; env `INTENT_STAGE2_FMT`
  override or auto-build-with-cache; composes with `--check`; no stage1
  fallback on stage2 error.
- **42.5 invariants** (+ constructor contracts + intent blocks, folded in
  because the target files need them) — `invariant <expr>;` between fields and
  constructor; `intent "desc" { ...; verified_by: [...] }`.
- **42.12 char_string_demo** — resolved by the harness design (compares vs
  stage1 output, so it PASSES; no stage2 bug).

## Differential corpus status: 16/22 PASS, 0 divergences

Run `make diff-formatter` (or `bash selfhost/formatter/difftest.sh`). **Key
property: there are zero true divergences** — whenever the stage2 parser accepts
a file, the formatter output is byte-identical to `intentc fmt`. So all remaining
work is **parser coverage**. Remaining parse-errs (each its own TASKS.md row):

| Task | Construct | Example(s) |
|---|---|---|
| 42.6 | `forall` / `exists` quantifier expressions | sorted_check |
| 42.7 | `implies` operator | try_operator |
| 42.8 | generic type params on declarations `<T>` | generic_stack |
| 42.9 | `Fn(...) -> T` types + lambdas | closure_demo |
| 42.10 | `async` functions + `await` | async_demo |
| 42.11 | attributes `@name(args)` | target_specific_demo |

## How to resume

1. `git log --oneline -10`, then read this file + `prds/TASKS.md`.
2. `continue norman` — the next ready tasks are 42.6–42.11 (independent; any
   order). Pick one, mark it ACTIVE, follow the Phase 41 / 42.5 pattern.
3. Each gap = (lexer keyword if needed) → AST kind/field → parser production →
   formatter emit → round-trip test. Verify with `make diff-formatter` (the
   target example should flip to PASS) AND re-confirm byte-equal self-format on
   the 4 stage2 files.

## Hard-won patterns (full notes in progress.md)

- **Differential test = stage2-output vs stage1-OUTPUT**, not vs the raw file.
  difftest.sh canonicalizes each example with `intentc fmt` first.
- **Closing one gap unblocks the next construct in the same file** — expect to
  fold in adjacent constructs to make a target example fully PASS; the harness's
  per-file PASS is the real done-signal, not the single named construct.
- **Verify byte-equal self-format per-file.** A combined probe that round-trips
  multiple large stage2 files (format.intent + parser.intent) in ONE `intentc
  test` run gave a spurious abort; single-file byte-equal checks are reliable.
  Read files via ABSOLUTE paths (the in-language gate skips under the test cwd).
- **`print` adds one trailing newline** (println!/console.log); `main.intent`
  stdout = canonical + "\n"; the shim strips exactly one with `TrimSuffix`.
- **Avoid Rust keywords** (`fn`, `match`, `type`, ...) as Intent identifiers —
  the stage1 Rust backend doesn't escape them (rust-only build failure).
