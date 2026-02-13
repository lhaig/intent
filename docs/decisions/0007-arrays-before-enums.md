# 0007: Arrays Before Enums

**Date:** 2025-02-12
**Status:** accepted
**Phase:** 1.3

## Context

Both arrays and enums are needed as composite types. The question is
which to implement first given limited development bandwidth.

## Options

- **Arrays first** -- Unlock data processing, sorting, searching.
- **Enums first** -- Unlock error handling (Result/Option types).
- **Both at once** -- Complete but large implementation surface.

## Decision

Arrays first (Phase 1.3), enums later (Phase 2.1).

## Consequences

- Arrays unlock practical programs: sorting, searching, data processing,
  batch operations. Combined with while loops and print, arrays make the
  language usable for real tasks.
- Enums primarily unlock error handling via Result and Option types.
  Error handling is a robustness concern, not a usability one.
- Usability comes first. A language that can process data but panics on
  errors is more useful than one that handles errors but cannot process
  data.
- Arrays also enable for-each iteration (Phase 1.4), creating a chain
  of unlocked features.
- Enums in Phase 2.1 will build on a more mature type system, making
  their implementation cleaner.
