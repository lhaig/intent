# Pickup Notes — 2026-07-08 (Phase 55 milestone + 13 scale-up slices DONE; NEXT = the type bridge (cloneIfNeeded) then enums+match+float — see prds/active/prd-phase-55-self-hosted-compiler.md)

## ✅ PHASE 55 SHIPPED: milestone + 13 construct slices (2026-07-07..08) — ADR 0059

The self-hosted compiler's back half exists in Intent:
`selfhost/compiler/{ir,lower,rustbe,compile_main}.intent`, wired via `intentc build --emit
--self-hosted` (harness mirrors Phase 54: `stage2CompilerBinary`, env
`INTENT_STAGE2_COMPILE`). The bootstrap loop is closed for a growing corpus: **`make
diff-emit` is 18/18 EQUAL** — NINE real examples (`hello`, `divergence_demo`, `fibonacci`,
`target_specific_demo`, `array_sum`, `verify_example`, `sorted_check`, `closure_demo`,
`result_option`) + 9 construct fixtures — plus `make selfcheck-formatter` (7/7) and `make
selfcheck-checker` (13/13). Supported: functions/params/calls, let, binops, unary
(`-`/`not`), quantifiers (forall/exists), closures/lambdas + Fn types, match +
Ok/Err/Some/None construction, `?` operator, if/while/for + assignment, strings/print,
contracts, arrays + Array/Map by-ref params + call-site borrow + method calls, parens
(unwrapped) (see the frontier list below for the exact construct set). Every slice was
verified byte-equal against stage1 before landing; all pre-existing gates stayed green
throughout.

**NEXT FRONT — scale the emitter construct-by-construct** (PRD "Then scale up"). Each slice:
grow `lower.intent` + `rustbe.intent` for one construct, add a byte-equal corpus entry to
`selfhost/compiler/diff-emit.sh`. Order (✅ = done, byte-equal-gated in diff-emit):
✅ let-bindings & locals → ✅ binops → ✅ if/while/for + assignment → ✅ functions & calls →
✅ strings & `print` → ✅ contracts (requires/ensures) → ✅ arrays (local) + the type-string
parser → ✅ array params + call-site borrow + method calls (finishes array_sum) →
✅ unary (`-`/`not`) + paren transparency (finishes verify_example) →
✅ quantifiers (forall/exists in contracts) (finishes sorted_check) →
✅ closures/lambdas + Fn types (finishes closure_demo) →
✅ match + Ok/Err/Some/None construction (finishes result_option) →
✅ `?` operator (gated via a Copy-arg fixture) →
**NEXT (foundational): the ADR 0059 D3 type bridge — stamp VarRef.expr_type from local
param/let types in lowering (scope threaded through lower_expr), then add cloneIfNeeded
(non-Copy VarRef/index/field arg -> `.clone()`) to generate_call / generate_builtin_call.
Unblocks try_operator + error_handling (`a.clone()`) and io_demo (`content.clone()`).** Then
enums + match + float (shape_area, enum_basic) → entities (bank_account) → generics
(generic_stack) → async (async_demo, task_queue) → char/float + string concat/interp
(char_string_demo) → traits (handler_trait) → Map (map_demo). Emitter must stay COMPLETE per
supported construct.

## ▶ NEXT SLICE — the ADR 0059 D3 type bridge + cloneIfNeeded (detailed plan)

`try_operator`, `error_handling`, and `io_demo` all diverge only because they pass non-Copy
VarRef args that stage1 clones (`parse_number(a.clone())`, `content.clone()`). The stage2 IR
carries no inferred types, so `cloneIfNeeded` has nothing to check. This slice adds local
type propagation. Steps:

