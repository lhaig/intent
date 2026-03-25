package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/parser"
)

// ModuleRegistry manages parsed modules and their dependency graph.
// It discovers all transitive imports from an entry file using BFS,
// then provides topological ordering with cycle detection.
type ModuleRegistry struct {
	modules           map[string]*ast.Program // absolute file path -> parsed AST
	dependencies      map[string][]string     // absolute file path -> imported absolute file paths
	entryPath         string                  // absolute path to the entry point file
	projectRoot       string                  // directory containing the entry file
	workspaceRoot     string                  // root for path traversal checks (may be parent of projectRoot)
	manifest          *Manifest               // optional manifest from intent.toml
	packageDirs       map[string]string       // package name -> resolved directory path
	hasPackageImports bool                    // true if any source file uses a package import
}

// NewModuleRegistry creates a new registry rooted at the given entry file.
// The entryPath is resolved to an absolute path, and projectRoot is set to
// the directory containing the entry file.
func NewModuleRegistry(entryPath string) (*ModuleRegistry, error) {
	absPath, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve entry path: %w", err)
	}

	projRoot := filepath.Dir(absPath)
	return &ModuleRegistry{
		modules:       make(map[string]*ast.Program),
		dependencies:  make(map[string][]string),
		entryPath:     absPath,
		projectRoot:   projRoot,
		workspaceRoot: projRoot,
	}, nil
}

// DiscoverDependencies performs BFS from the entry file, parsing each
// discovered .intent file and collecting its imports. Returns diagnostics
// for any parse errors (with file paths set) and an error for fatal issues
// like missing files.
//
// If an intent.toml manifest is found in the project root, package imports
// (import pkg_name;) are resolved via the manifest's dependency declarations.
func (r *ModuleRegistry) DiscoverDependencies() (*diagnostic.Diagnostics, error) {
	diag := diagnostic.New()

	// Resolve all package directories transitively via BFS.
	// Starting from the project root's intent.toml, we follow each package's
	// own intent.toml to discover transitive dependencies.
	r.packageDirs = make(map[string]string)
	manifestPath := filepath.Join(r.projectRoot, "intent.toml")
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		if !os.IsNotExist(statErr) {
			return diag, fmt.Errorf("cannot stat %s: %w", manifestPath, statErr)
		}
		// No manifest file — skip manifest loading
	} else {
		manifest, err := LoadManifest(r.projectRoot)
		if err != nil {
			return diag, fmt.Errorf("failed to parse %s: %w", manifestPath, err)
		}
		r.manifest = manifest
		// If any path dependency resolves outside projectRoot, widen workspaceRoot
		// to the common ancestor of projectRoot and all dependency paths.
		for _, dep := range manifest.Dependencies {
			if dep.Path != "" {
				resolved := filepath.Clean(filepath.Join(r.projectRoot, dep.Path))
				rel, relErr := filepath.Rel(r.projectRoot, resolved)
				if relErr != nil || strings.HasPrefix(rel, "..") {
					ancestor := commonAncestor(r.workspaceRoot, resolved)
					r.workspaceRoot = ancestor
				}
			}
		}
		if err := r.resolveTransitivePackages(r.projectRoot, manifest); err != nil {
			return diag, err
		}
	}

	queue := []string{r.entryPath}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		filePath := queue[0]
		queue = queue[1:]

		if visited[filePath] {
			continue
		}
		visited[filePath] = true

		// Read source from disk
		source, err := os.ReadFile(filePath)
		if err != nil {
			return diag, fmt.Errorf("imported file not found: %s", filePath)
		}

		// Parse
		p := parser.New(string(source))
		prog := p.Parse()

		// Collect parse errors with file context
		if p.Diagnostics().HasErrors() {
			for _, d := range p.Diagnostics().Errors() {
				diag.ErrorfInFile(filePath, d.Line, d.Column, "%s", d.Message)
			}
		}

		r.modules[filePath] = prog

		// Extract and resolve import paths
		var deps []string
		for _, imp := range prog.Imports {
			if imp.IsPackage {
				r.hasPackageImports = true
				// Package import: resolve via manifest
				resolved, err := r.resolvePackageImport(imp)
				if err != nil {
					diag.ErrorfInFile(filePath, imp.Line, imp.Column, "%s", err.Error())
					continue
				}
				deps = append(deps, resolved...)
				for _, res := range resolved {
					if !visited[res] {
						queue = append(queue, res)
					}
				}
			} else {
				// Module import: resolve relative path
				resolved := resolveImportPath(imp.Path, r.projectRoot)

				// Validate .intent extension
				if !strings.HasSuffix(resolved, ".intent") {
					diag.ErrorfInFile(filePath, imp.Line, imp.Column,
						"import path must have .intent extension: %s", imp.Path)
					continue
				}

				// Validate file exists
				if _, err := os.Stat(resolved); err != nil {
					if os.IsNotExist(err) {
						return diag, fmt.Errorf("imported file not found: %s (resolved from %q in %s)",
							resolved, imp.Path, filePath)
					}
					return diag, fmt.Errorf("cannot stat imported file %s: %w", resolved, err)
				}

				deps = append(deps, resolved)

				if !visited[resolved] {
					queue = append(queue, resolved)
				}
			}
		}
		r.dependencies[filePath] = deps
	}

	return diag, nil
}

