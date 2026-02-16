package rustbe

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

// Generate produces Rust source code from a single IR Module.
func Generate(mod *ir.Module) string {
	g := &generator{
		entities:  make(map[string]*ir.Entity),
		enums:     make(map[string]*ir.Enum),
		functions: make(map[string]*ir.Function),
	}

	for _, e := range mod.Entities {
		g.entities[e.Name] = e
	}
	for _, e := range mod.Enums {
		g.enums[e.Name] = e
	}
	for _, f := range mod.Functions {
		g.functions[f.Name] = f
	}

	g.emitLine("// Generated Rust code from Intent")
	g.emitLine("#![allow(unused_parens, unused_variables, dead_code)]")
	g.emitLine("")

	for _, e := range mod.Entities {
		g.generateEntity(e)
		g.emitLine("")
	}
	for _, e := range mod.Enums {
		g.generateEnumDecl(e)
		g.emitLine("")
	}
	for _, f := range mod.Functions {
		g.generateFunction(f)
		g.emitLine("")
	}
	for _, i := range mod.Intents {
		g.generateIntent(i)
		g.emitLine("")
	}

	return g.sb.String()
}

// GenerateAll produces Rust source from a multi-file IR Program.
func GenerateAll(prog *ir.Program) string {
	if len(prog.Modules) == 0 {
		return ""
	}

	// Build module manglings map
	moduleManglings := make(map[string]string)
	for _, mod := range prog.Modules {
		if !mod.IsEntry {
			moduleManglings[mod.Name] = mod.Name + "_"
		}
	}

	var sb strings.Builder
	sb.WriteString("// Generated Rust code from Intent (multi-file)\n")
	sb.WriteString("#![allow(unused_parens, unused_variables, dead_code)]\n\n")

	for _, mod := range prog.Modules {
		g := &generator{
			entities:        make(map[string]*ir.Entity),
			enums:           make(map[string]*ir.Enum),
			functions:       make(map[string]*ir.Function),
			isEntryFile:     mod.IsEntry,
			moduleManglings: moduleManglings,
		}

		if !mod.IsEntry {
			g.namePrefix = mod.Name + "_"
			g.structPrefix = strings.ToUpper(mod.Name[:1]) + mod.Name[1:]
		}

		for _, e := range mod.Entities {
			g.entities[e.Name] = e
		}
		for _, e := range mod.Enums {
			g.enums[e.Name] = e
		}
		for _, f := range mod.Functions {
			g.functions[f.Name] = f
		}

		for _, e := range mod.Entities {
			g.generateEntity(e)
			g.emitLine("")
		}
		for _, e := range mod.Enums {
			g.generateEnumDecl(e)
			g.emitLine("")
		}
		for _, f := range mod.Functions {
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
	entities       map[string]*ir.Entity
	enums          map[string]*ir.Enum
	functions      map[string]*ir.Function
	inConstructor  bool
	inLabeledBlock bool
	ensuresContext bool

	// Multi-file fields
	namePrefix      string
	structPrefix    string
	isEntryFile     bool
	moduleManglings map[string]string
}

func (g *generator) emit(s string) {
	g.sb.WriteString(s)
}

func (g *generator) emitf(format string, args ...any) {
	g.sb.WriteString(fmt.Sprintf(format, args...))
}

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

func (g *generator) incIndent() { g.indent++ }
func (g *generator) decIndent() { g.indent-- }

func (g *generator) indentStr() string {
	return strings.Repeat("    ", g.indent)
}

// --- Type mapping ---

func (g *generator) mapType(t *checker.Type) string {
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
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Vec<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Vec<_>"
	case "Result":
		if t.IsGeneric && len(t.TypeParams) == 2 {
			return "Result<" + g.mapType(t.TypeParams[0]) + ", " + g.mapType(t.TypeParams[1]) + ">"
		}
		return "Result<_, _>"
	case "Option":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Option<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Option<_>"
	default:
		return t.Name
	}
}

func (g *generator) defaultValue(t *checker.Type) string {
	if t == nil {
		return "()"
	}
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
		if t.IsEnum && t.EnumInfo != nil {
			// Use the first unit variant as default
			for _, v := range t.EnumInfo.Variants {
				if len(v.Fields) == 0 {
					return fmt.Sprintf("%s::%s", g.mangledEnumName(t.Name), v.Name)
				}
			}
		}
		if t.IsEntity {
			return fmt.Sprintf("%s { /* default fields */ }", g.mangledEntityName(t.Name))
		}
		return fmt.Sprintf("%s { /* default fields */ }", t.Name)
	}
}

