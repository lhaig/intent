package jsbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
	"github.com/lhaig/intent/internal/parser"
)

func TestGenerateHello(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*ir.Function{
			{
				Name:    "__intent_main",
				IsEntry: true,
				Body: []ir.Stmt{
					&ir.ExprStmt{
						Expr: &ir.CallExpr{
							Function: "print",
							Args: []ir.Expr{
								&ir.StringLit{
									Value: "\"Hello, World!\"",
									Type:  &checker.Type{Name: "String"},
								},
							},
							Kind: ir.CallBuiltin,
							Type: &checker.Type{Name: "Void"},
						},
					},
					&ir.ReturnStmt{
						Value: &ir.IntLit{
							Value: 0,
							Type:  &checker.Type{Name: "Int"},
						},
					},
				},
				ReturnType: &checker.Type{Name: "Int"},
			},
		},
	}

	result := Generate(mod, Options{})

	// Check for essential components
	if !strings.Contains(result, "function __intent_main()") {
		t.Errorf("Expected function __intent_main, got:\n%s", result)
	}
	if !strings.Contains(result, "console.log") {
		t.Errorf("Expected console.log for print, got:\n%s", result)
	}
	if !strings.Contains(result, "return 0") {
		t.Errorf("Expected return 0, got:\n%s", result)
	}
	if !strings.Contains(result, "process.exit(__exitCode)") {
		t.Errorf("Expected process.exit call, got:\n%s", result)
	}
}

