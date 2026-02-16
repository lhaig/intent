package checker

import (
	"path/filepath"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/lexer"
)

// ContractContext tracks where we are during checking for contract validation
type ContractContext int

const (
	CtxNormal    ContractContext = iota // Normal code
	CtxRequires                         // Inside requires clause
	CtxEnsures                          // Inside ensures clause
	CtxInvariant                        // Inside invariant
)

// EntityContext tracks the current entity being checked
type EntityContext struct {
	Entity        *EntityInfo
	InConstructor bool
	InMethod      bool
}

// ModuleSymbols holds the public symbols exported by a module
type ModuleSymbols struct {
	Functions map[string]*FuncInfo
	Entities  map[string]*EntityInfo
	Enums     map[string]*EnumInfo
	// Keep references to the AST declarations for codegen and contract checking
	FunctionDecls map[string]*ast.FunctionDecl
	EntityDecls   map[string]*ast.EntityDecl
	EnumDecls     map[string]*ast.EnumDecl
}

// Checker performs semantic analysis on the AST
type Checker struct {
	prog         *ast.Program
	diag         *diagnostic.Diagnostics
	entities     map[string]*EntityInfo
	enums        map[string]*EnumInfo
	enumVariants map[string]*EnumVariantLookup
	functions    map[string]*FuncInfo
	scope        *Scope
	exprTypes    map[ast.Expression]*Type

	// Context tracking
	contractCtx     ContractContext
	entityCtx       *EntityContext
	loopDepth       int
	currentFunc     *FuncInfo // Track current function for Result/Option variant inference
	letDeclaredType *Type     // Track type annotation from let statement for variant inference

	// Cross-file (multi-module) context
	moduleImports map[string]*ModuleSymbols // module alias -> public symbols
	moduleFile    string                    // file path for error reporting
}

// EnumVariantLookup maps a variant name to its parent enum and variant info
type EnumVariantLookup struct {
	EnumInfo    *EnumInfo
	VariantInfo *EnumVariantInfo
}

// FuncInfo holds information about a function
type FuncInfo struct {
	Name       string
	Params     []ParamInfo
	ReturnType *Type
}

// CheckResult holds the results of type checking for use by later pipeline stages
type CheckResult struct {
	Diagnostics *diagnostic.Diagnostics
	ExprTypes   map[ast.Expression]*Type
	Entities    map[string]*EntityInfo
	Enums       map[string]*EnumInfo
}

// CheckWithResult performs semantic analysis and returns results for downstream stages
func CheckWithResult(prog *ast.Program) *CheckResult {
	c := &Checker{
		prog:         prog,
		diag:         diagnostic.New(),
		entities:     make(map[string]*EntityInfo),
		enums:        make(map[string]*EnumInfo),
		enumVariants: make(map[string]*EnumVariantLookup),
		functions:    make(map[string]*FuncInfo),
		scope:        NewScope(nil),
		contractCtx:  CtxNormal,
		entityCtx:    nil,
		exprTypes:    make(map[ast.Expression]*Type),
	}

	c.registerEnums()
	c.registerEntities()
	c.registerFunctions()
	c.checkFunctions()
	c.checkEntities()
	c.verifyIntents()

	return &CheckResult{
		Diagnostics: c.diag,
		ExprTypes:   c.exprTypes,
		Entities:    c.entities,
		Enums:       c.enums,
	}
}

// Check performs semantic analysis on an AST program
func Check(prog *ast.Program) *diagnostic.Diagnostics {
	return CheckWithResult(prog).Diagnostics
}

// CheckAllResult holds results from multi-file type checking
type CheckAllResult struct {
	Diagnostics *diagnostic.Diagnostics
	ExprTypes   map[ast.Expression]*Type
	Entities    map[string]*EntityInfo
	Enums       map[string]*EnumInfo
}

// moduleNameFromPath derives a module name from a file path.
// e.g., "/project/math.intent" -> "math", "/project/sub/helpers.intent" -> "helpers"
func moduleNameFromPath(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, ".intent")
}

// CheckAll performs two-pass cross-file type checking.
// Pass 1: Register public symbols from all files.
// Pass 2: Type-check each file with cross-file context (qualified name resolution, visibility).
func CheckAll(registry map[string]*ast.Program, sortedPaths []string) *CheckAllResult {
	diag := diagnostic.New()
	allExprTypes := make(map[ast.Expression]*Type)
	allEntities := make(map[string]*EntityInfo)
	allEnums := make(map[string]*EnumInfo)

	// Pass 1: Register public symbols from all files
	publicSymbols := make(map[string]*ModuleSymbols) // moduleName -> symbols

	for _, filePath := range sortedPaths {
		prog := registry[filePath]
		if prog == nil {
			continue
		}

		modName := moduleNameFromPath(filePath)
		modSyms := &ModuleSymbols{
			Functions:     make(map[string]*FuncInfo),
			Entities:      make(map[string]*EntityInfo),
			Enums:         make(map[string]*EnumInfo),
			FunctionDecls: make(map[string]*ast.FunctionDecl),
			EntityDecls:   make(map[string]*ast.EntityDecl),
			EnumDecls:     make(map[string]*ast.EnumDecl),
		}

		// Create a temporary checker to register entities/enums/functions for type resolution
		tmpChecker := &Checker{
			prog:         prog,
			diag:         diagnostic.New(),
			entities:     make(map[string]*EntityInfo),
			enums:        make(map[string]*EnumInfo),
			enumVariants: make(map[string]*EnumVariantLookup),
			functions:    make(map[string]*FuncInfo),
			scope:        NewScope(nil),
			contractCtx:  CtxNormal,
			exprTypes:    make(map[ast.Expression]*Type),
		}
		tmpChecker.registerEnums()
		tmpChecker.registerEntities()
		tmpChecker.registerFunctions()

		// Collect public functions
		for _, fn := range prog.Functions {
			if fn.IsPublic {
				if fi, ok := tmpChecker.functions[fn.Name]; ok {
					modSyms.Functions[fn.Name] = fi
					modSyms.FunctionDecls[fn.Name] = fn
				}
			}
		}

		// Collect public entities
		for _, entity := range prog.Entities {
			if entity.IsPublic {
				if ei, ok := tmpChecker.entities[entity.Name]; ok {
					modSyms.Entities[entity.Name] = ei
					modSyms.EntityDecls[entity.Name] = entity
				}
			}
		}

		// Collect public enums
		for _, enum := range prog.Enums {
			if enum.IsPublic {
				if ei, ok := tmpChecker.enums[enum.Name]; ok {
					modSyms.Enums[enum.Name] = ei
					modSyms.EnumDecls[enum.Name] = enum
				}
			}
		}

		publicSymbols[modName] = modSyms
	}

	// Pass 2: Type-check each file with cross-file context
	for _, filePath := range sortedPaths {
		prog := registry[filePath]
		if prog == nil {
			continue
		}

		// Build moduleImports for this file: for each import, look up the module's public symbols
		moduleImports := make(map[string]*ModuleSymbols)
		for _, imp := range prog.Imports {
			importedModName := strings.TrimSuffix(filepath.Base(imp.Path), ".intent")
			if syms, ok := publicSymbols[importedModName]; ok {
				moduleImports[importedModName] = syms
			}
		}

		// Create checker for this file
		c := &Checker{
			prog:          prog,
			diag:          diagnostic.New(),
			entities:      make(map[string]*EntityInfo),
			enums:         make(map[string]*EnumInfo),
			enumVariants:  make(map[string]*EnumVariantLookup),
			functions:     make(map[string]*FuncInfo),
			scope:         NewScope(nil),
			contractCtx:   CtxNormal,
			exprTypes:     make(map[ast.Expression]*Type),
			moduleImports: moduleImports,
			moduleFile:    filePath,
		}

		c.registerEnums()
		c.registerEntities()
		c.registerFunctions()

		// Inject imported public entities and enums into this checker's type maps
		// so that type annotations like `let c: Circle` resolve correctly when
		// Circle is a public entity from an imported module
		for _, modSyms := range moduleImports {
			for name, entityInfo := range modSyms.Entities {
				if _, exists := c.entities[name]; !exists {
					c.entities[name] = entityInfo
					c.scope.Define(name, &Symbol{
						Name: name,
						Type: &Type{Name: name, IsEntity: true, Entity: entityInfo},
						Kind: SymEntity,
					})
				}
			}
			for name, enumInfo := range modSyms.Enums {
				if _, exists := c.enums[name]; !exists {
					c.enums[name] = enumInfo
					c.scope.Define(name, &Symbol{
						Name: name,
						Type: &Type{Name: name, IsEnum: true, EnumInfo: enumInfo},
						Kind: SymEnum,
					})
				}
			}
		}

		c.checkFunctions()
		c.checkEntities()
		c.verifyIntents()

		// Collect diagnostics with file context
		for _, d := range c.diag.All() {
			if d.Severity == diagnostic.Error {
				diag.ErrorfInFile(filePath, d.Line, d.Column, "%s", d.Message)
			}
		}

		// Merge expression types and entity/enum info
		for expr, t := range c.exprTypes {
			allExprTypes[expr] = t
		}
		for name, info := range c.entities {
			allEntities[name] = info
		}
		for name, info := range c.enums {
			allEnums[name] = info
		}
	}

	return &CheckAllResult{
		Diagnostics: diag,
		ExprTypes:   allExprTypes,
		Entities:    allEntities,
		Enums:       allEnums,
	}
}

