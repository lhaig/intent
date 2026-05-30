# 0031: `@target_specific` Annotation for Tests

**Date:** 2026-05-30
**Status:** accepted (design); implementation deferred to Phase 18
**Phase:** v1.2 (Phase 17 design; Phase 18 implementation)

## Context

Phase 16 / ADR 0029 introduced `intentc test --all-targets`, which runs every test on every supported target (rust, js, wasm) and flags **divergence** — tests that pass on one target but fail on another — as `DIFF`. This is the cross-target equivalence story: same source, same behaviour, everywhere.

But equivalence is not always desirable. Some tests are legitimately target-specific:

- A test exercising a JS-only DOM interaction (Intent can call `console.log` differently on Node vs the browser).
- A test exercising a Rust-only FFI function via `extern function`.
- A test asserting overflow semantics that genuinely differ across targets (Rust panics on `i64` overflow in debug; JS silently produces a `Number` with precision loss).

Without a way to mark these tests, users have two bad choices: either skip cross-target validation entirely (lose the differentiator), or accept noisy `DIFF` reports that the user has to mentally filter every run.

This ADR introduces a syntactic surface for opting individual tests out of cross-target equivalence.

## Options

### O1. Comment-based opt-out

Tests get a special comment marker the runner recognises:

```intent
// @target rust
test "uses ffi" { ... }
```

Pros: zero language surface change, no parser changes. Cons: ad-hoc, easy to typo, no type-check time validation.

### O2. Keyword-prefix opt-out

A new modifier keyword:

```intent
target "rust" test "uses ffi" { ... }
```

Pros: grammar-checked. Cons: yet another contextual keyword; awkward read.

### O3. `@target_specific` annotation

A function-attribute-style annotation that prefixes the test declaration:

```intent
@target_specific("rust")
test "uses ffi" { ... }

@target_specific("rust", "js")
test "runtime introspection" { ... }
```

Pros: extensible (the same annotation surface can later carry other metadata); explicit; reads naturally. Cons: introduces a new top-level syntactic construct (`@<name>(...)`) that doesn't exist elsewhere in Intent.

### O4. Defer entirely

No syntax surface in v1; instead, document the limitation and tell users to write target-specific tests in separate files that they run with `--target` individually.

Pros: zero implementation cost. Cons: bad UX; defeats the cross-target story for any real-world project with legitimate target-specific behaviour.

## Decision

**Option O3.** Add `@target_specific("<target>", ...)` as an annotation that prefixes a test declaration. Multiple targets allowed.

### Syntax

```ebnf
annotation = "@" , identifier , "(" , string_literal , { "," , string_literal } , ")" ;

test_decl = { annotation } , [ "async" ] , "test" , string_literal , block ;
```

Only the `target_specific` annotation is supported in v1. Other annotation names are rejected at parse time with a diagnostic. This keeps the door open for future annotations (`@slow`, `@skip`, `@expect_fail`) without requiring an ADR-per-annotation; new ones will be added as concrete needs arise.

### Semantics

- `@target_specific("rust")` — the test only runs on the rust target. On `--all-targets`, other targets report it as `SKIP` (a third classification alongside `PASS`, `FAIL`, `DIFF`).
- `@target_specific("rust", "js")` — runs on either listed target; targets not listed report `SKIP`.
- `--target rust` of a test marked `@target_specific("js")` — the test is silently skipped (not in the report at all). Exit code unaffected.
- An unannotated test runs on every target the program supports. Its current cross-target behaviour (Phase 16) is unchanged.

### Reporting

`FormatResults` gains a `SKIP` classification distinct from the WASM-rejection skip. The visible difference: WASM-rejection skips name the rejection reason (`"tests not supported on the wasm target"`); annotation skips name the annotation (`"@target_specific(\"rust\") — skipped on js"`).

`AnyFailures` does not flag annotation-skips as failures — they're explicit opt-outs.

### Errors

- Unknown annotation name → parse-time error `unknown test annotation '@<name>'; supported: @target_specific`.
- Empty target list → parse-time error `@target_specific requires at least one target argument`.
- Target string outside {`"rust"`, `"js"`, `"wasm"`} → checker-time error `@target_specific: '<target>' is not a recognised target (expected: rust, js, wasm)`.
- Annotation on a non-test declaration (e.g. a function) → parse-time error `@target_specific is only valid on test declarations`.
- `@target_specific("wasm")` on a test (given WASM rejects tests entirely per 16.6) → checker-time warning `@target_specific("wasm") will never run; WASM rejects all test declarations`. Not an error — the user might be writing forward-looking code.

## Consequences

**Accepted trade-offs:**

- Introduces a new top-level syntactic construct (`@<name>(args)`). The construct is deliberately restricted to one annotation in v1 so the surface stays bounded.
- `--all-targets` reports become slightly more complex (PASS / FAIL / DIFF / SKIP / WASM-SKIP). Mitigation: clear formatting and per-classification counts in the summary line.
- An unannotated test that should be `@target_specific` will be reported as `DIFF` until the user adds the annotation. This is the right default — silent divergence is the failure mode worth catching.

**Differentiator preserved:**

- Cross-target equivalence is still the default. Opting out requires explicit annotation. The "same source, same behaviour" story holds for any test the user hasn't explicitly excluded.

**Future work:**

- `@slow(seconds)`, `@skip(reason)`, `@expect_fail` — natural follow-ups using the same annotation surface. Filed as Phase 18 candidates.
- Annotations on non-test declarations (e.g. `@deprecated function foo` or `@no_inline`) deferred — out of scope for the testing-focused v1 surface.
- A `--no-skipped` runner flag to hide annotation-skips from output (the WASM-rejection skip stays visible because it represents a real limitation, not a user choice).

**Relationship to other ADRs:**

- ADR 0029 (in-language testing): extends the test declaration surface; preserves all existing semantics.
- ADR 0030 (cross-package test visibility): orthogonal — annotations live with the test, not the package.
- Future annotations should be added incrementally; this ADR sets the precedent (one ADR per *family* of annotations, not per annotation).
