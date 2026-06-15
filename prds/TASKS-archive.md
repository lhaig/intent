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
