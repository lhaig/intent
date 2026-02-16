package backend

import "github.com/lhaig/intent/internal/ir"

// Backend is the interface that all code generation backends implement.
type Backend interface {
	// Name returns the backend name (e.g., "rust", "js", "wasm")
	Name() string
	// Generate produces output source code from a single IR module.
	Generate(mod *ir.Module) string
	// GenerateAll produces output from a multi-module IR program.
	GenerateAll(prog *ir.Program) string
}
