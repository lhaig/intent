package compiler

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version (major.minor.patch).
type Version struct {
	Major, Minor, Patch int
}

// Constraint represents a version constraint with an operator and a target version.
type Constraint struct {
	Op      string // "=", "^", "~", ">=", "<"
	Version Version
}

// ParseVersion parses a version string in "major.minor.patch" format.
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format %q: expected major.minor.patch", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("invalid major version in %q", s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("invalid minor version in %q", s)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, fmt.Errorf("invalid patch version in %q", s)
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String formats the version back to "major.minor.patch".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// ParseConstraint parses a single constraint string like "^1.0.0", "~1.0.0",
// ">=1.0.0", "<2.0.0", "=1.0.0", or "1.0.0".
//
// Limitation: combined range constraints (e.g., ">=1.0.0 <2.0.0") are not
// supported. Each constraint string must contain exactly one operator and
// version. To express a range, callers should parse each bound separately
// and pass both Constraint values to ValidateNoConflicts or check Matches
// individually.
func ParseConstraint(s string) (Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Constraint{}, fmt.Errorf("empty constraint string")
	}

	var op string
	var versionStr string

	switch {
	// Multi-char operators (must precede their single-char counterparts to avoid prefix ambiguity).
	case strings.HasPrefix(s, ">="):
		op = ">="
		versionStr = s[2:]
	case strings.HasPrefix(s, "<="):
		return Constraint{}, fmt.Errorf("unsupported constraint operator %q", "<=")
	// Single-char operators.
	case strings.HasPrefix(s, "^"):
		op = "^"
		versionStr = s[1:]
	case strings.HasPrefix(s, "~"):
		op = "~"
		versionStr = s[1:]
	case strings.HasPrefix(s, "<"):
		op = "<"
		versionStr = s[1:]
	case strings.HasPrefix(s, "="):
		op = "="
		versionStr = s[1:]
	case strings.HasPrefix(s, ">"):
		return Constraint{}, fmt.Errorf("unsupported constraint operator %q", ">")
	default:
		op = "="
		versionStr = s
	}

	v, err := ParseVersion(versionStr)
	if err != nil {
		return Constraint{}, fmt.Errorf("invalid constraint %q: %w", s, err)
	}

	return Constraint{Op: op, Version: v}, nil
}

// Matches checks if the given version satisfies this constraint.
func (c Constraint) Matches(v Version) bool {
	switch c.Op {
	case "=":
		return v.Compare(c.Version) == 0
	case "^":
		// Compatible: >=version AND <next breaking change.
		// For 0.0.x: only patch matches (upper = 0.0.(patch+1))
		// For 0.x.y: only minor matches (upper = 0.(minor+1).0)
		// For >=1.x.y: major matches (upper = (major+1).0.0)
		if v.Compare(c.Version) < 0 {
			return false
		}
		var upper Version
		switch {
		case c.Version.Major == 0 && c.Version.Minor == 0:
			upper = Version{Major: 0, Minor: 0, Patch: c.Version.Patch + 1}
		case c.Version.Major == 0:
			upper = Version{Major: 0, Minor: c.Version.Minor + 1, Patch: 0}
		default:
			upper = Version{Major: c.Version.Major + 1, Minor: 0, Patch: 0}
		}
		return v.Compare(upper) < 0
	case "~":
		// Patch only: >=version AND <next minor
		if v.Compare(c.Version) < 0 {
			return false
		}
		upper := Version{Major: c.Version.Major, Minor: c.Version.Minor + 1, Patch: 0}
		return v.Compare(upper) < 0
	case ">=":
		return v.Compare(c.Version) >= 0
	case "<":
		return v.Compare(c.Version) < 0
	default:
		return false
	}
}

// ValidateNoConflicts checks that no package has conflicting constraints.
// It returns an error if any package has constraints that cannot all be satisfied simultaneously.
func ValidateNoConflicts(deps map[string][]Constraint) error {
	for pkg, constraints := range deps {
		if len(constraints) <= 1 {
			continue
		}
		// Check if there exists at least one version that satisfies all constraints.
		// We test the constraint versions themselves and their boundaries.
		if !hasValidOverlap(constraints) {
			return fmt.Errorf("conflicting constraints for package %q: no version can satisfy all constraints", pkg)
		}
	}
	return nil
}

