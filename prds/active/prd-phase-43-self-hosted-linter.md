# PRD — Phase 43: Self-Hosted Linter (stage2)

## 1. Introduction / Overview

The stage2 toolchain (`selfhost/formatter/`) already reimplements Intent's lexer,
parser, and AST in Intent, and the formatter is byte-equal with stage1's
`intentc fmt` across the full examples corpus (Phase 42). The **linter** is the
next-smallest tool after the formatter and the best self-hosting proof point
(HARNESS.md §7): it reuses the exact same stage2 lexer/parser/AST and only adds
a read-only AST walk that emits diagnostics.

This phase rewrites the Go linter (`internal/linter/`) in Intent, wires it into
`intentc lint --self-hosted`, and stands up a **byte-equal differential harness**
(`make diff-linter`) that compares stage2 output against stage1 `intentc lint`
across the examples corpus plus a dedicated set of lint fixtures.

### The target: stage1 Go linter inventory

The Go linter (`internal/linter/linter.go`, `Lint(prog) -> *diagnostic.Diagnostics`)
is single-pass, no-config, no-suppression. Every diagnostic is a **warning** in the
exact format:

```
warning[<file>:<line>:<col>]: <message>
```

followed by a CLI summary line `N warning(s) found.` (or `No lint warnings.` when
empty). The **16 rule families**:

| # | Rule | Message template | Anchor |
|---|------|------------------|--------|
| R1 | function missing contracts (skip `entry`) | `function 'X' has no requires or ensures contracts` | function decl |
| R2 | method missing contracts | `method 'E.m' has no requires or ensures contracts` | method decl |
| R3 | trait method missing contracts | `trait method 'T.m' has no requires or ensures contracts` | trait method |
| R4 | extern missing contracts | `extern function 'X' has no requires/ensures; FFI declarations should document the boundary contract` | extern decl |
| R5 | function non-snake_case | `function 'X' should use snake_case naming` | function/impl-method decl |
| R6 | entity non-PascalCase | `entity 'X' should use PascalCase naming` | entity decl |
| R7 | enum non-PascalCase | `enum 'X' should use PascalCase naming` | enum decl |
| R8 | variant non-PascalCase | `variant 'V' in enum 'E' should use PascalCase naming` | variant |
| R9 | entity no invariant | `entity 'X' has fields but no invariant` | entity decl |
| R10 | empty function/method body | `function 'X' has an empty body` (methods use `'E.m'`) | function/method/impl-method decl |
| R11 | intent no verified_by | `intent block has no verified_by references` | intent block |
| R12 | unused variable | `variable 'X' is declared but never used` | `let` stmt |
| R13 | mutable never reassigned | `variable 'X' is declared mutable but never reassigned` | `let mutable` stmt |
| R14 | unused parameter | `parameter 'X' in 'scope' is never used` | param |
| R15 | unused type parameter | `type parameter 'X' in function 'F' is never used in parameters or return type` / entity variant | type param |
| R16 | discarded spawn | `spawn result in 'F' is discarded; assign to a Future variable and await it` | spawn expr-stmt |

Helpers: `is_snake_case` (lowercase/digits/underscore, no leading digit),
`is_pascal_case` (uppercase first, no underscores), and a used-name /
assigned-name collector walking every Stmt and Expr kind (powers R12–R14).

### Measured corpus baseline (2026-06-23)

`intentc lint examples/*.intent` fires **76 warnings across 13 files**, exercising
**8 of 16 rule families**: unused variable (25), mutable-never-reassigned (14),
method missing contracts (14), function missing contracts (12), function naming (4),
entity no-invariant (4), unused parameter (2), trait method contracts (1).

The other 8 families (R4 extern, R6/R7/R8 entity/enum/variant PascalCase, R10
empty-body, R11 intent verified_by, R15 type-params, R16 spawn) are **not** triggered
by the clean canonical examples, so they require dedicated `selfhost/linter/fixtures/`
programs with golden expected-output files to be differentially tested.

## 2. Goals

- Reimplement all 16 Go-linter rule families in Intent, reusing the stage2
  lexer/parser/AST in `selfhost/formatter/`.
- Achieve **byte-equal parity** with stage1 `intentc lint` — including the
  `:line:col` column — on the examples corpus and the lint fixtures.
