package codegen

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/lexer"
)

// Deprecated: Generate uses the legacy AST-to-Rust pipeline.
// Use ir.Lower + rustbe.Generate instead. Kept for testgen compatibility.
func Generate(prog *ast.Program) string {
	g := &generator{
		entities:  make(map[string]*ast.EntityDecl),
		enums:     make(map[string]*ast.EnumDecl),
		functions: make(map[string]*ast.FunctionDecl),
		oldExprs:  make(map[string]string),
	}

	// Build entity map for type resolution
	for _, e := range prog.Entities {
		g.entities[e.Name] = e
	}

	// Build enum map for variant lookup
	for _, e := range prog.Enums {
		g.enums[e.Name] = e
	}

	// Build function map for parameter type resolution
	for _, f := range prog.Functions {
		g.functions[f.Name] = f
	}

	// Generate code
	g.emitLine("// Generated Rust code from Intent")
	g.emitLine("#![allow(unused_parens, unused_variables, dead_code)]")
	g.emitLine("")

	// Generate entities
	for _, e := range prog.Entities {
		g.generateEntity(e)
		g.emitLine("")
	}

	// Generate enums
	for _, e := range prog.Enums {
		g.generateEnumDecl(e)
		g.emitLine("")
	}

	// Generate functions
	for _, f := range prog.Functions {
		g.generateFunction(f)
		g.emitLine("")
	}

	// Generate intents
	for _, i := range prog.Intents {
		g.generateIntent(i)
		g.emitLine("")
	}

	return g.sb.String()
}

// Deprecated: GenerateAll uses the legacy AST-to-Rust pipeline.
// Use ir.LowerAll + rustbe.GenerateAll instead. Kept for testgen compatibility.
func GenerateAll(registry map[string]*ast.Program, sortedPaths []string) string {
	if len(sortedPaths) == 0 {
		return ""
	}

	entryPath := sortedPaths[len(sortedPaths)-1]

	// Build module manglings map: module name -> function prefix
	moduleManglings := make(map[string]string)
	for _, filePath := range sortedPaths {
		modName := strings.TrimSuffix(filepath.Base(filePath), ".intent")
		if filePath != entryPath {
			moduleManglings[modName] = modName + "_"
		}
	}

	var sb strings.Builder
	sb.WriteString("// Generated Rust code from Intent (multi-file)\n")
	sb.WriteString("#![allow(unused_parens, unused_variables, dead_code)]\n\n")

	// Generate code for each module in dependency order
	for _, filePath := range sortedPaths {
		prog := registry[filePath]
		if prog == nil {
			continue
		}

		modName := strings.TrimSuffix(filepath.Base(filePath), ".intent")
		isEntry := filePath == entryPath

		g := &generator{
			entities:        make(map[string]*ast.EntityDecl),
			enums:           make(map[string]*ast.EnumDecl),
			functions:       make(map[string]*ast.FunctionDecl),
			oldExprs:        make(map[string]string),
			isEntryFile:     isEntry,
			moduleManglings: moduleManglings,
		}

		if !isEntry {
			g.namePrefix = modName + "_"
			// Capitalize first letter for struct prefix
			g.structPrefix = strings.ToUpper(modName[:1]) + modName[1:]
		}

		// Build entity/enum/function maps from this module
		for _, e := range prog.Entities {
			g.entities[e.Name] = e
		}
		for _, e := range prog.Enums {
			g.enums[e.Name] = e
		}
		for _, f := range prog.Functions {
			g.functions[f.Name] = f
		}

		// Also register entities/enums from imported modules for type resolution
		for _, imp := range prog.Imports {
			importedModName := strings.TrimSuffix(filepath.Base(imp.Path), ".intent")
			for _, otherPath := range sortedPaths {
				otherModName := strings.TrimSuffix(filepath.Base(otherPath), ".intent")
				if otherModName == importedModName {
					otherProg := registry[otherPath]
					if otherProg != nil {
						for _, e := range otherProg.Entities {
							if e.IsPublic {
								g.entities[e.Name] = e
							}
						}
						for _, e := range otherProg.Enums {
							if e.IsPublic {
								g.enums[e.Name] = e
							}
						}
						for _, f := range otherProg.Functions {
							if f.IsPublic {
								g.functions[f.Name] = f
							}
						}
					}
					break
				}
			}
		}

		// Generate entities
		for _, e := range prog.Entities {
			g.generateEntity(e)
			g.emitLine("")
		}

		// Generate enums
		for _, e := range prog.Enums {
			g.generateEnumDecl(e)
			g.emitLine("")
		}

		// Generate functions
		for _, f := range prog.Functions {
			g.generateFunction(f)
			g.emitLine("")
		}

		sb.WriteString(g.sb.String())
	}

	return sb.String()
}

type generator struct {
	sb             strings.Builder
	indent         int
	entities       map[string]*ast.EntityDecl
	enums          map[string]*ast.EnumDecl
	functions      map[string]*ast.FunctionDecl
	arrayRefParams map[string]bool // tracks array parameters passed by reference in current function
	inConstructor  bool
	inLabeledBlock bool
	ensuresContext bool
	oldExprs       map[string]string // mangled name -> original expression text

	// Override fields for ExprToRust helper
	selfVarOverride   string // when set, SelfExpr emits this instead of "self"/"__self"
	resultVarOverride string // when set, ResultExpr emits this instead of "__result"

	// Multi-file codegen fields
	namePrefix      string            // prefix for name mangling (e.g., "math_" for non-entry modules)
	structPrefix    string            // PascalCase prefix for entity/enum mangling (e.g., "Math")
	isEntryFile     bool              // true if this is the entry file (no mangling)
	moduleManglings map[string]string // module name -> function prefix (e.g., "math" -> "math_")
}

