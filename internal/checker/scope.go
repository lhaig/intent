package checker

import "fmt"

// SymbolKind represents the kind of symbol
type SymbolKind int

const (
	SymVariable SymbolKind = iota
	SymFunction
	SymEntity
	SymMethod
	SymField
	SymParam
	SymEnum
)

// String returns the string representation of the symbol kind
func (sk SymbolKind) String() string {
	switch sk {
	case SymVariable:
		return "variable"
	case SymFunction:
		return "function"
	case SymEntity:
		return "entity"
	case SymMethod:
		return "method"
	case SymField:
		return "field"
	case SymParam:
		return "parameter"
	case SymEnum:
		return "enum"
	default:
		return "unknown"
	}
}

// Symbol represents a symbol in the symbol table
type Symbol struct {
	Name    string
	Type    *Type
	Mutable bool
	Kind    SymbolKind
}

// Scope represents a lexical scope with a symbol table
type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
}

// NewScope creates a new scope with an optional parent
func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]*Symbol),
	}
}

// Define adds a symbol to the current scope
// Returns an error if the symbol is already defined in this scope
func (s *Scope) Define(name string, sym *Symbol) error {
	if _, exists := s.symbols[name]; exists {
		return fmt.Errorf("symbol '%s' already defined in this scope", name)
	}
	s.symbols[name] = sym
	return nil
}

// Resolve looks up a symbol in the current scope and parent scopes
// Returns nil if the symbol is not found
func (s *Scope) Resolve(name string) *Symbol {
	if sym, ok := s.symbols[name]; ok {
		return sym
	}
	if s.parent != nil {
		return s.parent.Resolve(name)
	}
	return nil
}

// ResolveLocal looks up a symbol only in the current scope (not parent scopes)
func (s *Scope) ResolveLocal(name string) *Symbol {
	if sym, ok := s.symbols[name]; ok {
		return sym
	}
	return nil
}
