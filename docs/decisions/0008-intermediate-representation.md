# 0008: Introduce an Intermediate Representation (IR)

**Date:** 2026-02-16
**Status:** accepted
**Phase:** Post-POC architecture

## Context

After completing the POC, we conducted a multi-angle review of Intent's
architecture and viability with three independent analyses: UX/developer
experience, technical architecture, and a devil's advocate challenge of the
core premise.

The current compiler pipeline is:

```
.intent source -> Lexer -> Parser -> Checker -> Rust text codegen -> cargo
```

The codegen phase operates directly on the AST and emits Rust source code as
strings. This works for the POC but creates several compounding problems:

1. **Fragile code generation.** The labeled block pattern (`'body: { break
   'body expr; }`) for postcondition capture, `__old_` variable generation,
   and `__check_invariants` insertion are all string-level manipulations.
   Adding complexity (closures, nested functions, async) will make this
   brittle.

2. **No path to static verification.** ADR 0003 deferred Z3/SMT integration
   to Milestone 3, but the current architecture has no representation where
   contract semantics can be analyzed. Contracts are lowered directly to
   `assert!()` text -- there is no intermediate form where a theorem prover
   could reason about them.

3. **Single backend lock-in.** The Rust text codegen is the only output path.
   Adding other targets (WASM, JavaScript/TypeScript, LLVM) would require
   building entirely separate codegen passes that duplicate AST traversal and
   contract lowering logic.

4. **No optimization opportunity.** Without an IR, there is nowhere to perform
   contract simplification, dead contract elimination, or contract
   strengthening before code generation.

We evaluated five positions on a spectrum from "tooling layer on Rust" to
"full independent backend," and concluded that the current position
(transpile to Rust) is correct but needs an IR to enable the next phase.

## Options

- **Continue with direct AST-to-Rust codegen.** Least effort, but compounds
  all problems above. Every new feature makes the codegen harder to maintain.

- **Introduce an IR between checking and codegen.** The checker lowers the
  annotated AST to an IR that preserves contract semantics as first-class
  nodes. Codegen backends consume the IR. Verification tools (Z3) also
  consume the IR.

- **Adopt an existing IR (LLVM, MIR, MLIR).** Maximum leverage, but these
  IRs have no concept of contracts, preconditions, or postconditions. We
  would lose the contract semantics that are Intent's core value.

- **Build a Rust tooling layer instead.** Abandon the language, deliver
  intent blocks as Rust proc macros. Rejected because it surrenders
  grammar-enforced contracts (the core thesis) and makes contracts opt-in
  rather than mandatory.

## Decision

Introduce a custom Intent IR between the checker and codegen phases. The
new pipeline becomes:

```
.intent source -> Lexer -> Parser -> Checker -> IR Lowering -> IR
                                                                |
                                                    +-----------+-----------+
                                                    |           |           |
                                                  Rust      Verify       Future
                                                 codegen    (Z3/SMT)    backends
```

The IR must preserve:
- **Contract nodes** as first-class elements (not assertion strings). A
  `Requires` IR node carries the original expression tree, not `assert!()`
  text.
- **`old()` semantics** as explicit pre-state capture nodes, not generated
  variable names.
- **Intent block traceability** linking goals to contract nodes by reference,
  not by string path resolution.
- **Type information** from the checker, attached to every expression node.

The IR does NOT need to be SSA (Static Single Assignment) form initially.
A structured IR preserving Intent's control flow is sufficient for the
first iteration.

## Consequences

- Rust codegen becomes a backend that consumes the IR rather than walking
  the AST directly. The existing codegen can be refactored incrementally.
- Z3/SMT integration (Milestone 3) becomes tractable: translate IR contract
  nodes to SMT-LIB formulae rather than trying to extract semantics from
  Rust assertion strings.
- Additional codegen backends (WASM, JS/TS) become pluggable -- they
  consume the same IR and implement contract enforcement in their target's
  idiom.
- The IR is a new ~2000-3000 LOC package that must be designed, built, and
  tested. This is significant but bounded effort.
- The existing AST and checker remain unchanged. The IR is an additional
  lowering step, not a replacement.
- Contract optimizations become possible at the IR level: removing redundant
  invariant checks, simplifying tautological contracts, merging quantifier
  loops.