func (g *generator) mangledEntityName(name string) string {
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

func (g *generator) mangledEnumName(name string) string {
	if g.structPrefix != "" {
		return g.structPrefix + name
	}
	return name
}

// --- Function generation ---

func (g *generator) generateFunction(f *ir.Function) {
	if f.IsEntry {
		g.emitLine("fn __intent_main() -> i64 {")
		g.incIndent()
		g.generateStmts(f.Body)
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
		g.emitLine("fn main() {")
		g.incIndent()
		g.emitLine("let __exit_code = __intent_main();")
		g.emitLine("std::process::exit(__exit_code as i32);")
		g.decIndent()
		g.emitLine("}")
	} else {
		// Track array params for reference passing
		arrayRefParams := make(map[string]bool)
		for _, p := range f.Params {
			if p.Type != nil && p.Type.Name == "Array" {
				arrayRefParams[p.Name] = true
			}
		}

		fnName := f.Name
		if g.namePrefix != "" {
			fnName = g.namePrefix + f.Name
		}

		g.emitLinef("fn %s(", fnName)
		for i, p := range f.Params {
			if i > 0 {
				g.emit(", ")
			}
			paramType := g.mapType(p.Type)
			if p.Type != nil && p.Type.Name == "Array" {
				paramType = "&" + paramType
			}
			g.emitf("%s: %s", p.Name, paramType)
		}
		g.emitf(") -> %s {\n", g.mapType(f.ReturnType))
		g.incIndent()

		// Requires
		for _, req := range f.Requires {
			g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
				g.generateExpr(req.Expr, arrayRefParams), escapeRustString(req.RawText))
		}

		// Ensures with labeled block
		needsLabeledBlock := len(f.Ensures) > 0 && f.ReturnType != nil && f.ReturnType.Name != "Void"

		if needsLabeledBlock {
			g.emitLinef("let __result: %s = 'body: {\n", g.mapType(f.ReturnType))
			g.incIndent()
			g.inLabeledBlock = true
			g.generateStmtsWithArrayRef(f.Body, arrayRefParams)
			g.inLabeledBlock = false
			g.decIndent()
			g.emitLine("};")

			g.ensuresContext = true
			for _, ens := range f.Ensures {
				g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr, arrayRefParams), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false
			g.emitLine("__result")
		} else {
			g.generateStmtsWithArrayRef(f.Body, arrayRefParams)

			if len(f.Ensures) > 0 {
				g.ensuresContext = true
				for _, ens := range f.Ensures {
					g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
						g.generateExpr(ens.Expr, arrayRefParams), escapeRustString(ens.RawText))
				}
				g.ensuresContext = false
			}
		}

		g.decIndent()
		g.emitLine("}")
	}
}

// --- Entity generation ---

