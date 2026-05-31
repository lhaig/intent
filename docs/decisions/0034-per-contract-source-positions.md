# 0034: Per-Contract Source Positions in Verify Results

**Date:** 2026-05-31
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (LSP UX polish)

## Context

`intentc verify` runs Z3 on every `requires` / `ensures` / `invariant` / loop-invariant / decreases clause in a module and produces a `VerifyResult` per clause. The struct carries `FunctionName`, `EntityName`, `ContractKind`, `ContractText`, and `Status` — but no source position. The downstream LSP layer (`internal/lsp/verify.go`) consequently anchors every verify diagnostic at `Position{Line: 0, Character: 0}` to `Position{Line: 0, Character: 1}`. In VS Code that draws the squiggly under the module declaration on line 1, regardless of which contract clause actually failed verification.

The AST `ContractClause` (and `DecreaseClause`, `WhileStmt.Invariants`) already carries `Line` and `Column` from the parser. The position information exists; it's just discarded during IR lowering, and the verify subsystem has no field to carry it onward.

Every comparable language with a static verification layer surfaces per-clause source positions in its tool output. Dafny prints `file.dfy(<line>,<col>): Error: ...` per failing obligation. SPARK Ada's `gnatprove` emits `<file>:<line>:<col>: medium: postcondition might fail` per VC. F* / Lean / Coq surface positions in the goal context. Liquid Haskell tags every refinement with its position. Eiffel ties assertion failures to the line where the contract was written. Intent's current behaviour — anchor everything at (1,1) — is a regression vs. this established UX baseline. Users typing in an editor want the underline to mark *which* contract failed, not "somewhere in this file."

The fix is mechanical: thread line/col from the AST through the IR and into `VerifyResult`, then have the LSP build proper LSP `Range` values from them. The choices that warrant capturing in an ADR are:

1. Do we anchor at the *clause keyword* (`requires`/`ensures`/`invariant`) or at the *expression inside the clause*?
2. Single position or a range (start..end)?
3. What about clauses with no natural source position (auto-generated postconditions, synthetic checks)?

## Options

### O1. Where the position points

**A. Start of the clause keyword.** [Chosen.] The position points at the `requires` / `ensures` / `invariant` token. Matches what the parser records (the AST `ContractClause.Line`/`Column` is the keyword position). Matches what users want to see underlined — the verification status applies to the whole clause, not just the expression body.

**B. Start of the contract expression.** Would require the parser to record a second position. Adds plumbing for no UX gain.

**C. Range covering the entire clause.** Would require the parser to record the end-of-clause token (semicolon or end-of-line). Possible but not done today; consistent with the rest of the LSP (parser/checker diagnostics also use single-position anchors today with a 1-character range). Defer until a real example exposes the limitation.

### O2. Single position vs. range

**A. Single (line, col) anchor.** [Chosen.] Mirrors the existing LSP diagnostic convention (`internal/lsp/verify.go` already emits a 1-character range from a single anchor). Consistent across parser, checker, lint, and verify diagnostics.

**B. Source range (start..end).** Would require end-position tracking through the AST. Worth doing project-wide as a future cleanup, but not specific to verify — every diagnostic source would benefit. Out of this ADR's scope.

### O3. Handling clauses without a natural source position

Some `VerifyResult` rows aren't a 1:1 reflection of user-written source:

- Errors from the `Verify` entry point itself (`"z3 not found on PATH"`) — no clause, no position.
- Loop-invariant checks: the loop's `Invariants[]` carry positions from the parser; thread them.
- Decreases checks: same; AST has positions on `DecreaseClause`.
- Termination metric "must be non-negative at entry" / "did not decrease" — synthetic checks that share the decreases clause's position. Reuse it.

**Convention:** when no natural source position exists (toolchain-error rows), `VerifyResult.Line = 0, Column = 0`. The LSP treats `Line == 0` as "no position" and falls back to anchoring at the start of the file with a clear message (`"verify error (no source position): ..."`). This preserves the existing LSP behaviour for that narrow case.

### O4. Where the data lives

