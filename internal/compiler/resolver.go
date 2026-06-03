package compiler

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedPackage describes one entry in the build list produced by MVS.
type ResolvedPackage struct {
	Name    string
	Source  DependencySpec // Git source or Path source
	Version Version        // resolved version (highest patch at the minimum's major+minor floor)
	Rev     string         // git commit hash (empty for path deps)
	Deps    []string       // names of direct deps declared by this package
}

// ResolvedSet is the deterministic output of MVS resolution, sorted by name.
type ResolvedSet struct {
	Packages []ResolvedPackage
}

// ManifestLoader fetches and parses a dep's intent.toml at a chosen version.
// The Resolver depends on this rather than Fetcher directly so tests can
// inject synthetic dependency graphs without needing real git fixtures.
type ManifestLoader interface {
	// LoadAt resolves source at >= min (MVS: highest patch at min's major+minor
	// floor, same major as min) and parses the manifest at that version.
	// Returns the parsed manifest, resolved version, and rev. For Path
	// sources, version and rev are taken from the loaded manifest's
	// [package] block; min is ignored.
	LoadAt(name string, source DependencySpec, min Version) (manifest *Manifest, version Version, rev string, err error)
}

// Resolver implements minimum-version selection over a dependency graph
// described by intent.toml manifests. See ADR 0039 §6.
type Resolver struct {
	Loader ManifestLoader
}

// Resolve walks the transitive dependency graph rooted at the given manifest
// and returns the deterministic build list. The rootDir is used to surface
// where ambiguity originates in error messages.
func (r *Resolver) Resolve(root *Manifest, rootDir string) (*ResolvedSet, error) {
	if r.Loader == nil {
		return nil, fmt.Errorf("resolver: ManifestLoader is required")
	}

	// MVS state: for each package name, track (max-minimum-seen, source).
	type entry struct {
		minimum   Version
		source    DependencySpec
		parents   []string // names of packages that pulled this in (best-effort error context)
		fetchedAt Version  // version we last loaded the manifest at; zero = never
		manifest  *Manifest
		resolved  Version
		rev       string
	}
	state := map[string]*entry{}

	// queueItem is a "this parent asks for this child at this minimum" edge.
	type queueItem struct {
		parent string
		name   string
		spec   DependencySpec
		min    Version // zero for path deps
	}
	var queue []queueItem
	rootName := root.Package.Name
	if rootName == "" {
		rootName = "<root>"
	}
	for name, dep := range root.Dependencies {
		min, err := minVersionForDep(name, dep)
		if err != nil {
			return nil, err
		}
		queue = append(queue, queueItem{parent: rootName, name: name, spec: dep, min: min})
	}

	// BFS until no minimums change. Bounded by depth and version monotonicity.
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		e, seen := state[item.name]
		if !seen {
			e = &entry{minimum: item.min, source: item.spec, parents: []string{item.parent}}
			state[item.name] = e
		} else {
			// Cross-major conflict is fatal (ADR 0039 §6 — no diamond tolerance).
			if !item.spec.IsPath() && !e.source.IsPath() && e.minimum.Major != item.min.Major {
				return nil, fmt.Errorf(
					"package %q: cross-major version conflict — %s wants %d.x, %s wants %d.x",
					item.name,
					strings.Join(e.parents, ","), e.minimum.Major,
					item.parent, item.min.Major,
				)
			}
			// Source mismatch (two parents point at different git URLs or
			// mix git + path) is also a hard error in v1.
			if !sourcesEqual(e.source, item.spec) {
				return nil, fmt.Errorf(
					"package %q: source mismatch — %s declares %s, %s declares %s",
					item.name,
					strings.Join(e.parents, ","), describeSource(e.source),
					item.parent, describeSource(item.spec),
				)
			}
			e.parents = append(e.parents, item.parent)
			// Take the max minimum (MVS).
			if item.min.Compare(e.minimum) > 0 {
				e.minimum = item.min
			}
		}

		// Fetch / re-fetch this dep's manifest if we haven't loaded it at
		// the current (possibly bumped) minimum yet.
		if e.manifest == nil || e.fetchedAt.Compare(e.minimum) < 0 {
			m, ver, rev, err := r.Loader.LoadAt(item.name, e.source, e.minimum)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", item.name, err)
			}
			e.manifest = m
			e.fetchedAt = e.minimum
			e.resolved = ver
			e.rev = rev
			// Push transitive deps onto the queue.
			for childName, childDep := range m.Dependencies {
				childMin, err := minVersionForDep(childName, childDep)
				if err != nil {
					return nil, fmt.Errorf("resolve %s: %w", item.name, err)
				}
				queue = append(queue, queueItem{parent: item.name, name: childName, spec: childDep, min: childMin})
			}
		}
	}

	// Materialise the build list, sorted by name for determinism.
	names := make([]string, 0, len(state))
	for n := range state {
		names = append(names, n)
	}
	sort.Strings(names)
	set := &ResolvedSet{Packages: make([]ResolvedPackage, 0, len(names))}
	for _, n := range names {
		e := state[n]
		direct := make([]string, 0, len(e.manifest.Dependencies))
		for d := range e.manifest.Dependencies {
			direct = append(direct, d)
		}
		sort.Strings(direct)
		set.Packages = append(set.Packages, ResolvedPackage{
			Name:    n,
			Source:  e.source,
			Version: e.resolved,
			Rev:     e.rev,
			Deps:    direct,
		})
	}
	return set, nil
}

// IsPath reports whether the spec is a local-path dependency.
func (d DependencySpec) IsPath() bool { return d.Path != "" }

// IsGit reports whether the spec is a git-sourced dependency.
func (d DependencySpec) IsGit() bool { return d.Git != "" }

func sourcesEqual(a, b DependencySpec) bool {
	switch {
	case a.IsGit() && b.IsGit():
		return a.Git == b.Git
	case a.IsPath() && b.IsPath():
		// Paths are compared after cleaning to absorb "./", "//" differences.
		return filepath.Clean(a.Path) == filepath.Clean(b.Path)
	case !a.IsGit() && !a.IsPath() && !b.IsGit() && !b.IsPath():
		// Bare-version legacy form on both sides — assume same registry.
		return true
	default:
		return false
	}
}

func describeSource(d DependencySpec) string {
	switch {
	case d.IsGit():
		return fmt.Sprintf("git %q", d.Git)
	case d.IsPath():
		return fmt.Sprintf("path %q", d.Path)
	default:
		return fmt.Sprintf("version %q (bare)", d.Version)
	}
}

// minVersionForDep extracts the MVS minimum from a DependencySpec.
// Path deps return the zero Version (irrelevant for path resolution).
// Constraint prefixes (^, ~) are stripped — ADR 0039 §6 reinterprets them
// as plain minima.
func minVersionForDep(name string, d DependencySpec) (Version, error) {
	if d.IsPath() {
		return Version{}, nil
	}
	v := strings.TrimSpace(d.Version)
	if len(v) == 0 {
		return Version{}, fmt.Errorf("dependency %q has no version", name)
	}
	if v[0] == '^' || v[0] == '~' || v[0] == '>' || v[0] == '=' {
		// Allow ">=", ">", "=", "^", "~" — strip operator chars; the base
		// number is the MVS minimum. ADR 0039 §6.
		v = strings.TrimLeft(v, "^~>=")
		v = strings.TrimSpace(v)
	}
	ver, err := ParseVersion(v)
	if err != nil {
		return Version{}, fmt.Errorf("dependency %q: %w", name, err)
	}
	return ver, nil
}
