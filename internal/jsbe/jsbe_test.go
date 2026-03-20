package jsbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	result := Generate(mod)

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

	result := Generate(mod)

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

	result := Generate(mod)

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

	result := Generate(mod)

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

	result := Generate(mod)

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

	result := GenerateAll(prog)

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

	result := Generate(mod)

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
	return Generate(mod)
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
		{"len", "BigInt(s.length)"},
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
	output := Generate(mod)

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

	result := Generate(mod)

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
	return Generate(mod)
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