// registerEntities registers all entities in the global scope
func (c *Checker) registerEntities() {
	for _, entity := range c.prog.Entities {
		if _, exists := c.entities[entity.Name]; exists {
			line, col := entity.Pos()
			c.diag.Errorf(line, col, "entity '%s' already defined", entity.Name)
			continue
		}

		info := &EntityInfo{
			Name:           entity.Name,
			Fields:         make(map[string]*Type),
			FieldOrder:     make([]string, 0),
			HasInvariant:   len(entity.Invariants) > 0,
			Methods:        make(map[string]*MethodInfo),
			HasConstructor: entity.Constructor != nil,
		}

		// Register fields
		for _, field := range entity.Fields {
			fieldType := ResolveType(field.Type, c.entities, c.enums)
			if fieldType == nil {
				line, col := field.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", field.Type.Name)
				fieldType = TypeInt // fallback
			}
			info.Fields[field.Name] = fieldType
			info.FieldOrder = append(info.FieldOrder, field.Name)
		}

		// Register methods
		for _, method := range entity.Methods {
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
					returnType = TypeVoid // fallback
				}
			}

			info.Methods[method.Name] = &MethodInfo{
				Name:        method.Name,
				Params:      params,
				ReturnType:  returnType,
				HasRequires: len(method.Requires) > 0,
				HasEnsures:  len(method.Ensures) > 0,
			}
		}

		c.entities[entity.Name] = info

		// Register entity in global scope
		c.scope.Define(entity.Name, &Symbol{
			Name: entity.Name,
			Type: &Type{Name: entity.Name, IsEntity: true, Entity: info},
			Kind: SymEntity,
		})
	}
}

// registerEnums registers all enums in the global scope
func (c *Checker) registerEnums() {
	for _, enum := range c.prog.Enums {
		// Check for duplicate enum name (against both entities and enums)
		if _, exists := c.entities[enum.Name]; exists {
			line, col := enum.Pos()
			c.diag.Errorf(line, col, "enum '%s' conflicts with existing entity", enum.Name)
			continue
		}
		if _, exists := c.enums[enum.Name]; exists {
			line, col := enum.Pos()
			c.diag.Errorf(line, col, "enum '%s' already defined", enum.Name)
			continue
		}

		info := &EnumInfo{
			Name:     enum.Name,
			Variants: make([]*EnumVariantInfo, 0, len(enum.Variants)),
		}

		// Track variant names for duplicate detection
		variantNames := make(map[string]bool)

		// Register variants
		for _, variant := range enum.Variants {
			if variantNames[variant.Name] {
				line, col := variant.Pos()
				c.diag.Errorf(line, col, "duplicate variant name '%s' in enum '%s'", variant.Name, enum.Name)
				continue
			}
			variantNames[variant.Name] = true

			// Resolve field types
			fields := make([]ParamInfo, 0, len(variant.Fields))
			for _, field := range variant.Fields {
				fieldType := ResolveType(field.Type, c.entities, c.enums)
				if fieldType == nil {
					line, col := field.Pos()
					c.diag.Errorf(line, col, "unknown type '%s'", field.Type.Name)
					fieldType = TypeInt // fallback
				}
				fields = append(fields, ParamInfo{Name: field.Name, Type: fieldType})
			}

			variantInfo := &EnumVariantInfo{
				Name:   variant.Name,
				Fields: fields,
			}

			info.Variants = append(info.Variants, variantInfo)

			// Register variant in variant lookup map for constructor checking
			c.enumVariants[variant.Name] = &EnumVariantLookup{
				EnumInfo:    info,
				VariantInfo: variantInfo,
			}
		}

		c.enums[enum.Name] = info

		// Register enum in global scope
		enumType := &Type{Name: enum.Name, IsEnum: true, EnumInfo: info}
		c.scope.Define(enum.Name, &Symbol{
			Name: enum.Name,
			Type: enumType,
			Kind: SymEnum,
		})
	}
}

// registerFunctions registers all functions in the global scope
func (c *Checker) registerFunctions() {
	for _, fn := range c.prog.Functions {
		if _, exists := c.functions[fn.Name]; exists {
			line, col := fn.Pos()
			c.diag.Errorf(line, col, "function '%s' already defined", fn.Name)
			continue
		}

		params := make([]ParamInfo, 0, len(fn.Params))
		for _, p := range fn.Params {
			pType := ResolveType(p.Type, c.entities, c.enums)
			if pType == nil {
				line, col := p.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", p.Type.Name)
				pType = TypeInt // fallback
			}
			params = append(params, ParamInfo{Name: p.Name, Type: pType})
		}

		returnType := TypeVoid
		if fn.ReturnType != nil {
			returnType = ResolveType(fn.ReturnType, c.entities, c.enums)
			if returnType == nil {
				line, col := fn.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", fn.ReturnType.Name)
				returnType = TypeVoid // fallback
			}
		}

		c.functions[fn.Name] = &FuncInfo{
			Name:       fn.Name,
			Params:     params,
			ReturnType: returnType,
		}

		c.scope.Define(fn.Name, &Symbol{
			Name: fn.Name,
			Type: returnType,
			Kind: SymFunction,
		})
	}
}

// checkFunctions checks all function bodies
func (c *Checker) checkFunctions() {
	for _, fn := range c.prog.Functions {
		c.checkFunction(fn)
	}
}