// resolveTransitivePackages performs DFS over package manifests to discover
// all transitive package dependencies. It starts from the root manifest and
// follows each package's own intent.toml. Detects circular dependencies using
// a recursion stack (inProgress set) so that cycles through diamond-shaped
// graphs are correctly reported.
func (r *ModuleRegistry) resolveTransitivePackages(rootDir string, rootManifest *Manifest) error {
	visited := make(map[string]bool)    // package directory -> fully processed
	inProgress := make(map[string]bool) // package directory -> currently on DFS stack

	// Recursive DFS helper. chain tracks the package name path for error reporting.
	var visit func(name, dir string, chain []string) error
	visit = func(name, dir string, chain []string) error {
		// Resolve symlinks so cycle detection works with symlinked package dirs
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		if inProgress[dir] {
			// Build cycle path from the point where the cycle starts
			cycleStart := -1
			for i, c := range chain {
				if c == name {
					cycleStart = i
					break
				}
			}
			var cyclePath []string
			if cycleStart >= 0 {
				cyclePath = make([]string, len(chain[cycleStart:])+1)
				copy(cyclePath, chain[cycleStart:])
				cyclePath[len(cyclePath)-1] = name
			} else {
				cyclePath = make([]string, len(chain)+1)
				copy(cyclePath, chain)
				cyclePath[len(cyclePath)-1] = name
			}
			return fmt.Errorf("circular package dependency detected: %s", strings.Join(cyclePath, " -> "))
		}
		if visited[dir] {
			return nil
		}

		inProgress[dir] = true
		defer delete(inProgress, dir)
		visited[dir] = true
		r.packageDirs[name] = dir

		// Check if this package has its own intent.toml with dependencies
		subManifestPath := filepath.Join(dir, "intent.toml")
		if _, statErr := os.Stat(subManifestPath); statErr != nil {
			if os.IsNotExist(statErr) {
				return nil // no manifest in this package, nothing to recurse into
			}
			return fmt.Errorf("cannot stat %s: %w", subManifestPath, statErr)
		}
		subManifest, err := LoadManifest(dir)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", subManifestPath, err)
		}

		subDepNames := make([]string, 0, len(subManifest.Dependencies))
		for depName := range subManifest.Dependencies {
			subDepNames = append(subDepNames, depName)
		}
		sort.Strings(subDepNames)
		for _, depName := range subDepNames {
			dep := subManifest.Dependencies[depName]
			depDir, err := r.resolvePkgDir(dir, depName, dep)
			if err != nil {
				return err
			}
			if depDir == "" {
				continue
			}

			newChain := make([]string, len(chain)+1)
			copy(newChain, chain)
			newChain[len(chain)] = depName
			if err := visit(depName, depDir, newChain); err != nil {
				return err
			}
		}

		return nil
	}

	// Visit root dependencies in sorted order for determinism
	rootDepNames := make([]string, 0, len(rootManifest.Dependencies))
	for name := range rootManifest.Dependencies {
		rootDepNames = append(rootDepNames, name)
	}
	sort.Strings(rootDepNames)
	for _, name := range rootDepNames {
		dep := rootManifest.Dependencies[name]
		dir, err := r.resolvePkgDir(rootDir, name, dep)
		if err != nil {
			return err
		}
		if dir != "" {
			if err := visit(name, dir, []string{name}); err != nil {
				return err
			}
		}
	}

	return nil
}

