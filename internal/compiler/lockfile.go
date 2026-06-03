package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// LockfileFormatVersion is the lockfile schema version this build of intentc
// produces and is willing to read. Future versions can extend the schema
// (signing fields, etc.) by bumping this number.
const LockfileFormatVersion = 1

// Lockfile is the parsed shape of intent.lock. Phase 30 / ADR 0039 §4.
type Lockfile struct {
	Version   int
	Generated time.Time
	Packages  []LockedPackage
}

// LockedPackage is one [[package]] entry in the lockfile.
type LockedPackage struct {
	Name         string
	Version      Version
	Source       string   // canonical form: "git+<url>" or "path+<relpath>"
	Rev          string   // git commit hash (empty for path sources)
	Checksum     string   // "sha256:<hex>" of the cached tree (TreeHash)
	Dependencies []string // direct dep entries, format "<name> <version>"
}

// CanonicalSource renders a DependencySpec as the lockfile `source` string.
func CanonicalSource(d DependencySpec) string {
	switch {
	case d.IsGit():
		return "git+" + gitURL(d.Git)
	case d.IsPath():
		return "path+" + d.Path
	default:
		return ""
	}
}

// FromResolvedSet materialises a Lockfile from the resolver output, computing
// checksums via the supplied function (typically TreeHash over the cached
// tree). The function is injected so tests can pass a stub.
func FromResolvedSet(rs *ResolvedSet, checksumOf func(LockedPackage) (string, error), now time.Time) (*Lockfile, error) {
	lock := &Lockfile{
		Version:   LockfileFormatVersion,
		Generated: now.UTC().Truncate(time.Second),
	}
	for _, p := range rs.Packages {
		entry := LockedPackage{
			Name:         p.Name,
			Version:      p.Version,
			Source:       CanonicalSource(p.Source),
			Rev:          p.Rev,
			Dependencies: append([]string(nil), p.Deps...),
		}
		// Direct deps are stored as "<name> <version>" — version is the
		// dep's resolved version, looked up from the resolved set.
		byName := map[string]ResolvedPackage{}
		for _, rp := range rs.Packages {
			byName[rp.Name] = rp
		}
		for i, depName := range entry.Dependencies {
			if rp, ok := byName[depName]; ok {
				entry.Dependencies[i] = fmt.Sprintf("%s %s", depName, rp.Version.String())
			}
		}
		if checksumOf != nil {
			sum, err := checksumOf(entry)
			if err != nil {
				return nil, fmt.Errorf("checksum %s: %w", p.Name, err)
			}
			entry.Checksum = sum
		}
		lock.Packages = append(lock.Packages, entry)
	}
	sort.Slice(lock.Packages, func(i, j int) bool {
		return lock.Packages[i].Name < lock.Packages[j].Name
	})
	return lock, nil
}

// FormatChecksum encodes a raw 32-byte sha256 as the lockfile checksum form.
func FormatChecksum(sum []byte) string {
	return "sha256:" + hex.EncodeToString(sum)
}

// ParseChecksum decodes a lockfile checksum back to raw bytes. Returns an
// error if the prefix doesn't match or the hex is malformed.
func ParseChecksum(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "sha256:") {
		return nil, fmt.Errorf("checksum %q: missing sha256: prefix", s)
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(s, "sha256:"))
	if err != nil {
		return nil, fmt.Errorf("checksum %q: %w", s, err)
	}
	if len(raw) != sha256.Size {
		return nil, fmt.Errorf("checksum %q: expected %d bytes, got %d", s, sha256.Size, len(raw))
	}
	return raw, nil
}

// WriteLockfile serialises a Lockfile to the given path. The output is
// deterministic — packages sorted by name, fields in fixed order — so
// committing it to source control produces stable diffs.
func WriteLockfile(path string, l *Lockfile) error {
	var sb strings.Builder
	sb.WriteString("# intent.lock — DO NOT EDIT BY HAND\n")
	sb.WriteString(fmt.Sprintf("version = %d\n", l.Version))
	sb.WriteString(fmt.Sprintf("generated = %q\n", l.Generated.UTC().Format(time.RFC3339)))
	sb.WriteString("\n")

	pkgs := append([]LockedPackage(nil), l.Packages...)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })

	for _, p := range pkgs {
		sb.WriteString("[[package]]\n")
		sb.WriteString(fmt.Sprintf("name = %q\n", p.Name))
		sb.WriteString(fmt.Sprintf("version = %q\n", p.Version.String()))
		sb.WriteString(fmt.Sprintf("source = %q\n", p.Source))
		if p.Rev != "" {
			sb.WriteString(fmt.Sprintf("rev = %q\n", p.Rev))
		}
		if p.Checksum != "" {
			sb.WriteString(fmt.Sprintf("checksum = %q\n", p.Checksum))
		}
		if len(p.Dependencies) > 0 {
			deps := append([]string(nil), p.Dependencies...)
			sort.Strings(deps)
			quoted := make([]string, len(deps))
			for i, d := range deps {
				quoted[i] = fmt.Sprintf("%q", d)
			}
			sb.WriteString(fmt.Sprintf("dependencies = [%s]\n", strings.Join(quoted, ", ")))
		} else {
			sb.WriteString("dependencies = []\n")
		}
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// ReadLockfile parses an intent.lock from disk. Strict on the format version
// — refuses to read a lockfile from a future intentc.
func ReadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseLockfile(data)
}

