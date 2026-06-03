package compiler

import (
	"fmt"
	"path/filepath"
)

// GitFsLoader is the production ManifestLoader used by `intentc pkg install`.
// It composes a Fetcher (git operations) with a PackageCache (on-disk
// content-addressed storage) and a project root (for resolving Path
// dependencies). See ADR 0039 §6.
type GitFsLoader struct {
	Fetcher Fetcher
	Cache   *PackageCache
	// Root is the absolute path to the directory containing the project's
	// intent.toml. Path dependencies are resolved relative to it.
	Root string
}

// LoadAt implements ManifestLoader. For git sources it picks the highest tag
// at min's major with version >= min, clones if not already cached, and
// parses the cached intent.toml. For path sources it loads the manifest
// from the relative path in the project root.
func (l *GitFsLoader) LoadAt(name string, source DependencySpec, min Version) (*Manifest, Version, string, error) {
	if source.IsPath() {
		return l.loadPath(name, source)
	}
	if !source.IsGit() {
		return nil, Version{}, "", fmt.Errorf("dependency %q uses the bare-version short form, which is unsupported by the resolver (ADR 0039). Convert it to `%s = { git = \"<url>\", version = %q }` or `%s = { path = \"<dir>\" }`",
			name, name, source.Version, name)
	}
	return l.loadGit(name, source, min)
}

func (l *GitFsLoader) loadPath(name string, source DependencySpec) (*Manifest, Version, string, error) {
	resolved := source.Path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(l.Root, resolved)
	}
	m, err := LoadManifest(resolved)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("path dep %s: %w", name, err)
	}
	v, err := ParseVersion(m.Package.Version)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("path dep %s: invalid package version %q: %w", name, m.Package.Version, err)
	}
	return m, v, "", nil
}

func (l *GitFsLoader) loadGit(name string, source DependencySpec, min Version) (*Manifest, Version, string, error) {
	host, owner, repo, err := ParseGitURL(source.Git)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("dep %s: %w", name, err)
	}

	tags, err := l.Fetcher.ListTags(source.Git)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("dep %s: %w", name, err)
	}

	// Pick the highest tag at min's major with version >= min.
	var chosen Tag
	found := false
	for _, t := range tags {
		if t.Version.Major != min.Major {
			continue
		}
		if t.Version.Compare(min) < 0 {
			continue
		}
		if !found || t.Version.Compare(chosen.Version) > 0 {
			chosen = t
			found = true
		}
	}
	if !found {
		return nil, Version{}, "", fmt.Errorf("dep %s: no tag at major %d >= %s (available tags: %s)",
			name, min.Major, min.String(), summariseTags(tags))
	}

	// Ensure cached.
	cacheDir, err := l.Cache.GitCachePath(host, owner, repo, chosen.Rev)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("dep %s: %w", name, err)
	}
	if !l.Cache.HasGit(host, owner, repo, chosen.Rev) {
		if err := l.Fetcher.Clone(source.Git, chosen.Rev, cacheDir); err != nil {
			return nil, Version{}, "", fmt.Errorf("dep %s: clone: %w", name, err)
		}
	}

	m, err := LoadManifest(cacheDir)
	if err != nil {
		return nil, Version{}, "", fmt.Errorf("dep %s: parse cached manifest: %w", name, err)
	}
	return m, chosen.Version, chosen.Rev, nil
}

func summariseTags(tags []Tag) string {
	if len(tags) == 0 {
		return "<none>"
	}
	if len(tags) > 5 {
		tags = tags[:5]
	}
	out := ""
	for i, t := range tags {
		if i > 0 {
			out += ", "
		}
		out += t.Raw
	}
	return out
}
