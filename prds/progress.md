# Norman Progress Log

Append-only execution log for crash recovery. Newest entries at the bottom.

---

## 2026-06-15 — Norman initialized + full migration from ops/

Switched from aiki (config removed by user) to norman. Initially set up lightweight,
then per user direction did a **full migration** of the project's planning layer:

- Moved all 31 phase PRDs out of `ops/plans/`: 29 shipped → `prds/done/`,
  `phase-40a-comment-preservation` (active) → `prds/active/`,
  `phase-23-marketplace-publish` (Draft, blocked on credentials) → `prds/backlog/`.
- Moved `ops/NEXT-STEPS.md` → `prds/NEXT-STEPS.md`; removed the now-empty `ops/`.
- Created `prds/TASKS.md` (live driver: active Phase 40A.2 + backlog + completed pointer)
  and `prds/TASKS-archive.md` (29-phase completed index).
- Rewrote all 72 cross-references (ROADMAP, both READMEs, HARNESS.md, ADRs, Go test
  comments, inter-PRD links) from `ops/plans/...` → `prds/{done,active,backlog}/...`.
- Rewrote `docs/HARNESS.md` workflow sections to describe the norman flow.
- ADRs (`docs/decisions/`) and `docs/ROADMAP.md` stay put — norman does not own them.
- Flipped `config.md` scaffolding `lightweight` → `full`.

Stale-but-correct: `phase-30` and `phase-31` PRD `Status:` lines still read "Planning"
though both shipped (per ROADMAP + git). Left PRD bodies untouched; noted in the archive.

Current work stream: **Phase 40A.2 — comment preservation** in the stage2 self-hosted
formatter (`selfhost/formatter/`).
- step (1) trailing-EOF comments — DONE (commit 19d766e, pre-norman).
- step (2) body/between-statement comments — NEXT.
- step (3) inline-after comments — pending.
- step (4) byte-equal self-format dogfood gate — after 1–3.

PATTERN: [selfhost] - Comment preservation follows a uniform template: add a
`comments_before: Array<String>` field to the AST node, drain it from the relevant
token's `comments_before` in the parser, and emit it via `format_comments_before` in
the formatter. The lexer already attaches comments to the next non-trivia token (and
trailing comments to the synthetic EOF token), so no lexer change is usually needed.

PATTERN: [shell] - This environment's Bash tool runs under zsh, which does NOT
word-split unquoted `$var`. Use `... | while read -r f; do ...; done` for file-list
loops, not `for f in $files`.

PATTERN: [repo] - `intent/CLAUDE.md` is a symlink to `intent/AGENTS.md` — edit the
real target (AGENTS.md); writing through the symlink is refused.

---

## 2026-06-15 — Phase 40A.2 step (2): body/between-statement comments

Delegated to a Sonnet worker (first real norman worker run); advisor (Opus) wrote the
brief, reviewed the diff, and re-ran tests independently before committing. `Stmt`
gains `comments_before` (defaulted via a new module-local `empty_string_array()` in
ast.intent); `parse_block` captures each statement's first-token comments and assigns
them onto the Stmt; `format_stmt` is split into a comment-emitting wrapper +
`format_stmt_inner`. 5 new tests. 141/141 rust+js, independently re-verified. Code
review PASS — edits match the step-1 template exactly.

---

## 2026-06-15 — Phase 40A.2 step (3): inline-after comments on statements

Delegated to a Sonnet worker; advisor designed the lexer same-line-detection
mechanism, reviewed the diff, re-verified tests. Token gains `comment_after`;
Lexer gains `saw_newline_since_token` + `pending_inline_after`; `scan_all` rewritten
to hold the previous token in a local (so `comment_after` is set before push —
array-element mutation post-push is unreliable in the stage1 backend) and attach the
same-line comment to it. Stmt gains `comment_after`, captured from the `;` token in
parse_let/return/expr; the format_stmt wrapper appends ` <comment>` (canonical single
space). 5 new tests. 146/146 rust+js, independently re-verified. Code review PASS.

