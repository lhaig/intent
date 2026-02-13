# 0005: While Loops Before For Loops

**Date:** 2025-02-12
**Status:** accepted
**Phase:** 1.1

## Context

The language needs iteration constructs. The question is ordering:
which loop form to implement first.

## Options

- **While first** -- Simpler semantics, no dependency on iterables.
- **For first** -- More ergonomic, but requires an iterable concept.
- **Both at once** -- Complete but larger implementation surface.

## Decision

While loops first (Phase 1.1), for loops after arrays (Phase 1.4).

## Consequences

- While is simpler: just condition + body, with a direct mapping to
  Rust's while construct. No new type concepts needed.
- For loops require something to iterate over. Arrays arrive in Phase 1.3,
  so for loops follow naturally in Phase 1.4.
- While loops immediately unblock useful examples like fibonacci and
  other algorithmic programs.
- The gap between while (1.1) and for (1.4) is small. Users are not
  stuck without ergonomic iteration for long.
- Implementing while first validates the loop codegen infrastructure
  (break, continue, nested loops) before adding iterator complexity.
