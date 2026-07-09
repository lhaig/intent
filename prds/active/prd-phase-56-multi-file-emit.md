# PRD — Phase 56: Multi-File Emit → Full Bootstrap (stage2 emits its own source)

**Status:** ACTIVE (kickoff 2026-07-09). Post-endgame of Phase 55: the single-file
Rust emitter self-hosts the full corpus (diff-emit 31/31). This phase extends the
stage2 compiler to MULTI-FILE emit and drives toward the true capstone — the
self-hosted compiler emitting its OWN (multi-module) source into a working stage3.

> **Read first after any compaction.** Phase 55 closed the single-file bootstrap loop
> (front-end Phases 42-54 + IR/backend Phase 55, all byte-equal with stage1). "Full
> coverage" self-hosting means the stage2 compiler can emit a MULTI-FILE program —
> ultimately its own source — byte-equal with stage1's `LowerAll`/`GenerateAll`. Work
> it in thin, byte-equal-gated slices exactly like Phase 55.

## Why / end state

The compiler's own source is multi-module (`selfhost/shared/*` + `selfhost/compiler/*`).
Phase 55's `intentc build --emit --self-hosted` rejected multi-file input. Reproducing
stage1's multi-file lowering (`ir.LowerAll`) and generation (`rustbe.GenerateAll`) —
cross-module name mangling, per-module emission, once-at-end `use` injection — is the
last piece before stage2 can regenerate itself.

**Done when:** the stage2 compiler emits Rust byte-equal with stage1 for multi-file
programs across a growing corpus, culminating in the `selfhost/**` source itself, and a
stage3 binary built from that emit matches stage2. Gated by `make diff-emit` (multi-file
entries added to the existing corpus) and, for the capstone, a stage2-vs-stage3 check.

## Design decision — resolve mangling at LOWERING, not in the backend

Stage1 builds a `moduleManglings` map inside `GenerateAll` and threads
`namePrefix`/`structPrefix`/`typeOrigins` through the entire generator. The stage2
backend is FREE FUNCTIONS threading `funcs` (gotcha b), so threading per-module prefix
state through the whole `generate_expr` tree would be invasive. Instead we **resolve the
mangling during lowering** (`lower_all`): a non-entry module's declarations get their
final mangled names in the IR, and a `mod.fn(args)` call is lowered to a pre-mangled
`irex_call` named `mod_fn`. The backend then needs no per-module prefix state — the
OUTPUT is identical (byte-equal is what the gate enforces), the churn far smaller. This
matches the existing stage2 deviation from stage1's structure.

## Slices (byte-equal-gated; ✅ = done, in `make diff-emit`)

- ✅ **56.1 — functions + module-qualified calls** (`examples/multi_file`, diff-emit 32/32).
  - Go: `stage2CompilePaths(entry)` returns the import closure in TOPOLOGICAL order
    (deps first, ENTRY last — contrast `stage2CheckPaths`, entry first for diagnostics);
    `--emit --self-hosted` passes it; dropped the multi-file rejection.
  - `lower.intent`: `LowerScope` +`module_names`/`module_prefixes` (the global qualifier→
    prefix map, EMPTY in single-file so single-file emit is byte-unchanged), threaded via
    the 5 copy helpers; `path_module_name`; `LowerCtx`; `lower` → `lower_module` + new
    `lower_all`; the ex_field-callee site detects a module qualifier and emits a
    pre-mangled `irex_call`. Non-entry module functions are name-prefixed + demoted from
    entry in `lower_function`.
  - `rustbe.intent`: extracted `generate_module_body`, added `generate_all` (multi-file
    header, global funcs table for cross-module borrow/arity, once-at-end HashMap
    injection).
  - `compile_main.intent`: N==1 single-file path unchanged; N>1 → `lower_all`/`generate_all`.

- ✅ **56.2 — cross-module entities/enums** (structPrefix + typeOrigins), diff-emit 33/33.
  - Type-origins map (entity/enum name -> Capitalised-file-base prefix; "" for the entry
    module) built in `lower_all` from ALL modules; threaded on `LowerCtx` + `LowerScope`.
  - Mangling is applied at the EMISSION-side IR only — entity/enum decl names, field
    types, param/return types, let annotation types, constructor-call names, and
    variant/unit-variant `enum_name` — via `mangle_type_name`/`mangle_ir_type`.
    `expr_type` and the `EntityDecl` registry stay UNMANGLED so lowering-time lookups
    (self/field types, dispatch, clone) keep resolving on the declared names.
  - Global entity/enum REGISTRY (all modules') threaded into `lower_module` so a cross-
    module constructor (`Point(...)` in another module) is classified as a constructor
    (stage1's global `lowerer.entities`). `capitalize` helper added.
  - Fixture `selfhost/compiler/emit-fixtures/multimod_entity/` (entity as field/param/
    return type, cross-module constructor, unit-variant enum). Two `lower_test` cases.
  - **Known limitation (deferred to 56.3):** chained field access on a cross-module-typed
    LOCAL (`local.field.method()`) — the block scope stores the local's mangled type, so
    the field lookup would miss. Not hit by the corpus; fix when 56.3 needs it (keep the
    scope binding unmangled while emitting the mangled annotation).
  - **Deferred:** module-qualified constructor `mod.Entity(...)` (fixture uses unqualified
    cross-module `Point(...)`); traits across modules; `expr_type`/cast mangling.

- ✅ **56.3 — emit the compiler's own source** (`make diff-emit-self` 4/4). The stage2
  compiler emits the ENTIRE self-hosted toolchain byte-equal with stage1: compiler (8304
  lines), checker (6307), formatter (5113), linter (5042). Divergence driven 2667 -> 0.
  What it took, in order: same-module call prefixing; let-binding scope type kept unmangled
  (`IrStmt.let_type_emit`) so cross-module-typed-local field lookups resolve; place-value
  cloning in let/return/assign + field-access in clone_if_needed; `lower_test` ctx;
  call/method RETURN-type inference (free-function return registry + method-return + builtin
  string methods `to_string`/`to_lowercase`/`trim`) for string-concat `format!` and chained
  char dispatch; the `args()` builtin; a precise IR-Map HashMap trigger (replacing the
  substring proxy that misfired on the compiler's own `"HashMap<"` string literals); and the
  free-function Array-ref-param clone-on-let (methods/ctors take arrays BY VALUE, so
  excluded). NOTE: the decl-name→file-base mangling was already handled by slice 1's
  decl-name qualifier entry in module_names.

- **56.4 — stage3 bootstrap.** Compile the stage2-emitted Rust into a stage3 binary and
  verify it matches stage2 (byte-equal emit / functional parity). Closes the full triangle.

## Gates per slice

`make diff-emit` (add the new multi-file entry) + `make selfcheck-formatter` +
`make selfcheck-checker` + `intentc test selfhost/compiler/{lower,ir}_test.intent`. If
`selfhost/shared/*` is touched: also `make diff-checker diff-formatter diff-linter` +
`go test ./...` + `make validate`. Changes to `cmd/intentc/*.go`: `go test ./cmd/intentc/...`.

## Non-goals (for now)

Multi-file ERROR-diagnostic parity (deferred per ADR 0058); the js/wasm backends in
Intent (a separate front); cross-package (`intent.toml`) emit beyond simple imports.
