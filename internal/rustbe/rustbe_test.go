package rustbe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/codegen"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
)

// compareOutput runs both pipelines (old: AST->codegen, new: AST->check->lower->rustbe)
// on the given source and asserts identical output.
func compareOutput(t *testing.T, name, src string) {
	t.Helper()

	// Old pipeline: AST -> codegen
	p := parser.New(src)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		t.Fatalf("[%s] parse errors: %s", name, p.Diagnostics().Format("test"))
	}
	diag := checker.Check(prog)
	if diag.HasErrors() {
		t.Fatalf("[%s] check errors: %s", name, diag.Format("test"))
	}
	oldOutput := codegen.Generate(prog)

	// New pipeline: AST -> check with result -> lower -> rustbe
	p2 := parser.New(src)
	prog2 := p2.Parse()
	result := checker.CheckWithResult(prog2)
	if result.Diagnostics.HasErrors() {
		t.Fatalf("[%s] check errors (new): %s", name, result.Diagnostics.Format("test"))
	}
	mod := ir.Lower(prog2, result)
	newOutput := Generate(mod)

	if oldOutput != newOutput {
		// Find first difference for debugging
		oldLines := strings.Split(oldOutput, "\n")
		newLines := strings.Split(newOutput, "\n")
		for i := 0; i < len(oldLines) || i < len(newLines); i++ {
			var oldLine, newLine string
			if i < len(oldLines) {
				oldLine = oldLines[i]
			}
			if i < len(newLines) {
				newLine = newLines[i]
			}
			if oldLine != newLine {
				t.Errorf("[%s] output differs at line %d:\n  old: %q\n  new: %q", name, i+1, oldLine, newLine)
				break
			}
		}
		t.Logf("[%s] old output (%d bytes):\n%s", name, len(oldOutput), oldOutput)
		t.Logf("[%s] new output (%d bytes):\n%s", name, len(newOutput), newOutput)
	}
}

func TestCompareHello(t *testing.T) {
	src := `module hello version "1.0";
entry function main() returns Int {
    print("Hello, Intent!");
    return 0;
}
`
	compareOutput(t, "hello", src)
}

func TestCompareBankAccount(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "bank_account.intent"))
	if err != nil {
		t.Fatalf("failed to read bank_account.intent: %v", err)
	}
	compareOutput(t, "bank_account", string(src))
}

func TestCompareFibonacci(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "fibonacci.intent"))
	if err != nil {
		t.Fatalf("failed to read fibonacci.intent: %v", err)
	}
	compareOutput(t, "fibonacci", string(src))
}

func TestCompareArraySum(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "array_sum.intent"))
	if err != nil {
		t.Fatalf("failed to read array_sum.intent: %v", err)
	}
	compareOutput(t, "array_sum", string(src))
}

func TestCompareEnumBasic(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "enum_basic.intent"))
	if err != nil {
		t.Fatalf("failed to read enum_basic.intent: %v", err)
	}
	compareOutput(t, "enum_basic", string(src))
}

func TestCompareSortedCheck(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "sorted_check.intent"))
	if err != nil {
		t.Fatalf("failed to read sorted_check.intent: %v", err)
	}
	compareOutput(t, "sorted_check", string(src))
}

func TestCompareShapeArea(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "shape_area.intent"))
	if err != nil {
		t.Fatalf("failed to read shape_area.intent: %v", err)
	}
	compareOutput(t, "shape_area", string(src))
}

func TestCompareResultOption(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "result_option.intent"))
	if err != nil {
		t.Fatalf("failed to read result_option.intent: %v", err)
	}
	compareOutput(t, "result_option", string(src))
}

func TestCompareTryOperator(t *testing.T) {
	src, err := os.ReadFile(findExample(t, "try_operator.intent"))
	if err != nil {
		t.Fatalf("failed to read try_operator.intent: %v", err)
	}
	compareOutput(t, "try_operator", string(src))
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
