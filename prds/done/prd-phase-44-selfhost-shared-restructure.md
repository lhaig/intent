# PRD — Phase 44: `selfhost/shared/` Restructure

## 1. Introduction / Overview

The stage2 self-hosted toolchain has two tools — the **formatter** (Phase 38-42) and
the **linter** (Phase 43) — both living in `selfhost/formatter/` and sharing one
lexer/parser/AST. [ADR 0050](../../docs/decisions/0050-self-hosted-linter-strategy.md)
D1 deliberately deferred splitting the shared front-end into `selfhost/shared/`
**"until a third stage2 tool lands."** The **checker** (Phase 45) is that third tool.

This phase performs the restructure **before** the checker is written, so the checker
starts life as a clean sibling rather than piling a third tool into a directory named
"formatter". After this phase the layout is:

```
selfhost/
  shared/      lexer.intent, ast.intent, parser.intent   (the shared front-end)
  formatter/   format.intent, main.intent, format_test.intent
  linter/      lint.intent, lint_main.intent, lint_test.intent, lint-fixtures/
  (checker/    arrives in Phase 45)
```

It is a **pure refactor**: no behaviour changes. The four green gates
(`make selfcheck-formatter`, `make diff-formatter` 22/22, `make diff-linter` 26/26,
`make validate` / full Go suite) must hold identically before and after.

## 2. Goals

- Create `selfhost/shared/` and move `lexer.intent`, `ast.intent`, `parser.intent`
  into it.
- Move the linter (`lint.intent`, `lint_main.intent`, `lint_test.intent`,
  `lint-fixtures/`) into `selfhost/linter/`; the formatter stays in
  `selfhost/formatter/`.
- Re-point every import to the new locations (cross-directory `../shared/…`), rename
  the shared modules `formatter_{lexer,ast,parser}` → `shared_{lexer,ast,parser}` and
  the linter module `formatter_linter` → `linter`, and update every qualified
  reference.
- Update the harness scripts (`selfcheck.sh`, `difftest.sh`, `difftest-lint.sh`),
  the Makefile, and the Go shim path (`stage2LinterBinary` → `selfhost/linter/
  lint_main.intent`).
- Keep all four gates byte-for-byte green throughout.

## 3. Design decisions (and why) — recorded in ADR 0051

### D1 — Do the restructure NOW (the third tool landed)
ADR 0050 D1 set the explicit trigger: split when a third stage2 tool arrives. The
checker is that tool. Doing the split first means the checker is authored against the
final layout (no later move of checker files), and the formatter/linter moves happen
while only two tools exist (less to re-point than after a third is added).

### D2 — Three sibling tool dirs + one `shared/` (chosen over alternatives)
- **Chosen:** `shared/` (front-end) + `formatter/` + `linter/` + (later) `checker/`,
  each tool dir importing `../shared/…`.
- *Rejected:* keep everything flat in `formatter/` (the status quo — misleading once
  three tools share it; the cosmetic smell ADR 0050 D1 acknowledged).
- *Rejected:* one big `selfhost/` flat dir (loses tool boundaries).

### D3 — Cross-directory imports are used (verified feasible)
Module imports resolve via `filepath.Clean(filepath.Join(entryDir, importPath))`
(`internal/compiler/registry.go:509`), so `import "../shared/parser.intent"` resolves
correctly. No cross-dir import exists in the codebase today only because nothing
needed one — not because it is unsupported. Task 44.2 confirms this empirically before
the move.

### D4 — Rename the shared modules to `shared_*` (clean over low-churn)
The shared modules are renamed `formatter_{lexer,ast,parser}` → `shared_*` and the
linter module → `linter`, with all qualified references updated. This is the churniest
part but is what "clean restructure" means; keeping `formatter_*` names in `shared/`
would re-introduce the very smell this phase removes. Renames are pure identifier
changes — they preserve the self-format fixpoint as long as the files stay canonical.

### D5 — Pure refactor, gate-protected, no behaviour change
Every gate must produce identical output before/after. The moved `.intent` files stay
self-format fixpoints (verified via the stage2 formatter binary / `make
selfcheck-formatter`, never stage1 `intentc fmt`).

