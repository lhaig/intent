# 0030: Cross-Package Test Visibility

**Date:** 2026-05-30
**Status:** accepted
**Phase:** v1.2 (Phase 17, section 17.D)

## Context

Phase 16 / ADR 0029 introduced `test "name" { ... }` declarations and explicitly rejected `public test` because tests "don't cross module boundaries." That decision was correct in the narrow sense — tests should never be callable from user code — but it left a gap: when `intentc test main.intent` is run on a multi-file or multi-package project, only tests in the entry file are discovered. Tests in dependencies are silently ignored.

This breaks the natural expectation that running tests on a project runs *all* the project's tests. It also blocks larger projects from organising tests near the code they exercise (e.g. tests for `types_pkg` living in `types_pkg/`, separate from `app_pkg`).

This ADR resolves the tension by distinguishing two different forms of "visibility":

- **Discovery visibility** — can the test runner find this test?
- **Export visibility** — can other Intent code call this test as a function?

ADR 0029's rejection of `public test` was about export visibility. This ADR addresses discovery visibility.

## Options

### O1. `public test` opt-in

Tests are not discovered across module boundaries by default. Authors opt in with `public test "name" { ... }`. Matches the rest of Intent's visibility model.

### O2. Tests always discoverable, never exported

The runner walks the entry file's transitive import graph and discovers every test in every imported module. Tests remain non-callable from user code — `public test` is rejected as a syntax error (preserving ADR 0029's letter).

### O3. Per-module `test_visibility = "module" | "package" | "project"` config

Manifest-driven, ADR-heavy, premature.

## Decision

**Option O2.** Tests are auto-discoverable across the full import graph from the runner's perspective. They are never exported as callable symbols. The `public test` syntax remains rejected at parse time.

### Concrete behaviour

```
intentc test main.intent
```

discovers tests from:

- `main.intent` (the entry)
- every file directly or transitively imported by `main.intent`
- every package declared in `intent.toml` that is itself transitively reachable

Reporting:

- Test names are prefixed with their source module when there is any ambiguity. E.g. `types_pkg::point_distance` vs `geometry_pkg::point_distance`.
- If no ambiguity exists, the bare test name is used in the output.
- The `--list` flag (17.F) shows the fully-qualified name for every test so users can disambiguate when writing `--filter` queries.

### Edge cases

- **Tests in `intent.toml`-declared but unused dependencies** — not discovered. Discovery follows the import graph, not the manifest. A dependency that is declared but not imported is treated like any other unused declaration: its tests don't run.
- **Tests in generic modules** — discovered like any other test. The monomorphization pass in `ir.rewriteMonomorphizedCalls` (extended in Phase 16 task 16.10) walks test bodies, so generic types in test bodies resolve correctly.
- **Tests with colliding bare names across packages** — both run; report shows them fully qualified. No name-collision rejection at parse time.
- **A test in `types_pkg` exercises a symbol that `app_pkg` overrides** — runs against the symbol as resolved in the test's own module scope, not the entry's. Tests are scoped to where they are written.

## Consequences

**Accepted trade-offs:**

- Larger test runs by default. A user importing a heavy library inherits its test surface. Mitigation: `--filter <substring>` (Phase 17 section 17.F) lets users scope down. Future enhancement: a `--no-deps` flag for "run only my tests."
- Tests in third-party packages run too. If a downstream consumer wants to skip those, `--filter` or `--no-deps` is the workaround.
- Slight runner-side complexity: building the qualified name registry, deciding when to disambiguate.

**Preserved from ADR 0029:**

- Tests are never callable from user code.
- `public test` remains a parse error with the same diagnostic.
- Test bodies cannot `return` and cannot take parameters.

**Relationship to other ADRs:**

- ADR 0027 (package management): tests participate in the same module / file resolution as the rest of the package model. No new manifest fields.
- ADR 0029 (in-language testing): preserved. The `public test` rejection stands as a syntax-level rule; this ADR clarifies that "visibility" in ADR 0029 was about export visibility, not discovery.

**Future work:**

- A `--no-deps` flag (run tests only from the entry module's package) is filed as Phase 18 polish.
- Package-author-controlled visibility (`tests = "private"` in `intent.toml`) deferred until a real "library distributes private tests" need surfaces.
