# Norman Tasks — Archive

Collapsed index of completed phases, migrated from the former `ops/plans/` on
2026-06-15. Each phase's full PRD is preserved under `done/`. Rich shipped
summaries (rationale, prior-art, surfaced gaps) remain in
[docs/ROADMAP.md](../docs/ROADMAP.md); this table is the compact lookup.

## Completed Phases

| Phase | Name | Status | PRD |
|-------|------|--------|-----|
| 11 | Generics (type params, monomorphization) | DONE (2026-03-25, v1.1) | [phase-11-generics.md](done/phase-11-generics.md) |
| 12 | Async / concurrency (Rust + JS; WASM rejects) | DONE (2026-03-25, v1.1) | [phase-12-async.md](done/phase-12-async.md) |
| 13 | Package management — local path deps + manifest | DONE (2026-03-25, v1.1) | [phase-13-packages.md](done/phase-13-packages.md) |
| 14 | Phase 11–13 gap closure (audit fixes) | DONE (2026-05-28) | [phase-14-phase11-13-gaps.md](done/phase-14-phase11-13-gaps.md) |
| 15 | Rust FFI / crate imports (completes Milestone 7) | DONE (2026-05-28) | [phase-15-rust-ffi.md](done/phase-15-rust-ffi.md) |
| 16 | In-language testing framework (`intentc test`) | DONE (2026-05-29) | [phase-16-testing-framework.md](done/phase-16-testing-framework.md) |
| 17 | Testing framework polish (17.B/C/D/F) | DONE (2026-05-30) | [phase-17-testing-polish.md](done/phase-17-testing-polish.md) |
| 18 | LSP server v1 (`intentc lsp`, VS Code ext) | DONE (2026-05-30) | [phase-18-lsp-server.md](done/phase-18-lsp-server.md) |
| 19 | LSP v1 completion (scope walker, symbols, fmt) | DONE (2026-05-30) | [phase-19-lsp-v1-completion.md](done/phase-19-lsp-v1-completion.md) |
| 20 | LSP polish + production extension | DONE (2026-05-31) | [phase-20-lsp-polish-production.md](done/phase-20-lsp-polish-production.md) |
| 21 | LSP member completion (`.field` / `.method`) | DONE (2026-05-31) | [phase-21-lsp-member-completion.md](done/phase-21-lsp-member-completion.md) |
| 22 | `--strip-contracts` flag | DONE (2026-05-31) | [phase-22-release-flag.md](done/phase-22-release-flag.md) |
| 24 | Per-contract verify source positions | DONE (2026-05-31) | [phase-24-verify-source-positions.md](done/phase-24-verify-source-positions.md) |
| 25 | Cross-package goto-def (test-only) | DONE (2026-05-31) | [phase-25-cross-package-goto-def.md](done/phase-25-cross-package-goto-def.md) |
| 26 | LSP find-references (`textDocument/references`) | DONE (2026-05-31) | [phase-26-lsp-find-references.md](done/phase-26-lsp-find-references.md) |
| 27 | testgen entity/method emission (`--target intent`) | DONE (2026-05-31) | [phase-27-testgen-entity-emission.md](done/phase-27-testgen-entity-emission.md) |
| 28 | testgen multi-param iteration | DONE (2026-06-01) | [phase-28-testgen-multi-param.md](done/phase-28-testgen-multi-param.md) |
| 29 | Retire legacy Rust testgen path | DONE (2026-06-02) | _(no PRD file; recorded in ROADMAP + ADR 0038)_ |
| 30 | Package registry — git + MVS | DONE (2026-06-03) | [phase-30-package-registry.md](done/phase-30-package-registry.md) — PRD status line stale ("Planning"); shipped per ROADMAP + git |
| 31 | String indexing + `Char` type | DONE (2026-06-03) | [phase-31-string-primitives.md](done/phase-31-string-primitives.md) — PRD status line stale ("Planning"); shipped per ROADMAP + git |
| 32 | Lexer in Intent (stage2 step 1) | DONE (2026-06-03) | [phase-32-lexer-in-intent.md](done/phase-32-lexer-in-intent.md) |
| 33 | Parser top-level in Intent | DONE (2026-06-03) | [phase-33-parser-toplevel-in-intent.md](done/phase-33-parser-toplevel-in-intent.md) |
| 34 | Statement parser in Intent | DONE (2026-06-03) | [phase-34-statement-parser-in-intent.md](done/phase-34-statement-parser-in-intent.md) |
| 35 | Expression parser in Intent (Pratt) | DONE (2026-06-03) | [phase-35-expression-parser-in-intent.md](done/phase-35-expression-parser-in-intent.md) |
| 36 | Top-level decls in Intent + AST split | DONE (2026-06-03) | [phase-36-top-level-decls-in-intent.md](done/phase-36-top-level-decls-in-intent.md) |
| 37 | Stage2 lexer extensions (char/float/block comments) | DONE (2026-06-09) | [phase-37-stage2-lexer-extensions.md](done/phase-37-stage2-lexer-extensions.md) |
| 38 | Stage2 formatter MVP (`format.intent`) | DONE (2026-06-09) | [phase-38-stage2-formatter-mvp.md](done/phase-38-stage2-formatter-mvp.md) |
| 39 | Self-parse certification | DONE (2026-06-09) | [phase-39-self-parse-certification.md](done/phase-39-self-parse-certification.md) |
| 40B | Precedence-aware paren stripping | DONE (2026-06-09) | [phase-40b-paren-stripping.md](done/phase-40b-paren-stripping.md) |
| 40C | Source-order tracking (per-decl `line: Int`) | DONE (2026-06-09) | [phase-40c-source-order-tracking.md](done/phase-40c-source-order-tracking.md) |

