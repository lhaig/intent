package compiler

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeLocalGitRepo creates a bare git repo with a single commit and a
// vMAJOR.MINOR.PATCH tag pointing at it. Returns the repo path (usable as
// a Fetcher URL because git ls-remote / clone accept local paths).
//
// Tests that depend on git invoke t.Skip when the git binary isn't on PATH,
// which keeps the suite green on minimal CI images.
func makeLocalGitRepo(t *testing.T, tag, fileName, fileBody string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not on PATH; skipping fetcher test")
	}
	dir := t.TempDir()

	// Initialise a non-bare working repo so we can commit + tag, then export
	// it as a bare clone which is what remote URLs typically point at.
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}
	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		// Tests must not interact with the user's global git config.
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
	if err := os.WriteFile(filepath.Join(work, fileName), []byte(fileBody), 0644); err != nil {
		t.Fatalf("write %s: %v", fileName, err)
	}
	gitCmd("add", fileName)
	gitCmd("commit", "--quiet", "-m", "init")
	gitCmd("tag", tag)

	bare := filepath.Join(dir, "bare.git")
	if out, err := exec.Command("git", "clone", "--quiet", "--bare", work, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone bare: %v\n%s", err, out)
	}
	return bare
}

func TestParseLsRemoteTagsBasic(t *testing.T) {
	output := "deadbeef\trefs/tags/v1.0.0\n" +
		"cafef00d\trefs/tags/v1.2.3\n" +
		"abad1dea\trefs/tags/v0.9.0\n"
	tags := parseLsRemoteTags(output)
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	// Sorted descending.
	if tags[0].Raw != "v1.2.3" || tags[1].Raw != "v1.0.0" || tags[2].Raw != "v0.9.0" {
		t.Errorf("unexpected order: %v", tags)
	}
}

func TestParseLsRemoteTagsDereferenced(t *testing.T) {
	// Annotated tag: a tag-object SHA, then the commit SHA on a ^{} line.
	output := "tagobjsha\trefs/tags/v2.0.0\n" +
		"commitsha\trefs/tags/v2.0.0^{}\n"
	tags := parseLsRemoteTags(output)
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Rev != "commitsha" {
		t.Errorf("expected dereferenced commit SHA, got %q", tags[0].Rev)
	}
}

func TestParseLsRemoteTagsIgnoresNonSemver(t *testing.T) {
	output := "deadbeef\trefs/tags/v1.0.0\n" +
		"cafef00d\trefs/tags/latest\n" +
		"abad1dea\trefs/tags/v1.0-rc1\n" +
		"baadf00d\trefs/heads/main\n"
	tags := parseLsRemoteTags(output)
	if len(tags) != 1 {
		t.Errorf("expected 1 semver tag, got %d: %v", len(tags), tags)
	}
}

func TestGitFetcherListTagsLocalRepo(t *testing.T) {
	repo := makeLocalGitRepo(t, "v1.0.0", "intent.toml", "[package]\nname = \"x\"\nversion = \"1.0.0\"\n")

	tags, err := GitFetcher{}.ListTags(repo)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 || tags[0].Raw != "v1.0.0" {
		t.Errorf("expected single tag v1.0.0, got %v", tags)
	}
	if tags[0].Version.Major != 1 || tags[0].Version.Minor != 0 || tags[0].Version.Patch != 0 {
		t.Errorf("unexpected version: %+v", tags[0].Version)
	}
}

func TestGitFetcherCloneLocalRepo(t *testing.T) {
	repo := makeLocalGitRepo(t, "v0.1.0", "main.intent",
		`module x version "0.1.0";`+"\n"+
			`entry function main() returns Int { return 0; }`+"\n")

	tags, err := GitFetcher{}.ListTags(repo)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) == 0 {
		t.Fatal("no tags found")
	}
	dest := filepath.Join(t.TempDir(), "cloned")
	if err := (GitFetcher{}).Clone(repo, tags[0].Rev, dest); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "main.intent")); err != nil {
		t.Errorf("expected main.intent in clone, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		t.Error(".git directory should have been stripped after checkout")
	}
}

func TestGitFetcherCloneRefusesExistingDest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not on PATH; skipping")
	}
	dest := filepath.Join(t.TempDir(), "preexisting")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err := GitFetcher{}.Clone("github.com/example/foo", "main", dest)
	if err == nil {
		t.Fatal("expected error when destination exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestTreeHashStableForSameContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.intent", "alpha")
	writeFile(t, dir, "sub/b.intent", "bravo")

	h1, err := TreeHash(dir)
	if err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	h2, err := TreeHash(dir)
	if err != nil {
		t.Fatalf("TreeHash repeat: %v", err)
	}
	if !bytes.Equal(h1, h2) {
		t.Errorf("hash unstable across runs: %x vs %x", h1, h2)
	}
}

func TestTreeHashChangesOnContentEdit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.intent", "alpha")
	h1, _ := TreeHash(dir)
	writeFile(t, dir, "a.intent", "alphaX")
	h2, _ := TreeHash(dir)
	if bytes.Equal(h1, h2) {
		t.Error("hash unchanged after content edit")
	}
}

func TestTreeHashIgnoresGitAndOSJunk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.intent", "alpha")
	h1, _ := TreeHash(dir)

	writeFile(t, dir, ".git/config", "[core]\n")
	writeFile(t, dir, ".DS_Store", "junk")
	writeFile(t, dir, "target/cache.bin", "build artefact")
	writeFile(t, dir, "vendor/foo/x.intent", "stale vendor copy")

	h2, _ := TreeHash(dir)
	if !bytes.Equal(h1, h2) {
		t.Errorf("hash changed after adding excluded junk: %x vs %x", h1, h2)
	}
}

func TestTreeHashDistinguishesPathFromContent(t *testing.T) {
	// Two trees with the same total bytes but a different split between
	// path and content must hash differently. This catches a length-prefix
	// implementation that conflates the two streams.
	dirA := t.TempDir()
	writeFile(t, dirA, "ab", "cd")
	dirB := t.TempDir()
	writeFile(t, dirB, "a", "bcd")
	hA, _ := TreeHash(dirA)
	hB, _ := TreeHash(dirB)
	if bytes.Equal(hA, hB) {
		t.Error("hashes collided across path/content reshuffle")
	}
}

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
