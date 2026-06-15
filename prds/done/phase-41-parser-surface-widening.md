# Phase 41: Stage2 Parser Surface Widening

**Status:** Shipped (2026-06-15)
**Milestone:** Self-hosting (stage2 toolchain in `selfhost/formatter/`)
**Decision:** extends ADR 0040 (self-hosted formatter strategy)

## Goal

The stage2 parser handles a restricted subset of Intent — enough to parse and
byte-equal self-format its own source (Phase 40). To format *arbitrary* Intent
(and eventually replace the Go formatter at parity), it must handle the
constructs it currently skips or rejects. Widen the surface, one construct at a
time, each round-tripping through parse + format.

## Success Criteria (per sub-feature: parse → AST → format → round-trip test)

- [x] **41.1 Contracts** — `requires` / `ensures` / `decreases` clauses on
  functions and methods round-trip. Canonical layout (matches stage1):
  signature line, each clause on its own indented line (no `;`), then `{` on its
  own line. `result` inside an ensures expr round-trips as an identifier.
  (Were silently discarded before this phase.)
- [x] 41.2 `match` expressions over Result/Option round-trip.
- [x] 41.3 `for ... in ...` loops round-trip.
- [x] 41.4 `try ?` operator round-trips.
- [x] No regression: stage2 self-format stays byte-equal; full suite green on rust + js (170/170).

## Non-Goals

- Semantic checking of contracts/patterns (formatter only needs syntax round-trip).
- Replacing the Go formatter in the CLI (separate phase).

## Notes

Contracts come through as plain `tk_ident` ("requires"/"ensures"/"decreases"),
not lexer keywords; `parse_expr` stops naturally at the next clause keyword or
`{` (no `;` terminator in the surface grammar). Canonical reference:
`examples/fibonacci.intent`.
