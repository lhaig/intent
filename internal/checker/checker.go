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
	Traits    map[string]*TraitInfo
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
	traits       map[string]*TraitInfo
	implOrigins  map[string]string // "Entity.Method" -> "Trait" for codegen
	scope        *Scope
	exprTypes    map[ast.Expression]*Type

	// Context tracking
	contractCtx     ContractContext
	entityCtx       *EntityContext
	loopDepth       int
	currentFunc     *FuncInfo // Track current function for Result/Option variant inference
	letDeclaredType *Type     // Track type annotation from let statement for variant inference
	inAsyncFunc     bool      // Track whether we're inside an async function (or async test)
	inTest          bool      // Track whether we're inside a test body (phase 16)
	currentTestName string    // Current test name, for diagnostics
	testSawAwait    bool      // Set by checkAwaitExpr; checked after async test bodies

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
	Name           string
	Params         []ParamInfo
	ReturnType     *Type
	TypeParamNames []string // non-empty for generic functions
	IsAsync        bool     // true for async functions
}

// CheckResult holds the results of type checking for use by later pipeline stages
type CheckResult struct {
	Diagnostics *diagnostic.Diagnostics
	ExprTypes   map[ast.Expression]*Type
	Entities    map[string]*EntityInfo
	Enums       map[string]*EnumInfo
	Traits      map[string]*TraitInfo
	ImplOrigins map[string]string // "Entity.Method" -> "Trait"
	Functions   map[string]*FuncInfo
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
		traits:       make(map[string]*TraitInfo),
		implOrigins:  make(map[string]string),
		scope:        NewScope(nil),
		contractCtx:  CtxNormal,
		entityCtx:    nil,
		exprTypes:    make(map[ast.Expression]*Type),
	}

	c.registerEnums()
	c.registerEntities()
	c.registerTraits()
	c.registerFunctions()
	c.checkImplBlocks()
	c.checkFunctions()
	c.checkEntities()
	c.checkImplBlockBodies()
	c.checkTests()
	c.verifyIntents()

	return &CheckResult{
		Diagnostics: c.diag,
		ExprTypes:   c.exprTypes,
		Entities:    c.entities,
		Enums:       c.enums,
		Traits:      c.traits,
		ImplOrigins: c.implOrigins,
		Functions:   c.functions,
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
	Traits      map[string]*TraitInfo
	ImplOrigins map[string]string
	Functions   map[string]*FuncInfo
}

// isFileInPackage checks if a file path belongs to a package directory.
// When a resolved package directory is provided, the file must be directly
// inside that directory. Otherwise, falls back to matching the parent
// directory name against the package name.
func isFileInPackage(filePath, pkgName string, packageDirs map[string]string) bool {
	if pkgDir, ok := packageDirs[pkgName]; ok {
		cleanFile := filepath.Clean(filePath)
		cleanDir := filepath.Clean(pkgDir)
		return filepath.Dir(cleanFile) == cleanDir
	}
	dir := filepath.Base(filepath.Dir(filepath.Clean(filePath)))
	return dir == pkgName
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
func CheckAll(registry map[string]*ast.Program, sortedPaths []string, packageDirs map[string]string) *CheckAllResult {
	pkgDirs := map[string]string{}
	if packageDirs != nil {
		pkgDirs = packageDirs
	}

	diag := diagnostic.New()
	allExprTypes := make(map[ast.Expression]*Type)
	allEntities := make(map[string]*EntityInfo)
	allEnums := make(map[string]*EnumInfo)
	allTraits := make(map[string]*TraitInfo)
	allImplOrigins := make(map[string]string)
	allFunctions := make(map[string]*FuncInfo)

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
			Traits:        make(map[string]*TraitInfo),
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
			traits:       make(map[string]*TraitInfo),
			implOrigins:  make(map[string]string),
			scope:        NewScope(nil),
			contractCtx:  CtxNormal,
			exprTypes:    make(map[ast.Expression]*Type),
		}

		// Inject already-collected symbols from this module's imports so that
		// type resolution works for things like Array<NodeAttr> where NodeAttr
		// comes from an imported module. sortedPaths is in dependency order,
		// so imported modules have already been processed.
		for _, imp := range prog.Imports {
			var importNames []string
			if imp.IsPackage {
				// Package import: find all module names for this package
				pkgBase := strings.SplitN(imp.PackageName, ".", 2)[0]
				for symName := range publicSymbols {
					if symName == pkgBase {
						importNames = append(importNames, symName)
					}
				}
				// Also check individual module names that belong to this package
				for _, sp := range sortedPaths {
					if !isFileInPackage(sp, pkgBase, pkgDirs) {
						continue
					}
					mn := moduleNameFromPath(sp)
					if _, ok := publicSymbols[mn]; ok {
						importNames = append(importNames, mn)
					}
				}
			} else {
				importNames = []string{strings.TrimSuffix(filepath.Base(imp.Path), ".intent")}
			}
			for _, importedModName := range importNames {
				if syms, ok := publicSymbols[importedModName]; ok {
					for name, entityInfo := range syms.Entities {
						if _, exists := tmpChecker.entities[name]; !exists {
							tmpChecker.entities[name] = entityInfo
						}
					}
					for name, enumInfo := range syms.Enums {
						if _, exists := tmpChecker.enums[name]; !exists {
							tmpChecker.enums[name] = enumInfo
							for _, variant := range enumInfo.Variants {
								if _, exists := tmpChecker.enumVariants[variant.Name]; !exists {
									tmpChecker.enumVariants[variant.Name] = &EnumVariantLookup{
										EnumInfo:    enumInfo,
										VariantInfo: variant,
									}
								}
							}
						}
					}
					for name, traitInfo := range syms.Traits {
						if _, exists := tmpChecker.traits[name]; !exists {
							tmpChecker.traits[name] = traitInfo
						}
					}
				}
			}
		}

		tmpChecker.registerEnums()
		tmpChecker.registerEntities()
		tmpChecker.registerTraits()
		tmpChecker.checkImplBlocks()
		tmpChecker.registerFunctions()

		// Collect public traits
		for _, trait := range prog.Traits {
			if trait.IsPublic {
				if ti, ok := tmpChecker.traits[trait.Name]; ok {
					modSyms.Traits[trait.Name] = ti
				}
			}
		}

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
		// Also register by declared module name (e.g., "attractor_validation" vs file "validation")
		if prog.Module != nil && prog.Module.Name != "" && prog.Module.Name != modName {
			publicSymbols[prog.Module.Name] = modSyms
		}
	}

	// Pass 2: Type-check each file with cross-file context
	for _, filePath := range sortedPaths {
		prog := registry[filePath]
		if prog == nil {
			continue
		}

		// Build moduleImports for this file: for each import, look up the module's public symbols
		// Register by both file-derived name and declared module name
		moduleImports := make(map[string]*ModuleSymbols)
		for _, imp := range prog.Imports {
			if imp.IsPackage {
				// Package import: merge all public symbols from package modules
				// under the package name (e.g., "types_pkg")
				pkgBase := strings.SplitN(imp.PackageName, ".", 2)[0]
				merged := &ModuleSymbols{
					Functions:     make(map[string]*FuncInfo),
					Entities:      make(map[string]*EntityInfo),
					Enums:         make(map[string]*EnumInfo),
					Traits:        make(map[string]*TraitInfo),
					FunctionDecls: make(map[string]*ast.FunctionDecl),
					EntityDecls:   make(map[string]*ast.EntityDecl),
					EnumDecls:     make(map[string]*ast.EnumDecl),
				}
				// Collect symbols from all modules that might belong to this package
				for symName, syms := range publicSymbols {
					// Include the package-level symbols and any individual modules
					if symName == pkgBase {
						mergeModuleSymbols(merged, syms)
					}
				}
				// Also check each sorted path for files belonging to this package
				for _, sp := range sortedPaths {
					if !isFileInPackage(sp, pkgBase, pkgDirs) {
						continue
					}
					mn := moduleNameFromPath(sp)
					if syms, ok := publicSymbols[mn]; ok {
						mergeModuleSymbols(merged, syms)
					}
				}
				moduleImports[pkgBase] = merged
			} else {
				importedModName := strings.TrimSuffix(filepath.Base(imp.Path), ".intent")
				if syms, ok := publicSymbols[importedModName]; ok {
					moduleImports[importedModName] = syms
					// Also look up the declared module name from the imported program
					importedProg := registry[imp.Path]
					if importedProg == nil {
						// Try resolving relative to current file directory
						for regPath, regProg := range registry {
							if strings.TrimSuffix(filepath.Base(regPath), ".intent") == importedModName {
								importedProg = regProg
								break
							}
						}
					}
					if importedProg != nil && importedProg.Module != nil && importedProg.Module.Name != importedModName {
						moduleImports[importedProg.Module.Name] = syms
					}
				}
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
			traits:        make(map[string]*TraitInfo),
			implOrigins:   make(map[string]string),
			scope:         NewScope(nil),
			contractCtx:   CtxNormal,
			exprTypes:     make(map[ast.Expression]*Type),
			moduleImports: moduleImports,
			moduleFile:    filePath,
		}

		// Inject imported public symbols BEFORE registration so that
		// type resolution in registerEntities/registerFunctions can find
		// imported types (e.g., Array<NodeAttr> where NodeAttr is imported,
		// or type annotations like `let c: Circle` where Circle is from another module)
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
					// Also register enum variants so bare variant names resolve
					for _, variant := range enumInfo.Variants {
						if _, exists := c.enumVariants[variant.Name]; !exists {
							c.enumVariants[variant.Name] = &EnumVariantLookup{
								EnumInfo:    enumInfo,
								VariantInfo: variant,
							}
						}
					}
				}
			}
			// Also inject imported functions so cross-module function calls resolve
			for name, funcInfo := range modSyms.Functions {
				if _, exists := c.functions[name]; !exists {
					c.functions[name] = funcInfo
				}
			}
		}

		// Also inject imported traits
		for _, modSyms := range moduleImports {
			for name, traitInfo := range modSyms.Traits {
				if _, exists := c.traits[name]; !exists {
					c.traits[name] = traitInfo
				}
			}
		}

		c.registerEnums()
		c.registerEntities()
		c.registerTraits()
		c.registerFunctions()
		c.checkImplBlocks()
		c.checkFunctions()
		c.checkEntities()
		c.checkImplBlockBodies()
		c.checkTests()
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
		for name, info := range c.traits {
			allTraits[name] = info
		}
		for key, val := range c.implOrigins {
			allImplOrigins[key] = val
		}
		for name, info := range c.functions {
			allFunctions[name] = info
		}
	}

	return &CheckAllResult{
		Diagnostics: diag,
		ExprTypes:   allExprTypes,
		Entities:    allEntities,
		Enums:       allEnums,
		Traits:      allTraits,
		ImplOrigins: allImplOrigins,
		Functions:   allFunctions,
	}
}

