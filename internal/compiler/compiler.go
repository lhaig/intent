package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhaig/intent/internal/backend"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/rustbe"
	"github.com/lhaig/intent/internal/testgen"
	"github.com/lhaig/intent/internal/verify"
)

// buildCargoToml generates a Cargo.toml file content based on the Rust source and target type.
// If isCdylib is true, a [lib] section with crate-type = ["cdylib"] is added.
// Sniffer dependencies (tokio/futures/reqwest/serde_json) are added only if
// detected in the Rust source. User-supplied rustDeps (from intent.toml's
// [rust_dependencies] section, Phase 15 / ADR 0028) are appended afterwards
// and override the sniffer for any clashing crate name.
func buildCargoToml(rustSource string, isCdylib bool, rustDeps map[string]RustDependencySpec) string {
	var sb strings.Builder
	sb.WriteString("[package]\n")
	sb.WriteString("name = \"intent_output\"\n")
	sb.WriteString("version = \"0.1.0\"\n")
	sb.WriteString("edition = \"2021\"\n")

	if isCdylib {
		sb.WriteString("\n[lib]\n")
		sb.WriteString("crate-type = [\"cdylib\"]\n")
		sb.WriteString("path = \"src/lib.rs\"\n")
	}

	needsReqwest := strings.Contains(rustSource, "reqwest::")
	needsSerdeJson := strings.Contains(rustSource, "serde_json::")
	needsTokio := strings.Contains(rustSource, "tokio::") || strings.Contains(rustSource, "#[tokio::main]")
	needsFutures := strings.Contains(rustSource, "futures::")

	// User-supplied entries take precedence — skip the sniffer's defaults for
	// any crate that the user already pinned a version for.
	hasUser := func(name string) bool {
		if rustDeps == nil {
			return false
		}
		_, ok := rustDeps[name]
		return ok
	}

	if needsReqwest || needsSerdeJson || needsTokio || needsFutures || len(rustDeps) > 0 {
		sb.WriteString("\n[dependencies]\n")
		if needsReqwest && !hasUser("reqwest") {
			sb.WriteString("reqwest = { version = \"0.12\", features = [\"blocking\"] }\n")
		}
		if needsSerdeJson && !hasUser("serde_json") {
			sb.WriteString("serde_json = \"1\"\n")
		}
		if needsTokio && !hasUser("tokio") {
			sb.WriteString("tokio = { version = \"1\", features = [\"full\"] }\n")
		}
		if needsFutures && !hasUser("futures") {
			sb.WriteString("futures = \"0.3\"\n")
		}
		// Emit user crates in sorted order for deterministic output.
		names := make([]string, 0, len(rustDeps))
		for name := range rustDeps {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			sb.WriteString(rustDeps[name].CargoLine(name))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Result holds the output of a compilation
type Result struct {
	Diagnostics *diagnostic.Diagnostics
	RustSource  string
	BinaryPath  string
}

// Compile runs the full pipeline: parse -> check -> lower -> rustbe.
// Returns the result without writing files or invoking cargo. The
// emitter uses default options (contracts emitted as runtime
// `assert!`); callers needing `--strip-contracts` use Build / BuildToTarget
// directly.
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
	res.RustSource = rustbe.Generate(mod, rustbe.Options{})

	return res
}

// CompileWithOptions is like Compile but the emitter respects the
// supplied options (Phase 22 / ADR 0033 — currently StripContracts).
func CompileWithOptions(source string, opts backend.BuildOptions) *Result {
	res := &Result{}

	p := parser.New(source)
	prog := p.Parse()

	if p.Diagnostics().HasErrors() {
		res.Diagnostics = p.Diagnostics()
		return res
	}

	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	mod := ir.Lower(prog, checkResult)
	res.RustSource = rustbe.Generate(mod, rustbe.Options{StripContracts: opts.StripContracts})

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

// cargoBuild creates a temporary Cargo project from rustSource, runs cargo build --release,
// and copies the resulting binary to outPath. rustDeps may be nil; when present
// it is the resolved [rust_dependencies] section from the entry package's
// intent.toml.
func cargoBuild(rustSource, outPath string, rustDeps map[string]RustDependencySpec) error {
	// Create temp directory for Cargo project
	tmpDir, err := os.MkdirTemp("", "intent-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Cargo.toml
	cargoToml := buildCargoToml(rustSource, false, rustDeps)
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %w", err)
	}

	// Create src directory and write main.rs
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(rustSource), 0644); err != nil {
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

// Build runs the full pipeline and produces a native binary.
// It creates a temp Cargo project, writes generated Rust, runs cargo build,
// and copies the binary to outPath. Phase 22 / ADR 0033: opts carries
// StripContracts; cargo invocation is unchanged (always `--release`).
func Build(source, outPath string, opts backend.BuildOptions) error {
	res := CompileWithOptions(source, opts)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format("input"))
	}

	return cargoBuild(res.RustSource, outPath, nil)
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

	// Warn if cross-package imports are present (codegen doesn't handle them fully)
	if registry.HasCrossPackageImports() {
		diag.Warningf(0, 0, "cross-package type references (entities, enums, traits) in code generation have limited support; simple imports work but complex type hierarchies may produce incomplete output")
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
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Lower to IR, then generate Rust
	prog := ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())
	res.RustSource = rustbe.GenerateAll(prog, rustbe.Options{})

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
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	return checkResult.Diagnostics
}

// BuildProject runs the full multi-file pipeline and produces a native
// binary. Phase 22 / ADR 0033: opts carries StripContracts; cargo
// invocation is unchanged (always `--release`).
func BuildProject(entryPath, outPath string, opts backend.BuildOptions) error {
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		return fmt.Errorf("failed to initialize module registry: %w", err)
	}
	res := compileFromRegistryWithOptions(registry, entryPath, opts)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format(entryPath))
	}

	var rustDeps map[string]RustDependencySpec
	if m := registry.Manifest(); m != nil {
		rustDeps = resolveRustDepPaths(m.RustDependencies, registry.ProjectRoot())
	}
	return cargoBuild(res.RustSource, outPath, rustDeps)
}

