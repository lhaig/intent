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

// EmitToTarget compiles source to the given target and writes output file
func EmitToTarget(source, target, baseName string) error {
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
		bbe, err := getBinaryBackend(target)
		if err != nil {
			return err
		}
		wasmBytes := bbe.GenerateBytes(mod)
		outPath := baseName + ".wasm"
		if err := os.WriteFile(outPath, wasmBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Wrote %s\n", outPath)
		return nil
	}

	// Handle text targets (Rust, JS)
	be, err := getBackend(target)
	if err != nil {
		return err
	}
	code := be.Generate(mod)

	ext := getFileExtension(target)
	outPath := baseName + ext
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Wrote %s\n", outPath)
	return nil
}

// EmitProjectToTarget compiles a multi-file project to the given target and writes output file
func EmitProjectToTarget(entryPath, target, baseName string) error {
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

	// Topological sort
	sortedPaths, err := registry.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to sort dependencies: %w", err)
	}

	// Cross-file type checking
	allModules := registry.AllModules()
	checkResult := checker.CheckAll(allModules, sortedPaths)
	if checkResult.Diagnostics.HasErrors() {
		return fmt.Errorf("compilation errors:\n%s", checkResult.Diagnostics.Format(entryPath))
	}

	// Lower to IR
	prog := ir.LowerAll(allModules, sortedPaths, checkResult)

	// Handle binary targets (WASM)
	if target == "wasm" {
		bbe, err := getBinaryBackend(target)
		if err != nil {
			return err
		}
		wasmBytes := bbe.GenerateAllBytes(prog)
		outPath := baseName + ".wasm"
		if err := os.WriteFile(outPath, wasmBytes, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Wrote %s (multi-file)\n", outPath)
		return nil
	}

	// Handle text targets (Rust, JS)
	be, err := getBackend(target)
	if err != nil {
		return err
	}
	code := be.GenerateAll(prog)

	ext := getFileExtension(target)
	outPath := baseName + ext
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Wrote %s (multi-file)\n", outPath)
	return nil
}

// BuildToTarget compiles source to the given target and produces a binary
func BuildToTarget(source, target, baseName string) error {
	switch target {
	case "rust":
		return Build(source, baseName)
	case "js":
		// For JS, just emit the source (no binary build step)
		return EmitToTarget(source, target, baseName)
	case "wasm":
		// Direct WASM emission - no Rust toolchain required
		return EmitToTarget(source, target, baseName)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// BuildProjectToTarget compiles a multi-file project to the given target and produces a binary
func BuildProjectToTarget(entryPath, target, baseName string) error {
	switch target {
	case "rust":
		return BuildProject(entryPath, baseName)
	case "js":
		return EmitProjectToTarget(entryPath, target, baseName)
	case "wasm":
		// Direct WASM emission - no Rust toolchain required
		return EmitProjectToTarget(entryPath, target, baseName)
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

	// Write Cargo.toml with wasm target
	cargoToml := `[package]
name = "intent_output"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]
path = "src/lib.rs"
`
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
