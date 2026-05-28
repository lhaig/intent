package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest represents a parsed intent.toml file.
type Manifest struct {
	Package          PackageInfo
	Dependencies     map[string]DependencySpec
	RustDependencies map[string]RustDependencySpec // Phase 15 / ADR 0028
}

// RustDependencySpec describes a Cargo crate to bundle into the generated
// Cargo.toml when emitting Rust output. Mirrors a subset of Cargo's
// dependency table format.
type RustDependencySpec struct {
	Version  string   // version constraint (e.g. "1.5" or "^0.13") — empty when Path is set
	Path     string   // local path to a Cargo crate (relative to the intent.toml file)
	Features []string // optional feature flags
}

// CargoLine renders the spec as a single line for Cargo.toml's [dependencies]
// section. Examples:
//
//	blake3 = "1.5"
//	zstd = { version = "0.13", features = ["std"] }
//	blake3_intent = { path = "/abs/path/to/crate" }
//
// Path-style entries skip the version field. Callers that load the manifest
// from disk should resolve relative paths to absolute before calling
// CargoLine, since the generated Cargo.toml lives in a temp directory.
func (r RustDependencySpec) CargoLine(name string) string {
	if r.Path != "" {
		if len(r.Features) == 0 {
			return fmt.Sprintf("%s = { path = %q }", name, r.Path)
		}
		return fmt.Sprintf("%s = { path = %q, features = [%s] }",
			name, r.Path, quotedList(r.Features))
	}
	if len(r.Features) == 0 {
		return fmt.Sprintf("%s = %q", name, r.Version)
	}
	return fmt.Sprintf("%s = { version = %q, features = [%s] }",
		name, r.Version, quotedList(r.Features))
}

func quotedList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}

// PackageInfo holds package metadata from the [package] section.
type PackageInfo struct {
	Name        string
	Version     string
	Description string
}

// DependencySpec describes a single dependency.
type DependencySpec struct {
	Version string // semver constraint
	Path    string // local path (optional, for development)
}

// Validate checks that a DependencySpec does not have both Version and Path set.
func (d DependencySpec) Validate() error {
	if d.Version != "" && d.Path != "" {
		return fmt.Errorf("dependency cannot have both version %q and path %q", d.Version, d.Path)
	}
	return nil
}

// ParseManifest parses TOML-formatted data into a Manifest.
//
// Limitation: this is a minimal TOML subset parser. Inline comments (e.g.,
// key = "value" # comment) are not supported — the comment text would be
// included in the parsed value. Only full-line comments (lines starting with #)
// are recognized.
func ParseManifest(data []byte) (*Manifest, error) {
	m := &Manifest{
		Dependencies:     make(map[string]DependencySpec),
		RustDependencies: make(map[string]RustDependencySpec),
	}

	lines := strings.Split(string(data), "\n")
	section := ""

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip blanks and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(trimmed, "[") {
			if !strings.HasSuffix(trimmed, "]") {
				return nil, fmt.Errorf("line %d: unclosed section header: %s", lineNum, trimmed)
			}
			section = trimmed[1 : len(trimmed)-1]
			if section != "package" && section != "dependencies" && section != "rust_dependencies" {
				return nil, fmt.Errorf("line %d: unknown section: [%s]", lineNum, section)
			}
			continue
		}

		// Key = value
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			return nil, fmt.Errorf("line %d: expected key = value, got: %s", lineNum, trimmed)
		}

		key := strings.TrimSpace(trimmed[:eqIdx])
		value := strings.TrimSpace(trimmed[eqIdx+1:])

		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		switch section {
		case "package":
			str, err := parseString(value, lineNum)
			if err != nil {
				return nil, err
			}
			switch key {
			case "name":
				m.Package.Name = str
			case "version":
				m.Package.Version = str
			case "description":
				m.Package.Description = str
			default:
				return nil, fmt.Errorf("line %d: unknown package key: %s", lineNum, key)
			}

		case "dependencies":
			dep, err := parseDependencyValue(value, lineNum)
			if err != nil {
				return nil, err
			}
			// Validate is the single point of validation for dependency constraints
			// (e.g., rejecting both version and path).
			if err := dep.Validate(); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			m.Dependencies[key] = dep

		case "rust_dependencies":
			dep, err := parseRustDependencyValue(value, lineNum)
			if err != nil {
				return nil, err
			}
			m.RustDependencies[key] = dep

		default:
			return nil, fmt.Errorf("line %d: key-value pair outside of section: %s", lineNum, trimmed)
		}
	}

	return m, nil
}

// parseString extracts a quoted string value.
// Limitation: escape sequences (e.g., \") are not handled — this is a minimal
// TOML subset that only supports simple quoted strings without escapes.
func parseString(value string, lineNum int) (string, error) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("line %d: expected quoted string, got: %s", lineNum, value)
	}
	return value[1 : len(value)-1], nil
}

// parseDependencyValue parses either a quoted version string or an inline table.
func parseDependencyValue(value string, lineNum int) (DependencySpec, error) {
	// Simple string: "0.2.0"
	if len(value) >= 2 && value[0] == '"' {
		str, err := parseString(value, lineNum)
		if err != nil {
			return DependencySpec{}, err
		}
		return DependencySpec{Version: str}, nil
	}

	// Inline table: { key1 = "val1", key2 = "val2" }
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		dep := DependencySpec{}
		if inner == "" {
			return dep, nil
		}
		// Limitation: splitting on "," would break if quoted values contain commas.
		// This is acceptable for the minimal TOML subset we support.
		pairs := strings.Split(inner, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			pEq := strings.Index(pair, "=")
			if pEq < 0 {
				return DependencySpec{}, fmt.Errorf("line %d: invalid inline table entry: %s", lineNum, pair)
			}
			pKey := strings.TrimSpace(pair[:pEq])
			pVal := strings.TrimSpace(pair[pEq+1:])
			str, err := parseString(pVal, lineNum)
			if err != nil {
				return DependencySpec{}, fmt.Errorf("line %d: in inline table: %w", lineNum, err)
			}
			switch pKey {
			case "version":
				dep.Version = str
			case "path":
				dep.Path = str
			default:
				return DependencySpec{}, fmt.Errorf("line %d: unknown dependency key: %s", lineNum, pKey)
			}
		}
		return dep, nil
	}

	return DependencySpec{}, fmt.Errorf("line %d: expected quoted string or inline table, got: %s", lineNum, value)
}