func (g *generator) emit(s string) {
	g.sb.WriteString(s)
}

func (g *generator) emitf(format string, args ...any) {
	g.sb.WriteString(fmt.Sprintf(format, args...))
}

// emitLinef emits an indented formatted line (adds indent prefix)
func (g *generator) emitLinef(format string, args ...any) {
	g.sb.WriteString(g.indentStr())
	g.sb.WriteString(fmt.Sprintf(format, args...))
}

func (g *generator) emitLine(s string) {
	if s == "" {
		g.sb.WriteString("\n")
	} else {
		g.sb.WriteString(g.indentStr())
		g.sb.WriteString(s)
		g.sb.WriteString("\n")
	}
}

func (g *generator) incIndent() {
	g.indent++
}

func (g *generator) decIndent() {
	g.indent--
}

func (g *generator) indentStr() string {
	return strings.Repeat("    ", g.indent)
}

// generateFunction generates a Rust function from an Intent function declaration
func (g *generator) generateFunction(f *ast.FunctionDecl) {
	if f.IsEntry {
		// Generate entry function as __intent_main
		g.emitLine("fn __intent_main() -> i64 {")
		g.incIndent()
		g.generateBlock(f.Body)
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
		// Generate main wrapper
		g.emitLine("fn main() {")
		g.incIndent()
		g.emitLine("let __exit_code = __intent_main();")
		g.emitLine("std::process::exit(__exit_code as i32);")
		g.decIndent()
		g.emitLine("}")
	} else {
		// Regular function
		// Track array parameters passed by reference
		g.arrayRefParams = make(map[string]bool)
		for _, p := range f.Params {
			if p.Type.Name == "Array" {
				g.arrayRefParams[p.Name] = true
			}
		}

		// Apply name mangling for non-entry modules
		fnName := f.Name
		if g.namePrefix != "" {
			fnName = g.namePrefix + f.Name
		}

		g.emitLinef("fn %s(", fnName)
		for i, p := range f.Params {
			if i > 0 {
				g.emit(", ")
			}
			// Array parameters should be passed by reference to avoid ownership issues
			paramType := g.mapType(p.Type)
			if p.Type.Name == "Array" {
				paramType = "&" + paramType
			}
			g.emitf("%s: %s", p.Name, paramType)
		}
		g.emitf(") -> %s {\n", g.mapType(f.ReturnType))
		g.incIndent()

		// Generate requires clauses
		for _, req := range f.Requires {
			g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
				g.generateExpr(req.Expr), escapeRustString(req.RawText))
		}

		// Check if we need labeled block pattern (non-Void return with ensures)
		needsLabeledBlock := len(f.Ensures) > 0 && f.ReturnType.Name != "Void"

		if needsLabeledBlock {
			g.emitLinef("let __result: %s = 'body: {\n", g.mapType(f.ReturnType))
			g.incIndent()
			g.inLabeledBlock = true
			g.generateBlock(f.Body)
			g.inLabeledBlock = false
			g.decIndent()
			g.emitLine("};")

			// Generate ensures clauses
			g.ensuresContext = true
			for _, ens := range f.Ensures {
				g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false

			g.emitLine("__result")
		} else {
			// Simple case: just generate body
			g.generateBlock(f.Body)

			// For Void functions with ensures, add assertions at end
			if len(f.Ensures) > 0 {
				g.ensuresContext = true
				for _, ens := range f.Ensures {
					g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
						g.generateExpr(ens.Expr), escapeRustString(ens.RawText))
				}
				g.ensuresContext = false
			}
		}

		g.decIndent()
		g.emitLine("}")
		// Clear array ref params after function generation
		g.arrayRefParams = nil
	}
}

