package jsbe

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/lexer"
)

// Generate produces JavaScript source code from a single IR Module.
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

	g.emitLine("// Generated JavaScript code from Intent")
	g.emitLine("")

	for _, e := range mod.Enums {
		g.generateEnumDecl(e)
		g.emitLine("")
	}
	for _, e := range mod.Entities {
		g.generateEntity(e)
		g.emitLine("")
	}
	for _, f := range mod.Functions {
		g.generateFunction(f)
		g.emitLine("")
	}

	// Call entry function if present
	if mod.IsEntry {
		for _, f := range mod.Functions {
			if f.IsEntry {
				g.emitLine("// Entry point invocation")
				g.emitLine("const __exitCode = __intent_main();")
				g.emitLine("process.exit(__exitCode);")
			}
		}
	}

	return g.sb.String()
}

// GenerateAll produces JavaScript from a multi-file IR Program.
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
	sb.WriteString("// Generated JavaScript code from Intent (multi-file)\n\n")

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
			g.classPrefix = strings.ToUpper(mod.Name[:1]) + mod.Name[1:]
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

		for _, e := range mod.Enums {
			g.generateEnumDecl(e)
			g.emitLine("")
		}
		for _, e := range mod.Entities {
			g.generateEntity(e)
			g.emitLine("")
		}
		for _, f := range mod.Functions {
			g.generateFunction(f)
			g.emitLine("")
		}

		sb.WriteString(g.sb.String())
	}

	// Call entry function if present
	for _, mod := range prog.Modules {
		if mod.IsEntry {
			for _, f := range mod.Functions {
				if f.IsEntry {
					sb.WriteString("\n// Entry point invocation\n")
					sb.WriteString("const __exitCode = __intent_main();\n")
					sb.WriteString("process.exit(__exitCode);\n")
				}
			}
		}
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
	ensuresContext bool

	// Multi-file fields
	namePrefix      string
	classPrefix     string
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
	return strings.Repeat("  ", g.indent)
}

// --- Type mapping ---

func (g *generator) mapType(t *checker.Type) string {
	if t == nil {
		return "void"
	}
	switch t.Name {
	case "Int":
		return "number"
	case "Float":
		return "number"
	case "String":
		return "string"
	case "Bool":
		return "boolean"
	case "Void":
		return "void"
	case "Array":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Array<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Array<any>"
	case "Result":
		if t.IsGeneric && len(t.TypeParams) == 2 {
			return "Result<" + g.mapType(t.TypeParams[0]) + ", " + g.mapType(t.TypeParams[1]) + ">"
		}
		return "Result<any, any>"
	case "Option":
		if t.IsGeneric && len(t.TypeParams) == 1 {
			return "Option<" + g.mapType(t.TypeParams[0]) + ">"
		}
		return "Option<any>"
	default:
		return t.Name
	}
}

func (g *generator) defaultValue(t *checker.Type) string {
	if t == nil {
		return "undefined"
	}
	switch t.Name {
	case "Int", "Float":
		return "0"
	case "String":
		return "\"\""
	case "Bool":
		return "false"
	case "Array":
		return "[]"
	default:
		return "null"
	}
}

func (g *generator) mangledEntityName(name string) string {
	if g.classPrefix != "" {
		return g.classPrefix + name
	}
	return name
}

func (g *generator) mangledEnumName(name string) string {
	if g.classPrefix != "" {
		return g.classPrefix + name
	}
	return name
}

// --- Function generation ---