// checkFunction checks a single function
func (c *Checker) checkFunction(fn *ast.FunctionDecl) {
	funcScope := NewScope(c.scope)

	// Set current function context for Result/Option variant checking
	c.currentFunc = c.functions[fn.Name]

	// Add parameters to function scope
	for _, p := range fn.Params {
		pType := ResolveType(p.Type, c.entities, c.enums)
		if pType != nil {
			funcScope.Define(p.Name, &Symbol{
				Name:    p.Name,
				Type:    pType,
				Mutable: false,
				Kind:    SymParam,
			})
		}
	}

	// Check requires clauses
	oldCtx := c.contractCtx
	c.contractCtx = CtxRequires
	for _, req := range fn.Requires {
		exprType := c.checkExpression(req.Expr, funcScope)
		if exprType != nil && !exprType.Equal(TypeBool) {
			line, col := req.Pos()
			c.diag.Errorf(line, col, "requires clause must be boolean, got %s", exprType.Name)
		}
	}

	// Check ensures clauses
	c.contractCtx = CtxEnsures
	for _, ens := range fn.Ensures {
		exprType := c.checkExpression(ens.Expr, funcScope)
		if exprType != nil && !exprType.Equal(TypeBool) {
			line, col := ens.Pos()
			c.diag.Errorf(line, col, "ensures clause must be boolean, got %s", exprType.Name)
		}
	}
	c.contractCtx = oldCtx

	// Check body
	if fn.Body != nil {
		c.checkBlock(fn.Body, funcScope)
	}

	// Clear current function context
	c.currentFunc = nil
}

// checkEntities checks all entity constructors and methods
func (c *Checker) checkEntities() {
	for _, entity := range c.prog.Entities {
		info := c.entities[entity.Name]

		// Set entity context
		c.entityCtx = &EntityContext{
			Entity: info,
		}

		// Check invariants
		oldCtx := c.contractCtx
		c.contractCtx = CtxInvariant
		for _, inv := range entity.Invariants {
			entityScope := NewScope(c.scope)
			// Add 'self' to scope
			entityScope.Define("self", &Symbol{
				Name:    "self",
				Type:    &Type{Name: entity.Name, IsEntity: true, Entity: info},
				Mutable: false,
				Kind:    SymVariable,
			})

			exprType := c.checkExpression(inv.Expr, entityScope)
			if exprType != nil && !exprType.Equal(TypeBool) {
				line, col := inv.Pos()
				c.diag.Errorf(line, col, "invariant must be boolean, got %s", exprType.Name)
			}
		}
		c.contractCtx = oldCtx

		// Check constructor
		if entity.Constructor != nil {
			c.entityCtx.InConstructor = true
			c.checkConstructor(entity, entity.Constructor, info)
			c.entityCtx.InConstructor = false
		}

		// Check methods
		for _, method := range entity.Methods {
			c.entityCtx.InMethod = true
			c.checkMethod(entity, method, info)
			c.entityCtx.InMethod = false
		}

		c.entityCtx = nil
	}
}

// checkConstructor checks an entity constructor
func (c *Checker) checkConstructor(entity *ast.EntityDecl, ctor *ast.ConstructorDecl, info *EntityInfo) {
	ctorScope := NewScope(c.scope)

	// Add 'self' to constructor scope
	ctorScope.Define("self", &Symbol{
		Name:    "self",
		Type:    &Type{Name: entity.Name, IsEntity: true, Entity: info},
		Mutable: true,
		Kind:    SymVariable,
	})

	// Add parameters to constructor scope
	for _, p := range ctor.Params {
		pType := ResolveType(p.Type, c.entities, c.enums)
		if pType != nil {
			ctorScope.Define(p.Name, &Symbol{
				Name:    p.Name,
				Type:    pType,
				Mutable: false,
				Kind:    SymParam,
			})
		}
	}

	// Check requires clauses
	oldCtx := c.contractCtx
	c.contractCtx = CtxRequires
	for _, req := range ctor.Requires {
		exprType := c.checkExpression(req.Expr, ctorScope)
		if exprType != nil && !exprType.Equal(TypeBool) {
			line, col := req.Pos()
			c.diag.Errorf(line, col, "requires clause must be boolean, got %s", exprType.Name)
		}
	}

	// Check ensures clauses
	c.contractCtx = CtxEnsures
	for _, ens := range ctor.Ensures {
		exprType := c.checkExpression(ens.Expr, ctorScope)
		if exprType != nil && !exprType.Equal(TypeBool) {
			line, col := ens.Pos()
			c.diag.Errorf(line, col, "ensures clause must be boolean, got %s", exprType.Name)
		}
	}
	c.contractCtx = oldCtx

	// Check body
	if ctor.Body != nil {
		c.checkBlock(ctor.Body, ctorScope)
	}
}

// checkMethod checks an entity method
func (c *Checker) checkMethod(entity *ast.EntityDecl, method *ast.MethodDecl, info *EntityInfo) {
	methodScope := NewScope(c.scope)

	// Add 'self' to method scope
	methodScope.Define("self", &Symbol{
		Name:    "self",
		Type:    &Type{Name: entity.Name, IsEntity: true, Entity: info},
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

	// Check requires clauses
	oldCtx := c.contractCtx
	c.contractCtx = CtxRequires
	for _, req := range method.Requires {
		exprType := c.checkExpression(req.Expr, methodScope)
		if exprType != nil && !exprType.Equal(TypeBool) {
			line, col := req.Pos()
			c.diag.Errorf(line, col, "requires clause must be boolean, got %s", exprType.Name)
		}
	}

	// Check ensures clauses
	c.contractCtx = CtxEnsures
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
}

// checkBlock checks a block of statements
func (c *Checker) checkBlock(block *ast.Block, scope *Scope) {
	for _, stmt := range block.Statements {
		c.checkStatement(stmt, scope)
	}
}

// checkStatement checks a statement
func (c *Checker) checkStatement(stmt ast.Statement, scope *Scope) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		c.checkLetStmt(s, scope)
	case *ast.AssignStmt:
		c.checkAssignStmt(s, scope)
	case *ast.ReturnStmt:
		c.checkReturnStmt(s, scope)
	case *ast.IfStmt:
		c.checkIfStmt(s, scope)
	case *ast.WhileStmt:
		c.checkWhileStmt(s, scope)
	case *ast.ForInStmt:
		c.checkForInStmt(s, scope)
	case *ast.BreakStmt:
		c.checkBreakStmt(s)
	case *ast.ContinueStmt:
		c.checkContinueStmt(s)
	case *ast.ExprStmt:
		c.checkExpression(s.Expr, scope)
	case *ast.Block:
		blockScope := NewScope(scope)
		c.checkBlock(s, blockScope)
	}
}

// checkLetStmt checks a let statement
func (c *Checker) checkLetStmt(stmt *ast.LetStmt, scope *Scope) {
	// Check if already defined in this scope
	if scope.ResolveLocal(stmt.Name) != nil {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "variable '%s' already defined in this scope", stmt.Name)
		return
	}

	// Resolve declared type first
	var declaredType *Type
	if stmt.Type != nil {
		declaredType = ResolveType(stmt.Type, c.entities, c.enums)
		if declaredType == nil {
			line, col := stmt.Pos()
			c.diag.Errorf(line, col, "unknown type '%s'", stmt.Type.Name)
			return
		}
	}

	// Set letDeclaredType for Result/Option variant inference
	c.letDeclaredType = declaredType

	// Check the value expression
	valueType := c.checkExpression(stmt.Value, scope)

	// Clear letDeclaredType
	c.letDeclaredType = nil

	// Check type compatibility
	if declaredType != nil && valueType != nil {
		if !valueType.Equal(declaredType) {
			line, col := stmt.Pos()
			c.diag.Errorf(line, col, "type mismatch: cannot assign %s to %s", valueType.Name, declaredType.Name)
		}
	}

	// Use declared type if available, otherwise infer from value
	varType := declaredType
	if varType == nil {
		varType = valueType
	}

	// Add to scope
	if varType != nil {
		scope.Define(stmt.Name, &Symbol{
			Name:    stmt.Name,
			Type:    varType,
			Mutable: stmt.Mutable,
			Kind:    SymVariable,
		})
	}
}

