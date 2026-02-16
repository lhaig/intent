package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/rustbe"
	"github.com/lhaig/intent/internal/testgen"
	"github.com/lhaig/intent/internal/verify"
)

// Result holds the output of a compilation
type Result struct {
	Diagnostics *diagnostic.Diagnostics
	RustSource  string
	BinaryPath  string
}


// Compile runs the full pipeline: parse -> check -> lower -> rustbe
// Returns the result without writing files or invoking cargo.
func Compile(source string) *Result {
	res := &Result{}

	// Parse
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		res.Diagnostics = p.Diagnostics()
		return res
	}

	// Type check with result (needed for IR lowering)
	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Lower to IR, then generate Rust
	mod := ir.Lower(prog, checkResult)
	res.RustSource = rustbe.Generate(mod)

	return res
}

// Check runs parse + check only (no codegen).
func Check(source string) *diagnostic.Diagnostics {
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		return p.Diagnostics()
	}

	return checker.Check(prog)
}

// EmitRust runs the full pipeline and writes the Rust source to outPath.
func EmitRust(source, outPath string) error {
	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format("input"))
	}

	return os.WriteFile(outPath, []byte(res.RustSource), 0644)
}

// Build runs the full pipeline and produces a native binary.
// It creates a temp Cargo project, writes generated Rust, runs cargo build,
// and copies the binary to outPath.
func Build(source, outPath string) error {
	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format("input"))
	}

	// Create temp directory for Cargo project
	tmpDir, err := os.MkdirTemp("", "intent-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Cargo.toml
	cargoToml := `[package]
name = "intent_output"
version = "0.1.0"
edition = "2021"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %w", err)
	}

	// Create src directory and write main.rs
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(res.RustSource), 0644); err != nil {
		return fmt.Errorf("failed to write main.rs: %w", err)
	}

	// Run cargo build --release
	cmd := exec.Command("cargo", "build", "--release")
	cmd.Dir = tmpDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo build failed: %w", err)
	}

	// Copy binary to output path
	binaryName := "intent_output"
	binarySrc := filepath.Join(tmpDir, "target", "release", binaryName)

	// Ensure output directory exists
	outDir := filepath.Dir(outPath)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create output dir: %w", err)
		}
	}

	srcBin, err := os.ReadFile(binarySrc)
	if err != nil {
		return fmt.Errorf("failed to read built binary: %w", err)
	}

	if err := os.WriteFile(outPath, srcBin, 0755); err != nil {
		return fmt.Errorf("failed to write output binary: %w", err)
	}

	return nil
}

// HasImports checks if a source file contains import declarations by parsing it.
func HasImports(source string) bool {
	p := parser.New(source)
	prog := p.Parse()
	return len(prog.Imports) > 0
}

// CompileProject runs the multi-file pipeline: discover -> sort -> check -> lower -> rustbe.
// entryPath is the path to the entry file (e.g., "examples/multi_file/main.intent").
func CompileProject(entryPath string) *Result {
	res := &Result{}

	// Create module registry
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		res.Diagnostics = diagnostic.New()
		res.Diagnostics.Errorf(0, 0, "failed to initialize module registry: %s", err)
		return res
	}

	// Discover all dependencies
	diag, err := registry.DiscoverDependencies()
	if err != nil {
		if diag == nil {
			diag = diagnostic.New()
		}
		diag.Errorf(0, 0, "%s", err)
		res.Diagnostics = diag
		return res
	}
	if diag.HasErrors() {
		res.Diagnostics = diag
		return res
	}

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		res.Diagnostics = diagnostic.New()
		res.Diagnostics.Errorf(0, 0, "%s", err)
		return res
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths)
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Lower to IR, then generate Rust
	prog := ir.LowerAll(allModules, sortedPaths, checkResult)
	res.RustSource = rustbe.GenerateAll(prog)

	return res
}

// CheckProject runs the multi-file pipeline up to type checking (no codegen).
func CheckProject(entryPath string) *diagnostic.Diagnostics {
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		diag := diagnostic.New()
		diag.Errorf(0, 0, "failed to initialize module registry: %s", err)
		return diag
	}

	diag, err := registry.DiscoverDependencies()
	if err != nil {
		if diag == nil {
			diag = diagnostic.New()
		}
		diag.Errorf(0, 0, "%s", err)
		return diag
	}
	if diag.HasErrors() {
		return diag
	}

	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		diag.Errorf(0, 0, "%s", err)
		return diag
	}

	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths)
	return checkResult.Diagnostics
}

// BuildProject runs the full multi-file pipeline and produces a native binary.
func BuildProject(entryPath, outPath string) error {
	res := CompileProject(entryPath)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format(entryPath))
	}

	// Create temp directory for Cargo project
	tmpDir, err := os.MkdirTemp("", "intent-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Cargo.toml
	cargoToml := `[package]
name = "intent_output"
version = "0.1.0"
edition = "2021"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %w", err)
	}

	// Create src directory and write main.rs
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(res.RustSource), 0644); err != nil {
		return fmt.Errorf("failed to write main.rs: %w", err)
	}

	// Run cargo build --release
	cmd := exec.Command("cargo", "build", "--release")
	cmd.Dir = tmpDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo build failed: %w", err)
	}

	// Copy binary to output path
	binaryName := "intent_output"
	binarySrc := filepath.Join(tmpDir, "target", "release", binaryName)

	// Ensure output directory exists
	outDir := filepath.Dir(outPath)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create output dir: %w", err)
		}
	}

	srcBin, err := os.ReadFile(binarySrc)
	if err != nil {
		return fmt.Errorf("failed to read built binary: %w", err)
	}

	if err := os.WriteFile(outPath, srcBin, 0755); err != nil {
		return fmt.Errorf("failed to write output binary: %w", err)
	}

	return nil
}

