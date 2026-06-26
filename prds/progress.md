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

---

## 2026-06-16 — Phase 42.6: forall/exists quantifiers

ex_forall/ex_exists expression kinds (name=loop var, children=[domain, body]).
parse_quantifier in parse_primary: `forall <id> in <range>: <expr>` (`in` consumed
as a plain ident, matching for-loops; domain via parse_range; `:` = tk_colon).
format_expr_inner emits `forall <v> in <domain>: <body>`; expr_precedence=1.
+3 round-trip tests. sorted_check PASS → 19/22, 0 diverged. Stage2 suite 178/178
rust+js. selfcheck-formatter: all 4 EQUAL. Clean (no scope expansion).

---

## 2026-06-16 — Phase 42.8: generic type params + generic instantiation

EntityDecl/FunctionDecl gain type_params: Array<String>, DEFAULTED in the
constructor body (empty_string_array()) so no constructor-signature change and no
call-site churn (clean trick). parse_type_param_list parses optional `<T, U>` after
entity/function name. Generic instantiation `Ident<Args>(...)`: lookahead_is_generic_call
scans `< ... > (` without consuming to disambiguate from `<` comparison; on match,
the type-arg text is embedded in the callee ident (e.g. "Stack<Int>") so the
existing ex_call path formats it back verbatim. format_type_params emits "" for
empty (so the type-param-free stage2 files stay byte-equal). +12 tests. generic_stack
PASS → 20/22, 0 diverged. Stage2 suite 190/190 rust+js. selfcheck: all 4 EQUAL.

