# ADR 0027: Package Management -- Design Plan

## Status

Accepted

## Implementation Notes

Implemented in Phase 13. The design was followed closely with these notes:

- **TOML parsing**: Uses a lightweight custom parser (key=value and inline tables) rather than a full TOML library, sufficient for `intent.toml` manifests.
- **Package imports**: Parser supports both `import package_name;` and dotted `import package_name.submodule;` syntax. Package imports set `IsPackage=true` on the AST `ImportDecl` node.
- **Version resolution**: Semver parsing, constraint matching (`=`, `^`, `~`, `>=`, `<`), and conflict detection are implemented. Only one version per package in the dependency tree.
- **Cache**: Package cache at `~/.intent/cache/<name>/<version>/` with SHA256 checksum-based invalidation.
- **CLI**: `intentc pkg init`, `intentc pkg add`, `intentc pkg remove`, and `intentc pkg install` are implemented. `intentc build` auto-resolves packages.
- **Local path dependencies**: Supported via inline table syntax `{ path = "../pkg" }`. Local path dependencies are not cached.
- **Registry (Phase 4)**: Not yet implemented. Only local path dependencies and manual cache population are available.

## Context

Intent's multi-file compilation system handles imports within a single project directory. Package management would enable code reuse across projects, versioned dependencies, and a standard library distribution model.

## Design

### Package Definition

Each package is an Intent project with a manifest file `intent.toml` at its root:

```toml
[package]
name = "graph-validation"
version = "1.0.0"
description = "Graph structure validation rules for pipeline systems"
license = "MIT"

[dependencies]
graph-types = "0.2.0"
```

The existing `module <name> version "<semver>";` declarations continue to work for individual files within a package.

### Directory Structure

```
my-project/
  intent.toml
  main.intent          # entry point
  handlers.intent
  types.intent
  deps/                   # resolved dependencies (not committed)
    graph-types/
      intent.toml
      types.intent
      validation.intent
```

### Import Syntax

**Local imports** (unchanged):
```intent
import "handlers.intent";        // relative to project root
import "utils/helpers.intent";   // subdirectory
```

**Package imports** (new):
```intent
import graph_types;                        // import whole package
import graph_types.validation;             // import specific module

let g: graph_types.Graph = graph_types.Graph(...);
```

Package names use underscores (matching Intent's identifier rules). The compiler resolves package names via `intent.toml` dependencies.

### Version Resolution

Semver-compatible resolution:
- `"1.0.0"` -- exact version
- `"^1.0.0"` -- compatible updates (>=1.0.0, <2.0.0)
- `"~1.0.0"` -- patch updates only (>=1.0.0, <1.1.0)
- `">=1.0.0, <2.0.0"` -- range

Only one version of each package allowed in the dependency tree (no diamond dependency resolution needed initially).

### Resolution and Caching

```
~/.intent/
  cache/
    graph-types/
      1.0.0/
        intent.toml
        ...             # .intent files in package root
      1.1.0/
        ...
```

**Resolution flow:**
1. Read `intent.toml` from project root
2. For each dependency, check `~/.intent/cache/`
3. If not cached, fetch from registry or local path
4. Validate version constraints
5. Build dependency order (topological sort, extending existing infra)
6. Compile dependencies first, then project

### CLI Commands

```bash
intentc pkg init                    # create intent.toml
intentc pkg add graph-types 1.0.0   # add dependency
intentc pkg remove graph-types      # remove dependency
intentc pkg install                 # resolve and fetch all deps
intentc pkg list                    # show dependency tree
intentc build                       # unchanged -- auto-resolves packages
```

### Implementation Phases

**Phase 1: Manifest and local paths**
- Parse `intent.toml` (add TOML parser or use simple key=value format)
- Support local path dependencies: `graph-types = { path = "../graph-types" }`
- Extend `ModuleRegistry` to resolve package imports
- Add `import package_name;` and `import package_name.module;` syntax

**Phase 2: Version resolution**
- Implement semver parsing and comparison
- Validate version constraints in dependency tree
- Detect and report version conflicts
- Cache resolved packages in `~/.intent/cache/`

**Phase 3: Package compilation**
- Compile package dependencies to IR cache (avoid re-compilation)
- Store compiled IR alongside source in cache
- Invalidate cache when source changes (checksum-based)

**Phase 4: Registry (optional)**
- Design registry API (REST or git-based)
- `intentc pkg publish` command
- `intentc pkg search` command
- Registry could be a simple git repository initially

### Existing Infrastructure

The compiler already has strong foundations:
- `ModuleRegistry` with BFS import discovery and cycle detection
- `TopologicalSort` for dependency ordering
- Cross-module symbol visibility (public/private)
- Multi-file IR lowering and codegen with name mangling
- `typeOrigins` map for cross-module type resolution

The main additions are:
- `intent.toml` parsing
- Package path resolution (extending `DiscoverDependencies`)
- Cache directory management
- Version constraint validation

### Constraints

- No diamond dependency resolution initially (one version per package)
- No private registries initially (local paths and public registry only)
- No build scripts or conditional compilation
- Packages must be pure Intent (no FFI or native code)
- No workspace/monorepo support initially

### Dependencies

- Benefits from generics (generic packages are more reusable)
- Independent of async/concurrency

## Known Limitations

- **Semver overlap heuristic**: `hasValidOverlap` in `semver.go` uses 999 as a heuristic upper bound for minor/patch version components when probing constraint boundaries. This may produce false negatives (failing to detect valid overlaps) for packages with more than 999 minor or patch releases. In practice this is unlikely, but the overlap check is not exhaustive.

- **Minimal TOML parser**: The `intent.toml` parser in `manifest.go` implements a minimal TOML subset. It does not handle inline comments (`key = "value" # comment`), escape sequences in quoted strings, or multi-line values. Full-line comments (lines starting with `#`) are supported.

- **Closure/Fn type syntax**: The `Fn` type syntax was updated to `Fn(ArgType) -> ReturnType` as a drive-by fix in the closures demo (`examples/closure_demo.intent`). This is intentional and reflects the current parser expectations.

## Consequences

- Enables code sharing across Intent projects
- Standard library could be distributed as a package
- Local path dependencies enable monorepo-style development
- Version resolution prevents breaking changes from propagating
- Cache avoids redundant compilation of dependencies