Phase 40A (comment preservation) is still active — see [TASKS.md](TASKS.md).

## Phase 42: Stage2 Formatter CLI Wiring + Differential Test — COMPLETE (2026-06-16)

Make the stage2 formatter a runnable CLI tool, wire it into `intentc fmt
--self-hosted`, and stand up a committed differential-test harness vs `intentc
fmt` over `examples/*.intent`. The harness drives gap-closing: each example the
stage2 parser can't yet handle is a small parse+format task (Phase 41 pattern).
Baseline: 12/22 examples already byte-equal (= agree with `intentc fmt`). See
[phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 42.1 | `args()` builtin (Array<String>) + ADR; checker, IR, rust/js/wasm backends | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | ADR 0045; +2 checker tests; rust=env::args, js=process.argv.slice(1), wasm=stub; emit verified rust+js |
| 42.2 | `main.intent` runnable formatter (reads args()[1], prints format_program) | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | builds+runs rust+js; hello byte-equal modulo 1 trailing newline; exit codes 0/1/2/3. Fixed stage1 bug: entry fn in imported module no longer dupes main (rustbe+jsbe; +2 tests) |
| 42.3 | Differential-test harness: `difftest.sh` + `make diff-formatter` | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | canonicalize-first (compares vs intentc fmt output); 13/22 PASS, 0 diverge, 9 parse-err; exits 1 as gate; bash 3.2 compatible |
| 42.4 | `intentc fmt --self-hosted` Go shim (delegates to stage2 binary) | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | env override INTENT_STAGE2_FMT + auto-build-with-cache; composes with --check; parse error exits non-zero, no fallback; +13 tests (fake-binary, no cargo); byte-equal w/ native fmt verified e2e |
| 42.5 | Gap: entity `invariant <expr>;` (+ constructor contracts + intent blocks) | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | fixed invariant form+position (was wrong block form); folded in constructor contracts + intent-block `intent "d" {...verified_by:[...]}` (needed by the 3 files); bank_account/js_demo/task_queue PASS; 16/22; byte-equal preserved; 171/171 rust+js |
| 42.6 | Gap: `forall`/`exists` quantifier expressions | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | ex_forall/ex_exists primaries; `in` as ident; sorted_check PASS; 19/22 |
| 42.7 | Gap: `implies` operator | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | folded into parse_assign (no new parse-chain frame); try_operator PASS |
| 42.8 | Gap: generic type params on declarations `<T>` + generic instantiation | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | EntityDecl/FunctionDecl type_params (defaulted in ctor body, no call-site churn); Ident<Args>( disambiguation via lookahead; generic_stack PASS; 20/22 |
| 42.9 | Gap: `Fn(...) -> T` types + lambdas | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | tk_thin_arrow `->`; Fn type in parse_type_name; ex_lambda (lambda_params field); closure_demo PASS; 21/22 |
| 42.10 | Gap: `await` (+`spawn`, `async test`, async-modifier order) | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | ex_await/ex_spawn; async test in parse_program; fixed modifier order; async_demo PASS |
| 42.11 | Gap: attributes `@name(args)` | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-16) | TestDecl.annotations; parse before test in parse_program; target_specific_demo PASS; 22/22 corpus |
| 42.12 | char_string_demo: compare vs stage1 output; confirm or file follow-up | [phase-42-formatter-cli-differential.md](done/phase-42-formatter-cli-differential.md) | DONE (2026-06-15) | RESOLVED by 42.3 harness design: canonicalize-first comparison makes char_string_demo PASS — no real stage2 divergence (the raw fixture was simply non-canonical) |

## Phase 43: Self-Hosted Linter (stage2) — COMPLETE (2026-06-24)

Faithful port of the 16 Go-linter rule families into Intent, reusing the stage2
lexer/parser/AST. Byte-equal parity with stage1 `intentc lint` (including `:col`),
gated by `make diff-linter` (26/26). ADR 0046. Wired as `intentc lint --self-hosted`.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 43.1 | ADR 0046 — self-hosted linter strategy | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-23) | docs/decisions/0046; D1 reuse-formatter-dir, D2 byte-equal-w/-col, D3 all-16-gated |
| 43.2 | Column tracking in stage2 AST + parser | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | +column on 9 decls; +line+column on Stmt/Param/EnumVariant/TraitMethodSig; defaulted-in-body + post-construction assign; +4 tests |
| 43.3 | Linter core scaffold + diagnostic model | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | lint.intent + lint_test.intent; LintDiag, dispatch (fns→externs→entities→enums→traits→impls→intents), format_diags, is_snake/pascal_case, R5; +18 tests |
| 43.4 | Used-name + assigned-name engine | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | collect_used_names/from_stmt/from_expr + collect_assigned_names + name_in; functional (pass-by-value); +5 tests |
| 43.5 | Complete lint_function_decl (R10,R1,R5,R14,R12,R13,R16) | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | stage1 order; reusable check_unused_params/variables/mutable + find_discarded_spawns; advisor revise fixed R12/R13 split, R13 top-level, R16 recursion |
| 43.6 | Complete lint_entity_decl + lint_impl_decl | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | entity R6/R9/ctor-R14 + methods R10/R2/R5/R14/R12/R13; impl R10/R5/R14/R12; guard fix; +3 locking tests |
| 43.7 | Enum/trait-naming/intent dispatch (R7,R8,R6-trait,R5,R11) | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | enum R7/R8, trait R6("entity" quirk)+R5, intent R11 |
| 43.8 | R3 + R4 (trait-method + extern contracts) | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | always-fire (stage2 parser rejects contracts on trait-methods/externs); R3 after R5; R4 in lint_extern_decl |
| 43.9 | Type-param position enrichment + R15 (fn) + R15e (entity) | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | additive type_param_lines/columns + empty_int_array; parser plumbing; token-aware type_uses_param; +13 tests (188). ALL 16 rules |
| 43.10 | Runnable `lint_main.intent` | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | entry main; output byte-identical to `intentc lint` on map_demo(18)/task_queue(17)/enum_basic(12)/...; rust+js |
| 43.11 | `intentc lint --self-hosted` Go shim | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | parseLintFlags + stage2LinterBinary (INTENT_STAGE2_LINT override + build/cache) + runStage2Linter (verbatim); +Go tests |
| 43.12 | Differential harness + fixtures + `make diff-linter` | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | difftest-lint.sh + 4 fixtures (R6/R7/R8/R9/R10/R11/R15/R15e) + Makefile target; 26/26 PASS; R4 unit-test-only (extern syntax differs) |
| 43.13 | Docs (ROADMAP/NEXT-STEPS/README) + final validate | [prd-phase-43-self-hosted-linter.md](done/prd-phase-43-self-hosted-linter.md) | DONE (2026-06-24) | ROADMAP Phase 43 entry; NEXT-STEPS rewrite; both selfhost READMEs; make validate + all stage2 gates green |