PATTERN: [stage2-add-field] To add a field to an existing stage2 AST entity without
touching every constructor call site, declare the field and DEFAULT it in the
constructor body (don't add a constructor parameter); the parser assigns the real
value post-construction (mutable local). Keeps the diff small + low-risk.

PATTERN: [never-stage1-fmt-stage2] Do NOT run `intentc fmt` (stage1) on a stage2
source file — stage1 and stage2 have diverging comment/paren behavior, so stage1
can "reformat" a stage2 file into something the stage2 formatter no longer treats
as a fixpoint. Verify/maintain stage2 files only via the stage2 binary
(make selfcheck-formatter).

---

## 2026-06-16 — Phase 42.9: Fn types + lambdas

New tk_thin_arrow token for `->` (the `-` branch peeks for `>`; stage2's existing
tk_arrow is the FAT `=>`). parse_type_name detects `Fn` + `(` and reconstructs
`Fn(T1, T2) -> R` verbatim (stored as a type string, so formatting is automatic —
same pattern as generic types). ex_lambda kind with a new Expr.lambda_params:
Array<Param> field (defaulted in ctor body via empty_param_array(), no signature
change); parse_lambda parses `|name: type, ...| [-> Ret] => body`; format_expr_inner
emits it. +6 tests (+2 lexer). closure_demo PASS → 21/22, 0 diverged. Stage2 suite
198/198 rust+js. selfcheck: all 4 EQUAL.

---

## 2026-06-16 — Phase 42.11: test attributes; CORPUS 22/22 COMPLETE

TestDecl.annotations: Array<String> (defaulted via empty_string_array() in ctor
body). parse_program handles leading `@` (tk_at already lexed): parse_test_annotations
loops `@name("a", ...)` building canonical strings, then dispatches to the (async)
test and sets annotations. format_test_decl emits each annotation on its own line
before the header (empty list = no-op, byte-equal preserved). +5 tests.

MILESTONE: differential harness 22/22 PASS, 0 diverged, 0 parse-err — the ENTIRE
examples corpus now formats byte-identically to stage1 `intentc fmt`. Stage2 suite
203/203 rust+js. selfcheck-formatter: all 4 stage2 files EQUAL. Phase 42 parser-gap
closing complete (42.5-42.11).

PATTERN: [intent-ctor-field-init] In stage2 Intent, initialize a non-primitive
field in a constructor body with a direct helper CALL (= empty_string_array()), not
a local variable — the rust backend emits the struct literal before constructor body
statements, so a local referenced in a field init isn't in scope yet.

---

## 2026-06-23 — Phase 43.1: ADR 0050 self-hosted linter strategy

Kicked off Phase 43 (rewrite the linter in Intent). Norman-scoped: PRD
`prds/active/prd-phase-43-self-hosted-linter.md`, 13 tasks in TASKS.md.

Recon (2 Explore agents): mapped the stage1 Go linter (16 rule families, all
warnings, format `warning[file:line:col]: message`, single-pass recursive walk,
no config/suppression) and the stage2 infra (lexer.lex / parser.parse -> Program;
full AST; decls carry `line` but NOT `column`).

Corpus baseline measured: `intentc lint examples/*.intent` = 76 warnings / 13 files,
exercising 8 of 16 rule families (unused var 25, mutable-never-reassigned 14,
method no-contracts 14, function no-contracts 12, fn naming 4, entity no-invariant 4,
unused param 2, trait method 1). Other 8 families need golden fixtures.

ADR 0050 records: D1 reuse `selfhost/formatter/` (grep confirms NO cross-dir imports
exist anywhere -> separate dir is unproven/risky); D2 byte-equal parity INCLUDING
`:col` (requires adding `column:Int` to stage2 AST decls); D3 all 16 rules this
phase, gated by `make diff-linter`.

PATTERN: [linter-corpus-gate] The examples corpus DOES fire lint warnings (76),
so the differential is a real gate for the 8 common families. The 8 non-corpus
families (extern, entity/enum/variant PascalCase, empty-body, intent verified_by,
type-params, spawn-discard) must be covered by committed golden fixtures.

---

## 2026-06-24 — Phase 43.2: column tracking in stage2 AST + parser

Added source-column (and line where missing) to the stage2 AST nodes the linter
anchors on. ast.intent: `+column` on the 9 decls that already had `line`
(FunctionDecl, ImportDecl, EntityDecl, EnumDecl, TraitDecl, ImplDecl, IntentBlock,
TestDecl, ExternDecl); `+line +column` on the 4 position-less nodes (Stmt, Param,
EnumVariant, TraitMethodSig). All defaulted to 0 in the constructor BODY (no
signature change). parser.intent: post-construction assignment from the leading
token at every parse site, INCLUDING error return paths (imp_err, ed_err,
ib_err1/2). parse_expr_stmt peeks `self.tokens[self.position]` for its anchor.
+4 position tests in parser.intent.

Independently verified gates: build OK; parser suite 106/106 (rust); format_test
207/207 (rust+js per worker); selfcheck-formatter all 4 EQUAL; diff-formatter 22/22.

PATTERN: [ast-evolution] New optional position fields (line/column) are defaulted
to 0 in the constructor body and assigned post-construction at each parse site —
never added as constructor parameters — so all existing call sites stay unchanged.
Same pattern as Stmt.comments_before / FunctionDecl.type_params.

---

## 2026-06-24 — Phase 43.3: linter core scaffold + diagnostic model

New files selfhost/formatter/lint.intent (module formatter_linter) + lint_test.intent.
LintDiag entity (line, column, message); lint_program dispatch walk; format_diags;
is_snake_case / is_pascal_case (exact ports); R5 (function snake_case) wired
end-to-end. +18 lint tests. Gates: build OK, lint_test 124/124 rust+js,
selfcheck 4 EQUAL, diff-formatter 22/22.

CORRECTION: the stage1 Lint() dispatch order is functions -> externs -> entities ->
enums -> traits -> impls -> intents (linter.go:25-31), NOT externs-first as an
earlier recon note claimed. Verified against source.

CRITICAL for the differential (43.12): diagnostic.Format does NOT sort — it prints
in APPEND order (diagnostic.go). So stage2 must emit in stage1's exact order, BOTH
across decl kinds (above) AND per-decl when multiple rules fire on one decl. Each
rule task (43.4-43.9) must append in stage1's per-function/per-entity rule order.

STAGE1 TOTAL OUTPUT (for lint_main, 43.10): no warnings -> "No lint warnings.\n".
With warnings -> Format() (lines joined by \n, NO trailing) + Println() (one \n) +
"N warning(s) found.\n". Net: every warning line incl. last ends \n, no blank line
before summary. format_diags already emits trailing \n per line, which composes
correctly; lint_main must account for print()'s extra newline (formatter-shim trick).

PATTERN: [cross-target-naming] Avoid parameter/variable names reserved in Intent OR
Rust: fn, impl, result, type, trait, enum, use, mod, pub, let, for, in, loop, match,
where, self, super, crate. Use compound names (fdecl, impl_decl, out). Caught at
compile time but wastes a cycle.

PATTERN: [array-accumulation] Stage2 Intent arrays are pass-by-value: accumulate
with a local `let mutable arr: Array<T> = [];` + arr.push(...) directly. NEVER pass
an accumulator array into a helper and push inside the callee — the mutation is
silently discarded (and a drain_into helper generated broken Rust). lint_program
inlines the per-kind merge loops for this reason.

---

## 2026-06-24 — Phase 43 task reorg + authoritative port spec

Reorganized remaining rule tasks from rule-family (old 43.4-43.9) to
DISPATCH-FUNCTION-oriented (new 43.4-43.7). Reason: diagnostics are NOT sorted
(emit order = output order), and stage1 interleaves rules WITHIN each dispatch
function in a fixed order, so rule-family tasks would make fragile order-dependent
edits to the same lint_* functions. Dispatch-oriented tasks each own whole functions
with all their rules in stage1 order — no cross-task ordering coordination. Same 16
rules, same byte-equal gate. Net 13 -> 11 tasks.

Appended PRD section 9 "Stage1 port reference (AUTHORITATIVE)": full dispatch order,
per-decl check order, message templates, and the used-name/assigned-name engine
stage2 mapping. All remaining worker briefs reference it.

KEY SPEC FINDINGS (verified against linter.go + parser.intent):
- Dispatch order: functions -> externs -> entities -> enums -> traits -> impls ->
  intents (NOT externs-first).
- No sort in diagnostic.Format -> per-decl rule order is load-bearing.
- Trait-name naming uses checkEntityNaming -> emits "entity 'X' should use PascalCase
  naming" (literally "entity" even for a trait) — replicate verbatim.
- Trait methods: naming BEFORE contracts. Impl methods: no contracts, no
  mutable-never-reassigned. Methods: no type-param/spawn checks.
- STAGE2 HAS NO ASSIGNMENT STATEMENT: `x = y` parses as st_expr whose expr is
  ex_binop with name "=" (parser test 1557). The used-name engine must treat the
  LHS plain-ident as a WRITE (not collected), collect RHS + field/index-target
  objects; collect_assigned_names keys off this for R13.
- ex_call: skip the callee if it's a plain ident (function name not a "read"); if
  callee is ex_field, recurse it (method receiver IS read). ex_paren must be walked
  (stage2-only node; stage1 has no paren node).

---

## 2026-06-24 — Phase 43.4: used-name + assigned-name engine

Added to lint.intent (functional style, returns Array<String>, due to pass-by-value):
collect_used_names / collect_used_names_from_stmt / collect_used_names_from_expr,
collect_assigned_names, name_in, plus concat + append_names helpers. Faithful port
of stage1: assignment (st_expr ex_binop "=") LHS plain-ident is a write (not
collected); field/index targets collect their object/index; ex_call callee skipped
if ex_ident, recursed if ex_field (method receiver); ex_paren walked (stage2-only).
+5 engine unit tests incl. the assignment-target-not-read case. Gates: build OK,
lint_test 129/129 rust+js, selfcheck 4 EQUAL, diff-formatter 22/22.

Confirmed method-call layout (parser.intent:1824-1832): obj.m(a) =
ex_call(children=[ex_field(children=[obj], name="m"), a]).

PATTERN: [rust-temp-borrow] The rust backend emits Array params as &Vec<T>, and a
temporary (a function's return value) cannot be auto-borrowed into another call.
Store recursive-call results in a named `let mutable tmp = helper();` before passing
to concat/append. Recurs throughout the functional-style engine.

---

## 2026-06-24 — Phase 43.5: complete lint_function_decl

All function-level rules in stage1 order: R10 empty-body, R1 contracts (skip entry),
R5 naming, [R15 deferred to 43.8], then for non-empty body: R14 unused-params, R12
unused-vars, R13 mutable-never-reassigned, R16 discarded-spawn. Reusable helpers
check_unused_params / check_unused_variables (recursive via collect_let_stmts) /
check_mutable_never_reassigned (top-level only) / find_discarded_spawns (recursive) —
will be reused by 43.6 entity/impl methods. R5 tests rewritten to message-presence
(has_diag_msg) since R1 now fires on contract-less test fns. lint_test 143/143
rust+js; selfcheck 4 EQUAL; diff-formatter 22/22.

ADVISOR REVISE (caught 4 byte-equal divergences the worker's tests missed):
- R15 was wrongly implemented with .contains substring match + function-column
  anchor → removed (deferred to 43.8 where type_params get position).
- R12/R13 were interleaved in one loop → split into separate full passes (stage1
  runs checkUnusedVariables fully, THEN checkMutableNeverReassigned).
- R13 walked recursed lets → fixed to TOP-LEVEL only (stage1 asymmetry: R12 recurses,
  R13 does not; only collect_assigned_names recurses).
- R16 was top-level only → made recursive into if/while/for (matches stage1).

PATTERN: [stage1-rule-order] Diagnostics are emitted in append order (no sort), so
per-decl rule order AND each rule's recursion behavior must match stage1 exactly.
checkUnusedVariables recurses; checkMutableNeverReassigned does NOT; checkSpawnWithoutAwait
recurses. Verify recursion per-rule against linter.go, don't assume uniformity.

Cleanup: removed stray repo-root `lint_test` build artifact; added /lint_test +
/lint_main to .gitignore (mirrors /main for the formatter binary).

---

## 2026-06-24 — Phase 43.6: complete lint_entity_decl + lint_impl_decl

lint_entity_decl: R6 (PascalCase) -> R9 (no-invariant) -> [R15e deferred] -> ctor
R14 (scope "Entity.constructor") -> per method R10/R2/R5/R14/R12/R13 (methods get NO
R16, NO type-params). Exact message distinctions: R10 "function 'Entity.method' has
an empty body", R2 "method 'Entity.method' has no requires or ensures contracts", R5
"function 'method' ..." (UNqualified). lint_impl_decl: per method R10/R5/R14/R12 only
(impl methods skip R2/R13/R16). Reused 43.5 helpers. Also applied the guard fix in
lint_function_decl (body checks now unconditional, matching stage1 Body != nil).
+3 locking tests (R12-before-R13 ordering, R13-nested-no-fire, R16-nested-fires) +
entity/impl tests. lint_test 161/161 rust+js; selfcheck 4 EQUAL; diff-formatter 22/22.

PATTERN: [stage2-contract-clause-syntax] In a stage2 in-language test source string,
contract clauses are bare expressions WITHOUT semicolons: `requires true ensures
true` (NOT `requires true; ensures true;`). The semicolons cause stage2 parse errors.

---

## 2026-06-24 — Phase 43.7: enum/trait-naming/intent dispatch

lint_enum_decl (R7 enum PascalCase, R8 variant PascalCase), lint_trait_decl (R6
trait-name with the "entity 'X' should use PascalCase naming" stage1 QUIRK + R5
trait-method snake_case), lint_intent_block (R11 verifications empty). R3 (trait
method contracts) + R4 (extern contracts) DEFERRED to 43.8 because TraitMethodSig /
ExternDecl carry no requires/ensures (parser discards them). +11 tests. lint_test
172/172 rust+js; selfcheck 4 EQUAL; diff-formatter 22/22.

DISCOVERY: corpus has NO externs (R4 fixture-only) and handler_trait's lone trait
method has no contracts (R3 fires once). R3/R4/R15/R15e all need AST enrichment to
be byte-equal for the fixture cases → consolidated into 43.8.

PROCESS NOTE: out-of-order teammate messages had col-track re-confirming already-
committed work (43.6) instead of picking up 43.7; resolved by sending a disk-grounded
directive ("enum/intent dispatch are still stubs — execute now"). Lesson: trust the
working tree, not the message stream, and re-anchor the worker to disk when reports
drift from `git status`.

---

## 2026-06-24 — Phase 43.8: R3 (trait-method contracts) + R4 (extern contracts)

Both implemented as always-fire, which is byte-equal-correct: stage2's
parse_trait_method_sig and parse_extern_decl both expect `;` immediately after the
return type and CANNOT parse requires/ensures — so every trait method / extern that
parses is contract-less by construction. R3 inserted in lint_trait_decl AFTER R5
(stage1 order); R4 replaces the lint_extern_decl stub (uses ext.func_name). Matches
corpus (handler_trait R3 fires once; no corpus externs). +3 tests incl. R5-before-R3
order lock. lint_test 175/175 rust+js; selfcheck 4 EQUAL; diff-formatter 22/22.

PATTERN: [stage2-parser-surface] When a rule keys off a construct the stage2 parser
cannot represent (trait-method/extern contracts), check whether the parser can even
PARSE that construct. If it rejects it (parse error), then within the parseable
subset the rule's condition is constant, and "always fire" is byte-equal-correct —
no AST enrichment needed. (Contrast type-params, which ARE parsed but lack position.)

Task split: old 43.8 (enrichment + R3/R4/R15/R15e) split into 43.8 (R3/R4, trivial)
and 43.9 (type-param position enrichment + R15/R15e). Net 12 -> 13 tasks.

---

## 2026-06-24 — Phase 43.9: type-param position enrichment + R15/R15e (ALL 16 RULES DONE)

Additive parallel position arrays type_param_lines/type_param_columns on FunctionDecl
and EntityDecl (defaulted via new empty_int_array() helper; formatter untouched, so
selfcheck/diff-formatter unaffected). Parser: Parser.tp_lines/tp_columns fields,
populated in parse_type_param_list (reset + push per type-param token), assigned to
fd/ed at the call sites (incl. entity error path). token-aware type_uses_param
(extracts maximal identifier runs, whole-token match — "Type"/"TT" do NOT match "T").
R15 in lint_function_decl after R5 (anchored at type_param position); R15e in
lint_entity_decl after R9 (message has NO "in parameters or return type" suffix —
stage1 difference). +13 tests incl. a column-position assertion. lint_test 188/188
rust+js; selfcheck 4 EQUAL; diff-formatter 22/22.

MILESTONE: all 16 Go-linter rule families now implemented in the stage2 Intent linter.

PATTERN: [intent-no-break] Intent has no `break` statement — early-exit loops use a
`let mutable running: Bool = true; while i < n and running { ... running = false; }`
guard pattern (used in type_uses_param).

PROCESS NOTE: a clean-tree + zero-marker disk snapshot wrongly read as "stalled" while
col-track was actually about to write 43.9; I sent a stand-down, then saw ast/parser
edits land and retracted it. Lesson: with laggy teammate messages, a single disk
snapshot can race the worker's writes — confirm with a second snapshot before
concluding a stall, and prefer retract-and-let-finish over taking over mid-edit.

Cleanup: removed stray probe_col.intent (worker's column probe).

---

## 2026-06-24 — Phase 43.10: runnable lint_main.intent

Created selfhost/formatter/lint_main.intent (entry main: args() -> read_file ->
parse -> lint_program -> output). Output composition byte-matches stage1
`intentc lint`: zero diags -> "No lint warnings." ; else format_diags(diags, path)
+ len(diags).to_string() + " warning(s) found." (print adds the single trailing
newline; format_diags already ends each line with \n, so this reproduces stage1's
Print(Format)+Println()+Printf exactly). Builds on rust + js. Exit codes 0/1/2/3
mirror the formatter main.

INDEPENDENT VERIFICATION (built rust binary, diffed vs `intentc lint`): byte-IDENTICAL
on array_sum(1), map_demo(18), task_queue(17), enum_basic(12), io_demo(9),
handler_trait(5). The high-warning files exercise multiple rules per decl + emit
ordering — strong evidence the full port is faithful. Effectively the differential
gate already passing; 43.12 formalizes it across the whole corpus + fixtures.
Gates: selfcheck 4 EQUAL, diff-formatter 22/22, lint_test 188/188.

---

## 2026-06-24 — Phase 43.11: intentc lint --self-hosted Go shim

Mirrored the fmt --self-hosted shim in cmd/intentc/main.go: parseLintFlags
(--self-hosted), stage2LinterBinary (INTENT_STAGE2_LINT env override; else builds
selfhost/formatter/lint_main.intent to a cached temp binary `intent-stage2-lint`,
rebuilding when any selfhost/formatter/*.intent is newer), runStage2Linter (runs the
binary, returns stdout VERBATIM — no trailing-newline trim, since lint_main output
already byte-matches stage1; non-zero exit surfaced as error, no fallback). Usage
text updated. +Go tests (parseLintFlags, runStage2Linter fake-binary, env-override
CLI path incl. parse-error-no-fallback). Verified: `intentc lint --self-hosted`
byte-identical to `intentc lint` on array_sum/map_demo/enum_basic/hello. Full Go
suite passes.

NOTE: done INLINE by the orchestrator (not delegated) in parallel with col-track's
43.12 (differential harness) — disjoint files (cmd/intentc/*.go vs selfhost/ +
Makefile) so no write race.

KEY DIFF vs fmt shim: runStage2Linter does NOT trim a trailing newline. The fmt
shim trims one (formatter stdout = source + print's \n, but file content has no
extra \n). The linter's stdout already equals stage1's exact bytes (the summary
line's \n IS print's \n), so it's emitted as-is.

---

## 2026-06-24 — Phase 43.12: differential harness + fixtures + make diff-linter

selfhost/formatter/difftest-lint.sh builds the stage2 lint_main binary and compares
its stdout to stage1 `intentc lint` across examples/*.intent + lint-fixtures/*.intent.
4 fixtures cover the non-corpus rules: r6_r7_r8_naming (R6/R7/R8/R9), r10_empty_body
(R10/R1), r11_intent_no_verified (R11), r15_unused_type_params (R15/R15e/R1/R9).
Makefile `diff-linter` target + .PHONY. RESULT: `make diff-linter` = 26/26 PASS,
0 diverged, 0 parse-err — the stage2 self-hosted linter is BYTE-EQUAL with stage1
`intentc lint` across the full corpus + fixtures. With the 188 unit tests (which
cover R4), all 16 rule families are validated.

FINDING (real, verified): R4 (extern no-contracts) CANNOT be differentially gated —
stage1 grammar requires `extern function NAME(...) returns T from "crate::path";`
(grammar.ebnf:96, `from` clause mandatory) while stage2 parses `extern "target"
function NAME(...) returns T;`. No single source parses in both. R4 is verified by
the stage2 unit test in lint_test.intent instead; documented in the Makefile comment
+ difftest-lint.sh header. (Removed an erroneous stage2-syntax R4 fixture I had added.)

Other gates unregressed: selfcheck 4 EQUAL, diff-formatter 22/22, lint_test 188,
full Go suite passing.

---

## 2026-06-24 — Phase 43.13: docs + final validation (PHASE 43 COMPLETE)

docs/ROADMAP.md: added "Phase 43: Self-Hosted Linter — SHIPPED" entry (above Phase
42). prds/NEXT-STEPS.md: rewritten for Phase 43 complete + candidate next directions
(recommend self-hosting the checker next). selfhost/README.md + selfhost/formatter/
README.md: updated to document the linter living alongside the formatter (shared
stage2 lexer/parser/AST per ADR 0050 D1), the new files, the Phase 43 row, and the
diff-linter command. PRD moved active/ -> done/. Phase 43 collapsed in TASKS.md ->
one-line summary; full 13-row table appended to TASKS-archive.md.

FINAL VALIDATION (all green): make validate OK (gofmt + build + go test + examples);
make selfcheck-formatter 4 EQUAL; make diff-formatter 22/22; make diff-linter 26/26.

PHASE 43 COMPLETE. The self-hosted (stage2) linter is byte-equal with stage1
`intentc lint` across the corpus + fixtures, all 16 rule families, wired as
`intentc lint --self-hosted`. Second self-hosted toolchain artefact after the
formatter. Next milestone: self-hosting the compiler (start with the checker).

---

## 2026-06-25 — Phase 44.1: ADR 0051 selfhost/shared restructure

Kicked off Phase 44 (selfhost/shared restructure — the precondition for the
self-hosted checker, Phase 45). User chose BOTH the ambitious checker scope
(scope-stack + name-resolution + arity) AND doing the shared/ restructure now (the
3rd-tool trigger ADR 0050 D1 set). Split into Phase 44 (restructure, pure refactor)
+ Phase 45 (checker) to isolate restructure risk behind its own green-gate checkpoint.

ADR 0051 records D1-D5: do-it-now, shared/+sibling-dirs layout, cross-dir imports
(verified feasible — registry.go:509 joins entryDir+importPath, `..` supported),
shared_* module rename, pure-refactor-gate-protected. PRD in active/.

Recon (2 Explore agents): Go checker is ~4281 LOC / ~167 diagnostics / full type
system (Type struct, scope stack, generics) — far too big for one phase. Stage2 AST:
types are flat Strings, no symbol table, no Map (only Array). Structural checks
(dup-decl, break/continue, return-in-test) need zero machinery; undefined-var + arity
need an Array-based scope stack. Checker errors use the same diagnostic.Format ->
`error[file:line:col]: message`. Valid corpus produces ZERO errors, so the Phase 45
differential needs INVALID fixtures + a no-false-positives-on-corpus direction.

---

## 2026-06-25 — Phase 44.2: cross-directory import spike — CONFIRMED

Built a throwaway app/main.intent with `import "../lib/mathx.intent"` and a sibling
lib/. `intentc build --target rust --emit` and `--target js --emit` both succeeded —
the `../` cross-directory module import resolves correctly (registry.go:509 joins the
entry file's dir + import path, `..` handled by filepath.Clean). The selfhost/shared
restructure needs NO stage1 change. Greenlight for 44.3.

---

## 2026-06-26 — Phase 44.3: selfhost/shared/ + re-point formatter & linter

git mv lexer/ast/parser → selfhost/shared/; renamed modules formatter_{lexer,ast,
parser} → shared_{...} (~601 qualified refs); formatter AND linter files (still in
selfhost/formatter/) now import "../shared/...". Updated selfcheck.sh (checks
shared/{lexer,ast,parser} + formatter/format) and difftest.sh probe imports. The
identifier-only renames preserved canonical formatting (no reformat needed).

Gates green: build OK; selfcheck-formatter 4 EQUAL (shared/lexer, shared/ast,
shared/parser, formatter/format); diff-formatter 22/22; diff-linter 26/26 (linter
still in formatter/ but importing ../shared/); go test ./... pass; stage2 suites
207 (format_test) + 188 (lint_test) rust+js. Stray main.js js-emit artifact removed;
/main.js + /lint_main.js added to .gitignore.

NOTE: linter still physically in selfhost/formatter/ — it relocates to selfhost/
linter/ in 44.4 (imports already ../shared/, so the move won't change them since
formatter/ and linter/ are both siblings of shared/).

---

## 2026-06-26 — Phase 44.4: relocate linter to selfhost/linter/

git mv lint.intent / lint_main.intent / lint_test.intent / lint-fixtures/ →
selfhost/linter/. Renamed modules formatter_linter→linter, formatter_linter_test→
linter_test, formatter_lint_main→lint_main. The ../shared/ imports were unchanged
(linter/ and formatter/ are both siblings of shared/). Updated difftest-lint.sh paths
(lint_main + fixtures dir). Go shim cmd/intentc/main.go: stage2LinterBinary source →
selfhost/linter/lint_main.intent; AND both stage2LinterBinary + stage2FormatterBinary
staleness checks now ALSO scan selfhost/shared/ (so editing a shared file rebuilds the
cached binary — a 44.3 oversight fixed here).

Gates green: build, go test ./...; selfcheck 4 EQUAL; diff-formatter 22/22; diff-linter
26/26; lint_test 188 rust+js; `lint --self-hosted` byte-identical to stage1;
`fmt --self-hosted` OK.

The selfhost/ restructure (shared/ + formatter/ + linter/) is structurally complete;
44.5 is docs + final validate + push.

---

## 2026-06-26 — Phase 44.5: docs + final validate (PHASE 44 COMPLETE)

selfhost/README.md updated to the shared/+formatter/+linter/ layout. New
selfhost/shared/README.md and selfhost/linter/README.md; selfhost/formatter/README.md
trimmed to formatter-only (front-end → ../shared/, linter → ../linter/). docs/ROADMAP.md
Phase 44 entry. prds/NEXT-STEPS.md rewritten for Phase 44 complete + Phase 45 (checker)
recon carried forward. PRD active/→done/. Phase 44 collapsed in TASKS.md → archive.

FINAL VALIDATION (all green): make validate OK; selfcheck-formatter 4 EQUAL;
diff-formatter 22/22; diff-linter 26/26.

PHASE 44 COMPLETE. selfhost/ is now shared/ + formatter/ + linter/, each a clean
sibling importing ../shared/. Ready for Phase 45 (self-hosted checker, ADR 0052).

---

## 2026-06-26 — Phase 45.1: ADR 0052 self-hosted checker strategy

Kicked off Phase 45 (self-hosted checker — first compiler subsystem). New
selfhost/checker/ sibling reusing ../shared/. ADR 0052 records: D1 first-slice scope
(structural + name-resolution + arity, NO type inference), D2 type inference deferred
(needs structured TypeRef/Type entity), D3 Array-based scope stack (no Map), D4
two-directional differential (invalid fixtures byte-equal + no-false-positives on the
22 valid examples, which produce zero errors), D5 faithful gate-protected port.

Verified `intentc check` output contract (main.go:220-266): valid → "No errors found.\n"
stdout exit 0; invalid → diag.Format() (error[f:l:c]: msg, \n-joined, NO trailing) on
STDERR exit 1; not sorted (walk order). check_main prints stdout-only, so the shim +
differential reconcile the stderr/stdout split + exit code. PRD in active/, 9 tasks.

---

## 2026-06-26 — Phase 45.2: checker scaffold + duplicate-declaration check

selfhost/checker/check.intent (module checker, imports ../shared/) + check_test.intent.
CheckDiag(line,column,message); format_diags renders `error[file:line:col]: message`
joined by \n with NO trailing newline (matches stage1 diagnostic.Format exactly —
differs from the linter's trailing-\n format_diags). check_program does duplicate
top-level decl detection for all 4 kinds. +8 tests (incl. format_diags edge cases),
114 passed rust+js. Unaffected gates green: selfcheck 4 EQUAL, diff-formatter 22/22,
diff-linter 26/26. No intent.toml needed.

KEY: dispatch/emit order is enums → entities → traits → functions, matching the stage1
register passes (checker.go:113-116: registerEnums, registerEntities, registerTraits,
registerFunctions). Diagnostics are NOT sorted, so this order is load-bearing for
byte-equal when a program has dups in multiple kinds. (My Phase-44 recon note had the
register order wrong; the worker correctly read the source — verified checker.go:113-116.)

---

## 2026-06-26 — Phase 45 plan refinement: discovered break/continue parser gap

DISCOVERY: examples/error_handling.intent (valid corpus, in the 22/22 formatter set)
uses real `break;` / `continue;`, but the stage2 lexer has NO break/continue keyword —
they lex as plain identifiers, so `break;` parses as st_expr(ex_ident("break")). That
round-trips fine for the formatter (emits the identifier + `;`), but for the checker:
(1) break/continue-outside-loop can't be detected (no break/continue stmt nodes), and
(2) the undeclared-variable check (45.7) would FALSE-POSITIVE on `break`/`continue` on
error_handling.intent, breaking the no-false-positives gate.

DECISION: add real break/continue statement support to the stage2 front-end as a
discovered prerequisite (new task 45.4): kw_break/kw_continue in the lexer, st_break/
st_continue stmt kinds in ast, parser support, formatter emit. This unblocks the
outside-loop check AND keeps name-resolution faithful. Formatter must still round-trip
error_handling.intent byte-equal (diff-formatter 22/22). Phase 45 grows 9 -> 11 tasks.
45.3 (dup-variant + return-in-test) is unblocked and proceeds first.

---

## 2026-06-26 — Phase 45.3: dup enum variant + return-in-test

check.intent now structured as register phase (dup-decl + dup-variant) then check
phase (return-in-test, and later break/continue/undeclared/arity) to match stage1's
unsorted emit order. dup-variant emitted inside the per-enum loop at the duplicate
variant's line/col (`duplicate variant name 'X' in enum 'Y'`). return-in-test walks
each test body recursively for st_return, message `'return' is not allowed inside a
test body; test "NAME" has implicit Void return` (Go %q → double-quoted name). +6
tests incl. a register-before-check ORDER lock; 120 passed rust+js. Gates green:
selfcheck 4 EQUAL, diff-formatter 22/22, diff-linter 26/26.

---

## 2026-06-26 — Phase 45.4: stage2 break/continue statement support

Front-end widening (the discovered prereq): kw_break()/kw_continue() + keyword-table
entries (shared/lexer.intent); st_break()/st_continue() stmt kinds (shared/ast.intent);
parser builds st_break/st_continue Stmts on those keywords + `;` (shared/parser.intent);
format.intent emits `break;`/`continue;`. Now `break;`/`continue;` parse as real
statements, NOT st_expr(ex_ident("break")).

Gate-CRITICAL result: examples/error_handling.intent (uses break;/continue; at lines
60/76) still round-trips byte-equal → diff-formatter 22/22; selfcheck 4 EQUAL;
diff-linter 26/26. stage2 suites: parser 108, format_test 210, lint_test 190,
check_test 122; go test ./... green. This unblocks 45.5 (break/continue-outside-loop)
and removes the undeclared-var false-positive risk for 45.7.

---

## 2026-06-26 — Phase 45.5: break/continue-outside-loop (unified body walker)

Refactored the check phase into a single recursive check_body_stmts(stmts, loop_depth,
in_test, test_name) walker emitting both break/continue-outside-loop (when loop_depth==0:
`break statement outside loop` / `continue statement outside loop`) AND return-in-test
(when in_test), in source order. loop_depth increments inside while/for bodies only (not
if). Called per body in stage1 check order: functions → entity methods+ctors → impl
methods → tests. +8 tests (128 rust+js) incl. nested if-in-while (no fire) and a
return-before-break order lock. Gates green: selfcheck 4 EQUAL, diff-formatter 22/22,
diff-linter 26/26.

---

## 2026-06-26 — Phase 45.6: Array-based scope stack / symbol table

Scope entity { local_names: Array<String>, outer_names: Array<String> } — flattened,
NO recursive parent field (would be infinitely-sized in rust), no Map. Functional ops
(return new Scope, dodging pass-by-value): scope_empty, scope_define (append to local),
scope_enter (outer = outer++local, local=[]), scope_resolve (local OR outer),
scope_resolve_local (local). build_global_scope seeds all decl names (entities/enums/
functions/traits) + 23 free builtins: print, len, assert, assert_eq, assert_close,
assert_panics, read_file, write_file, create_dir, file_exists, env_get, args, http_post,
http_get, json_get, json_path, emit_event, timestamp_ms, sleep, await_all, await_any,
timeout, char_from_codepoint. (Method-style builtins like push/get/set/remove are NOT
seeded — they're resolved via field access, not bare identifiers.) +4 tests (132).
Gates green. Foundation for 45.7 undeclared-variable; the no-false-positives gate
(45.7/45.10) will catch any missing builtin on the corpus.
