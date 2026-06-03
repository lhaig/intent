package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CachedFile represents a single file stored in the cache.
type CachedFile struct {
	Name    string
	Content []byte
}

// PackageCache caches compiled packages to avoid recompilation.
type PackageCache struct {
	CacheDir string // defaults to ~/.intent/cache
}

// validCacheName matches only safe cache name characters: alphanumeric, dot, hyphen, underscore.
var validCacheName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateCacheName rejects names containing path traversal sequences or unsafe characters.
func validateCacheName(s string) error {
	if s == "" {
		return fmt.Errorf("cache name must not be empty")
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("cache name %q contains path traversal sequence", s)
	}
	if strings.ContainsAny(s, "/\\") {
		return fmt.Errorf("cache name %q contains path separator", s)
	}
	if !validCacheName.MatchString(s) {
		return fmt.Errorf("cache name %q contains invalid characters", s)
	}
	return nil
}

// NewPackageCache creates a new PackageCache with the default cache directory.
func NewPackageCache() (*PackageCache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(home, ".intent", "cache")
	return &PackageCache{CacheDir: cacheDir}, nil
}

// CachePath returns the filesystem path for a cached package version.
// It validates both name and version to prevent path traversal.
func (c *PackageCache) CachePath(name, version string) (string, error) {
	if err := validateCacheName(name); err != nil {
		return "", fmt.Errorf("invalid package name: %w", err)
	}
	if err := validateCacheName(version); err != nil {
		return "", fmt.Errorf("invalid version: %w", err)
	}
	return filepath.Join(c.CacheDir, name, version), nil
}

// Has reports whether the cache contains the given package version.
func (c *PackageCache) Has(name, version string) bool {
	dir, err := c.CachePath(name, version)
	if err != nil {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// FindMatchingVersion scans the cache directory for any version of the named
// package that satisfies the given constraint string. It returns (true, version)
// if a match is found, or (false, "") otherwise.
func (c *PackageCache) FindMatchingVersion(name, constraintStr string) (bool, string) {
	if err := validateCacheName(name); err != nil {
		return false, ""
	}
	constraint, err := ParseConstraint(constraintStr)
	if err != nil {
		return false, ""
	}
	pkgDir := filepath.Join(c.CacheDir, name)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return false, ""
	}
	var bestVersion Version
	var bestName string
	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		v, err := ParseVersion(entry.Name())
		if err != nil {
			continue
		}
		if constraint.Matches(v) {
			if !found || v.Compare(bestVersion) > 0 {
				bestVersion = v
				bestName = entry.Name()
				found = true
			}
		}
	}
	if found {
		return true, bestName
	}
	return false, ""
}

// Store writes cached files for a package version, creating directories as needed.
func (c *PackageCache) Store(name, version string, files []CachedFile) error {
	dir, err := c.CachePath(name, version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	for _, f := range files {
		path := filepath.Clean(filepath.Join(dir, f.Name))
		// Reject file names that would escape the cache directory.
		if !strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return fmt.Errorf("cached file name %q escapes cache directory", f.Name)
		}
		// Ensure subdirectories exist for nested file names.
		if sub := filepath.Dir(path); sub != dir {
			if err := os.MkdirAll(sub, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory for %s: %w", f.Name, err)
			}
		}
		if err := os.WriteFile(path, f.Content, 0644); err != nil {
			return fmt.Errorf("failed to write cached file %s: %w", f.Name, err)
		}
	}

	return nil
}

// Load reads all cached files for a package version.
func (c *PackageCache) Load(name, version string) ([]CachedFile, error) {
	dir, err := c.CachePath(name, version)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache entry not found: %s@%s", name, version)
	}

	var files []CachedFile
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, CachedFile{Name: rel, Content: content})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read cache for %s@%s: %w", name, version, err)
	}

	return files, nil
}

// Invalidate removes the cached data for a package version.
func (c *PackageCache) Invalidate(name, version string) error {
	dir, err := c.CachePath(name, version)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // already gone
	}
	return os.RemoveAll(dir)
}