func (g *generator) generateFunction(f *ir.Function) {
	if f.IsEntry {
		g.emitLine("/**")
		g.emitLine(" * Entry function")
		g.emitLine(" * @returns {number}")
		g.emitLine(" */")
		g.emitLine("function __intent_main() {")
		g.incIndent()
		g.generateStmts(f.Body)
		g.decIndent()
		g.emitLine("}")
	} else {
		fnName := f.Name
		if g.namePrefix != "" {
			fnName = g.namePrefix + f.Name
		}

		g.emitLine("/**")
		for _, p := range f.Params {
			g.emitLinef(" * @param {%s} %s\n", g.mapType(p.Type), p.Name)
		}
		g.emitLinef(" * @returns {%s}\n", g.mapType(f.ReturnType))
		g.emitLine(" */")
		g.emitLinef("function %s(", fnName)
		for i, p := range f.Params {
			if i > 0 {
				g.emit(", ")
			}
			g.emitf("%s", p.Name)
		}
		g.emit(") {\n")
		g.incIndent()

		// Requires
		for _, req := range f.Requires {
			g.emitLinef("if (!(%s)) throw new Error(\"Precondition failed: %s\");\n",
				g.generateExpr(req.Expr), escapeJSString(req.RawText))
		}

		// Ensures with result capture
		needsResultCapture := len(f.Ensures) > 0 && f.ReturnType != nil && f.ReturnType.Name != "Void"

		if needsResultCapture {
			g.emitLine("let __result;")
			g.emitLine("{")
			g.incIndent()
			g.generateStmts(f.Body)
			g.decIndent()
			g.emitLine("}")

			g.ensuresContext = true
			for _, ens := range f.Ensures {
				g.emitLinef("if (!(%s)) throw new Error(\"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr), escapeJSString(ens.RawText))
			}
			g.ensuresContext = false
			g.emitLine("return __result;")
		} else {
			g.generateStmts(f.Body)

			if len(f.Ensures) > 0 {
				g.ensuresContext = true
				for _, ens := range f.Ensures {
					g.emitLinef("if (!(%s)) throw new Error(\"Postcondition failed: %s\");\n",
						g.generateExpr(ens.Expr), escapeJSString(ens.RawText))
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

	g.emitLine("/**")
	g.emitLinef(" * Entity: %s\n", e.Name)
	g.emitLine(" */")
	g.emitLinef("class %s {\n", mangledName)
	g.incIndent()

	// Constructor
	if e.Constructor != nil {
		g.generateConstructor(e)
		g.emitLine("")
	}

	// Invariant check method
	if len(e.Invariants) > 0 {
		g.emitLine("/**")
		g.emitLine(" * Check invariants")
		g.emitLine(" */")
		g.emitLine("__checkInvariants() {")
		g.incIndent()
		for _, inv := range e.Invariants {
			g.emitLinef("if (!(%s)) throw new Error(\"Invariant failed: %s\");\n",
				g.generateExpr(inv.Expr), escapeJSString(inv.RawText))
		}
		g.decIndent()
		g.emitLine("}")
		g.emitLine("")
	}

	// Methods
	for _, m := range e.Methods {
		g.generateMethod(e, m)
		g.emitLine("")
	}

	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateConstructor(e *ir.Entity) {
	ctor := e.Constructor

	g.emitLine("/**")
	g.emitLine(" * Constructor")
	for _, p := range ctor.Params {
		g.emitLinef(" * @param {%s} %s\n", g.mapType(p.Type), p.Name)
	}
	g.emitLine(" */")
	g.emitLinef("constructor(")
	for i, p := range ctor.Params {
		if i > 0 {
			g.emit(", ")
		}
		g.emitf("%s", p.Name)
	}
	g.emit(") {\n")
	g.incIndent()

	// Requires
	for _, req := range ctor.Requires {
		g.emitLinef("if (!(%s)) throw new Error(\"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr), escapeJSString(req.RawText))
	}

	// Initialize fields with defaults
	for _, f := range e.Fields {
		g.emitLinef("this.%s = %s;\n", f.Name, g.defaultValue(f.Type))
	}

	// Body
	g.inConstructor = true
	g.generateStmts(ctor.Body)

	// Ensures
	g.ensuresContext = true
	for _, ens := range ctor.Ensures {
		g.emitLinef("if (!(%s)) throw new Error(\"Postcondition failed: %s\");\n",
			g.generateExpr(ens.Expr), escapeJSString(ens.RawText))
	}
	g.ensuresContext = false
	g.inConstructor = false

	// Invariant check
	if len(e.Invariants) > 0 {
		g.emitLine("this.__checkInvariants();")
	}

	g.decIndent()
	g.emitLine("}")
}

func (g *generator) generateMethod(e *ir.Entity, m *ir.Method) {
	g.emitLine("/**")
	g.emitLinef(" * Method: %s\n", m.Name)
	for _, p := range m.Params {
		g.emitLinef(" * @param {%s} %s\n", g.mapType(p.Type), p.Name)
	}
	g.emitLinef(" * @returns {%s}\n", g.mapType(m.ReturnType))
	g.emitLine(" */")
	g.emitLinef("%s(", m.Name)
	for i, p := range m.Params {
		if i > 0 {
			g.emit(", ")
		}
		g.emitf("%s", p.Name)
	}
	g.emit(") {\n")
	g.incIndent()

	// Old captures
	for _, cap := range m.OldCaptures {
		g.emitLinef("const %s = %s;\n", cap.Name, g.generateExpr(cap.Expr))
	}

	// Requires
	for _, req := range m.Requires {
		g.emitLinef("if (!(%s)) throw new Error(\"Precondition failed: %s\");\n",
			g.generateExpr(req.Expr), escapeJSString(req.RawText))
	}

	// Body with result capture if needed
	hasInvariants := len(e.Invariants) > 0
	needsResultCapture := (m.ReturnType != nil && m.ReturnType.Name != "Void") && (len(m.Ensures) > 0 || hasInvariants)

	if needsResultCapture {
		g.emitLine("let __result;")
		g.emitLine("{")
		g.incIndent()
		g.generateStmts(m.Body)
		g.decIndent()
		g.emitLine("}")

		g.ensuresContext = true
		for _, ens := range m.Ensures {
			g.emitLinef("if (!(%s)) throw new Error(\"Postcondition failed: %s\");\n",
				g.generateExpr(ens.Expr), escapeJSString(ens.RawText))
		}
		g.ensuresContext = false

		if hasInvariants {
			g.emitLine("this.__checkInvariants();")
		}

		g.emitLine("return __result;")
	} else {
		g.generateStmts(m.Body)

		if len(m.Ensures) > 0 {
			g.ensuresContext = true
			for _, ens := range m.Ensures {
				g.emitLinef("if (!(%s)) throw new Error(\"Postcondition failed: %s\");\n",
					g.generateExpr(ens.Expr), escapeJSString(ens.RawText))
			}
			g.ensuresContext = false
		}

		if hasInvariants {
			g.emitLine("this.__checkInvariants();")
		}
	}

	g.decIndent()
	g.emitLine("}")
}

// --- Enum generation ---

func (g *generator) generateEnumDecl(e *ir.Enum) {
	mangledName := g.mangledEnumName(e.Name)

	g.emitLine("/**")
	g.emitLinef(" * Enum: %s\n", e.Name)
	g.emitLine(" */")
	g.emitLinef("const %s = {\n", mangledName)
	g.incIndent()

	for _, v := range e.Variants {
		if len(v.Fields) == 0 {
			// Unit variant
			g.emitLinef("%s: () => ({ _tag: \"%s\" }),\n", v.Name, v.Name)
		} else {
			// Data variant
			g.emitLinef("%s: (", v.Name)
			for i, f := range v.Fields {
				if i > 0 {
					g.emit(", ")
				}
				g.emitf("%s", f.Name)
			}
			g.emit(") => ({ _tag: \"")
			g.emitf("%s\"", v.Name)
			for _, f := range v.Fields {
				g.emitf(", %s", f.Name)
			}
			g.emit(" }),\n")
		}
	}

	g.decIndent()
	g.emitLine("};")
}

// --- Statement generation ---

func (g *generator) generateStmts(stmts []ir.Stmt) {
	for _, stmt := range stmts {
		g.generateStmt(stmt)
	}
}

func (g *generator) generateStmt(s ir.Stmt) {
	switch stmt := s.(type) {
	case *ir.LetStmt:
		g.emitLinef("let %s = %s;\n",
			stmt.Name, g.generateExpr(stmt.Value))

	case *ir.AssignStmt:
		g.emitLinef("%s = %s;\n",
			g.generateExpr(stmt.Target),
			g.generateExpr(stmt.Value))

	case *ir.ReturnStmt:
		if g.ensuresContext {
			// Inside ensures context, assign to __result
			if stmt.Value != nil {
				g.emitLinef("__result = %s;\n", g.generateExpr(stmt.Value))
			}
		} else {
			if stmt.Value != nil {
				g.emitLinef("return %s;\n", g.generateExpr(stmt.Value))
			} else {
				g.emitLine("return;")
			}
		}

	case *ir.WhileStmt:
		g.generateWhileStmt(stmt)

	case *ir.ForInStmt:
		g.generateForInStmt(stmt)

	case *ir.BreakStmt:
		g.emitLine("break;")

	case *ir.ContinueStmt:
		g.emitLine("continue;")

	case *ir.IfStmt:
		g.generateIfStmt(stmt)

	case *ir.ExprStmt:
		g.emitLinef("%s;\n", g.generateExpr(stmt.Expr))
	}
}

func (g *generator) generateIfStmt(stmt *ir.IfStmt) {
	g.emitLinef("if (%s) {\n", g.generateExpr(stmt.Condition))
	g.incIndent()
	g.generateStmts(stmt.Then)
	g.decIndent()
	g.emitLinef("}")

	if stmt.Else != nil {
		if len(stmt.Else) == 1 {
			if elseIf, ok := stmt.Else[0].(*ir.IfStmt); ok {
				g.emit(" else if (")
				g.emitf("%s) {\n", g.generateExpr(elseIf.Condition))
				g.incIndent()
				g.generateStmts(elseIf.Then)
				g.decIndent()
				g.emitLinef("}")
				if elseIf.Else != nil {
					g.emit(" else {\n")
					g.incIndent()
					g.generateStmts(elseIf.Else)
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
		g.generateStmts(stmt.Else)
		g.decIndent()
		g.emitLine("}")
	} else {
		g.emit("\n")
	}
}

func (g *generator) generateWhileStmt(stmt *ir.WhileStmt) {
	hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil

	if hasContracts {
		g.emitLine("{")
		g.incIndent()

		// Old captures from invariants
		for _, cap := range stmt.OldCaptures {
			g.emitLinef("let %s = %s;\n", cap.Name, g.generateExpr(cap.Expr))
		}

		// Check invariants at entry
		savedEnsures := g.ensuresContext
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("if (!(%s)) throw new Error(\"Loop invariant failed at entry: %s\");\n",
				g.generateExpr(inv.Expr), escapeJSString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Decreases initialization
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr)
			g.emitLinef("let __decreasesPrev = %s;\n", metricExpr)
			g.emitLinef("if (__decreasesPrev < 0) throw new Error(\"Decreases metric must be non-negative at entry: %s\");\n",
				escapeJSString(stmt.Decreases.RawText))
		}

		g.emitLinef("while (%s) {\n", g.generateExpr(stmt.Condition))
		g.incIndent()

		g.generateStmts(stmt.Body)

		// Check invariants after iteration
		g.ensuresContext = true
		for _, inv := range stmt.Invariants {
			g.emitLinef("if (!(%s)) throw new Error(\"Loop invariant failed after iteration: %s\");\n",
				g.generateExpr(inv.Expr), escapeJSString(inv.RawText))
		}
		g.ensuresContext = savedEnsures

		// Check decreases
		if stmt.Decreases != nil {
			metricExpr := g.generateExpr(stmt.Decreases.Expr)
			g.emitLinef("const __decreasesNext = %s;\n", metricExpr)
			g.emitLinef("if (__decreasesNext >= __decreasesPrev) throw new Error(\"Termination metric did not decrease: %s\");\n",
				escapeJSString(stmt.Decreases.RawText))
			g.emitLinef("if (__decreasesNext < 0) throw new Error(\"Termination metric became negative: %s\");\n",
				escapeJSString(stmt.Decreases.RawText))
			g.emitLine("__decreasesPrev = __decreasesNext;")
		}

		g.decIndent()
		g.emitLine("}")

		g.decIndent()
		g.emitLine("}")
	} else {
		g.emitLinef("while (%s) {\n", g.generateExpr(stmt.Condition))
		g.incIndent()
		g.generateStmts(stmt.Body)
		g.decIndent()
		g.emitLine("}")
	}
}

func (g *generator) generateForInStmt(stmt *ir.ForInStmt) {
	g.emit(g.indentStr())
	g.emitf("for (const %s of ", stmt.Variable)

	if rangeExpr, ok := stmt.Iterable.(*ir.RangeExpr); ok {
		// Generate range helper
		g.emitf("Array.from({ length: (%s) - (%s) }, (_, i) => (%s) + i)",
			g.generateExpr(rangeExpr.End),
			g.generateExpr(rangeExpr.Start),
			g.generateExpr(rangeExpr.Start))
	} else {
		g.emitf("%s", g.generateExpr(stmt.Iterable))
	}

	g.emit(") {\n")
	g.incIndent()
	g.generateStmts(stmt.Body)
	g.decIndent()
	g.emitLine("}")
}

// --- Expression generation ---

func (g *generator) generateExpr(e ir.Expr) string {
	if e == nil {
		return "null"
	}
	switch expr := e.(type) {
	case *ir.BinaryExpr:
		left := g.generateExpr(expr.Left)
		right := g.generateExpr(expr.Right)
		op := g.mapOperator(expr.Op)

		if expr.Op == lexer.IMPLIES {
			return fmt.Sprintf("(!%s || %s)", left, right)
		}

		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *ir.StringConcat:
		left := g.generateExpr(expr.Left)
		right := g.generateExpr(expr.Right)
		return fmt.Sprintf("(%s + %s)", left, right)

	case *ir.UnaryExpr:
		operand := g.generateExpr(expr.Operand)
		if expr.Op == lexer.NOT {
			return fmt.Sprintf("!%s", operand)
		}
		return fmt.Sprintf("-%s", operand)

	case *ir.CallExpr:
		return g.generateCallExpr(expr)

	case *ir.MethodCallExpr:
		return g.generateMethodCallExpr(expr)

	case *ir.FieldAccessExpr:
		obj := g.generateExpr(expr.Object)
		return fmt.Sprintf("%s.%s", obj, expr.Field)

	case *ir.OldRef:
		return expr.Name

	case *ir.VarRef:
		return expr.Name

	case *ir.SelfRef:
		return "this"

	case *ir.ResultRef:
		return "__result"

	case *ir.IntLit:
		return fmt.Sprintf("%d", expr.Value)

	case *ir.FloatLit:
		return expr.Value

	case *ir.StringLit:
		return expr.Value

	case *ir.StringInterp:
		return g.generateStringInterp(expr)

	case *ir.BoolLit:
		if expr.Value {
			return "true"
		}
		return "false"

	case *ir.ArrayLit:
		if len(expr.Elements) == 0 {
			return "[]"
		}
		elems := make([]string, len(expr.Elements))
		for i, el := range expr.Elements {
			elems[i] = g.generateExpr(el)
		}
		return fmt.Sprintf("[%s]", strings.Join(elems, ", "))

	case *ir.IndexExpr:
		return fmt.Sprintf("%s[%s]",
			g.generateExpr(expr.Object),
			g.generateExpr(expr.Index))

	case *ir.RangeExpr:
		// Range expressions are typically handled by for-in, but if needed standalone:
		return fmt.Sprintf("{ start: %s, end: %s }",
			g.generateExpr(expr.Start),
			g.generateExpr(expr.End))

	case *ir.ForallExpr:
		return g.generateForallExpr(expr)

	case *ir.ExistsExpr:
		return g.generateExistsExpr(expr)

	case *ir.MatchExpr:
		return g.generateMatchExpr(expr)

	case *ir.TryExpr:
		// For JavaScript, try expressions can be a simple wrapper
		return g.generateExpr(expr.Expr)

	default:
		return "undefined"
	}
}

func (g *generator) generateCallExpr(expr *ir.CallExpr) string {
	switch expr.Kind {
	case ir.CallBuiltin:
		return g.generateBuiltinCall(expr)

	case ir.CallVariant:
		return g.generateVariantConstructor(expr)

	case ir.CallConstructor:
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg)
		}
		return fmt.Sprintf("new %s(%s)", expr.Function, strings.Join(args, ", "))

	default: // CallFunction
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg)
		}
		return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))
	}
}

func (g *generator) generateBuiltinCall(expr *ir.CallExpr) string {
	switch expr.Function {
	case "print":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("console.log(%s)", arg)
		}
	case "len":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("(%s.length)", arg)
		}
	case "Ok", "Err", "Some":
		if len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("{ _tag: \"%s\", value: %s }", expr.Function, arg)
		}
	case "None":
		return "{ _tag: \"None\" }"
	}
	// Fallback
	args := make([]string, len(expr.Args))
	for i, a := range expr.Args {
		args[i] = g.generateExpr(a)
	}
	return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))
}

