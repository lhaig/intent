# 0054: Additive AST position fields for checker diagnostics

**Date:** 2026-07-02
**Status:** accepted
**Phase:** 46.4b — self-hosted checker `unknown type` over the remaining annotation sites

## Context

[ADR 0053](0053-self-hosted-checker-type-foundation.md) D1 kept the type
**representation** front-end-free: the self-hosted checker parses the type *strings*
the AST already carries (`parse_type`) rather than enriching the parser/AST with
structured `TypeRef` nodes, because a structured representation would force the
**formatter** to reconstruct the exact type string from the tree, risking the byte-equal
self-format + `diff-formatter` gates. ADR 0053 also claimed "zero front-end churn."

ADR 0053 D2 scopes the `unknown type 'X'` check over **every** type annotation —
param / **field** / return / `let`. To be byte-equal with stage1 (`make diff-checker`),
each diagnostic must anchor at the **same source position** stage1 uses. Most stage2 AST
nodes already carry positions: `Param.line/column` (parser populates from the param
name token), `FunctionDecl.line/column`, `Stmt.line/column`. But `FieldDecl` does **not**.
Stage1 anchors an entity field's `unknown type` at `FieldDecl.Pos()`, which is the
`field` **keyword** token (`internal/parser/parser.go` `parseFieldDecl`). Without a
position on stage2's `FieldDecl`, the self-hosted checker cannot reproduce that anchor,
so entity-field `unknown type` diagnostics cannot be made byte-equal.

Phase 46.4a shipped function param + return `unknown type` (positions already present).
The remaining sites (entity fields especially) forced the question this ADR settles:
*may the stage2 front-end gain position fields, given ADR 0053 D1's "no front-end change"?*

## Decision

**Position fields (`line`, `column`) may be added to a stage2 AST node additively when a
checker diagnostic needs to anchor there** — populated by the parser, defaulted to 0,
with **no constructor-signature change** (assigned as fields after construction).

This is **distinct** from ADR 0053 D1's constraint, and does not violate it:

- D1 forbids adding **structured types** to the AST (`String` type fields → a `TypeRef`
  entity). That change is visible to the **formatter**, which would have to reconstruct
  the exact type string from the tree — the byte-equal-self-format risk D1 avoids.
- Position integers are **inert to the formatter**: it never reads or emits them.
  Adding them cannot change formatter output, so `selfcheck-formatter` (byte-equal
  self-format) and `diff-formatter` are unaffected.

**Precedent:** Phase 45.7 already added `line`/`column` to `Expr` (populated at the
`ex_ident` sites) so the `undeclared variable` diagnostic could anchor at the identifier
— under exactly this reasoning, and selfcheck stayed EQUAL. ADR 0054 promotes that
one-off to a stated policy.

**For 46.4b specifically:** `FieldDecl` gains `line`/`column`, populated from the
`field` keyword token (matching stage1 `FieldDecl.Pos()`), enabling a byte-equal
entity-field `unknown type` diagnostic. Any node that later needs diagnostic anchoring
follows the same additive pattern.

## Consequences

### Benefits
- Entity-field (and any future node-anchored) diagnostics can be byte-equal with stage1
  — unblocking the "field" half of ADR 0053 D2 that positions were the only thing
  gating.
- The formatter/linter gates stay green: positions are inert to formatting, so no
  reconstruction risk (unlike structured types).
- Minimal, well-precedented churn (the 45.7 pattern), reversible, and constructor
  call-sites are untouched.

### Costs / refinement
- This **refines** ADR 0053's "zero front-end churn" wording: the type *representation*
  stays front-end-free, but diagnostic *anchoring* may require additive position fields.
  The dividing line is the formatter — representation changes affect it, position
  additions do not.
- Every node that gains positions must keep `selfcheck-formatter` + `diff-formatter`
  green (the forcing function that positions really are inert).

### Non-goals
- Adding structured type nodes to the AST — still avoided (ADR 0053 D1 stands).
- Adding positions to nodes that need no diagnostic anchor (add them when a diagnostic
  demands it, not speculatively).

## References
- [ADR 0053](0053-self-hosted-checker-type-foundation.md) — type-representation foundation (D1/D2 refined here).
- [ADR 0044](0044-stage2-comment-preservation.md) — prior additive AST fields (comments) with the same formatter-inertness argument.
- Phase 45.7 — `Expr` line/column for the `undeclared variable` anchor (the precedent).
- `internal/parser/parser.go` `parseFieldDecl` — stage1 anchors `FieldDecl.Pos()` at the `field` keyword.
- PRD: `prds/active/prd-phase-46-checker-type-foundation.md` (US-004).
