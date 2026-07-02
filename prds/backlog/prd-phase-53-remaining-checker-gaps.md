# PRD — Phase 53: Remaining Checker Gaps (extern, trait contracts, generic arity)

## 1. Introduction / Overview

Phase 53 closes the small, independent gaps tracked across earlier phases so the
self-hosted checker reaches parity with stage1 on the diagnostics the earlier phases
deliberately deferred. Each is bounded and low-risk. Strategic frame: the checker ADRs
[0052](../../docs/decisions/0052-self-hosted-checker-strategy.md)–[0056](../../docs/decisions/0056-self-hosted-expression-inference.md).

## 2. Goals / sub-items

- **Extern param/return `unknown type`** (TASKS.md 46.4b.5): stage2 parses `ExternDecl`
  (parser ~1324); emit `unknown type '<base>'` at the extern param / return positions with
  plain `type_is_known` (no type params), matching stage1 `checker.go:806–825`. 0 corpus
  usage → needs hand-written fixtures.
- **Extern well-formedness**: `extern function 'f': from path "..." must be of the form
  "crate::path::to::function"` and FFI-bridgeable-type checks (stage1 `checker.go` extern
  block ~797, 811) — port if in scope after the unknown-type part.
- **Generic-entity instantiation arity**: the Phase-46 simplification (entity type-param
  counts not threaded into `type_is_known`) — thread entity/enum type-param arity so
  `Box<Int, Int>` (wrong arity) and a non-generic entity's stray type args match stage1.
- **Trait-method contract parser gap**: the stage2 parser doesn't fully parse
  `extern from "path"` / trait-method contracts (noted in NEXT-STEPS); close the parser gap
  so the linter's R3/R4 and any trait-method-signature checks can be differentially gated.

## 3. Design decisions (record in a Phase-53 ADR if any is non-trivial)

- Do the sub-items as independent, individually-gated slices — each byte-equal via
  `make diff-checker`, each with its own fixture(s).
- Parser changes (trait-method contracts) are additive front-end work — keep the formatter
  byte-equal (ADR 0054 discipline) and re-run `selfcheck-formatter` + `diff-formatter`.
- Extern checks reuse `type_is_known` (Phase 46); generic-arity threads entity type-param
  counts into the resolver.

## 4. Tasks (indicative)

- US-1: extern param/return `unknown type` + fixtures.
- US-2: extern `from`-path + FFI-bridgeable-type checks + fixtures.
- US-3: generic-entity instantiation arity in `type_is_known` + fixtures.
- US-4: trait-method contract parser gap (additive; re-validate formatter gates).
- US-5: docs + validate + push per slice.

## 5. Non-Goals

- Anything already covered by Phases 46–52. Multi-file `CheckAll` remains a later,
  separate effort.

## 6. Success Metrics

`make diff-checker` green with new byte-equal fixtures + 22 examples clean; all
formatter/linter/self-check gates + `make validate` + full Go suite green. At completion,
the self-hosted checker matches stage1 across the corpus + fixture surface.