// GenerateTests runs parse -> check -> codegen -> testgen for a single file.
// Returns Rust source with appended contract test module.
// Note: testgen still uses codegen.ExprToRust() directly -- see Phase 5 plan.
func GenerateTests(source string) *Result {
	res := &Result{}

	// Parse
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		res.Diagnostics = p.Diagnostics()
		return res
	}

	// Type check
	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Generate Rust via IR pipeline
	mod := ir.Lower(prog, checkResult)
	rustSource := rustbe.Generate(mod)

	// Generate tests (still uses codegen.ExprToRust internally)
	testSource := testgen.Generate(prog)

	res.RustSource = rustSource + testSource

	return res
}

// GenerateTestsProject runs the multi-file pipeline with test generation.
func GenerateTestsProject(entryPath string) *Result {
	res := &Result{}

	// Create module registry
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		res.Diagnostics = diagnostic.New()
		res.Diagnostics.Errorf(0, 0, "failed to initialize module registry: %s", err)
		return res
	}

	// Discover all dependencies
	diag, err := registry.DiscoverDependencies()
	if err != nil {
		if diag == nil {
			diag = diagnostic.New()
		}
		diag.Errorf(0, 0, "%s", err)
		res.Diagnostics = diag
		return res
	}
	if diag.HasErrors() {
		res.Diagnostics = diag
		return res
	}

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		res.Diagnostics = diagnostic.New()
		res.Diagnostics.Errorf(0, 0, "%s", err)
		return res
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths)
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Multi-file code generation via IR pipeline
	prog := ir.LowerAll(allModules, sortedPaths, checkResult)
	rustSource := rustbe.GenerateAll(prog)

	// Generate tests from the entry file's AST (still uses codegen internally)
	entryPath = sortedPaths[len(sortedPaths)-1]
	entryProg := allModules[entryPath]
	testSource := testgen.Generate(entryProg)

	res.RustSource = rustSource + testSource

	return res
}

// IsMultiFile checks if the given file path is a multi-file project
// by parsing it and checking for import declarations.
func IsMultiFile(filePath string) (bool, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	p := parser.New(string(source))
	prog := p.Parse()
	return len(prog.Imports) > 0, nil
}

// Verify runs the full pipeline (parse -> check -> lower -> verify) for a single file
// and returns the verification results.
func Verify(source string) ([]*verify.VerifyResult, error) {
	// Parse
	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		return nil, fmt.Errorf("parse errors:\n%s", p.Diagnostics().Format("input"))
	}

	// Type check
	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("type check errors:\n%s", checkResult.Diagnostics.Format("input"))
	}

	// Lower to IR
	mod := ir.Lower(prog, checkResult)

	// Verify
	results := verify.Verify(mod)
	return results, nil
}

// VerifyProject runs the full pipeline (discover -> check -> lower -> verify)
// for a multi-file project and returns the verification results.
func VerifyProject(entryPath string) ([]*verify.VerifyResult, error) {
	// Create module registry
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize module registry: %w", err)
	}

	// Discover all dependencies
	diag, err := registry.DiscoverDependencies()
	if err != nil {
		return nil, fmt.Errorf("failed to discover dependencies: %w", err)
	}
	if diag.HasErrors() {
		return nil, fmt.Errorf("discovery errors:\n%s", diag.Format(entryPath))
	}

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to sort dependencies: %w", err)
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths)
	if checkResult.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("type check errors:\n%s", checkResult.Diagnostics.Format(entryPath))
	}

	// Lower to IR
	prog := ir.LowerAll(allModules, sortedPaths, checkResult)

	// Verify all modules
	var results []*verify.VerifyResult
	for _, mod := range prog.Modules {
		modResults := verify.Verify(mod)
		results = append(results, modResults...)
	}

	return results, nil
}
