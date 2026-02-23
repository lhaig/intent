package rustbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
)

// generateFromSource runs the new pipeline (parse -> check -> lower -> rustbe)
// and returns the generated Rust source.
func generateFromSource(t *testing.T, name, src string) string {
	t.Helper()

	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("[%s] parse errors: %s", name, p.Diagnostics().Format("test"))
	}
	result := checker.CheckWithResult(prog)
	if result.Diagnostics.HasErrors() {
		t.Fatalf("[%s] check errors: %s", name, result.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, result)
	return Generate(mod)
}

func TestGenerateHello(t *testing.T) {
	src := `module hello version "1.0";
entry function main() returns Int {
    print("Hello, Intent!");
    return 0;
}
`
	output := generateFromSource(t, "hello", src)

	expects := []string{
		"fn __intent_main() -> i64",
		`println!("{}"`,
		`"Hello, Intent!".to_string()`,
		"fn main()",
		"std::process::exit",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateBankAccount(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "bank_account.intent"))
	if err != nil {
		t.Fatalf("failed to read bank_account.intent: %v", err)
	}
	output := generateFromSource(t, "bank_account", string(src))

	expects := []string{
		"struct BankAccount",
		"fn new(",
		"fn deposit(",
		"fn withdraw(",
		"fn get_balance(",
		"__check_invariants",
		"assert!(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateFibonacci(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "fibonacci.intent"))
	if err != nil {
		t.Fatalf("failed to read fibonacci.intent: %v", err)
	}
	output := generateFromSource(t, "fibonacci", string(src))

	expects := []string{
		"fn fib(",
		"i64",
		"assert!(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateArraySum(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "array_sum.intent"))
	if err != nil {
		t.Fatalf("failed to read array_sum.intent: %v", err)
	}
	output := generateFromSource(t, "array_sum", string(src))

	expects := []string{
		"Vec<i64>",
		"fn sum_array(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateEnumBasic(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "enum_basic.intent"))
	if err != nil {
		t.Fatalf("failed to read enum_basic.intent: %v", err)
	}
	output := generateFromSource(t, "enum_basic", string(src))

	expects := []string{
		"enum ",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateSortedCheck(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "sorted_check.intent"))
	if err != nil {
		t.Fatalf("failed to read sorted_check.intent: %v", err)
	}
	output := generateFromSource(t, "sorted_check", string(src))

	expects := []string{
		"fn check_sorted(",
		"Vec<i64>",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateShapeArea(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "shape_area.intent"))
	if err != nil {
		t.Fatalf("failed to read shape_area.intent: %v", err)
	}
	output := generateFromSource(t, "shape_area", string(src))

	expects := []string{
		"enum ",
		"f64",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateResultOption(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "result_option.intent"))
	if err != nil {
		t.Fatalf("failed to read result_option.intent: %v", err)
	}
	output := generateFromSource(t, "result_option", string(src))

	expects := []string{
		"Result<",
		"Option<",
		"Ok(",
		"Err(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateTryOperator(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "try_operator.intent"))
	if err != nil {
		t.Fatalf("failed to read try_operator.intent: %v", err)
	}
	output := generateFromSource(t, "try_operator", string(src))

	if !strings.Contains(output, "?") {
		t.Errorf("expected output to contain try operator '?', got:\n%s", output)
	}
}

func TestGenerateStringMethods(t *testing.T) {
	src := `module test version "1.0";

function test_len(s: String) returns Int {
    return s.len();
}

function test_to_lowercase(s: String) returns String {
    return s.to_lowercase();
}

function test_trim(s: String) returns String {
    return s.trim();
}

function test_starts_with(s: String) returns Bool {
    return s.starts_with("hello");
}

function test_contains(s: String) returns Bool {
    return s.contains("world");
}

function test_split(s: String) returns Array<String> {
    return s.split(",");
}

function test_chain(s: String) returns String {
    return s.trim().to_lowercase();
}
`
	output := generateFromSource(t, "string_methods", src)

	tests := []struct {
		name     string
		expected string
	}{
		{"len", "(s.len() as i64)"},
		{"to_lowercase", "s.to_lowercase()"},
		{"trim", "s.trim().to_string()"},
		{"starts_with", "s.starts_with("},
		{"contains", "s.contains("},
		{"split", "s.split("},
		{"split_collect", "collect::<Vec<String>>()"},
	}
	for _, tt := range tests {
		if !strings.Contains(output, tt.expected) {
			t.Errorf("[%s] expected output to contain %q, got:\n%s", tt.name, tt.expected, output)
		}
	}
}

// findExample locates an example file relative to the project root.
func findExample(t *testing.T, name string) string {
	t.Helper()
	// Walk up from the test file to find the project root
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(dir, "examples", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find examples/%s from %s", name, dir)
		}
		dir = parent
	}
}
