# 0059: Self-Hosted Compiler — IR Node Model

**Date:** 2026-07-07
**Status:** accepted
**Phase:** 55a — self-hosted compiler, first slice (IR node model; lowering + Rust backend follow)

## Context

Three tools are self-hosted: the formatter (Phase 42), the linter (Phase 43), and the
checker (Phases 45–54, culminating in cross-module self-checking, [ADR 0058](0058-self-hosted-checker-cross-module-resolution.md)).
The compiler's back half — AST → IR (lowering, `internal/ir/lower.go`) → target source
(the Rust backend, `internal/rustbe/rustbe.go`) — is still Go-only. Reimplementing it in
Intent closes the bootstrap loop.

Per the Phase 55 plan the work proceeds in thin, byte-equal-gated slices, starting from
`examples/hello.intent`:

- **55a (this ADR)** — the IR node model for the trivial subset.
- **55b** — `lower.intent`: AST → IR for `hello.intent`.
- **55c** — `rustbe.intent`: IR → Rust, byte-equal with stage1 `build --emit`, wiring
  `build --emit --self-hosted` (mirroring the Phase 54 harness) and a `make diff-emit` gate.

This ADR records how the stage2 IR is modelled in Intent, mirroring
[`internal/ir/nodes.go`](../../internal/ir/nodes.go) for the trivial subset
(`Program`/`Module`/`Function`/`Test` + `ExprStmt`/`ReturnStmt` + call / int / string /
bool literal) — exactly what `hello.intent` lowers to. It also resolves, for the trivial
subset, the "reuse vs. reimplement the checker in the emit path" question the plan raised.

## Decision

### D1 — Tagged-entity `kind: Int` discriminator + child arrays (not sum types)

Each IR statement and expression is **one entity** with an `Int kind` discriminator plus
the fields used across the kinds it represents (`IrStmt`, `IrExpr`). This mirrors exactly
how the stage2 AST models `Stmt`/`Expr` ([ADR 0053](0053-self-hosted-checker-type-foundation.md)
context; the pattern predates it in `shared/ast.intent`) and how the lexer models `Token`.

Chosen over Intent enums-with-payloads: a sum-typed `Stmt`/`Expr` would force every field
access through a single-expression `match` arm, making the node awkward to consume in both
lowering and the backend. Mutually-recursive sizing (`IrExpr` holds `Array<IrExpr>`,
`IrFunction` holds `Array<IrStmt>`) is safe because children are reached via
heap-allocated `Array<...>` — the same reasoning `shared/ast.intent` documents. Kind-
specific fields unused by a given kind are defaulted (e.g. `call_kind`, `enum_name`),
matching the AST's `match_arms`/`lambda_params` defaulted-then-assigned pattern.

Prior art: this is the Intent-flavoured equivalent of the Go IR's interface + type-switch
(`Stmt`/`Expr` interfaces in `nodes.go`, `switch stmt := s.(type)` in `rustbe.go`). Intent
has no interfaces with downcast, so a discriminator field is the idiomatic substitute —
consistent with every other stage2 node model.

### D2 — `Ir`-prefixed entity names to coexist with the AST in a flat namespace

Every IR entity is named with an `Ir` prefix: `IrProgram`, `IrModule`, `IrFunction`,
`IrParam`, `IrContract`, `IrTest`, `IrStmt`, `IrExpr`, `IrType`.

Intent resolves **entity type references unqualified** even across imports (e.g.
`shared/parser.intent` writes `let p: Param = Param(...)` for an entity defined in
`shared/ast.intent`; only *function* calls are module-qualified, as in `shared_ast.st_let()`).
The post-merge namespace is therefore effectively flat for entity names. `lower.intent`
(55b) will import **both** `shared/ast.intent` (whose `Program`/`Expr`/`Stmt`/`Param` it
lowers *from*) and this module (which it lowers *into*), so the IR nodes cannot reuse the
AST's names — `IrProgram` vs `Program`, `IrExpr` vs `Expr`, etc. Kind constants and helpers
are likewise prefixed (`irst_*`, `irex_*`, `callkind_*`, `empty_ir_*`, `ir_*`) for the same
reason and for readability at call sites. This is the flat-namespace cost that Go avoids
for free via package qualification (`ir.Program` vs `ast.Program`).

