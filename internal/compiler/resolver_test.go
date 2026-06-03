package compiler

import (
	"fmt"
	"strings"
	"testing"
)

// stubLoader implements ManifestLoader with a static "tag → manifest" map per
// package. Used to exercise the resolver without touching git or the filesystem.
type stubLoader struct {
	// versions["foo"] = sorted list of available tags
	versions map[string][]Version
	// manifests["foo@1.2.3"] = the parsed intent.toml at that version
	manifests map[string]*Manifest
}

func (s *stubLoader) LoadAt(name string, source DependencySpec, min Version) (*Manifest, Version, string, error) {
	if source.IsPath() {
		// Path deps: load the in-tree manifest. Tests bake it into
		// manifests[name+"@path"] for clarity.
		key := name + "@path"
		m, ok := s.manifests[key]
		if !ok {
			return nil, Version{}, "", fmt.Errorf("stub: no manifest for path dep %s", name)
		}
		ver, _ := ParseVersion(m.Package.Version)
		return m, ver, "", nil
	}
	// Git source: pick the highest tag at min's major, version >= min.
	tags := s.versions[name]
	var best Version
	bestKey := ""
	for _, t := range tags {
		if t.Major != min.Major {
			continue
		}
		if t.Compare(min) < 0 {
			continue
		}
		if bestKey == "" || t.Compare(best) > 0 {
			best = t
			bestKey = fmt.Sprintf("%s@%s", name, t.String())
		}
	}
	if bestKey == "" {
		return nil, Version{}, "", fmt.Errorf("stub: no version of %s satisfies %s.x >= %s", name, formatMajor(min), min)
	}
	m, ok := s.manifests[bestKey]
	if !ok {
		return nil, Version{}, "", fmt.Errorf("stub: missing manifest fixture for %s", bestKey)
	}
	rev := fmt.Sprintf("rev-%s", bestKey)
	return m, best, rev, nil
}

func formatMajor(v Version) string { return fmt.Sprintf("%d", v.Major) }

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("bad version literal %q: %v", s, err)
	}
	return v
}

// makeManifest is a tiny manifest factory for fixtures.
func makeManifest(name, version string, deps map[string]DependencySpec) *Manifest {
	if deps == nil {
		deps = map[string]DependencySpec{}
	}
	return &Manifest{
		Package:      PackageInfo{Name: name, Version: version},
		Dependencies: deps,
	}
}

func TestResolverSimpleChain(t *testing.T) {
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0"), mustVersion(t, "1.0.1")},
			"bar": {mustVersion(t, "2.0.0"), mustVersion(t, "2.0.1")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.1": makeManifest("foo", "1.0.1", map[string]DependencySpec{
				"bar": {Git: "github.com/lhaig/bar", Version: "2.0.0"},
			}),
			"bar@2.0.1": makeManifest("bar", "2.0.1", nil),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.0.0"},
	})

	rs, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(rs.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(rs.Packages))
	}
	byName := indexByName(rs.Packages)
	if got := byName["foo"].Version.String(); got != "1.0.1" {
		t.Errorf("foo resolved to %s, want 1.0.1", got)
	}
	if got := byName["bar"].Version.String(); got != "2.0.1" {
		t.Errorf("bar resolved to %s, want 2.0.1", got)
	}
}

func TestResolverMaxMinimumWins(t *testing.T) {
	// app -> foo >= 1.0.0, lib -> foo >= 1.5.0  => foo resolved to highest 1.x with min 1.5.0
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {
				mustVersion(t, "1.0.0"), mustVersion(t, "1.2.0"),
				mustVersion(t, "1.5.0"), mustVersion(t, "1.6.3"),
			},
			"lib": {mustVersion(t, "0.1.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.0": makeManifest("foo", "1.0.0", nil),
			"foo@1.6.3": makeManifest("foo", "1.6.3", nil),
			"lib@0.1.0": makeManifest("lib", "0.1.0", map[string]DependencySpec{
				"foo": {Git: "github.com/lhaig/foo", Version: "1.5.0"},
			}),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.0.0"},
		"lib": {Git: "github.com/lhaig/lib", Version: "0.1.0"},
	})
	rs, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	byName := indexByName(rs.Packages)
	if got := byName["foo"].Version.String(); got != "1.6.3" {
		t.Errorf("foo resolved to %s, want 1.6.3 (highest 1.x >= max minimum 1.5.0)", got)
	}
}

