# PRD — Phase 42: Stage2 Formatter CLI Wiring + Differential Test

## 1. Introduction / Overview

The stage2 Intent formatter (`selfhost/formatter/`) byte-equal self-formats its
own four source files (Phase 40) and round-trips contracts/match/for/try
(Phase 41). It is, however, still only reachable from in-language tests — there
is no runnable tool and no repeatable check that it agrees with stage1's
`intentc fmt` on real code.

This phase makes the stage2 formatter a **runnable CLI tool**, wires it into
`intentc` as `intentc fmt --self-hosted`, and stands up a **committed
differential-test harness** that compares stage2 output against `intentc fmt`
across the `examples/*.intent` corpus. The harness drives the gap-closing work:
each example the stage2 parser can't yet handle becomes a small parse+format
task (same pattern as Phase 41).

### Key enabling insight

21 of 22 `examples/*.intent` files are already stage1-canonical (verified:
`intentc fmt --check` passes on all but `char_string_demo.intent`). Because the
examples *are* `intentc fmt`'s output, "stage2 reproduces the file byte-for-byte"
is **exactly equivalent** to "stage2 agrees with `intentc fmt`". The differential
test therefore reuses the proven `self_format_one` byte-equal pattern
(`format_program(parse(src)) == src`), applied to the examples corpus.

### Baseline differential result (measured 2026-06-15, throwaway probe)

PASS (12): array_sum, divergence_demo, enum_basic, error_handling, fibonacci,
handler_trait, hello, io_demo, map_demo, result_option, shape_area,
verify_example.

Gaps (parse errors) grouped by construct:
- Entity `invariant { … }` blocks — bank_account, js_demo, task_queue
- `forall` / quantifier expressions in contracts — sorted_check
- `implies` operator — try_operator
- Generic params on declarations (`entity Stack<T>`) — generic_stack
- `Fn(…) -> T` types / lambdas — closure_demo
- `async` / `await` — async_demo
- Attributes (`@target_specific("rust")`) — target_specific_demo
- Non-canonical source (diverges; stage1 fmt also rewrites it) — char_string_demo

## 2. Goals

- Add a stage1 `args()` builtin returning `Array<String>` (command-line args),
  recorded by an ADR — the CLI primitive self-hosting needs.
- Ship `selfhost/formatter/main.intent`: an `entry function main()` that reads a
  file path from `args()`, formats it, and prints the result to stdout.
- Wire `intentc fmt --self-hosted <file>` to delegate to the stage2 formatter.
- Commit a differential-test harness (`selfhost/formatter/difftest.sh` +
  `make diff-formatter`) reporting per-file PASS / DIVERGE / PARSE-ERR with a
  summary and non-zero exit on any failure.
- Close the surfaced parser gaps so progressively more of the corpus passes,
  each gap round-tripping through parse + format and preserving byte-equal
  self-format on the stage2 files.

## 3. User Stories

### US-001: `args()` builtin
**Description:** As an Intent program author, I want `args()` so a program can read
its command-line arguments.

**Acceptance Criteria:**
- [ ] `args()` type-checks to `Array<String>`; calling it with any argument is a
  checker error: `args() takes no arguments, got N`.
- [ ] Rust backend emits `std::env::args().collect::<Vec<String>>()` (so
  `args()[0]` is the program name, `args()[1]` the first user argument).
- [ ] JS backend emits `process.argv.slice(1)` (so `args()[0]` is the script
  path, `args()[1]` the first user argument — same indexing as Rust).
- [ ] WASM backend emits an empty `Vec<String>` (args unavailable in wasm; out of
  scope for runtime semantics) and still compiles.
- [ ] A Go test in `internal/checker` asserts `args()` checks to `Array<String>`
  and that `args(1)` produces the arity error.
- [ ] An `.intent` example/test builds on rust + js and prints `len(args())`.

### US-002: Runnable `main.intent`
**Description:** As a user, I want to run the stage2 formatter on a file and see
the formatted output on stdout.

**Acceptance Criteria:**
- [ ] `selfhost/formatter/main.intent` has `entry function main() returns Int`
  that reads `args()[1]`, `read_file`s it, `parse`s, calls
  `format_program`, and prints the result. On parse error it prints the
  `prog.error` to stdout (or a diagnostic) and returns non-zero.
- [ ] Building it (`intentc build --target rust selfhost/formatter/main.intent`)
  produces a binary that, run on `examples/hello.intent`, prints output
  byte-equal to the file's contents (modulo a single trailing newline if
  `print` adds one — the harness/shim accounts for this explicitly).
- [ ] Builds on both rust and js targets.

### US-003: `intentc fmt --self-hosted`
**Description:** As a user, I want `intentc fmt --self-hosted <file>` to format
using the stage2 (Intent) formatter instead of the stage1 (Go) formatter.

**Acceptance Criteria:**
- [ ] `intentc fmt --self-hosted examples/hello.intent` writes the same bytes
  that native `intentc fmt examples/hello.intent` would (verified on the 12
  passing examples).
- [ ] `intentc fmt --self-hosted --check <file>` exits 0 when formatted, non-zero
  otherwise, matching native `--check` semantics on the passing examples.