1. **lower.intent — stamp VarRef types.** Thread a scope (var name → `IrType`) through
   `lower_block` / `lower_stmt` / `lower_expr` (and `lower_contracts` / `lower_test`), seeded
   from the function's params (`name → param_type`) and accumulating each `let` binding
   (`name → let_type`). In the `ex_ident` case, look the name up and set the VarRef's
   `expr_type`. This threads through ~25 `lower_expr` call sites — mechanical but broad; the
   gates catch errors. A small `LowerScope` entity (parallel name/type arrays, push + lookup
   methods, mirroring the checker's scope) is the natural shape. **Gotcha (c):** a function's
   `Array` param lowers to `&Vec` and cannot be moved into an owned field, so if `LowerScope`
   holds arrays, fill them from LOCAL owned arrays at the call site (as `lower_expr` already
   does for `lambda_params` / `match_arms`).
2. **ir.intent — `ir_is_copy_type(t: IrType) returns Bool`.** Mirrors stage1 `isCopyType`:
   `Int`/`Float`/`Bool`/`Void`/`Fn`/empty-name → true; else false. Empty/unknown → true so you
   never OVER-clone (conservative; only clone when the type is known non-Copy).
3. **rustbe.intent — `clone_if_needed(arg_str, arg)`.** Mirror stage1 `cloneIfNeeded`: return
   as-is if the arg is a literal, or `arg_str` already starts with `&` or already ends with
   `.clone()`; else if `arg` is a VarRef / IndexExpr (FieldAccess once entities land) whose
   `expr_type` is non-Copy, append `.clone()`. Apply it to args in `generate_call` (AFTER the
   `&`-borrow check) and in `generate_builtin_call` for `Ok`/`Err`/`Some`, `assert_eq`,
   `read_file`/`write_file`/etc. — match stage1's exact call sites.

Then add `examples/try_operator.intent` to `diff-emit.sh` (should go byte-equal — probe first).
`error_handling` additionally needs StringConcat → `format!` and `continue`/`break` statements.
`io_demo` additionally needs the IO builtins (`create_dir`/`write_file`/`read_file`/`file_exists`/
`env_get` → their `std::fs::…` emit), Float literals, and the **mutatedVars** analysis
(method-call receivers → `let mut`, e.g. `num.to_string()` makes `num` `let mut`).

**Optional cleanup once VarRefs carry types:** the match-scrutinee `.clone()` is currently
inferred from builtin-pattern arms (gotcha e); you MAY switch it to the type check to match
stage1 exactly — verify byte-equal either way, don't regress the 18/18.

## Critical gotchas (cost time to rediscover)

- **(a) Braces.** A literal `{`/`}` in an Intent string lexes as string interpolation — emit
  every Rust brace via `lbrace()`/`rbrace()`; the `\{` escape does NOT work.
- **(b) Free functions, not a generator entity.** The rustbe backend threads
  `funcs: Array<IrFunction>` as a param rather than being a `RustGenerator` entity: stage1's
  shallow `methodMutatesSelf` inference makes a non-mutating entity's methods an inconsistent
  `&self`/`&mut self` mix that fails to compile. Keep threading params.
- **(c) `&Vec` can't move into an owned field.** See step 1 above — array-field helpers take
  scalars; the caller assigns the array from a LOCAL owned array (`ir_lambda`, `ir_match`).
- **(d) match indentation is RELATIVE** (forall/exists are FIXED-indent templates). match is
  emitted only from statement-direct positions via `gen_stmt_value(e, funcs, level)` in
  `generate_stmt`; nested match → loud marker. Extending match to more positions may finally
  require threading `level` through `generate_expr`.
- **(e) match scrutinee `.clone()`** is inferred from builtin (Ok/Err/Some/None) arms, not types.
- **(f) No `st_assign`** — `x = y` is `st_expr` wrapping `ex_binop` op `"="`; `result` is
  `ex_ident "result"` → lower to `__result`; parens are `ex_paren` → unwrap to `children[0]`
  in lowering (the Go AST has no paren node).
- **(g) Never run stage1 `intentc fmt` on `selfhost/**`** — it strips comments/reorders. The
  source is maintained as a stage2 formatter fixpoint (`make selfcheck-formatter`). If that gate
  DIFFs after an edit, run the stage2 formatter binary on the file and take its output as canonical.

## Per-slice workflow / discipline

Capture stage1's exact emit first (`build --emit` in a temp dir, `od -c` for whitespace) →
mirror the relevant `lower.go`/`rustbe.go` logic in `lower.intent`/`rustbe.intent` (+ new IR
kinds/helpers in `ir.intent`) → add a fixture (or real example) → `make diff-emit` must stay
green → add `ir_test`/`lower_test` cases → run `make selfcheck-formatter` + `make
selfcheck-checker` + `make diff-emit`; **if you touch `selfhost/shared/*`** also run `make
diff-checker diff-formatter diff-linter` + `go test ./...` + `make validate` (all must stay
green) → commit (conventional, NO Claude co-author) and `git push`. The emitter must be
COMPLETE per supported construct — unsupported ones emit a loud `// unsupported:` /
`/* unsupported */` marker, never silently-wrong Rust. To probe examples, build the stage2
binary once and set `INTENT_STAGE2_COMPILE` (see how `selfhost/compiler/diff-emit.sh` does it).

_Note: `AGENTS.md` has a pre-existing uncommitted modification that is NOT part of this work —
leave it unstaged._

**Foundational win:** `ir_parse_type` (in `ir.intent`, mirrors the checker's TypeParser)
now parses the AST's flat type strings ("Array<Int>", "Map<String, Int>", "Fn(..) -> R")
into structured IrType — the ADR 0059 D3 bridge. Unblocks all generic-typed constructs.

**Array-params/method-calls slice (DONE 2026-07-08, ADR 0059 update):** (1) Array/Map params
emit `&Vec`/`&HashMap` (generate_function param loop) + call-site borrow `f(&arr)` via
`param_is_arrayref(funcs, name, idx)` (mirrors `g.functions`); (2) method calls `x.push(v)` —
the stage2 AST has NO MethodCallExpr: `x.push(v)` is `ex_call(children=[ex_field(x,push), v])`;
lowering detects the ex_field callee → new IR kind `irex_method` → backend emits
`recv.method(args)`. **Structural note:** the backend stays FREE FUNCTIONS threading `funcs`
as a param, NOT a generator entity — stage1's shallow `methodMutatesSelf` inference makes a
non-mutating entity's methods a mix of `&self`/`&mut self` that fails to compile (see ADR 0059).

**diff-emit is at 18/18** — NINE real examples (`hello`, `divergence_demo`, `fibonacci`,
`target_specific_demo`, `array_sum`, `verify_example`, `sorted_check`, `closure_demo`,
`result_option`) plus 9 fixtures (incl. `arrays`, `unary`, `quantifiers`, `try_op`)
(`let_locals`, `binops`, `control_flow`, `functions`, `strings`). Supported constructs:
`?` operator, entry+non-entry
functions & params & calls, `return`, `let`/`let mutable`, var refs, int/bool/string
literals, all binops (`(l op r)`, `implies`), unary (`-X`/`!X`), forall/exists (contract scan
blocks), closures/lambdas (`|p: T| -> R { body }`) + Fn types (`impl Fn(..) -> R`,
inferred-let), match (`Ok(v)`/`Err(e)`/`Some`/`None`/`_` patterns, scrutinee `.clone()` for
Result/Option) + Ok/Err/Some/None construction, parens (unwrapped),
if/else(+1-level else-if)/while/for-in, ranges, assignment,
`print`/`assert`/`assert_eq`/`len` builtins, scalar + Array/Result/Option type mapping,
Array/Map by-ref params + call-site borrow, method calls (general path), requires/ensures
contracts. Remaining (need arg TYPES → ADR 0059 D3 bridge): `cloneIfNeeded` for non-Copy
VarRef/field/index args (`a.clone()`), `arrayRefParams` rebinding clone, receiver-type
method paths (String/Map/Char); enum-variant match patterns (enums slice).

**Frontier — each remaining example needs a specific unbuilt slice** (probed all 22):
- **Contracts** (fibonacci, sorted_check, bank_account, array_sum, generic_stack, …) — the
  biggest unlock. `requires`/`ensures` → `assert!(pred, "Pre/Postcondition failed: <raw>")`
  plus the ensures `'body:` labeled block (`let __result: T = 'body: { … }; <post asserts>;
  __result`, with `return x` inside becoming `break 'body x`). **BLOCKER:** the `<raw>` message
  is the clause's **tokens joined by single spaces** (`parser.go:extractRawText`, e.g.
  `len ( arr ) > 0`, `n >= 0`). The stage2 AST's `FunctionDecl.requires_clauses`/
  `ensures_clauses` are `Array<Expr>` with NO raw text. So contracts need a SHARED front-end
  enrichment sub-task first: capture per-clause raw text in `shared/parser.intent` +
  `shared/ast.intent` (additive/defaulted, ADR 0054 style; the parser already has the tokens,
  join their literals with " "). This touches the fmt/lint/check self-hosting gates — verify
  selfcheck-formatter/checker + diff-* stay green. Then IrContract lowering (expr + raw_text +
  line/col) and the labeled-block emit (mind `--strip-contracts`: assert! → debug_assert!).
- **Arrays/Map**: literals, indexing (`arr[i as usize]`, String→`.chars()`), `len`
  (`(x.len() as i64)`), `Array<T>`→`&Vec<T>` params + call-site borrow, `Map`→`HashMap` + the
  `use std::collections::HashMap;` header injection (Generate's post-processing).
- **Entities**: struct + constructor (`Entity::new`, `__self`) + methods (`&self`/`&mut self`,
  `methodMutatesSelf`) + field access + invariants.
- **Enums + match**, **Result/Option/`?`** (TryExpr), **generics** (monomorphization —
  MangleGenericName, collectInstantiations), **closures/lambdas**, **async** (tokio/futures
  use-injection, spawn/await, `#[tokio::main]`), **char/float** literals, **string
  concat/interp** (StringConcat needs operand types; StringInterp → `format!`).

Each is a substantial byte-exact slice — the remaining bulk of the ~4000 LOC lower.go+rustbe.go
port. Recommended next: the contracts shared-AST-enrichment sub-task, then contract emit.

## ✅ SELF-HOSTING BLOCKER RESOLVED (Phase 54, 2026-07-03) — ADR 0058

The stage2 checker used to check a SINGLE file under `--self-hosted`, flooding false
positives on the compiler's own multi-file source (imported types → `unknown type`;
module-name call qualifiers like `shared_lexer.foo()` → `undeclared variable`; 84 on
check.intent, 461+ on parser.intent). This — valid code rejected — was the real
self-hosting blocker, invisible to the single-file `diff-checker` gate.

**Fixed:** the Go harness (`stage2CheckPaths`) reuses the module registry to discover the
import closure and passes the entry + all module paths to the stage2 binary;
`check_main.merge_programs` flattens them into one Program (cross-module dedup, entry
verbatim); `check_program_seeded` seeds imported module names so qualifiers resolve.
`check_program` stays a thin wrapper → single-file + tests byte-identical. New
`make selfcheck-checker` gate: **9/9** core self-hosting modules match stage1 (kept out of
`make validate` — stage2 is slow on big merges). See ADR 0058, prds/progress.md.

**Known non-goal (deferred):** valid-source parity only — multi-file ERROR positions/paths
and stage1's per-module import scoping are not reproduced (flat merge gives broader
visibility). Fine for the self-hosting target ("No errors found").