// parseRustDependencyValue parses either a quoted version string or an inline
// table for the [rust_dependencies] section. Supported inline keys: version,
// features (an array of quoted strings).
func parseRustDependencyValue(value string, lineNum int) (RustDependencySpec, error) {
	// Simple string: "1.5"
	if len(value) >= 2 && value[0] == '"' {
		str, err := parseString(value, lineNum)
		if err != nil {
			return RustDependencySpec{}, err
		}
		return RustDependencySpec{Version: str}, nil
	}

	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		dep := RustDependencySpec{}
		if inner == "" {
			return dep, nil
		}
		// Split on top-level commas only; commas inside [...] are part of a list.
		pairs := splitTopLevelCommas(inner)
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			pEq := strings.Index(pair, "=")
			if pEq < 0 {
				return RustDependencySpec{}, fmt.Errorf("line %d: invalid inline table entry: %s", lineNum, pair)
			}
			pKey := strings.TrimSpace(pair[:pEq])
			pVal := strings.TrimSpace(pair[pEq+1:])
			switch pKey {
			case "version":
				str, err := parseString(pVal, lineNum)
				if err != nil {
					return RustDependencySpec{}, fmt.Errorf("line %d: rust_dependencies: %w", lineNum, err)
				}
				dep.Version = str
			case "path":
				str, err := parseString(pVal, lineNum)
				if err != nil {
					return RustDependencySpec{}, fmt.Errorf("line %d: rust_dependencies: %w", lineNum, err)
				}
				dep.Path = str
			case "features":
				feats, err := parseStringArray(pVal, lineNum)
				if err != nil {
					return RustDependencySpec{}, fmt.Errorf("line %d: rust_dependencies features: %w", lineNum, err)
				}
				dep.Features = feats
			default:
				return RustDependencySpec{}, fmt.Errorf("line %d: unknown rust_dependencies key: %s", lineNum, pKey)
			}
		}
		if dep.Version == "" && dep.Path == "" {
			return RustDependencySpec{}, fmt.Errorf("line %d: rust_dependencies entry must specify either a version or a path", lineNum)
		}
		if dep.Version != "" && dep.Path != "" {
			return RustDependencySpec{}, fmt.Errorf("line %d: rust_dependencies entry cannot have both version and path", lineNum)
		}
		return dep, nil
	}

	return RustDependencySpec{}, fmt.Errorf("line %d: expected quoted version or inline table, got: %s", lineNum, value)
}

// splitTopLevelCommas splits s on commas that are not inside [...] or "...".
// Used only for the rust_dependencies inline-table form because it can contain
// a features = [...] array.
func splitTopLevelCommas(s string) []string {
	var out []string
	depth := 0
	inStr := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inStr = !inStr
		case c == '[' && !inStr:
			depth++
		case c == ']' && !inStr:
			if depth > 0 {
				depth--
			}
		case c == ',' && depth == 0 && !inStr:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// parseStringArray parses a TOML inline array of quoted strings, e.g. `["a", "b"]`.
func parseStringArray(s string, lineNum int) ([]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("expected array literal, got: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		str, err := parseString(p, lineNum)
		if err != nil {
			return nil, err
		}
		out = append(out, str)
	}
	return out, nil
}

// LoadManifest loads and parses an intent.toml file from the given directory.
func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "intent.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return ParseManifest(data)
}

// validateName checks that a name contains only safe characters for use as
// a TOML bare key or section header: ASCII letters, digits, hyphens, and
// underscores. The name must also be non-empty.
func validateName(name, context string) error {
	if name == "" {
		return fmt.Errorf("%s name must not be empty", context)
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '-' && ch != '_' {
			return fmt.Errorf("%s name %q contains invalid character %q", context, name, ch)
		}
	}
	return nil
}

// WriteManifest writes a Manifest to the given file path in TOML format.
func WriteManifest(path string, m *Manifest) error {
	if err := validateName(m.Package.Name, "package"); err != nil {
		return err
	}
	for name := range m.Dependencies {
		if err := validateName(name, "dependency"); err != nil {
			return err
		}
	}

	var sb strings.Builder

	sb.WriteString("[package]\n")
	sb.WriteString(fmt.Sprintf("name = %q\n", m.Package.Name))
	sb.WriteString(fmt.Sprintf("version = %q\n", m.Package.Version))
	if m.Package.Description != "" {
		sb.WriteString(fmt.Sprintf("description = %q\n", m.Package.Description))
	}

	if len(m.Dependencies) > 0 {
		sb.WriteString("\n[dependencies]\n")
		// Sort dependency names for deterministic output.
		names := make([]string, 0, len(m.Dependencies))
		for name := range m.Dependencies {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			dep := m.Dependencies[name]
			if dep.Path != "" {
				sb.WriteString(fmt.Sprintf("%s = { path = %q }\n", name, dep.Path))
			} else {
				sb.WriteString(fmt.Sprintf("%s = %q\n", name, dep.Version))
			}
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}