// checkAssignStmt checks an assignment statement
func (c *Checker) checkAssignStmt(stmt *ast.AssignStmt, scope *Scope) {
	// Check target
	targetType := c.checkExpression(stmt.Target, scope)

	// Check if target is mutable
	if ident, ok := stmt.Target.(*ast.Identifier); ok {
		sym := scope.Resolve(ident.Name)
		if sym != nil && !sym.Mutable {
			line, col := stmt.Pos()
			c.diag.Errorf(line, col, "cannot assign to immutable variable '%s'", ident.Name)
		}
	}

	// Check mutability for index assignment: arr[i] = x
	if indexExpr, ok := stmt.Target.(*ast.IndexExpr); ok {
		if ident, ok := indexExpr.Object.(*ast.Identifier); ok {
			sym := scope.Resolve(ident.Name)
			if sym != nil && !sym.Mutable {
				line, col := stmt.Pos()
				c.diag.Errorf(line, col, "cannot assign to index of immutable array '%s'", ident.Name)
			}
		}
	}

	// Check value
	valueType := c.checkExpression(stmt.Value, scope)

	// Check type compatibility
	if targetType != nil && valueType != nil {
		if !valueType.Equal(targetType) {
			line, col := stmt.Pos()
			c.diag.Errorf(line, col, "type mismatch: cannot assign %s to %s", valueType.Name, targetType.Name)
		}
	}
}

// checkReturnStmt checks a return statement
func (c *Checker) checkReturnStmt(stmt *ast.ReturnStmt, scope *Scope) {
	if stmt.Value != nil {
		c.checkExpression(stmt.Value, scope)
	}
}

// checkIfStmt checks an if statement
func (c *Checker) checkIfStmt(stmt *ast.IfStmt, scope *Scope) {
	// Check condition
	condType := c.checkExpression(stmt.Condition, scope)
	if condType != nil && !condType.Equal(TypeBool) {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "if condition must be boolean, got %s", condType.Name)
	}

	// Check then block
	if stmt.Then != nil {
		thenScope := NewScope(scope)
		c.checkBlock(stmt.Then, thenScope)
	}

	// Check else block
	if stmt.Else != nil {
		elseScope := NewScope(scope)
		c.checkStatement(stmt.Else, elseScope)
	}
}

// checkWhileStmt checks a while statement
func (c *Checker) checkWhileStmt(stmt *ast.WhileStmt, scope *Scope) {
	condType := c.checkExpression(stmt.Condition, scope)
	if condType != nil && !condType.Equal(TypeBool) {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "while condition must be boolean, got %s", condType.Name)
	}

	// Validate invariant clauses -- each must be Bool expression
	// Set contractCtx to CtxInvariant so old()/result validation works
	for _, inv := range stmt.Invariants {
		oldCtx := c.contractCtx
		c.contractCtx = CtxInvariant
		invType := c.checkExpression(inv.Expr, scope)
		if invType != nil && !invType.Equal(TypeBool) {
			c.diag.Errorf(inv.Line, inv.Column,
				"loop invariant must be boolean, got %s", invType.Name)
		}
		c.contractCtx = oldCtx
	}

	// Validate decreases clause -- must be Int expression
	if stmt.Decreases != nil {
		decType := c.checkExpression(stmt.Decreases.Expr, scope)
		if decType != nil && !decType.Equal(TypeInt) {
			c.diag.Errorf(stmt.Decreases.Line, stmt.Decreases.Column,
				"decreases metric must be Int, got %s", decType.Name)
		}
	}

	c.loopDepth++
	whileScope := NewScope(scope)
	c.checkBlock(stmt.Body, whileScope)
	c.loopDepth--
}

// checkForInStmt checks a for-in statement
func (c *Checker) checkForInStmt(stmt *ast.ForInStmt, scope *Scope) {
	line, col := stmt.Pos()

	var elemType *Type

	// Check if iterable is a range expression
	if rangeExpr, ok := stmt.Iterable.(*ast.RangeExpr); ok {
		// Range iteration: both start and end must be Int
		startType := c.checkExpression(rangeExpr.Start, scope)
		endType := c.checkExpression(rangeExpr.End, scope)

		if startType != nil && !startType.Equal(TypeInt) {
			c.diag.Errorf(line, col, "range start must be Int, got %s", startType.String())
		}
		if endType != nil && !endType.Equal(TypeInt) {
			c.diag.Errorf(line, col, "range end must be Int, got %s", endType.String())
		}

		elemType = TypeInt
	} else {
		// Array iteration
		iterType := c.checkExpression(stmt.Iterable, scope)
		if iterType == nil {
			return
		}

		if iterType.Name != "Array" || !iterType.IsGeneric || len(iterType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "cannot iterate over type %s (expected Array or range)", iterType.String())
			return
		}

		elemType = iterType.TypeParams[0]
	}

	// Create loop scope with loop variable
	loopScope := NewScope(scope)
	if elemType != nil {
		loopScope.Define(stmt.Variable, &Symbol{
			Name:    stmt.Variable,
			Type:    elemType,
			Mutable: false, // loop variable is immutable
			Kind:    SymVariable,
		})
	}

	// Track loop depth for break/continue validation
	c.loopDepth++
	c.checkBlock(stmt.Body, loopScope)
	c.loopDepth--
}

// checkBreakStmt checks a break statement
func (c *Checker) checkBreakStmt(stmt *ast.BreakStmt) {
	if c.loopDepth == 0 {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "break statement outside loop")
	}
}

// checkContinueStmt checks a continue statement
func (c *Checker) checkContinueStmt(stmt *ast.ContinueStmt) {
	if c.loopDepth == 0 {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "continue statement outside loop")
	}
}

// storeExprType stores the type of an expression for later use by codegen
func (c *Checker) storeExprType(expr ast.Expression, t *Type) *Type {
	if t != nil && c.exprTypes != nil {
		c.exprTypes[expr] = t
	}
	return t
}

// checkExpression checks an expression and returns its type
func (c *Checker) checkExpression(expr ast.Expression, scope *Scope) *Type {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		return c.storeExprType(expr, c.checkBinaryExpr(e, scope))
	case *ast.UnaryExpr:
		return c.storeExprType(expr, c.checkUnaryExpr(e, scope))
	case *ast.CallExpr:
		return c.storeExprType(expr, c.checkCallExpr(e, scope))
	case *ast.MethodCallExpr:
		return c.storeExprType(expr, c.checkMethodCallExpr(e, scope))
	case *ast.FieldAccessExpr:
		return c.storeExprType(expr, c.checkFieldAccessExpr(e, scope))
	case *ast.OldExpr:
		return c.storeExprType(expr, c.checkOldExpr(e, scope))
	case *ast.Identifier:
		return c.storeExprType(expr, c.checkIdentifier(e, scope))
	case *ast.SelfExpr:
		return c.storeExprType(expr, c.checkSelfExpr(e, scope))
	case *ast.ResultExpr:
		return c.storeExprType(expr, c.checkResultExpr(e, scope))
	case *ast.IntLit:
		return c.storeExprType(expr, TypeInt)
	case *ast.FloatLit:
		return c.storeExprType(expr, TypeFloat)
	case *ast.StringLit:
		return c.storeExprType(expr, TypeString)
	case *ast.BoolLit:
		return c.storeExprType(expr, TypeBool)
	case *ast.ArrayLit:
		return c.storeExprType(expr, c.checkArrayLit(e, scope))
	case *ast.IndexExpr:
		return c.storeExprType(expr, c.checkIndexExpr(e, scope))
	case *ast.RangeExpr:
		return c.storeExprType(expr, c.checkRangeExpr(e, scope))
	case *ast.ForallExpr:
		return c.storeExprType(expr, c.checkForallExpr(e, scope))
	case *ast.ExistsExpr:
		return c.storeExprType(expr, c.checkExistsExpr(e, scope))
	case *ast.MatchExpr:
		return c.storeExprType(expr, c.checkMatchExpr(e, scope))
	case *ast.TryExpr:
		return c.storeExprType(expr, c.checkTryExpr(e, scope))
	default:
		return nil
	}
}

