# Pickup Notes — 2026-06-26 (Phase 45 COMPLETE: self-hosted checker, first slice)

## Where we are

**Phase 45 (Self-Hosted Checker — first slice) — COMPLETE.** The first *compiler*
subsystem is now self-hosted: `selfhost/checker/` reuses the `../shared/` front-end and
is **byte-equal with stage1 `intentc check`** across the examples corpus + fixtures —
`make diff-checker` → **34/34 PASS** (22 valid examples produce zero errors = no false
positives; 12 invalid fixtures match byte-for-byte). Wired as `intentc check
--self-hosted`. ~150 in-language checker tests. ADR 0052.

The self-hosted toolchain now has three tools, all byte-equal with their stage1
counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45, diff-checker 34/34)
```

### Implemented checker checks (first slice — NO type inference)
duplicate decl (entity/enum/function/trait), duplicate enum variant, break/continue
outside loop, return-in-test, undeclared-variable + variable-redefinition (Array-based
scope stack), function/variant call arity.

### Two front-end prerequisites landed this phase (gap-driven, HARNESS.md §7)
- **break/continue statements** (45.4): stage2 had lexed them as identifiers; now real
  `st_break`/`st_continue` (kw + ast + parser + formatter). error_handling.intent stays
  byte-equal.
- **Expr source positions** (45.7): added `line`/`column` to `Expr` (populated at the
  ex_ident sites) so `undeclared variable` anchors at the identifier. Additive — the
  formatter ignores it; selfcheck stayed 4 EQUAL.

## Next: Phase 46 — Checker type-inference foundation (the big one)

The remaining ~140 checker diagnostics are type-inference-heavy and blocked on a
**structured type representation** the stage2 AST doesn't have yet (types are flat
`String`s; `Expr` has no type). The natural next phase:

1. **Structured types** — a `TypeRef`/`Type` entity (name + type-args tree) parsed from
   the type strings (or carried by the parser), so `Array<Map<K,V>>` is a tree not a
   string. ADR-worthy. This is the gating prerequisite for everything below.
2. **Expression type inference** — infer types for every `Expr` kind (literals, idents
   via scope-with-types, binops, calls, field/index, match, etc.), storing the result
   (the Go checker keeps `exprTypes`). Needs the scope to carry types, not just names.
3. **Type-rule checks** — assignability (`type mismatch`), operator typing, condition-
   must-be-boolean, argument-type mismatch, return-type, generic instantiation, match
   exhaustiveness + arm-type consistency, contract well-typedness, etc. Each is a
   `make diff-checker` fixture (and many need new invalid fixtures + corpus
   no-false-positive coverage).

Also still open (smaller, independent): **method-call arity** and **builtin-call arity**
(deferred from Phase 45), and the stage2 **extern `from "path"` / trait-method
contract** parser gaps (would let the linter's R3/R4 be differentially gated too).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md`.
2. `continue norman` finds nothing queued — scope Phase 46 (start with the structured
   `Type`/`TypeRef` representation — it gates all type-inference checks), write its ADR,
   add TASKS.md rows, then proceed. The checker lives in `selfhost/checker/`.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker`.
