package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempCache(t *testing.T) *PackageCache {
	t.Helper()
	dir := t.TempDir()
	return &PackageCache{CacheDir: dir}
}

func TestCacheNewPackageCache(t *testing.T) {
	c, err := NewPackageCache()
	if err != nil {
		t.Fatalf("NewPackageCache() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".intent", "cache")
	if c.CacheDir != expected {
		t.Errorf("CacheDir = %q, want %q", c.CacheDir, expected)
	}
}

func TestCachePath(t *testing.T) {
	c := &PackageCache{CacheDir: "/tmp/test-cache"}
	got, err := c.CachePath("mypkg", "1.0.0")
	if err != nil {
		t.Fatalf("CachePath() error: %v", err)
	}
	want := "/tmp/test-cache/mypkg/1.0.0"
	if got != want {
		t.Errorf("CachePath() = %q, want %q", got, want)
	}
}

func TestCacheHasEmpty(t *testing.T) {
	c := tempCache(t)
	if c.Has("nonexistent", "0.0.0") {
		t.Error("Has() returned true for missing package")
	}
}

func TestCacheStoreAndLoad(t *testing.T) {
	c := tempCache(t)

	files := []CachedFile{
		{Name: "output.ir", Content: []byte("some IR data")},
		{Name: "meta.json", Content: []byte(`{"compiled":true}`)},
	}

	if err := c.Store("mypkg", "1.0.0", files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	if !c.Has("mypkg", "1.0.0") {
		t.Error("Has() returned false after Store()")
	}

	loaded, err := c.Load("mypkg", "1.0.0")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Load() returned %d files, want 2", len(loaded))
	}

	// Build a map for order-independent comparison.
	m := make(map[string]string)
	for _, f := range loaded {
		m[f.Name] = string(f.Content)
	}

	if m["output.ir"] != "some IR data" {
		t.Errorf("output.ir content = %q, want %q", m["output.ir"], "some IR data")
	}
	if m["meta.json"] != `{"compiled":true}` {
		t.Errorf("meta.json content = %q", m["meta.json"])
	}
}

func TestCacheLoadMissing(t *testing.T) {
	c := tempCache(t)
	_, err := c.Load("nope", "0.0.0")
	if err == nil {
		t.Error("Load() on missing entry should return error")
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := tempCache(t)

	files := []CachedFile{
		{Name: "data.bin", Content: []byte("hello")},
	}
	if err := c.Store("pkg", "2.0.0", files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	if !c.Has("pkg", "2.0.0") {
		t.Fatal("Has() returned false after Store()")
	}

	if err := c.Invalidate("pkg", "2.0.0"); err != nil {
		t.Fatalf("Invalidate() error: %v", err)
	}

	if c.Has("pkg", "2.0.0") {
		t.Error("Has() returned true after Invalidate()")
	}
}

func TestCacheInvalidateMissing(t *testing.T) {
	c := tempCache(t)
	// Should not error when entry doesn't exist.
	if err := c.Invalidate("nope", "0.0.0"); err != nil {
		t.Errorf("Invalidate() on missing entry returned error: %v", err)
	}
}

func TestCacheChecksumMatch(t *testing.T) {
	c := tempCache(t)

	// Create some source files.
	srcDir := t.TempDir()
	src1 := filepath.Join(srcDir, "a.intent")
	src2 := filepath.Join(srcDir, "b.intent")
	os.WriteFile(src1, []byte("fn main() {}"), 0644)
	os.WriteFile(src2, []byte("fn helper() {}"), 0644)

	sourceFiles := []string{src1, src2}

	cached := []CachedFile{
		{Name: "output.ir", Content: []byte("ir data")},
	}

	// Store with checksum.
	if err := c.StoreWithChecksum("mypkg", "1.0.0", cached, sourceFiles, srcDir); err != nil {
		t.Fatalf("StoreWithChecksum() error: %v", err)
	}

	// Checksum should match with same sources.
	match, err := c.ChecksumMatch("mypkg", "1.0.0", sourceFiles, srcDir)
	if err != nil {
		t.Fatalf("ChecksumMatch() error: %v", err)
	}
	if !match {
		t.Error("ChecksumMatch() returned false, want true")
	}

	// Modify a source file — checksum should no longer match.
	os.WriteFile(src1, []byte("fn main() { changed }"), 0644)

	match, err = c.ChecksumMatch("mypkg", "1.0.0", sourceFiles, srcDir)
	if err != nil {
		t.Fatalf("ChecksumMatch() error after modification: %v", err)
	}
	if match {
		t.Error("ChecksumMatch() returned true after source change, want false")
	}
}

func TestCacheChecksumMatchMissing(t *testing.T) {
	c := tempCache(t)
	match, err := c.ChecksumMatch("nope", "0.0.0", []string{}, ".")
	if err != nil {
		t.Fatalf("ChecksumMatch() error: %v", err)
	}
	if match {
		t.Error("ChecksumMatch() returned true for missing entry")
	}
}

func TestCacheDirectoryCreatedOnFirstUse(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "new", "nested", "cache")
	c := &PackageCache{CacheDir: cacheDir}

	files := []CachedFile{
		{Name: "test.bin", Content: []byte("data")},
	}
	if err := c.Store("pkg", "1.0.0", files); err != nil {
		t.Fatalf("Store() error creating nested dirs: %v", err)
	}

	if !c.Has("pkg", "1.0.0") {
		t.Error("Has() returned false after Store() with nested dir creation")
	}
}

func TestCacheStoreSubdirectories(t *testing.T) {
	c := tempCache(t)

	files := []CachedFile{
		{Name: "sub/nested/file.ir", Content: []byte("nested content")},
	}
	if err := c.Store("pkg", "1.0.0", files); err != nil {
		t.Fatalf("Store() with subdirectories error: %v", err)
	}

	loaded, err := c.Load("pkg", "1.0.0")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Load() returned %d files, want 1", len(loaded))
	}

	// filepath.Walk uses OS separators; normalize for comparison.
	if loaded[0].Name != filepath.Join("sub", "nested", "file.ir") {
		t.Errorf("loaded file name = %q, want %q", loaded[0].Name, filepath.Join("sub", "nested", "file.ir"))
	}
	if string(loaded[0].Content) != "nested content" {
		t.Errorf("loaded content = %q", string(loaded[0].Content))
	}
}

// --- Path traversal rejection tests ---

func TestCachePathTraversalRejection(t *testing.T) {
	c := tempCache(t)

	tests := []struct {
		name    string
		pkg     string
		version string
	}{
		{"dotdot in name", "../../../etc", "1.0.0"},
		{"dotdot in version", "mypkg", "../../../etc"},
		{"slash in name", "my/pkg", "1.0.0"},
		{"backslash in name", "my\\pkg", "1.0.0"},
		{"space in name", "my pkg", "1.0.0"},
		{"empty name", "", "1.0.0"},
		{"empty version", "mypkg", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.CachePath(tt.pkg, tt.version)
			if err == nil {
				t.Errorf("CachePath(%q, %q) should return error", tt.pkg, tt.version)
			}

			// Has should return false for invalid names.
			if c.Has(tt.pkg, tt.version) {
				t.Errorf("Has(%q, %q) should return false", tt.pkg, tt.version)
			}

			// Store should return error for invalid names.
			err = c.Store(tt.pkg, tt.version, []CachedFile{{Name: "x", Content: []byte("x")}})
			if err == nil {
				t.Errorf("Store(%q, %q) should return error", tt.pkg, tt.version)
			}

			// Load should return error for invalid names.
			_, err = c.Load(tt.pkg, tt.version)
			if err == nil {
				t.Errorf("Load(%q, %q) should return error", tt.pkg, tt.version)
			}

			// Invalidate should return error for invalid names.
			err = c.Invalidate(tt.pkg, tt.version)
			if err == nil {
				t.Errorf("Invalidate(%q, %q) should return error", tt.pkg, tt.version)
			}
		})
	}
}

func TestValidateCacheNameAcceptsValid(t *testing.T) {
	valid := []string{"mypkg", "my-pkg", "my_pkg", "my.pkg", "v1.0.0", "pkg123"}
	for _, name := range valid {
		if err := validateCacheName(name); err != nil {
			t.Errorf("validateCacheName(%q) = %v, want nil", name, err)
		}
	}
}

func TestCacheStoreRejectsFileNameTraversal(t *testing.T) {
	c := tempCache(t)

	tests := []struct {
		name     string
		fileName string
	}{
		{"dotdot escape", "../../etc/malicious"},
		{"dotdot in subdir", "sub/../../etc/malicious"},
		{"dot resolves to dir", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := []CachedFile{{Name: tt.fileName, Content: []byte("bad")}}
			err := c.Store("pkg", "1.0.0", files)
			if err == nil {
				t.Errorf("Store() with file name %q should return error", tt.fileName)
			}
		})
	}
}