// checkBinaryExpr checks a binary expression
func (c *Checker) checkBinaryExpr(expr *ast.BinaryExpr, scope *Scope) *Type {
	leftType := c.checkExpression(expr.Left, scope)
	rightType := c.checkExpression(expr.Right, scope)

	if leftType == nil || rightType == nil {
		return nil
	}

	line, col := expr.Pos()

	switch expr.Op {
	case lexer.PLUS:
		// Int + Int, Float + Float, String + String
		if leftType.Equal(TypeInt) && rightType.Equal(TypeInt) {
			return TypeInt
		}
		if leftType.Equal(TypeFloat) && rightType.Equal(TypeFloat) {
			return TypeFloat
		}
		if leftType.Equal(TypeString) && rightType.Equal(TypeString) {
			return TypeString
		}
		c.diag.Errorf(line, col, "operator '+' not defined for %s and %s", leftType.Name, rightType.Name)
		return nil

	case lexer.MINUS, lexer.STAR, lexer.SLASH, lexer.PERCENT:
		// Int op Int, Float op Float
		if leftType.Equal(TypeInt) && rightType.Equal(TypeInt) {
			return TypeInt
		}
		if leftType.Equal(TypeFloat) && rightType.Equal(TypeFloat) {
			return TypeFloat
		}
		c.diag.Errorf(line, col, "operator '%s' not defined for %s and %s", expr.Op, leftType.Name, rightType.Name)
		return nil

	case lexer.EQ, lexer.NEQ:
		// Works on Int, Float, String, Bool (same types)
		if leftType.Equal(rightType) {
			if leftType.Equal(TypeInt) || leftType.Equal(TypeFloat) || leftType.Equal(TypeString) || leftType.Equal(TypeBool) {
				return TypeBool
			}
		}
		c.diag.Errorf(line, col, "operator '%s' not defined for %s and %s", expr.Op, leftType.Name, rightType.Name)
		return nil

	case lexer.LT, lexer.GT, lexer.LEQ, lexer.GEQ:
		// Works on Int, Float, String (same types)
		if leftType.Equal(rightType) {
			if leftType.Equal(TypeInt) || leftType.Equal(TypeFloat) || leftType.Equal(TypeString) {
				return TypeBool
			}
		}
		c.diag.Errorf(line, col, "operator '%s' not defined for %s and %s", expr.Op, leftType.Name, rightType.Name)
		return nil

	case lexer.AND, lexer.OR, lexer.IMPLIES:
		// Works on Bool
		if leftType.Equal(TypeBool) && rightType.Equal(TypeBool) {
			return TypeBool
		}
		c.diag.Errorf(line, col, "operator '%s' requires boolean operands, got %s and %s", expr.Op, leftType.Name, rightType.Name)
		return nil

	default:
		c.diag.Errorf(line, col, "unknown binary operator")
		return nil
	}
}

// checkUnaryExpr checks a unary expression
func (c *Checker) checkUnaryExpr(expr *ast.UnaryExpr, scope *Scope) *Type {
	operandType := c.checkExpression(expr.Operand, scope)
	if operandType == nil {
		return nil
	}

	line, col := expr.Pos()

	switch expr.Op {
	case lexer.MINUS:
		if operandType.Equal(TypeInt) || operandType.Equal(TypeFloat) {
			return operandType
		}
		c.diag.Errorf(line, col, "unary '-' not defined for %s", operandType.Name)
		return nil

	case lexer.NOT:
		if operandType.Equal(TypeBool) {
			return TypeBool
		}
		c.diag.Errorf(line, col, "unary 'not' requires boolean operand, got %s", operandType.Name)
		return nil

	default:
		c.diag.Errorf(line, col, "unknown unary operator")
		return nil
	}
}

// checkCallExpr checks a function call or entity construction
func (c *Checker) checkCallExpr(expr *ast.CallExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Handle print() built-in
	if expr.Function == "print" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "print() expects 1 argument, got %d", len(expr.Args))
			return TypeVoid
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil {
			if !argType.Equal(TypeInt) && !argType.Equal(TypeFloat) &&
				!argType.Equal(TypeBool) && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "print() cannot print type %s (accepts Int, Float, Bool, String)", argType.Name)
			}
		}
		return TypeVoid
	}

	// Handle len() built-in
	if expr.Function == "len" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "len() requires exactly 1 argument, got %d", len(expr.Args))
			return TypeInt
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil {
			if argType.Name != "Array" || !argType.IsGeneric {
				c.diag.Errorf(line, col, "len() requires Array argument, got %s", argType.String())
			}
		}
		return TypeInt
	}

	// Check if it's a built-in Result/Option variant constructor (Ok, Err, Some)
	if expr.Function == "Ok" || expr.Function == "Err" || expr.Function == "Some" {
		return c.checkBuiltinVariant(expr, scope)
	}

	// Check if it's a variant constructor (enum variant with data)
	if lookup, exists := c.enumVariants[expr.Function]; exists {
		variant := lookup.VariantInfo
		// Check argument count matches field count
		if len(expr.Args) != len(variant.Fields) {
			c.diag.Errorf(line, col, "variant '%s' expects %d arguments, got %d",
				expr.Function, len(variant.Fields), len(expr.Args))
		}
		// Check argument types match field types
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if i < len(variant.Fields) && argType != nil && !argType.Equal(variant.Fields[i].Type) {
				argLine, argCol := arg.Pos()
				c.diag.Errorf(argLine, argCol, "variant '%s' field '%s' expects %s, got %s",
					expr.Function, variant.Fields[i].Name, variant.Fields[i].Type.String(), argType.String())
			}
		}
		return &Type{Name: lookup.EnumInfo.Name, IsEnum: true, EnumInfo: lookup.EnumInfo}
	}

	// Check if it's an entity constructor
	if entity, exists := c.entities[expr.Function]; exists {
		if !entity.HasConstructor {
			c.diag.Errorf(line, col, "entity '%s' has no constructor", expr.Function)
			return nil
		}

		// For now, we don't track constructor parameter info separately
		// In a full implementation, we'd check argument types here

		return &Type{Name: expr.Function, IsEntity: true, Entity: entity}
	}

	// Check if it's a function call
	fn, exists := c.functions[expr.Function]
	if !exists {
		c.diag.Errorf(line, col, "unknown function '%s'", expr.Function)
		return nil
	}

	// Check argument count
	if len(expr.Args) != len(fn.Params) {
		c.diag.Errorf(line, col, "function '%s' expects %d arguments, got %d",
			expr.Function, len(fn.Params), len(expr.Args))
		return fn.ReturnType
	}

	// Check argument types
	for i, arg := range expr.Args {
		argType := c.checkExpression(arg, scope)
		if argType != nil && !argType.Equal(fn.Params[i].Type) {
			argLine, argCol := arg.Pos()
			c.diag.Errorf(argLine, argCol, "argument %d to '%s': expected %s, got %s",
				i+1, expr.Function, fn.Params[i].Type.Name, argType.Name)
		}
	}

	return fn.ReturnType
}

