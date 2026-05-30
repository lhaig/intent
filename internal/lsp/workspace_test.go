package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhaig/intent/internal/ast"
)

// Phase 18 task 18.7: workspace + multi-file handling.

func TestWorkspaceSingleFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stand_alone.intent")
	if err := os.WriteFile(path, []byte("module x version \"1.0\";\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wm := newWorkspaceManager()
	ws := wm.workspaceForURI(pathToURI(path))
	if ws.multiFile {
		t.Errorf("expected single-file workspace, got multiFile=true")
	}
	if got := ws.siblingModules(path); got != nil {
		t.Errorf("single-file siblingModules should be nil, got %d entries", len(got))
	}
}

func TestWorkspaceMultiFileMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte(`
[package]
name = "demo"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.intent")
	libPath := filepath.Join(dir, "lib.intent")
	if err := os.WriteFile(mainPath, []byte("module main version \"1.0\";\nimport \"lib.intent\";\nentry function main() returns Int { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, []byte("module lib version \"1.0\";\nfunction helper() returns Int { return 42; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wm := newWorkspaceManager()
	ws := wm.workspaceForURI(pathToURI(mainPath))
	if !ws.multiFile {
		t.Fatalf("expected multiFile workspace with intent.toml present, got multiFile=false")
	}
	siblings := ws.siblingModules(mainPath)
	if _, ok := siblings[libPath]; !ok {
		t.Errorf("expected lib.intent in siblings, got keys: %v", keysOf(siblings))
	}
	if _, ok := siblings[mainPath]; ok {
		t.Errorf("siblings should exclude the open document; got: %v", keysOf(siblings))
	}
}

func TestWorkspaceCacheSharedAcrossDocs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte("[package]\nname=\"d\"\nversion=\"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path1 := filepath.Join(dir, "a.intent")
	path2 := filepath.Join(dir, "b.intent")
	if err := os.WriteFile(path1, []byte("module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte("module b version \"1.0\";\nfunction foo() returns Int { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wm := newWorkspaceManager()
	ws1 := wm.workspaceForURI(pathToURI(path1))
	ws2 := wm.workspaceForURI(pathToURI(path2))
	if ws1 != ws2 {
		t.Error("two documents in the same project should share a workspace instance")
	}
}

func TestManifestPresentLookup(t *testing.T) {
	dir := t.TempDir()
	if manifestPresent(dir) {
		t.Error("empty dir should not have manifest")
	}
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if !manifestPresent(dir) {
		t.Error("expected manifest detection after writing intent.toml")
	}
}

func keysOf(m map[string]*ast.Program) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
