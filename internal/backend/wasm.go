package backend

import (
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/wasmbe"
)

// WasmBackend wraps the wasmbe as a BinaryBackend implementation.
//
// Phase 22 / ADR 0033: the WASM backend doesn't emit contract checks
// today (see wasmbe.go), so BuildOptions.StripContracts has no effect.
// The option is propagated for consistency and to leave room for a
// future contract-emitting WASM path.
type WasmBackend struct{}

// Name returns the backend name.
func (b *WasmBackend) Name() string {
	return "wasm"
}

// GenerateBytes produces WASM binary from a single IR module.
func (b *WasmBackend) GenerateBytes(mod *ir.Module, opts BuildOptions) []byte {
	_ = opts
	return wasmbe.Generate(mod)
}

// GenerateAllBytes produces WASM binary from a multi-module IR program.
func (b *WasmBackend) GenerateAllBytes(prog *ir.Program, opts BuildOptions) []byte {
	_ = opts
	return wasmbe.GenerateAll(prog)
}
