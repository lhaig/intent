package checker

import (
	"fmt"

	"github.com/lhaig/intent/internal/ast"
)

// registerTraits registers all trait declarations
func (c *Checker) registerTraits() {
	for _, trait := range c.prog.Traits {
		if _, exists := c.traits[trait.Name]; exists {
			line, col := trait.Pos()
			c.diag.Errorf(line, col, "trait '%s' already defined", trait.Name)
			continue
		}

		info := &TraitInfo{
			Name:    trait.Name,
			Methods: make(map[string]*MethodInfo),
		}

		for _, method := range trait.Methods {
			params := make([]ParamInfo, 0, len(method.Params))
			for _, p := range method.Params {
				pType := ResolveType(p.Type, c.entities, c.enums)
				if pType == nil {
					line, col := p.Pos()
					c.diag.Errorf(line, col, "unknown type '%s'", p.Type.Name)
					pType = TypeInt // fallback
				}
				params = append(params, ParamInfo{Name: p.Name, Type: pType})
			}

			returnType := TypeVoid
			if method.ReturnType != nil {
				returnType = ResolveType(method.ReturnType, c.entities, c.enums)
				if returnType == nil {
					line, col := method.Pos()
					c.diag.Errorf(line, col, "unknown type '%s'", method.ReturnType.Name)
					returnType = TypeVoid
				}
			}

			if _, dup := info.Methods[method.Name]; dup {
				line, col := method.Pos()
				c.diag.Errorf(line, col, "duplicate method '%s' in trait '%s'", method.Name, trait.Name)
				continue
			}

			info.Methods[method.Name] = &MethodInfo{
				Name:        method.Name,
				Params:      params,
				ReturnType:  returnType,
				HasRequires: len(method.Requires) > 0,
				HasEnsures:  len(method.Ensures) > 0,
			}
		}

		c.traits[trait.Name] = info
	}
}

// checkImplBlocks validates impl blocks: verifies trait/entity exist, methods match signatures,
// and merges methods into entity info for method resolution.
func (c *Checker) checkImplBlocks() {
	for _, impl := range c.prog.ImplBlocks {
		line, col := impl.Pos()

		// Verify trait exists
		traitInfo, traitExists := c.traits[impl.TraitName]
		if !traitExists {
			c.diag.Errorf(line, col, "unknown trait '%s'", impl.TraitName)
			continue
		}

		// Verify entity exists
		entityInfo, entityExists := c.entities[impl.EntityName]
		if !entityExists {
			c.diag.Errorf(line, col, "unknown entity '%s'", impl.EntityName)
			continue
		}

		// Build map of implemented methods
		implMethods := make(map[string]*ast.MethodDecl)
		for _, m := range impl.Methods {
			if _, dup := implMethods[m.Name]; dup {
				mLine, mCol := m.Pos()
				c.diag.Errorf(mLine, mCol, "duplicate method '%s' in impl block", m.Name)
				continue
			}
			implMethods[m.Name] = m
		}

		// Check all trait methods are implemented
		for traitMethodName, traitMethod := range traitInfo.Methods {
			implMethod, ok := implMethods[traitMethodName]
			if !ok {
				c.diag.Errorf(line, col, "impl '%s' for '%s' is missing method '%s'",
					impl.TraitName, impl.EntityName, traitMethodName)
				continue
			}

			// Check parameter count matches
			if len(implMethod.Params) != len(traitMethod.Params) {
				mLine, mCol := implMethod.Pos()
				c.diag.Errorf(mLine, mCol,
					"method '%s' has %d parameters, trait '%s' requires %d",
					traitMethodName, len(implMethod.Params), impl.TraitName, len(traitMethod.Params))
				continue
			}

			// Check parameter types match
			for i, implParam := range implMethod.Params {
				implParamType := ResolveType(implParam.Type, c.entities, c.enums)
				if implParamType != nil && !implParamType.Equal(traitMethod.Params[i].Type) {
					pLine, pCol := implParam.Pos()
					c.diag.Errorf(pLine, pCol,
						"parameter '%s' of method '%s' has type %s, trait '%s' requires %s",
						implParam.Name, traitMethodName, implParamType.String(),
						impl.TraitName, traitMethod.Params[i].Type.String())
				}
			}

			// Check return type matches
			implRetType := TypeVoid
			if implMethod.ReturnType != nil {
				implRetType = ResolveType(implMethod.ReturnType, c.entities, c.enums)
				if implRetType == nil {
					implRetType = TypeVoid
				}
			}
			if !implRetType.Equal(traitMethod.ReturnType) {
				mLine, mCol := implMethod.Pos()
				c.diag.Errorf(mLine, mCol,
					"method '%s' returns %s, trait '%s' requires %s",
					traitMethodName, implRetType.String(), impl.TraitName, traitMethod.ReturnType.String())
			}
		}

		// Check no extra methods
		for implMethodName := range implMethods {
			if _, ok := traitInfo.Methods[implMethodName]; !ok {
				m := implMethods[implMethodName]
				mLine, mCol := m.Pos()
				c.diag.Errorf(mLine, mCol,
					"method '%s' is not defined in trait '%s'", implMethodName, impl.TraitName)
			}
		}

		// Merge trait methods into entity info for method resolution
		for _, m := range impl.Methods {
			if _, exists := traitInfo.Methods[m.Name]; !exists {
				continue // skip extra methods (already reported)
			}

			params := make([]ParamInfo, 0, len(m.Params))
			for _, p := range m.Params {
				pType := ResolveType(p.Type, c.entities, c.enums)
				if pType == nil {
					pType = TypeInt
				}
				params = append(params, ParamInfo{Name: p.Name, Type: pType})
			}

			retType := TypeVoid
			if m.ReturnType != nil {
				retType = ResolveType(m.ReturnType, c.entities, c.enums)
				if retType == nil {
					retType = TypeVoid
				}
			}

			// Use trait's contracts if impl method doesn't specify its own
			traitMethod := traitInfo.Methods[m.Name]

			entityInfo.Methods[m.Name] = &MethodInfo{
				Name:        m.Name,
				Params:      params,
				ReturnType:  retType,
				HasRequires: len(m.Requires) > 0 || traitMethod.HasRequires,
				HasEnsures:  len(m.Ensures) > 0 || traitMethod.HasEnsures,
			}

			// Record origin for codegen
			c.implOrigins[fmt.Sprintf("%s.%s", impl.EntityName, m.Name)] = impl.TraitName
		}
	}
}

