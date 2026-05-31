# 0036: Entity and Method Auto-Test Emission for `--target intent`

**Date:** 2026-05-31
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (Phase 17.A.1 prerequisite for legacy Rust-testgen retirement)

## Context

`intentc test-gen` is the contract-derived auto-test generator. It walks the input program's contracts (`requires` / `ensures` / `invariant`) and emits test code that exercises them. Two emission targets exist today:

- `--target rust` (legacy default). Generates Rust property-style tests with PRNG-driven input synthesis. Handles standalone functions, entities, methods, multi-param iteration, and complex constraints. ~520 lines in `internal/testgen/rustutil.go`.
- `--target intent` (Phase 16, partial). Generates `.intent` source files consumed by `intentc test`. Handles only standalone functions with Int parameters and deterministic ranges. Entities, methods, and complex param types fall through to a "TODO" comment in the output.

The strategic intent (Phase 17 PRD, HARNESS.md §7) is to retire the Rust emission path entirely. Two prerequisites block that retirement, codified in Phase 17.A.1 (entity / method emission) and Phase 17.A.2 (multi-param iteration). This ADR scopes the first prerequisite. Without it, retiring the Rust path silently loses entity-test coverage for every user.

The design question is **how `--target intent` should construct entities and exercise their methods** when the only inputs available are the entity's source declaration: constructor signature, constructor contracts, field declarations, methods, and method contracts. No PRNG, no Z3 — those are escalations belonging in future ADRs. The v1 strategy must be deterministic, fast, and produce tests that compile and type-check.

Comparable systems:

- **Dafny:** doesn't generate runtime tests at all. Proves contracts statically.
- **JML (Java) + jmlc:** runtime-checks contracts but doesn't auto-construct objects; user provides fixtures.
- **Microsoft Pex (legacy):** used Z3 to synthesise inputs satisfying `requires`. Complex; deprecated by Microsoft. Not the v1 model.
- **QuickCheck / Hypothesis / fast-check (PBT):** user provides input generators; no auto-construction.
- **AutoTest (Eiffel):** randomised test-case generation against contracts; Eiffel uses default-value heuristics.
- **Intent's existing Rust path:** PRNG over input ranges. Real but unsound (no guarantee `requires` is satisfied).

The closest v1 fit is the **AutoTest** model: derive a default for each constructor / method parameter from its declared type, emit a single example test per (entity, method) pair, and document the limitation that values may not satisfy non-trivial `requires` clauses. Future ADRs can layer Z3-driven input synthesis on top.

## Options

### O1. Entity construction strategy

**A. Default-value heuristic per param type.** [Chosen.] Reuse the existing `defaultArgFor` table from standalone function emission (Int → 1, Float → 1.0, Bool → true, String → "", Array → [], unknown → `/* TODO: provide a value */`). Constructor invocation becomes `Entity(default(p1), default(p2), ...)`.

**B. User-supplied fixtures.** Require the user to annotate entities with `@testfixture` constructors. Adds annotation surface and burden. Out for v1.

**C. Z3-driven input synthesis.** Have testgen invoke `intentc verify` against the constructor's `requires` to synthesise satisfying inputs. Real, but couples test-gen to the verify subsystem and adds a Z3 dependency to the generator. Separate ADR.

### O2. What about constructors whose `requires` rejects the defaults?

**A. Emit the test anyway with a "may panic" comment.** [Chosen.] The runner will hit the precondition assert, fail visibly, and the user knows to write a hand-tuned test. Better than emitting nothing — the failure is informative.

**B. Skip the test entirely.** Silently drops coverage; user might not notice the entity is uncovered.

**C. Try multiple default candidates** (e.g., try Int=1, then Int=0, then Int=-1). Combinatorial blow-up for multi-param. Defer.

### O3. Method call site shape

**A. Mutable entity binding per test.** [Chosen.] `let mutable a: Entity = Entity(...);` then `a.method(args);`. Methods that mutate `self.field` need the binding to be `mutable`.

**B. Immutable per call.** Doesn't work for any entity with state-mutating methods (the common case). Rejected.

### O4. Asserting `ensures` clauses

Method `ensures` may reference:
- `self.field` — the new field value after the call. Test rewrites to `a.field`.
- `result` — the method's return value. Test captures into `__r` (same as standalone-function emission).
- `old(expr)` — value at method entry. Test captures into a generated local before the call (`let __old_0: T = <expr>;`).
- Other method parameters by name. No rewrite needed; param names match.

**Rewriting strategy:**

- Pre-call: scan the method's `ensures` clauses for `old(<expr>)` references; emit one `let __old_<i>: T = <expr>;` per unique sub-expression. The receiver-rewrite from `self` to the binding name (`a`) applies to the captured expression too.
- Call: `a.method(args)` for `Void`; `let __r: T = a.method(args);` for non-`Void`.
- Per ensures clause: emit `assert(<rewritten-clause>);` where:
  - `self` → `a`
  - `old(<expr>)` → `__old_<i>` for the matching capture
  - `result` → `__r`

### O5. Type detection for `old()` capture lets

