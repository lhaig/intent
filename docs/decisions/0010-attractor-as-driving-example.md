# 0010: Attractor as Driving Example for Language Development

**Date:** 2026-02-20
**Status:** accepted
**Phase:** post-v1.0

## Context

After shipping the v1.0 MVP, the language roadmap contained many potential features (generics, traits, string interpolation, maps, I/O, etc.) but no clear prioritization signal. Abstract feature planning risks building things nobody uses or building them in the wrong order.

We needed a real-world project complex enough to stress-test the language and expose gaps organically.

## Options

- **Abstract roadmap** -- Prioritize features based on language design intuition. Risk: features may not compose well in practice.
- **Multiple small examples** -- Write many small programs to exercise different features. Risk: toy examples miss integration issues.
- **Single large driving example** -- Implement a substantial real-world spec in Intent. Risk: one project may bias feature priorities.

## Decision

Use the Attractor pipeline orchestration spec as the primary driver for Intent language development. Attractor is a DOT-based AI pipeline orchestration system with a detailed spec covering type models, execution engines, retry policies, graph validation, condition evaluation, and I/O.

The approach: implement Attractor in Intent incrementally. Each phase expands the implementation, discovers what Intent cannot yet express, and those gaps become the next language features to build.

## Consequences

- Feature prioritization is driven by real usage, not speculation. The 8 codegen/checker bugs found in Phase 1 would never have been discovered with toy examples.
- The roadmap becomes concrete: String standard library is next because condition parsing needs it, not because "strings are useful."
- Risk of single-project bias is mitigated because Attractor is genuinely complex -- it exercises types, enums, pattern matching, arrays, error handling, modules, contracts, and eventually traits, maps, I/O, and concurrency.
- Progress is measurable: each phase expands the percentage of the Attractor spec that Intent can express (currently ~40%).
- The strategy is documented in `examples/attractor/STRATEGY.md` with a gap-to-feature mapping and phased plan.