// mangledEntityName returns the mangled entity name for Rust codegen
func (g *generator) mangledEntityName(name string) string {
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

// generateEntity generates a Rust struct and impl block for an entity
func (g *generator) generateEntity(e *ast.EntityDecl) {
	mangledName := g.mangledEntityName(e.Name)

	// Generate struct
	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("struct %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.mapType(f.Type))
	}
	g.decIndent()
	g.emitLine("}")
	g.emitLine("")

	// Generate impl block
	g.emitLinef("impl %s {\n", mangledName)
	g.incIndent()

	// Generate invariant check method if there are invariants
	if len(e.Invariants) > 0 {
		g.emitLine("fn __check_invariants(&self) {")
		g.incIndent()
		for _, inv := range e.Invariants {
			g.emitLinef("assert!(%s, \"Invariant failed: %s\");\n",
				g.generateExpr(inv.Expr), escapeRustString(inv.RawText))
		}
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
	}

	// Generate constructor
	if e.Constructor != nil {
		g.generateConstructor(e)
		g.emitLine("")
	}

	// Generate methods
	for _, m := range e.Methods {
		g.generateMethod(e, m)
		g.emitLine("")
	}

	g.decIndent()
	g.emitLine("}")
}

// mangledEnumName returns the mangled enum name for Rust codegen
func (g *generator) mangledEnumName(name string) string {
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

// generateEnumDecl generates a Rust enum from an Intent enum declaration
func (g *generator) generateEnumDecl(e *ast.EnumDecl) {
	mangledName := g.mangledEnumName(e.Name)

	// Add derive macros for usability
	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("enum %s {\n", mangledName)
	g.incIndent()
	for _, v := range e.Variants {
		if len(v.Fields) == 0 {
			// Unit variant
			g.emitLinef("%s,\n", v.Name)
		} else {
			// Data-carrying variant with named fields
			g.emitf("%s%s { ", g.indentStr(), v.Name)
			for i, f := range v.Fields {
				if i > 0 {
					g.emit(", ")
				}
				g.emitf("%s: %s", f.Name, g.mapType(f.Type))
			}
			g.emit(" },\n")
		}
	}
	g.decIndent()
	g.emitLine("}")
}

// generateConstructor generates a Rust constructor (new function)
func (g *generator) generateConstructor(e *ast.EntityDecl) {
	mangledName := g.mangledEntityName(e.Name)

	g.emitLinef("fn new(")
	for i, p := range e.Constructor.Params {
		if i > 0 {
			g.emit(", ")
		}
		g.emitf("%s: %s", p.Name, g.mapType(p.Type))
	}
	g.emitf(") -> %s {\n", mangledName)
	g.incIndent()

	// Generate requires clauses
	for _, req := range e.Constructor.Requires {
		g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr), escapeRustString(req.RawText))
	}

	// Initialize with default values
	g.emitLinef("let mut __self = %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.defaultValue(f.Type))
	}
	g.decIndent()
	g.emitLine("};")

	// Generate constructor body
	g.inConstructor = true
	g.generateBlock(e.Constructor.Body)
	// Keep inConstructor=true for ensures clauses so self becomes __self

	// Generate ensures clauses
	g.ensuresContext = true
	for _, ens := range e.Constructor.Ensures {
		g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
			g.generateExpr(ens.Expr), escapeRustString(ens.RawText))
	}
	g.ensuresContext = false
	g.inConstructor = false

	// Check invariants
	if len(e.Invariants) > 0 {
		g.emitLine("__self.__check_invariants();")
	}

	g.emitLine("__self")
	g.decIndent()
	g.emitLine("}")
}

// generateMethod generates a Rust method
func (g *generator) generateMethod(e *ast.EntityDecl, m *ast.MethodDecl) {
	// Clear old expressions for this method
	g.oldExprs = make(map[string]string)

	// Collect old() expressions from ensures clauses
	for _, ens := range m.Ensures {
		g.collectOldExprs(ens.Expr)
	}

	g.emitLinef("fn %s(&mut self", m.Name)
	for _, p := range m.Params {
		g.emitf(", %s: %s", p.Name, g.mapType(p.Type))
	}
	if m.ReturnType.Name == "Void" {
		g.emit(") {\n")
	} else {
		g.emitf(") -> %s {\n", g.mapType(m.ReturnType))
	}
	g.incIndent()

	// Generate old value captures
	for mangledName, exprText := range g.oldExprs {
		g.emitLinef("let %s = %s;\n", mangledName, exprText)
	}

	// Generate requires clauses
	for _, req := range m.Requires {
		g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr), escapeRustString(req.RawText))
	}

	// Use labeled block pattern when method returns a value and has
	// ensures or invariants (so post-checks run after all return paths)
	hasInvariants := len(e.Invariants) > 0
	needsLabeledBlock := m.ReturnType.Name != "Void" && (len(m.Ensures) > 0 || hasInvariants)

	if needsLabeledBlock {
		g.emitLinef("let __result: %s = 'body: {\n", g.mapType(m.ReturnType))
		g.incIndent()
		g.inLabeledBlock = true
		g.generateBlock(m.Body)
		g.inLabeledBlock = false
		g.decIndent()
		g.emitLine("};")

		// Generate ensures clauses
		g.ensuresContext = true
		for _, ens := range m.Ensures {
			g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
				g.generateExpr(ens.Expr), escapeRustString(ens.RawText))
		}
		g.ensuresContext = false

		// Check invariants
		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}

		g.emitLine("__result")
	} else {
		// Void method or no post-checks needed
		g.generateBlock(m.Body)

		// For methods with ensures (Void), add assertions at end
		if len(m.Ensures) > 0 {
			g.ensuresContext = true
			for _, ens := range m.Ensures {
				g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false
		}

		// Check invariants (Void methods)
		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}
	}

	g.decIndent()
	g.emitLine("}")
}

// generateIntent generates structured comments for an intent block
func (g *generator) generateIntent(i *ast.IntentDecl) {
	g.emitLinef("// Intent: %s\n", i.Description)
	for _, goal := range i.Goals {
		g.emitLinef("// Goal: %s\n", goal)
	}
	for _, constraint := range i.Constraints {
		g.emitLinef("// Constraint: %s\n", constraint)
	}
	for _, guarantee := range i.Guarantees {
		g.emitLinef("// Guarantee: %s\n", guarantee)
	}
	for _, vb := range i.VerifiedBy {
		g.emitLinef("// Verified by: %s\n", strings.Join(vb.Parts, "."))
	}
	g.emitLine("")

	// Generate test module stub
	testName := g.mangleIntentName(i.Description)
	g.emitLine("#[cfg(test)]")
	g.emitLinef("mod %s {\n", testName)
	g.incIndent()
	g.emitLine("// Intent verification completed at compile time.")
	g.decIndent()
	g.emitLine("}")
}