PATTERN: [stage1-backend] Mutating an already-pushed Array element (`arr[i].field = x`)
is unreliable; hold the element in a local, mutate, then push.

---

## 2026-06-15 — Phase 40A.2 step (4) + byte-equal probe finding; re-scoped 40A.3

Wrote a throwaway probe (compiled by stage1) that ran `format(parse(src)) == src` on
the four stage2 files. ALL diverge: first diff at index 0 (file-header comment before
`module` is dropped) and large length deltas (parser −6566 chars) from comments in
non-statement positions (entity method-docs, fields, end-of-block) plus column-aligned
inline comments a canonical formatter can't reproduce. So the "byte-equal gate" is a
mini-phase, not a test.

Delivered the achievable part as step (4): a comprehensive synthetic round-trip test
exercising all four supported comment positions together (leading-decl + inline-after +
leading-body + trailing-EOF). 147/147 rust+js. Probe deleted (not committed).

Re-scoped the remaining real-file byte-equal work as **Phase 40A.3** in TASKS.md
(module-leading, entity field/method, end-of-block, inline-after-field comments, then
canonicalize the source files + add the real-file gate). Phase 40A.2 is complete.

PATTERN: [stage1-test-io] `intentc test` runs the test binary from a temp cwd, so
`read_file` needs ABSOLUTE paths (relative paths silently skip); `print` inside a test
is suppressed — `write_file` to an absolute path to surface diagnostics.

---

## 2026-06-15 — Phase 40A.3 (real-file byte-equal): 40A.3.1, .2, .3, .5

- 40A.3.1 (module-leading comments): ModuleDecl.comments_before from the module token;
  format_program emits before the module line. Done myself; 149/149. Commit ada3c4d.
- 40A.3.2/.3/.5 (entity field + method/ctor comments, inline-after on fields): delegated
  to a Sonnet worker; advisor designed + reviewed + re-verified. FieldDecl gains
  comments_before + comment_after; entity/impl parse loops capture leading comments per
  member from the current token; format_entity_decl/format_impl_decl emit them (4-space
  indent) and field comment_after (single space). FunctionDecl.comments_before reused for
  methods/ctor. 4 new tests; 153/153 rust+js. Code review PASS.

Remaining in 40A.3: 40A.3.4 (end-of-block comments before `}`) and 40A.3.6 (canonicalize
the 4 stage2 files + add the real-file byte-equal gate).

---

## 2026-06-15 — Phase 40A.3 complete: byte-equal self-format achieved

- 40A.3.4 end-of-block comments: Block.trailing_comments from the rbrace token's
  comments_before; format_block_body emits them. 155/155. Commit e0fdc03.
- 40A.3.7 inline-after on declaration closing `}`: a probe revealed the formatter was
  DROPPING inline doc-comments on one-liner functions (`ex_void() ... } // placeholder`).
  Added Block.brace_comment_after (from rbrace.comment_after); brace_trail() helper
  appended by the 4 decl-body formatters.
- 40A.3.8 generic type round-trip: the REAL byte-equal blocker. parse_type_name discarded
  generic args and emitted the placeholder `Array<...>` (a Phase 33 shortcut). The first
  canonicalization attempt OVERWROTE the files with `Array<...>` as actual type syntax →
  broke compilation. Caught it before committing (validated the reformatted files compile),
  reverted via git checkout, fixed parse_type_name to reconstruct args verbatim with
  canonical spacing (`, ` after commas; nested `<>` via depth counter; `>>` = two `>`),
  re-applied brace_trail.
