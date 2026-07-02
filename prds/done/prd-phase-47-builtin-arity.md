# PRD — Phase 47: Self-Hosted Checker Builtin-Call Arity

## 1. Introduction / Overview

Phase 45 shipped user function + variant call-arity in the self-hosted checker but
deferred **builtin-call arity** (~23 builtins, each with a bespoke stage1 message).
Phase 47 closes that gap: arity (argument count) only — the per-argument *type* checks
need the expression-inference engine and are deferred to Phase 48. Strategic frame:
[ADR 0055](../../docs/decisions/0055-self-hosted-builtin-arity.md).

## 2. Goals

- Arity-check all 23 stage1 builtins against a name→count table.
- Emit stage1's exact message in its three shapes ("expects N" / "requires exactly N" /
  "takes no arguments"), `argument` singular at N=1.
- Byte-equal via `make diff-checker`; the 22 valid examples stay clean.

## 3. Design decisions

Per ADR 0055: D1 arity only (defer argument typing + async-context to Phase 48; the
omissions are corpus-invisible false negatives); D2 a parallel name/count table reusing
`arity_lookup`, with the three verbatim message shapes; D3 checked first in
`check_expr_names` (stage1 builtin-before-variant/function precedence), anchored at the
callee, early-return on mismatch; D4 gated by `make diff-checker`.

## 4. Tasks

### US-001 (47.1): ADR 0055 — builtin-arity strategy
**AC:** `docs/decisions/0055-...md` records D1-D4. Done.

### US-002 (47.2): builtin-arity table + message helper
**AC:** `builtin_arity_names`/`builtin_arity_counts` (23 entries), `builtin_arity(name)`
(−1 if absent), `builtin_uses_expects_verb`, `builtin_arity_message(name, got)` producing
the three shapes with correct singular/plural. Done.

### US-003 (47.3): wire into `check_expr_names`
**AC:** builtin arity checked first in the `ex_call`/`ex_ident`-callee branch; emit +
return on mismatch (no arg recursion); recurse args + return on match. Anchored at the
callee. Done.

### US-004 (47.4): fixtures + tests + no-false-positives
**AC:** one fixture per message shape byte-equal vs stage1; 22 valid examples stay clean;
in-language tests incl. plural (`assert_eq`) and the early-return case. `make diff-checker`
44/44; 188 checker tests. Done.

### US-005 (47.5): docs + validate + push
**AC:** ROADMAP + NEXT-STEPS + checker README + this PRD; `make validate` + all gates
green; commit + push. Done.

## 5. Non-Goals

- Builtin **argument typing** (`print() cannot print type …`, `assert() argument must be
  Bool`, `await_*` async-context) — needs expression inference (Phase 48).
- Method-call arity (needs the receiver type — Phase 48).
- `Ok`/`Err`/`Some`/`None` (variant-constructor path, unchanged).

## 6. Success Metrics

`make diff-checker` 44/44 (22 examples clean + 22 fixtures byte-equal, incl. one per
message shape). All formatter/linter/self-check gates + full Go suite + `make validate`
green.