// checkBuiltinVariant checks built-in Result/Option variant constructors (Ok, Err, Some)
func (c *Checker) checkBuiltinVariant(expr *ast.CallExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Try to infer the expected type from context
	var expectedType *Type

	// First, check if we're in a function with a Result/Option return type
	if c.currentFunc != nil && c.currentFunc.ReturnType != nil {
		returnType := c.currentFunc.ReturnType
		if returnType.Name == "Result" || returnType.Name == "Option" {
			expectedType = returnType
		}
	}

	// If not, check if we have a let declaration type annotation
	if expectedType == nil && c.letDeclaredType != nil {
		if c.letDeclaredType.Name == "Result" || c.letDeclaredType.Name == "Option" {
			expectedType = c.letDeclaredType
		}
	}

	// Handle each variant
	switch expr.Function {
	case "Ok":
		// Ok requires exactly 1 argument and Result<T, E> context
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "Ok() expects 1 argument, got %d", len(expr.Args))
			return nil
		}
		if expectedType == nil || expectedType.Name != "Result" {
			c.diag.Errorf(line, col, "Ok() requires explicit Result<T,E> type annotation or Result return type")
			return nil
		}
		// Check argument type matches T (first type param)
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && len(expectedType.TypeParams) >= 1 {
			if !argType.Equal(expectedType.TypeParams[0]) {
				c.diag.Errorf(line, col, "Ok() argument type mismatch: expected %s, got %s",
					expectedType.TypeParams[0].String(), argType.String())
			}
		}
		return expectedType

	case "Err":
		// Err requires exactly 1 argument and Result<T, E> context
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "Err() expects 1 argument, got %d", len(expr.Args))
			return nil
		}
		if expectedType == nil || expectedType.Name != "Result" {
			c.diag.Errorf(line, col, "Err() requires explicit Result<T,E> type annotation or Result return type")
			return nil
		}
		// Check argument type matches E (second type param)
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && len(expectedType.TypeParams) >= 2 {
			if !argType.Equal(expectedType.TypeParams[1]) {
				c.diag.Errorf(line, col, "Err() argument type mismatch: expected %s, got %s",
					expectedType.TypeParams[1].String(), argType.String())
			}
		}
		return expectedType

	case "Some":
		// Some requires exactly 1 argument and Option<T> context
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "Some() expects 1 argument, got %d", len(expr.Args))
			return nil
		}
		if expectedType == nil || expectedType.Name != "Option" {
			c.diag.Errorf(line, col, "Some() requires explicit Option<T> type annotation or Option return type")
			return nil
		}
		// Check argument type matches T (first type param)
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && len(expectedType.TypeParams) >= 1 {
			if !argType.Equal(expectedType.TypeParams[0]) {
				c.diag.Errorf(line, col, "Some() argument type mismatch: expected %s, got %s",
					expectedType.TypeParams[0].String(), argType.String())
			}
		}
		return expectedType
	}

	return nil
}

// checkMethodCallExpr checks a method call
func (c *Checker) checkMethodCallExpr(expr *ast.MethodCallExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Check if this is a module-qualified call (e.g., math.add(1, 2))
	if ident, ok := expr.Object.(*ast.Identifier); ok && c.moduleImports != nil {
		if modSyms, isModule := c.moduleImports[ident.Name]; isModule {
			return c.checkModuleQualifiedCall(expr, modSyms, ident.Name, line, col, scope)
		}
		// Check if it's a module that exists but wasn't imported
		// (We can't easily know all modules, so we skip this for now)
	}

	// Check object type
	objType := c.checkExpression(expr.Object, scope)
	if objType == nil {
		return nil
	}

	// Handle Array methods
	if objType.Name == "Array" && objType.IsGeneric {
		switch expr.Method {
		case "push":
			if len(expr.Args) != 1 {
				c.diag.Errorf(line, col, "push() requires exactly 1 argument, got %d", len(expr.Args))
				return TypeVoid
			}
			// Check mutability: the object must be a mutable variable
			if ident, ok := expr.Object.(*ast.Identifier); ok {
				sym := scope.Resolve(ident.Name)
				if sym != nil && !sym.Mutable {
					c.diag.Errorf(line, col, "cannot call push() on immutable array '%s'", ident.Name)
				}
			}
			// Check element type matches
			argType := c.checkExpression(expr.Args[0], scope)
			if argType != nil && len(objType.TypeParams) == 1 {
				if !argType.Equal(objType.TypeParams[0]) {
					c.diag.Errorf(line, col, "push() argument type mismatch: expected %s, got %s",
						objType.TypeParams[0].String(), argType.String())
				}
			}
			return TypeVoid
		default:
			c.diag.Errorf(line, col, "Array has no method '%s'", expr.Method)
			return nil
		}
	}

	// Handle Result predicate methods
	if objType.IsEnum && objType.Name == "Result" {
		switch expr.Method {
		case "is_ok", "is_err":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "%s() requires no arguments, got %d", expr.Method, len(expr.Args))
			}
			return TypeBool
		}
	}

	// Handle Option predicate methods
	if objType.IsEnum && objType.Name == "Option" {
		switch expr.Method {
		case "is_some", "is_none":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "%s() requires no arguments, got %d", expr.Method, len(expr.Args))
			}
			return TypeBool
		}
	}

	if !objType.IsEntity {
		c.diag.Errorf(line, col, "cannot call method on non-entity type %s", objType.Name)
		return nil
	}

	// Check if method exists
	method, exists := objType.Entity.Methods[expr.Method]
	if !exists {
		c.diag.Errorf(line, col, "entity '%s' has no method '%s'", objType.Name, expr.Method)
		return nil
	}

	// Check argument count
	if len(expr.Args) != len(method.Params) {
		c.diag.Errorf(line, col, "method '%s' expects %d arguments, got %d",
			expr.Method, len(method.Params), len(expr.Args))
		return method.ReturnType
	}

	// Check argument types
	for i, arg := range expr.Args {
		argType := c.checkExpression(arg, scope)
		if argType != nil && !argType.Equal(method.Params[i].Type) {
			argLine, argCol := arg.Pos()
			c.diag.Errorf(argLine, argCol, "argument %d to method '%s': expected %s, got %s",
				i+1, expr.Method, method.Params[i].Type.Name, argType.Name)
		}
	}

	return method.ReturnType
}

// checkModuleQualifiedCall checks a module-qualified function call or entity constructor
// e.g., math.add(1, 2) or geometry.Circle(5.0)
func (c *Checker) checkModuleQualifiedCall(expr *ast.MethodCallExpr, modSyms *ModuleSymbols, moduleName string, line, col int, scope *Scope) *Type {
	symbolName := expr.Method

	// Check if it's a function call
	if fn, ok := modSyms.Functions[symbolName]; ok {
		// Check argument count
		if len(expr.Args) != len(fn.Params) {
			c.diag.Errorf(line, col, "function '%s.%s' expects %d arguments, got %d",
				moduleName, symbolName, len(fn.Params), len(expr.Args))
			return fn.ReturnType
		}

		// Check argument types
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(fn.Params[i].Type) {
				argLine, argCol := arg.Pos()
				c.diag.Errorf(argLine, argCol, "argument %d to '%s.%s': expected %s, got %s",
					i+1, moduleName, symbolName, fn.Params[i].Type.Name, argType.Name)
			}
		}

		return fn.ReturnType
	}

	// Check if it's an entity constructor
	if entity, ok := modSyms.Entities[symbolName]; ok {
		if !entity.HasConstructor {
			c.diag.Errorf(line, col, "entity '%s.%s' has no constructor", moduleName, symbolName)
			return nil
		}
		return &Type{Name: symbolName, IsEntity: true, Entity: entity}
	}

	// Check if it's an enum variant constructor -- not typical with qualified name but handle it
	// (usually enums are referenced directly)

	// Symbol not found in module
	c.diag.Errorf(line, col, "symbol '%s' is not exported from module '%s'", symbolName, moduleName)
	return nil
}