func (g *generator) generateEntity(e *ir.Entity) {
	mangledName := g.mangledEntityName(e.Name)

	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("struct %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.mapType(f.Type))
	}
	g.decIndent()
	g.emitLine("}")
	g.emitLine("")

	g.emitLinef("impl %s {\n", mangledName)
	g.incIndent()

	if len(e.Invariants) > 0 {
		g.emitLine("fn __check_invariants(&self) {")
		g.incIndent()
		for _, inv := range e.Invariants {
			g.emitLinef("assert!(%s, \"Invariant failed: %s\");\n",
				g.generateExpr(inv.Expr, nil), escapeRustString(inv.RawText))
		}
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
	}

	if e.Constructor != nil {
		g.generateConstructor(e)
		g.emitLine("")
	}

	for _, m := range e.Methods {
		g.generateMethod(e, m)
		g.emitLine("")
	}

	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateConstructor(e *ir.Entity) {
	mangledName := g.mangledEntityName(e.Name)
	ctor := e.Constructor

	g.emitLinef("fn new(")
	for i, p := range ctor.Params {
		if i > 0 {
			g.emit(", ")
		}
		g.emitf("%s: %s", p.Name, g.mapType(p.Type))
	}
	g.emitf(") -> %s {\n", mangledName)
	g.incIndent()

	// Requires
	for _, req := range ctor.Requires {
		g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr, nil), escapeRustString(req.RawText))
	}

	// Initialize with defaults
	g.emitLinef("let mut __self = %s {\n", mangledName)
	g.incIndent()
	for _, f := range e.Fields {
		g.emitLinef("%s: %s,\n", f.Name, g.defaultValue(f.Type))
	}
	g.decIndent()
	g.emitLine("};")

	// Body
	g.inConstructor = true
	g.generateStmts(ctor.Body)

	// Ensures
	g.ensuresContext = true
	for _, ens := range ctor.Ensures {
		g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
			g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
	}
	g.ensuresContext = false
	g.inConstructor = false

	// Invariant check
	if len(e.Invariants) > 0 {
		g.emitLine("__self.__check_invariants();")
	}

	g.emitLine("__self")
	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateMethod(e *ir.Entity, m *ir.Method) {
	g.emitLinef("fn %s(&mut self", m.Name)
	for _, p := range m.Params {
		g.emitf(", %s: %s", p.Name, g.mapType(p.Type))
	}
	if m.ReturnType == nil || m.ReturnType.Name == "Void" {
		g.emit(") {\n")
	} else {
		g.emitf(") -> %s {\n", g.mapType(m.ReturnType))
	}
	g.incIndent()

	// Old captures
	for _, cap := range m.OldCaptures {
		g.emitLinef("let %s = %s;\n", cap.Name, g.generateExpr(cap.Expr, nil))
	}

	// Requires
	for _, req := range m.Requires {
		g.emitLinef("assert!(%s, \"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr, nil), escapeRustString(req.RawText))
	}

	// Labeled block for non-Void methods with ensures/invariants
	hasInvariants := len(e.Invariants) > 0
	needsLabeledBlock := (m.ReturnType != nil && m.ReturnType.Name != "Void") && (len(m.Ensures) > 0 || hasInvariants)

	if needsLabeledBlock {
		g.emitLinef("let __result: %s = 'body: {\n", g.mapType(m.ReturnType))
		g.incIndent()
		g.inLabeledBlock = true
		g.generateStmts(m.Body)
		g.inLabeledBlock = false
		g.decIndent()
		g.emitLine("};")

		g.ensuresContext = true
		for _, ens := range m.Ensures {
			g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
				g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
		}
		g.ensuresContext = false

		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}

		g.emitLine("__result")
	} else {
		g.generateStmts(m.Body)

		if len(m.Ensures) > 0 {
			g.ensuresContext = true
			for _, ens := range m.Ensures {
				g.emitLinef("assert!(%s, \"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr, nil), escapeRustString(ens.RawText))
			}
			g.ensuresContext = false
		}

		if hasInvariants {
			g.emitLine("self.__check_invariants();")
		}
	}

	g.decIndent()
	g.emitLine("}")
}

// --- Enum generation ---

func (g *generator) generateEnumDecl(e *ir.Enum) {
	mangledName := g.mangledEnumName(e.Name)

	g.emitLine("#[derive(Clone, Debug)]")
	g.emitLinef("enum %s {\n", mangledName)
	g.incIndent()
	for _, v := range e.Variants {
		if len(v.Fields) == 0 {
			g.emitLinef("%s,\n", v.Name)
		} else {
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

// --- Intent generation ---

func (g *generator) generateIntent(i *ir.Intent) {
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
		g.emitLinef("// Verified by: %s\n", strings.Join(vb, "."))
	}
	g.emitLine("")

	testName := g.mangleIntentName(i.Description)
	g.emitLine("#[cfg(test)]")
	g.emitLinef("mod %s {\n", testName)
	g.incIndent()
	g.emitLine("// Intent verification completed at compile time.")
	g.decIndent()
	g.emitLine("}")
}

// --- Statement generation ---

func (g *generator) generateStmts(stmts []ir.Stmt) {
	g.generateStmtsWithArrayRef(stmts, nil)
}

func (g *generator) generateStmtsWithArrayRef(stmts []ir.Stmt, arrayRefParams map[string]bool) {
	for _, stmt := range stmts {
		g.generateStmt(stmt, arrayRefParams)
	}
}

func (g *generator) generateStmt(s ir.Stmt, arrayRefParams map[string]bool) {
	switch stmt := s.(type) {
	case *ir.LetStmt:
		isMut := stmt.Mutable || g.isEntityType(stmt.Type)
		valueExpr := g.generateExpr(stmt.Value, arrayRefParams)

		// Clone array ref params when binding
		if stmt.Type != nil && stmt.Type.Name == "Array" {
			if vr, ok := stmt.Value.(*ir.VarRef); ok {
				if arrayRefParams != nil && arrayRefParams[vr.Name] {
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

	case *ir.AssignStmt:
		g.emitLinef("%s = %s;\n",
			g.generateExpr(stmt.Target, arrayRefParams),
			g.generateExpr(stmt.Value, arrayRefParams))

	case *ir.ReturnStmt:
		if g.inLabeledBlock {
			if stmt.Value != nil {
				g.emitLinef("break 'body %s;\n", g.generateExpr(stmt.Value, arrayRefParams))
			} else {
				g.emitLine("break 'body;")
			}
		} else {
			if stmt.Value != nil {
				g.emitLinef("return %s;\n", g.generateExpr(stmt.Value, arrayRefParams))
			} else {
				g.emitLine("return;")
			}
		}

	case *ir.WhileStmt:
		g.generateWhileStmt(stmt, arrayRefParams)

	case *ir.ForInStmt:
		g.generateForInStmt(stmt, arrayRefParams)

	case *ir.BreakStmt:
		g.emitLine("break;")

	case *ir.ContinueStmt:
		g.emitLine("continue;")

	case *ir.IfStmt:
		g.generateIfStmt(stmt, arrayRefParams)

	case *ir.ExprStmt:
		g.emitLinef("%s;\n", g.generateExpr(stmt.Expr, arrayRefParams))
	}
}

func (g *generator) generateIfStmt(stmt *ir.IfStmt, arrayRefParams map[string]bool) {
	g.emitLinef("if %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
	g.incIndent()
	g.generateStmtsWithArrayRef(stmt.Then, arrayRefParams)
	g.decIndent()
	g.emitLinef("}")

	if stmt.Else != nil {
		if len(stmt.Else) == 1 {
			if elseIf, ok := stmt.Else[0].(*ir.IfStmt); ok {
				g.emit(" else ")
				g.emitf("if %s {\n", g.generateExpr(elseIf.Condition, arrayRefParams))
				g.incIndent()
				g.generateStmtsWithArrayRef(elseIf.Then, arrayRefParams)
				g.decIndent()
				g.emitLinef("}")
				if elseIf.Else != nil {
					g.emit(" else {\n")
					g.incIndent()
					g.generateStmtsWithArrayRef(elseIf.Else, arrayRefParams)
					g.decIndent()
					g.emitLinef("}\n")
				} else {
					g.emit("\n")
				}
				return
			}
		}
		g.emit(" else {\n")
		g.incIndent()
		g.generateStmtsWithArrayRef(stmt.Else, arrayRefParams)
		g.decIndent()
		g.emitLine("}")
	} else {
		g.emit("\n")
	}
}

func (g *generator) generateWhileStmt(stmt *ir.WhileStmt, arrayRefParams map[string]bool) {
	hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil

	if hasContracts {
		g.emitLine("{")
		g.incIndent()

		// Old captures from invariants
		for _, cap := range stmt.OldCaptures {
			g.emitLinef("let %s = %s;\n", cap.Name, g.generateExpr(cap.Expr, arrayRefParams))
		}

		// Check invariants at entry
		savedEnsures := g.ensuresContext
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("assert!(%s, \"Loop invariant failed at entry: %s\");\n",
				g.generateExpr(inv.Expr, arrayRefParams), escapeRustString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Decreases initialization
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr, arrayRefParams)
			g.emitLinef("let mut __decreases_prev: i64 = %s;\n", metricExpr)
			g.emitLinef("assert!(__decreases_prev >= 0, \"Decreases metric must be non-negative at entry: %s\");\n",
				escapeRustString(stmt.Decreases.RawText))
		}

		g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
		g.incIndent()

		g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)

		// Check invariants after iteration
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("assert!(%s, \"Loop invariant failed after iteration: %s\");\n",
				g.generateExpr(inv.Expr, arrayRefParams), escapeRustString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Check decreases
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr, arrayRefParams)
			g.emitLinef("let __decreases_next: i64 = %s;\n", metricExpr)
			g.emitLinef("assert!(__decreases_next < __decreases_prev, \"Termination metric did not decrease: %s\");\n",
				escapeRustString(stmt.Decreases.RawText))
			g.emitLinef("assert!(__decreases_next >= 0, \"Termination metric became negative: %s\");\n",
				escapeRustString(stmt.Decreases.RawText))
			g.emitLine("__decreases_prev = __decreases_next;")
		}

		g.decIndent()
		g.emitLine("}")

		g.decIndent()
		g.emitLine("}")
	} else {
		g.emitLinef("while %s {\n", g.generateExpr(stmt.Condition, arrayRefParams))
		g.incIndent()
		g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)
		g.decIndent()
		g.emitLine("}")
	}
}

func (g *generator) generateForInStmt(stmt *ir.ForInStmt, arrayRefParams map[string]bool) {
	g.emit(g.indentStr())
	g.emitf("for %s in ", stmt.Variable)

	if rangeExpr, ok := stmt.Iterable.(*ir.RangeExpr); ok {
		g.emitf("(%s..%s)",
			g.generateExpr(rangeExpr.Start, arrayRefParams),
			g.generateExpr(rangeExpr.End, arrayRefParams))
	} else {
		g.emitf("%s.iter()", g.generateExpr(stmt.Iterable, arrayRefParams))
	}

	g.emit(" {\n")
	g.incIndent()
	g.generateStmtsWithArrayRef(stmt.Body, arrayRefParams)
	g.decIndent()
	g.emitLine("}")
}

// --- Expression generation ---

func (g *generator) generateExpr(e ir.Expr, arrayRefParams map[string]bool) string {
	if e == nil {
		return "<nil>"
	}
	switch expr := e.(type) {
	case *ir.BinaryExpr:
		left := g.generateExpr(expr.Left, arrayRefParams)
		right := g.generateExpr(expr.Right, arrayRefParams)
		op := g.mapOperator(expr.Op)

		// String concatenation detection by AST node type (fallback - should be StringConcat in IR)
		if expr.Op == lexer.PLUS {
			if _, ok := expr.Left.(*ir.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
			if _, ok := expr.Right.(*ir.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
		}

		if expr.Op == lexer.IMPLIES {
			return fmt.Sprintf("(!%s || %s)", left, right)
		}

		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *ir.StringConcat:
		left := g.generateExpr(expr.Left, arrayRefParams)
		right := g.generateExpr(expr.Right, arrayRefParams)
		return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)

	case *ir.UnaryExpr:
		operand := g.generateExpr(expr.Operand, arrayRefParams)
		if expr.Op == lexer.NOT {
			return fmt.Sprintf("!%s", operand)
		}
		return fmt.Sprintf("-%s", operand)

	case *ir.CallExpr:
		return g.generateCallExpr(expr, arrayRefParams)

	case *ir.MethodCallExpr:
		return g.generateMethodCallExpr(expr, arrayRefParams)

	case *ir.FieldAccessExpr:
		obj := g.generateExpr(expr.Object, arrayRefParams)
		return fmt.Sprintf("%s.%s", obj, expr.Field)

	case *ir.OldRef:
		return expr.Name

	case *ir.VarRef:
		return expr.Name

	case *ir.SelfRef:
		if g.inConstructor {
			return "__self"
		}
		return "self"

	case *ir.ResultRef:
		return "__result"

	case *ir.IntLit:
		return fmt.Sprintf("%di64", expr.Value)

	case *ir.FloatLit:
		return expr.Value

	case *ir.StringLit:
		return expr.Value + ".to_string()"

	case *ir.StringInterp:
		return g.generateStringInterp(expr)

	case *ir.BoolLit:
		if expr.Value {
			return "true"
		}
		return "false"

	case *ir.ArrayLit:
		if len(expr.Elements) == 0 {
			return "Vec::new()"
		}
		elems := make([]string, len(expr.Elements))
		for i, el := range expr.Elements {
			elems[i] = g.generateExpr(el, arrayRefParams)
		}
		return fmt.Sprintf("vec![%s]", strings.Join(elems, ", "))

	case *ir.IndexExpr:
		return fmt.Sprintf("%s[%s as usize]",
			g.generateExpr(expr.Object, arrayRefParams),
			g.generateExpr(expr.Index, arrayRefParams))

	case *ir.RangeExpr:
		return fmt.Sprintf("(%s..%s)",
			g.generateExpr(expr.Start, arrayRefParams),
			g.generateExpr(expr.End, arrayRefParams))

	case *ir.ForallExpr:
		return g.generateForallExpr(expr, arrayRefParams)

	case *ir.ExistsExpr:
		return g.generateExistsExpr(expr, arrayRefParams)

	case *ir.MatchExpr:
		return g.generateMatchExpr(expr, arrayRefParams)

	case *ir.TryExpr:
		return g.generateExpr(expr.Expr, arrayRefParams) + "?"

	default:
		return "<unknown>"
	}
}

func (g *generator) generateCallExpr(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	switch expr.Kind {
	case ir.CallBuiltin:
		return g.generateBuiltinCall(expr, arrayRefParams)

	case ir.CallVariant:
		return g.generateVariantConstructor(expr, arrayRefParams)

	case ir.CallConstructor:
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg, arrayRefParams)
		}
		return fmt.Sprintf("%s::new(%s)", expr.Function, strings.Join(args, ", "))

	default: // CallFunction
		args := make([]string, len(expr.Args))
		funcDef := g.functions[expr.Function]
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg, arrayRefParams)
			// Pass arrays by reference
			if funcDef != nil && i < len(funcDef.Params) {
				if funcDef.Params[i].Type != nil && funcDef.Params[i].Type.Name == "Array" {
					if _, ok := arg.(*ir.VarRef); ok {
						argStr = "&" + argStr
					}
				}
			}
			args[i] = argStr
		}
		return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))
	}
}