// checksumFile is the sentinel filename used to store the source checksum.
const checksumFile = ".checksum"

// computeChecksum computes a SHA256 digest over the sorted source file contents.
// rootDir is used to compute relative paths, ensuring files with the same basename
// in different directories produce different checksums.
func computeChecksum(rootDir string, sourceFiles []string) (string, error) {
	sorted := make([]string, len(sourceFiles))
	copy(sorted, sourceFiles)
	sort.Strings(sorted)

	h := sha256.New()
	for _, path := range sorted {
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to open source file %s: %w", path, err)
		}
		// Use the path relative to rootDir as the separator so that
		// same-named files in different directories produce different checksums.
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			rel = path // fallback to absolute path
		}
		fmt.Fprintf(h, "file:%s\n", rel)
		_, copyErr := io.Copy(h, f)
		if closeErr := f.Close(); closeErr != nil && copyErr == nil {
			return "", fmt.Errorf("failed to close source file %s: %w", path, closeErr)
		}
		if copyErr != nil {
			return "", fmt.Errorf("failed to read source file %s: %w", path, copyErr)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// StoreWithChecksum stores files and records the checksum of the given source files.
func (c *PackageCache) StoreWithChecksum(name, version string, files []CachedFile, sourceFiles []string, rootDir string) error {
	sum, err := computeChecksum(rootDir, sourceFiles)
	if err != nil {
		return err
	}

	// Prepend the checksum sentinel to the stored files.
	all := make([]CachedFile, 0, len(files)+1)
	all = append(all, CachedFile{Name: checksumFile, Content: []byte(sum)})
	all = append(all, files...)

	return c.Store(name, version, all)
}

// ChecksumMatch checks whether the cached checksum for a package version
// matches the current source files. Returns false if no cache entry exists.
func (c *PackageCache) ChecksumMatch(name, version string, sourceFiles []string, rootDir string) (bool, error) {
	dir, err := c.CachePath(name, version)
	if err != nil {
		return false, err
	}
	sumPath := filepath.Join(dir, checksumFile)
	stored, err := os.ReadFile(sumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read cached checksum: %w", err)
	}

	current, err := computeChecksum(rootDir, sourceFiles)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(stored)) == current, nil
}

// --- Phase 30 / ADR 0039 §5: git-source cache layout -----------------------
//
// Git-sourced packages are keyed by (host, owner, repo, rev) where rev is the
// full commit hash. This is content-addressed at the commit level — the same
// (host, owner, repo, rev) always points at the same tree, even if the source
// tag is later moved. The legacy CacheDir/<name>/<version>/ layout is
// preserved for ADR-0027-era bare-version manifests.

// gitSubdir is the root of the git-source cache namespace.
const gitSubdir = "git"

// validGitComponent matches a safe single path component: alphanumeric plus
// dot, hyphen, underscore. Excludes path separators and "..".
var validGitComponent = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateGitComponent(s, label string) error {
	if s == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("%s %q contains path traversal sequence", label, s)
	}
	if !validGitComponent.MatchString(s) {
		return fmt.Errorf("%s %q contains invalid characters", label, s)
	}
	return nil
}