// findTraitMethodDecl looks up the AST TraitMethodDecl for a method in a trait
func (c *Checker) findTraitMethodDecl(traitName, methodName string) *ast.TraitMethodDecl {
	for _, trait := range c.prog.Traits {
		if trait.Name == traitName {
			for _, m := range trait.Methods {
				if m.Name == methodName {
					return m
				}
			}
		}
	}
	return nil
}

// checkImplBlockBodies type-checks each impl method body with entity context
func (c *Checker) checkImplBlockBodies() {
	for _, impl := range c.prog.ImplBlocks {
		entityInfo, entityExists := c.entities[impl.EntityName]
		if !entityExists {
			continue // error already reported
		}

		_, traitExists := c.traits[impl.TraitName]
		if !traitExists {
			continue // error already reported
		}

		// Set entity context for self resolution
		c.entityCtx = &EntityContext{
			Entity:   entityInfo,
			InMethod: true,
		}

		for _, method := range impl.Methods {
			// Look up trait method AST for contracts
			traitMethodDecl := c.findTraitMethodDecl(impl.TraitName, method.Name)

			methodScope := NewScope(c.scope)

			// Add 'self' to method scope
			methodScope.Define("self", &Symbol{
				Name:    "self",
				Type:    &Type{Name: impl.EntityName, IsEntity: true, Entity: entityInfo},
				Mutable: false,
				Kind:    SymVariable,
			})

			// Add parameters to method scope
			for _, p := range method.Params {
				pType := ResolveType(p.Type, c.entities, c.enums)
				if pType != nil {
					methodScope.Define(p.Name, &Symbol{
						Name:    p.Name,
						Type:    pType,
						Mutable: false,
						Kind:    SymParam,
					})
				}
			}

			// Set up function info for return type checking
			retType := TypeVoid
			if method.ReturnType != nil {
				retType = ResolveType(method.ReturnType, c.entities, c.enums)
				if retType == nil {
					retType = TypeVoid
				}
			}
			c.currentFunc = &FuncInfo{
				Name:       method.Name,
				ReturnType: retType,
			}

			// Check requires clauses (trait's + impl's)
			oldCtx := c.contractCtx
			c.contractCtx = CtxRequires
			if traitMethodDecl != nil {
				for _, req := range traitMethodDecl.Requires {
					exprType := c.checkExpression(req.Expr, methodScope)
					if exprType != nil && !exprType.Equal(TypeBool) {
						line, col := req.Pos()
						c.diag.Errorf(line, col, "requires clause must be boolean, got %s", exprType.Name)
					}
				}
			}
			for _, req := range method.Requires {
				exprType := c.checkExpression(req.Expr, methodScope)
				if exprType != nil && !exprType.Equal(TypeBool) {
					line, col := req.Pos()
					c.diag.Errorf(line, col, "requires clause must be boolean, got %s", exprType.Name)
				}
			}

			// Check ensures clauses (trait's + impl's)
			c.contractCtx = CtxEnsures
			if traitMethodDecl != nil {
				for _, ens := range traitMethodDecl.Ensures {
					exprType := c.checkExpression(ens.Expr, methodScope)
					if exprType != nil && !exprType.Equal(TypeBool) {
						line, col := ens.Pos()
						c.diag.Errorf(line, col, "ensures clause must be boolean, got %s", exprType.Name)
					}
				}
			}
			for _, ens := range method.Ensures {
				exprType := c.checkExpression(ens.Expr, methodScope)
				if exprType != nil && !exprType.Equal(TypeBool) {
					line, col := ens.Pos()
					c.diag.Errorf(line, col, "ensures clause must be boolean, got %s", exprType.Name)
				}
			}
			c.contractCtx = oldCtx

			// Check body
			if method.Body != nil {
				c.checkBlock(method.Body, methodScope)
			}

			c.currentFunc = nil
		}

		c.entityCtx = nil
	}
}
