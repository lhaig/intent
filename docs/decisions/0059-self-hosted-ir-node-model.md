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

## Update — array-by-ref params + call-site borrow + method calls (diff-emit 11/11)

`examples/array_sum.intent` self-hosts byte-equal, adding a 5th real example. Three
sub-changes and one structural decision:

- **Array/Map params emit `&Vec`/`&HashMap`** and the call site borrows the matching arg
  (`sum_array(&arr)`), mirroring `generateFunction`'s param loop and the `g.functions`
  arg-borrow lookup in `rustbe.go`. `param_is_arrayref(funcs, name, idx)` is the stage2
  registry lookup.
- **Method calls are a new AST path, not a new AST node.** The stage2 AST has no
  `MethodCallExpr`: `x.push(v)` parses as `ex_call(children=[ex_field(object=x, name=push), v])`.
  Lowering detects the `ex_field` callee and produces a new IR kind `irex_method`
  (name=method, children=[receiver, args…], `call_kind` = method); the backend emits the
  general `receiver.method(args)` path. Module-qualified calls (`mod.fn()`) look identical in
  the AST but the single-file emit corpus has none, so an `ex_field` callee is always a
  genuine method call here — the module-call split lands with multi-file emit.
- **The backend stays free functions, threading `funcs` as a parameter — NOT a generator
  entity.** The call-site borrow needs the callee's param types at the call site, which
  stage1 holds as `g.functions` on the `generator` struct. Modelling that as a `RustGenerator`
  entity was tried and rejected: stage1's method-receiver inference (`methodMutatesSelf`) is
  **shallow** — a method whose statement is a bare `return self.foo()` is marked `&mut self`,
  but one that buries `self.foo()` inside a string concat (`return "(" + self.foo() + ")"`)
  stays `&self`, and a `&self` method may not call a `&mut self` one, so the emitted stage2
  binary fails to compile. Existing self-hosted entities (parser, checker) escape this only
  because they genuinely mutate `self` (e.g. `self.pos`), making every method uniformly
  `&mut self`. A non-mutating generator does not, so free functions with a threaded `funcs`
  parameter are the robust choice (and the working style of the earlier slices). Future
  generator state (Map's `needsHashMap`, entity/enum tables, async's `needsTokio`) threads the
  same way, or waits for a genuinely-mutating generator entity if one becomes warranted.

## Update — unary operators + paren transparency (diff-emit 13/13)

`examples/verify_example.intent` self-hosts byte-equal (6th real example).

- **Unary** is a new IR kind `irex_unary` (op text in `name`, `children[0]` the operand);
  the backend emits `!X` / `-X` with no added parens, exactly as `generateExpr`'s `UnaryExpr`
  case does — the operand self-parenthesises when it is a binary node.
- **Parens are unwrapped in lowering, not modelled in the IR.** The Go AST has **no** paren
  node — `internal/parser` discards parentheses at parse time, so stage1's output parens come
  solely from binary self-parenthesisation. The stage2 AST, by contrast, keeps `ex_paren`
  (the formatter needs it to round-trip source). Lowering therefore unwraps `ex_paren`
  transparently to `lower_expr(children[0])` — the same thing the checker does (recurse
  `children[0]`) — producing IR identical to stage1's and thus byte-equal Rust. This is the
  right call: the IR is a *compilation* representation (parens carry no semantics once
  precedence is resolved), whereas the formatter's AST is a *source* representation. The
  existing corpus had simply never used an explicit source paren; a unary fixture surfaced it.

## Update — the D3 type bridge: local type reconstruction + cloneIfNeeded (diff-emit 19/19)

`examples/try_operator.intent` self-hosts byte-equal — a TENTH real example — by
building the first piece of the D3 `checker.Type -> IrType` bridge that this ADR
deferred. It diverged only on `parse_number(a.clone())` / `parse_number(b.clone())`:
stage1 clones a non-Copy (`String`) argument so it is not moved out of its owner,
but the stage2 IR carried no argument types for `cloneIfNeeded` to consult.

- **D3 resolved for local variable types (not yet full inference).** Rather than
  thread the checker's `CheckResult`, lowering reconstructs the *only* types the
  backend needs here — a `VarRef`'s type — from a new `LowerScope` (parallel
  name/type arrays) threaded through `lower_block`/`lower_stmt`/`lower_expr`. It is
  seeded from the function's params (`name -> param_type`) and grows with each
  `let` binding (`name -> let_type`); the `ex_ident` case stamps `VarRef.expr_type`
  from it. The scope is **functional/immutable** (each `lower_scope_define` copies
  the arrays and returns a new scope), mirroring the checker's `Scope` — which also
  sidesteps gotcha (c): the copied arrays are local owned arrays, so an `Array`
  param's `&Vec` type still moves into the entity field. This is a deliberately
  *narrow* bridge: for-loop / lambda-param / match-arm bindings are left unstamped
  (their VarRefs get the empty/unknown type), landing with the slice that needs one
  of them cloned.
