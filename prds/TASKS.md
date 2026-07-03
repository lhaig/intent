# Norman Tasks

Live task list. The active phase is expanded into steps; completed phases are
collapsed into [TASKS-archive.md](TASKS-archive.md). Per-phase design detail
lives in the PRD files under `done/`, `active/`, and `backlog/`, and the rich
shipped summaries remain in [docs/ROADMAP.md](../docs/ROADMAP.md).

Resuming? Read [NEXT-STEPS.md](NEXT-STEPS.md) first.

## Phase 40A.2: Stage2 Comment Preservation — COMPLETE (2026-06-15)

Closes the comment-preservation half of the byte-equal self-format gate for the
self-hosted formatter (`selfhost/formatter/`). Sub-pieces C (source-order) and B
(paren stripping) and A.1 (leading-decl comments) already shipped (Phases 40C /
40B / 40A.1).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.2.1 | Trailing-EOF comments | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | commit 19d766e; +5 tests; 136/136 rust+js |
| 40A.2.2 | Body / between-statement comments (`Stmt.comments_before`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 141/141 rust+js |
| 40A.2.3 | Inline-after comments on statements (`let x = 1; // ...`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | +5 tests; 146/146 rust+js; statements only |
| 40A.2.4 | Comprehensive synthetic comment round-trip (partial gate) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | 1 test exercising all 4 supported positions; 147/147. Real-file byte-equal gate moved to Phase 40A.3 (see below) |

**Phase 40A.2 complete** — comments now round-trip in 4 positions: leading-decl, between-statement, inline-after-statement, trailing-EOF.

## Phase 40A.3: Real-file byte-equal self-format — COMPLETE (2026-06-15)

**Byte-equal self-format achieved on all 4 stage2 files** (`lexer.intent`,
`ast.intent`, `parser.intent`, `format.intent`): `format(parse(src)) == src`.
A probe drove discovery of each remaining divergence; the 4 files were then
canonicalized (reformatted by the formatter) so it is a fixpoint on them.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 40A.3.1 | Module-leading comments (before `module`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | ModuleDecl.comments_before; +2 tests |
| 40A.3.2 | Comments before entity fields | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comments_before |
| 40A.3.3 | Comments before entity methods / constructor | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | + impl methods; FunctionDecl.comments_before |
| 40A.3.4 | End-of-block comments (before `}`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | Block.trailing_comments from rbrace token; +2 tests |
| 40A.3.5 | Inline-after on fields | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | FieldDecl.comment_after; +4 tests |
| 40A.3.7 | Inline-after on declaration closing `}` | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | Block.brace_comment_after; fixed dropped one-liner doc-comments; +2 tests |
| 40A.3.8 | Generic type-arg round-trip (`Array<String>`) | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | parse_type_name reconstructs args (was `<...>` placeholder) — the real byte-equal blocker; +1 test |
| 40A.3.6 | Canonicalize stage2 files + real-file gate test | [phase-40a-comment-preservation.md](done/phase-40a-comment-preservation.md) | DONE (2026-06-15) | reformatted all 4 files (still compile + self-parse + 158/158); self_format_one asserts byte-equality; probe confirmed firstdiff -1 + idempotent on all 4 |

**Phase 40 complete.** Byte-equal self-format gate met (sub-pieces 40C source-order,
40B paren-stripping, 40A.1/40A.2/40A.3 comments). Stage2 formatter is a fixpoint on its
own source.

## Phase 41: Stage2 Parser Surface Widening — COMPLETE (2026-06-15)

Widened the stage2 parser beyond its self-hostable subset so it can format arbitrary Intent. Each sub-feature round-trips through parse + format; byte-equal self-format on the stage2 files preserved throughout. See [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md).

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 41.1 | Contracts: `requires` / `ensures` / `decreases` on functions + methods | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | FunctionDecl.{requires,ensures,decreases}_clauses; formatted between signature and `{`; +4 tests |
| 41.2 | `match` expressions over Result/Option | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | ex_match + MatchArm; level-aware format_match via format_expr_indented; +3 tests |
| 41.3 | `for ... in ...` loops | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | st_for (reuses Stmt name/expr/then_block); +3 tests |
| 41.4 | `try ?` operator | [phase-41-parser-surface-widening.md](done/phase-41-parser-surface-widening.md) | DONE (2026-06-15) | ex_try postfix; +2 tests; 170/170 rust+js |

## Phase 42: Stage2 Formatter CLI Wiring + Differential Test — 12 tasks completed 2026-06-16 (corpus 22/22 vs `intentc fmt`; see [TASKS-archive.md](TASKS-archive.md))

## Phase 43: Self-Hosted Linter (stage2) — 13 tasks completed 2026-06-24 (all 16 rule families; `make diff-linter` 26/26 byte-equal vs `intentc lint`; see [TASKS-archive.md](TASKS-archive.md))

## Phase 44: selfhost/shared Restructure — 5 tasks completed 2026-06-26 (shared/ + formatter/ + linter/ siblings; all gates green; see [TASKS-archive.md](TASKS-archive.md))

## Phase 45: Self-Hosted Checker (first slice) — 11 tasks completed 2026-06-26 (selfhost/checker/; make diff-checker 34/34 byte-equal vs `intentc check`; see [TASKS-archive.md](TASKS-archive.md))

## Phase 46: Checker Type Representation Foundation — COMPLETE (2026-07-02)

Build the self-hosted checker's type-system foundation: a structured `Type` tree +
`parse_type(string)` (parsing the type strings the AST already carries — NO front-end
change) + a resolver + the `unknown type 'X'` check. Expression inference + type-rule
checks are Phase 47+. Gated by `make diff-checker` (unknown-type fixtures byte-equal +
no false positives on the 22 valid examples). See [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) and ADR 0053.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 46.1 | ADR 0053 — type representation foundation | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-06-26) | docs/decisions/0053; D1 in-checker Type from strings (no front-end change), D2 first-slice=resolver+unknown-type, D3 two-dir diff, D4 faithful port |
| 46.2 | `Type` entity + `parse_type(string)` | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Type{name,type_args,fn_param_count}; Fn = name "Fn", type_args=params++[ret], fn_param_count=N; TypeParser recursive-descent scanner (mutates self.pos like Lexer); parse_type public fn; +8 tests (158 pass rust+js); all gates green (diff-checker 34/34, diff-formatter 22/22, diff-linter 26/26, selfcheck EQUAL, validate OK) |
| 46.3 | resolver `type_is_known` | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Faithful port of ResolveTypeWithParams (types.go): primitives + Array(1)/Option(1)/Future(1)/Result(2)/Map(2, unhashable-key reject) + Fn(all parts) + entity/enum/type-param names, recurse into args. type_is_known == "ResolveType != nil" (every checker.go unknown-type site is `if resolve==nil`). +11 tests (168 pass rust+js); all gates green + validate OK. NOTE: rust backend won't auto-borrow a call-result passed to an Array<T> (&Vec) param (E0308) — initially bound name lists to `let` vars, reverted to inline call-result args once BE-1 was fixed (same session) |
| 46.4a | `unknown type 'X'` — function param + return (first slice) | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Interleaved into check_dup_functions (stage1 registerFunctions order: dup→continue, else params@param-pos then return@fn-pos); base = outer ref name via parse_type().name; threads f.type_params; collect_entity_names/enum_names helpers. +5 in-lang tests (173 pass rust+js) + 3 fixtures (param/return/nested-outer-name) byte-equal. diff-checker 37/37, all gates + validate green |
| 46.4b.1 | FieldDecl positions + ADR 0054 | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Additive line/column on FieldDecl, populated from the `field` keyword (matches stage1 FieldDecl.Pos()). ADR 0054: additive positions permitted (distinct from ADR 0053 D1's no-structured-types; formatter-inert; 45.7 precedent). Validated in isolation: selfcheck + diff-formatter byte-equal + all gates |
| 46.4b.2 | entity field + method unknown-types | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | check_dup_entities now two-pass (stage1 registerEntities): pass 1 dup names, pass 2 per-entity fields (anchored at `field` kw) then methods (params/return), threading entity.type_params. Constructor EXCLUDED (stage1 checkConstructor doesn't emit). +4 tests (177 pass rust+js) + 2 fixtures (field 8:5, method 10:16) byte-equal. diff-checker 39/39, all gates + validate green |
| 46.4b.3 | `let` statement unknown-types | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Restructured check_body_stmts st_let to stage1 checkLetStmt order (redefinition→unknown-type→RHS→define, each an early-return; unknown type suppresses RHS + define). PLAIN resolve — NO type params (stage1:1259) — so `no_tp` empty passed. entity_names/enum_names threaded into check_body_stmts + 4 recursive calls; 5 caller sites pass collect_*(prog) (call-result args, enabled by BE-1 fix). +3 tests (180 pass rust+js) + 1 fixture (let 4:5; early-return suppression verified byte-equal). diff-checker 40/40, all gates + validate green |
| 46.4b.4 | enum-variant field unknown-types | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | Interleaved into check_dup_enums matching stage1 registerEnums quirk: entities EMPTY (registered later), enums = those declared strictly before this one (`enums_so_far`, added after each variant loop), no type params. Anchored at variant Param position (via parse_param). +3 tests incl. the entity-typed-variant-field quirk (183 pass rust+js) + 1 fixture (variant 8:7). diff-checker 41/41 (36 corpus variant fields clean), all gates + validate green. (parser:830 was the LAMBDA parser, not variants — variant fields already had positions.) |
| 46.4b.5 | extern param/return unknown-types | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | TODO (low priority) | Deferred documented gap: 0 corpus usage. stage2 DOES parse ExternDecl (parser:1324); stage1 emits at ext param p.Pos() / ext.Pos() (checker.go:806-825, plain ResolveType). Add when externs are exercised; needs a hand-written fixture (no corpus coverage) |
| 46.5 | diff-checker fixtures + no-false-positives | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | 7 unknown-type fixtures (param/return/nested/field/method/let/variant), all byte-equal; 22 valid examples stay clean. diff-checker 41/41 |
| 46.6 | docs + final validate + push | [prd-phase-46-checker-type-foundation.md](done/prd-phase-46-checker-type-foundation.md) | DONE (2026-07-02) | checker README + ROADMAP (Phase 46 entry) + NEXT-STEPS (Phase 47 next) + ADR 0054; PRD active→done. make validate + all gates green; committed + pushed |

## Phase 47: Self-Hosted Checker — Builtin-Call Arity — COMPLETE (2026-07-02)

Closes the builtin-call arity gap deferred from Phase 45 (arity only; argument typing +
async-context deferred to Phase 48). ADR 0055. Gated by `make diff-checker`.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 47.1 | ADR 0055 — builtin-arity strategy | [prd-phase-47-builtin-arity.md](done/prd-phase-47-builtin-arity.md) | DONE (2026-07-02) | D1 arity-only (defer typing/async to Phase 48); D2 name/count table + 3 message shapes; D3 builtins-first, callee-anchored, early-return; D4 diff-checker gate |
| 47.2 | builtin-arity table + message helper | [prd-phase-47-builtin-arity.md](done/prd-phase-47-builtin-arity.md) | DONE (2026-07-02) | 23 builtins; builtin_arity/-1, builtin_uses_expects_verb, builtin_arity_message (expects/requires-exactly/takes-no, singular at N=1) |
| 47.3 | wire into check_expr_names | [prd-phase-47-builtin-arity.md](done/prd-phase-47-builtin-arity.md) | DONE (2026-07-02) | first in ex_call/ex_ident branch (stage1 builtin-before-variant/function); emit+return on mismatch, recurse args on match; callee anchor |
| 47.4 | fixtures + tests + no-false-positives | [prd-phase-47-builtin-arity.md](done/prd-phase-47-builtin-arity.md) | DONE (2026-07-02) | 3 shape fixtures byte-equal; +5 tests incl. plural + early-return; 22 examples clean. diff-checker 44/44, 188 tests |
| 47.5 | docs + validate + push | [prd-phase-47-builtin-arity.md](done/prd-phase-47-builtin-arity.md) | DONE (2026-07-02) | ROADMAP + NEXT-STEPS + checker README + PRD; make validate + all gates green; committed + pushed |

## Phase 48: Expression Type Inference + Type-Rule Checks — IN PROGRESS (foundation shipped 2026-07-02)

Sound-but-incomplete `infer_expr_type` (returns a Type only when certain, else an
Unknown sentinel), with type-rule checks layered on incrementally behind the diff-checker
gate. ADR 0056. A large, open-ended phase (full stage1 type-system parity). Foundation +
first two checks shipped + pushed; the rest are the continuation.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 48a | `infer_expr_type` engine (sound/incomplete) | ADR 0056 | DONE (2026-07-02) | literals + comparison/logical→Bool + arithmetic(operand type when both same known) + unary + paren; ident/call/method/field/index/match/array/range → Unknown (need typed scope). type_unknown/is_unknown_type. commit 8198ce5 |
| 48b | condition-must-be-boolean (if/while) | ADR 0056 | DONE (2026-07-02) | emit on confident non-Bool, Unknown→skip (stage1 condType!=nil). 2 fixtures + 6 tests. commit 8198ce5 |
| 48c | `let` type-mismatch | ADR 0056 | DONE (2026-07-02) | declared type vs confidently-inferred RHS (`cannot assign X to Y`); Unknown RHS→skip. 1 fixture + 4 tests. commit e156de7 |
| 48d | type-carrying scope (params) | ADR 0056 | DONE (2026-07-02) | Enriched Scope with parallel type arrays; scope_define records Unknown (name-resolution byte-identical), scope_define_typed seeds params from annotations; scope_type_of (innermost wins). infer_expr_type takes a scope, resolves idents. condition-boolean + let-mismatch now fire on typed params (`if n:Int`, `let y:Bool = x:Int`), byte-equal; Unknown still skips. +1 fixture, tests updated. diff-checker 48/48, 200 tests. commit ef9bb96. (self/field/call-return still Unknown — later.) |
| 48d+ | let-variable binding | ADR 0056 | DONE (2026-07-02) | `let` records its type in scope (declared when annotated, else inferred RHS) so condition-boolean/let-mismatch/arg-type extend to let-bound vars downstream. +3 tests. commit 9f3df8a |
| lit-pos | literal Expr positions (front-end) | ADR 0054 | DONE (2026-07-02) | parser sets line/col on Int/Float/String/Char/Bool literals (was only ex_ident); formatter-inert; enables arg-type anchoring at literal args. commit c97d936 |
| 48f | function argument-type mismatch | ADR 0056 | DONE (2026-07-02) | threaded prog into check_body_stmts/check_expr_names (lookup_function); non-generic call, confident+positioned arg vs param type → `argument N to 'fn': expected X, got Y` at the arg. generics + Unknown args skipped. +1 fixture + 4 tests. diff-checker 49/49, 207 tests. commit ea8d60e |
| binop-pos | ex_binop operator positions (front-end) | ADR 0054 | DONE (2026-07-03) | all 8 binop sites routed through Parser.make_binop, anchored at the operator token (matches stage1 BinaryExpr.Pos()). parser.intent stays a formatter fixpoint; all gates unchanged. commit b77bdf3 |
| 48e | operator-typing errors | ADR 0056 | DONE (2026-07-03) | binop_result_type mirrors checkBinaryExpr (result Type or Unknown-if-undefined); infer_expr_type's ex_binop delegates to it (comparison/logical → Bool only for VALID operands, else Unknown — no eager Bool). check_expr_names emits `operator 'OP' not defined for X and Y` / `operator 'OP' requires boolean operands, got X and Y` at the operator when both operands confident + binop_result_type Unknown; Unknown operand skips. operator_display reproduces stage1's TOKEN-NAME quirk (MINUS/STAR/EQ/LT/AND; '+' literal). '=' excluded. Removed dead is_*_op helpers. +4 fixtures (diff-checker 59/59), +7 tests (227). commit e2a986f. (unary operator-typing deferred — corpus-invisible) |
| 48g | variant-constructor argument-type mismatch | ADR 0056 | DONE (2026-07-02) | `variant 'V' field 'f' expects X, got Y` at the arg; find_variant_params(prog) reads field types from prog.enums; confident+positioned arg vs field type. +1 fixture + 2 tests. commit 1da6fc1 |
| 48h | assignment-stmt type-mismatch | ADR 0056 | DONE (2026-07-02) | `target = value` (ex_binop op "=" in an st_expr); target scope-type vs inferred value → `type mismatch: cannot assign X to Y` at stmt pos. +1 fixture + 2 tests. commit 39c3da6. (immutable-target check needs mutability tracking — deferred) |
| 48i.1 | `self` + field-access inference | ADR 0056 | DONE (2026-07-03) | `self` typed as its enclosing entity (make_method_scope threads e.name/ib.entity_name); infer_expr_type gains an ex_field case (entity_field_type over prog.entities) so `self.field`/`x.field` resolve → condition-boolean, let/assignment mismatch, arg-type all extend to field access. Unknown (primitive/unknown entity/missing field) skips. +1 fixture + 2 tests. diff-checker 52/52, 213 tests. commit e42b66d |
| fld-pos | ex_field/method-name Expr positions (front-end) | ADR 0054 | DONE (2026-07-03) | parser sets line/col on ex_field to the field/method-name token (matches stage1 FieldAccessExpr/MethodCallExpr.Pos()); formatter+linter-inert; enables method-call diagnostics to anchor at the method name. commit 033e7dd |
| 48i.2 | method-call arity + arg-types | ADR 0056 | DONE (2026-07-03) | check_expr_names' ex_field-callee branch: infer receiver via infer_expr_type; when confidently a known user entity whose method resolves to exactly one decl (body OR trait-impl, via entity_method_decls) → `method M expects N arguments, got A` at the method name (early return), then `argument i to method M: expected X, got Y` at each arg. Arg-types only for non-generic entities (type-param skip, like the fn path). Sound skips: unknown/primitive/collection receivers (builtin Array/Map/String/Char deferred), unresolved/ambiguous names (no `no method`). +3 fixtures + 7 tests. diff-checker 55/55, 220 tests. commit 587f084 |
| contract-pos | contract-clause keyword positions (front-end) | ADR 0054 | DONE (2026-07-03) | the 3 contract-clause loops + parse_invariant_decl stamp the clause Expr with the requires/ensures/invariant KEYWORD position (stage1 RequiresClause/Invariant.Pos()). parser.intent stays a formatter fixpoint; all gates unchanged. commit bc060ae |
| 48j-b | contract well-typedness | ADR 0056 | DONE (2026-07-03) | check_bool_contracts infers each requires/ensures/invariant clause → `requires clause must be boolean, got X` / `ensures clause must be boolean, got X` / `invariant must be boolean, got X` at the clause keyword when confidently non-Bool; Unknown (old()/result/quantifiers/calls) skips. check_functions + check_entities check contracts before bodies in stage1 order (invariants→ctor→methods); check_entities now skips generic entities (matches stage1 checkEntities:1057). Impl-method contracts deferred (trait+impl clause mix; stage2 trait-contract parser gap). +4 fixtures (diff-checker 63/63), +7 tests (234). commit 7ca2dab |
| match-pos | MatchArm + ex_match positions (front-end) | ADR 0054 | DONE (2026-07-03) | MatchArm gains line/col (pattern's first token); parse_match_expr anchors ex_match at the `match` keyword (stage1 MatchArm/MatchPattern.Pos() + MatchExpr.Pos()). ast.intent + parser.intent stay formatter fixpoints; all gates unchanged. commit 88807f7 |
| 48j-a | match-arm structural checks | ADR 0056 | DONE (2026-07-03) | stage1 checkMatchExpr (checker.go:2915) for a confident USER-enum scrutinee: `variant 'V' is not a variant of enum 'E'`, `duplicate match arm for variant 'V'`, `variant 'V' has N fields but pattern has M bindings`, `unreachable pattern after wildcard '_'`, `non-exhaustive match on enum 'E': missing variants: ...` (enum order, at the match keyword). Helpers find_enum_index/enum_has_variant/variant_field_count/check_arm_body. Unreachable/unknown-variant arms don't recurse their body (stage1 early-continue). +5 fixtures (diff-checker 68/68), +8 tests (242). commit 80b6857 |
| 48j-a2 | match arm-type consistency + scrutinee-must-be-enum | ADR 0056 | DONE (2026-07-03) | `match scrutinee must be an enum type, got X` for a confident PRIMITIVE scrutinee (is_primitive_type; emit + return like stage1) — commit e65292f. `match arm type mismatch: expected X, got Y` — arm bodies inferred in a scope with bindings TYPED from variant fields (arm_typed_scope/variant_fields); arm 0 = baseline (stage1 i==0), later confident different-typed arm flagged; restricted to no-type_args types (String()==name) — commit d461273. +2 fixtures (diff-checker 70/70), +4 tests (246). All gates + validate green |
| 48j-c | builtin argument typing (uniform-type group + print + assert_close) | ADR 0056 | DONE (2026-07-03) | builtin_arg_type table: assert(Bool), char_from_codepoint/sleep(Int), read_file/write_file/create_dir/file_exists/env_get/http_post/http_get/json_get/json_path/emit_event(String) → `NAME() argument [N ]must be T, got X` (numbered iff arity>1); print → `print() cannot print type X (accepts Int, Float, Bool, String)` (Char rejected; uses .name); assert_close → `assert_close() argument N (label) must be Float, got X` (actual/expected/epsilon). Confident + no-type_args args only. commits 42b3f0c + bd3d22f + 87d04c3. +5 fixtures (diff-checker 75/75), +9 tests (255) |
| 48j-c2a | len() argument typing | ADR 0056 | DONE (2026-07-03) | len() accepts String or a generic Array/Map (stage1 checker.go:1759); a confidently-inferred simple type that is NOT String → `len() requires Array, Map, or String argument, got X` at the call. Confident + no-type_args args only (skips generic Array/Map — accepted anyway — and defers rejected generics like Option<Int> that would need .String() rendering: a corpus-invisible false negative). Dedicated branch (accepts-set doesn't fit builtin_arg_type). commit 3a54a67. +1 fixture (diff-checker 76/76), +3 tests (258) |
| 48j-c2b | assert_panics() argument typing | ADR 0056 | DONE (2026-07-03) | assert_panics() requires `Fn() -> Void` (stage1 checker.go:1728). A Fn type always carries type_args (>= its return), so a confidently-inferred no-type_args type is definitely a non-function that stage1 rejects → `assert_panics() argument must be Fn() -> Void, got X` at the call. Fn-typed params + lambdas skip soundly (lambda infers Unknown; a wrong-shape `Fn(Int)->Void` param has type_args → deferred false negative needing Fn .String()). commit a8d9773. +1 fixture (diff-checker 77/77), +2 tests (260) |
| 48j-c2c | assert_eq() argument typing (mismatch + Float) | ADR 0056 | DONE (2026-07-03) | Cross-arg check (stage1 checker.go:1691) run once after per-arg recursion. Both args confident + no-type_args (Equal reduces to name equality; .String()==.name): different names → `assert_eq() type mismatch: actual is X, expected is Y` (stage1 returns after this); same-name Float → `assert_eq does not support Float; use assert_close(...)`. Remaining comparable-set rules (entity eq method, Map/Future, generic recursion) deferred — need entity/method lookup or generic .String() (sound false negatives). commit 27642ea. +2 fixtures (diff-checker 79/79), +3 tests (263) |
| 48j-c2d | await_*/timeout async-context | ADR 0057 | DONE (2026-07-03) | await_all/await_any/timeout emit `<name> can only be used inside async functions` after the arity check passes (stage1 checker.go:1956/1983/2009). Async context is threaded on the Scope (new `in_async` field; scope_enter/scope_define_typed preserve it, scope_set_async flips it per function/test entry — check_functions from f.is_async, check_tests from t.is_async; methods/constructors/impls inherit false, matching stage1). ADR 0057 (chose Scope-field over a ~40-site parameter thread). Arg-type checks (Array<Future<T>>) still deferred. commit 90573b1. +3 fixtures (diff-checker 82/82), +6 tests (269) |
| 48j-c2e | `await` expression async-context | ADR 0057, 0054 | DONE (2026-07-03) | The `await` EXPRESSION emits `await can only be used inside async functions` (stage1 checkAwaitExpr:3161) when scope.in_async is false, anchored at the await keyword. Front-end: stamped the ex_await keyword position (parser, own inert commit fd191c3, ADR 0054). Checker: new ex_await case in check_expr_names emits the async error then recurses the operand for name-checking (matching stage1 order; also closes the pre-existing await-operand recursion gap). Operand Future<T> type check still deferred. commit 710919c. +1 fixture (diff-checker 83/83), +3 tests (272) |
| 48j-c2f | assert_eq entity-eq comparable-set (no eq method) | ADR 0056 | DONE (2026-07-03) | Extends the 48j-c2c assert_eq else-branch: two same-name args of a user entity with no `eq` method → `entity 'X' used in assert_eq but has no eq method; define 'method eq(other: X) returns Bool' to enable equality checks` (stage1 assertEqUnsupportedReason:3265). Presence test via entity_method_decls (entity body + trait-impl methods = stage1's EntityInfo.Methods, so byte-equal, no false positive). The eq-method SIGNATURE sub-checks (wrong return/param count/param type) + Map/Future/generic recursion still deferred (an entity WITH any eq method is skipped — sound false negative). commit f7b23be. +1 fixture (diff-checker 84/84), +2 tests (274) |
| 48j-c2 | remaining Phase 48 gaps | — | TODO | assert_eq eq-method SIGNATURE sub-checks + Map/Future/generic recursion (need generic .String()); async-test `declared 'async' but contains no 'await'` warning (needs testSawAwait tracking); unary operator-typing (needs ex_unary positions); spawn/try operand recursion in check_expr_names (same latent gap await just closed) |

**NOTE:** stage1 `checkReturnStmt` does NOT compare the return value to the declared
return type — there is no return-type-mismatch diagnostic to port.

## Phase 54: Multi-file self-hosted checking — COMPLETE (2026-07-03)

`intentc check --self-hosted` used to check a single file, so it flooded false
positives on the compiler's own multi-file source (imported types → `unknown
type`, module-name qualifiers → `undeclared variable`). This was the real
self-hosting blocker (valid code rejected), invisible to the single-file
`diff-checker` gate. ADR 0058.

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 54 | cross-module resolution for --self-hosted | ADR 0058 | DONE (2026-07-03) | Harness (`stage2CheckPaths`, main.go) reuses the Go ModuleRegistry to discover the import closure and passes the entry + all module paths to the stage2 binary. `check_main.merge_programs` flattens the modules into one Program (cross-module dedup by name, first-seen kept; entry merged verbatim so within-module dups still fire). `check_program_seeded` seeds imported module names so call qualifiers (`shared_lexer.foo()`) resolve; `check_program` is a thin wrapper (empty names) → single-file + tests byte-identical. New `make selfcheck-checker` gate: 9/9 core self-hosting modules match stage1 (not in `make validate` — stage2 is slow on big merges). commit TBD. diff-checker still 86/86, 278 checker tests, go test + validate green |

## Phase 53: Remaining checker diagnostics (independent gaps) — IN PROGRESS

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 53a | generic-entity-instantiation arity | ADR 0056 | DONE (2026-07-03) | A generic constructor call parses as an ex_ident callee with type args baked into the name (`Stack<Int>()` → callee.name == "Stack<Int>"); parse_type splits base + type_args. After the arg-recursion (stage1 order: args then arity), when the base is a generic entity WITH a constructor: no type args → `generic entity 'X' requires type arguments`; wrong count → `entity 'X' expects N type arguments, got M`. Anchored at the callee (= stage1 CallExpr.Pos()), base name in the message. has_constructor guard avoids the generic-arity error when stage1 emits `has no constructor` first. Fixtures use BARE constructor-call statements (a let-binding would make stage1 emit a second `type mismatch` that stage2's Unknown ctor-return can't reproduce). commit TBD. +2 fixtures (diff-checker 86/86), +4 tests (278) |
| 53 | remaining phase-53 gaps | — | TODO | entity constructor `has no constructor` diagnostic (broader than 53a; also non-generic); extern param/return `unknown type` (0 corpus usage; stage2 parses ExternDecl); trait-method contract parser gap |

## Backlog

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 23 | VS Code Marketplace publish | [phase-23-marketplace-publish.md](backlog/phase-23-marketplace-publish.md) | BLOCKED | engineering done; needs publisher account, PAT, branded icon |
| BE-1 | rustbe: auto-borrow call-result args to `Array<T>` (&Vec) params | — | DONE (2026-07-02) | Array/Map params are always `&Vec`/`&HashMap` in the signature, so ALL args must be references. Three arg-borrow sites only borrowed place expressions (extern: VarRef only; function: VarRef/field/index; module-call: VarRef only), so a call-result/temporary → E0308. Now borrow any Array/Map arg not already `&`. +1 rustbe unit test; 46.3 checker tests reverted to inline call-result args as end-to-end proof (168 pass rust+js). All gates green |
| HARN-1 | test runner: surface swallowed rust cargo-compile errors | — | DONE (2026-07-02) | runRustTests gated error-surfacing on `len(results)==0`, but parseCargoTestOutput returns one placeholder row per declared test on a build failure, so it never tripped. Now returns (results, ran) where ran = real verdicts parsed; gate on `ran==0` and return the raw cargo stderr. +2 go tests. Verified: forcing E0308 now prints the rustc error |

## Completed Phases (11–40) — see [TASKS-archive.md](TASKS-archive.md)

Phases 11–40 shipped (incl. Phase 40 byte-equal self-format). Full index with
per-phase status, dates, and PRD links is in the archive. Next: Phase 41 (parser
surface widening) — see [NEXT-STEPS.md](NEXT-STEPS.md).
