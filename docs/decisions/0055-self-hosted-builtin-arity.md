# 0055: Self-Hosted Checker — Builtin-Call Arity

**Date:** 2026-07-02
**Status:** accepted
**Phase:** 47 — self-hosted builtin-call arity (deferred from Phase 45)

## Context

Phase 45 shipped user function + variant call-arity in the self-hosted checker but
**deferred builtin-call arity** — stage1 checks ~23 builtins (`print`, `assert`, `len`,
`read_file`, `sleep`, …) in `checkCallExpr` (`internal/checker/checker.go:1655-2028`),
each with a **bespoke message** and, beyond arity, **argument-type checks** that need
type inference (which the self-hosted checker does not yet have). Currently a builtin
call passes name resolution (builtins are seeded into the global scope) but is not
arity-checked — an explicit gate hole vs stage1.

Full expression type inference is the larger Phase 48. Builtin **arity** (argument count)
is separable from builtin **argument typing** and needs no inference, so it is a bounded,
low-risk parity win to land first.

## Decision

### D1 — Arity only; defer argument typing to Phase 48
Check each builtin call's **argument count** against stage1's expected count and emit the
exact arity message on mismatch. The per-argument **type** checks (`print() cannot print
type …`, `assert() argument must be Bool`, `len() requires Array, Map, or String`, the
`await_*` async-context checks, etc.) are **deferred** — they need the expression
inference engine (Phase 48). This is gate-safe: the omitted checks are false *negatives*
(errors stage1 catches that we don't), which never fire on the valid corpus (so no
`diff-checker` false positives) and are simply not exercised by fixtures until Phase 48.

### D2 — A table matching stage1's three message shapes
The 23 builtins map to an expected count and one of three verbatim message shapes:
- `NAME() expects N argument(s), got G` — the assert family (`print`, `assert`,
  `assert_eq`, `assert_close`, `assert_panics`).
- `NAME() requires exactly N argument(s), got G` — `char_from_codepoint`, `len`,
  `read_file`, `write_file`, `create_dir`, `file_exists`, `env_get`, `http_post`,
  `http_get`, `json_get`, `json_path`, `emit_event`, `sleep`, `await_all`, `await_any`,
  `timeout`.
- `NAME() takes no arguments, got G` — the zero-arg builtins (`timestamp_ms`, `args`).

`argument` is singular for N == 1, else `arguments`. The result/option constructors
(`Ok`/`Err`/`Some`/`None`) are NOT here — stage1 treats them as variant constructors
(handled by the existing variant path). Encoded as parallel name/count arrays reusing
`arity_lookup` (−1 = not a builtin; 0 is a valid count).

### D3 — Builtins first, anchored at the callee
Stage1 `checkCallExpr` handles builtins (1655) before user variant (2034) and function
(2167) resolution, so the self-hosted `check_expr_names` checks the builtin table FIRST
in the `ex_call`/`ex_ident`-callee branch — matching stage1 precedence if a name ever
collides with a user decl. The diagnostic anchors at the callee position
(`callee.line/column`), which is byte-equal with stage1's `expr.Pos()` (same anchor the
existing function-arity check already uses). On mismatch: emit and return (no arg
recursion); on match: recurse args for undeclared-variable, then return (builtins never
fall through to the variant/function checks).

### D4 — Gate: `make diff-checker` (unchanged shape)
One minimized fixture per message shape, byte-equal vs stage1; the 22 valid examples stay
clean (they call builtins with correct arity).

## Consequences

### Benefits
- Closes a Phase-45 gate hole with no dependency on the (larger, riskier) inference
  engine — a bounded, fully byte-equal-testable parity step.
- The builtin table is the natural home for the Phase-48 argument-type checks to hang off.

### Costs
- Deferred builtin argument-type + async-context checks remain false negatives until
  Phase 48 (documented; not corpus-visible).
- Message shapes are duplicated from stage1 by hand (the differential keeps them honest).

### Non-goals
- Builtin argument typing, `await_*` async-context, and all expression inference — Phase 48.
- Method-call arity (needs the receiver type — a first slice of inference).

## References
- [ADR 0053](0053-self-hosted-checker-type-foundation.md) / [ADR 0052](0052-self-hosted-checker-strategy.md) — checker foundation / strategy.
- `internal/checker/checker.go:1655-2028` — the stage1 builtin handling being ported (arity portion).
- `selfhost/checker/check.intent` — `check_expr_names` (the `ex_call` arity site) + the arity registries.
