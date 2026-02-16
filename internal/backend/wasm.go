package backend

import (
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/wasmbe"
)

// WasmBackend wraps the wasmbe as a BinaryBackend implementation.
type WasmBackend struct{}

// Name returns the backend name.
func (b *WasmBackend) Name() string {
	return "wasm"
}

// GenerateBytes produces WASM binary from a single IR module.
func (b *WasmBackend) GenerateBytes(mod *ir.Module) []byte {
	return wasmbe.Generate(mod)
}

// GenerateAllBytes produces WASM binary from a multi-module IR program.
func (b *WasmBackend) GenerateAllBytes(prog *ir.Program) []byte {
	return wasmbe.GenerateAll(prog)
}