func TestResolverCrossMajorIsHardError(t *testing.T) {
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0"), mustVersion(t, "2.0.0")},
			"lib": {mustVersion(t, "0.1.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.0": makeManifest("foo", "1.0.0", nil),
			"lib@0.1.0": makeManifest("lib", "0.1.0", map[string]DependencySpec{
				"foo": {Git: "github.com/lhaig/foo", Version: "2.0.0"},
			}),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.0.0"},
		"lib": {Git: "github.com/lhaig/lib", Version: "0.1.0"},
	})
	_, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err == nil {
		t.Fatal("expected cross-major conflict error")
	}
	if !strings.Contains(err.Error(), "cross-major") {
		t.Errorf("expected cross-major error, got: %v", err)
	}
}

func TestResolverMissingVersionIsError(t *testing.T) {
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.0": makeManifest("foo", "1.0.0", nil),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.5.0"},
	})
	_, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err == nil {
		t.Fatal("expected error when no version satisfies the minimum")
	}
}

func TestResolverSourceMismatchIsError(t *testing.T) {
	// app -> foo from github.com/lhaig/foo
	// lib -> foo from github.com/fork/foo  (different URL — conflict)
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0")},
			"lib": {mustVersion(t, "0.1.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.0": makeManifest("foo", "1.0.0", nil),
			"lib@0.1.0": makeManifest("lib", "0.1.0", map[string]DependencySpec{
				"foo": {Git: "github.com/fork/foo", Version: "1.0.0"},
			}),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.0.0"},
		"lib": {Git: "github.com/lhaig/lib", Version: "0.1.0"},
	})
	_, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err == nil {
		t.Fatal("expected source-mismatch error")
	}
	if !strings.Contains(err.Error(), "source mismatch") {
		t.Errorf("expected source mismatch error, got: %v", err)
	}
}

func TestResolverDiamondSameMajor(t *testing.T) {
	// app -> [foo >= 1.0.0, bar >= 0.1.0]; bar -> foo >= 1.2.0
	// Both parents agree on major 1; max minimum is 1.2.0.
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0"), mustVersion(t, "1.2.0"), mustVersion(t, "1.4.5")},
			"bar": {mustVersion(t, "0.1.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.0.0": makeManifest("foo", "1.0.0", nil),
			"foo@1.4.5": makeManifest("foo", "1.4.5", nil),
			"bar@0.1.0": makeManifest("bar", "0.1.0", map[string]DependencySpec{
				"foo": {Git: "github.com/lhaig/foo", Version: "1.2.0"},
			}),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "1.0.0"},
		"bar": {Git: "github.com/lhaig/bar", Version: "0.1.0"},
	})
	rs, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	byName := indexByName(rs.Packages)
	if got := byName["foo"].Version.String(); got != "1.4.5" {
		t.Errorf("foo resolved to %s, want 1.4.5", got)
	}
}

func TestResolverAcceptsCaretAsMinimum(t *testing.T) {
	loader := &stubLoader{
		versions: map[string][]Version{
			"foo": {mustVersion(t, "1.0.0"), mustVersion(t, "1.5.0")},
		},
		manifests: map[string]*Manifest{
			"foo@1.5.0": makeManifest("foo", "1.5.0", nil),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"foo": {Git: "github.com/lhaig/foo", Version: "^1.0.0"}, // legacy form
	})
	rs, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	byName := indexByName(rs.Packages)
	if got := byName["foo"].Version.String(); got != "1.5.0" {
		t.Errorf("foo resolved to %s, want 1.5.0 (^1.0.0 treated as >= 1.0.0)", got)
	}
}

func TestResolverPathDepResolves(t *testing.T) {
	loader := &stubLoader{
		manifests: map[string]*Manifest{
			"local@path": makeManifest("local", "0.0.1", nil),
		},
	}
	root := makeManifest("app", "0.1.0", map[string]DependencySpec{
		"local": {Path: "../local"},
	})
	rs, err := (&Resolver{Loader: loader}).Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	byName := indexByName(rs.Packages)
	if got := byName["local"].Version.String(); got != "0.0.1" {
		t.Errorf("local resolved to %s, want 0.0.1 (from manifest)", got)
	}
	if byName["local"].Rev != "" {
		t.Errorf("path dep should have empty Rev, got %q", byName["local"].Rev)
	}
}

func indexByName(pkgs []ResolvedPackage) map[string]ResolvedPackage {
	out := map[string]ResolvedPackage{}
	for _, p := range pkgs {
		out[p.Name] = p
	}
	return out
}