// commonAncestor returns the deepest directory that is an ancestor of both
// absolute paths a and b, computed at directory boundaries.
func commonAncestor(a, b string) string {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	aParts := strings.Split(a, string(filepath.Separator))
	bParts := strings.Split(b, string(filepath.Separator))
	n := len(aParts)
	if len(bParts) < n {
		n = len(bParts)
	}
	common := 0
	for i := 0; i < n; i++ {
		if aParts[i] != bParts[i] {
			break
		}
		common = i + 1
	}
	if common == 0 {
		return string(filepath.Separator)
	}
	result := strings.Join(aParts[:common], string(filepath.Separator))
	// On Unix, ensure we have a leading separator for absolute paths.
	if !filepath.IsAbs(result) && filepath.IsAbs(a) {
		result = string(filepath.Separator) + result
	}
	return result
}

// resolvePkgDir resolves a dependency spec to an absolute directory path.
func (r *ModuleRegistry) resolvePkgDir(baseDir, name string, dep DependencySpec) (string, error) {
	if dep.Path != "" {
		resolved := filepath.Clean(filepath.Join(baseDir, dep.Path))
		// Ensure the resolved path doesn't escape the workspace root directory.
		// Resolve symlinks on both paths to handle OS-level symlinks (e.g. /var -> /private/var on macOS).
		wsRoot := r.workspaceRoot
		if real, err := filepath.EvalSymlinks(wsRoot); err == nil {
			wsRoot = real
		}
		if real, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = real
		}
		rel, err := filepath.Rel(wsRoot, resolved)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("dependency path %q escapes project directory", dep.Path)
		}
		return resolved, nil
	}
	if dep.Version != "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory for package cache: %w", err)
		}
		resolved, err := ConstraintBaseVersion(dep.Version)
		if err != nil {
			return "", fmt.Errorf("invalid version constraint %q for package %q: %w", dep.Version, name, err)
		}
		return filepath.Join(homeDir, ".intent", "cache", name, resolved), nil
	}
	return "", nil
}

// resolvePackageImport resolves a package import to a list of .intent files.
// For "pkg_name", it finds all .intent files in the package directory.
// For "pkg_name.submodule", it finds the specific submodule file.
func (r *ModuleRegistry) resolvePackageImport(imp *ast.ImportDecl) ([]string, error) {
	parts := strings.SplitN(imp.PackageName, ".", 2)
	pkgName := parts[0]

	pkgDir, ok := r.packageDirs[pkgName]
	if !ok {
		return nil, fmt.Errorf("unknown package %q: not declared in intent.toml dependencies", pkgName)
	}

	// Check package directory exists
	if _, err := os.Stat(pkgDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package directory not found for %q: %s", pkgName, pkgDir)
		}
		return nil, fmt.Errorf("cannot stat package directory for %q: %w", pkgName, err)
	}

	if len(parts) == 2 {
		// Specific submodule: pkg_name.submodule -> pkg_dir/submodule.intent
		subFile := filepath.Join(pkgDir, parts[1]+".intent")
		if _, err := os.Stat(subFile); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("submodule %q not found in package %q: %s", parts[1], pkgName, subFile)
			}
			return nil, fmt.Errorf("cannot stat submodule %q in package %q: %w", parts[1], pkgName, err)
		}
		return []string{subFile}, nil
	}

	// Whole package: find all .intent files in the package directory
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read package directory %q: %w", pkgName, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".intent") {
			files = append(files, filepath.Join(pkgDir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .intent files found in package %q: %s", pkgName, pkgDir)
	}

	return files, nil
}

