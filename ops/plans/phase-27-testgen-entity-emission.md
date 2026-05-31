# Phase 27: `--target intent` Entity / Method Auto-Test Emission

**Status:** In Progress
**Milestone:** v1.2 — Self-Improvement Foundations (Phase 17.A.1 prerequisite for legacy Rust-testgen retirement)
**Decision:** [ADR 0036](../../docs/decisions/0036-testgen-entity-method-emission.md)

## Goal

Extend `intentc test-gen --target intent` to emit auto-tests for entities and their methods. Today the generator stops at standalone functions; entities produce a single "TODO" comment in the output. After this phase, a contract-bearing entity gets one auto-test per (entity, method) pair, exercising the constructor and the method's `requires` / `ensures` clauses against default-value-derived inputs.

This closes Phase 17.A.1 — one of the two blockers (the other is 17.A.2 multi-param iteration) for retiring the legacy Rust testgen path.

## Success Criteria

- [ ] `intentc test-gen --target intent examples/bank_account.intent` emits one auto-test per method on `BankAccount` instead of the current TODO stub
- [ ] Generated tests use `let mutable a: Entity = Entity(...);` to enable mutating-method calls
- [ ] `old(<expr>)` references in method `ensures` are captured via `let __old_<i>: T = <expr>;` before the method call
- [ ] `self.x` references in method `ensures` are rewritten to `a.x` in the emitted assertion
- [ ] `result` references rewrite to `__r` for non-Void methods
- [ ] Methods with no `requires` / `ensures` are skipped (no signal in emitting a test)
- [ ] Entities without a constructor are skipped with a TODO comment
- [ ] Generic entities are skipped with a TODO comment
- [ ] When default args may violate a constructor / method `requires`, a header comment surfaces the trade
- [ ] Existing standalone-function emission is unaffected
- [ ] Generated tests parse and type-check (i.e., `intentc check` on the output file succeeds)
- [ ] Where possible, generated tests actually pass when run (i.e., `intentc test` on the output passes for examples whose defaults satisfy `requires`)
- [ ] New unit tests cover: entity with constructor + invariant, method with `old()` in ensures, method with `result` reference, method with no contracts (skipped), generic entity (skipped), constructor-less entity (skipped)
- [ ] `make validate` green
- [ ] No regression in existing `intentgen_test.go` tests

## Reference

- ADR 0036: `docs/decisions/0036-testgen-entity-method-emission.md`
- Existing intent emission: `internal/testgen/intentgen.go` (extended in this phase)
- Existing standalone-function logic: `internal/testgen/intentgen.go` — `generateIntentTestForFunction`
- Default-arg table: `internal/testgen/intentgen.go` — `defaultArgFor`
- Bank-account example: `examples/bank_account.intent` (canonical test target)
- Phase 17 PRD: `ops/plans/phase-17-testing-polish.md` §17.A.1
- Existing intentgen tests: `internal/testgen/intentgen_test.go`

## Tasks

### 27.1 Entity walk + per-method test scaffolding

**Files:** `internal/testgen/intentgen.go`

After the existing standalone-function loop, add `emitEntityTests(sb, prog)`. It iterates `prog.Entities`, skipping:

- Entities without a `Constructor` (TODO comment in output)
- Entities with `TypeParams` (generic — TODO comment in output)
- Entities whose methods all lack `requires` and `ensures` (no signal)

For each remaining entity, iterate `entity.Methods`. For each method with at least one `requires` or `ensures`, emit one test block named `test "auto: <Entity>.<method>"`.

**Acceptance:** Unit test asserts `BankAccount` produces 3 entity tests (deposit, withdraw, get_balance has no ensures → skip if ADR 0036 §O6 holds — actually get_balance has no contracts in the current example).

### 27.2 Constructor call site + default args

**Files:** `internal/testgen/intentgen.go`

For each entity test, emit:

```intent
let mutable a: <Entity> = <Entity>(<default-args>);
```

`<default-args>` is comma-joined `defaultArgFor(param)` over the constructor's `Params`. The existing `defaultArgFor` handles Int / Float / Bool / String / Array; unknown types emit a `/* TODO: provide a value */` placeholder.

If the constructor has any `requires` clause, prepend a header comment:

```
// note: default args may not satisfy this entity's constructor requires.
// If this test panics on a precondition, hand-write the constructor args.
```

**Acceptance:** Test for `BankAccount(owner: String, initial_balance: Int)` produces `let mutable a: BankAccount = BankAccount("", 1);`.

### 27.3 `old()` capture before method call

**Files:** `internal/testgen/intentgen.go`

