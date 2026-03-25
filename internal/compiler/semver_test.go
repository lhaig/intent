package compiler

import (
	"testing"
)

func TestSemverParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    Version
		wantErr bool
	}{
		{"1.2.3", Version{1, 2, 3}, false},
		{"0.0.0", Version{0, 0, 0}, false},
		{"10.20.30", Version{10, 20, 30}, false},
		{"1.0.0", Version{1, 0, 0}, false},
		{" 1.2.3 ", Version{1, 2, 3}, false}, // trimmed
		{"1.2", Version{}, true},
		{"1.2.3.4", Version{}, true},
		{"abc", Version{}, true},
		{"1.2.x", Version{}, true},
		{"-1.0.0", Version{}, true},
		{"", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSemverString(t *testing.T) {
	v := Version{1, 2, 3}
	if got := v.String(); got != "1.2.3" {
		t.Errorf("Version.String() = %q, want %q", got, "1.2.3")
	}
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b Version
		want int
	}{
		{Version{1, 0, 0}, Version{1, 0, 0}, 0},
		{Version{1, 0, 0}, Version{2, 0, 0}, -1},
		{Version{2, 0, 0}, Version{1, 0, 0}, 1},
		{Version{1, 1, 0}, Version{1, 2, 0}, -1},
		{Version{1, 2, 0}, Version{1, 1, 0}, 1},
		{Version{1, 0, 1}, Version{1, 0, 2}, -1},
		{Version{1, 0, 2}, Version{1, 0, 1}, 1},
		{Version{0, 0, 0}, Version{0, 0, 1}, -1},
	}

	for _, tt := range tests {
		t.Run(tt.a.String()+"_vs_"+tt.b.String(), func(t *testing.T) {
			if got := tt.a.Compare(tt.b); got != tt.want {
				t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSemverParseConstraint(t *testing.T) {
	tests := []struct {
		input   string
		wantOp  string
		wantVer Version
		wantErr bool
	}{
		{"1.0.0", "=", Version{1, 0, 0}, false},
		{"=1.0.0", "=", Version{1, 0, 0}, false},
		{"^1.0.0", "^", Version{1, 0, 0}, false},
		{"~1.0.0", "~", Version{1, 0, 0}, false},
		{">=1.0.0", ">=", Version{1, 0, 0}, false},
		{"<2.0.0", "<", Version{2, 0, 0}, false},
		{" ^1.2.3 ", "^", Version{1, 2, 3}, false},
		{">1.0.0", "", Version{}, true},
		{"<=1.0.0", "", Version{}, true},
		{"", "", Version{}, true},
		{"^invalid", "", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseConstraint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConstraint(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Op != tt.wantOp {
					t.Errorf("ParseConstraint(%q).Op = %q, want %q", tt.input, got.Op, tt.wantOp)
				}
				if got.Version != tt.wantVer {
					t.Errorf("ParseConstraint(%q).Version = %v, want %v", tt.input, got.Version, tt.wantVer)
				}
			}
		})
	}
}

func TestSemverConstraintMatches(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		// Exact match
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"=1.0.0", "1.0.0", true},
		{"=1.0.0", "2.0.0", false},

		// Caret (^) - compatible: >=version AND <next major
		{"^1.0.0", "1.0.0", true},
		{"^1.0.0", "1.5.3", true},
		{"^1.0.0", "1.99.99", true},
		{"^1.0.0", "2.0.0", false},
		{"^1.0.0", "0.9.9", false},
		{"^0.1.0", "0.1.0", true},
		{"^0.1.0", "0.1.5", true},
		{"^0.1.0", "0.2.0", false}, // 0.x: caret locks minor
		{"^0.1.0", "1.0.0", false},
		{"^0.0.1", "0.0.1", true},
		{"^0.0.1", "0.0.2", false}, // 0.0.x: caret locks patch
		{"^0.0.1", "0.1.0", false},

		// Tilde (~) - patch only: >=version AND <next minor
		{"~1.0.0", "1.0.0", true},
		{"~1.0.0", "1.0.9", true},
		{"~1.0.0", "1.1.0", false},
		{"~1.0.0", "0.9.9", false},
		{"~1.2.0", "1.2.5", true},
		{"~1.2.0", "1.3.0", false},

		// Greater than or equal (>=)
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "2.0.0", true},
		{">=1.0.0", "0.9.9", false},

		// Less than (<)
		{"<2.0.0", "1.9.9", true},
		{"<2.0.0", "2.0.0", false},
		{"<2.0.0", "2.0.1", false},
		{"<1.0.0", "0.9.9", true},
	}

	for _, tt := range tests {
		t.Run(tt.constraint+"_"+tt.version, func(t *testing.T) {
			c, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error: %v", tt.constraint, err)
			}
			v, err := ParseVersion(tt.version)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error: %v", tt.version, err)
			}
			if got := c.Matches(v); got != tt.want {
				t.Errorf("Constraint(%q).Matches(%q) = %v, want %v", tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

func TestSemverValidateNoConflicts(t *testing.T) {
	t.Run("no conflicts - compatible constraints", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, "^1.0.0"),
				mustParseConstraint(t, ">=1.2.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no conflicts - single constraint", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {mustParseConstraint(t, "^1.0.0")},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("conflict - incompatible constraints", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, "^1.0.0"),
				mustParseConstraint(t, "^2.0.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err == nil {
			t.Error("expected conflict error, got nil")
		}
	})

	t.Run("conflict - exact vs range", func(t *testing.T) {
		deps := map[string][]Constraint{
			"bar": {
				mustParseConstraint(t, "=1.0.0"),
				mustParseConstraint(t, ">=2.0.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err == nil {
			t.Error("expected conflict error, got nil")
		}
	})

	t.Run("no conflicts - tilde and caret overlap", func(t *testing.T) {
		deps := map[string][]Constraint{
			"baz": {
				mustParseConstraint(t, "^1.0.0"),
				mustParseConstraint(t, "~1.2.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("conflict - less than and greater equal no overlap", func(t *testing.T) {
		deps := map[string][]Constraint{
			"qux": {
				mustParseConstraint(t, ">=2.0.0"),
				mustParseConstraint(t, "<1.0.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err == nil {
			t.Error("expected conflict error, got nil")
		}
	})

	t.Run("no conflicts - wide range with >= and <", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, ">=1.0.0"),
				mustParseConstraint(t, "<1.15.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no conflicts - wide range >=1.0.0 <1.100.0", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, ">=1.0.0"),
				mustParseConstraint(t, "<1.100.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no conflicts - caret with >= lower bound", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, "^1.0.0"),
				mustParseConstraint(t, ">=1.5.0"),
				mustParseConstraint(t, "<1.15.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("conflict - >= above < bound", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {
				mustParseConstraint(t, ">=2.0.0"),
				mustParseConstraint(t, "<1.15.0"),
			},
		}
		if err := ValidateNoConflicts(deps); err == nil {
			t.Error("expected conflict error, got nil")
		}
	})

	t.Run("multiple packages independent", func(t *testing.T) {
		deps := map[string][]Constraint{
			"foo": {mustParseConstraint(t, "^1.0.0")},
			"bar": {mustParseConstraint(t, "^2.0.0")},
		}
		if err := ValidateNoConflicts(deps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSemverParseConstraintUnsupportedOperators(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{">1.0.0", `unsupported constraint operator ">"`},
		{"<=1.0.0", `unsupported constraint operator "<="`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseConstraint(tt.input)
			if err == nil {
				t.Fatalf("ParseConstraint(%q) expected error, got nil", tt.input)
			}
			if err.Error() != tt.want {
				t.Errorf("ParseConstraint(%q) error = %q, want %q", tt.input, err.Error(), tt.want)
			}
		})
	}
}

func TestConstraintBaseVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"1.0.0", "1.0.0", false},
		{"=1.0.0", "1.0.0", false},
		{"^1.0.0", "1.0.0", false},
		{"~1.2.3", "1.2.3", false},
		{">=2.0.0", "2.0.0", false},
		{"<3.0.0", "3.0.0", false},
		{"^0.1.0", "0.1.0", false},
		{"", "", true},
		{"^abc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ConstraintBaseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConstraintBaseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ConstraintBaseVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func mustParseConstraint(t *testing.T, s string) Constraint {
	t.Helper()
	c, err := ParseConstraint(s)
	if err != nil {
		t.Fatalf("mustParseConstraint(%q): %v", s, err)
	}
	return c
}