## Strategic state (2026-07-06) — checker milestone hit; pick the next front

The self-hosted CHECKER is essentially complete: 28 stage1 type-rule diagnostics byte-equal,
and Phase 54 made `check --self-hosted` check the compiler's own source (selfcheck-checker
9/9). Backlog PRDs 49-52 (type-carrying scope, operator/assignment typing, arg-typing/method
arity, return/match/contract typing) are **largely superseded** by the Phase 48 slices already
shipped. Three tools — fmt, lint, check — are now self-hosted and byte-equal with stage1.

**DECIDED (2026-07-07): next front is Option 1 — the self-hosted compiler.** See the kickoff
PRD: `prds/active/prd-phase-55-self-hosted-compiler.md` (read it first after any compaction).

**Phase 55 — self-hosted compiler (IR + backend):** reimplement in Intent the IR lowering
(`internal/ir` ~2,745 LOC) + the Rust backend (`internal/rustbe` ~2,420 LOC) on the existing
self-hosted front-end. Goal: `intentc build --emit --self-hosted <f>` emits Rust byte-equal
with stage1, gated by a new `make diff-emit`. **Thin first slice (start here):** 55a IR node
entities for a trivial subset → 55b lower `examples/hello.intent` → 55c emit Rust byte-equal
+ wire `--emit --self-hosted` (mirror the Phase 54 harness) + `make diff-emit` at 1/1. Then
scale construct-by-construct (let → binops → if/while → functions → entities → enums/match →
contracts → Result/Option → generics → closures → async), growing the emit corpus per slice.
~5,000+ LOC, multi-phase. Full plan, module layout, gate strategy, and ADR list in the PRD.

