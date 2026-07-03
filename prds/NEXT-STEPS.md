# Pickup Notes — 2026-07-03 (Phase 48 IN PROGRESS: method-call typing shipped)

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + SEVEN CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a** the inference engine
(literals + operator result types), **48b** `if/while condition must be boolean`, **48c**
`let` type-mismatch, **48d** the type-carrying scope (params typed), **48d+** let-var
binding, **48f** function arg-type, **48g** variant arg-type, **48h** assignment mismatch,
**48i.1** `self` + field-access inference, and **48i.2** method-call arity + arg-types.
`make diff-checker` → **55/55**, **220** checker tests. Full stage1 type-system parity is a
large, open-ended goal; the rest (48e onward) is the continuation below.

**Phase 47 (builtin-call arity) — COMPLETE** (prior): all 23 builtins arity-checked, three
message shapes, byte-equal. ADR 0055.

**Phase 46 (type foundation + `unknown type`) — COMPLETE** (prior): a structured `Type` +
`parse_type` + `type_is_known` resolver, and `unknown type 'X'` across every corpus
annotation site. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-48, diff-checker 55/55)
```

## What 48i.2 shipped (this session)

- **fld-pos front-end** (033e7dd, ADR 0054): the parser now sets `ex_field`'s line/column
  to the field/method-name token (matches stage1 `FieldAccessExpr`/`MethodCallExpr.Pos()`,
  was 0). Additive/inert to the formatter + linter; diff-checker unchanged. Same pattern as
  `lit-pos` (c97d936). Prerequisite so method-call diagnostics anchor at the method name.
- **48i.2 method-call check** (587f084): `check_expr_names`' ex_call-with-ex_field-callee
  branch infers the receiver via `infer_expr_type`; when it is confidently a known USER
  entity whose method resolves to **exactly one** declared method (entity body OR trait
  impl, via the new `entity_method_decls` — stage1 merges impl methods into Entity.Methods),
  it ports stage1 `checkMethodCallExpr`'s user-entity path: `method 'M' expects N arguments,
  got A` at the method name (early return), then `argument i to method 'M': expected X, got
  Y` at each arg. Arg-types only for **non-generic** entities (type-param skip, like the fn
  path). Sound skips: unknown/primitive/collection receivers (builtin Array/Map/String/Char
  deferred), unresolved/ambiguous names (`no method` NOT emitted — trait methods live in
  impls stage2 doesn't fully model). New helpers `find_entity_index`, `entity_method_decls`.

## Next: Phase 48e — operator-typing, then the rest

Remaining, in rough value order:

- **48e (recommended next) — operator-typing** (`operator '+' not defined for X and Y`,
  `requires boolean operands`): needs an **ex_binop-positions front-end change** — the
  parser must set `Expr.line/column` at binop nodes (currently 0; stage1 anchors at the
  OPERATOR token `op.Line`). Same additive pattern as fld-pos/lit-pos. Then port
  `checkBinaryExpr`'s messages; emit only when BOTH operands are confidently known and the
  operator is invalid for them. Low real-bug value but a clean, well-scoped slice.
- **48j — match-arm consistency / exhaustiveness, contract well-typedness**: complex; needs
  match + contract inference. Plus the **builtin argument typing** + `await_*` async-context
  deferred from Phase 47 (hang off the builtin table).
- **method-call RETURN-type inference**: `infer_expr_type` on a method call still returns
  Unknown; typing it needs generic type-param substitution (stage1 resolves the return type
  through the receiver's type args). Would unlock `let x: T = obj.method()` mismatch etc.
- **phase-53 gaps** (independent): **extern param/return `unknown type`** (0 corpus usage;
  stage2 parses `ExternDecl`), **generic-entity-instantiation arity**, and the stage2
  **trait-method contract** parser gap.

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Known deferred (need new machinery): immutable-target assignment/push (needs mutability
tracking in Scope).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056.
2. Continue Phase 48 at **48e** — operator-typing. First the ex_binop-positions front-end
   change (set `Expr.line/column` at binop nodes in the parser; verify formatter/linter
   inert with all gates), then the `checkBinaryExpr` diagnostics behind the diff-checker
   gate. Keep inference SOUND (Unknown skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