- Ship a runnable `lint_main.intent`, wire `intentc lint --self-hosted <file>`,
  and commit a `make diff-linter` differential gate (mirrors `make diff-formatter`).
- Preserve byte-equal self-format on the four stage2 source files throughout
  (the new files added here are themselves stage1-canonical) and keep the full
  Go test suite + stage2 suite green.

## 3. Design decisions (and why)

### D1 — Reuse `selfhost/formatter/`; do not create a separate directory (chosen)

The stage2 lexer/parser/AST modules (`formatter_lexer`, `formatter_parser`,
`formatter_ast`) live flat in `selfhost/formatter/` and are reached by
same-directory relative imports (`import "parser.intent"`). A grep confirms **no
cross-directory or subdirectory imports exist anywhere** in the codebase, so a
`selfhost/linter/` dir importing `../formatter/` is unproven and risky. The linter
files (`lint.intent`, `lint_main.intent`, `lint_test.intent`, `fixtures/`) are
therefore added next to the formatter modules. The cosmetic smell (a dir named
"formatter" housing the linter) is acknowledged and deferred: when a third stage2
tool arrives, a dedicated restructure phase (with its own ADR) can split shared
modules into `selfhost/shared/`. Simplicity over premature abstraction.

### D2 — Byte-equal parity including column (chosen over line+message)

The differential gate compares stage2 output to stage1 `intentc lint` **byte for
byte**, matching `warning[file:line:col]: message` exactly — the same rigor the
formatter differential uses. Today stage2 AST decls carry `line: Int` but not
`column`. This phase threads a `column: Int` onto the stage2 AST nodes the linter
anchors on (decls, `let` statements, params, etc.), captured from the same
anchoring token the stage1 AST uses. The two parsers differ, so columns must be
made to agree per construct; the differential harness surfaces each mismatch as a
small fix task — the same gap-closing loop the formatter used.

### D3 — All 16 rule families this phase, gated by the differential (chosen)

Partial rule coverage would diverge on the corpus (and the fixtures), so the
byte-equal gate forces completeness. The 8 corpus-exercised families are covered by
the examples differential; the 8 non-corpus families are covered by golden lint
fixtures committed under `selfhost/linter/fixtures/`.

### D4 — ADR recorded up front (ADR 0046)

Per project convention (write ADRs along the way), the strategy — self-hosted
linter, byte-equal-with-column parity, reuse-formatter-dir, fixtures for non-corpus
rules — is recorded as ADR 0046 before implementation, mirroring ADR 0040 for the
formatter.

## 4. User Stories / Tasks

### US-001 (43.1): ADR 0046 — self-hosted linter strategy
**AC:**
- [ ] `docs/decisions/0046-self-hosted-linter-strategy.md` exists following the ADR
  template, recording D1–D3 with prior-art context (ADR 0040 formatter strategy,
  HARNESS.md §7) and the corpus baseline (76 warnings / 13 files / 8 families).
- [ ] Linked from the PRD and referenced by the linter source header comment.

### US-002 (43.2): Column tracking in stage2 AST + parser
**AC:**
- [ ] A `column: Int` field is added (mirroring `line`) to every stage2 AST entity
  the linter anchors on: `FunctionDecl`, `EntityDecl`, `EnumDecl`, `EnumVariant`,
  `TraitDecl`, `TraitMethodSig`, `ImplDecl`, `IntentBlock`, `ExternDecl`, `Param`,
  `FieldDecl`, and `Stmt` (for `let`). Default via the constructor body (helper
  call), not by changing constructor signatures (per the stage2 ctor-field-init
  pattern).
- [ ] The parser sets `column` from the same anchoring token it already uses for
  `line`, at every site that sets `line`.
- [ ] In-language tests assert the captured `column` for a function decl, an entity
  decl, a `let` statement, and a param.
- [ ] Byte-equal self-format on all four stage2 files preserved (`make
  selfcheck-formatter`); `make diff-formatter` still 22/22; stage2 suite green
  rust + js.