// generateBlock generates statements for a block
func (g *generator) generateBlock(b *ast.Block) {
	for _, stmt := range b.Statements {
		g.generateStmt(stmt)
	}
}

// generateStmt generates a statement
func (g *generator) generateStmt(s ast.Statement) {
	switch stmt := s.(type) {
	case *ast.LetStmt:
		// Entity-type bindings are always mut in Rust (methods take &mut self)
		isMut := stmt.Mutable || g.isEntityType(stmt.Type)
		valueExpr := g.generateExpr(stmt.Value)

		// If binding an array and the value is an identifier that's an array ref param, clone it
		if stmt.Type != nil && stmt.Type.Name == "Array" {
			if ident, ok := stmt.Value.(*ast.Identifier); ok {
				if g.arrayRefParams[ident.Name] {
					valueExpr = valueExpr + ".clone()"
				}
			}
		}

		if isMut {
			g.emitLinef("let mut %s: %s = %s;\n",
				stmt.Name, g.mapType(stmt.Type), valueExpr)
		} else {
			g.emitLinef("let %s: %s = %s;\n",
				stmt.Name, g.mapType(stmt.Type), valueExpr)
		}
	case *ast.AssignStmt:
		g.emitLinef("%s = %s;\n", g.generateExpr(stmt.Target), g.generateExpr(stmt.Value))
	case *ast.ReturnStmt:
		if g.inLabeledBlock {
			if stmt.Value != nil {
				g.emitLinef("break 'body %s;\n", g.generateExpr(stmt.Value))
			} else {
				g.emitLine("break 'body;")
			}
		} else {
			if stmt.Value != nil {
				g.emitLinef("return %s;\n", g.generateExpr(stmt.Value))
			} else {
				g.emitLine("return;")
			}
		}
	case *ast.WhileStmt:
		hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil

		if hasContracts {
			// Open a scope block for old() captures and decreases tracking
			g.emitLine("{")
			g.incIndent()

			// Collect and emit old() value captures from invariants
			savedOldExprs := g.oldExprs
			g.oldExprs = make(map[string]string)
			for _, inv := range stmt.Invariants {
				g.collectOldExprs(inv.Expr)
			}
			for mangledName, exprText := range g.oldExprs {
				g.emitLinef("let %s = %s;\n", mangledName, exprText)
			}

			// Check invariants at loop entry
			savedEnsures := g.ensuresContext
			g.ensuresContext = true // Enable old() substitution
			for _, inv := range stmt.Invariants {
				g.emitLinef("assert!(%s, \"Loop invariant failed at entry: %s\");\n",
					g.generateExpr(inv.Expr), escapeRustString(inv.RawText))
			}
			g.ensuresContext = savedEnsures

			// Initialize decreases tracking
			if stmt.Decreases != nil {
				metricExpr := g.generateExpr(stmt.Decreases.Expr)
				g.emitLinef("let mut __decreases_prev: i64 = %s;\n", metricExpr)
				g.emitLinef("assert!(__decreases_prev >= 0, \"Decreases metric must be non-negative at entry: %s\");\n",
					escapeRustString(stmt.Decreases.RawText))
			}

			// Emit while loop
			g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition))
			g.incIndent()

			// Loop body
			g.generateBlock(stmt.Body)

			// Check invariants after each iteration
			g.ensuresContext = true
			for _, inv := range stmt.Invariants {
				g.emitLinef("assert!(%s, \"Loop invariant failed after iteration: %s\");\n",
					g.generateExpr(inv.Expr), escapeRustString(inv.RawText))
			}
			g.ensuresContext = savedEnsures

			// Check decreases metric
			if stmt.Decreases != nil {
				metricExpr := g.generateExpr(stmt.Decreases.Expr)
				g.emitLinef("let __decreases_next: i64 = %s;\n", metricExpr)
				g.emitLinef("assert!(__decreases_next < __decreases_prev, \"Termination metric did not decrease: %s\");\n",
					escapeRustString(stmt.Decreases.RawText))
				g.emitLinef("assert!(__decreases_next >= 0, \"Termination metric became negative: %s\");\n",
					escapeRustString(stmt.Decreases.RawText))
				g.emitLine("__decreases_prev = __decreases_next;")
			}

			g.decIndent()
			g.emitLine("}") // end while

			// Restore old expressions
			g.oldExprs = savedOldExprs

			g.decIndent()
			g.emitLine("}") // end scope block
		} else {
			// No contracts -- simple while loop (existing behavior)
			g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition))
			g.incIndent()
			g.generateBlock(stmt.Body)
			g.decIndent()
			g.emitLine("}")
		}
	case *ast.ForInStmt:
		g.generateForInStmt(stmt)
	case *ast.BreakStmt:
		g.emitLine("break;")
	case *ast.ContinueStmt:
		g.emitLine("continue;")
	case *ast.IfStmt:
		g.emitLinef("if %s {\n", g.generateExpr(stmt.Condition))
		g.incIndent()
		g.generateBlock(stmt.Then)
		g.decIndent()
		g.emitLinef("}")
		if stmt.Else != nil {
			if elseIf, ok := stmt.Else.(*ast.IfStmt); ok {
				g.emit(" else ")
				g.emitf("if %s {\n", g.generateExpr(elseIf.Condition))
				g.incIndent()
				g.generateBlock(elseIf.Then)
				g.decIndent()
				g.emitLinef("}")
				if elseIf.Else != nil {
					g.emit(" else {\n")
					g.incIndent()
					g.generateStmt(elseIf.Else)
					g.decIndent()
					g.emitLinef("}\n")
				} else {
					g.emit("\n")
				}
			} else if elseBlock, ok := stmt.Else.(*ast.Block); ok {
				g.emit(" else {\n")
				g.incIndent()
				g.generateBlock(elseBlock)
				g.decIndent()
				g.emitLine("}")
			}
		} else {
			g.emit("\n")
		}
	case *ast.ExprStmt:
		g.emitLinef("%s;\n", g.generateExpr(stmt.Expr))
	case *ast.Block:
		g.generateBlock(stmt)
	}
}