Deferred alternatives (lower-leverage, parked): stage2 parser gaps (extern `from`-clause,
trait-method contracts — **zero corpus usage**, extern is a syntax mismatch not a widening);
the checker error-diagnostic tail (48/53 remainder + 54b multi-file error parity —
diminishing returns, corpus-invisible). See TASKS.md rows 48j-c2 / 53 / 54b.

## Where we are

**Phase 48 (Expression type inference + type-rule checks) — FOUNDATION + 18 CHECKS
SHIPPED, IN PROGRESS.** ADR 0056: `infer_expr_type` is SOUND but INCOMPLETE (a Type only
when certain, else an Unknown sentinel); type-rule checks fire only on a confident result,
so each is corpus-safe while inference grows. Shipped + pushed: **48a-48h** (inference engine,
condition-boolean, let-mismatch, typed scope, function/variant arg-type, assignment mismatch),
**48i.1/48i.2** (self + field access, method-call arity/arg-types), **48e** binary operator
typing, **48j-b** contract well-typedness, **48j-a/48j-a2** the complete match checking (all
seven checkMatchExpr diagnostics), **48j-c/48j-c2a-c** builtin argument typing
for the uniform-type group + print + assert_close + len + assert_panics + assert_eq
(mismatch + Float), and **48j-c2d/e** the async-context checks (await_all/await_any/
timeout builtins + the `await` expression, ADR 0057 — async flag threaded on the
Scope), **48j-c2f** the assert_eq entity no-eq-method comparable-set rule, and
**53a** generic-entity-instantiation arity. `make diff-checker` → **86/86**, **278**
checker tests. Full stage1 type-system parity is a large, open-ended goal; the rest
(remaining Phase 48 gaps / phase-53) is below.