func (g *generator) generateBuiltinCall(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	switch expr.Function {
	case "print":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("println!(\"{}\", %s)", arg)
		}
	case "len":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("(%s.len() as i64)", arg)
		}
	case "Ok", "Err", "Some":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0], arrayRefParams)
			return fmt.Sprintf("%s(%s)", expr.Function, arg)
		}
	case "None":
		return "None"
	}
	// Fallback
	args := make([]string, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = g.generateExpr(a, arrayRefParams)
	}
	return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))
}

func (g *generator) generateVariantConstructor(expr *ir.CallExpr, arrayRefParams map[string]bool) string {
	enumName := expr.EnumName

	// Find variant declaration from IR enums
	var variant *ir.EnumVariant
	if e, ok := g.enums[enumName]; ok {
		for _, v := range e.Variants {
			if v.Name == expr.Function {
				variant = v
				break
			}
		}
	}

	// Unit variant
	if variant == nil || len(variant.Fields) == 0 {
		return fmt.Sprintf("%s::%s", enumName, expr.Function)
	}

	// Data variant
	var sb strings.Builder
	sb.WriteString(enumName)
	sb.WriteString("::")
	sb.WriteString(expr.Function)
	sb.WriteString(" { ")
	for i, f := range variant.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(": ")
		sb.WriteString(g.generateExpr(expr.Args[i], arrayRefParams))
	}
	sb.WriteString(" }")
	return sb.String()
}