// generateForInStmt generates a for-in loop
func (g *generator) generateForInStmt(stmt *ast.ForInStmt) {
	g.emit(g.indentStr())
	g.emitf("for %s in ", stmt.Variable)

	if rangeExpr, ok := stmt.Iterable.(*ast.RangeExpr); ok {
		// Range iteration: for i in (start..end)
		g.emitf("(%s..%s)", g.generateExpr(rangeExpr.Start), g.generateExpr(rangeExpr.End))
	} else {
		// Array iteration: for x in arr.iter()
		g.emitf("%s.iter()", g.generateExpr(stmt.Iterable))
	}

	g.emit(" {\n")
	g.incIndent()
	g.generateBlock(stmt.Body)
	g.decIndent()
	g.emitLine("}")
}

// generateExpr generates an expression
func (g *generator) generateExpr(e ast.Expression) string {
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		left := g.generateExpr(expr.Left)
		right := g.generateExpr(expr.Right)
		op := g.mapOperator(expr.Op)

		// Special case: String concatenation
		if expr.Op == lexer.PLUS {
			// Check if this might be string concatenation (we'd need type info ideally)
			// For now, assume it's string concat if we see string literals
			if _, ok := expr.Left.(*ast.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
			if _, ok := expr.Right.(*ast.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
		}

		// Special case: implies
		if expr.Op == lexer.IMPLIES {
			return fmt.Sprintf("(!%s || %s)", left, right)
		}

		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *ast.UnaryExpr:
		operand := g.generateExpr(expr.Operand)
		if expr.Op == lexer.NOT {
			return fmt.Sprintf("!%s", operand)
		}
		return fmt.Sprintf("-%s", operand)

	case *ast.CallExpr:
		// Handle print() built-in
		if expr.Function == "print" && len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("println!(\"{}\", %s)", arg)
		}
		// Handle len() built-in
		if expr.Function == "len" && len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("(%s.len() as i64)", arg)
		}
		// Handle built-in Result/Option variant constructors (Ok, Err, Some)
		// These use tuple syntax, not named struct syntax
		if expr.Function == "Ok" || expr.Function == "Err" || expr.Function == "Some" {
			if len(expr.Args) == 1 {
				arg := g.generateExpr(expr.Args[0])
				return fmt.Sprintf("%s(%s)", expr.Function, arg)
			}
		}
		// Check if this is a variant constructor
		if enumDecl, variantDecl := g.lookupVariant(expr.Function); enumDecl != nil {
			return g.generateVariantConstructor(expr, enumDecl, variantDecl)
		}
		// Check if this is an entity constructor call
		if _, isEntity := g.entities[expr.Function]; isEntity {
			return g.generateEntityConstructorCall(expr)
		}
		// Regular function call
		args := make([]string, len(expr.Args))
		funcDecl := g.functions[expr.Function]
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg)
			// If the corresponding parameter is an Array type, pass by reference
			if funcDecl != nil && i < len(funcDecl.Params) {
				if funcDecl.Params[i].Type.Name == "Array" {
					// Only add & for simple identifiers (variables), not for complex expressions
					if _, ok := arg.(*ast.Identifier); ok {
						argStr = "&" + argStr
					}
				}
			}
			args[i] = argStr
		}
		return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))

	case *ast.MethodCallExpr:
		// Check if this is a module-qualified call (e.g., math.add(1, 2) or geometry.Circle(5.0))
		if ident, ok := expr.Object.(*ast.Identifier); ok && g.moduleManglings != nil {
			if _, isModule := g.moduleManglings[ident.Name]; isModule {
				args := make([]string, len(expr.Args))
				funcDecl := g.functions[expr.Method]
				for i, arg := range expr.Args {
					argStr := g.generateExpr(arg)
					// If the corresponding parameter is an Array type, pass by reference
					if funcDecl != nil && i < len(funcDecl.Params) {
						if funcDecl.Params[i].Type.Name == "Array" {
							// Only add & for simple identifiers (variables), not for complex expressions
							if _, ok := arg.(*ast.Identifier); ok {
								argStr = "&" + argStr
							}
						}
					}
					args[i] = argStr
				}

				// Check if this is an entity constructor from the other module
				if _, isEntity := g.entities[expr.Method]; isEntity {
					// Entity constructor: emit MangledName::new(args)
					modPrefix := strings.ToUpper(ident.Name[:1]) + ident.Name[1:]
					mangledStructName := modPrefix + expr.Method
					return fmt.Sprintf("%s::new(%s)", mangledStructName, strings.Join(args, ", "))
				}

				// Regular function call: emit mangled function name
				mangledFnName := ident.Name + "_" + expr.Method
				return fmt.Sprintf("%s(%s)", mangledFnName, strings.Join(args, ", "))
			}
		}

		obj := g.generateExpr(expr.Object)

		// Handle Result/Option predicate methods
		if expr.Method == "is_ok" || expr.Method == "is_err" || expr.Method == "is_some" || expr.Method == "is_none" {
			// These map directly to Rust methods
			return fmt.Sprintf("%s.%s()", obj, expr.Method)
		}

		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg)
		}
		return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))

	case *ast.FieldAccessExpr:
		obj := g.generateExpr(expr.Object)
		return fmt.Sprintf("%s.%s", obj, expr.Field)

	case *ast.OldExpr:
		// In ensures context, replace with captured old value
		if g.ensuresContext {
			mangledName := g.mangleOldExpr(expr.Expr)
			return mangledName
		}
		// Shouldn't happen, but fallback to current value
		return g.generateExpr(expr.Expr)

	case *ast.Identifier:
		// Handle built-in None (Option unit variant)
		if expr.Name == "None" {
			return "None"
		}
		// Check if it's a unit variant
		if enumDecl, variantDecl := g.lookupVariant(expr.Name); enumDecl != nil && len(variantDecl.Fields) == 0 {
			return fmt.Sprintf("%s::%s", enumDecl.Name, expr.Name)
		}
		return expr.Name

	case *ast.SelfExpr:
		if g.selfVarOverride != "" {
			return g.selfVarOverride
		}
		if g.inConstructor {
			return "__self"
		}
		return "self"

	case *ast.ResultExpr:
		if g.resultVarOverride != "" {
			return g.resultVarOverride
		}
		return "__result"

	case *ast.IntLit:
		return expr.Value + "i64"

	case *ast.FloatLit:
		return expr.Value

	case *ast.StringLit:
		// String literals become owned Strings in Rust
		return expr.Value + ".to_string()"

	case *ast.StringInterp:
		// Generate format!() for interpolated strings
		var fmtStr string
		var args []string
		for _, part := range expr.Parts {
			if part.IsExpr {
				fmtStr += "{}"
				args = append(args, g.generateExpr(part.Expr))
			} else {
				escaped := strings.ReplaceAll(part.Static, "{", "{{")
				escaped = strings.ReplaceAll(escaped, "}", "}}")
				escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
				escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
				fmtStr += escaped
			}
		}
		if len(args) == 0 {
			return fmt.Sprintf("\"%s\".to_string()", fmtStr)
		}
		return fmt.Sprintf("format!(\"%s\", %s)", fmtStr, strings.Join(args, ", "))

	case *ast.BoolLit:
		if expr.Value {
			return "true"
		}
		return "false"

	case *ast.ArrayLit:
		if len(expr.Elements) == 0 {
			return "Vec::new()"
		}
		elems := make([]string, len(expr.Elements))
		for i, elem := range expr.Elements {
			elems[i] = g.generateExpr(elem)
		}
		return fmt.Sprintf("vec![%s]", strings.Join(elems, ", "))

	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s as usize]", g.generateExpr(expr.Object), g.generateExpr(expr.Index))

	case *ast.RangeExpr:
		return fmt.Sprintf("(%s..%s)", g.generateExpr(expr.Start), g.generateExpr(expr.End))

	case *ast.ForallExpr:
		return g.generateForallExpr(expr)

	case *ast.ExistsExpr:
		return g.generateExistsExpr(expr)

	case *ast.MatchExpr:
		return g.generateMatchExpr(expr)

	case *ast.TryExpr:
		return g.generateExpr(expr.Expr) + "?"

	default:
		return "<unknown>"
	}
}