**Phase 47 (builtin-call arity) — COMPLETE**. ADR 0055.
**Phase 46 (type foundation + `unknown type`) — COMPLETE**. ADR 0053 + ADR 0054.

The self-hosted toolchain — three tools, all byte-equal with their stage1 counterparts:
```
selfhost/
  shared/    lexer · ast · parser
  formatter/ intentc fmt   --self-hosted   (Phase 42, diff-formatter 22/22)
  linter/    intentc lint  --self-hosted   (Phase 43, diff-linter 26/26)
  checker/   intentc check --self-hosted   (Phase 45-54, diff-checker 86/86 + selfcheck-checker 9/9)
```

## What 48j-c shipped (this session)

- **uniform-type builtins** (42b3f0c): `builtin_arg_type` maps each builtin whose args all
  require one simple type → that type (assert→Bool, char_from_codepoint/sleep→Int, read_file/
  write_file/create_dir/file_exists/env_get/http_post/http_get/json_get/json_path/emit_event→
  String). In the builtin arity-match path, a confidently other-typed arg → `NAME() argument
  [N ]must be T, got X` at the call (numbered iff arity>1). No-type_args + confident args only.
- **print** (bd3d22f): accepts only Int/Float/Bool/String (not Char) → `print() cannot print
  type X (accepts Int, Float, Bool, String)` (uses base .name, byte-equal for generic/entity).
- **assert_close** (87d04c3): each of the 3 args must be Float → `assert_close() argument N
  (label) must be Float, got X` (labels actual/expected/epsilon).

**Match checking (48j-a/a2) is COMPLETE** — all seven checkMatchExpr diagnostics byte-equal.
Also pushed this session: **48i.2** method calls, **48e** binary operators, **48j-b** contracts.

## Next: Phase 48j-c2 / phase-53 / Phase 54b gaps

Remaining, in rough value order:

- **Phase 54b — multi-file ERROR-diagnostic parity** (deferred non-goal of ADR 0058; the
  self-hosting VALID-source milestone is done). On INVALID multi-file input the flat merge
  diverges from stage1: (1) **wrong path** — a diagnostic in an imported module prints with
  the ENTRY's path, not the module's (repro: an `import`ed `helper.intent` with a type error
  → stage1 `helper.intent:4:5`, stage2 `app.intent:4:5`); (2) **broader visibility** — the
  flat merge makes all closure symbols visible everywhere, so a reference valid only because
  the ENTRY (not the referencing module) imports it is accepted by stage2 but flagged by
  stage1's per-module import scoping; (3) **dedup masking** — same-name cross-module decls
  dedup first-seen (no same-name/different-signature collision in the corpus today). Real
  fix: replicate stage1 `checker.CheckAll` — per-module checking with import-scoped symbol
  tables in topological order, each decl carrying its source module/path. Substantial: the
  stage2 checker moves from single-Program to a module-registry model. See TASKS.md row 54b
  and ADR 0058 non-goals. Not needed for self-hosting valid-source parity.

- **Builtin arg typing + async-context — DONE this session**: len() (48j-c2a),
  assert_panics (48j-c2b), assert_eq mismatch+Float (48j-c2c), the async-only builtins
  await_all/await_any/timeout (48j-c2d, ADR 0057 — `<name> can only be used inside async
  functions`; async flag `scope.in_async` threaded on the Scope rather than as a ~40-site
  parameter), and the **`await` EXPRESSION** async check (48j-c2e — reused scope.in_async;
  stamped the ex_await keyword position first per ADR 0054, then added the ex_await case that
  also recurses the operand, closing the latent await-operand recursion gap).