func (g *generator) generateVariantConstructor(expr *ir.CallExpr) string {
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
		return fmt.Sprintf("%s.%s()", enumName, expr.Function)
	}

	// Data variant
	args := make([]string, len(expr.Args))
	for i, arg := range expr.Args {
		args[i] = g.generateExpr(arg)
	}
	return fmt.Sprintf("%s.%s(%s)", enumName, expr.Function, strings.Join(args, ", "))
}

func (g *generator) generateMethodCallExpr(expr *ir.MethodCallExpr) string {
	// Module-qualified calls
	if expr.IsModuleCall && g.moduleManglings != nil {
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg)
		}

		if expr.CallKind == ir.CallConstructor {
			modPrefix := strings.ToUpper(expr.ModuleName[:1]) + expr.ModuleName[1:]
			mangledClassName := modPrefix + expr.Method
			return fmt.Sprintf("new %s(%s)", mangledClassName, strings.Join(args, ", "))
		}

		mangledFnName := expr.ModuleName + "_" + expr.Method
		return fmt.Sprintf("%s(%s)", mangledFnName, strings.Join(args, ", "))
	}

	obj := g.generateExpr(expr.Object)

	// Result/Option predicate methods
	if expr.Method == "is_ok" {
		return fmt.Sprintf("(%s._tag === \"Ok\")", obj)
	}
	if expr.Method == "is_err" {
		return fmt.Sprintf("(%s._tag === \"Err\")", obj)
	}
	if expr.Method == "is_some" {
		return fmt.Sprintf("(%s._tag === \"Some\")", obj)
	}
	if expr.Method == "is_none" {
		return fmt.Sprintf("(%s._tag === \"None\")", obj)
	}

	args := make([]string, len(expr.Args))
	for i, arg := range expr.Args {
		args[i] = g.generateExpr(arg)
	}
	return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))
}

