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
		firstName := ref.Parts[0]
		contractType := ref.Parts[1]

		// Check if it's an entity reference (EntityName.invariant)
		if entity, exists := c.entities[firstName]; exists {
			if contractType == "invariant" {
				if !entity.HasInvariant {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "entity '%s' has no invariant", firstName)
				}
			} else {
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "invalid contract reference '%s.%s'; expected 'invariant' or EntityName.MethodName.{requires|ensures}",
					firstName, contractType)
			}
			return
		}

		// Check if it's a standalone function reference (function_name.requires/ensures)
		if fn, exists := c.functions[firstName]; exists {
			_ = fn // function exists, check contract type
			// Look up the AST function to check for requires/ensures
			var hasRequires, hasEnsures bool
			for _, fnDecl := range c.prog.Functions {
				if fnDecl.Name == firstName {
					hasRequires = len(fnDecl.Requires) > 0
					hasEnsures = len(fnDecl.Ensures) > 0
					break
				}
			}
			switch contractType {
			case "requires":
				if !hasRequires {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "function '%s' has no requires clause", firstName)
				}
			case "ensures":
				if !hasEnsures {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "function '%s' has no ensures clause", firstName)
				}
			default:
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "invalid contract type '%s'; expected 'requires' or 'ensures'", contractType)
			}
			return
		}

		line, col := ref.Pos()
		c.diag.Errorf(line, col, "unknown entity or function '%s' in verified_by reference", firstName)
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

		// Handle constructor as a special case
		if memberName == "constructor" {
			if !entity.HasConstructor {
				line, col := ref.Pos()
				c.diag.Errorf(line, col, "entity '%s' has no constructor", entityName)
				return
			}
			// Check constructor contracts from AST
			var hasRequires, hasEnsures bool
			for _, entityDecl := range c.prog.Entities {
				if entityDecl.Name == entityName && entityDecl.Constructor != nil {
					hasRequires = len(entityDecl.Constructor.Requires) > 0
					hasEnsures = len(entityDecl.Constructor.Ensures) > 0
					break
				}
			}
			switch contractType {
			case "requires":
				if !hasRequires {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "constructor '%s.constructor' has no requires clause", entityName)
				}
			case "ensures":
				if !hasEnsures {
					line, col := ref.Pos()
					c.diag.Errorf(line, col, "constructor '%s.constructor' has no ensures clause", entityName)
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