// ParseGitURL decomposes a short URL like "github.com/lhaig/foo" or
// "https://github.com/lhaig/foo.git" into (host, owner, repo). The owner is
// the first path segment and repo is the second (further segments are
// rejected — sub-paths inside a repo are out of scope in v1).
//
// Local filesystem paths (starting with "/", "./", "../") are mapped to a
// sentinel "_local" host with the parent directory as owner and the basename
// (sans ".git") as repo. This keeps the cache layout intact for test fixtures
// without leaking arbitrary path structure into ~/.intent/cache/.
func ParseGitURL(url string) (host, owner, repo string, err error) {
	s := strings.TrimSpace(url)
	// Local filesystem paths get a synthetic key — they're only used by
	// tests and would otherwise be rejected by the host/owner/repo shape.
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return localPathCacheKey(s)
	}
	// Strip scheme.
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Strip user@ for SSH-style: git@github.com:lhaig/foo.git
	if at := strings.Index(s, "@"); at >= 0 && !strings.Contains(s[:at], "/") {
		s = s[at+1:]
		// SSH form uses ":" between host and path; normalise to "/".
		s = strings.Replace(s, ":", "/", 1)
	}
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(s, "/")
	// Filter empty segments (handles trailing slashes).
	clean := parts[:0]
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) < 3 {
		return "", "", "", fmt.Errorf("git URL %q: expected host/owner/repo", url)
	}
	if len(clean) > 3 {
		return "", "", "", fmt.Errorf("git URL %q: sub-paths inside a repo are not supported in v1", url)
	}
	host, owner, repo = clean[0], clean[1], clean[2]
	for _, pair := range []struct{ s, l string }{{host, "host"}, {owner, "owner"}, {repo, "repo"}} {
		if err := validateGitComponent(pair.s, pair.l); err != nil {
			return "", "", "", err
		}
	}
	return host, owner, repo, nil
}

// localPathCacheKey derives a (host, owner, repo) tuple from a local
// filesystem path. The owner is a sha256 prefix of the canonical path so two
// distinct local repos with the same basename don't collide.
func localPathCacheKey(p string) (host, owner, repo string, err error) {
	cleaned := filepath.Clean(p)
	abs, absErr := filepath.Abs(cleaned)
	if absErr == nil {
		cleaned = abs
	}
	base := filepath.Base(cleaned)
	base = strings.TrimSuffix(base, ".git")
	if base == "" || base == "." || base == "/" {
		return "", "", "", fmt.Errorf("local git path %q: cannot derive repo name", p)
	}
	sum := sha256.Sum256([]byte(cleaned))
	owner = hex.EncodeToString(sum[:4])
	// Validate the synthesized components against the same rules as
	// network-hosted ones so subsequent path joins remain safe.
	for _, pair := range []struct{ s, l string }{{"_local", "host"}, {owner, "owner"}, {base, "repo"}} {
		if err := validateGitComponent(pair.s, pair.l); err != nil {
			return "", "", "", err
		}
	}
	return "_local", owner, base, nil
}

// GitCachePath returns the cache path for a git-sourced package keyed by
// commit hash. ADR 0039 §5. Rev must be the full commit hash (40 hex chars).
func (c *PackageCache) GitCachePath(host, owner, repo, rev string) (string, error) {
	for _, pair := range []struct{ s, l string }{{host, "host"}, {owner, "owner"}, {repo, "repo"}, {rev, "rev"}} {
		if err := validateGitComponent(pair.s, pair.l); err != nil {
			return "", err
		}
	}
	return filepath.Join(c.CacheDir, gitSubdir, host, owner, repo+"@"+rev), nil
}

// HasGit reports whether the git cache contains a tree for (host, owner, repo, rev).
func (c *PackageCache) HasGit(host, owner, repo, rev string) bool {
	dir, err := c.GitCachePath(host, owner, repo, rev)
	if err != nil {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// RefreshGit wipes the git-source portion of the cache while leaving the
// legacy name/version entries alone. Used by `intentc pkg install --refresh`.
func (c *PackageCache) RefreshGit() error {
	dir := filepath.Join(c.CacheDir, gitSubdir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// GitTreeChecksum computes the ADR 0039 §7 tree-hash of a cached git package
// and returns the lockfile-format string. Used by the resolver / lockfile
// integration in CLI wiring (30.6).
func (c *PackageCache) GitTreeChecksum(host, owner, repo, rev string) (string, error) {
	dir, err := c.GitCachePath(host, owner, repo, rev)
	if err != nil {
		return "", err
	}
	sum, err := TreeHash(dir)
	if err != nil {
		return "", err
	}
	return FormatChecksum(sum), nil
}