### US-003 (43.3): Linter core scaffold + diagnostic model
**AC:**
- [ ] `selfhost/formatter/lint.intent` (`module formatter_linter`) exists,
  importing parser/ast/lexer. Defines a `LintDiag` entity (`line`, `column`,
  `message`), `public function lint_program(prog: Program) returns Array<LintDiag>`
  dispatching over decl-kind arrays in stage1's order (externs, functions,
  entities, enums, traits, impls, intents), and a `format_diags(diags, file)`
  producing `warning[file:line:col]: message` lines exactly.
- [ ] `is_snake_case` / `is_pascal_case` helpers match stage1 semantics.
- [ ] One rule (R5 function snake_case) wired end-to-end to prove the pipeline,
  with at least 2 in-language tests.
- [ ] Byte-equal self-format preserved; stage2 suite green rust + js.

### US-004 (43.4): Contract-absence rules (R1, R2, R3, R4)
**AC:** function/method/trait-method/extern missing-contracts rules implemented;
`entry` functions skipped for R1; messages byte-match the templates; ≥2 tests per
rule (synthetic fixtures asserting exact warning strings). Self-format preserved.

### US-005 (43.5): Naming rules (R5 entity/enum coverage, R6, R7, R8)
**AC:** entity/enum/variant PascalCase + impl-method snake_case implemented; exact
messages; ≥2 tests each. Self-format preserved.

### US-006 (43.6): Structural rules (R9, R10, R11)
**AC:** entity no-invariant, empty function/method/impl-method body, intent no
verified_by implemented; method empty-body uses `'E.m'` form; exact messages; ≥2
tests each. Self-format preserved.

### US-007 (43.7): Variable rules + used/assigned-name engine (R12, R13)
**AC:**
- [ ] `collect_used_names(stmts)` and `collect_assigned_names(stmts)` walk every
  Stmt and Expr kind the stage2 AST defines (recursing into if/else, while, for,
  match arms, lambdas, calls, field/index, range, forall/exists, try/await/spawn),
  matching stage1's collection semantics.
- [ ] R12 unused variable and R13 mutable-never-reassigned implemented on top.
- [ ] In-language tests cover: a used var (no warning), an unused var (warning),
  a reassigned mutable (no warning), a mutable assigned once (warning), and a var
  used only inside a nested block (no warning). Exact message + position.
- [ ] Self-format preserved; stage2 suite green.

### US-008 (43.8): Parameter rules (R14)
**AC:** unused-parameter for function/method/constructor/impl-method, reusing the
43.7 engine; scope label matches stage1 (`'name' in 'scope'`, scope = function
name or `Entity.method` / `Entity.constructor`); ≥2 tests. Self-format preserved.

### US-009 (43.9): Type-param + spawn rules (R15, R16)
**AC:** unused type parameter (function + entity variants, checking param/return
type usage via the stage2 type-string representation) and discarded-spawn
(spawn expr inside an expr-statement, recursing nested blocks); exact messages;
≥2 tests each (fixtures, not corpus). Self-format preserved.

### US-010 (43.10): Runnable `lint_main.intent`
**AC:**
- [ ] `selfhost/formatter/lint_main.intent` has `entry function main() returns Int`
  reading `args()[1]`, `read_file`, `parse`, `lint_program`, printing
  `format_diags` lines then the summary (`N warning(s) found.` / `No lint
  warnings.`), with exit codes mirroring formatter main (0 ok, 1 usage, 2 read,
  3 parse error).
- [ ] Builds on rust + js; run on a fixture, its stdout is byte-equal to
  `intentc lint` on the same file (modulo the single trailing newline `print`
  adds, accounted for explicitly).

### US-011 (43.11): `intentc lint --self-hosted`
**AC:**
- [ ] `intentc lint --self-hosted <file>` delegates to the built stage2 linter
  binary and writes the same bytes stage1 `intentc lint <file>` would, on every
  passing corpus/fixture file.
- [ ] On a file the stage2 parser cannot handle, exits non-zero naming the file and
  the stage2 parse error (no silent fallback).
- [ ] Binary location/build mechanism documented in the linter README; a Go test
  exercises `--self-hosted` on a fixture. Mirrors the `fmt --self-hosted` shim.

### US-012 (43.12): Differential harness + fixtures + `make diff-linter`
**AC:**
- [ ] `selfhost/linter/fixtures/` (or `selfhost/formatter/lint-fixtures/`) holds
  intentionally-non-canonical `.intent` programs exercising the 8 non-corpus rule
  families, each with a committed golden `.expected` output captured from stage1
  `intentc lint`.