func (g *generator) generateMethodCallExpr(expr *ir.MethodCallExpr, arrayRefParams map[string]bool) string {
	// Module-qualified calls
	if expr.IsModuleCall && g.moduleManglings != nil {
		args := make([]string, len(expr.Args))
		funcDecl := g.functions[expr.Method]
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg, arrayRefParams)
			if funcDecl != nil && i < len(funcDecl.Params) {
				if funcDecl.Params[i].Type != nil && funcDecl.Params[i].Type.Name == "Array" {
					if _, ok := arg.(*ir.VarRef); ok {
						argStr = "&" + argStr
					}
				}
			}
			args[i] = argStr
		}

		if expr.CallKind == ir.CallConstructor {
			modPrefix := strings.ToUpper(expr.ModuleName[:1]) + expr.ModuleName[1:]
			mangledStructName := modPrefix + expr.Method
			return fmt.Sprintf("%s::new(%s)", mangledStructName, strings.Join(args, ", "))
		}

		mangledFnName := expr.ModuleName + "_" + expr.Method
		return fmt.Sprintf("%s(%s)", mangledFnName, strings.Join(args, ", "))
	}

	obj := g.generateExpr(expr.Object, arrayRefParams)

	// Result/Option predicate methods
	if expr.Method == "is_ok" || expr.Method == "is_err" || expr.Method == "is_some" || expr.Method == "is_none" {
		return fmt.Sprintf("%s.%s()", obj, expr.Method)
	}

	args := make([]string, len(expr.Args))
	for i, arg := range expr.Args {
		args[i] = g.generateExpr(arg, arrayRefParams)
	}
	return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))
}

