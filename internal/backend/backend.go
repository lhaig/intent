package backend

import "github.com/lhaig/intent/internal/ir"

// BuildOptions carries cross-cutting flags that affect codegen but are
// not part of the IR. Phase 22 / ADR 0033 (revised): currently only
// StripContracts. The struct is the extension point for future
// codegen-time flags (e.g., verification-aware stripping).
//
// The zero value preserves pre-Phase-22 behaviour: contracts emit as
// runtime `assert!(...)` (Rust) / `if (!(cond)) throw ...` (JS).
// Callers that don't care about these flags pass `BuildOptions{}`.
type BuildOptions struct {
	// StripContracts drops runtime contract checks from emitted output.
	// Rust: `assert!(...)` becomes `debug_assert!(...)`, which cargo's
	// always-on `--release` profile compiles out. JS: the
	// `if (!(cond)) throw ...` lines are omitted entirely. WASM: no
	// effect (the backend doesn't emit contract checks today).
	//
	// User-written assertion builtins (`assert(...)`, `assert_eq(...)`,
	// `assert_close(...)`, `assert_panics(...)`) from test bodies are
	// NOT contracts and are unaffected by this flag.
	StripContracts bool
}

// Backend is the interface that all code generation backends implement.
type Backend interface {
	// Name returns the backend name (e.g., "rust", "js", "wasm")
	Name() string
	// Generate produces output source code from a single IR module.
	Generate(mod *ir.Module, opts BuildOptions) string
	// GenerateAll produces output from a multi-module IR program.
	GenerateAll(prog *ir.Program, opts BuildOptions) string
}

// BinaryBackend is the interface for backends that produce binary output (e.g., WASM).
type BinaryBackend interface {
	// Name returns the backend name.
	Name() string
	// GenerateBytes produces binary output from a single IR module.
	GenerateBytes(mod *ir.Module, opts BuildOptions) []byte
	// GenerateAllBytes produces binary output from a multi-module IR program.
	GenerateAllBytes(prog *ir.Program, opts BuildOptions) []byte
}