// generateEntityConstructorCall generates Entity::new(args)
func (g *generator) generateEntityConstructorCall(c *ast.CallExpr) string {
	args := make([]string, len(c.Args))
	for i, arg := range c.Args {
		args[i] = g.generateExpr(arg)
	}
	return fmt.Sprintf("%s::new(%s)", c.Function, strings.Join(args, ", "))
}

// lookupVariant searches for a variant by name across all enums
func (g *generator) lookupVariant(name string) (*ast.EnumDecl, *ast.EnumVariant) {
	for _, enumDecl := range g.enums {
		for _, variant := range enumDecl.Variants {
			if variant.Name == name {
				return enumDecl, variant
			}
		}
	}
	return nil, nil
}

// generateVariantConstructor generates variant constructor: EnumName::VariantName or EnumName::VariantName { field: value }
func (g *generator) generateVariantConstructor(expr *ast.CallExpr, enumDecl *ast.EnumDecl, variantDecl *ast.EnumVariant) string {
	// Unit variant: EnumName::VariantName (should not happen in CallExpr, but handle it)
	if len(variantDecl.Fields) == 0 {
		return fmt.Sprintf("%s::%s", enumDecl.Name, expr.Function)
	}
	// Data variant: EnumName::VariantName { field1: arg1, field2: arg2 }
	var sb strings.Builder
	sb.WriteString(enumDecl.Name)
	sb.WriteString("::")
	sb.WriteString(expr.Function)
	sb.WriteString(" { ")
	for i, f := range variantDecl.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(": ")
		sb.WriteString(g.generateExpr(expr.Args[i]))
	}
	sb.WriteString(" }")
	return sb.String()
}

