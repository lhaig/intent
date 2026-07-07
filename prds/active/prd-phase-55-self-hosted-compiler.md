# PRD — Phase 55: Self-Hosted Compiler (IR + Backend)

**Status:** ACTIVE (kickoff 2026-07-07) — this is the post-checker next front.
**Goal:** `intentc build --emit --self-hosted <file>` emits target source **byte-equal**
with stage1 `intentc build --emit <file>`, starting with the Rust backend. This is the
real bootstrapping endgame: a stage2 that *compiles*, not just checks/formats/lints.

> **KICKOFF NOTE (read first after any context compaction).** The self-hosted front-end
> (lexer/parser/AST/checker) and fmt/lint are DONE and byte-equal with stage1. Phase 54
> made `check --self-hosted` handle the compiler's own multi-file source (ADR 0058). The
> remaining piece for true self-hosting is IR lowering + a backend emitter, reimplemented
> in Intent. Work it in thin, byte-equal-gated slices exactly like the checker phases.

## Why / end state

Three tools are self-hosted (fmt Phase 42, lint Phase 43, check Phases 45-54). The compiler
back half — AST → IR (lowering) → target source (backend) — is still Go-only. Reimplementing
it in Intent closes the bootstrap loop (Intent compiling itself).

**Done when:** a growing corpus of `.intent` programs emits Rust byte-equal between stage1
and stage2, gated by a new `make diff-emit` (analogue of `diff-checker`), starting from
`examples/hello.intent` and expanding construct-by-construct.

## What must be reimplemented in Intent (sized)

Mirror these Go packages (LOC = non-test):
- `internal/ir/nodes.go` — IR type model: `Program`, `Module`, `Function`, `Stmt` (interface),
  `Expr` (interface), etc. (part of ~2,745 LOC IR total).
- `internal/ir/lower.go` — `Lower(prog, checkResult) -> *Module` / `LowerAll(...)`: AST → IR.
- `internal/rustbe/rustbe.go` — `Generate(mod, opts) -> string` / `GenerateAll(...)`: IR → Rust
  (~2,420 LOC). This is the FIRST backend (default target; `build --emit` defaults to Rust).
- `internal/backend` (~123 LOC) — registry/shared helpers (may not need a full port).

Minimum viable self-hosted compiler = IR model + lower + rustbe ≈ **5,000+ LOC of Intent**,
on top of the existing ~8,700-LOC self-hosted front-end. Multi-phase. jsbe (1,712) / wasmbe
(1,161) are later, out of scope for the first milestone.

## Module layout (proposed)

`selfhost/compiler/` as a sibling of `checker/`, importing `../shared/` (lexer/ast/parser)
and `../checker/` (reuse `check_program` — the emit path assumes a checked program):
- `selfhost/compiler/ir.intent` — IR node entities (mirror nodes.go).
- `selfhost/compiler/lower.intent` — AST → IR (mirror lower.go).
- `selfhost/compiler/rustbe.intent` — IR → Rust (mirror rustbe.go).
- `selfhost/compiler/compile_main.intent` — CLI entry: read file(s) → parse → (check) → lower
  → emit; reuse the Phase 54 multi-file discovery (`stage2CheckPaths` pattern in the harness).

Model the IR `Stmt`/`Expr` interfaces the same way the AST does it: tagged entities with a
`kind: Int` discriminator + parallel/child arrays (ADR 0053/0054 precedent), NOT trait
objects. This is an ADR-worthy decision (see below).

## Phasing (thin, byte-equal-gated slices)

**Thin first slice (do this first — proves the whole pipeline end-to-end):**
- **55a** IR node entity model for the trivial subset (Program/Module/Function + a handful of
  Stmt/Expr kinds: expr-stmt, call, string/int literal). Mirror nodes.go for just those.
- **55b** `lower.intent`: lower `examples/hello.intent` (one `main` function, `print("...")`,
  literals) to the stage2 IR.
- **55c** `rustbe.intent`: emit Rust for that IR, byte-equal with stage1 `build --emit`
  on hello.intent. Wire `intentc build --emit --self-hosted` in the harness (mirror
  `stage2CheckerBinary`/`runStage2Checker`); add `make diff-emit` gating hello.intent.

**Then scale up, one construct per slice, each a new emit-corpus entry gated byte-equal:**
let-bindings & locals → arithmetic/comparison/logical binops → if/while/for → user functions
& calls & args → entities (structs) + field access + methods → enums + match → contracts
(`requires`/`ensures`/`invariant` → `assert!` injection; mind `--strip-contracts`) →
Result/Option/`?` → generics → closures/lambdas → async. Grow `make diff-emit`'s corpus as
each construct lands; the 22 examples are the target corpus (mirror `TESTED_EXAMPLES`).

## Strategy note (differs from the checker's sound-but-incomplete stance)

The checker (ADR 0056) could be *incomplete* (skip = false negative, corpus-safe). The
emitter cannot: emitted code must be COMPLETE and correct for every construct it claims to
support, or the byte-equal gate fails immediately. So coverage grows by **explicitly
expanding the supported construct set per slice**, and the `diff-emit` corpus only contains
programs whose constructs are all supported. A program using an unsupported construct is
out-of-scope until its slice lands (stage2 may error/stub; don't add it to the gate yet).

## Gate strategy

- New `make diff-emit` (or `diff-compiler`): for each corpus file, compare
  `intentc build --emit <f>` vs `intentc build --emit --self-hosted <f>` byte-for-byte
  (stage1 writes `<base>.rs`; capture and diff). Starts at 1/1 (hello), grows per slice.
- Keep `selfhost/compiler/*.intent` a formatter fixpoint (extend `selfcheck-formatter`) and
  clean under `check --self-hosted` (extend `selfcheck-checker`).
- Existing gates (diff-checker 86/86, diff-formatter, diff-linter, go test, validate) must
  stay green throughout.

## Decisions to make (ADRs, cite prior art per repo convention)

1. **IR modeling in Intent** — tagged-entity discriminator vs. alternatives; reuse of the
   AST's `kind: Int` + child-array pattern. (New ADR; cite ADR 0008 IR, ADR 0053/0054.)
2. **Reuse vs. reimplement the checker in the emit path** — stage1 lowers a *checked*
   program (`Lower(prog, checkResult)`); does stage2 need the CheckResult, or can it lower a
   parsed-only program for the trivial subset? Scope what `checkResult` actually feeds lowering.
3. **`--emit --self-hosted` wiring** — mirror the Phase 54 harness (discovery + a stage2
   compiler binary); how multi-file emit works (stage1 `LowerAll`/`GenerateAll`).
4. **Contract lowering parity** — `assert!` injection order/format and `--strip-contracts`.

## References
- `internal/ir/nodes.go`, `internal/ir/lower.go`, `internal/rustbe/rustbe.go` — the port targets.
- ADR 0008 (IR), ADR 0009 (backends per target), ADR 0053/0054 (self-hosted type/position
  modeling), ADR 0056 (checker inference strategy — contrast), ADR 0058 (Phase 54 multi-file
  harness pattern to reuse for `--emit --self-hosted`).
- `cmd/intentc/main.go` — `stage2CheckerBinary`, `runStage2Checker`, `stage2CheckPaths`
  (Phase 54) — the wiring pattern to mirror for the compiler binary.
- `prds/NEXT-STEPS.md` — strategic state; `prds/progress.md` — per-slice log.