// mergeModuleSymbols merges src symbols into dst without overwriting existing entries.
func mergeModuleSymbols(dst, src *ModuleSymbols) {
	for k, v := range src.Functions {
		if _, exists := dst.Functions[k]; !exists {
			dst.Functions[k] = v
		}
	}
	for k, v := range src.Entities {
		if _, exists := dst.Entities[k]; !exists {
			dst.Entities[k] = v
		}
	}
	for k, v := range src.Enums {
		if _, exists := dst.Enums[k]; !exists {
			dst.Enums[k] = v
		}
	}
	for k, v := range src.Traits {
		if _, exists := dst.Traits[k]; !exists {
			dst.Traits[k] = v
		}
	}
	for k, v := range src.FunctionDecls {
		if _, exists := dst.FunctionDecls[k]; !exists {
			dst.FunctionDecls[k] = v
		}
	}
	for k, v := range src.EntityDecls {
		if _, exists := dst.EntityDecls[k]; !exists {
			dst.EntityDecls[k] = v
		}
	}
	for k, v := range src.EnumDecls {
		if _, exists := dst.EnumDecls[k]; !exists {
			dst.EnumDecls[k] = v
		}
	}
}

// registerEntities registers all entities in the global scope.
//
// Two-pass: first register all entity skeletons (name + empty fields/methods
// maps) so that subsequent type resolution can see every entity name; then
// resolve field types and method signatures. This lets methods reference
// their own enclosing entity type (e.g. `method eq(other: Point) returns
// Bool` inside `entity Point`) and lets fields hold self-referential
// generics like `Option<Self>`. Phase 16 / ADR 0029 needs this for the
// explicit-`eq`-method rule on entity equality.
func (c *Checker) registerEntities() {
	// Pass 1: register skeletons.
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

		if len(entity.TypeParams) > 0 {
			for _, tp := range entity.TypeParams {
				info.TypeParamNames = append(info.TypeParamNames, tp.Name)
			}
		}

		c.entities[entity.Name] = info

		// Register entity in global scope so identifier lookups resolve.
		c.scope.Define(entity.Name, &Symbol{
			Name: entity.Name,
			Type: &Type{Name: entity.Name, IsEntity: true, Entity: info},
			Kind: SymEntity,
		})
	}

	// Pass 2: resolve fields and method signatures now that every entity name
	// is known.
	for _, entity := range c.prog.Entities {
		info, ok := c.entities[entity.Name]
		if !ok {
			// Duplicate-name error already reported in pass 1.
			continue
		}

		// Build type param map for generic entities.
		var typeParamMap map[string]bool
		if len(entity.TypeParams) > 0 {
			typeParamMap = make(map[string]bool)
			for _, tp := range entity.TypeParams {
				typeParamMap[tp.Name] = true
			}
		}

		// Resolve fields.
		for _, field := range entity.Fields {
			fieldType := ResolveTypeWithParams(field.Type, c.entities, c.enums, typeParamMap)
			if fieldType == nil {
				line, col := field.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", field.Type.Name)
				fieldType = TypeInt // fallback
			}
			info.Fields[field.Name] = fieldType
			info.FieldOrder = append(info.FieldOrder, field.Name)
		}

		// Resolve method signatures.
		for _, method := range entity.Methods {
			params := make([]ParamInfo, 0, len(method.Params))
			for _, p := range method.Params {
				pType := ResolveTypeWithParams(p.Type, c.entities, c.enums, typeParamMap)
				if pType == nil {
					line, col := p.Pos()
					c.diag.Errorf(line, col, "unknown type '%s'", p.Type.Name)
					pType = TypeInt // fallback
				}
				params = append(params, ParamInfo{Name: p.Name, Type: pType})
			}

			returnType := TypeVoid
			if method.ReturnType != nil {
				returnType = ResolveTypeWithParams(method.ReturnType, c.entities, c.enums, typeParamMap)
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

		// Build type param map for generic functions
		var typeParamMap map[string]bool
		var typeParamNames []string
		if len(fn.TypeParams) > 0 {
			typeParamMap = make(map[string]bool)
			for _, tp := range fn.TypeParams {
				typeParamMap[tp.Name] = true
				typeParamNames = append(typeParamNames, tp.Name)
			}
		}

		params := make([]ParamInfo, 0, len(fn.Params))
		for _, p := range fn.Params {
			pType := ResolveTypeWithParams(p.Type, c.entities, c.enums, typeParamMap)
			if pType == nil {
				line, col := p.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", p.Type.Name)
				pType = TypeInt // fallback
			}
			params = append(params, ParamInfo{Name: p.Name, Type: pType})
		}

		returnType := TypeVoid
		if fn.ReturnType != nil {
			returnType = ResolveTypeWithParams(fn.ReturnType, c.entities, c.enums, typeParamMap)
			if returnType == nil {
				line, col := fn.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", fn.ReturnType.Name)
				returnType = TypeVoid // fallback
			}
		}

		c.functions[fn.Name] = &FuncInfo{
			Name:           fn.Name,
			Params:         params,
			ReturnType:     returnType,
			TypeParamNames: typeParamNames,
			IsAsync:        fn.IsAsync,
		}

		c.scope.Define(fn.Name, &Symbol{
			Name: fn.Name,
			Type: returnType,
			Kind: SymFunction,
		})
	}

	// Register extern (FFI) function declarations alongside regular functions
	// so call sites resolve through the same path. ADR 0028 / Phase 15.
	for _, ext := range c.prog.ExternFunctions {
		if _, exists := c.functions[ext.Name]; exists {
			line, col := ext.Pos()
			c.diag.Errorf(line, col, "function '%s' already defined", ext.Name)
			continue
		}

		// Validate rust_path: must be non-empty and contain a `::` separator
		// (the first segment is the crate, the rest is the path inside it).
		if !strings.Contains(ext.RustPath, "::") {
			line, col := ext.Pos()
			c.diag.Errorf(line, col,
				"extern function '%s': from path %q must be of the form \"crate::path::to::function\"",
				ext.Name, ext.RustPath)
		}

		params := make([]ParamInfo, 0, len(ext.Params))
		for _, p := range ext.Params {
			pType := ResolveType(p.Type, c.entities, c.enums)
			if pType == nil {
				line, col := p.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", p.Type.Name)
				pType = TypeInt // fallback
			} else if reason := isFFIBridgeableType(pType); reason != "" {
				line, col := p.Pos()
				c.diag.Errorf(line, col,
					"extern function '%s' parameter '%s': %s",
					ext.Name, p.Name, reason)
			}
			params = append(params, ParamInfo{Name: p.Name, Type: pType})
		}

		returnType := TypeVoid
		if ext.ReturnType != nil {
			returnType = ResolveType(ext.ReturnType, c.entities, c.enums)
			if returnType == nil {
				line, col := ext.Pos()
				c.diag.Errorf(line, col, "unknown type '%s'", ext.ReturnType.Name)
				returnType = TypeVoid
			} else if reason := isFFIBridgeableType(returnType); reason != "" {
				line, col := ext.Pos()
				c.diag.Errorf(line, col,
					"extern function '%s' return type: %s",
					ext.Name, reason)
			}
		}

		c.functions[ext.Name] = &FuncInfo{
			Name:       ext.Name,
			Params:     params,
			ReturnType: returnType,
		}

		c.scope.Define(ext.Name, &Symbol{
			Name: ext.Name,
			Type: returnType,
			Kind: SymFunction,
		})
	}
}

// isFFIBridgeableType reports the empty string when t is allowed in an extern
// function signature, or a human-readable reason why it is not. ADR 0028
// defines the supported set: Int, Float, Bool, String, Void, Array<T>,
// Result<T,E>, Option<T> with bridged inner types.
func isFFIBridgeableType(t *Type) string {
	if t == nil {
		return "unknown type"
	}
	switch t.Name {
	case "Int", "Float", "Bool", "String", "Void":
		return ""
	case "Array":
		if len(t.TypeParams) == 1 {
			if reason := isFFIBridgeableType(t.TypeParams[0]); reason != "" {
				return "Array element type: " + reason
			}
			return ""
		}
		return "Array missing type parameter"
	case "Result":
		if len(t.TypeParams) == 2 {
			if reason := isFFIBridgeableType(t.TypeParams[0]); reason != "" {
				return "Result Ok type: " + reason
			}
			if reason := isFFIBridgeableType(t.TypeParams[1]); reason != "" {
				return "Result Err type: " + reason
			}
			return ""
		}
		return "Result needs exactly two type parameters"
	case "Option":
		if len(t.TypeParams) == 1 {
			if reason := isFFIBridgeableType(t.TypeParams[0]); reason != "" {
				return "Option inner type: " + reason
			}
			return ""
		}
		return "Option missing type parameter"
	case "Map":
		return "Map<K,V> is not supported across the FFI boundary (see ADR 0028)"
	case "Future":
		return "Future<T> is not supported across the FFI boundary"
	case "Fn":
		return "Fn(...) closures are not supported across the FFI boundary"
	}
	if t.IsEntity {
		return "entity types are not supported across the FFI boundary"
	}
	if t.IsEnum {
		return "user-defined enums are not supported across the FFI boundary (only Result and Option)"
	}
	if t.IsTypeParam {
		return "extern functions cannot be generic"
	}
	return "type '" + t.Name + "' is not bridgeable; supported: Int, Float, Bool, String, Void, Array<T>, Result<T,E>, Option<T>"
}

// checkFunctions checks all function bodies
func (c *Checker) checkFunctions() {
	for _, fn := range c.prog.Functions {
		c.checkFunction(fn)
	}
}

// checkFunction checks a single function
func (c *Checker) checkFunction(fn *ast.FunctionDecl) {
	// Skip detailed body checking for generic functions (type params are placeholders)
	if len(fn.TypeParams) > 0 {
		return
	}

	funcScope := NewScope(c.scope)

	// Set current function context for Result/Option variant checking
	c.currentFunc = c.functions[fn.Name]

	// Track async context
	oldAsync := c.inAsyncFunc
	c.inAsyncFunc = fn.IsAsync

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
	c.inAsyncFunc = oldAsync
}

// checkTests checks all in-language test declarations (Phase 16 / ADR 0029).
func (c *Checker) checkTests() {
	for _, t := range c.prog.Tests {
		c.checkTest(t)
	}
}

// checkTest type-checks a single test declaration. Tests have no parameters
// and no return type; the body is checked as a Void-returning block. `return`
// statements are rejected. Async tests track whether an `await` was seen so
// that declared-async-but-never-awaits emits a warning.
//
// ADR 0031: @target_specific annotations are validated here — target strings
// must be in {rust, js, wasm}; duplicate annotations are rejected; a wasm
// target warns because WASM rejects test declarations entirely (16.6).
func (c *Checker) checkTest(t *ast.TestDecl) {
	c.checkTestAnnotations(t)

	testScope := NewScope(c.scope)

	// Track context: tests reject `return` statements; async tests warn if
	// they never await.
	oldInTest := c.inTest
	oldTestName := c.currentTestName
	oldAsync := c.inAsyncFunc
	oldSawAwait := c.testSawAwait
	c.inTest = true
	c.currentTestName = t.Name
	c.inAsyncFunc = t.IsAsync
	c.testSawAwait = false

	if t.Body != nil {
		c.checkBlock(t.Body, testScope)
	}

	if t.IsAsync && !c.testSawAwait {
		c.diag.Warningf(t.Line, t.Column,
			"test %q declared 'async' but contains no 'await' expression", t.Name)
	}

	c.inTest = oldInTest
	c.currentTestName = oldTestName
	c.inAsyncFunc = oldAsync
	c.testSawAwait = oldSawAwait
}

// checkTestAnnotations validates ADR 0031 @target_specific annotations on a
// test declaration. Parse-time errors (unknown annotation name, empty args,
// annotation on non-test) are already reported by the parser; this enforces
// the semantic constraints.
func (c *Checker) checkTestAnnotations(t *ast.TestDecl) {
	seen := make(map[string]bool)
	for _, ann := range t.Annotations {
		if ann.Name != "target_specific" {
			// Unknown names are reported at parse time; skip here.
			continue
		}
		if seen[ann.Name] {
			c.diag.Errorf(ann.Line, ann.Column,
				"duplicate @%s annotation on test %q", ann.Name, t.Name)
			continue
		}
		seen[ann.Name] = true
		for _, target := range ann.Args {
			switch target {
			case "rust", "js", "wasm":
				// recognised target
			default:
				c.diag.Errorf(ann.Line, ann.Column,
					"@target_specific: %q is not a recognised target (expected: rust, js, wasm)", target)
			}
			if target == "wasm" {
				c.diag.Warningf(ann.Line, ann.Column,
					"@target_specific(\"wasm\") will never run; WASM rejects all test declarations (phase 16 task 16.6)")
			}
		}
	}
}

// checkEntities checks all entity constructors and methods
func (c *Checker) checkEntities() {
	for _, entity := range c.prog.Entities {
		// Skip detailed body checking for generic entities (type params are placeholders)
		if len(entity.TypeParams) > 0 {
			continue
		}

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

	// Set target type context for empty array literal inference
	c.letDeclaredType = targetType

	// Check value
	valueType := c.checkExpression(stmt.Value, scope)

	// Clear target type context
	c.letDeclaredType = nil

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
	if c.inTest {
		line, col := stmt.Pos()
		c.diag.Errorf(line, col, "'return' is not allowed inside a test body; test %q has implicit Void return", c.currentTestName)
	}
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
	case *ast.StringInterp:
		// Type-check each expression part
		for _, part := range e.Parts {
			if part.IsExpr && part.Expr != nil {
				c.checkExpression(part.Expr, scope)
			}
		}
		return c.storeExprType(expr, TypeString)
	case *ast.BoolLit:
		return c.storeExprType(expr, TypeBool)
	case *ast.CharLit:
		return c.storeExprType(expr, TypeChar)
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
	case *ast.LambdaExpr:
		return c.storeExprType(expr, c.checkLambdaExpr(e, scope))
	case *ast.AwaitExpr:
		return c.storeExprType(expr, c.checkAwaitExpr(e, scope))
	case *ast.SpawnExpr:
		return c.storeExprType(expr, c.checkSpawnExpr(e, scope))
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
		// Works on Int, Float, String, Bool, Char (same types). ADR 0041 adds Char.
		if leftType.Equal(rightType) {
			if leftType.Equal(TypeInt) || leftType.Equal(TypeFloat) || leftType.Equal(TypeString) || leftType.Equal(TypeBool) || leftType.Equal(TypeChar) {
				return TypeBool
			}
		}
		c.diag.Errorf(line, col, "operator '%s' not defined for %s and %s", expr.Op, leftType.Name, rightType.Name)
		return nil

	case lexer.LT, lexer.GT, lexer.LEQ, lexer.GEQ:
		// Works on Int, Float, String, Char (same types). ADR 0041 adds Char.
		if leftType.Equal(rightType) {
			if leftType.Equal(TypeInt) || leftType.Equal(TypeFloat) || leftType.Equal(TypeString) || leftType.Equal(TypeChar) {
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

	// Phase 16 / ADR 0029: in-language test assertion builtins.
	// assert(cond: Bool) returns Void
	if expr.Function == "assert" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "assert() expects 1 argument, got %d", len(expr.Args))
			return TypeVoid
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeBool) {
			c.diag.Errorf(line, col, "assert() argument must be Bool, got %s", argType.String())
		}
		return TypeVoid
	}

	// assert_eq<T>(actual: T, expected: T) returns Void
	// T must be in the comparable set. Float is rejected (use assert_close).
	// Entities require an explicit eq method.
	if expr.Function == "assert_eq" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "assert_eq() expects 2 arguments, got %d", len(expr.Args))
			return TypeVoid
		}
		actualType := c.checkExpression(expr.Args[0], scope)
		expectedType := c.checkExpression(expr.Args[1], scope)
		if actualType == nil || expectedType == nil {
			return TypeVoid
		}
		if !actualType.Equal(expectedType) {
			c.diag.Errorf(line, col, "assert_eq() type mismatch: actual is %s, expected is %s", actualType.String(), expectedType.String())
			return TypeVoid
		}
		if reason := assertEqUnsupportedReason(actualType); reason != "" {
			c.diag.Errorf(line, col, "%s", reason)
		}
		return TypeVoid
	}

	// assert_close(actual: Float, expected: Float, epsilon: Float) returns Void
	if expr.Function == "assert_close" {
		if len(expr.Args) != 3 {
			c.diag.Errorf(line, col, "assert_close() expects 3 arguments, got %d", len(expr.Args))
			return TypeVoid
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeFloat) {
				labels := []string{"actual", "expected", "epsilon"}
				c.diag.Errorf(line, col, "assert_close() argument %d (%s) must be Float, got %s", i+1, labels[i], argType.String())
			}
		}
		return TypeVoid
	}

	// assert_panics(fn: Fn() -> Void) returns Void
	if expr.Function == "assert_panics" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "assert_panics() expects 1 argument, got %d", len(expr.Args))
			return TypeVoid
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil {
			if !argType.IsFunction || len(argType.FnParams) != 0 || argType.FnReturn == nil || !argType.FnReturn.Equal(TypeVoid) {
				c.diag.Errorf(line, col, "assert_panics() argument must be Fn() -> Void, got %s", argType.String())
			}
		}
		return TypeVoid
	}

	// Handle len() built-in
	// Phase 31 / ADR 0041: Char.from_codepoint(n) is exposed as a free builtin
	// `char_from_codepoint(n: Int) returns Result<Char, String>`. The naming
	// avoids needing Type::method syntax in v1; a future ADR can graduate it
	// to a static method form.
	if expr.Function == "char_from_codepoint" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "char_from_codepoint() requires exactly 1 argument, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeChar, TypeString}, EnumInfo: instantiateResult(TypeChar, TypeString)}
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeInt) {
			c.diag.Errorf(line, col, "char_from_codepoint() argument must be Int, got %s", argType.String())
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeChar, TypeString}, EnumInfo: instantiateResult(TypeChar, TypeString)}
	}

	if expr.Function == "len" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "len() requires exactly 1 argument, got %d", len(expr.Args))
			return TypeInt
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil {
			// Phase 31 / ADR 0041: len() now also accepts String (codepoint count).
			if argType.Equal(TypeString) {
				return TypeInt
			}
			if (argType.Name != "Array" && argType.Name != "Map") || !argType.IsGeneric {
				c.diag.Errorf(line, col, "len() requires Array, Map, or String argument, got %s", argType.String())
			}
		}
		return TypeInt
	}

	// Handle read_file() built-in
	if expr.Function == "read_file" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "read_file() requires exactly 1 argument, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeString) {
			c.diag.Errorf(line, col, "read_file() argument must be String, got %s", argType.String())
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
	}

	// Handle write_file() built-in
	if expr.Function == "write_file" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "write_file() requires exactly 2 arguments, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
		}
		argType0 := c.checkExpression(expr.Args[0], scope)
		if argType0 != nil && !argType0.Equal(TypeString) {
			c.diag.Errorf(line, col, "write_file() argument 1 must be String, got %s", argType0.String())
		}
		argType1 := c.checkExpression(expr.Args[1], scope)
		if argType1 != nil && !argType1.Equal(TypeString) {
			c.diag.Errorf(line, col, "write_file() argument 2 must be String, got %s", argType1.String())
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
	}

	// Handle create_dir() built-in
	if expr.Function == "create_dir" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "create_dir() requires exactly 1 argument, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeString) {
			c.diag.Errorf(line, col, "create_dir() argument must be String, got %s", argType.String())
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
	}

	// Handle file_exists() built-in
	if expr.Function == "file_exists" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "file_exists() requires exactly 1 argument, got %d", len(expr.Args))
			return TypeBool
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeString) {
			c.diag.Errorf(line, col, "file_exists() argument must be String, got %s", argType.String())
		}
		return TypeBool
	}

	// Handle env_get() built-in
	if expr.Function == "env_get" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "env_get() requires exactly 1 argument, got %d", len(expr.Args))
			return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeString) {
			c.diag.Errorf(line, col, "env_get() argument must be String, got %s", argType.String())
		}
		return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
	}

	// Handle http_post() built-in
	if expr.Function == "http_post" {
		if len(expr.Args) != 3 {
			c.diag.Errorf(line, col, "http_post() requires exactly 3 arguments, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "http_post() argument %d must be String, got %s", i+1, argType.String())
			}
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
	}

	// Handle http_get() built-in
	if expr.Function == "http_get" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "http_get() requires exactly 2 arguments, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "http_get() argument %d must be String, got %s", i+1, argType.String())
			}
		}
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString, TypeString}, EnumInfo: instantiateResult(TypeString, TypeString)}
	}

	// Handle json_get() built-in
	if expr.Function == "json_get" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "json_get() requires exactly 2 arguments, got %d", len(expr.Args))
			return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "json_get() argument %d must be String, got %s", i+1, argType.String())
			}
		}
		return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
	}

	// Handle json_path() built-in
	if expr.Function == "json_path" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "json_path() requires exactly 2 arguments, got %d", len(expr.Args))
			return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "json_path() argument %d must be String, got %s", i+1, argType.String())
			}
		}
		return &Type{Name: "Option", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeString}, EnumInfo: instantiateOption(TypeString)}
	}

	// Handle emit_event() built-in
	if expr.Function == "emit_event" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "emit_event() requires exactly 2 arguments, got %d", len(expr.Args))
			return TypeVoid
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "emit_event() argument %d must be String, got %s", i+1, argType.String())
			}
		}
		return TypeVoid
	}

	// Handle timestamp_ms() built-in
	if expr.Function == "timestamp_ms" {
		if len(expr.Args) != 0 {
			c.diag.Errorf(line, col, "timestamp_ms() takes no arguments, got %d", len(expr.Args))
		}
		return TypeInt
	}

	// Handle args() built-in: args() -> Array<String>
	if expr.Function == "args" {
		if len(expr.Args) != 0 {
			c.diag.Errorf(line, col, "args() takes no arguments, got %d", len(expr.Args))
		}
		return &Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{TypeString}}
	}

	// Handle sleep() built-in: sleep(Int) -> Future<Void>
	if expr.Function == "sleep" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "sleep() requires exactly 1 argument, got %d", len(expr.Args))
			return &Type{Name: "Future", IsGeneric: true, TypeParams: []*Type{TypeVoid}}
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType != nil && !argType.Equal(TypeInt) {
			c.diag.Errorf(line, col, "sleep() argument must be Int, got %s", argType.String())
		}
		return &Type{Name: "Future", IsGeneric: true, TypeParams: []*Type{TypeVoid}}
	}

	// Handle await_all() built-in: await_all(Array<Future<T>>) -> Array<T>
	if expr.Function == "await_all" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "await_all() requires exactly 1 argument, got %d", len(expr.Args))
			return nil
		}
		if !c.inAsyncFunc {
			c.diag.Errorf(line, col, "await_all can only be used inside async functions")
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType == nil {
			return nil
		}
		// Must be Array<Future<T>>
		if argType.Name != "Array" || !argType.IsGeneric || len(argType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "await_all() requires Array<Future<T>> argument, got %s", argType.String())
			return nil
		}
		elemType := argType.TypeParams[0]
		if elemType.Name != "Future" || !elemType.IsGeneric || len(elemType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "await_all() requires Array<Future<T>> argument, got %s", argType.String())
			return nil
		}
		innerType := elemType.TypeParams[0]
		return &Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{innerType}}
	}

	// Handle await_any() built-in: await_any(Array<Future<T>>) -> T
	if expr.Function == "await_any" {
		if len(expr.Args) != 1 {
			c.diag.Errorf(line, col, "await_any() requires exactly 1 argument, got %d", len(expr.Args))
			return nil
		}
		if !c.inAsyncFunc {
			c.diag.Errorf(line, col, "await_any can only be used inside async functions")
		}
		argType := c.checkExpression(expr.Args[0], scope)
		if argType == nil {
			return nil
		}
		// Must be Array<Future<T>>
		if argType.Name != "Array" || !argType.IsGeneric || len(argType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "await_any() requires Array<Future<T>> argument, got %s", argType.String())
			return nil
		}
		elemType := argType.TypeParams[0]
		if elemType.Name != "Future" || !elemType.IsGeneric || len(elemType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "await_any() requires Array<Future<T>> argument, got %s", argType.String())
			return nil
		}
		return elemType.TypeParams[0]
	}

	// Handle timeout() built-in: timeout(Future<T>, Int) -> Result<T, String>
	if expr.Function == "timeout" {
		if len(expr.Args) != 2 {
			c.diag.Errorf(line, col, "timeout() requires exactly 2 arguments, got %d", len(expr.Args))
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
		}
		if !c.inAsyncFunc {
			c.diag.Errorf(line, col, "timeout can only be used inside async functions")
		}
		futureType := c.checkExpression(expr.Args[0], scope)
		timeoutArg := c.checkExpression(expr.Args[1], scope)
		if timeoutArg != nil && !timeoutArg.Equal(TypeInt) {
			c.diag.Errorf(line, col, "timeout() second argument must be Int, got %s", timeoutArg.String())
		}
		if futureType == nil {
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
		}
		if futureType.Name != "Future" || !futureType.IsGeneric || len(futureType.TypeParams) != 1 {
			c.diag.Errorf(line, col, "timeout() first argument must be Future<T>, got %s", futureType.String())
			return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{TypeVoid, TypeString}, EnumInfo: instantiateResult(TypeVoid, TypeString)}
		}
		innerType := futureType.TypeParams[0]
		return &Type{Name: "Result", IsEnum: true, IsGeneric: true, TypeParams: []*Type{innerType, TypeString}, EnumInfo: instantiateResult(innerType, TypeString)}
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

		// Check args
		for _, arg := range expr.Args {
			c.checkExpression(arg, scope)
		}

		// Handle generic entity constructor with type args: Stack<Int>()
		if len(entity.TypeParamNames) > 0 {
			if len(expr.TypeArgs) == 0 {
				c.diag.Errorf(line, col, "generic entity '%s' requires type arguments", expr.Function)
				return &Type{Name: expr.Function, IsEntity: true, Entity: entity}
			}
			if len(expr.TypeArgs) != len(entity.TypeParamNames) {
				c.diag.Errorf(line, col, "entity '%s' expects %d type arguments, got %d",
					expr.Function, len(entity.TypeParamNames), len(expr.TypeArgs))
				return &Type{Name: expr.Function, IsEntity: true, Entity: entity}
			}
			var resolvedArgs []*Type
			for _, ta := range expr.TypeArgs {
				resolved := ResolveType(ta, c.entities, c.enums)
				if resolved == nil {
					c.diag.Errorf(line, col, "unknown type '%s' in type argument", ta.Name)
					return nil
				}
				resolvedArgs = append(resolvedArgs, resolved)
			}
			return &Type{
				Name:       expr.Function,
				IsEntity:   true,
				Entity:     entity,
				IsGeneric:  true,
				TypeParams: resolvedArgs,
			}
		}

		return &Type{Name: expr.Function, IsEntity: true, Entity: entity}
	}

	// Check if it's a variable with a function type (closure call)
	if sym := scope.Resolve(expr.Function); sym != nil && sym.Type != nil && sym.Type.IsFunction {
		fnType := sym.Type
		if len(expr.Args) != len(fnType.FnParams) {
			c.diag.Errorf(line, col, "function '%s' expects %d arguments, got %d",
				expr.Function, len(fnType.FnParams), len(expr.Args))
			return fnType.FnReturn
		}
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if argType != nil && !argType.Equal(fnType.FnParams[i]) {
				argLine, argCol := arg.Pos()
				c.diag.Errorf(argLine, argCol, "argument %d to '%s': expected %s, got %s",
					i+1, expr.Function, fnType.FnParams[i].String(), argType.String())
			}
		}
		return fnType.FnReturn
	}

	// Check if it's a function call
	fn, exists := c.functions[expr.Function]
	if !exists {
		c.diag.Errorf(line, col, "unknown function '%s'", expr.Function)
		return nil
	}

	// Handle generic function call with type args: identity<Int>(42)
	if len(fn.TypeParamNames) > 0 {
		if len(expr.TypeArgs) == 0 {
			c.diag.Errorf(line, col, "generic function '%s' requires type arguments", expr.Function)
		} else if len(expr.TypeArgs) != len(fn.TypeParamNames) {
			c.diag.Errorf(line, col, "function '%s' expects %d type arguments, got %d",
				expr.Function, len(fn.TypeParamNames), len(expr.TypeArgs))
		}
		// Build substitution map
		substMap := make(map[string]*Type)
		for i, ta := range expr.TypeArgs {
			resolved := ResolveType(ta, c.entities, c.enums)
			if resolved == nil {
				c.diag.Errorf(line, col, "unknown type '%s' in type argument", ta.Name)
				continue
			}
			if i < len(fn.TypeParamNames) {
				substMap[fn.TypeParamNames[i]] = resolved
			}
		}

		// Check argument count
		if len(expr.Args) != len(fn.Params) {
			c.diag.Errorf(line, col, "function '%s' expects %d arguments, got %d",
				expr.Function, len(fn.Params), len(expr.Args))
		}

		// Check argument types with substitution
		for i, arg := range expr.Args {
			argType := c.checkExpression(arg, scope)
			if i < len(fn.Params) && argType != nil {
				expectedType := SubstituteType(fn.Params[i].Type, substMap)
				if !argType.Equal(expectedType) && !expectedType.IsTypeParam {
					argLine, argCol := arg.Pos()
					c.diag.Errorf(argLine, argCol, "argument %d to '%s': expected %s, got %s",
						i+1, expr.Function, expectedType.String(), argType.String())
				}
			}
		}

		// Return substituted return type
		return SubstituteType(fn.ReturnType, substMap)
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
		// Async functions may declare their return as `Future<Result<T,E>>` or
		// `Future<Option<T>>`; the body still returns the inner Result/Option.
		// Peel the Future wrapper before inferring the variant context.
		if returnType.Name == "Future" && returnType.IsGeneric && len(returnType.TypeParams) == 1 {
			returnType = returnType.TypeParams[0]
		}
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

	// Handle Map methods
	if objType.Name == "Map" && objType.IsGeneric && len(objType.TypeParams) == 2 {
		keyType := objType.TypeParams[0]
		valType := objType.TypeParams[1]
		switch expr.Method {
		case "get":
			if len(expr.Args) != 2 {
				c.diag.Errorf(line, col, "get() requires exactly 2 arguments (key, default), got %d", len(expr.Args))
				return valType
			}
			argKeyType := c.checkExpression(expr.Args[0], scope)
			if argKeyType != nil && !argKeyType.Equal(keyType) {
				c.diag.Errorf(line, col, "get() key type mismatch: expected %s, got %s", keyType.String(), argKeyType.String())
			}
			argDefType := c.checkExpression(expr.Args[1], scope)
			if argDefType != nil && !argDefType.Equal(valType) {
				c.diag.Errorf(line, col, "get() default type mismatch: expected %s, got %s", valType.String(), argDefType.String())
			}
			return valType
		case "set":
			if len(expr.Args) != 2 {
				c.diag.Errorf(line, col, "set() requires exactly 2 arguments (key, value), got %d", len(expr.Args))
				return TypeVoid
			}
			// Check mutability
			if ident, ok := expr.Object.(*ast.Identifier); ok {
				sym := scope.Resolve(ident.Name)
				if sym != nil && !sym.Mutable {
					c.diag.Errorf(line, col, "cannot call set() on immutable map '%s'", ident.Name)
				}
			}
			argKeyType := c.checkExpression(expr.Args[0], scope)
			if argKeyType != nil && !argKeyType.Equal(keyType) {
				c.diag.Errorf(line, col, "set() key type mismatch: expected %s, got %s", keyType.String(), argKeyType.String())
			}
			argValType := c.checkExpression(expr.Args[1], scope)
			if argValType != nil && !argValType.Equal(valType) {
				c.diag.Errorf(line, col, "set() value type mismatch: expected %s, got %s", valType.String(), argValType.String())
			}
			return TypeVoid
		case "contains":
			if len(expr.Args) != 1 {
				c.diag.Errorf(line, col, "contains() requires exactly 1 argument, got %d", len(expr.Args))
				return TypeBool
			}
			argKeyType := c.checkExpression(expr.Args[0], scope)
			if argKeyType != nil && !argKeyType.Equal(keyType) {
				c.diag.Errorf(line, col, "contains() key type mismatch: expected %s, got %s", keyType.String(), argKeyType.String())
			}
			return TypeBool
		case "keys":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "keys() requires no arguments, got %d", len(expr.Args))
			}
			return &Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{keyType}}
		case "remove":
			if len(expr.Args) != 1 {
				c.diag.Errorf(line, col, "remove() requires exactly 1 argument, got %d", len(expr.Args))
				return TypeVoid
			}
			// Check mutability
			if ident, ok := expr.Object.(*ast.Identifier); ok {
				sym := scope.Resolve(ident.Name)
				if sym != nil && !sym.Mutable {
					c.diag.Errorf(line, col, "cannot call remove() on immutable map '%s'", ident.Name)
				}
			}
			argKeyType := c.checkExpression(expr.Args[0], scope)
			if argKeyType != nil && !argKeyType.Equal(keyType) {
				c.diag.Errorf(line, col, "remove() key type mismatch: expected %s, got %s", keyType.String(), argKeyType.String())
			}
			return TypeVoid
		default:
			c.diag.Errorf(line, col, "Map has no method '%s'", expr.Method)
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

	// Handle String methods
	if objType.Name == "String" {
		switch expr.Method {
		case "len":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "len() requires no arguments, got %d", len(expr.Args))
			}
			return TypeInt
		case "to_lowercase", "trim":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "%s() requires no arguments, got %d", expr.Method, len(expr.Args))
			}
			return TypeString
		case "starts_with", "contains":
			if len(expr.Args) != 1 {
				c.diag.Errorf(line, col, "%s() requires exactly 1 argument, got %d", expr.Method, len(expr.Args))
				return TypeBool
			}
			argType := c.checkExpression(expr.Args[0], scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "%s() argument must be String, got %s", expr.Method, argType.String())
			}
			return TypeBool
		case "split":
			if len(expr.Args) != 1 {
				c.diag.Errorf(line, col, "split() requires exactly 1 argument, got %d", len(expr.Args))
				return &Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{TypeString}}
			}
			argType := c.checkExpression(expr.Args[0], scope)
			if argType != nil && !argType.Equal(TypeString) {
				c.diag.Errorf(line, col, "split() argument must be String, got %s", argType.String())
			}
			return &Type{Name: "Array", IsGeneric: true, TypeParams: []*Type{TypeString}}
		default:
			c.diag.Errorf(line, col, "String has no method '%s'", expr.Method)
			return nil
		}
	}

	// Handle to_string() method on Int, Float, Bool, Char.
	// Phase 31 / ADR 0041 adds Char.
	if expr.Method == "to_string" {
		if objType.Equal(TypeInt) || objType.Equal(TypeFloat) || objType.Equal(TypeBool) || objType.Equal(TypeChar) {
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "to_string() requires no arguments, got %d", len(expr.Args))
			}
			return TypeString
		}
	}

	// Phase 31 / ADR 0041: Char methods.
	if objType.Equal(TypeChar) {
		switch expr.Method {
		case "to_codepoint":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "to_codepoint() requires no arguments, got %d", len(expr.Args))
			}
			return TypeInt
		case "is_digit", "is_alpha", "is_alphanumeric", "is_whitespace", "is_lowercase", "is_uppercase":
			if len(expr.Args) != 0 {
				c.diag.Errorf(line, col, "%s() requires no arguments, got %d", expr.Method, len(expr.Args))
			}
			return TypeBool
		default:
			c.diag.Errorf(line, col, "Char has no method '%s'", expr.Method)
			return nil
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

	// Check argument types (skip for type params since they match anything)
	for i, arg := range expr.Args {
		argType := c.checkExpression(arg, scope)
		expectedType := method.Params[i].Type
		if argType != nil && !argType.Equal(expectedType) && !expectedType.IsTypeParam {
			argLine, argCol := arg.Pos()
			c.diag.Errorf(argLine, argCol, "argument %d to method '%s': expected %s, got %s",
				i+1, expr.Method, expectedType.Name, argType.Name)
		}
	}

	// Return type: if it's a type param, return the concrete type based on the object's type args
	returnType := method.ReturnType
	if returnType != nil && returnType.IsTypeParam && objType.IsGeneric && len(objType.TypeParams) > 0 {
		if entity, ok := c.entities[objType.Name]; ok {
			for i, tpName := range entity.TypeParamNames {
				if tpName == returnType.Name && i < len(objType.TypeParams) {
					returnType = objType.TypeParams[i]
					break
				}
			}
		}
	}

	return returnType
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
		// Try to infer element type from let declaration type annotation
		if c.letDeclaredType != nil && c.letDeclaredType.Name == "Array" && c.letDeclaredType.IsGeneric && len(c.letDeclaredType.TypeParams) == 1 {
			return c.letDeclaredType
		}
		// Also support empty [] for Map types
		if c.letDeclaredType != nil && c.letDeclaredType.Name == "Map" && c.letDeclaredType.IsGeneric && len(c.letDeclaredType.TypeParams) == 2 {
			return c.letDeclaredType
		}
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