- 40A.3.6 canonicalize + gate: reran the formatter over all 4 files, confirmed comment-
  losslessness (zero dropped // comments) and idempotence, then overwrote the sources with
  their canonical form. Full suite 158/158 on the reformatted files. Probe confirms
  `format(parse(src)) == src` (firstdiff -1) on all 4. self_format_one upgraded from
  len>0 to assert_eq(out, src). Throwaway probe deleted.

PATTERN: [formatter] Idempotence (format(format(x))==format(x)) does NOT imply losslessness
— a stable drop is still a drop. Verify losslessness separately (compare extracted comments;
confirm reformatted files still compile + pass the suite) BEFORE canonicalizing committed source.

PATTERN: [stage2] The stage2 parser stored generic types as a `<...>` placeholder; any
formatter round-trip needs parse_type_name to reconstruct the real args.

---

## 2026-06-15 — Phase 41.1: contract clauses (requires/ensures/decreases)

Started Phase 41 (parser surface widening). 41.1: contracts were silently DISCARDED by
the parser (crude skip + skip_method_contracts, now removed). FunctionDecl gains
requires_clauses / ensures_clauses / decreases_clauses (Array<Expr>, defaulted via new
empty_expr_array()); parse_function_decl and parse_method_decl parse `<kw> <expr>` clauses
(idents, not keywords; no `;` — parse_expr stops at the next clause kw or `{`) into those
fields; an `is_contract_kw()` helper gates the loop. The formatter emits them between the
signature and the `{` (which moves to its own line at the decl's indent), via
format_contract_clauses, matching stage1's canonical layout (fibonacci.intent). `result`
in an ensures round-trips as a plain ident. 4 new tests; 162/162 rust+js; byte-equal
self-format preserved (verified with a throwaway probe).

PATTERN: [stage1-rust-backend] An Intent parameter/local named `fn` (or any Rust keyword)
generates invalid Rust (`fn.field` → "expected expression, found keyword fn") — JS is
unaffected, so it shows up as a rust-only "build or harness failure" / divergence. The
backend does NOT escape reserved words; avoid Rust keywords as identifier names in stage2.
(Renamed the offending param `fn` → `decl`.)

---

## 2026-06-15 — Phase 41.2: match expressions

Added `match <scrutinee> { <pattern> => <body>, ... }`. Lexer: `match` keyword
(kw_match_marker = 75). AST: `ex_match` kind, `MatchArm` entity (variant / bindings /
is_wildcard / body), `Expr.match_arms` (defaulted via empty_matcharm_array). Parser:
parse_match_expr + parse_match_arm dispatched from parse_primary; scrutinee parse stops
at `{`; patterns are `_` / `Variant` / `Variant(a, b)`; trailing comma optional.
Formatter: match is multi-line and indent-sensitive, so a new format_expr_indented(e,
level) is used by the let/return/expr statement formatters — it delegates to format_match
(level-aware: arms at level+1, `}` at level) for matches and to format_expr otherwise;
nested matches recurse through it. Non-statement matches (e.g. call args) fall back to
level 0 in format_expr_inner. 3 new tests; 165/165 rust+js; byte-equal self-format
preserved (stage2 files contain no match).

PATTERN: [formatter] Multi-line, indent-dependent expressions (match) don't fit the
level-agnostic format_expr. Thread level only where needed via a parallel
format_expr_indented(e, level) at statement boundaries, rather than refactoring every
format_expr call site.

---

## 2026-06-15 — Phase 41.3 (for-in) + 41.4 (try); Phase 41 complete

- 41.3 for-in: new st_for statement kind (reuses Stmt name=loop var, expr=iterable,
  then_block=body). `for` reuses kw_for_marker (shared with `impl ... for ...`; statement
  position disambiguates); `in` lexes as a plain ident and is consumed positionally. parse
  dispatched from parse_statement; formatted like while. +3 tests (collection, range,
  parser-populates).
- 41.4 try: ex_try postfix kind added to parse_postfix's loop (after `.`); expr_precedence
  10; format_expr_inner emits `inner?` with inner at postfix precedence so lower-prec inners
  keep parens. +2 tests.
- 170/170 rust+js; byte-equal self-format preserved on all 4 stage2 files (probe).

Phase 41 complete: the stage2 parser now handles contracts, match, for-in, and try —
the constructs it previously skipped or rejected. PRD moved to prds/done/.

---

## 2026-06-15 — Remove aiki block from CLAUDE.md / AGENTS.md

Replaced the ~500-line `<aiki>` instruction block in `AGENTS.md` (the real target of
the `CLAUDE.md` symlink) with concise norman-oriented project guidance: where state
lives (`prds/`), how to drive norman, and the project conventions (PRD lifecycle,
ADRs, validation harness, commit style). Dropped aiki/JJ-workspace-specific machinery
(workspace isolation, aiki task IDs, `aiki task run` delegation). Fixed the
`docs/HARNESS.md` "distinct from" list to note CLAUDE.md/AGENTS.md are the same file.

---

## 2026-06-15 — Phase 42 started: CLI wiring + differential test (42.1 args() builtin)

Phase 42 makes the stage2 formatter a runnable CLI tool + wires it into `intentc
fmt --self-hosted` + commits a differential-test harness vs `intentc fmt` over
examples/*.intent. Decisions (user): build the CLI tool up front (main.intent +
Go shim) then close gaps; harness = shell script + make target; input mechanism =
new `args()` builtin (not env var). Baseline differential probe: 12/22 examples
already byte-equal (= agree with stage1 fmt). Gaps: invariant blocks, forall,
implies, generics-on-decls, Fn/lambdas, async/await, attributes; char_string_demo
is non-canonical (needs vs-stage1-output compare).

42.1 args() builtin: returns Array<String>, program/script name at index 0 (ADR
0045, Rust+Go convention). Three-layer plumbing mirroring timestamp_ms: checker
type rule + arity error, IR resolveCallKind, backends — rust
`std::env::args().collect::<Vec<String>>()`, js `process.argv.slice(1)` (the
slice(1) normalizes node's [node,script,...] so script is index 0 matching rust),
wasm stub (push 0; no argv in pure wasm). +2 checker tests; full make test green;
emit verified on rust+js (incl args()[i] indexing + len(args())). Code review PASS.

PATTERN: [builtin-plumbing] A new builtin needs all three layers atomically:
checker (type rule + arity), IR resolveCallKind (add to CallBuiltin list, else it's
treated as a user call), and each backend's generateBuiltinCall switch. Missing a
layer = distinct failure (unknown fn / unresolved call kind / fallback sprintf).

---

## 2026-06-15 — Phase 42.2: runnable main.intent + stage1 entry-dedup fix

selfhost/formatter/main.intent: entry function main() reads args()[1], read_file,
parse, format_program, print. Exit codes 0 ok / 1 usage / 2 read-error / 3 parse-
error. print lowers to println!/console.log (adds one trailing newline), so stdout
= format_program(...) + "\n"; callers strip one trailing newline. Verified: built
on rust AND js, run on examples/hello.intent → byte-equal modulo one trailing
newline (140 vs 139 bytes); missing-file → exit 2, no-arg → exit 1.

Discovered + fixed a latent stage1 multi-file bug: rustbe/jsbe generateFunction
emitted the `fn main`/`__intent_main` wrapper for ANY function marked `entry`,
even in an imported (non-entry) module — so importing parser.intent (which carries
a standalone `entry function main` stub) produced a DUPLICATE main and the build
failed (rust E0428). Fix: gate the entry wrapper on `f.IsEntry && g.isEntryFile`,
so an entry fn in an imported module is demoted to an ordinary prefixed function
(e.g. formatter_parser_main); only the entry module's entry fn becomes the program
main. Single-file Generate() now sets isEntryFile:true (a single-file program is
its own entry); jsbe GenerateForTest clears mod.IsEntry so the invocation stays
suppressed in test mode (definition still emitted, unchanged). +1 regression test
each in rustbe/jsbe (TestEntryFunctionInImportedModuleNoDuplicateMain[JS]).

Verified no regression: full `go test ./...` green; stage2 suite 170/170 on rust
AND js; byte-equal self-format EQUAL on all 4 stage2 files (absolute-path probe).

PATTERN: [stage1-multifile] An `entry` function only becomes the program entry in
the ENTRY module. Imported modules that declare `entry function main` (standalone
stubs) must have it demoted to a regular prefixed function, else multi-file builds
emit a duplicate main. Gate entry-wrapper emission on f.IsEntry && g.isEntryFile;
single-file Generate sets isEntryFile=true. Relevant as the self-hosted compiler
will import many modules.

---

## 2026-06-15 — Phase 42.3: differential-test harness (+ 42.12 resolved)

selfhost/formatter/difftest.sh + `make diff-formatter`. Canonicalize-first design:
for each examples/*.intent it (1) copies + runs stage1 `intentc fmt` on the copy
(producing intentc fmt's canonical output), (2) runs the stage2 formatter
format_program(parse(canon)) via an absolute-path in-language probe, (3) PASS iff
stage2 reproduces the canonical form byte-for-byte = agrees with intentc fmt.
Per-file table + summary; exits 1 on any non-allowed divergence/parse-err (it's a
gate, NOT wired into make test/validate while gaps are open). bash-3.2 compatible
(replaced `mapfile` with a read-loop; macOS ships bash 3.2). Temp dir auto-cleaned
via trap.

Baseline: 13/22 PASS, 0 DIVERGE, 9 PARSE-ERR. KEY FINDING: zero true divergences —
whenever the stage2 parser accepts a file, the formatter emits byte-identical
output to stage1. All remaining work is PARSER COVERAGE (the 9 parse-errs =
gap buckets 42.5-42.11). The formatter's emit logic needs no corpus-driven fixes.

42.12 (char_string_demo) RESOLVED by this design: comparing vs stage1's *output*
(not the raw, non-canonical fixture) makes it PASS. The earlier standalone-probe
"DIVERGE" was only because examples/char_string_demo.intent isn't stage1-canonical
on disk; there is no stage2 formatter bug.

PATTERN: [difftest] The correct differential check is stage2-output == stage1-output,
NOT stage2-output == raw-file. Canonicalize each fixture with `intentc fmt` first,
then compare — otherwise non-canonical fixtures produce false divergences.

---

## 2026-06-15 — Phase 42.4: intentc fmt --self-hosted shim

Go shim wiring the stage2 formatter into the CLI. handleFmt now parses
--self-hosted (composes with --check, any order via parseFmtFlags). When set:
stage2FormatterBinary() resolves the binary (env INTENT_STAGE2_FMT override, else
auto-build selfhost/formatter/main.intent to $TMPDIR/intent-stage2-fmt, rebuilding
when any selfhost/formatter/*.intent is newer; build subprocess runs with cmd.Dir
= temp dir so no stray ./main in the repo). runStage2Formatter() execs it, trims
exactly one trailing newline (strings.TrimSuffix). Non-zero exit from the binary
(e.g. parse error, exit 3) is surfaced to stderr and the shim exits non-zero —
NO silent stage1 fallback. --check re-reads + compares; else writes in place.

Delegated to a golang-pro Sonnet worker; advisor (me) specified the factoring so
the unit tests use FAKE shell-script binaries (no cargo): runStage2Formatter +
parseFmtFlags + env-override = 13 tests. Code review PASS. End-to-end verified
myself: --self-hosted --check on canonical hello → exit 0 (auto-build ran);
--self-hosted --check async_demo → "parse error: ..." exit 1 (no fallback);
in-place stage2 output byte-equal to native intentc fmt on hello; no stray ./main.
make test PASS, gofmt clean.

CLI-wiring half of Phase 42 complete (42.1 args, 42.2 main.intent, 42.3 harness,
42.4 shim, 42.12 char_string_demo resolved). Remaining: parser-gap closing
42.5-42.11 (9 examples parse-err; 0 true divergences — emit logic is correct).

PATTERN: [cli-shim-testing] Factor subprocess-exec logic into a function taking a
binary path, then unit-test with a fake executable (shell script in t.TempDir(),
0755) that emits fixed bytes + chosen exit code — covers trailing-newline trim and
non-zero-exit handling without building the real (cargo-dependent) binary.

---

## 2026-06-15 — Phase 42.5: entity invariants (+ constructor contracts + intent blocks)

First harness-driven gap-closing task. The stage2 parser/formatter had a WRONG
invariant model: a block form `invariant { e; e; }` emitted at the END of the
entity body. Real Intent (grammar.ebnf:140) is `invariant <expr>;` per clause,
positioned BETWEEN fields and constructor (stage1 formatter.go formatEntityDecl).
Fixed: parser parse_invariant_block -> parse_invariant_decl (single expr + `;`);
formatter emits invariants after fields (blank line if fields existed), one
`    invariant <expr>;` per line, before the constructor.

To make the 3 target files (bank_account, js_demo, task_queue) FULLY pass, the
worker also folded in two adjacent constructs those files use (each was the NEXT
parse error after invariants):
- Constructor contract clauses: format_constructor_decl now emits
  requires/ensures/decreases via the existing format_contract_clauses helper
  (same layout as methods), parser parses them on constructors.
- Intent blocks: corrected IntentBlock AST (goal_text -> description + goals +
  constraints + guarantees) and format_intent_block to the real surface form
  `intent "desc" { goal: ...; constraint: ...; guarantee: ...; verified_by: [...]; }`.

Verified: differential harness 16/22 PASS (was 13), 0 diverged; byte-equal
self-format EQUAL on all 4 stage2 files (clean single-run probe); stage2 suite
171/171 on rust AND js. Diff is clean/idiomatic (reuses format_contract_clauses).

PATTERN: [verify-probe-reliability] A hand-written probe that round-trips MULTIPLE
large stage2 files (format.intent + parser.intent) in ONE `intentc test` process
gave a spurious partial/abort result (one file stuck pre-format). Single-file
byte-equal checks were reliable and all returned EQUAL. When a multi-file probe
shows an anomaly, re-check each file in its OWN test run before concluding a
regression — don't trust one combined probe.

PATTERN: [gap-closing-scope] Closing one parser gap on a real example often
unblocks the NEXT construct in the same file (invariant -> constructor contract ->
intent block). Expect to fold in adjacent constructs to make a target example go
green; the differential harness's per-file PASS is the true done-signal, not the
single named construct.

---

## 2026-06-16 — Phase 42.7 + 42.10: implies + await (and a key verification finding)

implies: binary operator, lowest precedence, right-associative. IMPORTANT: handled
inside parse_assign (after parse_or) rather than a dedicated parse_implies level,
to avoid deepening the recursive-descent chain (see stack finding below). await:
ex_await prefix in parse_unary; format_expr_inner emits "await " + inner;
expr_precedence(ex_await)=10. Also folded in (needed by async_demo): ex_spawn,
`async test` dispatch in parse_program, and fixed function modifier order to
stage1's public->async->entry. +4 round-trip tests. Harness 18/22 (try_operator +
async_demo now PASS), 0 diverged. Stage2 suite 175/175 rust+js.

KEY FINDING (corrected a wrong diagnosis): byte-equal self-format on the LARGE
stage2 files (parser.intent ~95KB, lexer.intent) CANNOT be reliably verified via an
in-language `intentc test` probe. `intentc test` runs rust tests through `cargo
test`/libtest, which executes each test on a SMALL (~2MB) thread stack; the deep
recursive-descent parse of a 95KB file overflows that thread stack and the test
binary aborts (no graceful error; non-deterministic because env/arg size shifts the
stack base). This is NOT a formatter bug: the BUILT stage2 binary (entry main, 8MB
main thread) self-formats all 4 files byte-equal with no problem — verified
directly. RUST_MIN_STACK did not help (the relevant frames are on a thread libtest
controls / or main thread, not std-spawned).

PATTERN: [verify-selfformat-via-binary] Verify byte-equal self-format on the large
stage2 files with the BUILT binary (`intentc build --target rust
selfhost/formatter/main.intent` then run it on each file, strip one trailing
newline, diff), NOT via an in-language `intentc test` probe — libtest's small
per-test thread stack overflows on the 95KB parser.intent and aborts. The earlier
"stack overflow blocker" was an artifact of this verification method, not the
formatter. Real usage (`intentc fmt --self-hosted`) is fine.
