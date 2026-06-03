package compiler

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Tag is a parsed semver tag returned by GitFetcher.ListTags.
type Tag struct {
	Version Version // parsed semver
	Rev     string  // full commit hash the tag points at
	Raw     string  // original tag name, e.g. "v1.2.3"
}

// Fetcher abstracts the git operations the resolver needs. Production code
// uses GitFetcher; tests can supply alternatives backed by local fixtures.
type Fetcher interface {
	// ListTags enumerates semver-style tags (vMAJOR.MINOR.PATCH) on the
	// remote, returned sorted descending. Non-semver tags are ignored.
	ListTags(url string) ([]Tag, error)
	// Clone shallow-clones url into a fresh directory at dest and checks out
	// the given rev (commit hash or tag name). The .git directory is removed
	// after checkout so dest holds only the package tree.
	Clone(url, rev, dest string) error
}

// GitFetcher invokes the system `git` binary. It honours whatever
// credentials git is already configured with (SSH keys, credential
// helpers, gh auth) — intentc never prompts for or stores credentials
// (ADR 0039 §10).
type GitFetcher struct{}

// gitURL converts a short URL like "github.com/lhaig/foo" to a full
// "https://github.com/lhaig/foo.git" form. URLs that already include a
// scheme, SSH-style URLs, and local filesystem paths are returned
// unchanged so test fixtures and self-hosted git servers work.
func gitURL(short string) string {
	if strings.Contains(short, "://") {
		return short
	}
	if strings.HasPrefix(short, "git@") {
		return short
	}
	// Local filesystem paths are passed through verbatim.
	if strings.HasPrefix(short, "/") || strings.HasPrefix(short, "./") || strings.HasPrefix(short, "../") {
		return short
	}
	out := "https://" + short
	if !strings.HasSuffix(out, ".git") {
		out += ".git"
	}
	return out
}

// ListTags runs `git ls-remote --tags <url>` and parses the output.
// Annotated-tag `^{}` lines override the lightweight tag SHA so the rev
// recorded points at the commit, not the tag object.
func (g GitFetcher) ListTags(url string) ([]Tag, error) {
	out, err := exec.Command("git", "ls-remote", "--tags", gitURL(url)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git ls-remote %s: %s", url, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git ls-remote %s: %w", url, err)
	}
	return parseLsRemoteTags(string(out)), nil
}

// parseLsRemoteTags is the lines→tags parser, factored out for testability.
func parseLsRemoteTags(output string) []Tag {
	byName := map[string]Tag{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <sha>\trefs/tags/<name>[^{}]
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		sha, ref := parts[0], parts[1]
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		name := strings.TrimPrefix(ref, "refs/tags/")
		dereferenced := strings.HasSuffix(name, "^{}")
		name = strings.TrimSuffix(name, "^{}")
		if !strings.HasPrefix(name, "v") {
			continue
		}
		v, err := ParseVersion(strings.TrimPrefix(name, "v"))
		if err != nil {
			continue
		}
		// Annotated tags appear twice: once pointing at the tag object,
		// then again with ^{} pointing at the commit. We always prefer
		// the dereferenced (^{}) line for the commit SHA, but the first
		// line establishes the entry so we don't lose lightweight tags.
		existing, seen := byName[name]
		if !seen {
			byName[name] = Tag{Version: v, Rev: sha, Raw: name}
		} else if dereferenced {
			existing.Rev = sha
			byName[name] = existing
		}
	}
	tags := make([]Tag, 0, len(byName))
	for _, t := range byName {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Version.Compare(tags[j].Version) > 0
	})
	return tags
}

// Clone implements the Fetcher interface. The directory at dest must not
// exist; Clone creates it. Failure leaves the parent directory unchanged.
func (g GitFetcher) Clone(url, rev, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("clone destination %s already exists", dest)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dest, err)
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create parent %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, ".intent-clone-*")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	// Defer cleanup on failure; on success we rename tmp → dest below
	// so the cleanup becomes a no-op.
	defer os.RemoveAll(tmp)

	if err := runGit(tmp, "clone", "--quiet", "--no-checkout", gitURL(url), "."); err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	if err := runGit(tmp, "checkout", "--quiet", rev); err != nil {
		return fmt.Errorf("git checkout %s: %w", rev, err)
	}
	// Strip the .git directory so the cached tree is just the package.
	if err := os.RemoveAll(filepath.Join(tmp, ".git")); err != nil {
		return fmt.Errorf("strip .git: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename %s → %s: %w", tmp, dest, err)
	}
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// TreeHash computes the ADR 0039 §7 content-addressed hash of a directory.
// Files are visited in byte-sorted relative-path order. Each contributes
//
//	uint64LE(len(relpath)) || relpath || uint64LE(len(content)) || content
//
// to a sha256 stream. The .git directory and common OS junk are excluded.
func TreeHash(dir string) ([]byte, error) {
	files, err := collectTreeFiles(dir)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	var lenBuf [8]byte
	for _, rel := range files {
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(rel)))
		h.Write(lenBuf[:])
		h.Write([]byte(rel))

		f, err := os.Open(filepath.Join(dir, rel))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", rel, err)
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("stat %s: %w", rel, err)
		}
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(info.Size()))
		h.Write(lenBuf[:])
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		f.Close()
	}
	return h.Sum(nil), nil
}

// collectTreeFiles returns the package files relative to dir, sorted by
// raw byte order. Excludes .git/, .DS_Store, vendored caches, and any
// path component that starts with `.git`.
func collectTreeFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip excluded directories.
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "target" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		// Normalise to forward slashes for cross-platform stability.
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		if base == ".DS_Store" {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
