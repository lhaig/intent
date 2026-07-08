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

---

## 2026-06-26 — Phase 45.7: Expr position + undeclared-variable + redefinition

Step 1: added line/column to the Expr entity (shared/ast.intent, defaulted 0 in ctor
body) and populated them at the two ex_ident parse sites (parser.intent ~785/791) from
the identifier token. Additive — selfcheck stayed 4 EQUAL, diff-formatter 22/22.
Step 2: extended check_body_stmts to thread a Scope: st_let checks the initializer in
the current scope, then resolve_local → `variable 'X' already defined in this scope`
(at the let stmt) else scope_define; nested if/while/for get child scopes; for-loop var
defined in the body scope. Step 3: check_expr_names walks expressions and emits
`undeclared variable 'X'` at the ident's line/column for unresolved ex_ident in use
position (skips plain-call callees + field names; recurses method receivers; defines
lambda/match/forall bindings in child scopes). Initial scopes: function=global+params,
method/ctor/impl=+self+params, test=global. +10 tests (142 rust+js). Gates green.

NOTE: tried an early no-false-positives spot-check over the 22 examples via an
`intentc test` probe calling check_program — it FAILS with a libtest stack overflow
(the deep parse+check of large examples exceeds the 2 MB test-thread stack), exactly
like the formatter/linter differentials. So the no-false-positives corpus sweep MUST
use the built checker binary (45.9) and runs in 45.10; a revise loop handles any false
positives surfaced there (module-qualified receivers are the main risk to watch).

---

## 2026-06-26 — Phase 45.8: function + variant call arity