func (g *generator) generateForallExpr(expr *ir.ForallExpr, arrayRefParams map[string]bool) string {
	rangeStart := g.generateExpr(expr.Domain.Start, arrayRefParams)
	rangeEnd := g.generateExpr(expr.Domain.End, arrayRefParams)
	body := g.generateExpr(expr.Body, arrayRefParams)

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

func (g *generator) generateExistsExpr(expr *ir.ExistsExpr, arrayRefParams map[string]bool) string {
	rangeStart := g.generateExpr(expr.Domain.Start, arrayRefParams)
	rangeEnd := g.generateExpr(expr.Domain.End, arrayRefParams)
	body := g.generateExpr(expr.Body, arrayRefParams)

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

func (g *generator) generateMatchExpr(expr *ir.MatchExpr, arrayRefParams map[string]bool) string {
	var buf strings.Builder
	buf.WriteString("match ")
	buf.WriteString(g.generateExpr(expr.Scrutinee, arrayRefParams))
	buf.WriteString(" {\n")

	g.incIndent()
	for _, arm := range expr.Arms {
		buf.WriteString(g.indentStr())
		buf.WriteString(g.generateMatchPattern(arm.Pattern))
		buf.WriteString(" => ")
		buf.WriteString(g.generateExpr(arm.Body, arrayRefParams))
		buf.WriteString(",\n")
	}
	g.decIndent()

	buf.WriteString(g.indentStr())
	buf.WriteString("}")
	return buf.String()
}

func (g *generator) generateMatchPattern(pattern *ir.MatchPattern) string {
	if pattern.IsWildcard {
		return "_"
	}

	// Builtin patterns (Ok, Err, Some, None) use tuple syntax
	if pattern.IsBuiltin {
		if pattern.VariantName == "None" {
			return "None"
		}
		if len(pattern.Bindings) == 1 {
			return fmt.Sprintf("%s(%s)", pattern.VariantName, pattern.Bindings[0])
		}
		return pattern.VariantName
	}

	enumName := pattern.EnumName

	// Unit variant
	if len(pattern.Bindings) == 0 {
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	// Data variant with field names
	var fields []string
	for i, binding := range pattern.Bindings {
		if i < len(pattern.FieldNames) {
			fields = append(fields, fmt.Sprintf("%s: %s", pattern.FieldNames[i], binding))
		}
	}

	return fmt.Sprintf("%s::%s { %s }", enumName, pattern.VariantName, strings.Join(fields, ", "))
}

// --- Helpers ---

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

func (g *generator) isEntityType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	_, ok := g.entities[t.Name]
	return ok
}

func (g *generator) mangleIntentName(desc string) string {
	desc = strings.ToLower(desc)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	desc = reg.ReplaceAllString(desc, "_")
	desc = strings.Trim(desc, "_")
	if len(desc) > 0 && desc[0] >= '0' && desc[0] <= '9' {
		desc = "_" + desc
	}
	if desc == "" {
		desc = "__intent"
	}
	return "__intent_" + desc
}

func escapeRustString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// generateStringInterp generates Rust format!() for string interpolation.
// "hello {expr} world" -> format!("hello {} world", expr)
func (g *generator) generateStringInterp(interp *ir.StringInterp) string {
	var fmtStr strings.Builder
	var args []string

	for _, part := range interp.Parts {
		if part.IsExpr {
			fmtStr.WriteString("{}")
			args = append(args, g.generateExpr(part.Expr, nil))
		} else {
			// Escape braces in static parts for Rust format!()
			escaped := strings.ReplaceAll(part.Static, "{", "{{")
			escaped = strings.ReplaceAll(escaped, "}", "}}")
			// Escape quotes and backslashes for Rust string literal
			escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			fmtStr.WriteString(escaped)
		}
	}

	if len(args) == 0 {
		return fmt.Sprintf("\"%s\".to_string()", fmtStr.String())
	}

	return fmt.Sprintf("format!(\"%s\", %s)", fmtStr.String(), strings.Join(args, ", "))
}
