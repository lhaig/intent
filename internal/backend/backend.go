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

// BinaryBackend is the interface for backends that produce binary output (e.g., WASM).
type BinaryBackend interface {
	// Name returns the backend name.
	Name() string
	// GenerateBytes produces binary output from a single IR module.
	GenerateBytes(mod *ir.Module) []byte
	// GenerateAllBytes produces binary output from a multi-module IR program.
	GenerateAllBytes(prog *ir.Program) []byte
}