// PackageDirs returns the resolved package name -> directory mapping.
func (r *ModuleRegistry) PackageDirs() map[string]string {
	return r.packageDirs
}

// HasCrossPackageImports returns true if any discovered module uses package imports.
func (r *ModuleRegistry) HasCrossPackageImports() bool {
	return r.hasPackageImports
}

// HasCrossPackageDeps is a convenience function that creates a registry,
// discovers dependencies, and reports whether any cross-package imports exist.
func HasCrossPackageDeps(entryPath string) (bool, error) {
	reg, err := NewModuleRegistry(entryPath)
	if err != nil {
		return false, err
	}
	diag, err := reg.DiscoverDependencies()
	if err != nil {
		return false, err
	}
	if diag.HasErrors() {
		return false, fmt.Errorf("dependency discovery failed:\n%s", diag.Format(entryPath))
	}
	return reg.HasCrossPackageImports(), nil
}

// TopologicalSort returns files in dependency order (dependencies first,
// entry file last). Returns an error if an import cycle is detected,
// with a clear message showing the cycle path.
func (r *ModuleRegistry) TopologicalSort() ([]string, error) {
	var sorted []string
	visiting := make(map[string]bool) // recursion stack (currently being visited)
	visited := make(map[string]bool)  // completed nodes

	var visit func(path string, stack []string) error
	visit = func(path string, stack []string) error {
		if visiting[path] {
			// Found cycle: build cycle path from the stack
			cycleStart := -1
			for i, p := range stack {
				if p == path {
					cycleStart = i
					break
				}
			}
			cyclePath := append(stack[cycleStart:], path)
			// Use base names for readability
			var names []string
			for _, p := range cyclePath {
				names = append(names, filepath.Base(p))
			}
			return fmt.Errorf("import cycle detected: %s", strings.Join(names, " -> "))
		}
		if visited[path] {
			return nil
		}

		visiting[path] = true
		stack = append(stack, path)

		for _, dep := range r.dependencies[path] {
			if err := visit(dep, stack); err != nil {
				return err
			}
		}

		visiting[path] = false
		visited[path] = true
		sorted = append(sorted, path)
		return nil
	}

	// Start from the entry path to ensure deterministic ordering
	if err := visit(r.entryPath, nil); err != nil {
		return nil, err
	}

	// Visit any remaining modules not reachable from entry (shouldn't happen
	// in normal operation since DiscoverDependencies starts from entry, but
	// handles edge cases)
	for path := range r.modules {
		if !visited[path] {
			if err := visit(path, nil); err != nil {
				return nil, err
			}
		}
	}

	return sorted, nil
}

// GetModule returns the parsed AST for a given absolute file path,
// or nil if the path has not been discovered.
func (r *ModuleRegistry) GetModule(path string) *ast.Program {
	return r.modules[path]
}

// AllModules returns all parsed modules keyed by their absolute file paths.
func (r *ModuleRegistry) AllModules() map[string]*ast.Program {
	return r.modules
}

// resolveImportPath resolves an import path relative to the project root.
// For example, "math.intent" resolves to "/project/root/math.intent".
func resolveImportPath(importPath, projectRoot string) string {
	return filepath.Clean(filepath.Join(projectRoot, importPath))
}
