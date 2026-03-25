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
	Package      PackageInfo
	Dependencies map[string]DependencySpec
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
		Dependencies: make(map[string]DependencySpec),
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
			if section != "package" && section != "dependencies" {
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
