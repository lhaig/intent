package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/parser"
)

// ModuleRegistry manages parsed modules and their dependency graph.
// It discovers all transitive imports from an entry file using BFS,
// then provides topological ordering with cycle detection.
type ModuleRegistry struct {
	modules      map[string]*ast.Program // absolute file path -> parsed AST
	dependencies map[string][]string     // absolute file path -> imported absolute file paths
	entryPath    string                  // absolute path to the entry point file
	projectRoot  string                  // directory containing the entry file
}

// NewModuleRegistry creates a new registry rooted at the given entry file.
// The entryPath is resolved to an absolute path, and projectRoot is set to
// the directory containing the entry file.
func NewModuleRegistry(entryPath string) (*ModuleRegistry, error) {
	absPath, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve entry path: %w", err)
	}

	return &ModuleRegistry{
		modules:      make(map[string]*ast.Program),
		dependencies: make(map[string][]string),
		entryPath:    absPath,
		projectRoot:  filepath.Dir(absPath),
	}, nil
}

// DiscoverDependencies performs BFS from the entry file, parsing each
// discovered .intent file and collecting its imports. Returns diagnostics
// for any parse errors (with file paths set) and an error for fatal issues
// like missing files.
func (r *ModuleRegistry) DiscoverDependencies() (*diagnostic.Diagnostics, error) {
	diag := diagnostic.New()
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
			resolved := resolveImportPath(imp.Path, r.projectRoot)

			// Validate .intent extension
			if !strings.HasSuffix(resolved, ".intent") {
				diag.ErrorfInFile(filePath, imp.Line, imp.Column,
					"import path must have .intent extension: %s", imp.Path)
				continue
			}

			// Validate file exists
			if _, err := os.Stat(resolved); os.IsNotExist(err) {
				return diag, fmt.Errorf("imported file not found: %s (resolved from %q in %s)",
					resolved, imp.Path, filePath)
			}

			deps = append(deps, resolved)

			if !visited[resolved] {
				queue = append(queue, resolved)
			}
		}
		r.dependencies[filePath] = deps
	}

	return diag, nil
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