// resolveRustDepPaths converts any relative `path` fields in rustDeps to
// absolute paths anchored at the intent.toml's directory, so the generated
// Cargo.toml (which lives in a temp directory) can resolve them correctly.
func resolveRustDepPaths(deps map[string]RustDependencySpec, projectRoot string) map[string]RustDependencySpec {
	if len(deps) == 0 {
		return deps
	}
	out := make(map[string]RustDependencySpec, len(deps))
	for name, dep := range deps {
		if dep.Path != "" && !filepath.IsAbs(dep.Path) {
			dep.Path = filepath.Join(projectRoot, dep.Path)
		}
		out[name] = dep
	}
	return out
}

// compileFromRegistry runs the multi-file pipeline using a pre-built registry,
// so callers (like BuildProject) can reuse the registry to access the parsed
// manifest. Mirrors CompileProject otherwise.
func compileFromRegistry(registry *ModuleRegistry, entryPath string) *Result {
	return compileFromRegistryWithOptions(registry, entryPath, backend.BuildOptions{})
}

// compileFromRegistryWithOptions threads BuildOptions through to the rustbe
// emitter so --strip-contracts reaches the generated source.
func compileFromRegistryWithOptions(registry *ModuleRegistry, entryPath string, opts backend.BuildOptions) *Result {
	res := &Result{}

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

	if registry.HasCrossPackageImports() {
		diag.Warningf(0, 0, "cross-package type references (entities, enums, traits) in code generation have limited support; simple imports work but complex type hierarchies may produce incomplete output")
	}

	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		res.Diagnostics = diagnostic.New()
		res.Diagnostics.Errorf(0, 0, "%s", err)
		return res
	}

	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	prog := ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())
	res.RustSource = rustbe.GenerateAll(prog, rustbe.Options{StripContracts: opts.StripContracts})
	return res
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
	rustSource := rustbe.Generate(mod, rustbe.Options{})

	// Generate tests (still uses codegen.ExprToRust internally)
	testSource := testgen.Generate(prog)

	res.RustSource = rustSource + testSource

	return res
}