### D3 — An independent `IrType`, keeping `ir.intent` a dependency-free leaf

The Go IR carries `*checker.Type` on every typed node. Two Intent options were weighed:

1. **Import the checker's `Type`** (`selfhost/checker/check.intent`, ADR 0053) — literal
   fidelity to `nodes.go`, no conversion in lowering.
2. **Define an independent `IrType`** in `ir.intent`, shape-identical to the checker's
   `Type` (`name: String`, `type_args: Array<IrType>`, `fn_param_count: Int`).

Chosen: **(2), an independent `IrType`**, so `ir.intent` imports **nothing** and is the
lowest, self-contained layer of the compiler — the same shape as `shared/ast.intent` and
`shared/lexer.intent` (leaf modules). Rationale:

- In Go, reusing `checker.Type` is free because package qualification isolates it. In
  Intent's flat post-merge namespace, importing the checker would pull the entire
  ~8,700-LOC front-end into the IR node model just to reference one entity, coupling the
  bottom compiler layer to the checker and bloating the `selfcheck-checker` merge for a
  file that defines no logic.
- `IrType` is shape-identical to the checker's `Type`, so a `checker.Type -> IrType`
  bridge in lowering is a trivial recursive field copy **if and when** it is needed.

This directly answers the plan's "reuse vs. reimplement the checker in the emit path"
question **for the trivial subset**: lowering assigns literal types **structurally**
(`0 -> Int`, `"..." -> String`, `true -> Bool`) — no `CheckResult` is threaded, and the
IR never touches the checker. Richer constructs (variable/field/method types, generics)
that genuinely need the checker's inferred types will introduce the `checker.Type ->
IrType` bridge in the slice that needs it; that is deliberately **deferred**, not decided
here.

### D4 — Trivial-subset scope, grown per slice

`ir.intent` models only what `hello.intent` lowers to: `IrProgram`/`IrModule`/`IrFunction`
/`IrTest`/`IrParam`/`IrContract`, `IrStmt` kinds `irst_expr`/`irst_return`, and `IrExpr`
kinds `irex_void`/`irex_int`/`irex_string`/`irex_bool`/`irex_call`. `IrModule` carries only
`functions` + `tests` (plus scalar identity fields); the other decl arrays (entities,
enums, traits, impls, intents, externs) and the remaining `IrStmt`/`IrExpr` kinds are
**additive** and land with their construct's slice. The full `CallKind` set (0–5) is
mirrored up front because it is a small closed enum the backend dispatches on.

Bare `return;` is represented as `irst_return` whose `expr` is the `irex_void` placeholder
(`ir_void_expr()`), mirroring how the AST uses `ex_void` for absent expressions rather than
a nullable field (Intent has no nil).

Note: `requires`/`ensures` are reserved keywords, so the contract-array fields on
`IrFunction` are named `requires_clauses`/`ensures_clauses`, matching the AST's
`FunctionDecl`.

## Consequences

- **Gates.** `ir.intent` is added to `selfcheck-formatter` (stage2 formatter fixpoint;
  now 5/5 EQUAL) and `selfcheck-checker` (stage1 `check` ≡ stage2 `check --self-hosted`;
  now 10/10 PASS). Because it is a leaf, `check --self-hosted` on it is fast (no large
  merge). `validate`, `diff-checker` (86/86), `diff-formatter` (22/22), `diff-linter`
  (26/26), and `go test ./...` stay green. `make diff-emit` — the byte-equal emit gate —
  arrives in 55c, so the IR is not yet exercised end-to-end here.
- **Runtime proof.** `ir_test.intent` (not gated, mirroring `check_test.intent`) builds a
  `hello`-shaped `IrModule` and checks constructors, defaulted fields, post-construction
  mutation, and nested arrays: 4/4 pass under `intentc test`.
- **Comment/format caveat.** Stage1 `intentc fmt` (in-place) strips comments and reorders;
  the *stage2* formatter preserves both. The self-hosted source is maintained as a **stage2**
  formatter fixpoint (as `selfcheck-formatter` checks) — do not run stage1 `intentc fmt`
  on these files.
- **Deferred (at 55a).** The plan's remaining decisions — `--emit --self-hosted` harness
  wiring and contract lowering — were 55c concerns; the wiring is now resolved below.

## Update — Phase 55b/55c (lowering + Rust backend, first milestone shipped)

Slices 55b (`lower.intent`) and 55c (`rustbe.intent` + `compile_main.intent` + harness
wiring) landed on the same day, closing the bootstrap loop for the trivial subset:
`examples/hello.intent` now emits Rust **byte-equal** between stage1 `intentc build --emit`
and stage2 `intentc build --emit --self-hosted`, gated by `make diff-emit` (1/1).

- **D3 confirmed.** Lowering assigns hello's literal types structurally; no `CheckResult`
  is threaded and the lowering never imports the checker. The `checker.Type -> IrType`
  bridge remains unbuilt until a construct needs inferred types.
- **Harness wiring (was plan decision #3).** `intentc build --emit --self-hosted` mirrors
  the Phase 54 checker harness ([ADR 0058](0058-self-hosted-checker-cross-module-resolution.md)):
  `stage2CompilerBinary()` builds/caches the stage2 compiler from
  `selfhost/compiler/compile_main.intent` (env override `INTENT_STAGE2_COMPILE`, staleness
  scan of `selfhost/compiler` + `selfhost/shared`). `compile_main` prints the generated Rust
  to stdout; the harness strips the single trailing newline `print()` appends and writes
  `<base>.rs`. Single-file, Rust-target only for now (multi-file `LowerAll`/`GenerateAll`
  and other targets are later milestones).
- **Two Intent lexer constraints shaped the backend.** A literal `{`/`}` in a string lexes
  as string interpolation, so every emitted brace goes through `lbrace()`/`rbrace()`
  (char-to-string); the `\{` escape is *not* usable (it stays literal backslash-brace in the
  value). `sanitise_test_name` mirrors stage1's ASCII rules exactly (codepoint comparisons,
  `String.to_lowercase()` for the A–Z branch) rather than the Unicode `is_alpha` helpers.
- **Completeness over soundness.** Per the emitter discipline, unsupported constructs emit a
  loud `// unsupported: ...` marker (fails `diff-emit`) rather than silently wrong Rust; the
  corpus only holds fully-supported programs and grows per slice.
- **Contract lowering (was plan decision #4) still deferred.** The stage2 AST carries only a
  contract's predicate `Expr`, not its raw source text, which byte-equal `assert!` messages
  need — so `requires`/`ensures`/`invariant` lowering + emission is its own future slice.
  hello uses no contracts, so the empty clause arrays are correct here.

## References

- [`internal/ir/nodes.go`](../../internal/ir/nodes.go) — the port target for this slice.
- [`internal/ir/lower.go`](../../internal/ir/lower.go), [`internal/rustbe/rustbe.go`](../../internal/rustbe/rustbe.go) — 55b / 55c port targets.
- [ADR 0008](0008-intermediate-representation.md) — the IR (Go).
- [ADR 0053](0053-self-hosted-checker-type-foundation.md) — the checker's `Type` model, whose shape `IrType` mirrors.
- [ADR 0054](0054-additive-ast-positions-for-diagnostics.md) — additive position fields (the `IrContract` line/column convention).
- [ADR 0058](0058-self-hosted-checker-cross-module-resolution.md) — the Phase 54 multi-file harness pattern that `--emit --self-hosted` will mirror in 55c.
- `prds/active/prd-phase-55-self-hosted-compiler.md` — the phase plan.
