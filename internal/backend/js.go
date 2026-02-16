package backend

import (
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/jsbe"
)

// JSBackend wraps the jsbe as a Backend implementation.
type JSBackend struct{}

// Name returns the backend name.
func (b *JSBackend) Name() string {
	return "js"
}

// Generate produces JavaScript source code from a single IR module.
func (b *JSBackend) Generate(mod *ir.Module) string {
	return jsbe.Generate(mod)
}

// GenerateAll produces JavaScript source from a multi-module IR program.
func (b *JSBackend) GenerateAll(prog *ir.Program) string {
	return jsbe.GenerateAll(prog)
}