func (g *generator) generateForallExpr(expr *ir.ForallExpr) string {
	rangeStart := g.generateExpr(expr.Domain.Start)
	rangeEnd := g.generateExpr(expr.Domain.End)
	body := g.generateExpr(expr.Body)

	return fmt.Sprintf("(() => {\n"+
		"  let __forallHolds = true;\n"+
		"  for (let %s = %s; %s < %s; %s++) {\n"+
		"    if (!(%s)) {\n"+
		"      __forallHolds = false;\n"+
		"      break;\n"+
		"    }\n"+
		"  }\n"+
		"  return __forallHolds;\n"+
		"})()", expr.Variable, rangeStart, expr.Variable, rangeEnd, expr.Variable, body)
}

func (g *generator) generateExistsExpr(expr *ir.ExistsExpr) string {
	rangeStart := g.generateExpr(expr.Domain.Start)
	rangeEnd := g.generateExpr(expr.Domain.End)
	body := g.generateExpr(expr.Body)

	return fmt.Sprintf("(() => {\n"+
		"  let __existsFound = false;\n"+
		"  for (let %s = %s; %s < %s; %s++) {\n"+
		"    if (%s) {\n"+
		"      __existsFound = true;\n"+
		"      break;\n"+
		"    }\n"+
		"  }\n"+
		"  return __existsFound;\n"+
		"})()", expr.Variable, rangeStart, expr.Variable, rangeEnd, expr.Variable, body)
}