- [ ] `selfhost/formatter/difftest-lint.sh` runs from repo root, runs the stage2
  linter over every `examples/*.intent` and every fixture, and prints per-file
  `PASS` / `DIVERGE <first-diff>` / `PARSE-ERR <msg>` plus a summary; exits 0 only
  when every file is byte-equal PASS.
- [ ] `make diff-linter` invokes it; passes with **76/76 corpus warnings** plus all
  fixtures byte-equal.

### US-013 (43.13): Documentation + final validation
**AC:**
- [ ] `docs/ROADMAP.md` (Phase 43 summary), `prds/NEXT-STEPS.md`, and the
  `selfhost/` README updated; `prds/progress.md` entries appended as tasks land.
- [ ] `make build`, `make test`, `make validate`, `make diff-formatter`,
  `make selfcheck-formatter`, and `make diff-linter` all green; stage2 suite green
  on rust + js.

## 5. Non-Goals

- Replacing stage1's Go linter as the default (`intentc lint` without
  `--self-hosted` stays Go).
- New lint rules beyond the 16 the Go linter has — this is a faithful port.
- Lint configuration / suppression / per-rule enable-disable (stage1 has none).
- Restructuring `selfhost/` into `shared/` + `formatter/` + `linter/` (deferred to
  a future phase + ADR when a third tool lands).
- WASM runtime for the linter binary (rust + js targets only, like the formatter).

## 6. Technical Considerations

- **Column agreement (D2):** stage1 and stage2 are different parsers; the column a
  rule reports must match. Capture the column from the same token anchor stage1
  uses per construct; the differential surfaces mismatches to tune. Run
  `intentc lint` on each corpus file to read the exact expected `:line:col`.
- **Stack depth:** the differential/selfcheck use a built binary (8 MB main stack)
  because `intentc test` runs on a 2 MB libtest stack that overflows on deep parse
  (progress.md). `difftest-lint.sh` mirrors `difftest.sh` — build the binary, run
  it per file. Linter walk is shallow, but reuse the proven harness shape.
- **Absolute paths** in any in-language probe (`read_file` runs from a temp cwd).
- **ctor-field-init pattern:** initialize new non-primitive fields via a helper
  call in the constructor body, never a local referenced in a field initializer.
- **Never run stage1 `intentc fmt` on a stage2 file** — maintain stage2 files only
  via `make selfcheck-formatter`.
- **Print trailing newline:** the shim/harness must account for the single newline
  `print` appends, exactly as the formatter shim does.

## 7. Success Metrics

- `make diff-linter` reports byte-equal parity on all 13 warning-bearing examples
  (76 warnings) + the warning-free examples + all fixtures.
- `intentc lint --self-hosted` is byte-identical to native `intentc lint` on every
  passing file.
- `make build`, `make test`, `make validate`, `make diff-formatter`,
  `make selfcheck-formatter` stay green throughout.

## 8. Open Questions

- Should `--self-hosted` build the rust or js stage2 linter binary by default?
  (Lean js/node for speed + no cargo dependency, as the formatter shim concluded —
  confirm in US-011.)
- Exact column anchor per rule where stage1 and stage2 token positions differ —
  resolved empirically via the differential in US-012.

## 9. Stage1 port reference (AUTHORITATIVE — read before any rule task)

Source: `internal/linter/linter.go`. Diagnostics are NOT sorted
(`internal/diagnostic/diagnostic.go`) — they print in append order. So stage2
MUST emit in stage1's exact order, both across decl kinds and within a single decl.

### Dispatch order (linter.go:25-31)
functions → externs → entities → enums → traits → impls → intents.

### Per-decl check order (exact)
- **Top-level function** (lintFunctions): (1) empty-body R10; (2) if NOT entry:
  missing-contracts R1; (3) naming R5; (4) unused-type-params R15; (5) if body
  present, using collectUsedNames(body): unused-params R14, unused-vars R12,
  mutable-never-reassigned R13, spawn-discarded R16.
