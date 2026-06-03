package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeTaggedRepo extends makeLocalGitRepo (fetcher_test.go) with custom
// file/body lists, used by the integration tests below to mount an
// intent.toml plus arbitrary source files in the fixture repo.
func makeTaggedRepo(t *testing.T, tag string, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not on PATH; skipping integration test")
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	gitCmd("init", "--quiet", "--initial-branch=main")
	for rel, body := range files {
		full := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	gitCmd("add", "-A")
	gitCmd("commit", "--quiet", "-m", "init "+tag)
	gitCmd("tag", tag)

	bare := filepath.Join(dir, "bare.git")
	if out, err := exec.Command("git", "clone", "--quiet", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %v\n%s", err, out)
	}
	return bare
}

// TestGitFsLoaderResolvesEndToEnd exercises the full git-source path:
// real GitFetcher + real PackageCache + real ManifestLoader pulling a
// dep from a local bare git repo into a temp cache dir.
func TestGitFsLoaderResolvesEndToEnd(t *testing.T) {
	libRepo := makeTaggedRepo(t, "v1.0.0", map[string]string{
		"intent.toml": "[package]\nname = \"lib\"\nversion = \"1.0.0\"\n",
		"lib.intent":  "module lib version \"1.0.0\";\npublic function id(n: Int) returns Int { return n; }\n",
	})

	cache := &PackageCache{CacheDir: filepath.Join(t.TempDir(), "cache")}
	loader := &GitFsLoader{
		Fetcher: GitFetcher{},
		Cache:   cache,
		Root:    t.TempDir(),
	}

	src := DependencySpec{Git: libRepo, Version: "1.0.0"}
	min, _ := ParseVersion("1.0.0")
	m, ver, rev, err := loader.LoadAt("lib", src, min)
	if err != nil {
		t.Fatalf("LoadAt: %v", err)
	}
	if m.Package.Name != "lib" {
		t.Errorf("manifest name: got %q, want %q", m.Package.Name, "lib")
	}
	if ver.String() != "1.0.0" {
		t.Errorf("resolved version: got %s, want 1.0.0", ver.String())
	}
	if rev == "" {
		t.Error("rev should be populated for git source")
	}
}

// TestPkgInstallEndToEnd is the full registry integration test: real
// Resolver + GitFsLoader + lockfile against a local bare git fixture.
// Mirrors what `intentc pkg install` does inside the CLI handler.
func TestPkgInstallEndToEnd(t *testing.T) {
	libRepo := makeTaggedRepo(t, "v1.0.0", map[string]string{
		"intent.toml": "[package]\nname = \"lib\"\nversion = \"1.0.0\"\n",
		"lib.intent":  "module lib version \"1.0.0\";\n",
	})

	projDir := t.TempDir()
	manifestBody := "[package]\nname = \"app\"\nversion = \"0.1.0\"\n\n[dependencies]\n" +
		"lib = { git = \"" + libRepo + "\", version = \"1.0.0\" }\n"
	if err := os.WriteFile(filepath.Join(projDir, "intent.toml"), []byte(manifestBody), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m, err := LoadManifest(projDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	cache := &PackageCache{CacheDir: filepath.Join(t.TempDir(), "cache")}
	loader := &GitFsLoader{Fetcher: GitFetcher{}, Cache: cache, Root: projDir}
	rs, err := (&Resolver{Loader: loader}).Resolve(m, projDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(rs.Packages) != 1 {
		t.Fatalf("expected 1 resolved pkg, got %d", len(rs.Packages))
	}

	checksumOf := func(p LockedPackage) (string, error) {
		for _, rp := range rs.Packages {
			if rp.Name == p.Name && rp.Source.IsGit() {
				host, owner, repo, err := ParseGitURL(rp.Source.Git)
				if err != nil {
					return "", err
				}
				return cache.GitTreeChecksum(host, owner, repo, rp.Rev)
			}
		}
		return "", nil
	}
	lock, err := FromResolvedSet(rs, checksumOf, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("FromResolvedSet: %v", err)
	}
	lockPath := filepath.Join(projDir, "intent.lock")
	if err := WriteLockfile(lockPath, lock); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	// Sanity-check the lockfile we wrote.
	roundTripped, err := ReadLockfile(lockPath)
	if err != nil {
		t.Fatalf("ReadLockfile: %v", err)
	}
	if len(roundTripped.Packages) != 1 {
		t.Fatalf("lockfile pkg count: got %d, want 1", len(roundTripped.Packages))
	}
	libEntry := roundTripped.Packages[0]
	if libEntry.Name != "lib" {
		t.Errorf("lockfile name: got %q, want lib", libEntry.Name)
	}
	if libEntry.Version.String() != "1.0.0" {
		t.Errorf("lockfile version: got %s, want 1.0.0", libEntry.Version.String())
	}
	if !strings.HasPrefix(libEntry.Checksum, "sha256:") {
		t.Errorf("expected sha256 checksum, got %q", libEntry.Checksum)
	}
	if libEntry.Rev == "" {
		t.Error("lockfile rev should be populated")
	}

	// Re-verify the lockfile checksums against the cache.
	if err := roundTripped.Verify(checksumOf); err != nil {
		t.Errorf("Verify on freshly-written lockfile: %v", err)
	}
}