// collectOldExprs walks an expression tree and collects all old() expressions
func (g *generator) collectOldExprs(e ast.Expression) {
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		g.collectOldExprs(expr.Left)
		g.collectOldExprs(expr.Right)
	case *ast.UnaryExpr:
		g.collectOldExprs(expr.Operand)
	case *ast.CallExpr:
		for _, arg := range expr.Args {
			g.collectOldExprs(arg)
		}
	case *ast.MethodCallExpr:
		g.collectOldExprs(expr.Object)
		for _, arg := range expr.Args {
			g.collectOldExprs(arg)
		}
	case *ast.FieldAccessExpr:
		g.collectOldExprs(expr.Object)
	case *ast.OldExpr:
		// Found an old() expression - capture it
		mangledName := g.mangleOldExpr(expr.Expr)
		rustExpr := g.generateExpr(expr.Expr)
		g.oldExprs[mangledName] = rustExpr
		// Recursively collect old exprs in the argument
		g.collectOldExprs(expr.Expr)
	case *ast.ForallExpr:
		// Walk into quantifier domain and body
		if expr.Domain != nil {
			g.collectOldExprs(expr.Domain.Start)
			g.collectOldExprs(expr.Domain.End)
		}
		g.collectOldExprs(expr.Body)
	case *ast.ExistsExpr:
		// Walk into quantifier domain and body
		if expr.Domain != nil {
			g.collectOldExprs(expr.Domain.Start)
			g.collectOldExprs(expr.Domain.End)
		}
		g.collectOldExprs(expr.Body)
	case *ast.IndexExpr:
		g.collectOldExprs(expr.Object)
		g.collectOldExprs(expr.Index)
	}
}

// mangleOldExpr creates a mangled variable name for an old() expression
func (g *generator) mangleOldExpr(e ast.Expression) string {
	text := g.exprToText(e)
	// Replace dots with underscores, remove parentheses
	text = strings.ReplaceAll(text, ".", "_")
	text = strings.ReplaceAll(text, "(", "")
	text = strings.ReplaceAll(text, ")", "")
	text = strings.ReplaceAll(text, " ", "_")
	return "__old_" + text
}

// exprToText converts an expression to a simple text representation
func (g *generator) exprToText(e ast.Expression) string {
	switch expr := e.(type) {
	case *ast.FieldAccessExpr:
		return g.exprToText(expr.Object) + "." + expr.Field
	case *ast.SelfExpr:
		return "self"
	case *ast.Identifier:
		return expr.Name
	default:
		return "expr"
	}
}

// mangleIntentName converts an intent description to a valid Rust identifier
func (g *generator) mangleIntentName(desc string) string {
	// Convert to lowercase
	desc = strings.ToLower(desc)
	// Replace non-alphanumeric with underscore
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	desc = reg.ReplaceAllString(desc, "_")
	// Remove leading/trailing underscores
	desc = strings.Trim(desc, "_")
	// Ensure it starts with a letter or underscore
	if len(desc) > 0 && desc[0] >= '0' && desc[0] <= '9' {
		desc = "_" + desc
	}
	if desc == "" {
		desc = "__intent"
	}
	return "__intent_" + desc
}

// mapType maps Intent type to Rust type
func (g *generator) mapType(t *ast.TypeRef) string {
	if t == nil {
		return "()"
	}
	switch t.Name {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "String":
		return "String"
	case "Bool":
		return "bool"
	case "Void":
		return "()"
	case "Array":
		if len(t.TypeArgs) == 1 {
			return "Vec<" + g.mapType(t.TypeArgs[0]) + ">"
		}
		return "Vec<_>" // fallback, should not happen after checker
	case "Result":
		if len(t.TypeArgs) == 2 {
			return "Result<" + g.mapType(t.TypeArgs[0]) + ", " + g.mapType(t.TypeArgs[1]) + ">"
		}
		return "Result<_, _>" // fallback, should not happen after checker
	case "Option":
		if len(t.TypeArgs) == 1 {
			return "Option<" + g.mapType(t.TypeArgs[0]) + ">"
		}
		return "Option<_>" // fallback, should not happen after checker
	default:
		// Entity or enum type
		return t.Name
	}
}

// isEntityType checks if a type refers to a known entity
func (g *generator) isEntityType(t *ast.TypeRef) bool {
	_, ok := g.entities[t.Name]
	return ok
}

// mapOperator maps Intent operator to Rust operator
func (g *generator) mapOperator(op lexer.TokenType) string {
	switch op {
	case lexer.PLUS:
		return "+"
	case lexer.MINUS:
		return "-"
	case lexer.STAR:
		return "*"
	case lexer.SLASH:
		return "/"
	case lexer.PERCENT:
		return "%"
	case lexer.EQ:
		return "=="
	case lexer.NEQ:
		return "!="
	case lexer.LT:
		return "<"
	case lexer.GT:
		return ">"
	case lexer.LEQ:
		return "<="
	case lexer.GEQ:
		return ">="
	case lexer.AND:
		return "&&"
	case lexer.OR:
		return "||"
	default:
		return "?"
	}
}

// defaultValue returns the default Rust value for a type
func (g *generator) defaultValue(t *ast.TypeRef) string {
	switch t.Name {
	case "Int":
		return "0i64"
	case "Float":
		return "0.0"
	case "String":
		return "String::new()"
	case "Bool":
		return "false"
	case "Array":
		return "Vec::new()"
	default:
		// Entity type - would need constructor call, but for now use default
		return fmt.Sprintf("%s { /* default fields */ }", t.Name)
	}
}

