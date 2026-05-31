# 0035: LSP `textDocument/references` Scope and Semantics

**Date:** 2026-05-31
**Status:** accepted
**Phase:** v1.2 — Self-Improvement Foundations (LSP capability addition)

## Context

The LSP server today (Phase 18-21) supports hover, goto-def, document-symbol, formatting, signature help, completion, semantic tokens, and member completion. `textDocument/references` — "show me every place this symbol is used" — has been on the deferred list since the original ADR 0032 scoping. With cross-package goto-def now confirmed working (Phase 25), references is the largest remaining LSP capability gap before rename and refactorings.

Every modern LSP ships `textDocument/references`:

- **TypeScript / `tsserver`**: call sites, type references, decorator usages, JSX element references.
- **rust-analyzer**: type refs, trait impls, macro uses, doc-link mentions, derive expansions.
- **gopls**: function call sites, type refs, struct field accesses, method-set membership.
- **clangd**: name lookups, ADL candidates, member accesses.
- **pyright**: call sites, attribute accesses, dunder references.

The protocol is straightforward:

```
Request:  ReferenceParams { textDocument, position, context: { includeDeclaration: bool } }
Response: Location[]
```

What this ADR scopes is **which symbol kinds Intent supports in v1** and **what counts as a reference for each kind**. The implementation shape ("walk every AST in the workspace, match by name + a structural check") is mechanical; the scope is the design decision.

## Options

### O1. Which symbol kinds support references in v1

Intent's symbols fall into eight buckets:

1. Top-level functions (incl. extern)
2. Top-level entities (struct-like types)
3. Top-level enums (and their variants)
4. Top-level traits
5. Top-level tests
6. Entity methods + constructors
7. Entity fields
8. Locals + parameters + `self`

**A. Top-level decls only.** Cover 1-5. Cheapest; matches "I want to know who calls this function."
- Pros: small scope, well-defined, no expression-typing required.
- Cons: doesn't answer "who reads `balance` on `Account`" or "where does this local appear in this function" — both common refactoring questions.

**B. Top-level decls + locals/params.** [Chosen.] Covers 1-5 + 8. Locals are bounded to their enclosing function, so the scan is cheap and unambiguous.
- Pros: covers the majority of "find all uses of `x`" workflows. Same scope walker the resolver already uses (`internal/lsp/scope.go`).
- Cons: still doesn't cover methods/fields on entities.

**C. All symbol kinds.** Covers 1-8. Methods/fields require disambiguating across entities ("which `balance`?").
- Pros: complete.
- Cons: method/field disambiguation needs receiver-type resolution; same as Phase 21's member completion. The Phase 21 infrastructure (`receiverBeforeMember` + `resolveMemberOnReceiver`) handles single-step receivers — multi-step chains stay deferred. Substantially more work for marginal v1 gain.

### O2. What counts as a reference for each kind