// --- Constraint version cache consistency tests ---

func TestCacheConstraintVersionConsistency(t *testing.T) {
	c := tempCache(t)

	files := []CachedFile{
		{Name: "lib.ir", Content: []byte("ir data")},
	}

	// Store using the resolved version from a constraint.
	version, err := ConstraintBaseVersion("^1.0.0")
	if err != nil {
		t.Fatalf("ConstraintBaseVersion() error: %v", err)
	}

	if err := c.Store("mypkg", version, files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Lookup using the same resolved version should hit.
	lookupVersion, err := ConstraintBaseVersion("^1.0.0")
	if err != nil {
		t.Fatalf("ConstraintBaseVersion() error: %v", err)
	}

	if !c.Has("mypkg", lookupVersion) {
		t.Error("Has() returned false for stored constraint version")
	}

	// Different constraint operators on the same base version should also hit.
	for _, constraint := range []string{"=1.0.0", "~1.0.0", ">=1.0.0", "1.0.0"} {
		v, err := ConstraintBaseVersion(constraint)
		if err != nil {
			t.Fatalf("ConstraintBaseVersion(%q) error: %v", constraint, err)
		}
		if !c.Has("mypkg", v) {
			t.Errorf("Has() returned false for constraint %q (resolved to %q)", constraint, v)
		}
	}

	// A different base version should miss.
	v, err := ConstraintBaseVersion("^2.0.0")
	if err != nil {
		t.Fatalf("ConstraintBaseVersion() error: %v", err)
	}
	if c.Has("mypkg", v) {
		t.Error("Has() returned true for different base version")
	}
}

// --- Install -> Cache -> Build flow test ---

// TestCacheInstallCacheBuildFlow validates the full install->cache->build flow
// with constraint-based version lookup. It simulates:
//  1. Install: compile source files and store the output in the cache using
//     the base version resolved from a constraint (e.g., "^1.0.0" -> "1.0.0").
//  2. Cache: verify a subsequent lookup with the same constraint produces a hit.
//  3. Build: load the cached artifacts and confirm they match the original output.
//
// This exercises the known limitation where constraint-based caching uses the
// base version (e.g., "1.0.0") regardless of what version was actually installed.
func TestCacheInstallCacheBuildFlow(t *testing.T) {
	c := tempCache(t)

	constraint := "^1.0.0"
	pkgName := "example-lib"

	// --- Step 1: Install phase ---
	// Resolve the constraint to a cache key version.
	cacheVersion, err := ConstraintBaseVersion(constraint)
	if err != nil {
		t.Fatalf("ConstraintBaseVersion(%q) error: %v", constraint, err)
	}
	if cacheVersion != "1.0.0" {
		t.Fatalf("ConstraintBaseVersion(%q) = %q, want %q", constraint, cacheVersion, "1.0.0")
	}

	// Create source files to simulate compilation input.
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "lib.intent")
	if err := os.WriteFile(srcFile, []byte("fn greet() { return \"hello\" }"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	sourceFiles := []string{srcFile}

	// Simulate compiled output artifacts.
	compiledFiles := []CachedFile{
		{Name: "lib.ir", Content: []byte("compiled IR for greet()")},
		{Name: "lib.meta", Content: []byte(`{"exports":["greet"]}`)},
	}

	// Store compiled output with source checksum.
	if err := c.StoreWithChecksum(pkgName, cacheVersion, compiledFiles, sourceFiles, srcDir); err != nil {
		t.Fatalf("StoreWithChecksum() error: %v", err)
	}

	// --- Step 2: Cache lookup phase ---
	// A subsequent build resolves the same constraint and checks the cache.
	lookupVersion, err := ConstraintBaseVersion(constraint)
	if err != nil {
		t.Fatalf("ConstraintBaseVersion(%q) error on lookup: %v", constraint, err)
	}

	if !c.Has(pkgName, lookupVersion) {
		t.Fatal("cache miss: Has() returned false after install; expected cache hit")
	}

	// Verify source checksum still matches (no source changes).
	match, err := c.ChecksumMatch(pkgName, lookupVersion, sourceFiles, srcDir)
	if err != nil {
		t.Fatalf("ChecksumMatch() error: %v", err)
	}
	if !match {
		t.Fatal("ChecksumMatch() returned false; expected match (sources unchanged)")
	}

	// --- Step 3: Build phase ---
	// Load cached artifacts and verify they match what was stored.
	loaded, err := c.Load(pkgName, lookupVersion)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Expect compiled files + the checksum sentinel file.
	wantCount := len(compiledFiles) + 1 // +1 for .checksum
	if len(loaded) != wantCount {
		t.Fatalf("Load() returned %d files, want %d", len(loaded), wantCount)
	}

	fileMap := make(map[string]string)
	for _, f := range loaded {
		fileMap[f.Name] = string(f.Content)
	}

	if fileMap["lib.ir"] != "compiled IR for greet()" {
		t.Errorf("lib.ir content = %q, want %q", fileMap["lib.ir"], "compiled IR for greet()")
	}
	if fileMap["lib.meta"] != `{"exports":["greet"]}` {
		t.Errorf("lib.meta content = %q, want %q", fileMap["lib.meta"], `{"exports":["greet"]}`)
	}
	if _, ok := fileMap[checksumFile]; !ok {
		t.Error("checksum sentinel file missing from loaded cache")
	}
}

// --- FindMatchingVersion tests ---

func TestFindMatchingVersionExactMatch(t *testing.T) {
	c := tempCache(t)
	files := []CachedFile{{Name: "lib.ir", Content: []byte("data")}}
	if err := c.Store("mypkg", "1.0.0", files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	found, version := c.FindMatchingVersion("mypkg", "^1.0.0")
	if !found {
		t.Fatal("FindMatchingVersion() returned false, want true")
	}
	if version != "1.0.0" {
		t.Errorf("FindMatchingVersion() version = %q, want %q", version, "1.0.0")
	}
}

func TestFindMatchingVersionNoMatch(t *testing.T) {
	c := tempCache(t)
	files := []CachedFile{{Name: "lib.ir", Content: []byte("data")}}
	if err := c.Store("mypkg", "2.0.0", files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	found, version := c.FindMatchingVersion("mypkg", "^3.0.0")
	if found {
		t.Errorf("FindMatchingVersion() returned true with version %q, want false", version)
	}
}

func TestFindMatchingVersionHighest(t *testing.T) {
	c := tempCache(t)
	files := []CachedFile{{Name: "lib.ir", Content: []byte("data")}}
	for _, v := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		if err := c.Store("mypkg", v, files); err != nil {
			t.Fatalf("Store(%q) error: %v", v, err)
		}
	}

	found, version := c.FindMatchingVersion("mypkg", "^1.0.0")
	if !found {
		t.Fatal("FindMatchingVersion() returned false, want true")
	}
	if version != "1.2.0" {
		t.Errorf("FindMatchingVersion() version = %q, want %q", version, "1.2.0")
	}
}

func TestFindMatchingVersionInvalidConstraint(t *testing.T) {
	c := tempCache(t)
	files := []CachedFile{{Name: "lib.ir", Content: []byte("data")}}
	if err := c.Store("mypkg", "1.0.0", files); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	found, _ := c.FindMatchingVersion("mypkg", "not a valid constraint!!!")
	if found {
		t.Error("FindMatchingVersion() returned true for invalid constraint, want false")
	}
}

func TestFindMatchingVersionTraversalProtection(t *testing.T) {
	c := tempCache(t)

	traversalNames := []string{"../evil", "foo/bar", ".."}
	for _, name := range traversalNames {
		found, _ := c.FindMatchingVersion(name, "^1.0.0")
		if found {
			t.Errorf("FindMatchingVersion(%q, ...) returned true, want false (traversal rejected)", name)
		}
	}
}

// --- Checksum collision tests ---

func TestChecksumDifferentDirsSameBasename(t *testing.T) {
	rootDir := t.TempDir()

	// Create two files with the same basename in different directories.
	dir1 := filepath.Join(rootDir, "dir1")
	dir2 := filepath.Join(rootDir, "dir2")
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	file1 := filepath.Join(dir1, "main.intent")
	file2 := filepath.Join(dir2, "main.intent")
	os.WriteFile(file1, []byte("fn a() {}"), 0644)
	os.WriteFile(file2, []byte("fn a() {}"), 0644)

	// Checksum of [dir1/main.intent] vs [dir2/main.intent] should differ
	// because they have different relative paths.
	sum1, err := computeChecksum(rootDir, []string{file1})
	if err != nil {
		t.Fatalf("computeChecksum() error: %v", err)
	}

	sum2, err := computeChecksum(rootDir, []string{file2})
	if err != nil {
		t.Fatalf("computeChecksum() error: %v", err)
	}

	if sum1 == sum2 {
		t.Errorf("checksums should differ for same-named files in different dirs, both = %s", sum1)
	}
}

func TestChecksumSameFileSameResult(t *testing.T) {
	rootDir := t.TempDir()
	file := filepath.Join(rootDir, "main.intent")
	os.WriteFile(file, []byte("fn main() {}"), 0644)

	sum1, err := computeChecksum(rootDir, []string{file})
	if err != nil {
		t.Fatalf("computeChecksum() error: %v", err)
	}

	sum2, err := computeChecksum(rootDir, []string{file})
	if err != nil {
		t.Fatalf("computeChecksum() error: %v", err)
	}

	if sum1 != sum2 {
		t.Errorf("same file should produce same checksum: %s != %s", sum1, sum2)
	}
}

// --- Phase 30 / ADR 0039 §5: git-source cache layout tests ---

func TestParseGitURLBasic(t *testing.T) {
	cases := []struct {
		in         string
		host, o, r string
		wantErr    bool
	}{
		{"github.com/lhaig/foo", "github.com", "lhaig", "foo", false},
		{"https://github.com/lhaig/foo.git", "github.com", "lhaig", "foo", false},
		{"git@github.com:lhaig/foo.git", "github.com", "lhaig", "foo", false},
		{"github.com/lhaig/foo/extra", "", "", "", true}, // sub-path inside repo
		{"foo", "", "", "", true},                        // too few segments
		{"github.com/../foo/bar", "", "", "", true},      // traversal
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			host, owner, repo, err := ParseGitURL(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if host != c.host || owner != c.o || repo != c.r {
				t.Errorf("got %s/%s/%s, want %s/%s/%s", host, owner, repo, c.host, c.o, c.r)
			}
		})
	}
}

func TestGitCachePathLayout(t *testing.T) {
	c := &PackageCache{CacheDir: filepath.Join(t.TempDir(), "intentcache")}
	got, err := c.GitCachePath("github.com", "lhaig", "foo", "abc123def")
	if err != nil {
		t.Fatalf("GitCachePath: %v", err)
	}
	want := filepath.Join(c.CacheDir, "git", "github.com", "lhaig", "foo@abc123def")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestGitCachePathRejectsTraversal(t *testing.T) {
	c := &PackageCache{CacheDir: t.TempDir()}
	if _, err := c.GitCachePath("..", "lhaig", "foo", "abc"); err == nil {
		t.Error("expected error on traversal in host")
	}
	if _, err := c.GitCachePath("github.com", "lhaig", "..", "abc"); err == nil {
		t.Error("expected error on traversal in repo")
	}
}

func TestRefreshGitWipesOnlyGitEntries(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "intentcache")
	c := &PackageCache{CacheDir: cacheDir}

	// Plant a legacy entry and a git entry.
	legacyDir := filepath.Join(cacheDir, "legacy_pkg", "1.0.0")
	gitDir, err := c.GitCachePath("github.com", "lhaig", "foo", "abc")
	if err != nil {
		t.Fatalf("GitCachePath: %v", err)
	}
	for _, d := range []string{legacyDir, gitDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
		if err := os.WriteFile(filepath.Join(d, "x.intent"), []byte("hi"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if err := c.RefreshGit(); err != nil {
		t.Fatalf("RefreshGit: %v", err)
	}

	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		t.Error("git cache entry should have been removed")
	}
	if _, err := os.Stat(legacyDir); err != nil {
		t.Error("legacy cache entry should be untouched")
	}
}

func TestGitTreeChecksumMatchesTreeHash(t *testing.T) {
	c := &PackageCache{CacheDir: filepath.Join(t.TempDir(), "intentcache")}
	dir, err := c.GitCachePath("github.com", "lhaig", "foo", "abc")
	if err != nil {
		t.Fatalf("GitCachePath: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte("[package]\nname = \"foo\"\nversion = \"1.0.0\"\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := c.GitTreeChecksum("github.com", "lhaig", "foo", "abc")
	if err != nil {
		t.Fatalf("GitTreeChecksum: %v", err)
	}
	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", got)
	}
	expected, _ := TreeHash(dir)
	if got != FormatChecksum(expected) {
		t.Errorf("checksum mismatch: got %s, want %s", got, FormatChecksum(expected))
	}
}