// checkIndexExpr checks an index expression (Array<T>[i], String[i], String[i..j]).
// Phase 31 / ADR 0041 added String indexing (returns Char) and String slicing
// (returns String) via a RangeExpr in the Index slot.
func (c *Checker) checkIndexExpr(expr *ast.IndexExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	objType := c.checkExpression(expr.Object, scope)
	if objType == nil {
		return nil
	}

	// String indexing / slicing (ADR 0041).
	if objType.Equal(TypeString) {
		if rng, ok := expr.Index.(*ast.RangeExpr); ok {
			// String[Int..Int] -> String
			startT := c.checkExpression(rng.Start, scope)
			endT := c.checkExpression(rng.End, scope)
			if startT != nil && !startT.Equal(TypeInt) {
				c.diag.Errorf(line, col, "string slice start must be Int, got %s", startT.String())
			}
			if endT != nil && !endT.Equal(TypeInt) {
				c.diag.Errorf(line, col, "string slice end must be Int, got %s", endT.String())
			}
			return TypeString
		}
		// String[Int] -> Char
		indexType := c.checkExpression(expr.Index, scope)
		if indexType != nil && !indexType.Equal(TypeInt) {
			c.diag.Errorf(line, col, "string index must be Int, got %s", indexType.String())
		}
		return TypeChar
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

// checkLambdaExpr checks a lambda expression
func (c *Checker) checkLambdaExpr(expr *ast.LambdaExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// Create a new scope for the lambda body
	lambdaScope := NewScope(scope)

	// Resolve parameter types and add to scope
	var paramTypes []*Type
	for _, p := range expr.Params {
		pType := ResolveType(p.Type, c.entities, c.enums)
		if pType == nil {
			pLine, pCol := p.Pos()
			c.diag.Errorf(pLine, pCol, "unknown type in lambda parameter '%s'", p.Name)
			return nil
		}
		paramTypes = append(paramTypes, pType)
		lambdaScope.Define(p.Name, &Symbol{
			Name:    p.Name,
			Type:    pType,
			Mutable: false,
			Kind:    SymVariable,
		})
	}

	// Resolve return type if specified
	var returnType *Type
	if expr.ReturnType != nil {
		returnType = ResolveType(expr.ReturnType, c.entities, c.enums)
		if returnType == nil {
			c.diag.Errorf(line, col, "unknown return type in lambda")
			return nil
		}
	}

	// Type-check the body expression
	bodyType := c.checkExpression(expr.Body, lambdaScope)
	if bodyType == nil {
		return nil
	}

	// If return type was specified, verify it matches
	if returnType != nil {
		if !bodyType.Equal(returnType) {
			c.diag.Errorf(line, col, "lambda body type %s does not match declared return type %s",
				bodyType.String(), returnType.String())
			return nil
		}
	} else {
		returnType = bodyType
	}

	return &Type{
		Name:       "Fn",
		IsFunction: true,
		FnParams:   paramTypes,
		FnReturn:   returnType,
	}
}

// checkAwaitExpr checks an await expression
func (c *Checker) checkAwaitExpr(expr *ast.AwaitExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// await is only valid inside async functions or async tests
	if !c.inAsyncFunc {
		c.diag.Errorf(line, col, "await can only be used inside async functions")
	}
	c.testSawAwait = true

	innerType := c.checkExpression(expr.Expr, scope)
	if innerType == nil {
		return nil
	}

	// The inner expression must be a Future<T>
	if innerType.Name != "Future" || !innerType.IsGeneric || len(innerType.TypeParams) != 1 {
		c.diag.Errorf(line, col, "await requires Future type, got %s", innerType.String())
		return nil
	}

	// Unwrap Future<T> -> T
	return innerType.TypeParams[0]
}

// checkSpawnExpr checks a spawn expression
func (c *Checker) checkSpawnExpr(expr *ast.SpawnExpr, scope *Scope) *Type {
	line, col := expr.Pos()

	// spawn accepts either a direct call expression `f(args)` or a
	// module-qualified call `mod.f(args)` where the object identifier
	// resolves to no type (i.e. it's an imported module name).
	var callType *Type
	switch callExpr := expr.Expr.(type) {
	case *ast.CallExpr:
		callType = c.checkCallExpr(callExpr, scope)
		if fn, exists := c.functions[callExpr.Function]; exists && !fn.IsAsync {
			c.diag.Errorf(line, col, "spawn requires an async function, '%s' is not async", callExpr.Function)
		}
	case *ast.MethodCallExpr:
		// Module-qualified call: `module.fn(args)` where module is an imported
		// module name. Method-on-an-entity cannot be spawned currently.
		ident, identOk := callExpr.Object.(*ast.Identifier)
		isModule := false
		if identOk && c.moduleImports != nil {
			_, isModule = c.moduleImports[ident.Name]
		}
		if !isModule {
			c.diag.Errorf(line, col, "spawn requires a function call or module-qualified call to an async function")
			return nil
		}
		callType = c.checkMethodCallExpr(callExpr, scope)
		// Async check: look up the function in the same module's symbols.
		if modSyms, ok := c.moduleImports[ident.Name]; ok {
			if fn, exists := modSyms.Functions[callExpr.Method]; exists && !fn.IsAsync {
				c.diag.Errorf(line, col, "spawn requires an async function, '%s.%s' is not async", ident.Name, callExpr.Method)
			}
		}
	default:
		c.diag.Errorf(line, col, "spawn requires a function call")
		return nil
	}

	if callType == nil {
		return nil
	}

	// If the call already returns Future<T> (async functions declare Future<T> as return type),
	// return it as-is; otherwise wrap in Future<T>
	if callType.Name == "Future" && callType.IsGeneric && len(callType.TypeParams) == 1 {
		return callType
	}
	return &Type{Name: "Future", IsGeneric: true, TypeParams: []*Type{callType}}
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

// assertEqUnsupportedReason returns a non-empty diagnostic message if a type
// cannot appear in assert_eq, or "" if it can. Comparable set per ADR 0029:
// Int, Bool, String, Void, Array<T>, Option<T>, Result<T,E>, user enums, and
// entities that declare a method named `eq`. Float is rejected explicitly.
// Map, Future, and function types are not comparable.
func assertEqUnsupportedReason(t *Type) string {
	if t == nil {
		return ""
	}
	if t.Equal(TypeFloat) {
		return "assert_eq does not support Float; use assert_close(actual, expected, epsilon) for floating-point comparisons"
	}
	if t.IsFunction {
		return "assert_eq does not support function types"
	}
	if t.Name == "Map" {
		return "assert_eq does not support Map; compare maps via their keys() and get(key, default) instead"
	}
	if t.Name == "Future" {
		return "assert_eq does not support Future; await it first and compare the resolved value"
	}
	if t.IsEntity {
		// Require an explicit eq method on the entity type.
		if t.Entity == nil || t.Entity.Methods == nil || t.Entity.Methods["eq"] == nil {
			return "entity '" + t.Name + "' used in assert_eq but has no eq method; define 'method eq(other: " + t.Name + ") returns Bool' to enable equality checks"
		}
		// Sanity-check the signature: one parameter of the same entity type, returns Bool.
		m := t.Entity.Methods["eq"]
		if m.ReturnType == nil || !m.ReturnType.Equal(TypeBool) {
			return "entity '" + t.Name + "' has an eq method but it does not return Bool"
		}
		if len(m.Params) != 1 {
			return "entity '" + t.Name + "' eq method must take exactly one parameter (other: " + t.Name + ")"
		}
		if m.Params[0].Type == nil || !(m.Params[0].Type.IsEntity && m.Params[0].Type.Name == t.Name) {
			return "entity '" + t.Name + "' eq method must take a parameter of type " + t.Name
		}
		return ""
	}
	if t.IsGeneric {
		// Array<T>, Option<T>, Result<T,E>: each type param must itself be comparable.
		for _, tp := range t.TypeParams {
			if r := assertEqUnsupportedReason(tp); r != "" {
				return "assert_eq on " + t.String() + " is unsupported because " + r
			}
		}
	}
	return ""
}