// checkFieldAccessExpr checks a field access
func (c *Checker) checkFieldAccessExpr(expr *ast.FieldAccessExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Check if this is a module-qualified access (e.g., math.CONSTANT)
	if ident, ok := expr.Object.(*ast.Identifier); ok && c.moduleImports != nil {
		if _, isModule := c.moduleImports[ident.Name]; isModule {
			// Module-qualified field access is not supported (functions need call syntax)
			c.diag.Errorf(line, col, "cannot access '%s' from module '%s' without calling it", expr.Field, ident.Name)
			return nil
		}
	}

	// Check object type
	objType := c.checkExpression(expr.Object, scope)
	if objType == nil {
		return nil
	}

	if !objType.IsEntity {
		c.diag.Errorf(line, col, "cannot access field on non-entity type %s", objType.Name)
		return nil
	}

	// Check if field exists
	fieldType, exists := objType.Entity.Fields[expr.Field]
	if !exists {
		c.diag.Errorf(line, col, "entity '%s' has no field '%s'", objType.Name, expr.Field)
		return nil
	}

	return fieldType
}

// checkOldExpr checks an old() expression
func (c *Checker) checkOldExpr(expr *ast.OldExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// old() is only valid in ensures clauses and loop invariants
	if c.contractCtx != CtxEnsures && c.contractCtx != CtxInvariant {
		c.diag.Errorf(line, col, "'old()' can only be used in ensures clauses and loop invariants")
	}

	return c.checkExpression(expr.Expr, scope)
}

// checkIdentifier checks an identifier
func (c *Checker) checkIdentifier(expr *ast.Identifier, scope *Scope) *Type {
	line, col := expr.Pos()

	sym := scope.Resolve(expr.Name)
	if sym == nil {
		// Check if it's "None" (built-in Option unit variant)
		if expr.Name == "None" {
			// Try to infer Option type from context
			var expectedType *Type
			if c.currentFunc != nil && c.currentFunc.ReturnType != nil && c.currentFunc.ReturnType.Name == "Option" {
				expectedType = c.currentFunc.ReturnType
			} else if c.letDeclaredType != nil && c.letDeclaredType.Name == "Option" {
				expectedType = c.letDeclaredType
			}
			if expectedType == nil {
				c.diag.Errorf(line, col, "None requires explicit Option<T> type annotation or Option return type")
				return nil
			}
			return expectedType
		}

		// Check if it's a unit variant (enum variant with no fields)
		if lookup, exists := c.enumVariants[expr.Name]; exists && len(lookup.VariantInfo.Fields) == 0 {
			return &Type{Name: lookup.EnumInfo.Name, IsEnum: true, EnumInfo: lookup.EnumInfo}
		}
		c.diag.Errorf(line, col, "undeclared variable '%s'", expr.Name)
		return nil
	}

	return sym.Type
}

// checkSelfExpr checks a self expression
func (c *Checker) checkSelfExpr(expr *ast.SelfExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// self is only valid in entity context
	if c.entityCtx == nil {
		c.diag.Errorf(line, col, "'self' can only be used inside entity constructors, methods, or invariants")
		return nil
	}

	return &Type{
		Name:     c.entityCtx.Entity.Name,
		IsEntity: true,
		Entity:   c.entityCtx.Entity,
	}
}

// checkResultExpr checks a result expression
func (c *Checker) checkResultExpr(expr *ast.ResultExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// result is only valid in ensures clauses
	if c.contractCtx != CtxEnsures {
		c.diag.Errorf(line, col, "'result' can only be used in ensures clauses")
		return nil
	}

	// Return the function's return type
	if c.currentFunc != nil && c.currentFunc.ReturnType != nil {
		return c.currentFunc.ReturnType
	}

	// Fallback for backward compatibility (shouldn't happen in practice)
	return TypeInt
}

// checkArrayLit checks an array literal
func (c *Checker) checkArrayLit(lit *ast.ArrayLit, scope *Scope) *Type {
	line, col := lit.Pos()

	if len(lit.Elements) == 0 {
		c.diag.Errorf(line, col, "empty array literal requires type annotation (element type cannot be inferred)")
		return nil
	}

	// Infer element type from first element
	firstType := c.checkExpression(lit.Elements[0], scope)
	if firstType == nil {
		return nil
	}

	// Validate all elements have same type
	for i := 1; i < len(lit.Elements); i++ {
		elemType := c.checkExpression(lit.Elements[i], scope)
		if elemType == nil {
			continue
		}
		if !elemType.Equal(firstType) {
			elemLine, elemCol := lit.Elements[i].Pos()
			c.diag.Errorf(elemLine, elemCol,
				"array element type mismatch: expected %s, got %s", firstType.String(), elemType.String())
		}
	}

	return &Type{
		Name:       "Array",
		IsGeneric:  true,
		TypeParams: []*Type{firstType},
	}
}

// checkIndexExpr checks an index expression
func (c *Checker) checkIndexExpr(expr *ast.IndexExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	objType := c.checkExpression(expr.Object, scope)
	if objType == nil {
		return nil
	}

	// Object must be Array<T>
	if objType.Name != "Array" || !objType.IsGeneric || len(objType.TypeParams) != 1 {
		c.diag.Errorf(line, col, "cannot index into non-array type %s", objType.String())
		return nil
	}

	// Index must be Int
	indexType := c.checkExpression(expr.Index, scope)
	if indexType != nil && !indexType.Equal(TypeInt) {
		c.diag.Errorf(line, col, "array index must be Int, got %s", indexType.String())
	}

	// Return element type (T from Array<T>)
	return objType.TypeParams[0]
}

// checkRangeExpr checks a range expression (start..end)
func (c *Checker) checkRangeExpr(expr *ast.RangeExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Range expressions are primarily used in for-in loops
	// Type-check start and end
	startType := c.checkExpression(expr.Start, scope)
	endType := c.checkExpression(expr.End, scope)

	if startType != nil && !startType.Equal(TypeInt) {
		c.diag.Errorf(line, col, "range start must be Int, got %s", startType.String())
	}
	if endType != nil && !endType.Equal(TypeInt) {
		c.diag.Errorf(line, col, "range end must be Int, got %s", endType.String())
	}

	// Range itself doesn't have a simple type -- it's a special expression
	// Return Int as a placeholder since ranges produce Int values in iteration
	return TypeInt
}

// checkForallExpr checks a forall quantifier expression
func (c *Checker) checkForallExpr(expr *ast.ForallExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Quantifiers only valid in contract contexts
	if c.contractCtx == CtxNormal {
		c.diag.Errorf(line, col,
			"forall quantifier only allowed in contract expressions (requires, ensures, invariant)")
		return TypeBool
	}

	// Validate domain is bounded range (RangeExpr)
	if expr.Domain == nil {
		c.diag.Errorf(line, col,
			"forall requires bounded range domain")
		return TypeBool
	}

	// Check range bounds are Int
	startType := c.checkExpression(expr.Domain.Start, scope)
	endType := c.checkExpression(expr.Domain.End, scope)
	if startType != nil && !startType.Equal(TypeInt) {
		c.diag.Errorf(line, col,
			"quantifier range start must be Int, got %s", startType.String())
	}
	if endType != nil && !endType.Equal(TypeInt) {
		c.diag.Errorf(line, col,
			"quantifier range end must be Int, got %s", endType.String())
	}

	// Create scope with bound variable
	quantScope := NewScope(scope)
	quantScope.Define(expr.Variable, &Symbol{
		Name:    expr.Variable,
		Type:    TypeInt,
		Mutable: false,
		Kind:    SymVariable,
	})

	// Check body is boolean
	bodyType := c.checkExpression(expr.Body, quantScope)
	if bodyType != nil && !bodyType.Equal(TypeBool) {
		c.diag.Errorf(line, col,
			"forall body must be boolean, got %s", bodyType.String())
	}

	return TypeBool
}