// GenerateIntentTests is the phase 16 / ADR 0029 task 16.8 Intent-emission
// counterpart to GenerateTests. `sourceImport` is the relative path the
// generated file will use to import the source module so generated tests
// can call into it (typically the basename of the source file).
func GenerateIntentTests(source, sourceImport string) (string, error) {
	p := parser.New(source)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		return "", fmt.Errorf("parse errors:\n%s", p.Diagnostics().Format("input"))
	}
	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		return "", fmt.Errorf("type-check errors:\n%s", checkResult.Diagnostics.Format("input"))
	}
	return testgen.GenerateIntent(prog, sourceImport), nil
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

	// Warn if cross-package imports are present
	if registry.HasCrossPackageImports() {
		diag.Warningf(0, 0, "cross-package type references (entities, enums, traits) in code generation have limited support; simple imports work but complex type hierarchies may produce incomplete output")
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
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	if checkResult.Diagnostics.HasErrors() {
		res.Diagnostics = checkResult.Diagnostics
		return res
	}
	res.Diagnostics = checkResult.Diagnostics

	// Multi-file code generation via IR pipeline
	prog := ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())
	rustSource := rustbe.GenerateAll(prog, rustbe.Options{})

	// Generate tests from the entry file's AST (still uses codegen internally)
	entryPath = sortedPaths[len(sortedPaths)-1]
	entryProg := allModules[entryPath]
	testSource := testgen.Generate(entryProg)

	res.RustSource = rustSource + testSource

	return res
}

// IsMultiFile checks if the given file path is a multi-file project.
// A file is multi-file if it contains import declarations OR sits next to an
// intent.toml manifest (the manifest may carry [rust_dependencies] entries
// that the single-file Build path cannot see).
func IsMultiFile(filePath string) (bool, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	p := parser.New(string(source))
	prog := p.Parse()
	if len(prog.Imports) > 0 {
		return true, nil
	}
	manifestPath := filepath.Join(filepath.Dir(filePath), "intent.toml")
	if _, statErr := os.Stat(manifestPath); statErr == nil {
		return true, nil
	}
	return false, nil
}

// VerifyOutput holds verification results and intent reports.
type VerifyOutput struct {
	Results       []*verify.VerifyResult
	IntentReports []*verify.IntentReport
}

// Verify runs the full pipeline (parse -> check -> lower -> verify) for a single file
// and returns the verification results.
func Verify(source string) ([]*verify.VerifyResult, error) {
	out, err := VerifyWithReport(source)
	if err != nil {
		return nil, err
	}
	return out.Results, nil
}

// VerifyWithReport runs the full pipeline and returns results with intent reports.
func VerifyWithReport(source string) (*VerifyOutput, error) {
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
	reports := verify.BuildIntentReports(mod, results)

	return &VerifyOutput{Results: results, IntentReports: reports}, nil
}

// VerifyProject runs the full pipeline (discover -> check -> lower -> verify)
// for a multi-file project and returns the verification results.
func VerifyProject(entryPath string) ([]*verify.VerifyResult, error) {
	out, err := VerifyProjectWithReport(entryPath)
	if err != nil {
		return nil, err
	}
	return out.Results, nil
}

// VerifyProjectWithReport runs the multi-file pipeline and returns results with intent reports.
func VerifyProjectWithReport(entryPath string) (*VerifyOutput, error) {
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

	// Warn if cross-package imports are present
	if registry.HasCrossPackageImports() {
		diag.Warningf(0, 0, "cross-package type references (entities, enums, traits) in code generation have limited support; simple imports work but complex type hierarchies may produce incomplete output")
	}

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to sort dependencies: %w", err)
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	if checkResult.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("type check errors:\n%s", checkResult.Diagnostics.Format(entryPath))
	}

	// Lower to IR
	prog := ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())

	// Verify all modules and collect intent reports
	var results []*verify.VerifyResult
	var reports []*verify.IntentReport
	for _, mod := range prog.Modules {
		modResults := verify.Verify(mod)
		results = append(results, modResults...)
		modReports := verify.BuildIntentReports(mod, modResults)
		reports = append(reports, modReports...)
	}

	return &VerifyOutput{Results: results, IntentReports: reports}, nil
}