## 4. User Stories / Tasks

### US-001 (44.1): ADR 0051 — selfhost/shared restructure
**AC:** `docs/decisions/0051-selfhost-shared-restructure.md` records D1-D5 with the
ADR 0050 D1 trigger, the chosen layout, the cross-dir-import basis, and the
module-rename decision. Linked from this PRD.

### US-002 (44.2): Verify cross-directory imports (spike)
**AC:** A throwaway check confirms an Intent file in one dir can `import "../other/
file.intent"` and build on rust. If it does not work, the gap is captured and a
stage1 fix is scoped before proceeding. (Expected: works, per D3.)

### US-003 (44.3): Create `selfhost/shared/` + re-point the formatter
**AC:**
- [ ] `lexer.intent`, `ast.intent`, `parser.intent` moved (`git mv`) to
  `selfhost/shared/`; their modules renamed to `shared_{lexer,ast,parser}`; internal
  cross-references among them updated.
- [ ] `selfhost/formatter/{format,main,format_test}.intent` import the three via
  `../shared/…` and use the `shared_*` qualified names.
- [ ] `selfcheck.sh` updated to check the fixpoint on `shared/{lexer,ast,parser}` +
  `formatter/format` (the four stage2 source files at their new paths).
- [ ] `difftest.sh` + Makefile paths updated.
- [ ] Gates green: `make selfcheck-formatter` 4 EQUAL, `make diff-formatter` 22/22,
  formatter builds on rust+js.

### US-004 (44.4): Move the linter to `selfhost/linter/`
**AC:**
- [ ] `lint.intent`, `lint_main.intent`, `lint_test.intent`, `lint-fixtures/` moved
  to `selfhost/linter/`; linter module renamed `formatter_linter` → `linter`; imports
  re-pointed to `../shared/…`; qualified refs updated.
- [ ] `difftest-lint.sh` + Makefile `diff-linter` paths updated.
- [ ] The Go shim `stage2LinterBinary` (`cmd/intentc/main.go`) updated to build from
  `selfhost/linter/lint_main.intent`; its staleness check scans the right dirs.
- [ ] Gates green: `make diff-linter` 26/26, `intentc lint --self-hosted` works,
  full Go suite passes.

### US-005 (44.5): Docs + final validation + push
**AC:**
- [ ] `selfhost/README.md` + both tool READMEs updated to the new layout; ROADMAP
  Phase 44 entry; NEXT-STEPS updated.
- [ ] `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
  `make diff-linter` all green. Commit + push.

## 5. Non-Goals

- Any checker code (Phase 45).
- Any behaviour change to the formatter or linter (pure refactor).
- Enabling new import features beyond what 44.2 confirms is already supported.
- Renaming the formatter module (`format.intent`) unless trivially consistent —
  the formatter dir keeps its identity.

## 6. Technical Considerations

- **Never run stage1 `intentc fmt` on a stage2 file** — re-canonicalize moved/renamed
  files via the stage2 formatter binary or by hand; verify with `make
  selfcheck-formatter`.
- **Import base is the entry file's dir** — `../shared/…` from `formatter/` and
  `linter/` resolves to `selfhost/shared/`.
- **Module rename is mechanical but wide** — a missed qualified reference is a compile
  error (caught by build); the `shared_*` rename touches format/parser/lexer/ast +
  lint + the two mains + the two test files.
- **Go shim path** — `stage2LinterBinary` hard-codes `selfhost/formatter/
  lint_main.intent`; it must move to `selfhost/linter/`. `stage2FormatterBinary` stays
  (`selfhost/formatter/main.intent`).
- **selfcheck.sh** lists the four stage2 source files by path — update to the new
  `shared/` + `formatter/` paths.

## 7. Success Metrics

- Identical gate output before/after: `make selfcheck-formatter` 4 EQUAL,
  `make diff-formatter` 22/22, `make diff-linter` 26/26, `make validate` green.
- `intentc fmt --self-hosted` and `intentc lint --self-hosted` byte-identical to
  before the move.

## 8. Open Questions

- Final module name for the formatter (`format.intent`) — keep as-is vs. normalize.
  Resolved during 44.3 (prefer minimal change: keep its current name).