- **Function**: every `CallExpr` whose `Function` matches by name (or by member-call resolution to the same target). Plus any type-position occurrence (rare — functions aren't types). Excludes the declaration itself unless `includeDeclaration: true`.
- **Entity / Enum / Trait**: every type-position occurrence (variable declarations, function-parameter types, function-return types, `TypeRef` inside generic args). Plus every constructor call site for entities (`EntityName(...)` call site). Plus every variant-construction site for enum variants.
- **Test**: just the declaration site. Tests aren't referenced from code; the reference list is the declaration unless `includeDeclaration: false`, in which case it's empty. Cheap edge case.
- **Local / param / self**: every `VarRef` whose name matches within the scope walker's enclosing-function frame. Cross-function locals don't exist; scan stays in one function body.

### O3. `includeDeclaration` handling

Per LSP spec, the request's `context.includeDeclaration` controls whether the declaration site appears in the result. v1 honours it:

- `true` (default many clients send): declaration's name range first, then reference sites in source order.
- `false`: declaration omitted; reference sites only.

### O4. Cross-package scope

Same plumbing as cross-package goto-def (Phase 25): `workspace.siblingModules()` returns every AST in the dependency graph. References iterates them all. No new infrastructure.

### O5. Same-name ambiguity

Two functions with the same name in different modules: which references count?

**A. Conflate by name only.** [Chosen for v1.] Walk every AST in the workspace; match by symbol name. If two modules each declare `add()`, references on either include call sites from both. Documented as a known v1 limitation in INTENT.md and the VS Code README.

**B. Disambiguate by declaration site.** Re-resolve every candidate reference from its source position and check whether the resolved declaration's `(Path, Line, Column)` matches the target's. Sound but O(N × resolver-cost) per request. Defer to a future PRD if a real workspace exposes the limitation.

For locals/params/`self`, ambiguity isn't a question: the walker is bounded to the enclosing function's body (the scope walker already gives us that frame), so a `let x` in function A and a `let x` in function B never appear in the same reference list. Shadowing within a function is also clean — the inner `let x` only sees uses below its declaration in source order; the outer `x` sees the rest.

The same-name limitation matters in practice mostly for utility names (`new`, `default`, `add`). Intent's `public` keyword means cross-module collisions tend to be explicit; the user is aware that two modules export `add` and the false positives are easy to filter mentally. The trade-off favours simpler, faster v1; revisit when a real workspace surfaces a pain point.

### O6. Performance shape

Files are small (most under 200 LOC), workspaces are small (under 100 files in any realistic Intent project today), and the AST walk per file is linear. No caching needed for v1. If a workspace grows past where this becomes painful, add a name-keyed index — that's a future PRD.

### O7. Workspace boundary

`textDocument/references` returns ASTs from the open file's workspace only. A single-file open (no `intent.toml`) returns refs from that file. A multi-file open returns refs from the workspace's dependency graph.

## Decision

**O1.B + O2 (per-kind reference semantics above) + O3 (honour includeDeclaration) + O4 (cross-package) + O5.A (name-match only; same-name across modules is a known v1 limitation) + O6 (linear scan, no cache) + O7 (workspace scope).**

1. Add `textDocument/references` to the LSP server's `initialize` capabilities (`referencesProvider: true`).
2. Add a `references.go` handler in `internal/lsp/` that:
   - Resolves the symbol at the cursor via the existing `resolveAtPosition` machinery (same path as goto-def).
   - Dispatches on the resolved kind (function / entity / enum / trait / test / local-let / local-param / self).
   - Walks every AST in `workspace.siblingModules()` (plus the open document's AST), collecting reference positions.
   - Filters by declaration-site identity where needed (O5).
   - Returns `Location[]` per LSP spec, with `includeDeclaration` honoured.
3. Method/field references are deferred to a follow-on phase. The current scope walker already resolves `receiver.method` for hover/goto-def; references on those needs an additional walk that disambiguates against each receiver's entity. Defer until a real user asks.

## Precedent

| LSP | Symbol kinds covered | Notes |
|---|---|---|
| TypeScript (`tsserver`) | Functions, types, decorators, JSX, members | Most aggressive; uses program-wide index. |
| rust-analyzer | Functions, types, traits, fields, methods, macros, doc-links | Backed by salsa-cached index. |
| gopls | Functions, types, fields, methods | Per-package index; cross-package via go.mod. |
| clangd | All C++ name kinds | Backed by clang's AST. |
| pyright | Functions, classes, attributes, dunders | Per-module type-evaluator. |
| metals (Scala) | Functions, types, fields, methods | Index-based; supports incremental updates. |

Intent's v1.2 scope (top-level + locals, brute-force scan) is intentionally smaller than rust-analyzer's. The "find references on a method" feature is the obvious next request; this ADR documents why it's a separate phase (receiver-type disambiguation).

## Consequences

**Accepted trade-offs:**

- Method / field references don't work in v1.2. Users searching for "every place `deposit` is called" on `BankAccount` won't get a result; goto-def from a call site still works as a workaround.
- Brute-force scan means a `references` call on a popular function in a large workspace scans every AST. For Intent's current scale this is fine (microseconds); a future indexed approach lands when needed.
- Same-name disambiguation requires the resolver to be deterministic about which `add` you mean. The scope walker is — but a future refactor could surface edge cases.

**Things this enables:**

- "Show all uses" / "Find all references" in VS Code works for the symbol kinds most code edits care about.
- Rename refactoring (follow-on phase) builds directly on this — `rename` is `references` + `WorkspaceEdit`.
- Code actions that depend on "is this symbol used elsewhere?" (e.g., "remove unused function") gain a query primitive.

**Out of scope (separate ADR/PRD if anyone wants them):**

- Method / field references (needs receiver-type disambiguation across the workspace).
- Trait-method references / impl-site enumeration (needs trait-resolution machinery).
- Document highlights (`textDocument/documentHighlight`) — different LSP method but same shape; could reuse the same handler.
- Workspace symbol search (`workspace/symbol`) — different mechanism; future.
- Indexed reference search for large workspaces — performance optimisation, not a capability.