func TestGenerateFunction(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name: "add",
				Params: []*ir.Param{
					{Name: "a", Type: &checker.Type{Name: "Int"}},
					{Name: "b", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "a", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.PLUS,
							Right: &ir.VarRef{Name: "b", Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Int"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "function add(a, b)") {
		t.Errorf("Expected function add(a, b), got:\n%s", result)
	}
	if !strings.Contains(result, "@param {number} a") {
		t.Errorf("Expected JSDoc @param for a, got:\n%s", result)
	}
	if !strings.Contains(result, "@param {number} b") {
		t.Errorf("Expected JSDoc @param for b, got:\n%s", result)
	}
	if !strings.Contains(result, "@returns {number}") {
		t.Errorf("Expected JSDoc @returns, got:\n%s", result)
	}
	if !strings.Contains(result, "return (a + b)") {
		t.Errorf("Expected return (a + b), got:\n%s", result)
	}
}

func TestGenerateEntity(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Entities: []*ir.Entity{
			{
				Name: "Counter",
				Fields: []*ir.Field{
					{Name: "value", Type: &checker.Type{Name: "Int"}},
				},
				Constructor: &ir.Constructor{
					Params: []*ir.Param{
						{Name: "initial", Type: &checker.Type{Name: "Int"}},
					},
					Body: []ir.Stmt{
						&ir.AssignStmt{
							Target: &ir.FieldAccessExpr{
								Object: &ir.SelfRef{Type: &checker.Type{Name: "Counter"}},
								Field:  "value",
								Type:   &checker.Type{Name: "Int"},
							},
							Value: &ir.VarRef{Name: "initial", Type: &checker.Type{Name: "Int"}},
						},
					},
				},
				Methods: []*ir.Method{
					{
						Name:       "increment",
						ReturnType: &checker.Type{Name: "Void"},
						Body: []ir.Stmt{
							&ir.AssignStmt{
								Target: &ir.FieldAccessExpr{
									Object: &ir.SelfRef{Type: &checker.Type{Name: "Counter"}},
									Field:  "value",
									Type:   &checker.Type{Name: "Int"},
								},
								Value: &ir.BinaryExpr{
									Left: &ir.FieldAccessExpr{
										Object: &ir.SelfRef{Type: &checker.Type{Name: "Counter"}},
										Field:  "value",
										Type:   &checker.Type{Name: "Int"},
									},
									Op: lexer.PLUS,
									Right: &ir.IntLit{
										Value: 1,
										Type:  &checker.Type{Name: "Int"},
									},
									Type: &checker.Type{Name: "Int"},
								},
							},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "class Counter") {
		t.Errorf("Expected class Counter, got:\n%s", result)
	}
	if !strings.Contains(result, "constructor(initial)") {
		t.Errorf("Expected constructor(initial), got:\n%s", result)
	}
	if !strings.Contains(result, "this.value = 0") {
		t.Errorf("Expected field initialization, got:\n%s", result)
	}
	if !strings.Contains(result, "this.value = initial") {
		t.Errorf("Expected assignment in constructor, got:\n%s", result)
	}
	if !strings.Contains(result, "increment()") {
		t.Errorf("Expected increment method, got:\n%s", result)
	}
}

func TestGenerateEnum(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Enums: []*ir.Enum{
			{
				Name: "Color",
				Variants: []*ir.EnumVariant{
					{Name: "Red", Fields: nil},
					{Name: "Green", Fields: nil},
					{
						Name: "RGB",
						Fields: []*ir.Field{
							{Name: "r", Type: &checker.Type{Name: "Int"}},
							{Name: "g", Type: &checker.Type{Name: "Int"}},
							{Name: "b", Type: &checker.Type{Name: "Int"}},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "const Color = {") {
		t.Errorf("Expected const Color, got:\n%s", result)
	}
	if !strings.Contains(result, "Red: () => ({ _tag: \"Red\" })") {
		t.Errorf("Expected Red unit variant, got:\n%s", result)
	}
	if !strings.Contains(result, "Green: () => ({ _tag: \"Green\" })") {
		t.Errorf("Expected Green unit variant, got:\n%s", result)
	}
	if !strings.Contains(result, "RGB: (r, g, b) => ({ _tag: \"RGB\", r, g, b })") {
		t.Errorf("Expected RGB data variant, got:\n%s", result)
	}
}

func TestGenerateContracts(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name: "divide",
				Params: []*ir.Param{
					{Name: "a", Type: &checker.Type{Name: "Int"}},
					{Name: "b", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Int"},
				Requires: []*ir.Contract{
					{
						Expr: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "b", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.NEQ,
							Right: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						RawText: "b != 0",
					},
				},
				Ensures: []*ir.Contract{
					{
						Expr: &ir.BinaryExpr{
							Left:  &ir.ResultRef{Type: &checker.Type{Name: "Int"}},
							Op:    lexer.LT,
							Right: &ir.VarRef{Name: "a", Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						RawText: "result < a",
					},
				},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "a", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.SLASH,
							Right: &ir.VarRef{Name: "b", Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Int"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "if (!((b !== 0))) throw new Error(\"Precondition failed: b != 0\")") {
		t.Errorf("Expected precondition check, got:\n%s", result)
	}
	if !strings.Contains(result, "if (!((__result < a))) throw new Error(\"Postcondition failed: result < a\")") {
		t.Errorf("Expected postcondition check, got:\n%s", result)
	}
}

func TestGenerateAll(t *testing.T) {
	prog := &ir.Program{
		Modules: []*ir.Module{
			{
				Name:    "helper",
				IsEntry: false,
				Functions: []*ir.Function{
					{
						Name: "square",
						Params: []*ir.Param{
							{Name: "x", Type: &checker.Type{Name: "Int"}},
						},
						ReturnType: &checker.Type{Name: "Int"},
						Body: []ir.Stmt{
							&ir.ReturnStmt{
								Value: &ir.BinaryExpr{
									Left:  &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
									Op:    lexer.STAR,
									Right: &ir.VarRef{Name: "x", Type: &checker.Type{Name: "Int"}},
									Type:  &checker.Type{Name: "Int"},
								},
							},
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
							&ir.ReturnStmt{
								Value: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							},
						},
					},
				},
			},
		},
	}

	result := GenerateAll(prog, Options{})

	if !strings.Contains(result, "function helper_square(x)") {
		t.Errorf("Expected mangled function helper_square, got:\n%s", result)
	}
	if !strings.Contains(result, "function __intent_main()") {
		t.Errorf("Expected entry function __intent_main, got:\n%s", result)
	}
	if !strings.Contains(result, "process.exit(__exitCode)") {
		t.Errorf("Expected entry point invocation, got:\n%s", result)
	}
}

func TestGenerateStringInterp(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: true,
		Functions: []*ir.Function{
			{
				Name:    "__intent_main",
				IsEntry: true,
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "msg",
						Type: checker.TypeString,
						Value: &ir.StringInterp{
							Parts: []ir.StringInterpPart{
								{IsExpr: false, Static: "Hello "},
								{IsExpr: true, Expr: &ir.VarRef{Name: "name", Type: checker.TypeString}},
								{IsExpr: false, Static: ", age "},
								{IsExpr: true, Expr: &ir.VarRef{Name: "age", Type: checker.TypeInt}},
							},
							Type: checker.TypeString,
						},
					},
				},
				ReturnType: checker.TypeInt,
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "${name}") {
		t.Errorf("Expected ${name} in template literal, got:\n%s", result)
	}
	if !strings.Contains(result, "${age}") {
		t.Errorf("Expected ${age} in template literal, got:\n%s", result)
	}
	if !strings.Contains(result, "`Hello ") {
		t.Errorf("Expected backtick template literal, got:\n%s", result)
	}
}

func generateJSFromSource(t *testing.T, name, src string) string {
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
	return Generate(mod, Options{})
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
`
	output := generateJSFromSource(t, "string_methods", src)

	tests := []struct {
		name     string
		expected string
	}{
		{"len", "BigInt(Array.from(s).length)"},
		{"to_lowercase", "s.toLowerCase()"},
		{"trim", "s.trim()"},
		{"starts_with", "s.startsWith("},
		{"contains", "s.includes("},
		{"split", "s.split("},
	}
	for _, tt := range tests {
		if !strings.Contains(output, tt.expected) {
			t.Errorf("[%s] expected output to contain %q, got:\n%s", tt.name, tt.expected, output)
		}
	}
}

func TestGenerateMapDemoJS(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "map_demo.intent"))
	if err != nil {
		t.Fatalf("failed to read map_demo.intent: %v", err)
	}
	output := generateJSFromSource(t, "map_demo", string(src))

	expects := []string{
		"class Config",
		"new Map()",
		".set(",
		".has(",
		".get(",
		".delete(",
		"Array.from(",
		".keys())",
		".size)",
		"get_setting(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateMapTypeJS(t *testing.T) {
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
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("test"))
	}
	result := checker.CheckWithResult(prog)
	if result.Diagnostics.HasErrors() {
		t.Fatalf("check errors: %s", result.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, result)
	output := Generate(mod, Options{})

	expects := []string{
		"new Map()",
		".set(",
		".has(",
		".get(",
		".delete(",
		"Array.from(",
		".keys())",
		".size)",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateErrorHandlingJS(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "error_handling.intent"))
	if err != nil {
		t.Fatalf("failed to read error_handling.intent: %v", err)
	}
	output := generateJSFromSource(t, "error_handling", string(src))

	expects := []string{
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

// findExample locates an example file relative to the project root.
func findExample(t *testing.T, name string) string {
	t.Helper()
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

func TestGenerateHttpBuiltinsJS(t *testing.T) {
	src := `module test version "1.0";

function test_http_post(url: String, headers: String, body: String) returns Result<String, String> {
    return http_post(url, headers, body);
}

function test_http_get(url: String, headers: String) returns Result<String, String> {
    return http_get(url, headers);
}

function test_json_get(json: String, key: String) returns Option<String> {
    return json_get(json, key);
}

function test_emit_event(event_type: String, payload: String) returns Void {
    emit_event(event_type, payload);
}

function test_timestamp() returns Int {
    return 0;
}
`
	output := generateJSFromSource(t, "http_builtins", src)

	expects := []struct {
		name    string
		pattern string
	}{
		{"http_post execSync", "execSync"},
		{"http_post curl POST", "curl"},
		{"http_get curl", "curl"},
		{"json_get JSON.parse", "JSON.parse"},
		{"emit_event console.error", "console.error"},
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp.pattern) {
			t.Errorf("[%s] expected output to contain %q, got:\n%s", exp.name, exp.pattern, output)
		}
	}
}

func TestGenerateJsonPathJS(t *testing.T) {
	src := `module test version "1.0";

function test_json_path(json: String, path: String) returns Option<String> {
    return json_path(json, path);
}
`
	output := generateJSFromSource(t, "json_path", src)
	if !strings.Contains(output, "split('.')") {
		t.Errorf("Expected json_path to contain path split, got:\n%s", output)
	}
	if !strings.Contains(output, "reduce") {
		t.Errorf("Expected json_path to contain reduce, got:\n%s", output)
	}
}

func TestGenerateTimestampBuiltinJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "get_time",
				ReturnType: &checker.Type{Name: "Int"},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.CallExpr{
							Function: "timestamp_ms",
							Args:     []ir.Expr{},
							Kind:     ir.CallBuiltin,
							Type:     &checker.Type{Name: "Int"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "Date.now()") {
		t.Errorf("expected Date.now() for timestamp_ms, got:\n%s", result)
	}
}

// generateFromSource runs the full pipeline (parse -> check -> lower -> jsbe)
// and returns the generated JS source.
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
	return Generate(mod, Options{})
}

func TestGenerateTraitJS(t *testing.T) {
	src := `module test version "1.0";
trait Handler {
    method execute(x: Int) returns Int;
}
entry function main() returns Int { return 0; }
`
	output := generateFromSource(t, "trait", src)
	for _, want := range []string{"@interface Handler", "@method execute"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateImplBlockJS(t *testing.T) {
	src := `module test version "1.0";
entity Foo { field x: Int; constructor(v: Int) { self.x = v; } }
trait Handler { method execute(n: Int) returns Int; }
impl Handler for Foo { method execute(n: Int) returns Int { return self.x + n; } }
entry function main() returns Int { let f: Foo = Foo(5); return f.execute(10); }
`
	output := generateFromSource(t, "impl", src)
	for _, want := range []string{"impl Handler for Foo", "Foo.prototype.execute"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateHandlerTraitExampleJS(t *testing.T) {
	path := findExample(t, "handler_trait.intent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	output := generateFromSource(t, "handler_trait", string(data))
	for _, want := range []string{"@interface Handler", "StartHandler.prototype.execute", "StopHandler.prototype.execute"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestGenerateIOBuiltinsJS(t *testing.T) {
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
	output := generateJSFromSource(t, "io_builtins", src)

	expects := []string{
		"readFileSync(",
		"writeFileSync(",
		"mkdirSync(",
		"existsSync(",
		"process.env",
		"String(",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateLambdaExpressionJS(t *testing.T) {
	src := `module test version "1.0";

function apply(f: Fn(Int) -> Int, x: Int) returns Int {
    return f(x);
}

function test() returns Int {
    let double: Fn(Int) -> Int = |n: Int| -> Int => n * 2;
    return apply(double, 5);
}`

	output := generateJSFromSource(t, "lambda", src)

	expects := []string{
		"(n) => { return",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateLambdaMultipleParamsJS(t *testing.T) {
	src := `module test version "1.0";

function apply2(f: Fn(Int, Int) -> Int, x: Int, y: Int) returns Int {
    return f(x, y);
}

function test() returns Int {
    let add: Fn(Int, Int) -> Int = |x: Int, y: Int| -> Int => x + y;
    return apply2(add, 3, 4);
}`

	output := generateJSFromSource(t, "lambda_multi", src)

	expects := []string{
		"(x, y) => { return",
	}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestGenerateInlineLambdaCallJS(t *testing.T) {
	src := `module test version "1.0";

function apply(f: Fn(Int) -> Int, x: Int) returns Int {
    return f(x);
}

function test() returns Int {
    return apply(|n: Int| -> Int => n * 3, 7);
}`

	output := generateJSFromSource(t, "lambda_inline", src)

	if !strings.Contains(output, "(n) => { return") {
		t.Errorf("expected lambda to generate arrow function syntax, got:\n%s", output)
	}
}

func TestGenerateAsyncFunctionJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:    "fetchData",
				IsAsync: true,
				Params: []*ir.Param{
					{Name: "url", Type: &checker.Type{Name: "String"}},
				},
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.IntLit{Value: 42, Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "async function fetchData(url)") {
		t.Errorf("expected async function declaration, got:\n%s", result)
	}
}

func TestGenerateAwaitExprJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "process",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "val",
						Type: &checker.Type{Name: "Int"},
						Value: &ir.AwaitExpr{
							Expr: &ir.VarRef{Name: "future", Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}}},
							Type: &checker.Type{Name: "Int"},
						},
					},
					&ir.ReturnStmt{
						Value: &ir.VarRef{Name: "val", Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "await future") {
		t.Errorf("expected await expression, got:\n%s", result)
	}
}

func TestGenerateSpawnExprJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "run",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "handle",
						Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
						Value: &ir.SpawnExpr{
							Expr: &ir.CallExpr{
								Function: "compute",
								Args: []ir.Expr{
									&ir.IntLit{Value: 42, Type: &checker.Type{Name: "Int"}},
								},
								Kind: ir.CallFunction,
								Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
							},
							Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "(async () => { return compute(42); })()") {
		t.Errorf("expected spawn as IIFE, got:\n%s", result)
	}
}

func TestGenerateAwaitAllJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "gather",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Array", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}}}},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "results",
						Type: &checker.Type{Name: "Array", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
						Value: &ir.CallExpr{
							Function: "await_all",
							Args: []ir.Expr{
								&ir.VarRef{Name: "futures", Type: &checker.Type{Name: "Array", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}}}}},
							},
							Kind: ir.CallFunction,
							Type: &checker.Type{Name: "Array", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "await Promise.all(futures)") {
		t.Errorf("expected await Promise.all, got:\n%s", result)
	}
}

func TestGenerateAwaitAnyJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "race",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "first",
						Type: &checker.Type{Name: "Int"},
						Value: &ir.CallExpr{
							Function: "await_any",
							Args: []ir.Expr{
								&ir.VarRef{Name: "futures", Type: &checker.Type{Name: "Array", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}}}}},
							},
							Kind: ir.CallFunction,
							Type: &checker.Type{Name: "Int"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "await Promise.race(futures)") {
		t.Errorf("expected await Promise.race, got:\n%s", result)
	}
}

func TestGenerateTimeoutJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "withTimeout",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Result"}}},
				Body: []ir.Stmt{
					&ir.LetStmt{
						Name: "result",
						Type: &checker.Type{Name: "Result"},
						Value: &ir.CallExpr{
							Function: "timeout",
							Args: []ir.Expr{
								&ir.VarRef{Name: "future", Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}}},
								&ir.IntLit{Value: 5000, Type: &checker.Type{Name: "Int"}},
							},
							Kind: ir.CallFunction,
							Type: &checker.Type{Name: "Result"},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "Promise.race") {
		t.Errorf("expected Promise.race for timeout, got:\n%s", result)
	}
	if !strings.Contains(result, "setTimeout") {
		t.Errorf("expected setTimeout for timeout, got:\n%s", result)
	}
	if !strings.Contains(result, "5000") {
		t.Errorf("expected timeout ms value, got:\n%s", result)
	}
}

func TestGenerateSleepJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:       "delayed",
				IsAsync:    true,
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Void"}}},
				Body: []ir.Stmt{
					&ir.ExprStmt{
						Expr: &ir.CallExpr{
							Function: "sleep",
							Args: []ir.Expr{
								&ir.IntLit{Value: 1000, Type: &checker.Type{Name: "Int"}},
							},
							Kind: ir.CallFunction,
							Type: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Void"}}},
						},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "new Promise(resolve => setTimeout(resolve, 1000))") {
		t.Errorf("expected Promise-based sleep, got:\n%s", result)
	}
}

func TestGenerateAsyncFunctionWithContractsJS(t *testing.T) {
	mod := &ir.Module{
		Name:    "test",
		IsEntry: false,
		Functions: []*ir.Function{
			{
				Name:    "fetchPositive",
				IsAsync: true,
				Params: []*ir.Param{
					{Name: "id", Type: &checker.Type{Name: "Int"}},
				},
				ReturnType: &checker.Type{Name: "Future", IsGeneric: true, TypeParams: []*checker.Type{{Name: "Int"}}},
				Requires: []*ir.Contract{
					{
						Expr: &ir.BinaryExpr{
							Left:  &ir.VarRef{Name: "id", Type: &checker.Type{Name: "Int"}},
							Op:    lexer.GT,
							Right: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						RawText: "id > 0",
					},
				},
				Ensures: []*ir.Contract{
					{
						Expr: &ir.BinaryExpr{
							Left:  &ir.ResultRef{Type: &checker.Type{Name: "Int"}},
							Op:    lexer.GEQ,
							Right: &ir.IntLit{Value: 0, Type: &checker.Type{Name: "Int"}},
							Type:  &checker.Type{Name: "Bool"},
						},
						RawText: "result >= 0",
					},
				},
				Body: []ir.Stmt{
					&ir.ReturnStmt{
						Value: &ir.VarRef{Name: "id", Type: &checker.Type{Name: "Int"}},
					},
				},
			},
		},
	}

	result := Generate(mod, Options{})

	if !strings.Contains(result, "async function fetchPositive(id)") {
		t.Errorf("expected async function with contracts, got:\n%s", result)
	}
	if !strings.Contains(result, "Precondition failed: id > 0") {
		t.Errorf("expected precondition check in async function, got:\n%s", result)
	}
	if !strings.Contains(result, "Postcondition failed: result >= 0") {
		t.Errorf("expected postcondition check in async function, got:\n%s", result)
	}
}

func TestGenerateAsyncFullPipelineJS(t *testing.T) {
	src := `module test version "1.0";
async function fetchData() returns Future<Int> {
    return 42;
}
async function main() returns Future<Int> {
    let handle: Future<Int> = spawn fetchData();
    let val: Int = await handle;
    return val;
}
`
	output := generateFromSource(t, "async_pipeline", src)

	if !strings.Contains(output, "async function fetchData()") {
		t.Errorf("expected async function fetchData, got:\n%s", output)
	}
	if !strings.Contains(output, "async function main()") {
		t.Errorf("expected async function main, got:\n%s", output)
	}
	if !strings.Contains(output, "await handle") {
		t.Errorf("expected await expression, got:\n%s", output)
	}
	if !strings.Contains(output, "(async () => { return fetchData(); })()") {
		t.Errorf("expected spawn as IIFE, got:\n%s", output)
	}
}

// Regression for ops/plans/phase-14-phase11-13-gaps.md item 14.2: when a
// function has an `ensures` clause and the body uses an explicit `return X;`,
// the previous JS codegen placed the postcondition checks AFTER the return,
// making them dead code. The labeled-block pattern (mirroring Rust's 'body)
// captures the return value in __result and breaks out, then runs ensures.
func TestGenerateEnsuresLabeledBlockJS(t *testing.T) {
	src := `module result_block version "1.0";

function inc(x: Int) returns Int
    ensures result == x + 1
{
    return x + 1;
}

entry function main() returns Int {
    return inc(1);
}
`
	output := generateFromSource(t, "result_block", src)

	if !strings.Contains(output, "__body: {") {
		t.Errorf("expected labeled __body block around result-capturing function body, got:\n%s", output)
	}
	if !strings.Contains(output, "__result = (x + 1);") {
		t.Errorf("expected return rewritten to __result assignment, got:\n%s", output)
	}
	if !strings.Contains(output, "break __body;") {
		t.Errorf("expected break __body; to exit the labeled block, got:\n%s", output)
	}
	if !strings.Contains(output, "Postcondition failed: result == x + 1") {
		t.Errorf("expected postcondition check after the body block, got:\n%s", output)
	}
	// Ensure the postcondition check appears AFTER the body block, not skipped.
	bodyIdx := strings.Index(output, "__body: {")
	postIdx := strings.Index(output, "Postcondition failed: result == x + 1")
	if bodyIdx < 0 || postIdx < 0 || postIdx < bodyIdx {
		t.Errorf("postcondition check must appear after labeled block, got:\n%s", output)
	}
}

// Regression for ops/plans/phase-14-phase11-13-gaps.md item 14.1: when the
// entry function is async, the generated JS must mark __intent_main as async
// and await it at the top-level invocation; otherwise `await` inside the body
// raises SyntaxError at runtime and the returned Promise is never resolved.
func TestGenerateAsyncEntryFunctionJS(t *testing.T) {
	src := `module async_entry version "1.0";

async function delayed_add(a: Int, b: Int) returns Int
    ensures result == a + b
{
    await sleep(0);
    return a + b;
}

async entry function main() returns Int {
    let f: Future<Int> = spawn delayed_add(1, 2);
    let r: Int = await f;
    return r;
}
`
	output := generateFromSource(t, "async_entry", src)

	if !strings.Contains(output, "async function __intent_main()") {
		t.Errorf("expected async function __intent_main(), got:\n%s", output)
	}
	if !strings.Contains(output, "__intent_main().then(") {
		t.Errorf("expected __intent_main().then(...) top-level invocation, got:\n%s", output)
	}
	// Make sure we did not regress: non-async entry uses the sync form
	syncSrc := `module sync_entry version "1.0";

entry function main() returns Int {
    return 0;
}
`
	syncOutput := generateFromSource(t, "sync_entry", syncSrc)
	if !strings.Contains(syncOutput, "function __intent_main()") {
		t.Errorf("expected sync function __intent_main(), got:\n%s", syncOutput)
	}
	if strings.Contains(syncOutput, "async function __intent_main") {
		t.Errorf("non-async entry must not be marked async, got:\n%s", syncOutput)
	}
	if !strings.Contains(syncOutput, "const __exitCode = __intent_main();") {
		t.Errorf("expected sync invocation, got:\n%s", syncOutput)
	}
}

func makeJSProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse errors: %s", p.Diagnostics().Format("test"))
	}
	return prog
}

// Regression for ops/plans/phase-14-phase11-13-gaps.md item 14.6: when a
// package alias differs from the module name, both the entity definition and
// the constructor/function call site must agree on the mangled name.
func TestGenerateCrossPackageJSNameMangling(t *testing.T) {
	typesSrc := `module types version "1.0.0";

public entity Point {
    field x: Float;
    field y: Float;

    constructor(x: Float, y: Float)
        ensures self.x == x
        ensures self.y == y
    {
        self.x = x;
        self.y = y;
    }
}

public function distance_squared(a: Point, b: Point) returns Float
    ensures result >= 0.0
{
    return 0.0;
}
`
	mainSrc := `module main version "1.0.0";

import types_pkg;

entry function main() returns Int {
    let p1: Point = types_pkg.Point(0.0, 0.0);
    let p2: Point = types_pkg.Point(3.0, 4.0);
    let d: Float = types_pkg.distance_squared(p1, p2);
    return 0;
}
`
	packageDirs := map[string]string{
		"types_pkg": "/project/libs/types_pkg",
	}
	registry := map[string]*ast.Program{
		"/project/libs/types_pkg/types.intent": makeJSProgram(t, typesSrc),
		"/project/main.intent":                 makeJSProgram(t, mainSrc),
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

	// Entity defined as TypesPoint (module-name mangling). Call site must use
	// the same prefix, NOT package-name mangling like Types_pkgPoint.
	if !strings.Contains(output, "class TypesPoint") {
		t.Errorf("expected entity class TypesPoint, got:\n%s", output)
	}
	if !strings.Contains(output, "new TypesPoint(") {
		t.Errorf("expected constructor call new TypesPoint(, got:\n%s", output)
	}
	if strings.Contains(output, "Types_pkgPoint") {
		t.Errorf("constructor call must not use package-name prefix Types_pkgPoint, got:\n%s", output)
	}

	// Function defined as types_distance_squared (module-name prefix). Call
	// site must agree.
	if !strings.Contains(output, "function types_distance_squared(") {
		t.Errorf("expected function types_distance_squared, got:\n%s", output)
	}
	if !strings.Contains(output, "types_distance_squared(p1, p2)") {
		t.Errorf("expected call types_distance_squared(p1, p2), got:\n%s", output)
	}
	if strings.Contains(output, "types_pkg_distance_squared") {
		t.Errorf("function call must not use package-name prefix types_pkg_distance_squared, got:\n%s", output)
	}
}

// Phase 16 / ADR 0029: in-language testing framework — JS backend tests.

func TestJSGenerateSimpleTest(t *testing.T) {
	src := `module hello version "1.0";

test "addition works" {
    let x: Int = 1 + 1;
    assert(x == 2);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "simple-test", src)
	want := []string{
		"function __test_addition_works()",
		`if (!((x === 2))) throw new Error("assertion failed")`,
		"const __intent_tests = [",
		`{ name: "addition works", isAsync: false, fn: __test_addition_works }`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestJSGenerateAsyncTest(t *testing.T) {
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
	if !strings.Contains(out, "async function __test_awaits()") {
		t.Errorf("expected 'async function __test_awaits()', got:\n%s", out)
	}
	if !strings.Contains(out, `isAsync: true`) {
		t.Errorf("registry entry should mark isAsync true, got:\n%s", out)
	}
}

func TestJSGenerateAssertEq(t *testing.T) {
	src := `module hello version "1.0";

test "eq" {
    assert_eq(2 + 2, 4);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-eq", src)
	if !strings.Contains(out, "JSON.stringify") && !strings.Contains(out, "__a === __b") {
		t.Errorf("expected assert_eq emission, got:\n%s", out)
	}
}

func TestJSGenerateAssertClose(t *testing.T) {
	src := `module hello version "1.0";

test "close" {
    assert_close(1.0, 1.0, 0.001);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-close", src)
	if !strings.Contains(out, "Math.abs") {
		t.Errorf("expected Math.abs emission, got:\n%s", out)
	}
}

func TestJSGenerateAssertPanics(t *testing.T) {
	src := `module hello version "1.0";

test "panics" {
    let bomb: Fn() -> Void = || -> Void => assert(false);
    assert_panics(bomb);
}

entry function main() returns Int { return 0; }
`
	out := generateFromSource(t, "assert-panics", src)
	if !strings.Contains(out, "try {") || !strings.Contains(out, "__threw") {
		t.Errorf("expected try/catch panic emission, got:\n%s", out)
	}
}

func TestJSGenerateEntityAssertEqUsesEqMethod(t *testing.T) {
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
	if !strings.Contains(out, ".eq(") {
		t.Errorf("expected entity.eq() dispatch, got:\n%s", out)
	}
}

func TestJSSanitiseTestName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abs returns non-negative", "abs_returns_non_negative"},
		{"", "unnamed"},
		{"123 start", "_123_start"},
		{"!!!", "unnamed"},
	}
	for _, tc := range cases {
		got := sanitiseTestName(tc.in)
		if got != tc.want {
			t.Errorf("sanitiseTestName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Phase 22 / ADR 0033: --strip-contracts drops contract throw lines.

func TestStripContractsDropsContractThrows(t *testing.T) {
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
`
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse: %s", p.Diagnostics().Format("test"))
	}
	res := checker.CheckWithResult(prog)
	if res.Diagnostics.HasErrors() {
		t.Fatalf("check: %s", res.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, res)

	plain := Generate(mod, Options{})
	stripped := Generate(mod, Options{StripContracts: true})

	// Plain output must contain the contract throws.
	for _, marker := range []string{
		"throw new Error(\"Precondition failed:",
		"throw new Error(\"Postcondition failed:",
		"throw new Error(\"Invariant failed:",
	} {
		if !strings.Contains(plain, marker) {
			t.Errorf("plain output missing expected marker %q", marker)
		}
	}

	// Stripped output must contain none of them.
	for _, marker := range []string{
		"Precondition failed:",
		"Postcondition failed:",
		"Invariant failed:",
	} {
		if strings.Contains(stripped, marker) {
			t.Errorf("stripped output unexpectedly contains marker %q", marker)
		}
	}
}

func TestStripContractsPreservesNonContractJS(t *testing.T) {
	// Regression: Options{} produces identical output across two calls,
	// and that output contains the expected contract throws (proving
	// stripping isn't accidentally on by default).
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
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("parse: %s", p.Diagnostics().Format("test"))
	}
	res := checker.CheckWithResult(prog)
	if res.Diagnostics.HasErrors() {
		t.Fatalf("check: %s", res.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog, res)

	a := Generate(mod, Options{})
	b := Generate(mod, Options{})
	if a != b {
		t.Error("Generate is not deterministic across two calls with Options{}")
	}
	if !strings.Contains(a, "throw new Error(\"Precondition failed:") {
		t.Errorf("plain output missing expected contract throw:\n%s", a)
	}
}