- **Entity** (lintEntities): (1) entity naming R6 (PascalCase); (2) no-invariant R9
  (fields>0 && invariants==0); (3) unused-entity-type-params R15e; (4) constructor,
  if body: unused-params R14 with scope `Entity.constructor`; (5) per method, in
  order: empty-body R10 (name `Entity.method`), missing-method-contracts R2, naming
  R5, then if body: unused-params R14 (scope `Entity.method`), unused-vars R12,
  mutable-never-reassigned R13. Methods get NO type-param/spawn checks.
- **Enum** (lintEnums): (1) enum naming R7 (PascalCase); (2) per variant: variant
  naming R8.
- **Trait** (lintTraits): (1) trait-name naming via checkEntityNaming → emits the
  `entity 'X' should use PascalCase naming` message (QUIRK: literally says "entity",
  not "trait" — replicate verbatim); (2) per method: naming R5 FIRST, then contracts
  R3 (`trait method 'T.m' has no requires or ensures contracts`).
- **Impl** (lintImplBlocks): per method: empty-body R10 (name `EntityName.method`),
  naming R5, then if body: unused-params R14 (scope `EntityName.method`), unused-vars
  R12. Impl methods get NO contracts check and NO mutable-never-reassigned.
- **Intent** (lintIntents): empty-verified-by R11.

### Naming-helper anchors / messages
R5 `function 'X' should use snake_case naming`; R6/trait `entity 'X' should use
PascalCase naming`; R7 `enum 'X' should use PascalCase naming`; R8 `variant 'V' in
enum 'E' should use PascalCase naming`; R9 `entity 'X' has fields but no invariant`;
R10 `function 'X' has an empty body` (methods use `Entity.method` as X); R11 `intent
block has no verified_by references`; R1 `function 'X' has no requires or ensures
contracts`; R2 `method 'E.m' has no requires or ensures contracts`; R4 `extern
function 'X' has no requires/ensures; FFI declarations should document the boundary
contract`; R12 `variable 'X' is declared but never used`; R13 `variable 'X' is
declared mutable but never reassigned`; R14 `parameter 'X' in 'scope' is never used`;
R15 `type parameter 'X' in function 'F' is never used in parameters or return type`;
R16 `spawn result in 'F' is discarded; assign to a Future variable and await it`.

### Used-name engine — stage2 mapping (collect_used_names → set of read names)
stage2 has NO assignment statement: `x = y` is `st_expr` whose `expr` is `ex_binop`
with `name == "="`. Replicate stage1 exactly:
- st_let: collect initializer expr only (NOT the bound name).
- st_expr that IS an assignment (ex_binop name "="): collect RHS = children[1] fully;
  for LHS = children[0]: if ex_field collect its object, if ex_index collect
  object+index, if plain ex_ident do NOT collect (write target). (Mirrors stage1
  AssignStmt.)
- st_expr (non-assignment): collect the whole expr.
- st_return: collect value expr if present.
- st_if: collect condition; recurse then_block + else_block.
- st_while: collect condition; recurse body (+ loop invariants/decreases IF stage2
  represents them in the statement; otherwise skip).
- st_for: collect iterable expr; recurse body; do NOT collect the loop variable name.
- Expr kinds: ex_ident → add name. ex_binop/ex_unary/ex_array/ex_index/ex_range →
  recurse all children. ex_call → collect args (children[1..]); callee children[0]:
  if ex_field recurse it (collects receiver), if ex_ident skip (function name not a
  read). ex_field → recurse children[0] (object); field name not collected. ex_match
  → recurse children[0] (scrutinee) + each match_arm.body; skip arm bindings.
  ex_try/ex_await/ex_spawn → recurse children[0]. ex_lambda → recurse children[0]
  (body); skip lambda_params. ex_forall/ex_exists → recurse children (domain+body);
  skip bound name. ex_paren → recurse children[0] (STAGE2-ONLY; stage1 has no paren
  node, so this case is an addition, not a port). Literals → nothing.
- VERIFY stage2's method-call representation in the parser before porting ex_call
  (confirm `obj.m(a)` is ex_call(ex_field(obj,"m"), a)).

### Assigned-name engine — for R13 (collect_assigned_names → set of reassigned names)
Walk statements; for an assignment (st_expr, ex_binop name "=") whose LHS is a plain
ex_ident, mark that name assigned. Recurse into if/while/for blocks. Field/index
targets do NOT count as reassigning the variable.