Built function-arity (name→param count from prog.functions) and variant-arity
(variant name→field count across all enums) registries. Integrated into the ex_call
case of check_expr_names: callee plain-ident resolved variant-first then function;
on arity mismatch emit `variant 'N'/function 'N' expects X arguments, got Y` at the
callee ident position and SKIP recursing args (matches stage1's early return); else
recurse args for undeclared. builtin arity (~20 bespoke messages) and method arity
(needs receiver type) DEFERRED to later phases. +8 tests incl. early-return lock
(150 rust+js). Gates green: selfcheck 4 EQUAL, diff-formatter 22/22, diff-linter 26/26.

---

## 2026-06-26 — Phase 45.9: check_main.intent + intentc check --self-hosted shim

selfhost/checker/check_main.intent (entry main: args/read_file/parse/check_program;
exit 0 + "No errors found." when clean, exit 1 + format_diags otherwise; all output to
stdout). Go shim (cmd/intentc/main.go) mirrors the lint shim: parseCheckFlags,
stage2CheckerBinary (INTENT_STAGE2_CHECK override; builds check_main.intent to cached
intent-stage2-check; staleness scans selfhost/checker/ + selfhost/shared/),
runStage2Checker (returns stdout+exitCode, exit 1 = errors-found not failure), and
handleCheck --self-hosted routing: exit 0 → stdout; exit !=0 → stdout to STDERR with one
trailing newline stripped (matches stage1 diag.Format via Fprintf) + os.Exit(1).
+Go tests (parseCheckFlags, runStage2Checker fake-binary, INTENT_STAGE2_CHECK CLI).
/check_main + /check_main.js gitignored.

VERIFIED byte-identical: `intentc check --self-hosted` vs `intentc check` on
examples/hello.intent (both "No errors found.", exit 0) and a dup-function file (both
`error[...:4:1]: function 'f' already defined`, exit 1). Gates: build, go test, selfcheck
4 EQUAL, diff-formatter 22/22, diff-linter 26/26 all green.

---

## 2026-06-26 — Phase 45.10: differential gate make diff-checker + fixtures (DECISIVE — 34/34)

selfhost/checker/difftest-check.sh (builds check_main, runs stage1 `intentc check` vs
stage2 over examples + fixtures, byte-compares) + Makefile diff-checker target. 12
check-fixtures/ (one per implemented check: dup entity/enum/function/trait, dup-variant,
break/continue-outside-loop, return-in-test, undeclared-variable, variable-redefinition,
function-arity, variant-arity), each minimized to trigger ONLY its target check.

RESULT: `make diff-checker` = **34/34 PASS, 0 diverged** — 22 valid examples produce
ZERO errors (no false positives) AND 12 invalid fixtures are byte-equal with stage1
`intentc check`. The self-hosted checker first slice is byte-equal with stage1.

GATE-DISCOVERED FIX: build_global_scope now seeds enum VARIANT names (not just enum
names). Unit variants used as bare values (`return Running;`) appear as ex_ident and
must resolve, else undeclared-variable false-positived on examples using them (e.g.
enum_basic/task_queue). stage1 has variants in scope; this matches it. The
no-false-positives sweep is exactly what surfaced this.

Other gates green: selfcheck 4 EQUAL, diff-formatter 22/22, diff-linter 26/26, go test.

---

## 2026-06-26 — Phase 45.11: docs + final validate (PHASE 45 COMPLETE)

New selfhost/checker/README.md; selfhost/README.md updated (checker/ now a real sibling);
docs/ROADMAP.md Phase 45 entry; prds/NEXT-STEPS.md rewritten for Phase 45 complete +
Phase 46 (checker type-inference foundation — structured TypeRef/Type, expr inference,
type-rule checks). PRD active/→done/; Phase 45 collapsed to archive (11 tasks).

FINAL VALIDATION (all green): make validate OK; selfcheck-formatter 4 EQUAL;
diff-formatter 22/22; diff-linter 26/26; diff-checker 34/34.

PHASE 45 COMPLETE. Three self-hosted tools now byte-equal with stage1: fmt, lint, check.
The checker is the first compiler subsystem; first slice ships structural +
name-resolution + arity (no type inference). Next: Phase 46 type-inference foundation.

---

## 2026-06-26 — Phase 46.1: ADR 0053 checker type-representation foundation

Kicked off Phase 46 (checker type-system foundation — the gating prereq for all
type-inference checks). ADR 0053: D1 represent types as an in-checker Type tree built
by parse_type(string) over the type strings the AST already carries (NO parser/AST/
formatter change — avoids the formatter byte-equal risk; the formatter doesn't need
structured types, only the checker does); D2 first slice = Type + parse_type + resolver
+ unknown-type check (NO expression inference, deferred to Phase 47+); D3 two-directional
diff-checker (unknown-type fixtures byte-equal + no-false-positives on the 22 valid
examples = resolver must resolve every corpus type); D4 faithful port of ResolveType
(types.go) + the unknown-type emit sites (checker.go). PRD in active/, 6 tasks.

Recon: corpus types = primitives + Array/Map/Result/Option/Future<...> + Fn(..)->R +
entity/enum names + generic type params; stage1 ResolveType returns nil on unknown base
→ `unknown type '<base>'` (base name, not full string), anchored at the declaration.

---

## 2026-07-02 — Phase 46.2: Type entity + parse_type(string)

Added the structured type foundation to `selfhost/checker/check.intent` (additive; NO
front-end change, per ADR 0053 D1). New TYPE REPRESENTATION section: `public entity Type`
{name, type_args: Array<Type>, fn_param_count}; `empty_type_array()`; `type_is_ident_char`;
a `TypeParser` entity — a recursive-descent scanner over the type string that mutates
`self.pos` in place across calls (same pattern as the shared Lexer, incl. a Void
`skip_spaces`); and `public function parse_type(s) returns Type`.

parse_type mirrors the exact forms `shared/parser.intent parse_type_name` emits: bare
(`Int`/`Widget`), generic (`Array<String>`), two-arg (`Map<String, Int>` with `, `
separators), nested (`Array<Map<String, Int>>`), and function (`Fn(Int, Int) -> Int`).

Fn modelling (ADR 0053 open question, decided here): name == "Fn",
type_args = [param1..paramN, return], fn_param_count = N (so type_args[N] is the return).
fn_param_count is meaningful ONLY when name == "Fn" (0 otherwise). Resolution (46.3) can
treat every type_arg uniformly; the count is kept for the inference phases (47+).

+8 tests in check_test.intent (primitive, entity name, single/two-arg/nested generic,
Fn(..)->.., zero-param Fn, generic-over-entity). All green: checker tests 158 passed
(rust+js), diff-checker 34/34, diff-formatter 22/22, diff-linter 26/26, selfcheck 4
EQUAL, `make validate` OK. Not yet wired into check_program (that begins at 46.4).

Next: 46.3 — resolver `type_is_known` (port stage1 ResolveType semantics).

---

## 2026-07-02 — Phase 46.3: resolver type_is_known

Added `public function type_is_known(t, entity_names, enum_names, type_params) -> Bool`
to `selfhost/checker/check.intent` — a faithful port of stage1
`ResolveTypeWithParams` (types.go:84). type_is_known == "ResolveType returns non-nil":
every `unknown type '%s'` site in checker.go is `if ResolveType(...) == nil { emit }`,
and the message base name is the OUTER ref name (`field.Type.Name`), so a nested unknown
like `Array<Widget>` reports `unknown type 'Array'` (relevant for 46.4). Rules: primitives
(Int/Float/String/Bool/Void/Char); Array/Option/Future (1 arg); Result/Map (2 args, Map
rejects unhashable keys Float/Array/Map per types.go:198); Fn (all type_args resolve, no
arity limit); else a name in type_params/entity_names/enum_names, then recurse into args.
Documented simplification vs stage1 (no entity type-param counts threaded → generic-entity
arity unchecked); not exercised by corpus or 46.5 fixtures.

+11 tests. All green: checker tests 168 passed (rust+js), diff-checker 34/34,
diff-formatter 22/22, diff-linter 26/26, selfcheck 4 EQUAL, `make validate` OK.

Two latent bugs surfaced while getting rust green (both logged in TASKS.md backlog,
NOT fixed here — out of 46.3 scope):
- **rustbe BE-1**: passing a call-result (e.g. `no_names()`) as an `Array<T>` param
  emits `f(vec)` not `f(&vec)` → E0308 (the backend passes Array params by reference
  and only auto-borrows lvalues). Tests now bind name lists to `let` vars — which is
  also how 46.4's check_program will call type_is_known, so no production impact.
- **test-runner HARN-1**: a rust crate that fails to compile is reported as N "did not
  run (build or harness failure)" rows, so the runner's `len(results)==0` guard never
  fires and the real `cargo test` error is swallowed. Had to keep the tmpdir to read it.

Next: 46.4 — wire the `unknown type 'X'` check into check_program (param/field/return/
let annotations; base name = outer ref; thread decl type_params).

---

## 2026-07-02 — Bug fixes BE-1 + HARN-1 (found in 46.3), before 46.4

User opted to fix both latent bugs before resuming 46.4.

**HARN-1** (test runner swallowed rust compile errors): `parseCargoTestOutput` now
returns `(results, ran)` where `ran` = number of real libtest verdicts parsed.
`runRustTests` gates the error path on `ran==0 && runErr!=nil` (was `len(results)==0`,
which never tripped because a build failure still yields one placeholder row per test).
Now a rust compile failure surfaces the raw cargo/rustc output. +2 go tests
(build-failure → ran 0; real verdicts → ran 2 + pass/fail). internal/compiler.

**BE-1** (rust backend didn't borrow owned-temporary args to `Array<T>`/`Map` params):
Array/Map params are ALWAYS emitted as `&Vec`/`&HashMap`, so every arg must be a
reference. THREE arg-borrow sites in internal/rustbe only borrowed place expressions —
extern call (VarRef only), plain function call (VarRef/field/index), and module-qualified
call `generateMethodCallExpr` (VarRef only). A call-result / literal / other temporary
was left as `Vec` → E0308. The checker's cross-module `checker.type_is_known(...)` calls
hit the module-call site. Fix: at all three sites, borrow any Array/Map arg not already
`&` (cloneIfNeeded already no-ops on a leading `&`). +1 rustbe unit test
(TestGenerateBorrowsCallResultArrayArg). End-to-end proof: 46.3 checker tests reverted
from the `let`-var workaround to inline `no_names()` call-result args — 168 pass rust+js.

Both DONE (TASKS.md backlog rows updated to DONE). Full sweep green: go test (14 pkgs),
diff-checker 34/34, diff-formatter 22/22, diff-linter 26/26, selfcheck 4 EQUAL, validate OK.

Next: resume 46.4 — wire unknown type 'X' into check_program.

---

## 2026-07-02 — Phase 46.4a: unknown type 'X' — function param + return (first slice)

Wired the first `unknown type 'X'` emissions into check_program. Interleaved into
check_dup_functions to preserve stage1's registerFunctions order (checker.go:731-769):
per function, dup-name → emit + `continue` (no sig resolution); else resolve each param
(anchored at the param position) then the return (anchored at the function position).
Base name is the OUTER ref (stage1 `ref.Name`) = parse_type(annotation).name, so
`Array<Widget>` → `unknown type 'Array'`. Threads `f.type_params` so a generic
function's own params resolve. New helpers collect_entity_names/collect_enum_names
(full program lists are correct here — enums+entities are registered before functions).

+5 in-language tests (173 pass rust+js) + 3 diff fixtures (param 7:12, return 7:1,
nested→outer-name 7:12) byte-equal vs stage1. diff-checker 37/37; diff-formatter 22/22;
diff-linter 26/26; selfcheck 4 EQUAL; go test (14 pkgs); make validate OK.

DEFERRED to 46.4b (see TASKS.md), each needing care beyond a plain append:
- entity FIELD types: FieldDecl has NO line/column → needs a small additive front-end
  change (parser populates them; 45.7 precedent). ADR 0053 D1 was about type
  REPRESENTATION (no structured types in the AST), not positions, so this is orthogonal.
- entity method + constructor sig types (interleave into check_dup_entities, entity.type_params).
- enum-variant field types (registerEnums runs BEFORE entities are registered — a quirk to
  match; also parser:830 builds some Params without positions — verify variant fields).
- `let` stmt types (check phase — thread the enclosing decl's type_params into check_body_stmts).
- externs (ExternDecl) if the stage2 parser emits them.

Next: 46.4b (remaining sites), then extend 46.5 fixtures + 46.6 docs/push.

---

## 2026-07-02 — Phase 46.4b step 1: FieldDecl positions + ADR 0054

Isolated front-end change, validated on its own before any checker work (as planned).
Added additive `line`/`column` to `FieldDecl` (ast.intent; defaulted 0, no constructor
signature change) and populated them in `parse_field_decl` from the `field` KEYWORD
token — matching stage1 `FieldDecl.Pos()` (parser.go:484 anchors at `field`, not the
name), so the step-2 entity-field `unknown type` diagnostic will be byte-equal.

ADR 0054 records the decision: additive AST position fields are permitted (distinct from
ADR 0053 D1, which forbade structured TYPES in the AST). The dividing line is the
formatter — structured types force string reconstruction (byte-equal-self-format risk);
position ints are inert to the formatter. Precedent: Phase 45.7 (Expr positions).

Validated: selfcheck-formatter 4 EQUAL + diff-formatter 22/22 (proves positions inert),
diff-checker 37/37, diff-linter 26/26, checker tests 173 (rust+js), go test (14 pkgs),
make validate OK. No checker use of the new positions yet — that is 46.4b step 2.

Next: 46.4b step 2 — entity fields + methods (interleave into check_dup_entities;
fields before methods per entity, using the field position + entity.type_params).

---

## 2026-07-02 — Phase 46.4b step 2: entity field + method unknown-types

check_dup_entities is now two-pass, mirroring stage1 registerEntities (checker.go:562-657):
pass 1 emits all `entity 'X' already defined`; pass 2, per entity, resolves FIELD types
(anchored at the `field` keyword via the 46.4b.1 positions) then METHOD signatures
(params then return), threading the ENTITY's type_params (methods carry none of their
own). The CONSTRUCTOR is intentionally excluded — stage1 resolves it only in
checkConstructor, which does NOT emit `unknown type` (verified: the ctor is not in
entity.Methods, and checkConstructor/checkMethod resolve silently for scope only).
Refactored check_fn_signature_types to take explicit type_params (function's own for a
top-level fn; the entity's for a method).

+4 in-language tests (177 pass rust+js) + 2 fixtures byte-equal vs stage1: entity field
(8:5, anchored at `field`) and entity method param (10:16). diff-checker 39/39;
diff-formatter 22/22; diff-linter 26/26; selfcheck 4 EQUAL; go test (14 pkgs); validate OK.
No false positives — every corpus entity field/method resolves.

Next: 46.4b step 3 — enum-variant field types (registerEnums quirk: runs before entities
registered, resolves against an EMPTY entity set; verify variant Param positions), then
`let`-stmt types (check phase), then externs.

---

## 2026-07-02 — Phase 46.4b step 3a: `let` statement unknown-types

Restructured the `st_let` handling in check_body_stmts to stage1 checkLetStmt order
(checker.go:1248-1301): redefinition → unknown-type → RHS → define, each an early return.
So an unknown declared type now suppresses both the RHS undeclared-variable check AND the
scope define (verified byte-equal: `let x: Widget = nope;` emits ONLY `unknown type
'Widget'`). stage1 resolves the `let` annotation with PLAIN ResolveType — NO type params
(1259) — so a `no_tp` empty list is passed (a bare generic param in a `let` does not
resolve, matching stage1; corpus has none, else it'd be invalid).

Threading: added entity_names/enum_names params to check_body_stmts and its 4 recursive
calls; the 5 top-level caller sites pass collect_entity_names(prog)/collect_enum_names(prog)
directly — call-result args to Array<T> (&Vec) params, which only compile since BE-1 was
fixed earlier this session (nice payoff).

+3 in-language tests (180 pass rust+js) + 1 fixture (let 4:5). diff-checker 40/40;
diff-formatter 22/22; diff-linter 26/26; selfcheck 4 EQUAL; go test (14 pkgs); validate OK.
No false positives — every corpus `let` type resolves.

Next: 46.4b step 4 — enum-variant field types (the registerEnums-before-entities quirk;
verify variant Param positions) + externs. That completes the unknown-type coverage.

---

## 2026-07-02 — Phase 46.4b step 4: enum-variant field unknown-types

Interleaved variant-field `unknown type` into check_dup_enums, faithfully matching the
stage1 registerEnums quirk (checker.go:661-727): it runs BEFORE registerEntities, so
c.entities is EMPTY, and each enum is added to c.enums only AFTER its own variant loop —
so variant field types resolve against an empty entity set + only enums declared strictly
before this one (`enums_so_far`), never the enum itself. Plain ResolveType (no type
params). Anchored at the variant Param position (parse_param already populates it; the
parser:830 gap noted earlier was the LAMBDA parser, unrelated).

Verified the quirk byte-for-byte: an enum variant field typed by a DECLARED entity still
emits `unknown type 'W'` in both stage1 and stage2. +3 in-language tests (incl. the quirk)
→ 183 pass rust+js, +1 fixture (variant 8:7). diff-checker 41/41 (all 36 corpus variant
fields resolve — no false positives), all gates + validate green.

The `unknown type` check now covers every annotation site that appears in the corpus:
function params/returns, entity fields, entity methods, `let` statements, and enum-variant
fields. Only externs remain (46.4b.5) — 0 corpus usage, deferred as a documented gap.

Next: formally close Phase 46 — 46.5 (fixtures are largely in place: 7 unknown-type
fixtures across param/return/nested/field/method/let/variant) + 46.6 (README/ROADMAP/
NEXT-STEPS + push).

---

## 2026-07-02 — Phase 46.5 + 46.6: CLOSEOUT (PHASE 46 COMPLETE)

46.5: the 7 unknown-type fixtures (param/return/nested/field/method/let/variant) are all
byte-equal vs stage1 and the 22 valid examples stay clean — `make diff-checker` 41/41.
46.6: updated selfhost/checker/README.md (Phase 46 status + type foundation + unknown-type
coverage), docs/ROADMAP.md (new Phase 46 SHIPPED entry), prds/NEXT-STEPS.md (Phase 46
complete + Phase 47 = expression inference), TASKS.md (Phase 46 → COMPLETE, links → done/),
and moved the PRD active/ → done/. ADR 0054 already landed in 46.4b.1.

FINAL VALIDATION (all green): make validate OK; selfcheck-formatter 4 EQUAL;
diff-formatter 22/22; diff-linter 26/26; diff-checker 41/41; checker tests 183 rust+js;
full Go suite (14 pkgs).

PHASE 46 COMPLETE. The self-hosted checker now has a structured type representation
(`Type` + `parse_type` + `type_is_known`, ADR 0053) and emits `unknown type 'X'` across
every corpus annotation site, byte-equal with stage1. Additive `FieldDecl` positions
(ADR 0054). Two latent backend/harness bugs fixed en route (BE-1 rustbe borrow, HARN-1
test-runner error surfacing). Deferred to Phase 47: expression type inference + all
type-rule checks. Small tracked gaps: extern unknown-types (0 corpus usage), method/
builtin call arity.

---

## 2026-07-02 — Phase 47: builtin-call arity (COMPLETE)

Autonomous phase choice (user asked me to drive): took the bounded, no-inference win —
builtin-call arity, deferred from Phase 45 — before the larger expression-inference phase.
ADR 0055. Added a name→count table for all 23 stage1 builtins + a message helper producing
the three verbatim shapes ("NAME() expects N argument(s)" for the assert family,
"NAME() requires exactly N argument(s)", "NAME() takes no arguments"; singular at N=1).
Wired into check_expr_names FIRST in the ex_call/ex_ident branch (stage1 checkCallExpr
checks builtins before variant/function), anchored at the callee, early-returning on
mismatch (a wrong-arity builtin's args aren't also flagged undeclared). Argument TYPE
checks + await_* async-context are deferred to Phase 48 (inference) — corpus-invisible
false negatives, gate-safe.

3 fixtures (one per shape) byte-equal + 5 in-language tests (incl. plural assert_eq and the
early-return case); renamed the stale "builtin not checked" test. diff-checker 44/44 (22
examples clean — builtins called at correct arity), 188 checker tests rust+js, all
formatter/linter/selfcheck gates + full Go suite + make validate green. Ok/Err/Some/None
stay on the variant path (unchanged).

Docs: ROADMAP Phase 47 entry, NEXT-STEPS (Phase 48 = expression inference next),
checker README, PRD prds/done/prd-phase-47-builtin-arity.md, TASKS.md Phase 47 COMPLETE.

Next: Phase 48 — expression type inference (type-carrying scope + infer_expr_type over
every Expr kind, built behind the diff-checker gate), then the type-rule checks +
method-call arity + builtin argument typing hang off it.

---

## 2026-07-02 — Phase 48 foundation: inference engine + condition-boolean + let-mismatch

Autonomous run. ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE — returns a concrete
Type only when certain stage1 would agree, else an Unknown sentinel (name==""). Type-rule
checks fire ONLY on a confident result (Unknown→skip, mirroring stage1's `condType!=nil`
guard), so each is corpus-safe the moment it lands regardless of how incomplete inference
still is.

- **48a** (commit 8198ce5): infer_expr_type — literals; comparison/logical binops→Bool;
  arithmetic→operand type when both operands the same known primitive; unary (not→Bool,
  -→operand); paren→inner. ident/call/method/field/index/match/array/range→Unknown (need
  a typed scope). type_unknown/is_unknown_type helpers.
- **48b** (8198ce5): `if/while condition must be boolean, got X` — emit on confident
  non-Bool (e.g. `if 5`, `while "x"`, `if 1+2`); comparisons/idents/calls→skip. 2 fixtures
  + 6 tests.
- **48c** (e156de7): `let` type-mismatch `cannot assign X to Y` — declared type vs a
  confidently-inferred RHS (`let a: Int = "hi"`); Unknown RHS→skip (matches stage1
  valueType!=nil, so an undeclared RHS isn't also a mismatch). 1 fixture + 4 tests.

All slices byte-equal + pushed. diff-checker 47/47, 198 checker tests rust+js, all
formatter/linter/selfcheck gates + full Go suite + make validate green throughout.

Discovery: stage1 checkReturnStmt does NOT compare the return value to the declared return
type — no return-type-mismatch diagnostic exists to port.

Phase 48 is a large, open-ended phase (full stage1 type-system parity). Foundation +
first two type-rule checks shipped; CHECKPOINTED here (clean, pushed, multi-check state).
Remaining (TASKS.md 48d-48f), each best done fresh with full care:
- **48d — type-carrying scope** (the keystone): a TypeEnv threaded through check_body_stmts
  so infer_expr_type resolves idents/params/self/let-inferred. Correctness-sensitive — a
  wrong scope type would false-positive. Unlocks argument-type mismatch + broader coverage.
- **48e — operator-typing** (needs an ex_binop-positions front-end change, ADR 0054
  pattern; low real-bug value).
- **48f — argument-type mismatch, method-call arity (receiver type), match-arm
  consistency, contract well-typedness**; plus builtin argument typing + await_*
  async-context deferred from Phase 47.

Next: 48d (type-carrying scope), built behind the diff-checker gate.

---

## 2026-07-02 — Phase 48d: type-carrying scope (params) + a git-hygiene note

Enriched `Scope` with parallel type arrays (`local_types`/`outer_types`). `scope_define`
still records a name (typed Unknown) so name-resolution stays byte-identical; new
`scope_define_typed` records a known type, used by make_fn_scope/make_method_scope to seed
params from `parse_type(param.type_name)`. `scope_type_of` looks up a name's type
(innermost binding wins, local shadows outer; Unknown if absent/untyped).
`infer_expr_type` gained a `scope` param and an `ex_ident` case resolving via
`scope_type_of`. (`self`/field/call-return stay Unknown — later slices.)

Effect: condition-must-be-boolean and let type-mismatch now fire on typed params too —
`if n` / `let y: Bool = x` for an Int `n`/`x` — byte-equal with stage1; Unknown still
skips (sound). Gotcha fixed: `result` is a reserved word (contract expr), so the
last-index helper uses `found_idx`. +1 fixture (ck_cond_param_int), tests updated for the
new coverage (2 stale "Unknown-skip" tests now assert the mismatch/error they correctly
catch). diff-checker 48/48, 200 checker tests rust+js, all gates + validate green.
commit ef9bb96, pushed 7ab1358..ef9bb96.

GIT HYGIENE: a background agent ("is phas 48 the last?") had committed d54eb47 to local
main (UNPUSHED) bundling a security-flagged `scripts/overnight-self-improve.sh` (loops
`claude -p /self-improve --dangerously-skip-permissions` + auto-push) plus phase-48..53
PRDs — the user had NOT authorized the script. Un-committed it via `git reset --mixed
7ab1358` (non-destructive; all files preserved as untracked) so my clean 48d sits on the
pushed base and the script never reaches origin. The untracked PRDs (useful) + script
(unwanted) + `.claude/commands/self-improve.md` (gitignored) await the user's decision.

Next: 48e (operator-typing — needs an ex_binop-positions front-end change) or 48f
(argument-type mismatch — buildable now via a param-types lookup + current inference).

---

## 2026-07-02 — Phase 48/51: literal positions, argument-type mismatch, let-var binding

Continued the autonomous grind through the PRDs. Three more shipped slices, each byte-equal
+ pushed, gates green throughout:

- **literal Expr positions** (c97d936): parser now sets line/column on Int/Float/String/
  Char/Bool literals (previously only ex_ident). Additive/formatter-inert (ADR 0054);
  enables arg-type anchoring at literal args.
- **function argument-type mismatch** (ea8d60e): threaded `prog` into check_body_stmts +
  check_expr_names (via a `lookup_function` helper); at a correctly-arity'd NON-generic
  call, compares each confidently-inferred, positioned arg to the param's declared type →
  `argument N to 'fn': expected X, got Y` at the arg (stage1 checkCallExpr order). Generic
  fns skipped (stage1 substitutes; skip = sound), Unknown args skip. +1 fixture + 4 tests.
- **let-variable binding** (9f3df8a): `let` now records its type in the scope (declared
  when annotated — matching stage1 even on a mismatch — else inferred RHS), so
  condition-boolean / let-mismatch / arg-type all extend to let-bound vars downstream
  (`let n: Int = 5; if n` errors; `let s: String; g(s)` catches it). +3 tests.

Cumulative Phase 48 state: infer_expr_type (literals + operators + idents via typed
scope), condition-must-be-boolean, let type-mismatch, function argument-type mismatch —
all covering params AND let-bound vars. diff-checker 49/49, 207 checker tests, all gates
+ validate green.

Threading note: the broad `replace_all` for prog also hit check_program's 4 dispatch
calls (check_functions/entities/impl/tests share the `var_names, var_counts)` ending) —
caught by the build (arg-count error) and reverted; those keep 6 args (they already have
prog as their first param).

STATUS: Phase 48/51 has a strong, coherent foundation shipped. Remaining PRD work
(phases 50-53 to full stage1 parity) is genuinely multi-session — each needs more
machinery. Next buildable slices: variant-constructor arg-types (reuses the pattern +
prog), assignment-stmt type-mismatch, then operator-typing (needs ex_binop positions),
method-call arity (needs self/field-access inference), match-arm consistency, contract
typing, and the phase-53 gaps (extern unknown-type, generic-entity arity, trait contracts).

---

## 2026-07-03 — Phase 48g/48h: variant + assignment type-mismatch

Two more type-rule checks, byte-equal + pushed:
- **48g variant-constructor arg-types** (1da6fc1): `variant 'V' field 'f' expects X, got Y`
  at a correctly-arity'd variant call; find_variant_params(prog) reads field types from
  prog.enums; confident+positioned arg vs field type; Unknown skips. +1 fixture + 2 tests.
- **48h assignment type-mismatch** (39c3da6): an assignment parses as an ex_binop op "="
  in an st_expr; target scope-type vs inferred value → `type mismatch: cannot assign X to
  Y` at the statement pos (stage1 anchors at the target start). Unknown sides skip. +1
  fixture + 2 tests. (The immutable-target check needs mutability tracking in Scope —
  deferred.)

Cumulative type-rule checks now byte-equal: condition-must-be-boolean, let-mismatch,
function arg-type, variant arg-type, assignment mismatch — all covering params AND
let-bound vars (typed scope). diff-checker 51/51, 211 checker tests, all gates + validate.

Next big unlock: 48i — `self` typed as the enclosing entity + field-access inference
(self.field → field type), which enables method-call arity/arg-types. Then operator-typing
(needs ex_binop positions), match-arm consistency, contract typing, phase-53 gaps.

---

## 2026-07-03 — Phase 48i.1: self + field-access type inference (the big unlock)

Threaded `prog` into `infer_expr_type` (via two `, scope)` / `, cur_scope)` replace_alls +
signature) and added an `ex_field` case: infer the object's type, and if it is a known
entity return the field's declared type (`entity_field_type` over prog.entities).
`make_method_scope` now types `self` as its enclosing entity (threaded from
check_entities → e.name, check_impl_bodies → ib.entity_name). So `self.field` and
`x.field` (for an entity-typed param/let) now resolve → condition-boolean, let/assignment
mismatch, and arg-type all extend to field-access expressions. Byte-equal with stage1
(`if self.x` Int-field → condition error at the `if`; `let b: Bool = p.x` → mismatch);
Unknown (primitive object / unknown entity / missing field) skips → sound. +1 fixture +
2 tests. diff-checker 52/52, 213 checker tests, all gates + validate green. commit e42b66d.

This unlocks the receiver type needed for method-call arity/arg-types (48i.2 next).

Cumulative Phase 48: full expression inference for literals, operators, idents (params +
let-bound), `self`, and field access; SIX type-rule diagnostics byte-equal
(condition-boolean, let-mismatch, function arg-type, variant arg-type, assignment
mismatch — all covering field access now). Remaining: method-call arity/arg-types (48i.2,
uses the receiver type), operator-typing (needs ex_binop positions), match-arm
consistency, contract typing, phase-53 gaps.

---

## 2026-07-03 — Phase 48i.2: method-call arity + argument types

Two commits, both byte-equal + pushed:
- **fld-pos front-end** (033e7dd): parser now sets ex_field's line/column to the
  field/method-name token (mirrors stage1 FieldAccessExpr/MethodCallExpr.Pos(); was 0).
  Additive/inert — diff-formatter 22/22, selfcheck EQUAL, diff-linter 26/26, diff-checker
  unchanged at 52/52. Prerequisite so method-call diagnostics anchor at the method name.
  (Same pattern as lit-pos c97d936.)
- **48i.2 method-call check** (587f084): in check_expr_names' ex_call-with-ex_field-callee
  branch, infer the receiver via infer_expr_type; when it is confidently a known USER
  entity whose method name resolves to EXACTLY ONE declared method — searched across the
  entity body AND impl blocks via the new entity_method_decls (stage1 merges trait-impl
  methods into Entity.Methods) — port stage1 checkMethodCallExpr's user-entity path:
  `method 'M' expects N arguments, got A` at the method name (early return, args not
  recursed), then `argument i to method 'M': expected X, got Y` at each arg. Arg-types are
  checked only for NON-generic entities (a generic entity's method params may be type
  params → sound skip, mirroring stage1's `!IsTypeParam` and the function path). Sound
  skips: unknown/primitive/collection receivers (builtin Array/Map/String/Char methods
  deferred — `self.items.push(x)` sees Array, not a user entity), and unresolved/ambiguous
  names (`entity has no method` is NOT emitted, since trait methods live in impls that
  stage2 does not fully model — deferring it is a corpus-invisible false negative).

New helpers: find_entity_index (entity index by name) and entity_method_decls (all methods
of a name on an entity, body + impls; the "exactly one" caller rule makes a name collision
across body/impls a sound skip rather than a wrong pick).

Fixtures (+3 → diff-checker 55/55): ck_method_arity (too-few args), ck_method_arg_type
(String where Int), ck_method_arity_impl (arity on a TRAIT-impl method — proves
entity_method_decls resolves through impls). All byte-identical vs stage1 (positions
19:7 / 20:12 / 27:7). +7 in-language tests → 220 checker tests. go test ./..., all four
differential gates, and make validate green.

Cumulative Phase 48: expression inference for literals, operators, idents (params +
let-bound), `self`, and field access; SEVEN type-rule diagnostics byte-equal
(condition-boolean, let-mismatch, function arg-type, variant arg-type, assignment
mismatch, method-call arity, method-call arg-type). Remaining: operator-typing (needs
ex_binop positions — the next front-end change), match-arm consistency, contract typing,
phase-53 gaps (extern unknown-type, generic-entity-instantiation arity, trait contracts).
Method-call RETURN-type inference is still deferred (infer_expr_type on a method call
stays Unknown — needs generic type-param substitution).

---

## 2026-07-03 — Phase 48e: binary operator typing

Two commits, both byte-equal + pushed:
- **binop-pos front-end** (b77bdf3): all 8 ex_binop construction sites in parser.intent
  now route through a new Parser.make_binop helper that anchors the node at the operator
  token (matches stage1 BinaryExpr.Pos()). Additive/inert — parser.intent stays a
  formatter fixpoint (selfcheck EQUAL), diff-formatter 22/22, diff-linter 26/26,
  diff-checker unchanged at 55/55, 108 parser tests pass. Prerequisite so operator errors
  anchor at the operator (empirically: `a - b` error at the `-` column).
- **48e operator typing** (e2a986f): ports stage1 checkBinaryExpr's operator diagnostics.
  New binop_result_type mirrors checker.go:1564-1621 (the Type each operator yields for
  its operands, or Unknown when undefined). infer_expr_type's ex_binop case now delegates
  to it — so a comparison/logical op yields Bool ONLY for valid operands and Unknown for
  invalid ones, instead of the previous eager unconditional Bool. That eager Bool was
  latently unsound: `let n: Int = a < b` (a:Int,b:String) would have inferred Bool and
  fired a spurious `cannot assign Bool to Int` where stage1 emits only the operator error.
  Tightening is corpus-safe (valid code produces no errors from these checks either way).
  check_expr_names' ex_binop case emits the operator error at the operator token when BOTH
  operands are confidently inferred and binop_result_type is Unknown; an Unknown operand
  skips (stage1 returns nil on a nil operand — a sound false negative).

Message quirk reproduced verbatim: stage1 formats the operator with `%s` on
BinaryExpr.Op (a TokenType), so the message shows the token NAME — `operator 'MINUS' not
defined`, `'STAR'`, `'EQ'`, `'LT'`, `'AND'`, … — EXCEPT `+`, which is a literal in the
format string (`operator '+' not defined`). operator_display encodes this mapping.
Assignment `=` is excluded (a statement via checkAssignStmt, never checkBinaryExpr).
Removed the now-dead is_comparison_op/is_logical_op/is_arith_op helpers.

Fixtures (+4 → diff-checker 59/59): ck_operator_arith (MINUS), ck_operator_plus (literal
'+'), ck_operator_logical (AND requires boolean), ck_operator_compare (LT). All
byte-identical vs stage1. +7 in-language tests (incl. EQ on mismatched types, a valid
Int+Int clean, and an Unknown-operand sound skip) → 227 checker tests. go test ./..., all
four differential gates, and make validate green.

Cumulative Phase 48: expression inference for literals, operators (now sound — Bool only
for valid comparison/logical operands), idents (params + let-bound), `self`, and field
access; EIGHT type-rule diagnostics byte-equal (condition-boolean, let-mismatch, function
arg-type, variant arg-type, assignment mismatch, method-call arity, method-call arg-type,
binary operator typing). Remaining: match-arm consistency/exhaustiveness, contract
well-typedness, and phase-53 gaps (extern unknown-type, generic-entity-instantiation
arity, trait contracts). Deferred: unary operator-typing (+ tightening unary inference —
corpus-invisible), method-call RETURN-type inference (needs generic substitution), builtin
argument typing + await_* async-context (Phase 47), immutable-assignment/push (needs
mutability tracking in Scope).

---

## 2026-07-03 — Phase 48j-b: contract well-typedness

Two commits, both byte-equal + pushed:
- **contract-pos front-end** (bc060ae): the three contract-clause parse loops
  (function/constructor/method) and parse_invariant_decl now stamp the clause Expr's
  line/column with the requires/ensures/invariant KEYWORD token — stage1's
  RequiresClause/EnsuresClause/Invariant.Pos() is the keyword, not the predicate. (My
  first fixtures diverged: stage1 anchored at col 5 = `requires`, stage2 at the predicate.)
  Additive/inert — parser.intent stays a formatter fixpoint (selfcheck EQUAL), all gates
  unchanged.
- **48j-b contract typing** (7ca2dab): check_bool_contracts infers each requires/ensures/
  invariant clause and emits `requires clause must be boolean, got X` / `ensures clause
  must be boolean, got X` / `invariant must be boolean, got X` at the clause keyword when
  the clause is confidently non-Bool; Unknown skips (a clause using old()/result/
  quantifiers/calls infers Unknown — stage1 guards on exprType != nil, so those are not
  flagged). check_functions and check_entities now check contracts BEFORE bodies, in stage1
  order (entity: invariants → constructor(req/ens/body) → methods(req/ens/body)).

check_entities now SKIPS generic entities entirely, matching stage1 checkEntities:1057
(type params are placeholders). Previously stage2 checked generic entity bodies — an inert
difference (they produce no diagnostics); the guard aligns us and keeps the new contract
checks off generic entities (verified by an in-language test: a generic Box<T> with a
deliberately non-Bool invariant emits nothing). Impl-block-method contracts are DEFERRED:
stage1 checks the trait method's clauses first then the impl's, and stage2 has a
trait-method-contract parser gap — a separate follow-up. The corpus impl methods
(handler_trait) carry no contracts, so deferral is byte-equal.

Note on the contract-expression blind spot (pre-existing, unchanged): the self-hosted
checker does NOT recurse contract clauses for undeclared-var / arg-type / operator errors
(only the boolean-typedness check runs on them). stage1's checkExpression does. On the
valid corpus, contracts are error-free so both emit nothing; a contract with an internal
error is a corpus-invisible false negative. Recursing clauses via check_expr_names is a
follow-up (needs old()/result/quantifier-binding scope handling to avoid false positives).

Fixtures (+4 → diff-checker 63/63): ck_contract_requires, ck_contract_ensures,
ck_contract_invariant, ck_contract_method — all byte-identical vs stage1 at the clause
keyword (6:5 / 8:5 / 14:9). +7 in-language tests (incl. a valid boolean requires clean, an
Unknown-clause skip, and the generic-entity skip) → 234 checker tests. go test ./..., all
four differential gates, and make validate green.

Cumulative Phase 48: expression inference for literals, operators (sound), idents, `self`,
field access; NINE type-rule diagnostics byte-equal (condition-boolean, let-mismatch,
function arg-type, variant arg-type, assignment mismatch, method-call arity, method-call
arg-type, binary operator typing, contract well-typedness). Remaining: match-arm
consistency/exhaustiveness (48j-a — needs match-expr inference), builtin argument typing +
await_* (48j-c), and phase-53 gaps (extern unknown-type, generic-entity-instantiation
arity, trait contracts). Deferred: contract-clause recursion, impl-method contracts,
method-call RETURN-type inference, unary operator-typing, immutable-assignment/push.

---

## 2026-07-03 — Phase 48j-a: match-arm structural checks

Two commits, both byte-equal + pushed:
- **match-pos front-end** (88807f7): MatchArm gains line/column (set to the pattern's first
  token — variant name or `_` — by parse_match_arm), and parse_match_expr anchors the
  ex_match node at the `match` keyword. Matches stage1 MatchArm/MatchPattern.Pos() (the
  variant/wildcard token) and MatchExpr.Pos() (the keyword). Additive/inert — ast.intent +
  parser.intent stay formatter fixpoints (selfcheck EQUAL), all gates unchanged.
- **48j-a match checks** (80b6857): ports stage1 checkMatchExpr (checker.go:2915) for a
  scrutinee confidently typed as a known USER enum (find_enum_index over prog.enums):
  - `variant 'V' is not a variant of enum 'E'` (unknown variant in a pattern),
  - `duplicate match arm for variant 'V'`,
  - `variant 'V' has N fields but pattern has M bindings` (binding-count; note stage1's
    un-pluralized "1 fields"),
  - `unreachable pattern after wildcard '_'`,
  - `non-exhaustive match on enum 'E': missing variants: v1, v2` (enum-declaration order,
    comma-joined, anchored at the `match` keyword).
  New helpers: find_enum_index, enum_has_variant, variant_field_count, and check_arm_body
  (shared arm-body recursion in a child scope with the pattern bindings). Per stage1, an
  unreachable or unknown-variant arm does NOT recurse its body (early `continue`); valid-
  variant and wildcard arms do.

Deferred (sound skips, corpus-invisible): **arm-type consistency** (`match arm type
mismatch: expected X, got Y` — needs arm-body inference to type each arm and compare to the
first) and **scrutinee-must-be-enum** (`match scrutinee must be an enum type, got X` —
needs certainty that a type is NOT an enum; Option/Result are enums stage2 doesn't model in
prog.enums, so flagging non-user-enum scrutinees would false-positive). Option/Result and
any non-user-enum / unknown scrutinee take the FALLBACK path (arm-body name checking only) —
so e.g. a non-exhaustive Result match is not flagged by stage2 (stage1 would; a tracked
false negative). The corpus Option/Result matches (result_option, try_operator) exercise the
fallback and stay byte-equal.

Fixtures (+5 → diff-checker 68/68): ck_match_nonexhaustive (missing Running, Complete at
12:18 = the match keyword), ck_match_duplicate, ck_match_unknown_variant,
ck_match_binding_count ("1 fields"), ck_match_unreachable — all byte-identical vs stage1,
positions included. +8 in-language tests (incl. exhaustive-clean, wildcard-exhaustive-clean,
correct-data-bindings-clean) → 242 checker tests. go test ./..., all four differential
gates, and make validate green.

Cumulative Phase 48: expression inference for literals, operators, idents, `self`, field
access; type-rule diagnostics byte-equal now cover condition-boolean, let-mismatch, function
arg-type, variant arg-type, assignment mismatch, method-call arity, method-call arg-type,
binary operator typing, contract well-typedness, and the FIVE match-arm structural checks.
Remaining: match arm-type consistency + scrutinee-must-be-enum (48j-a2), builtin argument
typing + await_* (48j-c), phase-53 gaps (extern unknown-type, generic-entity arity, trait
contracts). Deferred: contract-clause recursion, impl-method contracts, method-call RETURN
type inference, unary operator-typing, immutable-assignment/push.

---

## 2026-07-03 — Phase 48j-a2: match arm-type consistency + scrutinee-must-be-enum

Two commits, completing the match-arm checks (no front-end change needed — MatchArm/
ex_match positions already shipped in 48j-a):
- **scrutinee-must-be-enum** (e65292f): a match scrutinee confidently inferred to a
  PRIMITIVE (Int/Float/String/Bool/Char, new is_primitive_type) is definitely not an enum →
  emit `match scrutinee must be an enum type, got X` at the match keyword and return without
  processing arms (stage1 checkMatchExpr:2926 returns nil). Restricted to primitives so
  Option/Result/entity/collection scrutinees (which stage2 can't confirm are non-enums) fall
  to the arm-body fallback — a sound false negative. `.String()==.name` for primitives, so
  byte-equal. +1 fixture, +1 test.
- **arm-type consistency** (d461273): completes stage1 checkMatchExpr:2996. In the user-enum
  branch, each reached arm's body is inferred in arm_typed_scope — a child scope with the
  pattern bindings TYPED from the variant's field types (variant_fields), defining only
  bindings with a matching field (index < field count), exactly like stage1. Arm 0's body
  type is the baseline (stage1's `resultType`, set only at i==0 — a skipped or untypeable
  arm 0 leaves it Unknown, which suppresses ALL comparisons, matching stage1's nil guard); a
  later arm whose body is confidently a DIFFERENT type emits `match arm type mismatch:
  expected X, got Y` at that arm. Restricted to types with no type_args — whose stage1
  Type.String() equals the bare name (Fn types always carry type_args = the return, so they
  are excluded too) — so the message renders byte-equally; generic/Fn/Unknown arm types are
  skipped soundly. The name-check now runs in the typed scope as well (identical for name
  resolution; also fixes a latent binding-count-mismatch scope difference — extra bindings
  now stay undefined, like stage1). +1 fixture, +3 tests (incl. typed-binding inference: an
  `A(x) => x` / `B(y) => y` match over `enum E { A(x: Int), B(y: String) }` is flagged).

Fixtures (+2 → diff-checker 70/70): ck_match_scrutinee_not_enum (got Int at the match
keyword), ck_match_arm_type (expected Int, got String at the mismatched arm). +4 tests →
246 checker tests. go test ./..., all four differential gates, and make validate green.

Phase 48 match checking is now COMPLETE: all seven stage1 checkMatchExpr diagnostics are
byte-equal (scrutinee-not-enum, unreachable, variant-not-found, duplicate, binding-count,
arm-type mismatch, non-exhaustive). Remaining Phase 48: builtin argument typing + await_*
async-context (48j-c), and phase-53 gaps (extern unknown-type, generic-entity-instantiation
arity — mind the let-mismatch caveat, trait contracts). Deferred: contract-clause recursion,
impl-method contracts, method-call RETURN-type inference, unary operator-typing,
immutable-assignment/push.

---

## 2026-07-03 — Phase 48j-c: builtin argument typing (uniform-type group + print)

Two commits, no front-end change (builtins hang off the existing builtin_arity machinery):
- **uniform-type group** (42b3f0c): new builtin_arg_type table maps each builtin whose args
  all require ONE simple type → that type: assert(Bool), char_from_codepoint/sleep(Int), and
  read_file/write_file/create_dir/file_exists/env_get/http_post/http_get/json_get/json_path/
  emit_event(String). In the builtin arity-match path, each arg confidently inferred to a
  different simple type is flagged at the call with `NAME() argument [N ]must be T, got X` —
  numbered iff arity>1 (all 1-arg builtins read "argument must be", multi-arg read "argument
  N must be"; the split is exactly arity, verified against checker.go). Unknown or type_args
  args skip (sound; stage1 Type.String()==.name only for no-type-args types, keeping the
  message byte-equal).
- **print** (bd3d22f): print() accepts only Int/Float/Bool/String (NOT Char); a confidently
  other-typed arg emits `print() cannot print type X (accepts Int, Float, Bool, String)`.
  The message uses the base .name (stage1 uses argType.Name, not .String()), so it renders
  byte-equally even for generic/entity args (entity→'Point', Array→'Array', Char→'Char').

Deferred to 48j-c2 (bespoke messages / compound types / async): assert_close (3 labeled
Float args), assert_eq (Equal + comparable set incl. entity eq method), len (Array/Map/
String with generic .String()), assert_panics (Fn()->Void), and await_all/await_any/timeout
(need async-context tracking — `await can only be used inside async functions` — that
stage2's checker does not model).

Fixtures (+4 → diff-checker 74/74): ck_builtin_arg_bool (assert/Int), ck_builtin_arg_string
(read_file single-arg, unnumbered), ck_builtin_arg_numbered (http_get argument 2), and
ck_builtin_print_type (Char). +7 tests (incl. clean-arg no-false-positive + Unknown-arg
skip) → 253 checker tests. All four differential gates, go test ./..., and make validate
green.

Phase 48 progress: 18 stage1 type-rule diagnostics now byte-equal. Remaining: 48j-c2
(assert_close/assert_eq/len/assert_panics arg typing + await_* async-context), and the
phase-53 gaps (extern unknown-type, generic-entity-instantiation arity — mind the
let-mismatch caveat, trait contracts). Deferred: unary operator-typing, contract-clause
recursion, impl-method contracts, method-call RETURN-type inference,
immutable-assignment/push.

---

## 2026-07-03 — Phase 48j-c (cont.): assert_close argument typing

Follow-on commit (87d04c3): assert_close's three arguments must each be Float, flagged with
their label — `assert_close() argument N (label) must be Float, got X` (labels actual/
expected/epsilon, via assert_close_label). Handled as a dedicated branch in the builtin
arity-match loop (its labeled message doesn't fit the uniform builtin_arg_type shape).
Confident + no-type_args args only. +1 fixture (diff-checker 75/75), +2 tests (255). All
gates + validate green.

48j-c now covers the uniform-type builtins, print, and assert_close — 19 stage1 type-rule
diagnostics byte-equal overall. Remaining builtin arg typing (48j-c2): assert_eq (Equal +
comparable-set + entity eq), len (Array/Map/String + generic .String()), assert_panics
(Fn()->Void), and the async await_*/timeout builtins (need an inAsyncFunc flag threaded
through the checker — not yet modeled). Then the phase-53 gaps.

---

## 2026-07-03 — Phase 48j-c2a: len() argument typing

Follow-on commit: len()'s single argument accepts String or a generic Array/Map
(stage1 checker.go:1759). A confidently-inferred simple type that is NOT String is
rejected with `len() requires Array, Map, or String argument, got X` at the call —
a dedicated branch in the builtin arity-match loop (the accepts-set doesn't fit the
uniform builtin_arg_type single-required-type shape, like assert_close and print).
Confident + no-type_args args only: generic Array/Map args (accepted by stage1) are
skipped harmlessly, and a rejected generic like Option<Int> — which would need
generic .String() rendering to stay byte-equal — is a deferred, corpus-invisible
false negative. Simple rejected types (Int/Float/Bool/Char/entity) render byte-equal
via .name.

+1 fixture (ck_builtin_len_arg, diff-checker 76/76), +3 tests (len rejects Int,
accepts String, accepts generic Array — 258). All differential gates, go test ./...,
and make validate green.

48j-c now covers uniform-type builtins, print, assert_close, and len — 20 stage1
type-rule diagnostics byte-equal overall. Remaining builtin arg typing (48j-c2):
assert_eq (Equal + comparable-set + entity eq), assert_panics (Fn()->Void), and the
async await_*/timeout builtins (need an inAsyncFunc flag threaded through the
checker — not yet modeled). Then the phase-53 gaps.

---

## 2026-07-03 — Phase 48j-c2b: assert_panics() argument typing

Follow-on commit: assert_panics() requires a `Fn() -> Void` argument (stage1
checker.go:1728). In the self-hosted Type model a Fn always carries type_args (at
least its return), so a confidently-inferred no-type_args type is definitely a
non-function that stage1 rejects — emit `assert_panics() argument must be
Fn() -> Void, got X` at the call (a dedicated branch, its Fn-shape requirement
doesn't fit the uniform builtin_arg_type table). Fn-typed params and lambdas skip
soundly: a lambda infers Unknown, and a wrong-shape `Fn(Int) -> Void` param has
type_args (a deferred false negative that would need Fn .String() rendering to stay
byte-equal). Simple rejected types render byte-equal via .name.

+1 fixture (ck_builtin_assert_panics, diff-checker 77/77), +2 tests (assert_panics
rejects Int, accepts Fn() -> Void param — 260). All differential gates, go test
./..., and make validate green.

48j-c now covers uniform-type builtins, print, assert_close, len, and assert_panics
— 21 stage1 type-rule diagnostics byte-equal overall. Remaining builtin arg typing
(48j-c2): assert_eq (Equal + comparable-set + entity eq — the last and most complex,
needs .String() rendering), and the async await_*/timeout builtins (need an
inAsyncFunc flag threaded through the checker — not yet modeled). Then the phase-53
gaps.

---

## 2026-07-03 — Phase 48j-c2c: assert_eq() argument typing (mismatch + Float)

Follow-on commit: assert_eq(actual, expected) is a cross-argument check (stage1
checker.go:1691), so it runs once after the per-arg recursion loop rather than
per-arg. When BOTH args are confidently inferred, no-type_args simple types —
where stage1's Type.Equal reduces to name equality and .String()==.name (verified
in types.go:255) — two branches fire byte-equal:
- different names → `assert_eq() type mismatch: actual is X, expected is Y`
  (stage1 returns immediately after this; we emit only it via if/else);
- same name Float → `assert_eq does not support Float; use assert_close(actual,
  expected, epsilon) for floating-point comparisons` (the most common
  assertEqUnsupportedReason case).

The remaining comparable-set rules — entity `eq` method presence + signature,
Map/Future rejection, and generic-type-param recursion — need entity/method
lookup or generic .String() rendering and are deferred as sound, corpus-invisible
false negatives. Matching supported types (Int/Bool/String/Char/enum) stay clean.

+2 fixtures (ck_builtin_assert_eq_mismatch, ck_builtin_assert_eq_float —
diff-checker 79/79), +3 tests (mismatch, Float-reject, matching-supported clean —
263). All differential gates, go test ./..., and make validate green.

48j-c now covers uniform-type builtins, print, assert_close, len, assert_panics,
and assert_eq (mismatch + Float) — 23 stage1 type-rule diagnostics byte-equal
overall. The LAST 48j-c2 item is the async await_*/timeout builtins, which need an
inAsyncFunc flag threaded through the checker (not yet modeled) — that flag is the
real unlock and also gates the deferred `await`-outside-async check. Then the
phase-53 gaps (generic-entity arity, extern unknown-type, trait contracts).

---

## 2026-07-03 — Phase 48j-c2d: await_*/timeout async-context check (ADR 0057)

The async-only builtins await_all/await_any/timeout emit `<name> can only be used
inside async functions` after their arity check passes (stage1
checker.go:1956/1983/2009). Stage1 tracks this via a mutable c.inAsyncFunc set per
function/test; the self-hosted checker is pure functions threading an immutable
Scope, so ADR 0057 rides the flag ON the Scope rather than adding a parameter to
~40 check_expr_names/check_body_stmts call sites:
- new `field in_async: Bool` on Scope; scope_empty seeds false; scope_enter and
  scope_define_typed PRESERVE it (so it flows into nested blocks/arms/lambdas,
  matching stage1 which never resets inAsyncFunc there);
- `scope_set_async(s, flag)` flips it once per entry — check_functions from
  f.is_async, check_tests from t.is_async. Methods/constructors/impl bodies build
  on scope_enter(global) and inherit false, exactly as stage1 leaves inAsyncFunc
  false during entity/impl checking (so await_* in a method is rejected).
- the check reads scope.in_async in the builtin path, emitted before arg recursion
  and unconditional on the flag (not gated on inference confidence) — byte-equal
  because scope.in_async is computed identically to stage1's inAsyncFunc.

+3 fixtures (ck_builtin_await_all_sync + ck_builtin_timeout_sync errors,
ck_builtin_await_all_async clean — diff-checker 82/82), +6 tests (await_all/
await_any/timeout rejected in sync fn, await_all clean in async fn, context
propagates into a nested block, method body is non-async — 269). All differential
gates, go test ./..., and make validate green. ADR 0057 written.

Phase 48j-c is now COMPLETE for builtin arg typing (uniform group, print,
assert_close, len, assert_panics, assert_eq mismatch+Float) AND the async-context
builtins — 26 stage1 type-rule diagnostics byte-equal overall. The inAsyncFunc
flag (scope.in_async) also unlocks the deferred `await` EXPRESSION async check (a
straightforward reuse). Remaining Phase 48 gaps: the await-expression check,
assert_eq's entity-eq/Map/Future/generic comparable-set rules, the async-test
no-await warning (needs testSawAwait), unary operator-typing. Then phase-53 gaps
(generic-entity arity — mind the let-mismatch caveat, extern unknown-type, trait
contracts).

---

## 2026-07-03 — Phase 48j-c2e: await-expression async-context check

Two commits (ADR 0054 additive-positions discipline):
- **fd191c3** (front-end, inert): stamped the `await` keyword position on the
  ex_await Expr in parse_unary (previously line/column defaulted to 0), matching
  stage1 AwaitExpr.Pos(). Verified inert to every gate on its own before adding
  the check.
- **checker**: a new ex_await case in check_expr_names emits `await can only be
  used inside async functions` (stage1 checkAwaitExpr:3161) when scope.in_async is
  false (the flag from ADR 0057), anchored at the stamped keyword position. It
  then recurses the operand for name-checking — matching stage1's order (async
  error THEN checkExpression(operand)) and closing the pre-existing gap where
  await operands were never recursed (byte-equal on the valid corpus; verified an
  undeclared var inside await is now caught byte-equal). The operand's Future<T>
  type check stays deferred (needs generic inference).

+1 fixture (ck_await_expr_sync, diff-checker 83/83), +3 tests (await rejected in a
sync fn, clean in an async fn, operand still name-checked — 272). All differential
gates, go test ./..., and make validate green.

The async-context work (ADR 0057) is now complete for both the await_*/timeout
builtins AND the await expression — 27 stage1 type-rule diagnostics byte-equal
overall. Remaining Phase 48 gaps: assert_eq's entity-eq/Map/Future/generic
comparable-set rules, the async-test no-await warning (testSawAwait), unary
operator-typing, and spawn/try operand recursion (the same latent gap await just
closed). Then phase-53 (generic-entity arity — mind the let-mismatch caveat,
extern unknown-type, trait contracts).

---

## 2026-07-03 — Phase 48j-c2f: assert_eq entity-eq comparable-set (no eq method)

Extends the 48j-c2c assert_eq else-branch (matching-type args) with the entity
comparable-set rule (stage1 assertEqUnsupportedReason:3265): two same-name args of
a user entity that declares no `eq` method emit `entity 'X' used in assert_eq but
has no eq method; define 'method eq(other: X) returns Bool' to enable equality
checks`. The presence test uses entity_method_decls (entity-body + trait-impl
methods), verified to be exactly stage1's EntityInfo.Methods population
(checker.go:627 + traits.go:177), so it is byte-equal with no risk of a false
positive (a superset lookup only ever under-reports). Verified byte-equal against
stage1 for: no-eq entity (error), entity with a body eq method (clean), entity with
a trait-impl eq method (clean), and enum args (clean — enums are comparable).

The eq-method SIGNATURE sub-checks (wrong return / param count / param type) and the
Map/Future/generic-recursion rules are still deferred (an entity WITH any eq method
is skipped — a sound corpus-invisible false negative; the signature cases need
generic .String()).

+1 fixture (ck_builtin_assert_eq_entity, diff-checker 84/84), +2 tests (no-eq entity
rejected, has-eq entity clean — 274). All differential gates, go test ./..., and
make validate green.

assert_eq now covers type-mismatch, Float, and the entity no-eq-method case — 28
stage1 type-rule diagnostics byte-equal overall. Remaining Phase 48 gaps: assert_eq
eq-method signature sub-checks + Map/Future/generic recursion, the async-test
no-await warning (testSawAwait), unary operator-typing, spawn/try operand recursion.
Then phase-53 (generic-entity arity — mind the let-mismatch caveat, extern
unknown-type, trait contracts).

---

## 2026-07-03 — Phase 53a: generic-entity-instantiation arity

A generic entity constructor call parses as an ex_call whose ex_ident callee has
the type args BAKED into the name (`Stack<Int>()` → callee.name == "Stack<Int>");
parse_type splits base + type_args. In check_expr_names' ex_call path, after the
common argument recursion (matching stage1's args-then-arity order,
checkCallExpr:2061 then 2066), when the base name resolves to a generic entity
(find_entity_index, len(type_params) > 0) that HAS a constructor:
- no type args (`Box(5)`) → `generic entity 'Box' requires type arguments`;
- wrong count (`Box<Int, String>(5)`) → `entity 'Box' expects 1 type arguments,
  got 2`.
Anchored at the callee position (= stage1 CallExpr.Pos()), base name in the
message. The `has_constructor` guard is important: stage1 emits `entity 'X' has no
constructor` and returns BEFORE the generic-arity check, so a generic entity
without a constructor must NOT get the arity error (a sound skip; the `has no
constructor` diagnostic itself is a separate deferred gap).

Per the verified caveat, fixtures use BARE constructor-call statements, not
let-bindings: stage1 also infers the ctor's return type and emits a second
`type mismatch: cannot assign Box to Box` for a let, which stage2 (Unknown ctor
return) cannot reproduce — the bare statement keeps output byte-equal.

+2 fixtures (ck_generic_entity_arity, ck_generic_entity_no_targs — diff-checker
86/86), +4 tests (wrong count, missing type args, correct arity clean, non-generic
entity clean — 278). Verified byte-equal against stage1 for all four plus the
no-constructor divergence (pre-existing, corpus-invisible). All differential gates,
go test ./..., and make validate green.

This opens Phase 53 (independent checker diagnostics). Remaining: the entity
`has no constructor` diagnostic (broader — also non-generic), extern param/return
`unknown type` (0 corpus usage), trait-method contract parser gap. Plus the Phase
48 tail (assert_eq signature sub-checks, async-test no-await warning, unary
operator-typing, spawn/try operand recursion).

---

## 2026-07-03 — Phase 54: multi-file self-hosted checking (ADR 0058) — SELF-HOSTING BLOCKER FIXED

The stage2 checker now checks the compiler's own multi-file source byte-equal
with stage1. Previously `intentc check --self-hosted` checked a single file, so it
emitted hundreds of false positives on real source (check.intent 84, parser.intent
461+ `unknown type` for imported entities/enums, ~1172 `undeclared variable` for
module-name call qualifiers). Fix (ADR 0058):

- **Harness discovery** (`stage2CheckPaths`, cmd/intentc/main.go): for a
  multi-file entry, reuse the Go ModuleRegistry (NewModuleRegistry /
  DiscoverDependencies / TopologicalSort) to find the import closure and pass the
  entry first + every module path to the stage2 binary. runStage2Checker now takes
  a path slice. Single-file entries pass one path — unchanged.
- **Stage2 merge** (`check_main.merge_programs`): parse each path and flatten the
  modules into one Program so imported entities/enums/traits are visible to
  type_is_known / the global scope. Cross-module name collisions dedup first-seen
  (empty_string_array() is defined identically in ast + lexer; stage1 permits it
  via module scoping). The entry is merged verbatim so genuine within-module dups
  still fire; impls/externs appended.
- **Module-name seeding** (`check_program_seeded`): seed imported module names so
  `shared_lexer.foo()` resolves the qualifier instead of `undeclared variable`.
  check_program is now a thin wrapper (empty extra names) → single-file + the 278
  in-language tests are byte-identical.
- **Gate** (`make selfcheck-checker`, selfhost/checker/selfcheck-check.sh): diffs
  stage1 vs stage2 over the 9 core self-hosting modules → 9/9 PASS. Not wired into
  `make validate` (stage2 is slow — ~24s on check.intent's merged closure).

Verified: `intentc check --self-hosted` matches stage1 ("No errors found") on
selfhost/shared/{lexer,ast,parser}, selfhost/checker/{check,check_main},
selfhost/linter/{lint,lint_main}, selfhost/formatter/{format,main}. Single-file
gates unchanged: diff-checker 86/86, diff-formatter 22/22, diff-linter 26/26,
selfcheck-formatter, 278 checker tests, go test ./..., make validate all green.
main_test.go updated for the runStage2Checker signature.

This clears the [[project_stage2_checker_multifile_blocker]]. Remaining Phase 48/53
error-diagnostic gaps (the long tail — missing diagnostics on invalid input) are
now the next candidates, plus the deferred multi-file ERROR-position parity (this
phase targets valid-source parity; see ADR 0058 non-goals).

---

## 2026-07-07 — Phase 55 KICKOFF: self-hosted compiler (IR + backend)

Checker self-hosting milestone is complete (28 diagnostics byte-equal; Phase 54
self-checks the compiler source). Next front DECIDED with the user: the self-hosted
compiler — the bootstrapping endgame. Reimplement in Intent the IR lowering
(internal/ir ~2,745 LOC) + the Rust backend (internal/rustbe ~2,420 LOC) on the
existing self-hosted front-end (~8,700 LOC). Goal: `intentc build --emit
--self-hosted <f>` emits Rust byte-equal with stage1, gated by a new `make
diff-emit`, grown construct-by-construct.

Wrote the kickoff PRD (prds/active/prd-phase-55-self-hosted-compiler.md) with module
layout (selfhost/compiler/{ir,lower,rustbe,compile_main}.intent), phasing, the thin
first slice (55a IR entities → 55b lower hello.intent → 55c emit Rust byte-equal +
wire `--emit --self-hosted` mirroring the Phase 54 harness + `make diff-emit` 1/1),
the gate strategy, and the ADR list (IR modeling, checker-reuse-in-lowering,
`--emit --self-hosted` wiring, contract lowering). Key strategy contrast: unlike the
checker (sound-but-incomplete, ADR 0056), the emitter must be COMPLETE per supported
construct — coverage grows by explicitly expanding the supported set per slice and
the diff-emit corpus only holds fully-supported programs.

No code yet — this session prepared the durable kickoff so work continues cleanly
after a context compaction. NEXT-STEPS + TASKS (Phase 55 rows 55a/b/c/+) updated.

---

## 2026-07-07 — Phase 55a DONE: IR node model (ADR 0059)

Authored `selfhost/compiler/ir.intent` — the stage2 IR node model for the trivial
subset `examples/hello.intent` lowers to, mirroring `internal/ir/nodes.go`:
`IrProgram`/`IrModule`/`IrFunction`/`IrTest`/`IrParam`/`IrContract`, tagged `IrStmt`
(`irst_expr`/`irst_return`) + `IrExpr` (`irex_void`/`int`/`string`/`bool`/`call`),
the full `CallKind` 0–5, `IrType`, and construction/empty-array helpers.

Design (ADR 0059): (D1) tagged-entity `kind: Int` + child arrays, not sum types —
same trade as `shared/ast.intent`. (D2) `Ir`-prefixed names because Intent's
post-merge namespace is flat for entity refs; `lower.intent` (55b) must import both
`shared/ast` (Program/Expr/Stmt/Param) and this module. (D3) an **independent
`IrType`** (shape-identical to the checker's `Type`) keeps `ir.intent` a
dependency-free leaf — answering "reuse the checker in the emit path?" for the
trivial subset: lowering assigns literal types structurally, no `CheckResult`
threaded; a `checker.Type -> IrType` bridge is deferred to the slice that needs it.
(D4) trivial-subset scope, grown per construct; `requires`/`ensures` are keywords so
the fields are `requires_clauses`/`ensures_clauses`.

Gates: added to `selfcheck-formatter` (5/5 EQUAL) + `selfcheck-checker` (**10/10**
PASS, was 9/9). `ir_test.intent` (ungated, like `check_test.intent`) builds a
hello-shaped `IrModule` — 4/4 runtime tests pass. `validate` OK, diff-checker 86/86,
diff-formatter 22/22, diff-linter 26/26, `go test ./...` green. Caveat recorded:
stage1 `intentc fmt` strips comments/reorders — the self-hosted source is a **stage2**
formatter fixpoint. NEXT: 55b `lower.intent` (AST → IR for hello.intent).

---

## 2026-07-07 — Phase 55b DONE: AST → IR lowering for hello.intent

Authored `selfhost/compiler/lower.intent` — `lower(prog, path) -> IrModule` plus
`lower_function`/`lower_test`/`lower_block`/`lower_stmt`/`lower_expr`/
`resolve_call_kind`, mirroring `internal/ir/lower.go` for the trivial subset. Leaf-ish
module: imports `shared/ast` (lowers FROM) + `compiler/ir` (lowers INTO), no checker.

Stage2-vs-Go AST mapping that mattered: a call's callee is `children[0]` (an
ex_ident) with args `children[1..]` — not a `Function` field; a bare `return;` is
st_return whose AST expr is `ex_void`, lowered to `ir_void_expr()`. Confirms ADR 0059
D3: hello's literal types (Int/Bool) are assigned **structurally** by the `ir_*_lit`
helpers — no `CheckResult` threaded, the lowering never touches the checker. Deferred
per the per-construct discipline (ADR 0059 D4): contract lowering (stage2 AST carries
no clause raw-text) and constructor/variant call classification (needs entity/enum
tables) — hello uses neither.

Gates: `lower.intent` added to `selfcheck-formatter` (**6/6** EQUAL) +
`selfcheck-checker` (**11/11** PASS). `lower_test.intent` (ungated) parses embedded
hello source and asserts the full lowered IR shape (main entry / return 0i64 / builtin
`assert(true)` call) — passes under `intentc test` (109 total, 0 failed). Note: literal
`{`/`}` in a string lex as interpolation, so the embedded source injects braces via
`'{'.to_string()` helpers (same trick as check_test.intent). All prior gates green:
diff-checker 86/86, diff-formatter 22/22, diff-linter 26/26, go test cached-OK.

NEXT: 55c `rustbe.intent` — emit Rust byte-equal with stage1 `build --emit` on
hello.intent, wire `build --emit --self-hosted` (mirror the Phase 54 harness:
stage2CheckerBinary/runStage2Checker/stage2CheckPaths), add `make diff-emit` at 1/1.
This is where the IR + lowering get exercised end-to-end and byte-equal-gated.

---

## 2026-07-07 — Phase 55c DONE: IR → Rust byte-equal; BOOTSTRAP LOOP CLOSED for hello

Shipped the trivial-subset milestone. `examples/hello.intent` now emits Rust
**byte-equal** between stage1 `intentc build --emit` and stage2 `intentc build
--emit --self-hosted`. Intent compiles itself (front-end + IR + backend) for hello.

- `selfhost/compiler/rustbe.intent` — `generate(IrModule) -> String` + generate_
  function/test/stmts/stmt/expr/call/builtin_call/args + sanitise_test_name +
  indent_str, mirroring rustbe.go for the entry-fn / return-int / #[test] /
  `assert!` subset. Imports only `compiler/ir` (leaf-ish).
- `selfhost/compiler/compile_main.intent` — stage2 CLI entry: read → parse → lower
  → generate → print (single-file).
- Harness (`cmd/intentc/main.go`): `--self-hosted` flag on `build --emit`;
  `stage2CompilerBinary()` builds/caches the stage2 compiler (env override
  `INTENT_STAGE2_COMPILE`), mirroring `stage2CheckerBinary` (ADR 0058). Strips the
  one trailing print-newline, writes `<base>.rs`. Rust-target, single-file only.
- `make diff-emit` (+ `selfhost/compiler/diff-emit.sh`) — byte-equal gate, **1/1**
  (hello); grows per construct slice.

Two Intent lexer constraints shaped rustbe (ADR 0059 update): literal `{`/`}` in a
string lex as interpolation → braces emitted via `'{'.to_string()` helpers (the `\{`
escape stays literal backslash-brace, unusable); `sanitise_test_name` mirrors stage1's
ASCII rules via codepoints + `String.to_lowercase()`, not the Unicode is_alpha helpers.
Emitter discipline: unsupported constructs emit a loud `// unsupported:` marker (fails
diff-emit), never silently-wrong Rust.

Gates ALL green: diff-emit 1/1, selfcheck-formatter **7/7** EQUAL, selfcheck-checker
**13/13** PASS, new Go test `TestBuildEmitSelfHostedEnvOverride` (3 subtests, fake
binary, no cargo), go test ./... OK, validate OK, diff-checker 86/86, diff-formatter
22/22, diff-linter 26/26. gofmt clean.

MILESTONE: the "thin first slice proves the whole pipeline end-to-end" goal (PRD) is
met. NEXT front = scale construct-by-construct (PRD "Then scale up"): let-bindings &
locals → arithmetic/comparison/logical binops → if/while/for → user functions & calls
→ entities+fields+methods → enums+match → contracts (needs AST raw-text for assert!
messages) → Result/Option/? → generics → closures → async. Each new construct: grow
lower.intent + rustbe.intent, add a byte-equal-gated corpus entry to diff-emit.sh.

---

## 2026-07-07 — Phase 55 scale-up slice 1: let-bindings & locals (diff-emit 2/2)

First construct beyond the thin milestone. `emit-fixtures/let_locals.intent` (`let x:
Int = 42; let mutable y: Int = x; return y;`) emits byte-equal stage1 vs stage2.

- IR (ir.intent): added `irst_let` (=3) + `irex_var` (=5); extended IrStmt with
  name/is_mutable/let_type (defaulted in the ctor, assigned post-construction — the
  irst_expr/irst_return call sites stay 2-arg); added `ir_var_ref` helper.
- lower.intent: st_let → irst_let (name/is_mutable/type_name from AST); ex_ident →
  ir_var_ref. (An ex_ident is a plain var ref in this subset; unit-enum-variant idents
  come with the enum slice.)
- rustbe.intent: `map_type` (Int→i64, Float→f64, String→String, Bool→bool, Void→(),
  Char→char, + Array/Result/Option recursing through type_args; Map/Future/Fn/entity/
  enum fail loudly pending their slices — Map needs a HashMap use-injection). irst_let →
  `let [mut] NAME: TY = VAL;`; irex_var → the bare name. Note: `isMut = is_mutable`
  matches stage1's `Mutable || mutatedVars[name]` for valid programs (only mutable vars
  can be assigned, so mutation implies the flag).
- New emit corpus dir `selfhost/compiler/emit-fixtures/`; diff-emit corpus now 2/2.

Gates green: diff-emit 2/2 EQUAL, selfcheck-formatter EQUAL, selfcheck-checker 13/13,
ir_test 5/5, lower_test 110/110, validate OK. No new ADR (covered by ADR 0059 D4). NEXT:
arithmetic/comparison/logical binops (irex_binary + StringConcat detection + operator
mapping), then if/while/for.

---

## 2026-07-07 — Phase 55 scale-up slice 2: binops (diff-emit 3/3)

`emit-fixtures/binops.intent` (`1 + 2 * 3 - 4`, `a > 5 and a < 100`, `b or false`)
emits byte-equal. Because both stages share the parser, the binary tree is identical,
so recursive fully-parenthesised `(left op right)` emission is byte-equal for free.

- IR: `irex_binary` (=6), name=operator text, children=[left, right]; `ir_binary` helper.
- lower.intent: ex_binop → ir_binary (string-concat `+` → StringConcat deferred to the
  strings slice; needs operand types).
- rustbe.intent: `generate_binary` (`(l op r)`, `implies` → `(!l || r)`) + `map_op`
  (mirrors mapOperator: and→&&, or→||). map_type extended earlier covers the let types.
- diff-emit corpus now 3/3.

Gates green: diff-emit 3/3 EQUAL, selfcheck-formatter OK, selfcheck-checker 13/13,
ir_test 6/6, lower_test 111/111. NEXT: control flow (if/else, while, for-in) — blocks,
else-if chains, `for x in a..b`; while invariants/decreases deferred with contracts.

---

## 2026-07-07 — Phase 55 scale-up slices 3-5: control flow, functions, strings (diff-emit 7/7, TWO real examples)

Three more construct slices; diff-emit now **7/7 EQUAL** including a SECOND real example
(`examples/divergence_demo.intent`) alongside hello — two real programs self-host.

**Slice 3 — control flow (`control_flow.intent`):** if/else (+ one-level else-if flatten),
while, for-in ranges, assignment, blocks. IR: `irst_if/while/for/assign`, `irex_range`;
IrStmt gained then_body/else_body/has_else/target (defaulted). Key finding: the stage2 AST
has **no `st_assign`** — `x = y` is a st_expr wrapping an ex_binop op "=" (children
[target,value]); lower_stmt special-cases it to `irst_assign` (emit `target = value;`, not
`(target = value);`). Fiddly emit replicated exactly: the then-block's closing `}` has NO
trailing newline so ` else ` continues the line (generateIfStmt). All braces via lbrace()/
rbrace().

**Slice 4 — user functions & calls (`functions.intent`):** non-entry function emit
(`fn NAME(p: T, …) -> RET { … }`); the call path already worked via generate_call. Deferred:
Array/Map by-ref params, contracts (guarded by fail-loud checks in generate_function).

**Slice 5 — strings & print (`strings.intent`):** string literals lower by RE-QUOTING the
AST's unquoted content (`"\"" + str_value + "\""`) to match stage1's quoted StringLit.Value;
`print` → `println!("{}", ARG)`. String concatenation (StringConcat, needs operand types)
and interpolation still deferred.

Gates green throughout: diff-emit 7/7, selfcheck-formatter OK, selfcheck-checker 13/13,
ir_test 7/7, lower_test grown. No new ADR (all covered by ADR 0059 D4, per-construct scope).

### Frontier analysis (probed all 22 examples against the stage2 emitter)
Byte-equal now: **hello, divergence_demo**. The rest each need a specific unbuilt slice:
- **Contracts** (fibonacci, sorted_check, bank_account, array_sum, …): `requires`/`ensures`
  → `assert!` + the ensures `'body:` labeled block (return→`break 'body`). BLOCKER: the
  assert message needs the clause's raw text, which stage1 builds as its **tokens joined by
  single spaces** (`parser.go extractRawText`, e.g. `len ( arr ) > 0`). The stage2 AST does
  NOT capture this — so contracts require enriching the SHARED parser/AST (additive, ADR
  0054 style) to store per-clause raw text, a cross-cutting change touching fmt/lint/check.
- **Arrays/Map** (sorted_check, array_sum, map_demo): array/map literals, indexing, `len`,
  Array/Map types (`&Vec`/`HashMap` + HashMap use-injection), by-ref params + call borrow.
- **Entities** (bank_account, shape_area): struct + constructor (`::new`) + methods + field
  access + `self` + invariants.
- **Enums + match** (enum_basic, result_option, error_handling), **Result/Option/`?`**,
  **generics** (generic_stack — monomorphization), **closures** (closure_demo),
  **async** (async_demo, task_queue), **char/float**, **string concat/interp** (char_string_demo).

Each is a substantial, byte-exact slice (the remaining bulk of the ~4000 LOC lower.go+rustbe.go
port). NEXT recommended: contracts (unlocks the most examples) — start with the shared-AST
raw-text enrichment sub-task, then the ensures labeled-block emit.

---

## 2026-07-08 — Phase 55 scale-up slice 6: CONTRACTS (diff-emit 9/9, FOUR real examples)

The biggest unlock so far. `requires`/`ensures` now emit byte-equal, so
`examples/fibonacci.intent` (and `target_specific_demo`) self-host. diff-emit is
**9/9 EQUAL** — four real examples (hello, divergence_demo, fibonacci,
target_specific_demo) + 5 fixtures.

Three sub-tasks:
- **A. Shared-AST enrichment** (the flagged blocker): stage1's contract `RawText` is
  the clause's tokens joined by single spaces (`parser.go:extractRawText`). Added
  `requires_raw`/`ensures_raw: Array<String>` to `shared/ast.intent` FunctionDecl
  (defaulted, ADR 0054 style) + a `clause_raw_text(start)` parser method that joins
  `tokens[start..position-1].text` with " ", captured in the top-level function
  contract loop. **Verified inert**: selfcheck-formatter/checker, diff-checker 86/86,
  diff-formatter 22/22, diff-linter 26/26, go test all still green.
- **B. Lowering**: `lower_contracts` pairs each predicate Expr with its raw text into
  IrContracts. `result` (ex_ident "result", stage1's reserved ResultExpr) lowers to a
  var-ref emitting `__result`.
- **C. Emit**: requires -> `assert!(PRED, "Precondition failed: <raw>")`; ensures (with
  non-Void return) -> the `let __result: T = 'body: { ... };` labeled block + post
  asserts + `__result`. Threaded `in_labeled` through generate_stmts/stmt/if/while/for
  so `return X` inside the block becomes `break 'body X`. Added `assert_eq` builtin.

Deferred: old() in ensures (bank_account), --strip-contracts (assert!->debug_assert!),
entity-method contracts (the other two parser contract loops), arg cloning in assert_eq.

Gates green: diff-emit 9/9, selfcheck-formatter 7/7, selfcheck-checker 13/13, ir_test
8/8, lower_test 115/115, diff-checker/formatter/linter, go test, validate all OK.
NEXT: arrays/Map (sorted_check, array_sum) or entities (bank_account).
