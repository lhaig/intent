package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockfileWriteReadRoundTrip(t *testing.T) {
	lock := &Lockfile{
		Version:   LockfileFormatVersion,
		Generated: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
		Packages: []LockedPackage{
			{
				Name:         "bar",
				Version:      mustVersion(t, "0.5.0"),
				Source:       "git+https://github.com/lhaig/bar.git",
				Rev:          "deadbeefcafef00d",
				Checksum:     "sha256:" + strings.Repeat("ab", 32),
				Dependencies: nil,
			},
			{
				Name:         "foo",
				Version:      mustVersion(t, "1.2.3"),
				Source:       "git+https://github.com/lhaig/foo.git",
				Rev:          "0123456789abcdef",
				Checksum:     "sha256:" + strings.Repeat("cd", 32),
				Dependencies: []string{"bar 0.5.0"},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "intent.lock")
	if err := WriteLockfile(path, lock); err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}
	got, err := ReadLockfile(path)
	if err != nil {
		t.Fatalf("ReadLockfile: %v", err)
	}
	if got.Version != LockfileFormatVersion {
		t.Errorf("Version: got %d, want %d", got.Version, LockfileFormatVersion)
	}
	if !got.Generated.Equal(lock.Generated) {
		t.Errorf("Generated: got %v, want %v", got.Generated, lock.Generated)
	}
	if len(got.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(got.Packages))
	}
	// Packages should be sorted: bar, foo.
	if got.Packages[0].Name != "bar" || got.Packages[1].Name != "foo" {
		t.Errorf("packages not sorted by name: %v", []string{got.Packages[0].Name, got.Packages[1].Name})
	}
	if got.Packages[1].Dependencies[0] != "bar 0.5.0" {
		t.Errorf("foo dependencies entry: got %q, want %q", got.Packages[1].Dependencies[0], "bar 0.5.0")
	}
}

func TestLockfileRejectsFutureVersion(t *testing.T) {
	body := `version = 99
generated = "2026-06-03T12:00:00Z"
`
	_, err := parseLockfile([]byte(body))
	if err == nil {
		t.Fatal("expected error on future lockfile version")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("expected version mismatch error, got: %v", err)
	}
}

func TestLockfileRejectsMissingVersion(t *testing.T) {
	body := `generated = "2026-06-03T12:00:00Z"
`
	_, err := parseLockfile([]byte(body))
	if err == nil {
		t.Fatal("expected error on missing version")
	}
}

func TestLockfileRejectsMalformedChecksum(t *testing.T) {
	body := `version = 1
generated = "2026-06-03T12:00:00Z"

[[package]]
name = "foo"
version = "1.0.0"
source = "git+https://example.com/foo.git"
checksum = "md5:abcdef"
dependencies = []
`
	_, err := parseLockfile([]byte(body))
	if err == nil {
		t.Fatal("expected error on non-sha256 checksum")
	}
}

func TestLockfileDeterministicOutput(t *testing.T) {
	// Same input → same bytes. Packages written in alpha order.
	lock := &Lockfile{
		Version:   LockfileFormatVersion,
		Generated: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
		Packages: []LockedPackage{
			{Name: "zeta", Version: mustVersion(t, "1.0.0"), Source: "git+a", Rev: "r1", Checksum: "sha256:" + strings.Repeat("00", 32)},
			{Name: "alpha", Version: mustVersion(t, "1.0.0"), Source: "git+b", Rev: "r2", Checksum: "sha256:" + strings.Repeat("11", 32)},
		},
	}
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.lock")
	path2 := filepath.Join(dir, "b.lock")
	if err := WriteLockfile(path1, lock); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := WriteLockfile(path2, lock); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	b1 := mustRead(t, path1)
	b2 := mustRead(t, path2)
	if string(b1) != string(b2) {
		t.Error("write was non-deterministic across two passes")
	}
	if !strings.Contains(string(b1), "\n[[package]]\nname = \"alpha\"") {
		t.Error("expected alpha to appear before zeta")
	}
	if strings.Index(string(b1), "alpha") > strings.Index(string(b1), "zeta") {
		t.Error("alpha must precede zeta in output")
	}
}

func TestLockfileVerifyDetectsMismatch(t *testing.T) {
	lock := &Lockfile{
		Version:   LockfileFormatVersion,
		Generated: time.Now(),
		Packages: []LockedPackage{
			{
				Name:     "foo",
				Version:  mustVersion(t, "1.0.0"),
				Source:   "git+https://example.com/foo.git",
				Rev:      "abc",
				Checksum: "sha256:" + strings.Repeat("aa", 32),
			},
		},
	}
	tamperer := func(p LockedPackage) (string, error) {
		return "sha256:" + strings.Repeat("bb", 32), nil
	}
	err := lock.Verify(tamperer)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch, got: %v", err)
	}
}

func TestLockfileVerifyAcceptsMatch(t *testing.T) {
	want := "sha256:" + strings.Repeat("aa", 32)
	lock := &Lockfile{
		Version:   LockfileFormatVersion,
		Generated: time.Now(),
		Packages: []LockedPackage{
			{Name: "foo", Version: mustVersion(t, "1.0.0"), Source: "git+x", Rev: "abc", Checksum: want},
		},
	}
	ok := func(p LockedPackage) (string, error) { return want, nil }
	if err := lock.Verify(ok); err != nil {
		t.Errorf("verify on matching checksum: %v", err)
	}
}

func TestFromResolvedSetPopulatesEverything(t *testing.T) {
	rs := &ResolvedSet{
		Packages: []ResolvedPackage{
			{Name: "foo", Source: DependencySpec{Git: "github.com/lhaig/foo"}, Version: mustVersion(t, "1.2.3"), Rev: "rev1", Deps: []string{"bar"}},
			{Name: "bar", Source: DependencySpec{Git: "github.com/lhaig/bar"}, Version: mustVersion(t, "0.5.0"), Rev: "rev2"},
		},
	}
	checksumOf := func(p LockedPackage) (string, error) {
		return "sha256:" + strings.Repeat("ff", 32), nil
	}
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	lock, err := FromResolvedSet(rs, checksumOf, now)
	if err != nil {
		t.Fatalf("FromResolvedSet: %v", err)
	}
	if len(lock.Packages) != 2 || lock.Packages[0].Name != "bar" || lock.Packages[1].Name != "foo" {
		t.Errorf("packages not sorted: %v", lock.Packages)
	}
	foo := lock.Packages[1]
	if foo.Source != "git+https://github.com/lhaig/foo.git" {
		t.Errorf("source canonicalisation: got %q", foo.Source)
	}
	if len(foo.Dependencies) != 1 || foo.Dependencies[0] != "bar 0.5.0" {
		t.Errorf("dependencies entry: got %v", foo.Dependencies)
	}
	if foo.Checksum == "" {
		t.Error("checksum was not populated")
	}
}

func TestParseChecksumRejectsBadHex(t *testing.T) {
	if _, err := ParseChecksum("sha256:xyz"); err == nil {
		t.Error("expected error on non-hex checksum")
	}
}

func TestParseChecksumRejectsWrongLength(t *testing.T) {
	if _, err := ParseChecksum("sha256:abcd"); err == nil {
		t.Error("expected error on short checksum")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
