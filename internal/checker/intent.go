package checker

import (
	"github.com/lhaig/intent/internal/ast"
)

// verifyIntents checks that all verified_by references in intent blocks resolve
// to actual contracts in the program
func (c *Checker) verifyIntents() {
	for _, intent := range c.prog.Intents {
		for _, ref := range intent.VerifiedBy {
			c.verifyContractReference(ref)
		}
	}
}

// verifyContractReference verifies that a verified_by reference resolves to an actual contract
func (c *Checker) verifyContractReference(ref *ast.VerifiedByRef) {
	if len(ref.Parts) == 0 {
		line, col := ref.Pos()
		c.diag.Errorf(line, col, "empty verified_by reference")
		return
	}

	// A valid reference can be:
	// - EntityName.invariant (check entity has invariant)
	// - EntityName.MethodName.requires (check method has requires)
	// - EntityName.MethodName.ensures (check method has ensures)

	if len(ref.Parts) == 2 {
		// EntityName.invariant
		entityName := ref.Parts[0]
		contractType := ref.Parts[1]

		entity, exists := c.entities[entityName]
		if !exists {
			line, col := ref.Pos()
			c.diag.Errorf(line, col, "unknown entity '%s' in verified_by reference", entityName)
			return
		}

		if contractType == "invariant" {
			if !entity.HasInvariant {
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "entity '%s' has no invariant", entityName)
			}
		} else {
			line, col := ref.Pos()
			c.diag.Errorf(line, col, "invalid contract reference '%s.%s'; expected 'invariant' or EntityName.MethodName.{requires|ensures}",
				entityName, contractType)
		}
		return
	}

	if len(ref.Parts) == 3 {
		// EntityName.MethodName.{requires|ensures}
		// or EntityName.constructor.{requires|ensures}
		entityName := ref.Parts[0]
		memberName := ref.Parts[1]
		contractType := ref.Parts[2]

		entity, exists := c.entities[entityName]
		if !exists {
			line, col := ref.Pos()
			c.diag.Errorf(line, col, "unknown entity '%s' in verified_by reference", entityName)
			return
		}

		if memberName == "constructor" {
			// Look up constructor contracts from the AST
			hasRequires := false
			hasEnsures := false
			for _, e := range c.prog.Entities {
				if e.Name == entityName && e.Constructor != nil {
					hasRequires = len(e.Constructor.Requires) > 0
					hasEnsures = len(e.Constructor.Ensures) > 0
					break
				}
			}
			switch contractType {
			case "requires":
				if !hasRequires {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "constructor '%s' has no requires clause", entityName)
				}
			case "ensures":
				if !hasEnsures {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "constructor '%s' has no ensures clause", entityName)
				}
			default:
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "invalid contract type '%s'; expected 'requires' or 'ensures'", contractType)
			}
			return
		}

		method, exists := entity.Methods[memberName]
		if !exists {
			line, col := ref.Pos()
			c.diag.Errorf(line, col, "entity '%s' has no method '%s'", entityName, memberName)
			return
		}

		switch contractType {
		case "requires":
			if !method.HasRequires {
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "method '%s.%s' has no requires clause", entityName, memberName)
			}
		case "ensures":
			if !method.HasEnsures {
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "method '%s.%s' has no ensures clause", entityName, memberName)
			}
		default:
			line, col := ref.Pos()
			c.diag.Errorf(line, col, "invalid contract type '%s'; expected 'requires' or 'ensures'", contractType)
		}
		return
	}

	// Invalid reference format
	line, col := ref.Pos()
	c.diag.Errorf(line, col, "invalid verified_by reference format: %s", formatRefParts(ref.Parts))
}

// formatRefParts formats reference parts for error messages
func formatRefParts(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "."
		}
		result += part
	}
	return result
}
