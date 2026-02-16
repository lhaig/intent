package backend

import (
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/rustbe"
)

// RustBackend wraps the existing rustbe as a Backend implementation.
type RustBackend struct{}

// Name returns the backend name.
func (b *RustBackend) Name() string {
	return "rust"
}

// Generate produces Rust source code from a single IR module.
func (b *RustBackend) Generate(mod *ir.Module) string {
	return rustbe.Generate(mod)
}

// GenerateAll produces Rust source from a multi-module IR program.
func (b *RustBackend) GenerateAll(prog *ir.Program) string {
	return rustbe.GenerateAll(prog)
}