// generateForallExpr generates a runtime loop for forall quantifier
func (g *generator) generateForallExpr(expr *ast.ForallExpr) string {
	rangeStart := g.generateExpr(expr.Domain.Start)
	rangeEnd := g.generateExpr(expr.Domain.End)
	body := g.generateExpr(expr.Body)

	return fmt.Sprintf("{\n"+
		"    let mut __forall_holds = true;\n"+
		"    for %s in (%s..%s) {\n"+
		"        if !(%s) {\n"+
		"            __forall_holds = false;\n"+
		"            break;\n"+
		"        }\n"+
		"    }\n"+
		"    __forall_holds\n"+
		"}", expr.Variable, rangeStart, rangeEnd, body)
}

// generateExistsExpr generates a runtime loop for exists quantifier
func (g *generator) generateExistsExpr(expr *ast.ExistsExpr) string {
	rangeStart := g.generateExpr(expr.Domain.Start)
	rangeEnd := g.generateExpr(expr.Domain.End)
	body := g.generateExpr(expr.Body)

	return fmt.Sprintf("{\n"+
		"    let mut __exists_found = false;\n"+
		"    for %s in (%s..%s) {\n"+
		"        if %s {\n"+
		"            __exists_found = true;\n"+
		"            break;\n"+
		"        }\n"+
		"    }\n"+
		"    __exists_found\n"+
		"}", expr.Variable, rangeStart, rangeEnd, body)
}

// generateMatchExpr generates a Rust match expression
func (g *generator) generateMatchExpr(expr *ast.MatchExpr) string {
	var buf strings.Builder
	buf.WriteString("match ")
	buf.WriteString(g.generateExpr(expr.Scrutinee))
	buf.WriteString(" {\n")

	g.incIndent()
	for _, arm := range expr.Arms {
		buf.WriteString(g.indentStr())
		buf.WriteString(g.generateMatchPattern(arm.Pattern))
		buf.WriteString(" => ")
		buf.WriteString(g.generateExpr(arm.Body))
		buf.WriteString(",\n")
	}
	g.decIndent()

	buf.WriteString(g.indentStr())
	buf.WriteString("}")
	return buf.String()
}

// generateMatchPattern generates a Rust match pattern
func (g *generator) generateMatchPattern(pattern *ast.MatchPattern) string {
	// Wildcard pattern
	if pattern.IsWildcard {
		return "_"
	}

	// Handle built-in Result/Option variants (use tuple syntax)
	if pattern.VariantName == "Ok" || pattern.VariantName == "Err" || pattern.VariantName == "Some" {
		if len(pattern.Bindings) == 1 {
			return fmt.Sprintf("%s(%s)", pattern.VariantName, pattern.Bindings[0])
		}
		// Fallback if no bindings (shouldn't happen for these)
		return pattern.VariantName
	}
	if pattern.VariantName == "None" {
		return "None"
	}

	// Find enum name for this variant
	enumName := g.resolveEnumNameForVariant(pattern.VariantName)

	// Unit variant (no bindings)
	if len(pattern.Bindings) == 0 {
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	// Data-carrying variant (with bindings)
	// Look up variant declaration to get field names
	enumDecl := g.enums[enumName]
	if enumDecl == nil {
		// Fallback: shouldn't happen if checker passed
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	var variant *ast.EnumVariant
	for _, v := range enumDecl.Variants {
		if v.Name == pattern.VariantName {
			variant = v
			break
		}
	}

	if variant == nil {
		// Fallback: shouldn't happen if checker passed
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	// Build field: binding pairs
	var fields []string
	for i, binding := range pattern.Bindings {
		if i < len(variant.Fields) {
			fieldName := variant.Fields[i].Name
			fields = append(fields, fmt.Sprintf("%s: %s", fieldName, binding))
		}
	}

	return fmt.Sprintf("%s::%s { %s }", enumName, pattern.VariantName, strings.Join(fields, ", "))
}

// resolveEnumNameForVariant finds the enum name that contains the given variant
func (g *generator) resolveEnumNameForVariant(variantName string) string {
	for enumName, enumDecl := range g.enums {
		for _, v := range enumDecl.Variants {
			if v.Name == variantName {
				return enumName
			}
		}
	}
	// Fallback: shouldn't happen if checker passed
	return "UnknownEnum"
}

// MapType converts an Intent TypeRef to its Rust type string.
func MapType(t *ast.TypeRef) string {
	g := &generator{}
	return g.mapType(t)
}

// ExprToRust converts a contract expression to Rust source.
// selfVar: what "self" maps to (e.g., "__entity"), resultVar: what "result" maps to.
// ensuresCtx enables old() substitution.
func ExprToRust(expr ast.Expression, selfVar, resultVar string, ensuresCtx bool, entities map[string]*ast.EntityDecl, enums map[string]*ast.EnumDecl, functions map[string]*ast.FunctionDecl) string {
	if entities == nil {
		entities = make(map[string]*ast.EntityDecl)
	}
	if enums == nil {
		enums = make(map[string]*ast.EnumDecl)
	}
	if functions == nil {
		functions = make(map[string]*ast.FunctionDecl)
	}
	g := &generator{
		entities:          entities,
		enums:             enums,
		functions:         functions,
		oldExprs:          make(map[string]string),
		ensuresContext:    ensuresCtx,
		selfVarOverride:   selfVar,
		resultVarOverride: resultVar,
	}
	return g.generateExpr(expr)
}

// EscapeRustString escapes a string for use in Rust string literals.
func EscapeRustString(s string) string {
	return escapeRustString(s)
}

// escapeRustString escapes a string for use in Rust string literals
func escapeRustString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
