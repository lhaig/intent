package lsp

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/compiler"
)

// workspace represents the project context of an open document. v1
// recognises two modes:
//
//   - single-file: the document has no sibling files we need to consult.
//     Hover/goto-def see only the document's own AST.
//   - multi-file: an intent.toml lives in the document's directory (matching
//     compiler.IsMultiFile's heuristic). Sibling files are parsed via the
//     ModuleRegistry and exposed as a map for cross-file symbol resolution.
//
// v1 explicitly does not handle:
//   - intent.toml in an ancestor directory (only same-dir)
//   - cross-package go-to-definition (ADR 0032 §O4 → B; deferred to v1.1)
//   - didChangeWatchedFiles invalidation (registry caches until restart)
type workspace struct {
	rootDir   string // directory containing intent.toml, or empty for single-file
	multiFile bool

	mu      sync.Mutex
	modules map[string]*ast.Program // path → parsed AST (sibling files only; the open doc uses its own cached parse)
}

// workspaceManager keeps one workspace per project root. The workspace is
// built lazily on first lookup.
type workspaceManager struct {
	mu         sync.Mutex
	workspaces map[string]*workspace // keyed by rootDir (or empty key for single-file ad-hoc)
}

func newWorkspaceManager() *workspaceManager {
	return &workspaceManager{workspaces: map[string]*workspace{}}
}

// workspaceForURI resolves the workspace that should serve cross-file lookups
// for the given document. Returns a single-file workspace if no intent.toml
// is present.
func (m *workspaceManager) workspaceForURI(uri DocumentURI) *workspace {
	path := uriToPath(uri)
	if path == "" {
		return &workspace{multiFile: false}
	}
	dir := filepath.Dir(path)
	if !manifestPresent(dir) {
		return &workspace{rootDir: dir, multiFile: false}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if ws, ok := m.workspaces[dir]; ok {
		return ws
	}
	ws := &workspace{rootDir: dir, multiFile: true, modules: nil}
	m.workspaces[dir] = ws
	return ws
}

// invalidate drops the cached parse for sibling files. v1 calls this on no
// event in particular (file watcher reactivity is deferred); the method is
// here so future tasks can hook it.
func (m *workspaceManager) invalidate(rootDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ws, ok := m.workspaces[rootDir]; ok {
		ws.mu.Lock()
		ws.modules = nil
		ws.mu.Unlock()
	}
}

func manifestPresent(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "intent.toml"))
	return err == nil
}

// siblingModules returns the parsed ASTs of every file in this workspace's
// project EXCEPT the file matching excludePath. Used by goto-def / hover to
// search outside the currently-open document. Returns nil for single-file
// workspaces or when registry discovery fails.
//
// excludePath is the absolute path of the open document; the caller is
// responsible for its own AST and shouldn't see stale duplicates.
func (ws *workspace) siblingModules(excludePath string) map[string]*ast.Program {
	if !ws.multiFile {
		return nil
	}
	ws.mu.Lock()
	cached := ws.modules
	ws.mu.Unlock()
	if cached != nil {
		out := make(map[string]*ast.Program, len(cached))
		for p, m := range cached {
			if p != excludePath {
				out[p] = m
			}
		}
		return out
	}

	// Discover and parse on first lookup. Use any .intent file in the root
	// directory as the entry — ModuleRegistry walks imports from there.
	entry, ok := findEntry(ws.rootDir, excludePath)
	if !ok {
		return nil
	}
	reg, err := compiler.NewModuleRegistry(entry)
	if err != nil {
		return nil
	}
	if _, derr := reg.DiscoverDependencies(); derr != nil {
		return nil
	}
	all := reg.AllModules()

	ws.mu.Lock()
	ws.modules = all
	ws.mu.Unlock()

	out := make(map[string]*ast.Program, len(all))
	for p, m := range all {
		if p != excludePath {
			out[p] = m
		}
	}
	return out
}

// findEntry picks an .intent file in dir to use as the ModuleRegistry entry.
// Prefers the open document itself if it's in dir; otherwise picks any
// .intent file. Returns ("", false) if no .intent file exists.
func findEntry(dir, preferred string) (string, bool) {
	if preferred != "" && filepath.Dir(preferred) == dir {
		return preferred, true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".intent" {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}