- [ ] On a file the stage2 parser cannot handle, `--self-hosted` exits non-zero
  with a clear message naming the file and the stage2 parser error (does NOT
  silently fall back to stage1).
- [ ] The mechanism for locating/building the stage2 binary is documented in the
  formatter README; a Go test exercises `--self-hosted` on hello.intent.

### US-004: Differential-test harness
**Description:** As a maintainer, I want one command that reports where the stage2
formatter agrees with `intentc fmt` across the examples corpus.

**Acceptance Criteria:**
- [ ] `selfhost/formatter/difftest.sh` runs from repo root, formats every
  `examples/*.intent` through stage2 (via the absolute-path in-language probe),
  and prints a per-file line: `PASS` / `DIVERGE` / `PARSE-ERR <msg>`, plus a
  summary `N passed, M diverged, K parse-err`.
- [ ] The script exits 0 only when every example is PASS (or an explicitly
  allow-listed known-divergence such as the non-canonical char_string_demo).
- [ ] `make diff-formatter` invokes the script.
- [ ] The script cleans up any temp files it creates and works whether or not
  cargo/node are installed (it uses `intentc test --target rust`).

### US-005..US-012: Parser-gap closing (one story per construct)
**Description:** As the stage2 formatter, I want to parse and re-emit
`<construct>` so the corresponding example round-trips byte-equal.

Each story's acceptance criteria:
- [ ] The named example parses with no error through the stage2 parser.
- [ ] `format_program(parse(src)) == src` for that example (it is
  stage1-canonical), asserted by an absolute-path probe.
- [ ] At least 2 in-language round-trip tests in `parser.intent` /
  `format_test.intent` cover the construct.
- [ ] Byte-equal self-format on all four stage2 files is preserved.
- [ ] `make build && make test` and the stage2 suite stay green on rust + js.

Constructs (one task each):
- US-005 Entity `invariant { … }` blocks (bank_account, js_demo, task_queue)
- US-006 `forall` / `exists` quantifier expressions (sorted_check)
- US-007 `implies` operator (try_operator)
- US-008 Generic type params on declarations `<T>` (generic_stack)
- US-009 `Fn(…) -> T` types + lambdas (closure_demo)
- US-010 `async` functions + `await` (async_demo)
- US-011 Attributes `@name(args)` on decls/tests (target_specific_demo)
- US-012 char_string_demo: compare stage2 output against stage1 *output* (not the
  non-canonical source); confirm agreement or file a real-bug follow-up.

## 4. Functional Requirements

- FR-1: `args()` is a recognized builtin in checker, IR lowering, and the rust,
  js, and wasm backends, returning `Array<String>`.
- FR-2: `main.intent` is a buildable entry program reading `args()[1]`.
- FR-3: `intentc fmt` accepts `--self-hosted` (composes with `--check`).
- FR-4: `difftest.sh` + `make diff-formatter` exist and gate on full corpus PASS.
- FR-5: Each gap-closing task is additive — no regression to byte-equal
  self-format on the stage2 files, no regression to the Go test suite.

## 5. Non-Goals

- Replacing stage1's Go formatter as the default (`intentc fmt` without
  `--self-hosted` stays Go).
- WASM runtime support for `args()` (compiles to empty; no real argv).
- Rewriting the linter or compiler in Intent (later milestones).
- A stdin/stdout streaming mode for the formatter (file-path arg only this phase).
- Fixing `char_string_demo` to be stage1-canonical (US-012 only diagnoses).

## 6. Technical Considerations

- `print` trailing newline: verify whether the rust/js `print` builtin appends a
  newline; the shim and harness must compare modulo exactly one trailing newline,
  or `main.intent` must avoid adding one. Establish this in US-002 and document it.
- `--self-hosted` binary location: building the stage2 formatter via cargo on
  every invocation is slow. Acceptable first cut: build once to a cached path
  (e.g. under the OS temp/cache dir) or honor an `INTENT_STAGE2_FMT` env override
  pointing at a prebuilt binary; rebuild when stage2 sources are newer. Document
  the chosen mechanism. JS target (node) is a lighter alternative to cargo.
- Probe pattern: in-language `read_file` needs ABSOLUTE paths because
  `intentc test` runs from a temp cwd (see progress.md `stage1-test-io`).
- Avoid Rust keywords as Intent identifiers in stage2 (see progress.md
  `stage1-rust-backend`).
- Multi-line/indent-dependent emit uses `format_expr_indented(e, level)`.

## 7. Success Metrics

- `make diff-formatter` reports all 21 canonical examples PASS (char_string_demo
  allow-listed or fixed).
- `intentc fmt --self-hosted` produces byte-identical output to native
  `intentc fmt` on every passing example.
- `make build`, `make test`, `make validate`, and the stage2 `--all-targets`
  suite stay green throughout.

## 8. Open Questions

- Should `--self-hosted` build the rust or the js stage2 binary by default?
  (Leaning js/node for speed and no cargo dependency; decide in US-003.)
- Final disposition of char_string_demo (US-012): real formatter bug vs. an
  intentionally non-canonical fixture.
