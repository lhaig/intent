package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lhaig/intent/internal/backend"
	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
)

// getBackend returns the appropriate backend for the given target
func getBackend(target string) (backend.Backend, error) {
	switch target {
	case "rust":
		return &backend.RustBackend{}, nil
	case "js":
		return &backend.JSBackend{}, nil
	default:
		return nil, fmt.Errorf("unknown target: %s", target)
	}
}

// getBinaryBackend returns a binary backend for targets that produce binary output
func getBinaryBackend(target string) (backend.BinaryBackend, error) {
	switch target {
	case "wasm":
		return &backend.WasmBackend{}, nil
	default:
		return nil, fmt.Errorf("unknown binary target: %s", target)
	}
}

// asyncFunctionNames returns the names of any async functions in the program.
// Used to reject async code when targeting WASM (which has no runtime).
func asyncFunctionNames(prog *ir.Program) []string {
	var names []string
	for _, mod := range prog.Modules {
		for _, f := range mod.Functions {
			if f.IsAsync {
				names = append(names, f.Name)
			}
		}
	}
	return names
}

func asyncFunctionNamesInModule(mod *ir.Module) []string {
	var names []string
	for _, f := range mod.Functions {
		if f.IsAsync {
			names = append(names, f.Name)
		}
	}
	return names
}

// externFunctionNames returns the names of any extern (FFI) function
// declarations across the program. Used to reject FFI on non-Rust targets.
// Phase 15 / ADR 0028.
func externFunctionNames(prog *ir.Program) []string {
	var names []string
	for _, mod := range prog.Modules {
		for _, ext := range mod.ExternFunctions {
			names = append(names, ext.Name)
		}
	}
	return names
}

func externFunctionNamesInModule(mod *ir.Module) []string {
	var names []string
	for _, ext := range mod.ExternFunctions {
		names = append(names, ext.Name)
	}
	return names
}

// testNames returns the names of any in-language tests across the program.
// Used to reject test declarations on the wasm target (phase 16 / ADR 0029):
// WASM has no exception model, no async runtime, and limited surface for an
// assertion-message channel — implementing a real test runner for WASM is
// deferred to a future phase. Tests should target rust or js.
func testNames(prog *ir.Program) []string {
	var names []string
	for _, mod := range prog.Modules {
		for _, t := range mod.Tests {
			names = append(names, t.Name)
		}
	}
	return names
}

func testNamesInModule(mod *ir.Module) []string {
	var names []string
	for _, t := range mod.Tests {
		names = append(names, t.Name)
	}
	return names
}

// getFileExtension returns the file extension for the given target
func getFileExtension(target string) string {
	switch target {
	case "rust":
		return ".rs"
	case "js":
		return ".js"
	case "wasm":
		return ".wasm"
	default:
		return ""
	}
}

// EmitToTarget compiles source to the given target and writes output file.
// Phase 22 / ADR 0033: opts carries --release and --strip-contracts.
// Zero-value opts preserves pre-Phase-22 behaviour.
func EmitToTarget(source, target, baseName string, opts backend.BuildOptions) error {
	// Parse
	p := parser.New(source)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", p.Diagnostics().Format("input"))
	}

	// Type check
	checkResult := checker.CheckWithResult(prog)
	if checkResult.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", checkResult.Diagnostics.Format("input"))
	}

	// Lower to IR
	mod := ir.Lower(prog, checkResult)

	// Handle binary targets (WASM)
	if target == "wasm" {
		if names := asyncFunctionNamesInModule(mod); len(names) > 0 {
			return fmt.Errorf("async functions are not supported on the wasm target (found: %v); use --target rust or --target js", names)
		}
		if names := externFunctionNamesInModule(mod); len(names) > 0 {
			return fmt.Errorf("extern (Rust FFI) declarations are not supported on the wasm target (found: %v); use --target rust", names)
		}
		if names := testNamesInModule(mod); len(names) > 0 {
			return fmt.Errorf("test declarations are not supported on the wasm target (found: %v); use --target rust or --target js (phase 16 / ADR 0029)", names)
		}
		bbe, err := getBinaryBackend(target)
		if err != nil {
			return err
		}
		wasmBytes := bbe.GenerateBytes(mod, opts)
		outPath := baseName + ".wasm"
		if err := os.WriteFile(outPath, wasmBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Wrote %s\n", outPath)
		return nil
	}

	// Reject extern declarations on the JS target — Rust FFI is Rust-only.
	if target == "js" {
		if names := externFunctionNamesInModule(mod); len(names) > 0 {
			return fmt.Errorf("extern (Rust FFI) declarations are not supported on the js target (found: %v); use --target rust", names)
		}
	}

	// Handle text targets (Rust, JS)
	be, err := getBackend(target)
	if err != nil {
		return err
	}
	code := be.Generate(mod, opts)

	ext := getFileExtension(target)
	outPath := baseName + ext
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Wrote %s\n", outPath)
	return nil
}