**A. Add `Line int, Column int` to `ir.Contract` and `ir.DecreasesClause`.** [Chosen.] Lowering in `internal/ir/lower.go` copies them from `ast.ContractClause.Line` / `Column`. The verify subsystem reads them off the IR.

**B. Thread positions as a separate parallel structure.** Awkward; positions belong on the clause they describe. Rejected.

**C. Add them to `VerifyResult` only, parsing them out of the IR at verify time.** The IR is the right level to carry them — multiple consumers (verify, future tooling, possibly the rust backend's assertion-message format) all benefit. Rejected.

### O5. Backwards compatibility

- Existing `VerifyResult` consumers: `internal/verify/report.go` (`BuildIntentReports`) and `internal/lsp/verify.go` (`verifyResultsToDiagnostics`). Both add fields, neither breaks. `verifyResultsToDiagnostics` becomes correct rather than degenerately-anchored.
- `intentc verify`'s console output today doesn't print positions; this ADR doesn't change that surface. Adding per-clause positions to the CLI output is a separate UX choice (future PRD). [Chosen behaviour: positions land in `VerifyResult` and reach the LSP; console output unchanged this phase.]

## Decision

**O1.A + O2.A + O3 (single position; zero-position for toolchain errors) + O4.A + O5.A.**

1. Add `Line int` and `Column int` to `ir.Contract` and `ir.DecreasesClause`. Lowering copies them from the AST.
2. Add `Line int` and `Column int` to `verify.VerifyResult`. The two `verifyFunctionWithZ3` / `verifyEntityWithZ3` paths populate them from the IR contract. Toolchain-error rows (no clause) get `Line = 0, Column = 0`.
3. `internal/lsp/verify.go` `verifyResultsToDiagnostics` builds an LSP `Range` from the position: if `Line > 0`, anchor at `(Line - 1, Column - 1)` (LSP is 0-indexed; the parser is 1-indexed) and use a 1-character end position. If `Line == 0`, fall back to the file-start anchor as today.
4. The console verify output (`intentc verify <file>`) is unchanged this phase. A future PRD can add `file:line:col:` prefixes; out of scope here.
5. `intentc verify` semantics, exit codes, SMT-LIB output, and `IntentReport` aggregation are all unchanged.

## Precedent

| Language / tool | How they surface per-clause position |
|---|---|
| Dafny | `file.dfy(line,col): Error: A postcondition might not hold on this return path.` Per failing VC. |
| SPARK Ada / `gnatprove` | `<file>:<line>:<col>: medium: postcondition might fail` per VC; severity classification per clause. |
| F*, Lean, Coq | Positions first-class in goal state and error context. |
| Liquid Haskell | Per-refinement positions. |
| Eiffel | Assertion-failure message includes the source line of the clause. |
| C# Code Contracts (legacy MSR) | Per-contract positions in static-analyzer output. |
| Microsoft Verifying C compiler (Boogie/VCC) | Per-assertion source positions surfaced in IDE integration. |

Per-clause source positions are table stakes in this category. Intent's anchor-at-(1,1) is the outlier; this ADR fixes it.

## Consequences

**Accepted trade-offs:**

- Memory: two extra `int` fields per IR contract clause. Negligible.
- API: `VerifyResult` grows two fields. Internal API; no external consumer.
- The single-position convention means the editor underline is 1 character wide, not a full range. Matches existing parser / checker diagnostics; range-based anchoring is a project-wide future cleanup, not specific to verify.

**Things this enables:**

- LSP shows the failing verify diagnostic underneath the actual clause. Cursor on the underline shows hover with the contract message.
- Future cmd-line output (`file:line:col: verify: <clause>`) is a one-PRD change once anyone wants it.
- Other tooling that consumes `VerifyResult` (CI reporters, dashboards, future ADR's `--strip-contracts=verified` mode) all inherit positions for free.

**Out of scope (separate ADRs/PRDs if anyone wants them):**

- Range-based anchoring across all diagnostic sources (parser, checker, lint, verify).
- `intentc verify` console output with `file:line:col:` prefixes.
- Persisted `.verify.json` cache (touched in ADR 0033's future-work section).
- Quick-fix code actions on verify diagnostics (rename a precondition, add a missing one).
