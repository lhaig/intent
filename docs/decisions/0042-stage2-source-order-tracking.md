# 0042: Stage2 Source-Order Tracking for Top-Level Declarations

**Date:** 2026-06-09
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 40, sub-piece C)

## Context

Phase 36 chose a **parallel-array** shape for the stage2 `Program` entity: `imports: Array<ImportDecl>`, `functions: Array<FunctionDecl>`, `entities: Array<EntityDecl>`, etc. — one Array per top-level declaration kind. The decision was deliberate: at the time, the parser needed a clean container per kind and the formatter didn't exist yet. The PRD explicitly flagged that source-order across kinds would need to be revisited "when the formatter has concrete requirements in hand."

Phase 38 shipped a formatter MVP that emits decl kinds in a **fixed canonical order** (module → imports → functions → entities → enums → traits → impls → intent_blocks → tests → externs). `examples/hello.intent` happens to match that order so byte-equal worked, but any real program that interleaves kinds — entity, then function, then entity, then test — gets reordered by the stage2 formatter and diverges from `intentc fmt`'s output.

Self-format of `selfhost/formatter/parser.intent` is one of the inputs into the Phase 39 self-parse certification. That file interleaves dozens of free functions, entities, and helper functions across its 1740 LOC. Without source-order tracking, byte-equal self-format is structurally impossible.

This ADR records the decision of *how* to preserve source order across kinds.

## Decision

Add a `line: Int` field to every top-level declaration entity (`ImportDecl`, `FunctionDecl`, `EntityDecl`, `EnumDecl`, `TraitDecl`, `ImplDecl`, `IntentBlock`, `TestDecl`, `ExternDecl`). The parser populates it from the leading keyword token of each declaration; the formatter walks a k-way merge of the per-kind arrays sorted by this field.

`ModuleDecl` does not get a `line` field — the module declaration is always first by grammar.

## Considered alternatives

| Option | Description | Trade-off |
|---|---|---|
| **(A) Per-decl `line: Int`** [**chosen**] | Add a single Int field to each existing decl entity. Parser populates from token. Formatter merges by line. | Small, localised diff. Each decl carries a piece of metadata that doesn't affect semantics — purely there for the formatter. Future tooling (linter, LSP) can also use it. |
| **(B) Discriminated union** | Move `Program` to `Array<TopLevelDecl>` where `TopLevelDecl { kind: Int, idx: Int }` indexes into typed arrays. | Structurally cleaner — one sequence, one source-order. But requires rewriting every consumer that iterates `prog.functions` / `prog.entities` / etc. — that's ~30 call sites in `format.intent` alone, plus the parser, plus future stage2 tools. Diff is much bigger; benefit is purely aesthetic at this point. |
| **(C) Sort each per-kind array, formatter k-way merges anyway** | No new field. The parser already appends in source order so each per-kind array is sorted by source position; formatter just k-way merges by … what? There's no comparison key without the line field. | Doesn't work — we'd still need a per-decl line number to compare across arrays. |
| **(D) Always-canonical reorder** | Continue Phase 38's behavior. Document that stage2 formatter intentionally canonicalises decl order; stage1 won't match on interleaved files. | Cheapest but defeats the byte-equal self-format gate. The whole point of self-hosting is the dogfood loop; if the formatter diverges from stage1, the dogfood never closes. |

### Why (A) over (B)

(B) is what a from-scratch design would probably pick — declarations *are* a single sequence in source. But Intent's stage2 is already three files deep into the parallel-array shape, and changing the container affects:

- Every test in `parser.intent` that asserts `len(prog.functions) == N`.
- Every emitter in `format.intent` that iterates one kind at a time.
- Phase 39's self-parse certification (the stage2 parser parsing its own AST entities — they'd need updating).
- Any future stage2 tool (linter, etc.) that walks decls.

(A) adds one field to nine entities. The diff is mechanical, the call-site impact is zero outside parser and formatter, and (B) remains available as a future refactor once the stage2 surface stabilises.

### Why (D) was tempting but wrong

(D) is the "ship and move on" answer: declare canonical decl order an Intent convention, fix examples to match, and call it a day. The problem: the formatter exists to support `intentc fmt`-style **non-invasive** formatting. Reorder is invasive. A formatter that reorders user code is one users won't run.

## Consequences

### Positive

- Self-format on stage2 files (and any future Intent program) preserves source order.
- The `line: Int` field is reusable: a future linter / LSP can use it for diagnostics, jump-to-definition, etc.
- Diff is small and localised.

### Negative

- Every decl entity gets an extra field. The constructor signature widens; helper constructors (`empty_function_decl`, etc.) need updating.
- The formatter's `format_program` becomes a k-way merge instead of nine sequential loops — slightly more complex code, though still O(N) with K=9 a constant factor.

### Neutral

- The parallel-array `Program` shape persists. (B) remains available if the stage2 surface ever needs the deeper restructure.

## Out of scope

- Sub-line (column) ordering for declarations on the same line. Intent's grammar requires `;` or `}` between declarations, so two decls on the same line are vanishingly rare; line-only ordering is sufficient.
- Source-order tracking *inside* declarations (e.g. method order within an entity). The parser already appends to `entity.methods` in source order, and the formatter iterates in that order — no change needed.
- Line tracking for statements / expressions. Out of scope; the formatter renders bodies from AST structure, and stage1 fmt does the same.

## Implementation notes

- Parser: each `parse_*_decl` method captures `self.peek().line` (or the leading token's line if already advanced) before consuming the keyword, then passes it into the constructor as the last arg.
- Formatter: `format_program` builds a sorted iteration order across the nine arrays. K-way merge by `line` (each per-kind array is already sorted by source position since the parser appends in order, so a single pass over the arrays with nine index pointers and a min-by-line pick at each step suffices).
- Empty/placeholder helpers (`empty_function_decl`, etc.) default `line: Int = 0`.

## Reference

- ADR 0040 — strategic frame for self-hosted formatter
- Phase 36 PRD — original parallel-array `Program` decision
- Phase 38 PRD — fixed canonical decl order (this ADR replaces)
- Phase 39 PRD — self-parse certification