- **`ir_is_copy_type` (ir.intent) mirrors `isCopyType`.** `Int`/`Float`/`Bool`/
  `Void`/`Fn` and the **empty/unknown** name are Copy; everything else is non-Copy.
  Unknown -> Copy is the conservative choice (stage1's `nil -> true`): lowering
  never over-clones a type it could not reconstruct. `Future` (non-Copy but excluded
  from cloning via stage1's separate `isFutureType`) lands with the async slice.
- **`clone_if_needed` (rustbe.intent) mirrors `cloneIfNeeded`.** Skips literals,
  already-`&`-borrowed args, and already-`.clone()`d args; else appends `.clone()`
  to a `VarRef`/`IndexExpr` whose stamped `expr_type` is non-Copy. Applied at the
  exact stage1 call sites reachable today: `generate_call` (after the `&`-borrow, so
  a borrowed `Array` arg is left alone), and `generate_builtin_call` for `assert_eq`
  (both operands) and `Ok`/`Err`/`Some` (the single operand). The IO builtins
  (`read_file`/`write_file`/…) get their `clone_if_needed` when their emit slice lands.
- **Match-scrutinee `.clone()` left as-is.** It is still inferred from builtin-pattern
  arms (gotcha (e)); switching it to the now-available type check was optional and not
  needed for byte-equality, so it was deferred to avoid churn (19/19 either way).

Gates green throughout: `diff-emit` 19/19, `selfcheck-formatter` 7/7, `selfcheck-checker`
13/13, `ir_test` 17, `lower_test` grown (124), `intentc test examples/try_operator.intent`
2/2 (cargo). No Go / `shared/*` touched. **Follow-ups this unblocks** (each needs MORE
than the bridge): `error_handling` (StringConcat -> `format!`, `continue`/`break`);
`io_demo` (IO builtins' `std::fs` emit, Float literals, and the `mutatedVars` analysis
that makes method-call receivers `let mut`).

## Update — entities: structs + impl + constructor + methods + invariants + old() + intents

`examples/bank_account.intent` self-hosts byte-equal (diff-emit 24/24). This is the
largest single-file slice: an entity lowers to a `#[derive(Clone, Debug)] struct` +
`impl` block (`__check_invariants`, `fn new`, methods), with contract asserts, `old()`
captures, and the `intent` block emitted as doc-comments + a compile-time-verified mod.

- **Shared enrichment (touches `shared/*`).** Invariants had no raw text on the AST
  (unlike `requires`/`ensures`, enriched earlier), so `EntityDecl.invariants_raw`
  was added (additive, ADR 0054) and populated in `parse_entity_decl`. Constructors
  and methods parse via `parse_constructor_decl`/`parse_method_decl`, which — unlike
  `parse_function_decl` — did **not** capture `requires_raw`/`ensures_raw`; that was
  added too (a latent gap that only surfaced once the compiler consumed the raw text).
  All shared gates (`diff-checker`, `diff-formatter`, `diff-linter`, `go test`,
  `validate`, `selfcheck-*`) stay green — the new fields/method are inert to the
  checker/formatter/linter.
- **self / result / old modelled as synthetic var-refs, not new IR nodes.** The
  stage2 AST has no `ex_self`/`ex_old`/`ex_result` — `self` and `result` are
  `ex_ident`s and `old(x)` is an `ex_call` on `"old"`. So `self` lowers to
  `ir_var_ref(scope.self_name)` ("self" in methods, "__self" in a constructor, where
  stage1 emits the struct being built), `result` to `__result`, and `old(x)` (in an
  `in_old` ensures scope) to `ir_var_ref(<mangled>)` with the capture hoisted at
  method start. Only `irex_field` is a genuinely new node. This sidesteps threading
  an `inConstructor`/`ensuresContext` flag through the whole backend `generate_expr`.
- **`methodMutatesSelf` picks `&self`/`&mut self`.** Mirrored exactly (self-field
  assignment or a method call on self ⇒ `&mut self`), keeping the backend free
  functions (gotcha (b)) rather than a generator entity.
- **Enum + entity registries carried on `LowerScope`.** Constant program tables
  (enums from the previous slice, now entities) ride the already-threaded scope
  (seeded once, preserved by `lower_scope_define`), plus `self_name` and `in_old`
  flags — so only `lower_function`/`lower_test`/`lower_member` gained parameters, not
  the deep `lower_expr` chain.
- **Two reserved-word / Rust-keyword traps.** `verified_by` and `ensures` are Intent
  reserved words (IR field named `verifications`, param named `ens_preds`); a local
  named `fn` emits as the Rust keyword `fn` (renamed `func`).

## Update — Map (HashMap) + receiver-type method dispatch + the mutatedVars test-leak quirk

`examples/map_demo.intent` (diff-emit 26/26) and `examples/js_demo.intent` (a free win
from enums+entities, 25/25) self-host byte-equal.

- **Map -> HashMap**: `map_type` gains the `Map<K,V> -> HashMap<K,V>` case; an empty
  array literal typed `Map` (stamped in lowering when the `let` type is `Map`) emits
  `HashMap::new()`. The `use std::collections::HashMap;` header is injected by a
  post-generation substring scan of the body (equivalent to stage1's `needsHashMap`,
  since any Map type/literal emits the token `HashMap`).
- **Receiver-type method dispatch**: `generate_method_call` selects a rewrite from the
  RECEIVER's stamped type — Map's `get`/`set`/`contains`/`keys`/`remove` become the
  `HashMap` idioms (`.get(&k).cloned().unwrap_or(d)`, `.insert`, `.contains_key(&k)`,
  `.keys().cloned().collect()`, `.remove(&k)`). This needs field-access types, so
  lowering now stamps `irex_field.expr_type` from the owning entity's field
  declaration (the `self` entity is carried on `LowerScope.self_entity_name`) — so
  `self.settings.get(...)` dispatches as a Map. String/Char receiver methods reuse
  this machinery in a later slice.
- **Discovered stage1 quirk — `mutatedVars` leaks into tests.** `generateTest` does
  NOT reset `g.mutatedVars`, so every test body reuses the set left by the LAST
  function/method/constructor generated before the tests (generation order: entity
  ctors/methods, then functions). A test's own `let` that is never mutated is still
  emitted `let mut` if that name happened to be mutated in, e.g., the entry function.
  Reproduced exactly: `compute_leaked_mutated` returns the last such body's set and
  `generate_test` uses it for ALL tests (not the test's own). Required for byte-equal
  `let mut` decisions in test bodies (surfaced by map_demo's `cfg`).

## Update — async (spawn / await / sleep / tokio)

`examples/async_demo.intent` and `examples/task_queue.intent` self-host byte-equal
(diff-emit 28/28; task_queue was blocked only on async).

- **async functions/tests**: `async fn`; the async entry adds `#[tokio::main] async fn
  main()` that `.await`s `__intent_main`; async tests use `#[tokio::test] async fn`. The
  contract/labeled-block body is identical to the sync path.
- **spawn/await** (irex_spawn/irex_await): `tokio::spawn(call)` and `(x).await` (+
  `.expect("spawned task panicked")` for a JoinHandle). `IsJoinHandle` is carried on
  `irex_await.bool_value` and set in lowering when the operand is a spawn directly or a
  var bound to a spawn — tracked in `LowerScope.join_handle_vars` (accumulated in
  lower_block, mirroring stage1's `joinHandleVars`).
- **sleep** -> `tokio::time::sleep(std::time::Duration::from_millis(ms as u64))` (a
  function-call special case). **Future<T>** -> `tokio::task::JoinHandle<T>` (map_type;
  fully qualified, so no `use` injection).

## Update — traits + impl blocks

`examples/handler_trait.intent` self-hosts byte-equal (diff-emit 29/29). New `IrTrait`
(method sigs reuse IrFunction) emits `trait Name { fn m(&mut self, …) -> R; }`;
`IrImplBlock` emits `impl Trait for Entity { … }` with each method forced to `&mut
self` (a new `in_impl` flag on `generate_method`, mirroring stage1's `inImplBlock`).
Impl methods lower like entity methods (self.field types resolve against the impl's
entity, looked up by name). The general method-call path now clones non-Copy place
args (`start.execute(ctx.clone())`), matching stage1's cloneIfNeeded. Decl order:
entities, enums, traits, impls, functions, intents, tests.

## Update — char literals + char/String receiver methods + String indexing/slicing

`examples/char_string_demo.intent` self-hosts byte-equal (diff-emit 30/30). New
`irex_char` (lowering decodes the raw lexeme — quotes, escapes, `\u{HEX}` — to a
codepoint; backend emits `'\u{HEX}'`). Char receiver methods (`to_codepoint` ->
`((c) as u32 as i64)`, `is_digit`/`is_alpha`/… -> `is_ascii_*`, `is_whitespace` ->
the inline block) and String `len` (`.chars().count()`) via the receiver-type
dispatch; `len(s)` builtin type-checks its arg; String indexing/slicing ->
`.chars().nth(...)` / `.chars().skip().take().collect()`, with index/slice result
types stamped (String[i] -> Char, String[a..b] -> String, Array<T>[i] -> T) for
dispatch and cloning (Char is non-Copy). `char_from_codepoint` -> the
`match u32::try_from(...)` form. Match-arm bindings are typed from the scrutinee's
Result/Option args (`extend_scope_for_arm`), so `Ok(c) => c.to_codepoint()` dispatches.

## References

- [`internal/ir/nodes.go`](../../internal/ir/nodes.go) — the port target for this slice.
- [`internal/ir/lower.go`](../../internal/ir/lower.go), [`internal/rustbe/rustbe.go`](../../internal/rustbe/rustbe.go) — 55b / 55c port targets.
- [ADR 0008](0008-intermediate-representation.md) — the IR (Go).
- [ADR 0053](0053-self-hosted-checker-type-foundation.md) — the checker's `Type` model, whose shape `IrType` mirrors.
- [ADR 0054](0054-additive-ast-positions-for-diagnostics.md) — additive position fields (the `IrContract` line/column convention).
- [ADR 0058](0058-self-hosted-checker-cross-module-resolution.md) — the Phase 54 multi-file harness pattern that `--emit --self-hosted` will mirror in 55c.
- `prds/active/prd-phase-55-self-hosted-compiler.md` — the phase plan.
