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

func TestGenerateMapType(t *testing.T) {
	src := `module test version "1.0";
function test_map() returns Int {
    let mutable m: Map<String, Int> = [];
    m.set("alice", 100);
    let v: Int = m.get("alice", 0);
    let has: Bool = m.contains("alice");
    let k: Array<String> = m.keys();
    m.remove("alice");
    return len(m);
}
`
	output := generateFromSource(t, "map", src)

	expects := []string{
		"use std::collections::HashMap;",
		"HashMap::new()",
		".insert(",
		".get(&",
		".cloned().unwrap_or(",
		".contains_key(&",
		".keys().cloned().collect::<Vec<_>>()",
		".remove(&",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateMapDemo(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "map_demo.intent"))
	if err != nil {
		t.Fatalf("failed to read map_demo.intent: %v", err)
	}
	output := generateFromSource(t, "map_demo", string(src))

	expects := []string{
		"use std::collections::HashMap;",
		"struct Config",
		"HashMap<String, String>",
		"HashMap<String, i64>",
		"HashMap::new()",
		".insert(",
		".get(&",
		".cloned().unwrap_or(",
		".contains_key(&",
		".keys().cloned().collect::<Vec<_>>()",
		".remove(&",
		"(scores.len() as i64)",
		"fn get_setting(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateErrorHandling(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "error_handling.intent"))
	if err != nil {
		t.Fatalf("failed to read error_handling.intent: %v", err)
	}
	output := generateFromSource(t, "error_handling", string(src))

	expects := []string{
		"Result<",
		"Option<",
		"Ok(",
		"Err(",
		"Some(",
		"None",
		"match ",
		"?",
		"break",
		"continue",
		"for ",
		"while ",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateHttpBuiltins(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let resp: Result<String, String> = http_post("https://api.example.com", "headers", "body");
    let resp2: Result<String, String> = http_get("https://api.example.com", "headers");
    let val: Option<String> = json_get("json", "key");
    emit_event("test", "payload");
    let ts: Int = timestamp_ms();
    return 0;
}
`
	output := generateFromSource(t, "test", src)

	expects := []string{
		"__intent_http_post",
		"__intent_http_get",
		"serde_json::from_str",
		"eprintln!(\"[EVENT]",
		"SystemTime",
		"UNIX_EPOCH",
		"use reqwest;",
		"use serde_json;",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateJsonPath(t *testing.T) {
	src := `module test version "1.0";
entry function main() returns Int {
    let val: Option<String> = json_path("data", "a.0.b");
    return 0;
}
`
	output := generateFromSource(t, "json_path", src)
	if !strings.Contains(output, "split('.')") {
		t.Errorf("Expected json_path to contain path split, got:\n%s", output)
	}
	if !strings.Contains(output, "serde_json") {
		t.Errorf("Expected json_path to use serde_json, got:\n%s", output)
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

func TestGenerateTraitDecl(t *testing.T) {
	src := `module test version "1.0";
trait Handler {
    method execute(x: Int) returns Int;
}
entry function main() returns Int { return 0; }
`
	output := generateFromSource(t, "trait", src)
	for _, want := range []string{"trait Handler", "fn execute(&mut self", "i64"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateImplBlock(t *testing.T) {
	src := `module test version "1.0";
entity Foo { field x: Int; constructor(v: Int) { self.x = v; } }
trait Handler { method execute(n: Int) returns Int; }
impl Handler for Foo { method execute(n: Int) returns Int { return self.x + n; } }
entry function main() returns Int { let f: Foo = Foo(5); return f.execute(10); }
`
	output := generateFromSource(t, "impl", src)
	for _, want := range []string{"impl Handler for Foo", "fn execute(&mut self"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateHandlerTraitExample(t *testing.T) {
	path := findExample(t, "handler_trait.intent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	output := generateFromSource(t, "handler_trait", string(data))
	for _, want := range []string{"trait Handler", "impl Handler for StartHandler", "impl Handler for StopHandler"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateIOBuiltins(t *testing.T) {
	src := `module test version "1.0";

function test_read(path: String) returns Result<String, String> {
    return read_file(path);
}

function test_write(path: String, content: String) returns Result<Void, String> {
    return write_file(path, content);
}

function test_mkdir(path: String) returns Result<Void, String> {
    return create_dir(path);
}

function test_exists(path: String) returns Bool {
    return file_exists(path);
}

function test_env(name: String) returns Option<String> {
    return env_get(name);
}

function test_to_str(n: Int) returns String {
    return n.to_string();
}
`
	output := generateFromSource(t, "io_builtins", src)

	expects := []string{
		"std::fs::read_to_string(",
		"std::fs::write(",
		"std::fs::create_dir_all(",
		"std::path::Path::new(&",
		"std::env::var(",
		".to_string()",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateLambdaExpression(t *testing.T) {
	src := `module test version "1.0";

function apply(f: Fn(Int) -> Int, x: Int) returns Int {
    return f(x);
}

function test() returns Int {
    let double: Fn(Int) -> Int = |n: Int| -> Int => n * 2;
    return apply(double, 5);
}`

	output := generateFromSource(t, "lambda", src)

	expects := []string{
		"impl Fn(i64) -> i64",
		"|n: i64| -> i64 {",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateLambdaMultipleParams(t *testing.T) {
	src := `module test version "1.0";

function apply2(f: Fn(Int, Int) -> Int, x: Int, y: Int) returns Int {
    return f(x, y);
}

function test() returns Int {
    let add: Fn(Int, Int) -> Int = |x: Int, y: Int| -> Int => x + y;
    return apply2(add, 3, 4);
}`

	output := generateFromSource(t, "lambda_multi", src)

	expects := []string{
		"impl Fn(i64, i64) -> i64",
		"|x: i64, y: i64| -> i64 {",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateFnTypeParam(t *testing.T) {
	src := `module test version "1.0";

function call_with_ten(f: Fn(Int) -> Int) returns Int {
    return f(10);
}`

	output := generateFromSource(t, "fn_param", src)

	if !strings.Contains(output, "impl Fn(i64) -> i64") {
		t.Errorf("expected Fn type to map to impl Fn(i64) -> i64, got:\n%s", output)
	}
}