- **Remaining Phase 48 gaps** (all sound false negatives / corpus-invisible today):
  - **assert_eq comparable-set — remainder** (entity no-eq-method is DONE, 48j-c2f): the
    eq-method SIGNATURE sub-checks (wrong return / param count / param type), plus Map/Future
    rejection and generic-type-param recursion (need generic .String() rendering).
  - **async-test no-await warning** — stage1 `test "…" declared 'async' but contains no
    'await' expression` (checker.go:1009); needs testSawAwait tracking (a warning, distinct
    from the async-context errors).
  - **spawn/try operand recursion** — ex_spawn/ex_try in check_expr_names still don't recurse
    their operand (the same latent gap the await case just closed for ex_await).
  - **unary operator-typing** — `unary '-'/'not'` errors; needs ex_unary positions + tighter
    unary inference (corpus-invisible).
- **phase-53 gaps** (independent, smaller): **generic-entity-instantiation arity** is DONE
  (53a — `generic entity 'X' requires type arguments` / `entity 'X' expects N type arguments,
  got M`; the verified caveat below was applied — bare constructor-call fixtures, base name in
  message, has_constructor guard). Remaining: the entity **`has no constructor`** diagnostic
  (stage1 checker.go:2056; broader than 53a — also non-generic; currently a corpus-invisible
  divergence stage2 doesn't emit), **extern param/return `unknown type`** (0 corpus usage;
  stage2 parses `ExternDecl`), and the stage2 **trait-method contract** parser gap.
  - **CAVEAT (verified 2026-07-03, APPLIED in 53a)** for generic-entity arity: a generic
    constructor call parses as an ex_call whose callee is an ex_ident with the type args BAKED
    INTO THE NAME (`Stack<Int>()` → callee.name == "Stack<Int>"; `parse_type(callee.name)`
    splits base + type_args). Anchor at the callee position (= stage1 CallExpr.Pos() = the
    ident), and use the BASE name in the message (stage1 uses expr.Function = "Stack", not
    "Stack<Int>"). Arity checks return early, so a wrong-arity fixture emits only the arity
    error — BUT only if there is no `let`: stage1 ALSO infers the constructor's return type and
    emits a second `type mismatch: cannot assign Box to Box` for `let b: Box<Int> =
    Box<Int,String>(5)` (its Equal() compares type args; both print as "Box"). The self-hosted
    checker returns Unknown for constructor calls, so it would MISS that second diagnostic →
    fixtures must use a BARE constructor-call statement, not a let-binding, to stay byte-equal.
- **unary operator typing** (`unary '-' not defined for X`, `unary 'not' requires boolean
  operand, got X`): companion to 48e for checkUnaryExpr — needs ex_unary positions + tightening
  unary inference (currently eager Bool/operand-type). Low value, corpus-invisible.
- **method-call RETURN-type inference**: `infer_expr_type` on a method call still returns
  Unknown; typing it needs generic type-param substitution through the receiver's type args.
- **contract-clause recursion**: the checker does NOT yet recurse contract clauses for
  undeclared-var/arg/operator errors (only boolean-typedness). Needs old()/result/quantifier
  scope handling to avoid false positives. Corpus-invisible gap.

Note: stage1 `checkReturnStmt` does NOT compare the return value to the declared return
type — there is no return-type-mismatch diagnostic to port.

Known deferred (need new machinery): impl-block-method contracts; immutable-target
assignment/push (needs mutability tracking in Scope).

## How to resume

1. `git log --oneline -20`, then read this file + `prds/TASKS.md` (Phase 48 rows) + ADR 0056
   (+ ADR 0057 for the Scope async flag).
2. All builtin arg typing + the full async-context surface (builtins + await expr) is DONE
   (48j-c/48j-c2a-e). Best next slices: the remaining Phase 48 gaps (spawn/try operand
   recursion is trivial and mirrors the await case; then assert_eq comparable-set, async-test
   no-await warning, unary operator-typing). Or pick a **phase-53 gap** (generic-entity arity
   is well-scoped, but mind the let-mismatch caveat below). Keep inference SOUND (Unknown
   skips); one check per slice, gate after each.
3. Validate with `make validate`, `make selfcheck-formatter`, `make diff-formatter`,
   `make diff-linter`, `make diff-checker` after every slice.
