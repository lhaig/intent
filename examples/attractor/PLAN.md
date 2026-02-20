# Attractor-in-Intent: Bug Fix Plan

## Context

Phase 1 of implementing Attractor pipeline orchestration in Intent is complete. The file `examples/attractor/attractor.intent` (single-file, ~650 lines) passes type-checking and generates 838 lines of Rust via `intentc build --emit-rust`. However, the generated Rust does not compile due to codegen bugs, and the multi-file version doesn't type-check due to checker bugs.

Two parser/checker bugs were already fixed in this session:
- Parser: infinite loop on `constructor` keyword in `verified_by` paths (parser.go)
- Checker: enum types not resolved when used as entity fields — swapped `registerEnums()` before `registerEntities()` (checker.go, 3 call sites)

All existing tests pass after these fixes.

## Bugs to Fix

### Bug 1: Enum default initialization in entity constructors

**File:** `internal/codegen/codegen.go`

**Problem:** When an entity has a field of enum type, the codegen emits:
```rust
status: StageStatus { /* default fields */ },
```
This is invalid Rust — enums are not structs. It should emit a default variant.

**Fix:** When generating the entity constructor's initial struct literal, detect enum-typed fields and emit the first variant as default (e.g., `StageStatus::Success`) instead of `StageStatus { }`. The codegen already has access to enum info from the checker's `EnumInfo` map.

**Test:** `attractor.intent` entities `Outcome` (field `status: StageStatus`) and `Diagnostic` (field `severity: Severity`) should produce valid Rust.

### Bug 2: Entity methods generate `&mut self` instead of `&self`

**File:** `internal/codegen/codegen.go`

**Problem:** All entity methods are generated with `&mut self` receivers:
```rust
fn is_start(&mut self) -> bool {
```
Methods that only read fields (like `is_start`, `is_exit`, `is_conditional`) should use `&self`. When these methods are called on elements accessed via `&Vec<T>` (immutable borrow), Rust's borrow checker rejects it.

**Fix:** Analyze method bodies during codegen — if the method does not assign to `self.field`, generate `&self`. If it does (like `deposit`, `withdraw`), keep `&mut self`. Check if any `self.field = expr` assignments exist in the method's AST.

**Test:** `has_exactly_one_start` calls `nodes[i].is_start()` where `nodes` is `&Vec<NodeAttr>` — this must compile.

### Bug 3: String field access moves instead of borrowing/cloning

**File:** `internal/codegen/codegen.go`

**Problem:** Accessing a `String` field from an indexed entity moves the value:
```rust
edges[i as usize].from_node  // moves from_node out of the Vec element
```
Rust requires `.clone()` for owned String access from borrowed contexts.

**Fix:** When generating field access expressions on entity types where the field is `String`, append `.clone()`. Similarly, when passing String values as function arguments, clone them if the source is a field access or array index.

Scope this conservatively: add `.clone()` to all `String`-typed field access expressions. The performance cost is negligible and avoids complex borrow analysis.

**Test:** `all_edge_targets_exist` passes `edges[i].from_node` and `edges[i].to_node` to functions — these must compile.

### Bug 4: String value moves between function calls

**File:** `internal/codegen/codegen.go`

**Problem:** When a `String` variable is passed to multiple function calls, the first call moves it:
```rust
let step1 = find_condition_matched_edge(&edges, edge_count, outcome_status, preferred_label);
let step2 = find_preferred_label_edge(&edges, edge_count, preferred_label); // ERROR: value moved
```

**Fix:** Generate `.clone()` for `String`-typed arguments passed to functions, OR generate function parameters as `&str` instead of owned `String`. The simpler approach: clone at call sites.

**Test:** `select_edge` passes `preferred_label` to two different functions.

### Bug 5: Cross-module entity/enum type resolution

**File:** `internal/checker/checker.go`

**Problem:** When module A exports `public entity NodeAttr` and module B imports A and uses `Array<NodeAttr>` as a function parameter type, the checker for module B reports `unknown type 'Array'` (actually failing to resolve the type parameter `NodeAttr`).

The issue is in `CheckAll()` — when checking imported modules, the checker creates a temporary checker per module but doesn't inject entities/enums from that module's own imports. Module B's checker gets module A's public types injected into the *main* checker, but module B's own checker instance doesn't see them.

**Fix:** In `CheckAll()`, when creating the checker for each imported module, also inject public entities and enums from *its* transitive imports. The import chain is already resolved (the code walks `prog.Imports`), so this is about propagating types through the chain.

**Test:** The multi-file version (`main.intent` importing `types.intent`, `validation.intent`, etc.) should type-check.

### Bug 6: Empty array literals need typed initialization

**File:** `internal/checker/checker.go` and `internal/codegen/codegen.go`

**Problem:** `let arr: Array<NodeAttr> = [];` fails with "empty array literal requires type annotation (element type cannot be inferred)". The checker rejects empty array literals even when the variable declaration provides the type.

**Fix:** In the checker, when an empty array literal is assigned to a variable with a known `Array<T>` type annotation, infer the element type from the annotation instead of rejecting. The type info is available from the `let` statement's type ref.

**Test:** `Graph` constructor should be able to initialize `self.nodes = [];` when the field is declared as `Array<NodeAttr>`.

### Bug 7: `constructor` not recognized in verified_by paths

**File:** `internal/checker/checker.go` (verifyIntents function)

**Problem:** `verified_by NodeAttr.constructor.requires;` passes the parser (already fixed) but the checker reports "entity 'NodeAttr' has no method 'constructor'". The checker's `verifyIntents` looks up the second part of the path in `entity.Methods`, but constructors are stored separately from methods.

**Fix:** In `verifyIntents`, when resolving a verified_by path like `Entity.constructor.requires`, check for `entity.HasConstructor` instead of looking in the methods map. Accept `constructor` as a special case.

**Test:** The intent block in `types.intent` references `NodeAttr.constructor.requires`.

### Bug 8: verified_by does not support standalone function references

**File:** `internal/checker/checker.go` (verifyIntents function)

**Problem:** `verified_by: [delay_for_attempt.ensures, build_retry_policy.ensures];` reports "unknown entity 'delay_for_attempt'". The checker only looks up verified_by first parts in the entity map, not the function map.

**Fix:** In `verifyIntents`, if the first part of a verified_by path is not found in entities, also check the functions map. Accept `function_name.requires` and `function_name.ensures` as valid references.

**Test:** The intent blocks in `retry.intent` and `validation.intent` reference standalone functions.

## Verification

After all fixes:
1. `go test ./... -timeout 30s` — all existing tests pass
2. `./intentc check examples/attractor/attractor.intent` — no errors (already works)
3. `./intentc build examples/attractor/attractor.intent` — compiles to native binary and runs
4. `./intentc check examples/attractor/main.intent` — multi-file version type-checks
5. `./intentc build examples/attractor/main.intent` — multi-file version compiles
6. `./intentc check examples/bank_account.intent` — existing examples still work
7. `make test && make check-examples && make emit-examples` — full regression

## Files Changed

- `internal/codegen/codegen.go` — Bugs 1, 2, 3, 4
- `internal/checker/checker.go` — Bugs 5, 6, 7, 8
- `internal/parser/parser.go` — Already fixed (parser infinite loop + constructor keyword)

## Execution Order

Bugs 1-4 (codegen) are independent of Bugs 5-8 (checker) and can be parallelized.

**Codegen track:** Bug 1 → Bug 2 → Bug 3 + Bug 4 (3 and 4 are the same fix: clone Strings)
**Checker track:** Bug 5 → Bug 6 → Bug 7 → Bug 8

**Verify after each track completes, then full regression.**
