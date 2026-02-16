# Post-POC Review: Intent Language Viability Analysis

**Date:** 2026-02-16
**Reviewers:** Three independent AI analysts (UX, Technical Architecture, Devil's Advocate)

## Summary

After completing the POC (lexer, parser, checker, codegen, linter, CLI, examples), we conducted a multi-angle review to determine whether Intent should continue and in what direction.

**Verdict: Continue, with architectural pivot to an IR-based pipeline.**

## Review Dimensions

### UX / Developer Experience (Strong)

- AI authorship experience is well-suited: explicit types, regular grammar, comprehensive AGENT.md
- Human audit experience is strong for contracts: English-like logical operators, intent blocks bridge natural language to formal verification
- Syntax is clear with minor issues (`let mutable` verbosity, mandatory unused version strings)
- Error diagnostics are functional but need source context display for human users
- Contract-first workflow is sound but verification is runtime-only

Key finding: Intent blocks with `verified_by` are the most distinctive and valuable feature. The syntax inconsistency between examples and documentation needs resolution.

### Technical Architecture (Solid POC, needs evolution)

- Compiler pipeline is clean and well-structured (8/10)
- 301 tests across 7 test files with strong checker coverage
- Code quality is high for a POC: clean Go idioms, proper error handling

Issues found:
- Missing return type checking in the checker (bug)
- String concatenation detection in codegen is fragile (relies on AST node type, not checked type)
- Enum variant name collisions across enums are not detected
- No IR between checker and codegen limits future evolution
- Direct AST-to-Rust-text generation is fragile for complex features
- Ownership model is absent (all methods use `&mut self`)

### Devil's Advocate (Rigorous challenge)

Strongest challenges:
- Technical delta over "Rust + contract macros" is small; intent blocks are the only clear differentiator
- LLMs have zero Intent training data vs. extensive Rust/Python/TS data
- Runtime-only verification makes "verifiable correctness" claims misleading
- Ecosystem is zero: no packages, no IDE, no community, no FFI to Rust crates
- Competitive landscape: Dafny (Microsoft) already does static verification; Eiffel has 35 years of DbC

What survived the challenge:
- Intent blocks (`verified_by` linking goals to contracts) are genuinely novel
- The AI-authorship thesis is the right question for 2025-2026
- Grammar-enforced contracts (vs. opt-in macros) produce genuinely different structural guarantees
- The pedagogical and experimental value is real

## Position Analysis

We evaluated five positions:

| # | Position | Enforcement | Effort | Differentiation |
|---|----------|-------------|--------|-----------------|
| 1 | Better prompts only | Weakest | Trivial | None |
| 2 | Rust + macros + lints | Medium (opt-in) | Low | Low |
| 3 | Intent transpiler to Rust (current) | Strong (grammar) | Done | Medium |
| 4 | Intent with own IR + Rust codegen | Strong + verifiable | Medium | High |
| 5 | Intent with own backend (LLVM) | Strongest | Very high | Highest |

**Decision: Move from Position 3 to Position 4.**

- Position 2 (Rust tooling layer) was rejected: it surrenders grammar-enforced contracts and makes the AI write Rust (a human-centric language), undermining the core thesis.
- Position 5 (own backend) was rejected: LLVM is massive engineering effort with no user-facing benefit over leveraging Rust's codegen.
- Position 4 gives the best balance: keeps the language, enables static verification (Z3), and enables multi-target output (WASM, JS/TS) through a pluggable backend architecture.

## Architectural Decisions

- **ADR 0008:** Introduce an intermediate representation between checker and codegen
- **ADR 0009:** Multi-target code generation (Rust first, then WASM, then JS/TS)

## Next Steps (Post-POC Completion)

1. Finish current POC stages (in progress by parallel development)
2. Design and implement the Intent IR
3. Refactor Rust codegen to consume IR instead of AST
4. Implement Z3/SMT translation from IR contract nodes
5. Add WASM target (initially via Rust's wasm32 target)

## Success Criteria Update

Original criteria from ADR 0000 remain, with additions:

- [ ] AI writes Intent programs with fewer contract errors than equivalent Rust
- [ ] Human reviewers audit Intent faster than equivalent Rust
- [ ] **NEW:** At least one contract can be statically verified (Z3) at compile time
- [ ] **NEW:** Intent programs can target at least two backends (Rust + WASM)
- [ ] **NEW:** Intent blocks with `verified_by` catch at least one drift between stated goals and actual contracts in a real program