func (g *generator) generateMatchExpr(expr *ir.MatchExpr) string {
	scrutinee := g.generateExpr(expr.Scrutinee)

	// Generate if/else chain
	var sb strings.Builder
	sb.WriteString("(() => {\n")
	sb.WriteString("  const __scrutinee = ")
	sb.WriteString(scrutinee)
	sb.WriteString(";\n")

	for i, arm := range expr.Arms {
		if arm.Pattern.IsWildcard {
			sb.WriteString("  return ")
			sb.WriteString(g.generateExpr(arm.Body))
			sb.WriteString(";\n")
		} else {
			if i > 0 {
				sb.WriteString("  else ")
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString("if (__scrutinee._tag === \"")
			sb.WriteString(arm.Pattern.VariantName)
			sb.WriteString("\") {\n")

			// Destructure bindings
			for j, binding := range arm.Pattern.Bindings {
				if j < len(arm.Pattern.FieldNames) {
					sb.WriteString("    const ")
					sb.WriteString(binding)
					sb.WriteString(" = __scrutinee.")
					sb.WriteString(arm.Pattern.FieldNames[j])
					sb.WriteString(";\n")
				} else if arm.Pattern.IsBuiltin {
					// Builtin patterns use 'value' field
					sb.WriteString("    const ")
					sb.WriteString(binding)
					sb.WriteString(" = __scrutinee.value;\n")
				}
			}

			sb.WriteString("    return ")
			sb.WriteString(g.generateExpr(arm.Body))
			sb.WriteString(";\n")
			sb.WriteString("  }\n")
		}
	}

	sb.WriteString("})()")
	return sb.String()
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
		return "==="
	case lexer.NEQ:
		return "!=="
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

func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// generateStringInterp generates JS template literal for string interpolation.
// "hello {expr} world" -> `hello ${expr} world`
func (g *generator) generateStringInterp(interp *ir.StringInterp) string {
	var sb strings.Builder
	sb.WriteByte('`')

	for _, part := range interp.Parts {
		if part.IsExpr {
			sb.WriteString("${")
			sb.WriteString(g.generateExpr(part.Expr))
			sb.WriteByte('}')
		} else {
			// Escape backticks and ${} in static parts for JS template literal
			escaped := strings.ReplaceAll(part.Static, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "`", "\\`")
			escaped = strings.ReplaceAll(escaped, "${", "\\${")
			sb.WriteString(escaped)
		}
	}

	sb.WriteByte('`')
	return sb.String()
}
