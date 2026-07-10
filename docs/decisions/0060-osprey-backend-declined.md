# 0060: Osprey as an emit target — declined

**Date:** 2026-07-09
**Status:** accepted (decision: do not add an Osprey backend)
**Phase:** Phase 57 (raised while hardening the emitter)

## Context

We were asked to evaluate [Osprey](https://www.ospreylang.dev) as a fourth Intent
emit target, alongside the existing Rust, JavaScript, and WebAssembly backends
(ADR 0008 IR, ADR 0009 multi-target codegen).

Osprey, as of v0.10.0 (1 Jul 2026), is:

- A pure-functional language with two first-class syntaxes (brace "Default" flavour and
  offside-rule "ML" flavour), both lowering to one canonical AST; Hindley-Milner inference.
- Built around a **100% compile-time algebraic effects system** — side effects live in a
  function's type and an unhandled effect is a compile error. This is its marquee, and
  genuinely novel, feature.
- Memory-safe with **no GC**, via persistent immutable collections (HAMT maps, 32-way
  vector tries) plus C-level manual memory; concurrency via lightweight fibers + channels.
- Compiled through: flavour-lower -> canonical AST -> HM type-check -> effect-check ->
  optimise -> **LLVM** backend (shells out to `llc` + a C compiler); also emits
  `wasm32-wasip1`. The compiler itself is Rust/C.
- **Alpha:** ~54 GitHub stars, effectively a single maintainer, multiple releases per
  week. Generics, a module system, resumable effect handlers, and list pattern matching
  are on the roadmap but **not yet implemented**.
- Possessed of **no contract/verification features** — no `requires`/`ensures`/`invariant`/
  `old()` equivalent. Its safety story is types + effects, not assertions.

## Options

- **Add an Osprey backend now.** Emit-surface cost is well-isolated: a new
  `internal/ospreybe/` generator (~1,600-2,600 LOC, modelled on `internal/jsbe/jsbe.go`),
  a ~24-line adapter in `internal/backend/`, and small `switch`-arm edits in
  `internal/compiler/target.go`, `cmd/intentc/main.go`, `internal/checker/checker.go`, and
  `internal/compiler/test_runner.go`. No IR/checker changes; no self-hosting parity required
  (the stage2 self-hosted compiler emits Rust only).
- **Decline the backend; treat Osprey as design prior-art.** Record the evaluation and
  revisit Osprey's effects system as a possible input to Intent's *own* language design
  rather than as an output target.
- **Add a different target instead** (Go, C, direct LLVM) better matched to Intent's
  imperative + contract-first semantics, if broader reach is the actual goal.

## Decision

**Do not add an Osprey backend.** The objection is value and fit, not effort:

1. **Semantic impedance.** Intent's IR deliberately preserves imperative control flow
   (ADR 0008: not SSA; `AssignStmt`/`WhileStmt`/`ForInStmt`/`Break`/`Continue`, mutable
   entities with mutating methods, `old()` capture on mutation). Osprey is
   immutable-by-default and functional. Faithful emission would require either Osprey's
   mutable escape hatch everywhere — negating the very thing Osprey sells — or an
   imperative->functional transform the IR is not shaped to provide.
2. **No home for contracts,** which are Intent's defining feature. Osprey has no assertion
   mechanism and an explicit "no runtime panics" ethos; `requires`/`ensures`/`invariant`/
   `old()`/`forall`/`exists` would be bolted on via C-FFI `abort`, against the target's grain.
   This also cuts against ADR 0003 (runtime assertions) and ADR 0009's principle that each
   target enforces contracts in its own *native* idiom.
3. **Missing prerequisites.** Osprey has neither generics nor a module system today. Intent
   uses both heavily, and multi-module emit (`GenerateAll`) landed in Phase 56 — there is
   nothing to target for cross-module output.
4. **No new output surface.** Osprey emits LLVM-native + WASM, both already covered by the
   Rust and WASM backends. It would be a less-mature path to outputs we already produce.
5. **Maturity/volatility risk** relative to the bedrock Rust/JS/WASM toolchains.
6. **Opportunity cost** against the self-hosting priority.

## Consequences

- The backend set stays Rust / JavaScript / WASM. No code changes result from this ADR.
- Osprey's algebraic effects system is flagged as **prior-art worth studying for Intent's
  own design** (effect tracking in signatures, compile-time "unhandled effect = error") —
  a complementary axis to Intent's existing async and contract features. Any such work
  would be a separate design ADR, not a backend.
- **Revisit criteria:** reconsider an Osprey target only if Osprey reaches ~1.0 with
  generics and a module system, *and* a functional / effects-typed output becomes
  independently valuable to Intent.