func parseLockfile(data []byte) (*Lockfile, error) {
	lines := strings.Split(string(data), "\n")
	lock := &Lockfile{}
	var cur *LockedPackage

	commit := func() error {
		if cur != nil {
			// Validate required fields on commit.
			if cur.Name == "" || cur.Source == "" {
				return fmt.Errorf("lockfile: [[package]] entry missing name or source")
			}
			lock.Packages = append(lock.Packages, *cur)
		}
		cur = nil
		return nil
	}

	for i, raw := range lines {
		lineNum := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[package]]" {
			if err := commit(); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			cur = &LockedPackage{}
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: expected key = value", lineNum)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])

		if cur == nil {
			// Top-level key.
			switch key {
			case "version":
				n, err := parseInt(val, lineNum)
				if err != nil {
					return nil, err
				}
				if n > LockfileFormatVersion {
					return nil, fmt.Errorf("line %d: lockfile format version %d is newer than supported (%d) — upgrade intentc", lineNum, n, LockfileFormatVersion)
				}
				lock.Version = n
			case "generated":
				s, err := parseString(val, lineNum)
				if err != nil {
					return nil, err
				}
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					return nil, fmt.Errorf("line %d: generated timestamp: %w", lineNum, err)
				}
				lock.Generated = t
			default:
				return nil, fmt.Errorf("line %d: unknown top-level key %q", lineNum, key)
			}
			continue
		}

		// Package-scoped keys.
		switch key {
		case "name":
			s, err := parseString(val, lineNum)
			if err != nil {
				return nil, err
			}
			cur.Name = s
		case "version":
			s, err := parseString(val, lineNum)
			if err != nil {
				return nil, err
			}
			v, err := ParseVersion(s)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			cur.Version = v
		case "source":
			s, err := parseString(val, lineNum)
			if err != nil {
				return nil, err
			}
			cur.Source = s
		case "rev":
			s, err := parseString(val, lineNum)
			if err != nil {
				return nil, err
			}
			cur.Rev = s
		case "checksum":
			s, err := parseString(val, lineNum)
			if err != nil {
				return nil, err
			}
			if _, err := ParseChecksum(s); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			cur.Checksum = s
		case "dependencies":
			deps, err := parseStringArray(val, lineNum)
			if err != nil {
				return nil, fmt.Errorf("line %d: dependencies: %w", lineNum, err)
			}
			cur.Dependencies = deps
		default:
			return nil, fmt.Errorf("line %d: unknown package key %q", lineNum, key)
		}
	}
	if err := commit(); err != nil {
		return nil, err
	}
	if lock.Version == 0 {
		return nil, fmt.Errorf("lockfile missing required `version = N` line")
	}
	return lock, nil
}

func parseInt(value string, lineNum int) (int, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, fmt.Errorf("line %d: empty integer", lineNum)
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("line %d: expected integer, got %q", lineNum, value)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// Verify re-computes the tree-hash of each cached package and compares
// against the lockfile checksum. checksumOf is injected so tests don't need
// real cached trees.
func (l *Lockfile) Verify(checksumOf func(LockedPackage) (string, error)) error {
	for _, p := range l.Packages {
		if p.Checksum == "" {
			continue // path deps don't have checksums in v1
		}
		got, err := checksumOf(p)
		if err != nil {
			return fmt.Errorf("verify %s: %w", p.Name, err)
		}
		if got != p.Checksum {
			return fmt.Errorf("checksum mismatch for %s@%s: lockfile %s, computed %s — run `intentc pkg install --refresh`",
				p.Name, p.Version.String(), p.Checksum, got)
		}
	}
	return nil
}
