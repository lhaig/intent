# 0061: Cross-package code generation — support matrix and fixes

**Date:** 2026-07-10
**Status:** accepted (Phase 59)
**Supersedes:** the "cross-package code generation is not yet fully supported" note in
[DESIGN.md §15.9](../DESIGN.md) and the blanket cross-package build warning.

## Context

Since the package registry shipped ([ADR 0039](0039-package-registry-git-mvs.md), Phase 30)
a project can depend on another package by git URL or path. Dependency discovery,
manifest resolution, and type checking already worked across package boundaries, and
Phase 56 added cross-*module* name mangling for multi-file emit within one package. But
DESIGN.md still declared cross-*package* code generation "not yet fully supported —
only single-package builds produce fully correct output," and every cross-package build
emitted a warning to that effect.

An audit (Phase 59) found that claim was largely stale, in the same way the "registry
remote-fetch is stubbed" ROADMAP line was stale: the common cases already emitted,
compiled, and ran correctly on both Rust and JavaScript. The audit built two-package
fixtures (separate `intent.toml`, path dependency) exercising each feature and ran them
through emit → `rustc`/`node` → execute. It confirmed what worked, and isolated the few
constructs that genuinely broke. This ADR records the resulting support matrix, the fixes
made, and the limitations deliberately deferred.

## What already worked (verified emit → compile → run, Rust and JS)

- Entities across a boundary: struct, constructor, contracts, invariants, methods.
- Free functions called via a **qualified** name (`pkg.fn(...)`).
- Traits: `impl` in the dependency, method called from the consumer.
- Nested cross-package type references (a consumer entity with a field typed as an
  imported entity).
- Enum *declaration* and `match` under the module mangling.

The corpus never caught the failures below because its only committed multi-package
example (`examples/packages/types_pkg`) was entity-only — no enums, no generics.

## Bugs found and fixed

### G1 — Rust enum data-variant construction dropped its fields (fixed)

`generateVariantConstructor` looked up the variant declaration in `g.enums` by the
**mangled** enum name, but that map is keyed by the enum's **original** name. Under a
mangling the lookup missed, so a data variant silently fell through to the unit-variant
path and emitted `LibColor::Custom;` — dropping the payload. In single-file builds the
mangle was a no-op, so the lookup happened to succeed, which is why the corpus stayed
green. Fixed in `internal/rustbe`: resolve the variant by the original name, plus a
cross-module `allEnums` fallback (mirroring the existing `allFunctions`/`allEntities`
maps) and a reverse mangled-name match, so consumer-module construction of an imported
variant recovers its fields.

### G2 — JS enum construction used the unmangled name (fixed)

The enum object is declared under its mangled name (`const LibColor = {...}`) but
construction emitted the bare original name (`Color.Red()`), a `ReferenceError`. Two
faults: `generateVariantConstructor` emitted `expr.EnumName` directly, and
`mangledEnumName` only consulted `classPrefix` (empty in a consumer generator) rather
than `typeOrigins`. Fixed in `internal/jsbe`: emit through the mangled name and make
`mangledEnumName` `typeOrigins`-aware (matching `mangledClassName` and the Rust backend).
JS construction is positional, so no field-name lookup is needed.

### G3 — Rust multi-module generic monomorphization (fixed)

A generic instantiation used in more than one module (e.g. `Pair<Int>` in both a
dependency factory and the entry module) emitted the monomorphized `struct Pair__Int`
and its `impl` **twice** — a duplicate-definition error. Separately, a
generic-instantiation type in a **function signature** in a non-entry module was emitted
with the module prefix on the base name (`-> LibPair`) instead of the monomorphized name
(`-> Pair__Int`), a dangling reference. Monomorphizations are global — they take no
module prefix. Fixed by deduplicating top-level declarations by emitted name in
`GenerateAll` (safe because regular declarations get unique per-module names; only global
monomorphizations collide) and by emitting generic-instantiation types via
`ir.MangleGenericName` in `mapType`.

The stage2 (self-hosted, Intent) backend had its own variant of the same signature bug —
`mangle_ir_type` in `selfhost/compiler/lower.intent` prefixed the generic base name
(`LibPair__Int`). It was fixed the same way: a generic instantiation of a user type keeps
its bare base name and bare args, so the monomorphized name is global. The stage2 backend
already deduplicated, so no dedup change was needed there.

## Deferred limitations (documented, with workarounds)

These remain and are **not** silent-wrong output — they surface as a loud parse or
compile error, and each has a straightforward workaround:

- **Module-qualified type-argument / variant syntax** — `pkg.Generic<T>(...)` and
  `pkg.Enum.Variant(...)` do not parse. *Workaround:* construct with the bare form
  (`Generic<T>(...)`, which resolves through the flattened import namespace) or via a
  factory function exported from the dependency. Both are verified working.
- **WASM `test` declarations** — unsupported per [ADR 0029](0029-in-language-testing.md);
  orthogonal to packaging. A test-free cross-package program emits to WASM.

### Follow-up: unqualified imported function calls (fixed 2026-07-10)

`helper(...)` for a `helper` imported from a dependency was accepted by the checker but
emitted with the calling module's prefix (empty in the entry module) rather than the
defining module's, so it failed to compile. Fixed on both backends with a `funcOrigins`
map (function name → defining module's prefix, mirroring `typeOrigins`): a call that is
not local to the current module uses the defining module's prefix. Unqualified imported
calls now emit, compile, and run; Go unit tests
(`TestGenerateUnqualifiedImportedFunctionCall` in each backend) lock it in.

### Self-hosting parity (internal, not user-facing)

Two cross-module patterns are handled by the primary Go (stage1) compiler but not yet by
the self-hosted stage2 backend. Both are invisible to users — `intentc build` runs stage1,
which emits, compiles, and runs them correctly — but they constrain the byte-equal
`diff-emit-sweep` gate, so the repo's swept programs avoid them:

- A generic whose *only* instantiation is in the entry module of a multi-module build:
  stage2 collects instantiations from the module that both defines and uses the generic,
  so the `examples/packages` demo instantiates its generic through a dependency factory
  (`make_labeled`) rather than directly in the consumer.
- Unqualified calls to imported functions: stage2 emits the bare name, so no swept fixture
  uses that form (the corpus and toolchain call cross-module functions qualified).

Teaching stage2's lowering to fold in the entry module's instantiations and to mangle
unqualified imported calls is deferred to a later self-hosting slice.

## Consequences

- `make diff-emit` grows two fixtures — `multimod_enum` and `multimod_generic` — that
  exercise data-variant construction and cross-module generic monomorphization, keeping
  the fixes byte-equal between stage1 and stage2 (now 35/35). `bootstrap-stage3` still
  holds; `diff-emit-sweep` is unchanged.
- `examples/packages/types_pkg` + `app_pkg` gain a data-carrying enum and a generic
  entity so the flagship cross-package example demonstrates the now-supported features.
- The blanket "cross-package code generation is not yet fully supported" warning is
  removed; DESIGN.md §15.9 is rewritten as the support matrix above with the two deferred
  syntax limitations.

## References

- [ADR 0008](0008-intermediate-representation.md) — internal IR
- [ADR 0009](0009-multi-target-codegen.md) — multi-target backends
- [ADR 0039](0039-package-registry-git-mvs.md) — package registry