// EmitProjectToTarget compiles a multi-file project to the given target and writes output file
func EmitProjectToTarget(entryPath, target, baseName string, opts backend.BuildOptions) error {
	// Create module registry
	registry, err := NewModuleRegistry(entryPath)
	if err != nil {
		return fmt.Errorf("failed to initialize module registry: %w", err)
	}

	// Discover all dependencies
	diag, err := registry.DiscoverDependencies()
	if err != nil {
		return fmt.Errorf("failed to discover dependencies: %w", err)
	}
	if diag.HasErrors() {
		return fmt.Errorf("discovery errors:\n%s", diag.Format(entryPath))
	}

	// Warn if cross-package imports are present
	if registry.HasCrossPackageImports() {
		diag.Warningf(0, 0, "cross-package type references (entities, enums, traits) in code generation have limited support; simple imports work but complex type hierarchies may produce incomplete output")
	}

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to sort dependencies: %w", err)
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths, registry.PackageDirs())
	if checkResult.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", checkResult.Diagnostics.Format(entryPath))
	}

	// Lower to IR
	prog := ir.LowerAll(allModules, sortedPaths, checkResult, registry.PackageDirs())

	// Handle binary targets (WASM)
	if target == "wasm" {
		if names := asyncFunctionNames(prog); len(names) > 0 {
			return fmt.Errorf("async functions are not supported on the wasm target (found: %v); use --target rust or --target js", names)
		}
		if names := externFunctionNames(prog); len(names) > 0 {
			return fmt.Errorf("extern (Rust FFI) declarations are not supported on the wasm target (found: %v); use --target rust", names)
		}
		if names := testNames(prog); len(names) > 0 {
			return fmt.Errorf("test declarations are not supported on the wasm target (found: %v); use --target rust or --target js (phase 16 / ADR 0029)", names)
		}
		bbe, err := getBinaryBackend(target)
		if err != nil {
			return err
		}
		wasmBytes := bbe.GenerateAllBytes(prog, opts)
		outPath := baseName + ".wasm"
		if err := os.WriteFile(outPath, wasmBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Wrote %s (multi-file)\n", outPath)
		return nil
	}

	// Reject extern declarations on the JS target.
	if target == "js" {
		if names := externFunctionNames(prog); len(names) > 0 {
			return fmt.Errorf("extern (Rust FFI) declarations are not supported on the js target (found: %v); use --target rust", names)
		}
	}

	// Handle text targets (Rust, JS)
	be, err := getBackend(target)
	if err != nil {
		return err
	}
	code := be.GenerateAll(prog, opts)

	ext := getFileExtension(target)
	outPath := baseName + ext
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Wrote %s (multi-file)\n", outPath)
	return nil
}

// BuildToTarget compiles source to the given target and produces a binary.
// Phase 22 / ADR 0033: opts carries --release and --strip-contracts.
func BuildToTarget(source, target, baseName string, opts backend.BuildOptions) error {
	switch target {
	case "rust":
		return Build(source, baseName, opts)
	case "js":
		// For JS, just emit the source (no binary build step)
		return EmitToTarget(source, target, baseName, opts)
	case "wasm":
		// Direct WASM emission - no Rust toolchain required
		return EmitToTarget(source, target, baseName, opts)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// BuildProjectToTarget compiles a multi-file project to the given target and produces a binary
func BuildProjectToTarget(entryPath, target, baseName string, opts backend.BuildOptions) error {
	switch target {
	case "rust":
		return BuildProject(entryPath, baseName, opts)
	case "js":
		return EmitProjectToTarget(entryPath, target, baseName, opts)
	case "wasm":
		// Direct WASM emission - no Rust toolchain required
		return EmitProjectToTarget(entryPath, target, baseName, opts)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// buildWasmViaRust compiles single-file source to WASM via Rust (legacy path).
// This is retained for cases where the full Rust toolchain produces more optimized output.
func buildWasmViaRust(source, baseName string) error {
	res := Compile(source)
	if res.Diagnostics != nil && res.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", res.Diagnostics.Format("input"))
	}

	return buildWasmFromRust(res.RustSource, baseName)
}

// buildWasmFromRust builds WASM from Rust source
func buildWasmFromRust(rustSource, baseName string) error {
	// Create temp directory for Cargo project
	tmpDir, err := os.MkdirTemp("", "intent-wasm-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Cargo.toml with wasm target. Single-file wasm-via-rust path
	// does not carry an intent.toml context, so no user rust_dependencies.
	cargoToml := buildCargoToml(rustSource, true, nil)
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %w", err)
	}

	// Create src directory and write lib.rs (not main.rs for WASM)
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "lib.rs"), []byte(rustSource), 0644); err != nil {
		return fmt.Errorf("failed to write lib.rs: %w", err)
	}

	// Run cargo build --release --target wasm32-unknown-unknown
	fmt.Printf("Compiling to WASM via Rust (this may take a moment)...\n")
	cmd := exec.Command("cargo", "build", "--release", "--target", "wasm32-unknown-unknown")
	cmd.Dir = tmpDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo build failed: %w (make sure wasm32-unknown-unknown target is installed with: rustup target add wasm32-unknown-unknown)", err)
	}

	// Copy wasm file to output path
	wasmSrc := filepath.Join(tmpDir, "target", "wasm32-unknown-unknown", "release", "intent_output.wasm")
	wasmBytes, err := os.ReadFile(wasmSrc)
	if err != nil {
		return fmt.Errorf("failed to read built wasm: %w", err)
	}

	outPath := baseName + ".wasm"
	if err := os.WriteFile(outPath, wasmBytes, 0644); err != nil {
		return fmt.Errorf("failed to write output wasm: %w", err)
	}

	fmt.Printf("Built %s\n", outPath)
	return nil
}