// versionInterval represents a half-open version range [Min, Max).
// A nil-like Max (maxVersion) means no upper bound.
type versionInterval struct {
	Min          Version
	Max          Version
	MinInclusive bool // true for >=, false for >
	MaxInclusive bool // true for <=, false for <
}

// maxVersion is a sentinel representing no upper bound.
var maxVersion = Version{Major: 1<<31 - 1, Minor: 1<<31 - 1, Patch: 1<<31 - 1}

// constraintToInterval converts a single constraint to a version interval.
func constraintToInterval(c Constraint) versionInterval {
	v := c.Version
	switch c.Op {
	case "=":
		return versionInterval{Min: v, Max: v, MinInclusive: true, MaxInclusive: true}
	case ">=":
		return versionInterval{Min: v, Max: maxVersion, MinInclusive: true, MaxInclusive: true}
	case "<":
		return versionInterval{Min: Version{0, 0, 0}, Max: v, MinInclusive: true, MaxInclusive: false}
	case "^":
		var upper Version
		switch {
		case v.Major == 0 && v.Minor == 0:
			upper = Version{Major: 0, Minor: 0, Patch: v.Patch + 1}
		case v.Major == 0:
			upper = Version{Major: 0, Minor: v.Minor + 1, Patch: 0}
		default:
			upper = Version{Major: v.Major + 1, Minor: 0, Patch: 0}
		}
		return versionInterval{Min: v, Max: upper, MinInclusive: true, MaxInclusive: false}
	case "~":
		upper := Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0}
		return versionInterval{Min: v, Max: upper, MinInclusive: true, MaxInclusive: false}
	default:
		// Unknown operator: empty interval
		return versionInterval{Min: Version{1, 0, 0}, Max: Version{0, 0, 0}, MinInclusive: true, MaxInclusive: false}
	}
}

// hasValidOverlap checks whether there exists any version satisfying all constraints
// by computing the intersection of their version intervals.
func hasValidOverlap(constraints []Constraint) bool {
	if len(constraints) == 0 {
		return true
	}

	// Start with the interval of the first constraint, then intersect with each subsequent one.
	result := constraintToInterval(constraints[0])
	for _, c := range constraints[1:] {
		iv := constraintToInterval(c)

		// New min = max of the two mins
		if iv.Min.Compare(result.Min) > 0 {
			result.Min = iv.Min
			result.MinInclusive = iv.MinInclusive
		} else if iv.Min.Compare(result.Min) == 0 {
			result.MinInclusive = result.MinInclusive && iv.MinInclusive
		}

		// New max = min of the two maxes
		if iv.Max.Compare(result.Max) < 0 {
			result.Max = iv.Max
			result.MaxInclusive = iv.MaxInclusive
		} else if iv.Max.Compare(result.Max) == 0 {
			result.MaxInclusive = result.MaxInclusive && iv.MaxInclusive
		}
	}

	// Check if the resulting interval is non-empty.
	cmp := result.Min.Compare(result.Max)
	if cmp < 0 {
		return true
	}
	if cmp == 0 {
		return result.MinInclusive && result.MaxInclusive
	}
	return false
}

// ConstraintBaseVersion parses a version constraint string and returns the
// base version suitable for use as a legacy cache key (e.g. "^1.0.0" → "1.0.0").
//
// Phase 30 / ADR 0039: this function is retained only for the back-compat
// path in ModuleRegistry.resolvePackageImport, which falls back to the
// legacy ~/.intent/cache/<name>/<version>/ layout for bare-version manifests.
// New code should use the resolved version from intent.lock instead. The
// MVS resolver (Resolver.Resolve) is the authoritative source for which
// version a dependency resolves to; this function never consults a registry.
func ConstraintBaseVersion(constraintStr string) (string, error) {
	c, err := ParseConstraint(constraintStr)
	if err != nil {
		return "", err
	}
	return c.Version.String(), nil
}