`let __old_0: T = a.balance;` — what's `T`? The clause's source text says `old(self.balance)`. We need the type of `self.balance`. The entity declaration carries field types; we look up `balance` in `entity.Fields` and write its declared type.

If the expression inside `old(...)` isn't a simple field access (e.g., `old(self.balance + 1)`), the type isn't trivially derivable. v1 falls back to `Int` (the most common case) with a comment. Future versions can do better; deferred.

### O6. Methods with no `ensures` clauses

A method like `method log(msg: String) returns Void { ... }` has no `ensures`. There's nothing to assert. Two choices:

**A. Skip such methods.** [Chosen.] Generating a test that just calls the method with no assertion adds no signal — and Phase 16's standalone-function logic already skips functions with neither `requires` nor `ensures`. Consistent.

**B. Emit a "smoke test" that just calls the method.** Adds coverage but no contract validation. Future enhancement; not v1.

### O7. Methods with `requires`

Default args may fail the method's `requires`. Same trade-off as O2: emit the test anyway with a "may panic on preconditions" header comment, so the failure is visible and informative.

### O8. Generic entities

Entities declared as `entity Stack<T> { ... }` need a concrete `T` to construct. v1 skips them and emits a `// TODO: generic entity Stack<T> requires concrete instantiation` comment. Generic instantiation in test-gen is a separate piece of work; future ADR.

### O9. Constructor-less entities (data-only)

If an entity has no `constructor` declaration but does have invariants, the test would need to construct the entity through field initialisation. Intent's surface today doesn't allow `Entity { x = 1 }`-style struct literals from outside the entity's module without a constructor. Skip with a clear note.

### O10. Output layout

Per existing convention, generated tests live in a sibling `<name>_test.intent` file or pipe to stdout. No change for entity tests — they sit alongside standalone-function tests in the same file.

## Decision

**O1.A + O2.A + O3.A + O4 (per-clause rewrite strategy) + O5 (field-type lookup; fallback Int with comment) + O6.A + O7 (emit with header comment) + O8 (skip generics) + O9 (skip constructor-less) + O10 (no layout change).**

1. Add `emitEntityTests(sb, prog)` to `internal/testgen/intentgen.go`. Walks `prog.Entities`, skipping entities without a constructor, generic entities, and entities whose methods have no contract clauses.

2. For each remaining `(entity, method)` pair where the method has at least one `requires` or `ensures` clause:
   - Construct the entity once at the top of the test using default-value-derived arguments.
   - Capture each unique `old(<expr>)` sub-expression appearing in the method's `ensures` into a `let __old_<i>` binding (`<expr>` is rewritten from `self.x` to `a.x` before capture).
   - Call the method with default-value args.
   - Emit one `assert(<rewritten-clause>);` per `ensures` clause, with `self` → `a`, `old(<expr>)` → `__old_<i>`, `result` → `__r` substitutions.
   - Prepend a header comment when default args may violate a `requires` — `// note: default args may not satisfy constructor / method requires; if this test panics, hand-write the call site.`.

3. Drop the "Entity / method auto-tests are not emitted by --target intent yet" TODO comment in `intentgen.go`. Replace with a one-line note explaining the limitations that remain (generics, constructor-less entities, multi-method-call sequences).

4. The legacy `--target rust` path stays untouched in this phase. Retiring it is a separate phase that depends on Phase 17.A.2 (multi-param iteration for free functions) ALSO landing.

5. No new CLI flags. The existing `--target intent` covers entities once this lands.

## Consequences

**Accepted trade-offs:**

- Default-value tests may fail when constructor/method `requires` reject the defaults. Generated header comment makes this visible.
- `old()` capture-type fallback to `Int` is wrong for non-Int simple expressions. Acceptable v1; future ADR can read checker type info.
- Generic entities and constructor-less entities get a "TODO" comment instead of a test. Documented; users hand-write.
- The Rust path stays present. Retirement is conditional on 17.A.2 (multi-param) landing too.

**Things this enables:**

- `intentc test-gen --target intent examples/bank_account.intent` produces useful entity tests instead of TODO stubs.
- Phase 17.A retirement of the Rust testgen path moves one step closer (just 17.A.2 multi-param iteration left as a blocker, then the Rust path can go).
- The `auto:` test naming convention sticks — same as standalone-function emission — so users can scan the generated file and recognise auto vs. hand-written tests at a glance.

**Things this defers:**

- Z3-driven input synthesis (would satisfy `requires` correctly; complex).
- Multi-call sequences (test only calls the method once; real workflows often need setup-then-method-call-then-assert; future PRD).
- Generic-entity instantiation (needs concrete-type synthesis).
- Constructor-less entity coverage.
- Field-type lookup for non-field `old()` expressions.
- Smoke tests for contract-less methods.
- Snapshot / coverage testing (the larger Phase 17.H bucket).

**Open follow-ups:**

- Phase 17.A.2 (multi-param iteration) is the next prerequisite for Rust-path retirement.
- A future ADR can scope Z3-driven input synthesis for both this generator and a possible verify-aware `--release` mode (cross-cutting concern; same Z3 plumbing).
