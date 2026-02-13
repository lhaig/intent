# 0000: Why Intent Exists

**Date:** 2025-02-12
**Status:** accepted
**Phase:** Origin

## Context

AI coding assistants (Copilot, Claude, Cursor, etc.) write code in languages designed for humans: Python, TypeScript, Rust, Go. These languages optimize for human cognitive constraints -- concise syntax, implicit behavior, syntactic sugar -- because humans tire from verbosity.

AI has no such constraint. It can produce and consume arbitrarily verbose, maximally explicit code without fatigue.

The question: **can a programming language designed specifically for AI authorship produce better software than AI writing in human-designed languages?**

## Hypothesis

If a language forces every function to carry preconditions and postconditions, every data type to carry invariants, and every module to carry natural-language intent linked to formal contracts, then:

1. AI-generated code will be more correct (contracts catch bugs at boundaries)
2. AI-generated code will be more auditable (humans read contracts, not implementations)
3. Verification becomes tractable (contracts are machine-checkable specifications)
4. The "why" is preserved (intent blocks survive refactoring)

Human-designed languages can't mandate this level of explicitness because humans won't write it. AI will.

## Decision

Build Intent as a proof-of-concept to test this hypothesis. The language compiles to native binaries (via Rust) so the output is practical, not academic. The compiler is in Go for simplicity.

## Success Criteria

The experiment succeeds if:
- AI can write Intent programs faster than equivalent contracted Rust
- The generated programs have fewer runtime contract violations than uncontracted equivalents
- Human reviewers can audit Intent programs faster than equivalent Rust by reading contracts instead of implementations
- The verification story (Milestone 3) can prove properties that runtime testing alone would miss

## Consequences

- Features are evaluated by whether they advance the AI-authorship thesis, not by popularity or convention
- The language is deliberately verbose where verbosity serves auditability
- Human ergonomics are secondary to machine ergonomics
- The contract system is the product; everything else is infrastructure