Scan the method's `ensures` clauses' `RawText` for `old(<expr>)` patterns. For each unique sub-expression, emit a `let __old_<i>: T = <rewritten-expr>;` *before* the method call. Rewrite `self` → `a` in the captured expression.

Type `T`: if the expression is `self.<field>`, look up the field's declared type on the entity and use it. Otherwise fall back to `Int` with a comment.

**Acceptance:** Test on `deposit` (which has `ensures self.balance == old(self.balance) + amount`) produces:

```intent
let __old_0: Int = a.balance;
a.deposit(1);
assert(a.balance == __old_0 + 1);
```

### 27.4 Method call + assert rewrites

**Files:** `internal/testgen/intentgen.go`

After the `old()` captures:

- For `Void` return type: `a.<method>(<default-args>);`
- For non-`Void`: `let __r: <ReturnType> = a.<method>(<default-args>);`

For each `ensures` clause, emit `assert(<rewritten-clause>);` where:
- `self.<x>` → `a.<x>` (string replacement on the contract's `RawText`)
- `old(<expr>)` → `__old_<i>` (mapping back to the captures)
- `result` → `__r` (existing standalone-function logic applies)

**Acceptance:** Test on `withdraw` produces an assertion using `a.balance` and `__r` (since withdraw returns `Bool`).

### 27.5 Skip-and-comment cases

**Files:** `internal/testgen/intentgen.go`

For each skipped entity, emit a single-line `// auto-test: <Entity> skipped (<reason>)` comment so the user can see why.

**Acceptance:** A generic entity in the test input produces the skip comment; the output otherwise contains no `<Entity>` test.

### 27.6 Tests

**Files:** `internal/testgen/intentgen_test.go`

Add unit tests covering:

- Entity with constructor + one mutating method with `old()` → expected emission shape
- Entity with non-`Void` method using `result` → `__r` substitution
- Entity with method that has no contracts → skipped
- Generic entity → skipped with TODO comment
- Constructor-less entity → skipped with TODO comment
- Smoke: parse the generated output via `parser.New(...)` and assert it has no errors

**Acceptance:** All new tests pass; existing `intentgen_test.go` tests still pass.

### 27.7 Integration check on bank_account.intent

**Files:** `internal/testgen/intentgen_test.go` (or new file)

End-to-end: load `examples/bank_account.intent`, run `GenerateIntent`, parse the output, type-check it, and ideally `intentc test` it. Assert it parses, type-checks clean (modulo `public` requirements documented in Phase 16), and contains the expected method-test blocks.

**Acceptance:** Test passes; output is consumable by the rest of the toolchain.

### 27.8 Docs

**Files:** `docs/ROADMAP.md`, `INTENT.md`, `ops/plans/phase-27-testgen-entity-emission.md`, `ops/plans/phase-17-testing-polish.md` (note 17.A.1 satisfied)

- ROADMAP: `### Phase 27: testgen Entity/Method Emission -- SHIPPED (date)` under v1.2.
- INTENT.md (test-gen section, if present): drop the "entities / methods not emitted by --target intent" caveat.
- Phase 17 PRD: add a note on 17.A.1 — "satisfied by Phase 27."
- This PRD: status flip + checkbox ticks.

**Acceptance:** `make validate` green; no stale "entities deferred" claims in repo docs.

## Out of Scope

- **Z3-driven input synthesis** to satisfy `requires` correctly. Future ADR.
- **Multi-call sequences** (e.g., construct → mutate → query). v1 emits one call per test.
- **Generic-entity instantiation** for test generation. Out for v1; future ADR.
- **Constructor-less entities** (data-only types). Skipped with TODO.
- **Field-type lookup for non-field `old()` expressions** (e.g., `old(self.balance + 1)`). Falls back to `Int` with a comment.
- **Smoke tests for contract-less methods.** A method with no `requires` / `ensures` is skipped, not auto-tested.
- **Phase 17.A.2 multi-param iteration.** Separate prerequisite for Rust-path retirement.
- **Actually retiring the Rust path.** That happens after both 17.A.1 (this PRD) and 17.A.2 land.

## Suggested Order

1. **27.1 Entity walk + skip logic** — scaffolds the iteration without emitting test bodies yet
2. **27.2 Constructor call site** — entity construction with default args
3. **27.3 `old()` capture** — needed before method-call emission
4. **27.4 Method call + assert rewrites** — the core emission shape
5. **27.5 Skip-and-comment** — UX polish for the "we skipped this and here's why" cases
6. **27.6 Unit tests** — lock the emission shape
7. **27.7 Integration check** — end-to-end on a real example
8. **27.8 Docs + PRD flip** — last
