package rustbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
)

// generateFromSource runs the new pipeline (parse -> check -> lower -> rustbe)
// and returns the generated Rust source.
func generateFromSource(t *testing.T, name, src string) string {
	t.Helper()
	return generateFromSourceWithOpts(t, name, src, Options{})
}

func generateFromSourceWithOpts(t *testing.T, name, src string, opts Options) string {
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
	return Generate(mod, opts)
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
		{"len", "(s.chars().count() as i64)"},
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

func TestGenerateAsyncFunction(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
entry function main() returns Int {
    return 0;
}
`
	output := generateFromSource(t, "async_fn", src)

	// Async functions declared `returns Future<T>` in Intent should emit a
	// Rust signature with the inner type directly — `async fn` already wraps
	// the return in a Future. Emitting `-> JoinHandle<T>` here was the
	// Phase 14 mistake that broke compilation; the JoinHandle only appears
	// on the spawn *result* (i.e., at a `let h = spawn fn(...)` binding).
	expects := []string{
		"async fn fetchData() -> i64",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
	// Function signature must NOT advertise JoinHandle.
	if strings.Contains(output, "fetchData() -> tokio::task::JoinHandle") {
		t.Errorf("async fn signature must not return JoinHandle directly, got:\n%s", output)
	}
	// Non-async entry should remain sync
	if strings.Contains(output, "#[tokio::main]") {
		t.Errorf("non-async main should not have #[tokio::main], got:\n%s", output)
	}
}

func TestGenerateAsyncEntryFunction(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
async entry function main() returns Future<Int> {
    let f: Future<Int> = spawn fetchData();
    let val: Int = await f;
    return val;
}
`
	output := generateFromSource(t, "async_entry", src)

	expects := []string{
		"async fn __intent_main()",
		"#[tokio::main]",
		"async fn main()",
		"__intent_main().await",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateAwaitExpr(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
async entry function main() returns Future<Int> {
    let f: Future<Int> = spawn fetchData();
    let val: Int = await f;
    return val;
}
`
	output := generateFromSource(t, "await", src)

	// Spawn passes the async-fn call's Future directly to tokio::spawn —
	// no inner `async move` wrapper, no double-await. Await on the resulting
	// JoinHandle uses `.await.expect(...)`.
	expects := []string{
		".await",
		"tokio::spawn(fetchData())",
		".await.expect(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateAsyncBuiltins(t *testing.T) {
	src := `module test version "1.0";
async function work() returns Future<Int> {
    return 42;
}
async entry function main() returns Future<Int> {
    sleep(100);
    let futures: Array<Future<Int>> = [];
    let results: Array<Int> = await_all(futures);
    let f: Future<Int> = spawn work();
    let r: Result<Int, String> = timeout(f, 5000);
    return 0;
}
`
	output := generateFromSource(t, "async_builtins", src)

	expects := []string{
		"tokio::time::sleep(std::time::Duration::from_millis(",
		"futures::future::join_all(",
		"tokio::time::timeout(std::time::Duration::from_millis(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

// Regression for prds/done/phase-14-phase11-13-gaps.md item 14.4 (revised
// follow-up): async-fn signatures emit `-> T` so a direct call returns
// `impl Future<Output = T>`. `spawn` passes that future straight to
// tokio::spawn (no `async move {...}` wrapper) — wrapping would force a
// by-move capture of every argument in the enclosing scope, breaking
// ownership across multiple spawn sites. The spawn result is a
// JoinHandle<T> and `await` on a JoinHandle-bound variable unwraps via
// `.await.expect(...)`.
func TestGenerateAsyncSpawnAwaitJoinHandle(t *testing.T) {
	src := `module test version "1.0";

async function compute(x: Int) returns Future<Int> {
    return x + 1;
}

async entry function main() returns Future<Int> {
    let f: Future<Int> = spawn compute(5);
    let r: Int = await f;
    return r;
}
`
	output := generateFromSource(t, "spawn_await_join", src)

	// spawn passes the inner async-fn call directly.
	if !strings.Contains(output, "tokio::spawn(compute(5i64))") {
		t.Errorf("expected spawn to pass the future directly, got:\n%s", output)
	}
	// Must NOT wrap in an inner async move block (forces by-move capture).
	if strings.Contains(output, "tokio::spawn(async move {") {
		t.Errorf("spawn must not wrap the call in async move, got:\n%s", output)
	}

	// await on a JoinHandle must unwrap the JoinError.
	if !strings.Contains(output, "(f).await.expect(") {
		t.Errorf("expected await to unwrap JoinHandle with .expect(), got:\n%s", output)
	}
}

// Regression for prds/done/phase-14-phase11-13-gaps.md item 14.3: per ADR
// 0026, sleep(ms) returns Future<Void>. The source-level `await` adds the
// `.await`; the sleep builtin must NOT add its own `.await` or the emitted
// Rust contains `sleep(...).await.await` which does not compile.
func TestGenerateAsyncSleepNoDoubleAwait(t *testing.T) {
	src := `module test version "1.0";
async entry function main() returns Future<Int> {
    await sleep(100);
    return 0;
}
`
	output := generateFromSource(t, "sleep_no_double_await", src)

	if strings.Contains(output, ".await.await") {
		t.Errorf("emitted Rust must not contain double .await on sleep, got:\n%s", output)
	}
	// sleep emits the bare Sleep future; the source-level `await` adds the
	// `.await`. AwaitExpr's IsJoinHandle is false because the inner is a
	// CallExpr (not a SpawnExpr), so the emit is plain `.await` rather than
	// `.await.expect(...)`.
	if !strings.Contains(output, "tokio::time::sleep(std::time::Duration::from_millis(100i64 as u64))") {
		t.Errorf("expected bare tokio::time::sleep, got:\n%s", output)
	}
	if strings.Contains(output, "tokio::spawn(tokio::time::sleep(") {
		t.Errorf("sleep must not be wrapped in tokio::spawn (would change unwrap semantics), got:\n%s", output)
	}
}

// Phase 15 / ADR 0028: extern function calls emit the crate path directly,
// with no module prefix mangling and no body declaration. requires/ensures
// asserts surround the call as for normal functions.
func TestGenerateExternFunctionRustCall(t *testing.T) {
	src := `module test version "1.0";

extern function blake3_hash(input: String) returns String
    from "blake3_intent::hash_hex"
    requires len(input) > 0
    ensures len(result) == 64;

entry function main() returns Int {
    let h: String = blake3_hash("hello");
    return 0;
}
`
	output := generateFromSource(t, "extern_blake3", src)

	// The Rust path is used verbatim at the call site.
	if !strings.Contains(output, "blake3_intent::hash_hex(") {
		t.Errorf("expected extern call to use the rust path 'blake3_intent::hash_hex', got:\n%s", output)
	}
	// No `fn blake3_hash` declaration should be emitted — the crate provides it.
	if strings.Contains(output, "fn blake3_hash(") {
		t.Errorf("extern declaration must not emit a local Rust fn, got:\n%s", output)
	}
}

func TestGenerateAsyncWithContracts(t *testing.T) {
	src := `module test version "1.0";
async function fetchPositive(x: Int) returns Future<Int>
    requires x > 0
{
    return x;
}
entry function main() returns Int {
    return 0;
}
`
	output := generateFromSource(t, "async_contracts", src)

	expects := []string{
		"async fn fetchPositive(",
		"assert!(",
		"Precondition failed",
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

func makeProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("test"))
	}
	return prog
}

func TestGenerateCrossPackageFunctionCall(t *testing.T) {
	// Package module: types_pkg/types.intent
	typesSrc := `module types version "1.0.0";

public function distance(x: Int, y: Int) returns Int {
    return x + y;
}
`
	// Main module imports via package name
	mainSrc := `module main version "1.0.0";

import types_pkg;

entry function main() returns Int {
    let d: Int = types_pkg.distance(3, 4);
    return d;
}
`
	packageDirs := map[string]string{
		"types_pkg": "/project/libs/types_pkg",
	}

	registry := map[string]*ast.Program{
		"/project/libs/types_pkg/types.intent": makeProgram(t, typesSrc),
		"/project/main.intent":                 makeProgram(t, mainSrc),
	}
	sortedPaths := []string{
		"/project/libs/types_pkg/types.intent",
		"/project/main.intent",
	}

	checkResult := checker.CheckAll(registry, sortedPaths, packageDirs)
	if checkResult.Diagnostics.HasErrors() {
		t.Fatalf("check errors: %s", checkResult.Diagnostics.Format("test"))
	}

	prog := ir.LowerAll(registry, sortedPaths, checkResult, packageDirs)
	output := GenerateAll(prog, Options{})

	// The function should be called as types_distance (using module name "types"),
	// not types_pkg_distance (using package name "types_pkg").
	if !strings.Contains(output, "types_distance(") {
		t.Errorf("expected cross-package call to use module name prefix 'types_distance(', got:\n%s", output)
	}
	if strings.Contains(output, "types_pkg_distance(") {
		t.Errorf("cross-package call should NOT use package name prefix 'types_pkg_distance(', got:\n%s", output)
	}
}

// TestEntryFunctionInImportedModuleNoDuplicateMain guards against a multi-file
// build emitting two `fn main`/`__intent_main` when an imported (non-entry)
// module declares an `entry` function (e.g. selfhost/formatter/parser.intent's
// standalone entry stub). The imported entry function must be demoted to an
// ordinary prefixed function; only the entry module produces the program main.
func TestEntryFunctionInImportedModuleNoDuplicateMain(t *testing.T) {
	prog := &ir.Program{
		Modules: []*ir.Module{
			{
				Name:    "helper",
				IsEntry: false,
				Functions: []*ir.Function{
					{
						Name:       "main",
						IsEntry:    true, // imported module's standalone entry stub
						ReturnType: &checker.Type{Name: "Int"},
						Body: []ir.Stmt{
							&ir.ReturnStmt{Value: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}}},
						},
					},
				},
			},
			{
				Name:    "main",
				IsEntry: true,
				Functions: []*ir.Function{
					{
						Name:       "__intent_main",
						IsEntry:    true,
						ReturnType: &checker.Type{Name: "Int"},
						Body: []ir.Stmt{
							&ir.ReturnStmt{Value: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}}},
						},
					},
				},
			},
		},
	}

	output := GenerateAll(prog, Options{})

	if got := strings.Count(output, "fn main()"); got != 1 {
		t.Errorf("expected exactly one `fn main()`, got %d:\n%s", got, output)
	}
	if got := strings.Count(output, "fn __intent_main()"); got != 1 {
		t.Errorf("expected exactly one `fn __intent_main()`, got %d:\n%s", got, output)
	}
	// The imported module's entry function must be demoted to a prefixed fn.
	if !strings.Contains(output, "helper_main(") {
		t.Errorf("expected imported entry function demoted to `helper_main`, got:\n%s", output)
	}
}

func TestGenerateModuleManglingNoCollision(t *testing.T) {
	// Collision scenario: package named "math" contains module "math"
	// (math/math.intent), AND a separate package named "strings_pkg"
	// contains module "strings" whose PackageName is "math" — this would
	// overwrite the "math" module-name entry in the old code.
	//
	// We use a simpler reproducer: module named "alpha" from package "alpha_pkg",
	// and module named "beta" from package "alpha" (package name == other module name).
	// The package-name entry for "alpha" must not overwrite the module-name entry
	// for the "alpha" module.

	alphaSrc := `module alpha version "1.0.0";

public function greet() returns String {
    return "hello from alpha";
}
`
	betaSrc := `module beta version "1.0.0";

public function farewell() returns String {
    return "goodbye from beta";
}
`
	mainSrc := `module main version "1.0.0";

import alpha_pkg;
import alpha;

entry function main() returns Int {
    let g: String = alpha_pkg.greet();
    let f: String = alpha.farewell();
    return 0;
}
`
	packageDirs := map[string]string{
		"alpha_pkg": "/project/libs/alpha_pkg",
		"alpha":     "/project/libs/alpha",
	}

	registry := map[string]*ast.Program{
		"/project/libs/alpha_pkg/alpha.intent": makeProgram(t, alphaSrc),
		"/project/libs/alpha/beta.intent":      makeProgram(t, betaSrc),
		"/project/main.intent":                 makeProgram(t, mainSrc),
	}
	sortedPaths := []string{
		"/project/libs/alpha_pkg/alpha.intent",
		"/project/libs/alpha/beta.intent",
		"/project/main.intent",
	}

	checkResult := checker.CheckAll(registry, sortedPaths, packageDirs)
	if checkResult.Diagnostics.HasErrors() {
		t.Fatalf("check errors: %s", checkResult.Diagnostics.Format("test"))
	}

	prog := ir.LowerAll(registry, sortedPaths, checkResult, packageDirs)
	output := GenerateAll(prog, Options{})

	// alpha_pkg.greet() should resolve to alpha_greet (module name "alpha")
	if !strings.Contains(output, "alpha_greet(") {
		t.Errorf("expected alpha_pkg.greet() to resolve to alpha_greet(, got:\n%s", output)
	}
	// alpha.farewell() should resolve to beta_farewell (module name "beta"),
	// NOT alpha_farewell. The package "alpha" maps to module "beta".
	// However, the module-name entry for "alpha" (from the alpha module) must
	// not be overwritten by the package-name entry for "alpha" (from beta's package).
	if !strings.Contains(output, "beta_farewell(") {
		t.Errorf("expected alpha.farewell() to resolve to beta_farewell(, got:\n%s", output)
	}
}

// Phase 16 / ADR 0029: in-language testing framework — Rust backend tests.

func TestGenerateSimpleTest(t *testing.T) {
	src := `module hello version "1.0";

test "addition works" {
    let x: Int = 1 + 1;
    assert(x == 2);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "simple-test", src)
	want := []string{
		"#[test]",
		"fn __test_addition_works()",
		`assert!((x == 2i64), "assertion failed")`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestGenerateAsyncTest(t *testing.T) {
	src := `module hello version "1.0";

async function delayed() returns Future<Int> { return 1; }

async test "awaits" {
    let f: Future<Int> = spawn delayed();
    let r: Int = await f;
    assert(r == 1);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "async-test", src)
	if !strings.Contains(out, "#[tokio::test]") {
		t.Errorf("expected #[tokio::test] attribute, got:\n%s", out)
	}
	if !strings.Contains(out, "async fn __test_awaits()") {
		t.Errorf("expected 'async fn __test_awaits()', got:\n%s", out)
	}
}

func TestGenerateAssertEq(t *testing.T) {
	src := `module hello version "1.0";

test "eq" {
    assert_eq(2 + 2, 4);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-eq", src)
	if !strings.Contains(out, "assert_eq!") {
		t.Errorf("expected assert_eq! macro, got:\n%s", out)
	}
}

func TestGenerateAssertClose(t *testing.T) {
	src := `module hello version "1.0";

test "close" {
    assert_close(1.0, 1.0, 0.001);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-close", src)
	if !strings.Contains(out, ".abs() <=") {
		t.Errorf("expected .abs() <= emission, got:\n%s", out)
	}
}

func TestGenerateAssertPanics(t *testing.T) {
	src := `module hello version "1.0";

test "panics" {
    let bomb: Fn() -> Void = || -> Void => assert(false);
    assert_panics(bomb);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-panics", src)
	if !strings.Contains(out, "std::panic::catch_unwind") {
		t.Errorf("expected catch_unwind emission, got:\n%s", out)
	}
}

func TestGenerateEntityAssertEqUsesEqMethod(t *testing.T) {
	src := `module hello version "1.0";

entity Point {
    field x: Int;
    field y: Int;

    constructor(xi: Int, yi: Int) {
        self.x = xi;
        self.y = yi;
    }

    method eq(other: Point) returns Bool {
        return self.x == other.x and self.y == other.y;
    }
}

test "entity eq" {
    let p1: Point = Point(1, 2);
    let p2: Point = Point(1, 2);
    assert_eq(p1, p2);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "entity-eq", src)
	if !strings.Contains(out, ".eq(&") {
		t.Errorf("expected entity.eq(&other) emission, got:\n%s", out)
	}
	if strings.Contains(out, "assert_eq!(p1") {
		t.Errorf("entity comparison should NOT use Rust's assert_eq! macro, got:\n%s", out)
	}
}

func TestSanitiseTestName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abs returns non-negative", "abs_returns_non_negative"},
		{"  leading whitespace", "leading_whitespace"},
		{"trailing  ", "trailing"},
		{"123 starts with number", "_123_starts_with_number"},
		{"!!!", "unnamed"},
		{"", "unnamed"},
		{"MIXED Case", "mixed_case"},
		{"a__b___c", "a_b_c"},
	}
	for _, tc := range cases {
		got := sanitiseTestName(tc.in)
		if got != tc.want {
			t.Errorf("sanitiseTestName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Phase 22 / ADR 0033: --strip-contracts swaps `assert!` for `debug_assert!`
// on contract checks while leaving user-written test-body asserts alone.

func TestStripContractsSwapsToDebugAssert(t *testing.T) {
	src := `module bank version "1.0";

entity Account {
    field balance: Int;
    invariant self.balance >= 0;
    constructor(initial: Int)
        requires initial >= 0
        ensures self.balance == initial
    {
        self.balance = initial;
    }
    method deposit(amount: Int) returns Void
        requires amount > 0
        ensures self.balance == old(self.balance) + amount
    {
        self.balance = self.balance + amount;
    }
}

entry function main() returns Int {
    let a: Account = Account(10);
    a.deposit(5);
    return 0;
}

test "deposit increases balance" {
    let a: Account = Account(10);
    a.deposit(5);
    assert_eq(a.balance, 15);
}
`

	plain := generateFromSourceWithOpts(t, "strip", src, Options{})
	stripped := generateFromSourceWithOpts(t, "strip", src, Options{StripContracts: true})

	// Contract sites: plain has assert! for them, stripped has debug_assert!.
	contractMarkers := []string{
		"Precondition failed: initial >= 0",
		"Postcondition failed: self . balance == initial",
		"Invariant failed: self . balance >= 0",
		"Precondition failed: amount > 0",
		"Postcondition failed: self . balance == old ( self . balance ) + amount",
	}
	for _, marker := range contractMarkers {
		if !strings.Contains(plain, "assert!") || !strings.Contains(plain, marker) {
			t.Errorf("plain output missing contract assert! with marker %q", marker)
		}
		if strings.Contains(stripped, "assert!("+strings.Split(marker, ":")[0]) {
			// Looking for `assert!(` not `debug_assert!(` for the contract;
			// the message includes the marker.
		}
	}

	// Specifically: stripped has zero `assert!(<cond>, "Precondition`/`Postcondition`/`Invariant` lines
	// (those have been swapped to debug_assert!).
	for _, marker := range []string{"Precondition failed", "Postcondition failed", "Invariant failed"} {
		// Find lines containing the marker; each must be a debug_assert!.
		for _, line := range strings.Split(stripped, "\n") {
			if !strings.Contains(line, marker) {
				continue
			}
			if !strings.Contains(line, "debug_assert!") {
				t.Errorf("line with %q is not a debug_assert! in stripped output: %s", marker, line)
			}
			if strings.Contains(line, " assert!") {
				t.Errorf("line with %q still contains assert! in stripped output: %s", marker, line)
			}
		}
	}

	// User-written assert_eq in the test body must remain assert_eq! in both modes.
	if !strings.Contains(plain, "assert_eq!") {
		t.Error("plain output missing test-body assert_eq!")
	}
	if !strings.Contains(stripped, "assert_eq!") {
		t.Error("stripped output should still emit test-body assert_eq! (assertion API, not a contract)")
	}
	// And the test-body assert should NOT have been turned into debug_assert!.
	for _, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "debug_assert_eq!") {
			t.Errorf("test-body assert_eq was incorrectly stripped: %s", line)
		}
	}
}

func TestStripContractsPreservesNonContractOutput(t *testing.T) {
	// Regression: with zero options, output must be byte-identical to the
	// pre-Phase-22 behaviour. We can't compare to a hard-coded reference
	// (the generator evolves), so we compare two runs of Options{} against
	// each other and verify the contract assert! sites are still present.
	src := `module fib version "1.0";
function fib(n: Int) returns Int
    requires n >= 0
    ensures result >= 0
{
    if n < 2 { return n; }
    return fib(n - 1) + fib(n - 2);
}
entry function main() returns Int {
    return fib(5);
}
`
	plain1 := generateFromSourceWithOpts(t, "fib", src, Options{})
	plain2 := generateFromSourceWithOpts(t, "fib", src, Options{})
	if plain1 != plain2 {
		t.Error("Generate is not deterministic across two calls with Options{}")
	}
	if !strings.Contains(plain1, `assert!((n >= 0i64), "Precondition failed`) {
		t.Errorf("plain output missing expected contract assert! pattern:\n%s", plain1)
	}
	if strings.Contains(plain1, "debug_assert!") {
		t.Error("plain output unexpectedly contains debug_assert! (StripContracts not set)")
	}
}

// Regression: when a function takes `Array<T>` (compiled to `&Vec<T>`) and the
// call site passes a struct field, the previous emitter dropped the leading
// `&`, producing `obj.field.clone()` (owned Vec) where `&Vec` was expected.
// Surfaced by stage2 formatter (Phase 38) passing `EnumVariant.params` /
// `TraitMethodSig.params` / `ExternDecl.params` to a shared helper. Fix:
// extend the array-ref coercion to cover FieldAccessExpr and IndexExpr in
// addition to bare VarRef.
func TestArrayParamFieldAccessCallBorrows(t *testing.T) {
	src := `module m version "1.0";

entity Box {
    field xs: Array<Int>;

    constructor(xs: Array<Int>) {
        self.xs = xs;
    }
}

function sum(xs: Array<Int>) returns Int {
    let mutable s: Int = 0;
    let mutable i: Int = 0;
    while i < len(xs) {
        s = s + xs[i];
        i = i + 1;
    }
    return s;
}

entry function main() returns Int {
    let mutable xs: Array<Int> = [];
    xs.push(1);
    xs.push(2);
    let b: Box = Box(xs);
    return sum(b.xs);
}
`
	output := generateFromSource(t, "array_param_field", src)

	// The call site for sum(b.xs) must emit a borrow, not an owned clone.
	// Acceptable: `sum(&b.xs)` (no clone needed — function only reads it).
	// Unacceptable: `sum(b.xs.clone())` (owned Vec where &Vec expected).
	if strings.Contains(output, "sum(b.xs.clone())") {
		t.Errorf("regression: field-access array arg emitted as owned clone:\n%s", output)
	}
	if !strings.Contains(output, "sum(&b.xs)") {
		t.Errorf("expected borrowed field-access array arg `sum(&b.xs)`, got:\n%s", output)
	}
}

// Regression: builtin I/O calls (read_file, write_file, create_dir,
// file_exists, env_get) passing a String taken from an indexed array
// element used to emit `read_to_string(files[i])` — which moves out of
// the Vec and fails with E0507. cloneIfNeeded is now applied to the
// builtin arg.
func TestBuiltinIOClonesIndexedStringArg(t *testing.T) {
	src := `module m version "1.0";

entry function main() returns Int {
    let mutable paths: Array<String> = [];
    paths.push("/tmp/a.txt");
    paths.push("/tmp/b.txt");
    let r: Result<String, String> = read_file(paths[0]);
    return 0;
}
`
	output := generateFromSource(t, "builtin_io_clone", src)

	if !strings.Contains(output, "paths[0i64 as usize].clone()") {
		t.Errorf("expected read_file arg to be cloned out of Vec index, got:\n%s", output)
	}
	if strings.Contains(output, "read_to_string(paths[0i64 as usize])") {
		t.Errorf("regression: read_file arg moved out of indexed array without clone:\n%s", output)
	}
}

// Regression: `let x: Array<T> = self.field;` used to move out of self.
// Surfaced by Phase 40A (ADR 0044) — drain_comments needed to extract the
// pending_comments field into a local, swap in an empty array, and return
// the captured. cloneIfNeeded was applied for IndexExpr but not for
// FieldAccessExpr.
func TestLetBindingClonesFieldAccessOfNonCopyType(t *testing.T) {
	src := `module m version "1.0";

entity Holder {
    field xs: Array<Int>;

    constructor(xs: Array<Int>) {
        self.xs = xs;
    }

    method snapshot() returns Array<Int> {
        let snap: Array<Int> = self.xs;
        return snap;
    }
}

entry function main() returns Int {
    let mutable xs: Array<Int> = [];
    xs.push(1);
    let h: Holder = Holder(xs);
    return 0;
}
`
	output := generateFromSource(t, "let_field_clone", src)

	// The let binding must clone the field; otherwise rustc rejects with
	// "cannot move out of `self.xs` which is behind a mutable reference."
	if !strings.Contains(output, "let snap: Vec<i64> = self.xs.clone();") {
		t.Errorf("expected `let snap: Vec<i64> = self.xs.clone();`, got:\n%s", output)
	}
}

// Array/Map params are always emitted as `&Vec`/`&HashMap`, so an owned
// temporary passed as such an argument — here a call result — must be borrowed
// at the call site, not just place expressions. Previously only VarRef/field/
// index args were borrowed, so `takes(names())` emitted `Vec` where `&Vec` was
// expected (E0308).
func TestGenerateBorrowsCallResultArrayArg(t *testing.T) {
	src := `module hello version "1.0";

function names() returns Array<String> {
    let xs: Array<String> = [];
    return xs;
}

function takes(xs: Array<String>) returns Int {
    return len(xs);
}

entry function main() returns Int {
    return takes(names());
}
`
	out := generateFromSource(t, "borrow-call-result", src)
	if !strings.Contains(out, "takes(&names())") {
		t.Errorf("expected call-result Array arg to be borrowed as `takes(&names())`, got:\n%s", out)
	}
	if strings.Contains(out, "takes(names())") {
		t.Errorf("call-result Array arg was passed un-borrowed (E0308), got:\n%s", out)
	}
}
