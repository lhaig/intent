# Phase 13: Package Management

## Goal

Enable cross-project code reuse through a package system with `intent.toml` manifests, semver dependency resolution, local path dependencies, and a compiled IR cache.

## Success Criteria

- [ ] `intent.toml` manifest file parsed and used by compiler
- [ ] `import graph_types;` resolves to a package dependency
- [ ] Local path dependencies: `graph-types = { path = "../graph-types" }`
- [ ] Semver version constraints validated
- [ ] `intentc pkg init` creates a new manifest
- [ ] `intentc pkg add <name> <version>` adds a dependency
- [ ] `intentc build` auto-resolves package dependencies
- [ ] Cross-package type references work (entities, enums, traits)
- [ ] All existing tests pass (no regressions)
- [ ] Example multi-package project compiles and runs

## Reference

- Design: `docs/decisions/0027-package-management-design.md`
- Module registry: `internal/compiler/registry.go`
- Multi-file compilation: `internal/compiler/compiler.go`

## Prerequisites

- Phase 11 (Generics) beneficial for reusable packages but not required
- Phase 12 (Async) independent

## Tasks

### 13.1 Manifest Parsing

**Files:** `internal/compiler/manifest.go` (new), `cmd/intentc/main.go`

Create manifest parser for `intent.toml`:
```go
type Manifest struct {
    Package PackageInfo
    Dependencies map[string]DependencySpec
}

type PackageInfo struct {
    Name        string
    Version     string
    Description string
}

type DependencySpec struct {
    Version string  // semver constraint
    Path    string  // local path (optional, for development)
}
```

Parse TOML format (implement minimal parser -- no external dependencies per project convention):
```toml
[package]
name = "my-project"
version = "1.0.0"

[dependencies]
graph-types = "0.2.0"
utils = { path = "../utils" }
```

CLI:
- `intentc pkg init` -- creates `intent.toml` from existing `module` declarations
- Detect `intent.toml` in project root during `intentc build`

**Acceptance:**
- `go test ./internal/compiler/... -v` passes with manifest parsing tests
- `intentc pkg init` creates valid `intent.toml`
- Malformed TOML produces clear error messages

### 13.2 Semver Resolution

**Files:** `internal/compiler/semver.go` (new)

Implement semver parsing and comparison:
```go
type Version struct {
    Major, Minor, Patch int
}

type Constraint struct {
    Op      string  // "=", "^", "~", ">=", "<"
    Version Version
}

func ParseVersion(s string) (Version, error)
func ParseConstraint(s string) (Constraint, error)
func (c Constraint) Matches(v Version) bool
```

Rules:
- `"1.0.0"` -- exact match
- `"^1.0.0"` -- compatible: >=1.0.0, <2.0.0
- `"~1.0.0"` -- patch only: >=1.0.0, <1.1.0

Validate:
- All dependencies in tree satisfy their constraints
- No conflicting versions of same package
- Clear error messages for version conflicts

**Acceptance:**
- `go test ./internal/compiler/... -v` passes with semver tests
- Tests cover: parsing, comparison, constraint matching, conflict detection

### 13.3 Package Import Resolution

**Files:** `internal/compiler/registry.go`, `internal/parser/parser.go`, `internal/ast/nodes.go`

Extend import syntax:
```intent
import graph_types;              // whole package
import graph_types.validation;   // specific module from package
```

AST changes:
- `ImportDecl` gains `IsPackage bool` and `PackageName string`
- Or parse `import <ident>;` vs `import "<string>";` to distinguish

Registry changes:
- `DiscoverDependencies()` reads `intent.toml` if present
- For package imports, resolve to package directory via manifest
- Load package's `intent.toml` for transitive dependencies
- Build full dependency graph across packages
- Extend `TopologicalSort` to handle cross-package ordering

Cross-package symbol resolution:
- Package public symbols available via `package_name.symbol()` syntax
- Extend `CheckAll()` to register symbols from package dependencies
- Name mangling: package entities get package prefix (e.g., `GraphTypesNodeAttr`)

**Acceptance:**
- `go test ./internal/compiler/... -v` passes
- `go test ./internal/parser/... -v` passes with package import tests
- Multi-package example compiles successfully
- Cross-package function calls and entity construction work

### 13.4 Package Cache

**Files:** `internal/compiler/cache.go` (new)

Cache directory: `~/.intent/cache/<package-name>/<version>/`

Cache operations:
- Check if cached version exists and matches source checksum
- Store compiled IR for a package
- Load cached IR to skip re-compilation
- Invalidate on source changes (SHA256 of source files)

For local path dependencies:
- Always recompile (no caching -- development mode)

**Acceptance:**
- Second build of same package is faster (cache hit)
- Source changes invalidate cache
- `intentc pkg install` populates cache

### 13.5 CLI Commands

**Files:** `cmd/intentc/main.go`

Add `pkg` subcommand:
```bash
intentc pkg init                     # create intent.toml
intentc pkg add <name> <version>     # add dependency to manifest
intentc pkg add <name> --path <dir>  # add local path dependency
intentc pkg remove <name>            # remove dependency
intentc pkg install                  # resolve and cache all deps
intentc pkg list                     # show dependency tree
```

**Acceptance:**
- All commands work and produce clear output
- `intentc pkg init` in a project with `module` declarations creates correct manifest
- `intentc pkg add` modifies `intent.toml` correctly
- `intentc pkg list` shows tree with versions

### 13.6 Example Multi-Package Project

**Files:** `examples/packages/` (new directory)

Create a two-package example:

```
examples/packages/
  types_pkg/
    intent.toml
    types.intent     # public entity Point, public function distance
  app_pkg/
    intent.toml      # depends on types_pkg via local path
    main.intent      # imports types_pkg, uses Point and distance
```

`types_pkg/intent.toml`:
```toml
[package]
name = "types_pkg"
version = "0.1.0"
```

`app_pkg/intent.toml`:
```toml
[package]
name = "app_pkg"
version = "0.1.0"

[dependencies]
types_pkg = { path = "../types_pkg" }
```

**Acceptance:**
- `cd examples/packages/app_pkg && intentc build main.intent` succeeds
- Generated code references types from `types_pkg` correctly
- Runs and produces expected output

### 13.7 Docs + ADR

**Files:** docs, `INTENT.md`, `docs/grammar.ebnf`

- Update ADR 0027 status to "Accepted"
- Update DESIGN.md: add Package Management section
- Update INTENT.md: add Packages section with `intent.toml` and import syntax
- Update grammar.ebnf: add package import production
- Run `gofmt -w` on all changed Go files

**Acceptance:**
- All docs consistent with implementation
- `go test ./... -timeout 30s` passes
- `make clean` leaves no artifacts