// checkExistsExpr checks an exists quantifier expression
func (c *Checker) checkExistsExpr(expr *ast.ExistsExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Quantifiers only valid in contract contexts
	if c.contractCtx == CtxNormal {
		c.diag.Errorf(line, col,
			"exists quantifier only allowed in contract expressions (requires, ensures, invariant)")
		return TypeBool
	}

	// Validate domain is bounded range (RangeExpr)
	if expr.Domain == nil {
		c.diag.Errorf(line, col,
			"exists requires bounded range domain")
		return TypeBool
	}

	// Check range bounds are Int
	startType := c.checkExpression(expr.Domain.Start, scope)
	endType := c.checkExpression(expr.Domain.End, scope)
	if startType != nil && !startType.Equal(TypeInt) {
		c.diag.Errorf(line, col,
			"quantifier range start must be Int, got %s", startType.String())
	}
	if endType != nil && !endType.Equal(TypeInt) {
		c.diag.Errorf(line, col,
			"quantifier range end must be Int, got %s", endType.String())
	}

	// Create scope with bound variable
	quantScope := NewScope(scope)
	quantScope.Define(expr.Variable, &Symbol{
		Name:    expr.Variable,
		Type:    TypeInt,
		Mutable: false,
		Kind:    SymVariable,
	})

	// Check body is boolean
	bodyType := c.checkExpression(expr.Body, quantScope)
	if bodyType != nil && !bodyType.Equal(TypeBool) {
		c.diag.Errorf(line, col,
			"exists body must be boolean, got %s", bodyType.String())
	}

	return TypeBool
}

// checkMatchExpr checks a match expression for type correctness and exhaustiveness
func (c *Checker) checkMatchExpr(expr *ast.MatchExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Check scrutinee type
	scrutineeType := c.checkExpression(expr.Scrutinee, scope)
	if scrutineeType == nil {
		return nil
	}

	// Verify scrutinee is an enum type
	if !scrutineeType.IsEnum {
		c.diag.Errorf(line, col, "match scrutinee must be an enum type, got %s", scrutineeType.String())
		return nil
	}

	// Track covered variants for exhaustiveness checking
	coveredVariants := make(map[string]bool)
	hasWildcard := false
	var resultType *Type

	// Process each match arm
	for i, arm := range expr.Arms {
		armLine, armCol := arm.Pos()

		// Check for unreachable arms after wildcard
		if hasWildcard {
			c.diag.Errorf(armLine, armCol, "unreachable pattern after wildcard '_'")
			continue
		}

		var armType *Type

		if arm.Pattern.IsWildcard {
			hasWildcard = true
			// Check body expression in current scope (no bindings for wildcard)
			armType = c.checkExpression(arm.Body, scope)
		} else {
			// Validate variant exists in enum
			variantInfo := c.findEnumVariant(scrutineeType.EnumInfo, arm.Pattern.VariantName)
			if variantInfo == nil {
				patternLine, patternCol := arm.Pattern.Pos()
				c.diag.Errorf(patternLine, patternCol,
					"variant '%s' is not a variant of enum '%s'",
					arm.Pattern.VariantName, scrutineeType.Name)
				continue
			}

			// Check for duplicate variant in match
			if coveredVariants[arm.Pattern.VariantName] {
				patternLine, patternCol := arm.Pattern.Pos()
				c.diag.Errorf(patternLine, patternCol,
					"duplicate match arm for variant '%s'", arm.Pattern.VariantName)
			}
			coveredVariants[arm.Pattern.VariantName] = true

			// Check binding count matches field count
			if len(arm.Pattern.Bindings) != len(variantInfo.Fields) {
				patternLine, patternCol := arm.Pattern.Pos()
				c.diag.Errorf(patternLine, patternCol,
					"variant '%s' has %d fields but pattern has %d bindings",
					arm.Pattern.VariantName, len(variantInfo.Fields), len(arm.Pattern.Bindings))
			}

			// Create arm scope with pattern bindings
			armScope := NewScope(scope)
			for j, binding := range arm.Pattern.Bindings {
				if j < len(variantInfo.Fields) {
					armScope.Define(binding, &Symbol{
						Name:    binding,
						Type:    variantInfo.Fields[j].Type,
						Mutable: false,
						Kind:    SymVariable,
					})
				}
			}

			// Check body expression in arm scope
			armType = c.checkExpression(arm.Body, armScope)
		}

		// Arm type consistency: all arms must return same type
		if i == 0 {
			resultType = armType
		} else if armType != nil && resultType != nil && !armType.Equal(resultType) {
			c.diag.Errorf(armLine, armCol,
				"match arm type mismatch: expected %s, got %s",
				resultType.String(), armType.String())
		}
	}

	// Check exhaustiveness (only if no wildcard)
	if !hasWildcard {
		var missing []string
		for _, v := range scrutineeType.EnumInfo.Variants {
			if !coveredVariants[v.Name] {
				missing = append(missing, v.Name)
			}
		}
		if len(missing) > 0 {
			c.diag.Errorf(line, col,
				"non-exhaustive match on enum '%s': missing variants: %s",
				scrutineeType.Name, strings.Join(missing, ", "))
		}
	}

	return resultType
}

// checkTryExpr checks a try expression (expr?)
func (c *Checker) checkTryExpr(expr *ast.TryExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Check the inner expression type
	innerType := c.checkExpression(expr.Expr, scope)
	if innerType == nil {
		return nil
	}

	// Verify innerType is Result or Option
	if !innerType.IsEnum || (innerType.Name != "Result" && innerType.Name != "Option") {
		c.diag.Errorf(line, col, "try operator (?) requires Result or Option type, got %s", innerType.String())
		return nil
	}

	// Verify enclosing function exists
	if c.currentFunc == nil {
		c.diag.Errorf(line, col, "try operator (?) can only be used inside a function")
		return nil
	}

	// Verify enclosing function returns compatible type
	funcRetType := c.currentFunc.ReturnType
	if funcRetType == nil {
		c.diag.Errorf(line, col, "try operator (?) cannot be used in a function with no return type")
		return nil
	}

	if innerType.Name == "Result" {
		// Function must return Result
		if !funcRetType.IsEnum || funcRetType.Name != "Result" {
			c.diag.Errorf(line, col, "try operator (?) on Result can only be used in a function returning Result<T,E>")
			return nil
		}

		// Error types must match (TypeParams[1])
		if len(innerType.TypeParams) < 2 || len(funcRetType.TypeParams) < 2 {
			c.diag.Errorf(line, col, "Result type must have 2 type parameters")
			return nil
		}

		innerErrType := innerType.TypeParams[1]
		funcErrType := funcRetType.TypeParams[1]
		if !innerErrType.Equal(funcErrType) {
			c.diag.Errorf(line, col, "try operator (?) error type mismatch: function returns Result<_,%s> but expression is Result<_,%s>",
				funcErrType.String(), innerErrType.String())
			return nil
		}

		// Return the success type T (TypeParams[0])
		return innerType.TypeParams[0]
	}

	if innerType.Name == "Option" {
		// Function must return Option
		if !funcRetType.IsEnum || funcRetType.Name != "Option" {
			c.diag.Errorf(line, col, "try operator (?) on Option can only be used in a function returning Option<T>")
			return nil
		}

		// Return the success type T (TypeParams[0])
		if len(innerType.TypeParams) < 1 {
			c.diag.Errorf(line, col, "Option type must have 1 type parameter")
			return nil
		}
		return innerType.TypeParams[0]
	}

	return nil
}

// findEnumVariant finds a variant by name in an enum
func (c *Checker) findEnumVariant(enumInfo *EnumInfo, variantName string) *EnumVariantInfo {
	if enumInfo == nil {
		return nil
	}
	for _, v := range enumInfo.Variants {
		if v.Name == variantName {
			return v
		}
	}
	return nil
}
