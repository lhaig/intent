# Phase 24: Per-Contract Source Positions in Verify Diagnostics

**Status:** Shipped (2026-05-31; commits c778e40..HEAD)
**Milestone:** v1.2 — Self-Improvement Foundations (LSP UX polish)
**Decision:** [ADR 0034](../../docs/decisions/0034-per-contract-source-positions.md)

## Goal

Thread per-clause source positions through the AST → IR → verify subsystem → LSP so a failed `requires` / `ensures` / `invariant` / loop-invariant / decreases check anchors its editor diagnostic at the actual clause, not at line 1 column 1.

Today every verify diagnostic in VS Code draws the squiggly under the module declaration regardless of which clause failed. The position data already exists on `ast.ContractClause` from the parser — it's just discarded during IR lowering, and `verify.VerifyResult` has no field to carry it. ADR 0034 establishes the convention; this PRD lands it.

## Success Criteria

- [x] `ir.Contract` and `ir.DecreasesClause` carry `Line int` / `Column int` fields populated from the AST
- [x] `verify.VerifyResult` carries `Line int` / `Column int` fields populated from the IR contract during Z3 verification
- [x] Loop-invariant and decreases checks carry the loop's invariant / decreases clause position
- [x] Toolchain-error rows (z3-not-found, translation errors with no clause origin) leave `Line = 0, Column = 0` and the LSP falls back to anchoring at the file start with a "no source position" framing
- [x] `internal/lsp/verify.go` `verifyResultsToDiagnostics` converts 1-indexed parser positions to 0-indexed LSP positions and emits a 1-character range
- [x] New tests cover: requires/ensures positions populated on functions; invariant positions populated on entities; loop-invariant positions populated; toolchain-error rows safely default
- [x] LSP integration: a failing-`ensures` example produces a diagnostic at the `ensures` keyword, not at (1,1)
- [x] All existing verify / LSP tests pass
- [x] `make validate` green
- [x] No regression in `intentc verify`'s console output (positions don't surface there this phase — that's a separate UX call)

## Reference

- ADR 0034: `docs/decisions/0034-per-contract-source-positions.md`
- AST contract positions: `internal/ast/nodes.go` (`ContractClause.Line` / `Column` already exist)
- IR lowering: `internal/ir/lower.go` (loses positions today)
- IR contract type: `internal/ir/nodes.go` (`Contract`, `DecreasesClause` — gets new fields)
- Verify subsystem: `internal/verify/verifier.go` (`VerifyResult`, `verifyFunctionWithZ3`, `verifyEntityWithZ3`)
- LSP verify integration: `internal/lsp/verify.go` (`verifyResultsToDiagnostics`)
- Existing verify tests: `internal/verify/verify_test.go`
- Existing LSP verify tests: `internal/lsp/verify_test.go`

## Tasks

### 24.1 Position fields on IR contract types

**Files:** `internal/ir/nodes.go`, `internal/ir/lower.go`, `internal/ir/lower_test.go` (if present)

Add `Line int` and `Column int` to:
- `ir.Contract` — used for `requires` / `ensures` / `invariant`
- `ir.DecreasesClause` — used for termination metrics

Update lowering in `internal/ir/lower.go` to copy from the AST:
- Function `requires` / `ensures`: copy from `ast.FunctionDecl.Requires[i].Line/.Column`
- Method `requires` / `ensures`: same shape
- Constructor `requires` / `ensures`: same
- Entity `invariant`: copy from `ast.EntityDecl.Invariants[i]`
- Loop `invariant`: copy from `ast.WhileStmt.Invariants[i]`
- Loop `decreases`: copy from `ast.WhileStmt.Decreases`

**Acceptance:** A unit test on the lowering path verifies the IR contract carries the expected 1-indexed line/col for a multi-line function with two preconditions and two postconditions on different lines.

### 24.2 Position fields on VerifyResult

**Files:** `internal/verify/verifier.go`, `internal/verify/verify_test.go`

Add `Line int` and `Column int` to `VerifyResult`. Populate from the IR contract in both verify paths:
- `verifyFunctionWithZ3` — `result.Line = req.Line; result.Column = req.Column` on the requires loop; same for ensures and loop invariants
- `verifyEntityWithZ3` — same for invariants, then the entity's methods / constructor

Toolchain-error rows (the `"z3 not found on PATH"` row, translation errors) leave Line/Column at zero.

**Acceptance:** Existing tests pass. A new test seeds a function with two requires on lines 3 and 5; the corresponding `VerifyResult` rows carry those lines.

### 24.3 LSP: build proper Range from positions

**Files:** `internal/lsp/verify.go`, `internal/lsp/verify_test.go`

In `verifyResultsToDiagnostics`:

```go
var rng Range
if r.Line > 0 {
    // 1-indexed parser → 0-indexed LSP
    rng = Range{
        Start: Position{Line: r.Line - 1, Character: r.Column - 1},
        End:   Position{Line: r.Line - 1, Character: r.Column},
    }
} else {
    rng = Range{
        Start: Position{Line: 0, Character: 0},
        End:   Position{Line: 0, Character: 1},
    }
}
```

Preserve the existing `Severity` / `Source` / `Message` logic; only the `Range` is computed differently.

**Acceptance:** New LSP test: a document with a deliberately-unprovable `ensures` triggers a diagnostic whose Range start matches the `ensures` clause position.

### 24.4 LSP E2E test extension

**Files:** `internal/lsp/e2e_test.go`

The existing E2E test already exercises verify diagnostics indirectly through `publishDiagnostics`. Add an explicit assertion: open a document with a known-failing contract on a specific line; the returned `publishDiagnostics` Range start matches that line (not line 0).

If the existing E2E test is unstable around Z3 availability, gate the new assertion behind a `z3` PATH lookup.

**Acceptance:** When Z3 is available, the assertion holds. When not, the test logs "z3 unavailable; skipping verify-position assertion" and passes.

### 24.5 Docs

**Files:** `docs/ROADMAP.md`, `INTENT.md` (editor support section), `prds/done/phase-24-verify-source-positions.md`

- `docs/ROADMAP.md`: add `### Phase 24: Per-Contract Verify Source Positions -- SHIPPED (date)` under v1.2 with commit range; cross-reference ADR 0034.
- `INTENT.md` "Editor support" — remove or update the "Z3 anchors at (1,1)" caveat (currently lives in ADR 0032's deferred list; INTENT.md may or may not echo it — check and update).
- This PRD: flip Status to `Shipped (date)`.

**Acceptance:** `make validate` green. All cross-references consistent.

## Out of Scope

- **Range-based anchoring** across all diagnostic sources. Useful project-wide but not specific to verify; separate cleanup.
- **`intentc verify` console output** with `file:line:col:` prefixes. Worth doing eventually; not blocking this phase.
- **Quick-fix code actions** on verify diagnostics (add a missing precondition, etc.). Per-rule design.
- **Persisted `.verify.json`** cache. Belongs in the verify-aware-stripping ADR (ADR 0033's future-work section).

## Suggested Order

1. **24.1 IR position fields + lowering** — smallest unit; pure additive change
2. **24.2 VerifyResult fields + verifier population** — depends on 24.1
3. **24.3 LSP Range computation** — depends on 24.2
4. **24.4 E2E test extension** — locks the wire contract
5. **24.5 Docs + status flip** — last
