package formatter

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/lexer"
)

// Format takes an AST Program and returns canonical Intent source code.
func Format(prog *ast.Program) string {
	f := &formatter{}
	f.formatProgram(prog)
	return f.sb.String()
}

type formatter struct {
	sb     strings.Builder
	indent int
}

// --- helpers (same pattern as codegen.go) ---

func (f *formatter) emit(s string) {
	f.sb.WriteString(s)
}

func (f *formatter) emitf(format string, args ...any) {
	f.sb.WriteString(fmt.Sprintf(format, args...))
}

func (f *formatter) emitLine(s string) {
	if s == "" {
		f.sb.WriteString("\n")
	} else {
		f.sb.WriteString(f.indentStr())
		f.sb.WriteString(s)
		f.sb.WriteString("\n")
	}
}

func (f *formatter) emitLinef(format string, args ...any) {
	f.sb.WriteString(f.indentStr())
	f.sb.WriteString(fmt.Sprintf(format, args...))
	f.sb.WriteString("\n")
}

func (f *formatter) incIndent() { f.indent++ }
func (f *formatter) decIndent() { f.indent-- }

func (f *formatter) indentStr() string {
	return strings.Repeat("    ", f.indent)
}

func (f *formatter) blankLine() {
	f.sb.WriteString("\n")
}

// --- program-level ---

func (f *formatter) formatProgram(prog *ast.Program) {
	if prog.Module != nil {
		f.formatModuleDecl(prog.Module)
	}

	if len(prog.Imports) > 0 {
		f.blankLine()
		for _, imp := range prog.Imports {
			f.formatImportDecl(imp)
		}
	}

	// Emit declarations in canonical order: enums, entities, functions, intents
	for _, e := range prog.Enums {
		f.blankLine()
		f.formatEnumDecl(e)
	}
	for _, e := range prog.Entities {
		f.blankLine()
		f.formatEntityDecl(e)
	}
	for _, fn := range prog.Functions {
		f.blankLine()
		f.formatFunctionDecl(fn)
	}
	for _, i := range prog.Intents {
		f.blankLine()
		f.formatIntentDecl(i)
	}

	// Trailing newline
	f.blankLine()
}

func (f *formatter) formatModuleDecl(m *ast.ModuleDecl) {
	f.emitLinef("module %s version \"%s\";", m.Name, m.Version)
}

func (f *formatter) formatImportDecl(imp *ast.ImportDecl) {
	f.emitLinef("import \"%s\";", imp.Path)
}

// --- declarations ---

func (f *formatter) formatEnumDecl(e *ast.EnumDecl) {
	if e.IsPublic {
		f.emit(f.indentStr() + "public ")
	} else {
		f.emit(f.indentStr())
	}
	f.emitf("enum %s {\n", e.Name)
	f.incIndent()
	for _, v := range e.Variants {
		if len(v.Fields) == 0 {
			f.emitLinef("%s,", v.Name)
		} else {
			f.emit(f.indentStr())
			f.emitf("%s(", v.Name)
			for i, field := range v.Fields {
				if i > 0 {
					f.emit(", ")
				}
				f.emitf("%s: %s", field.Name, f.formatTypeRef(field.Type))
			}
			f.emit("),\n")
		}
	}
	f.decIndent()
	f.emitLine("}")
}

func (f *formatter) formatEntityDecl(e *ast.EntityDecl) {
	if e.IsPublic {
		f.emit(f.indentStr() + "public ")
	} else {
		f.emit(f.indentStr())
	}
	f.emitf("entity %s {\n", e.Name)
	f.incIndent()

	// Fields
	for _, field := range e.Fields {
		f.emitLinef("field %s: %s;", field.Name, f.formatTypeRef(field.Type))
	}

	// Invariants (blank line before if there were fields)
	if len(e.Invariants) > 0 && len(e.Fields) > 0 {
		f.blankLine()
	}
	for _, inv := range e.Invariants {
		f.emitLinef("invariant %s;", f.formatExpr(inv.Expr))
	}

	// Constructor (blank line before)
	if e.Constructor != nil {
		f.blankLine()
		f.formatConstructorDecl(e.Constructor)
	}

	// Methods (blank line before each)
	for _, m := range e.Methods {
		f.blankLine()
		f.formatMethodDecl(m)
	}

	f.decIndent()
	f.emitLine("}")
}

func (f *formatter) formatConstructorDecl(c *ast.ConstructorDecl) {
	f.emit(f.indentStr())
	f.emit("constructor(")
	for i, p := range c.Params {
		if i > 0 {
			f.emit(", ")
		}
		f.emitf("%s: %s", p.Name, f.formatTypeRef(p.Type))
	}
	f.emit(")")

	hasContracts := len(c.Requires) > 0 || len(c.Ensures) > 0
	if hasContracts {
		f.emit("\n")
		f.incIndent()
		for _, req := range c.Requires {
			f.emitLinef("requires %s", f.formatExpr(req.Expr))
		}
		for _, ens := range c.Ensures {
			f.emitLinef("ensures %s", f.formatExpr(ens.Expr))
		}
		f.decIndent()
		f.emitLine("{")
	} else {
		f.emit(" {\n")
	}
	f.incIndent()
	f.formatBlock(c.Body)
	f.decIndent()
	f.emitLine("}")
}

func (f *formatter) formatMethodDecl(m *ast.MethodDecl) {
	f.emit(f.indentStr())
	f.emitf("method %s(", m.Name)
	for i, p := range m.Params {
		if i > 0 {
			f.emit(", ")
		}
		f.emitf("%s: %s", p.Name, f.formatTypeRef(p.Type))
	}
	f.emitf(") returns %s", f.formatTypeRef(m.ReturnType))

	hasContracts := len(m.Requires) > 0 || len(m.Ensures) > 0
	if hasContracts {
		f.emit("\n")
		f.incIndent()
		for _, req := range m.Requires {
			f.emitLinef("requires %s", f.formatExpr(req.Expr))
		}
		for _, ens := range m.Ensures {
			f.emitLinef("ensures %s", f.formatExpr(ens.Expr))
		}
		f.decIndent()
		f.emitLine("{")
	} else {
		f.emit(" {\n")
	}
	f.incIndent()
	f.formatBlock(m.Body)
	f.decIndent()
	f.emitLine("}")
}

func (f *formatter) formatFunctionDecl(fn *ast.FunctionDecl) {
	f.emit(f.indentStr())
	if fn.IsPublic {
		f.emit("public ")
	}
	if fn.IsEntry {
		f.emit("entry ")
	}
	f.emitf("function %s(", fn.Name)
	for i, p := range fn.Params {
		if i > 0 {
			f.emit(", ")
		}
		f.emitf("%s: %s", p.Name, f.formatTypeRef(p.Type))
	}
	f.emitf(") returns %s", f.formatTypeRef(fn.ReturnType))

	hasContracts := len(fn.Requires) > 0 || len(fn.Ensures) > 0
	if hasContracts {
		f.emit("\n")
		f.incIndent()
		for _, req := range fn.Requires {
			f.emitLinef("requires %s", f.formatExpr(req.Expr))
		}
		for _, ens := range fn.Ensures {
			f.emitLinef("ensures %s", f.formatExpr(ens.Expr))
		}
		f.decIndent()
	}

	if fn.Body != nil {
		if hasContracts {
			f.emitLine("{")
		} else {
			f.emit(" {\n")
		}
		f.incIndent()
		f.formatBlock(fn.Body)
		f.decIndent()
		f.emitLine("}")
	}
}

func (f *formatter) formatIntentDecl(i *ast.IntentDecl) {
	f.emitLinef("intent \"%s\" {", i.Description)
	f.incIndent()
	for _, g := range i.Goals {
		f.emitLinef("goal: \"%s\";", g)
	}
	for _, c := range i.Constraints {
		f.emitLinef("constraint: \"%s\";", c)
	}
	for _, g := range i.Guarantees {
		f.emitLinef("guarantee: \"%s\";", g)
	}
	if len(i.VerifiedBy) > 0 {
		refs := make([]string, len(i.VerifiedBy))
		for idx, vb := range i.VerifiedBy {
			refs[idx] = strings.Join(vb.Parts, ".")
		}
		f.emitLinef("verified_by: [%s];", strings.Join(refs, ", "))
	}
	f.decIndent()
	f.emitLine("}")
}

// --- statements ---

func (f *formatter) formatBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for _, stmt := range b.Statements {
		f.formatStmt(stmt)
	}
}

func (f *formatter) formatStmt(s ast.Statement) {
	switch stmt := s.(type) {
	case *ast.LetStmt:
		f.emit(f.indentStr())
		f.emit("let ")
		if stmt.Mutable {
			f.emit("mutable ")
		}
		f.emitf("%s: %s = %s;\n", stmt.Name, f.formatTypeRef(stmt.Type), f.formatExpr(stmt.Value))

	case *ast.AssignStmt:
		f.emitLinef("%s = %s;", f.formatExpr(stmt.Target), f.formatExpr(stmt.Value))

	case *ast.ReturnStmt:
		if stmt.Value != nil {
			f.emitLinef("return %s;", f.formatExpr(stmt.Value))
		} else {
			f.emitLine("return;")
		}

	case *ast.IfStmt:
		f.formatIfStmt(stmt, false)

	case *ast.WhileStmt:
		f.formatWhileStmt(stmt)

	case *ast.ForInStmt:
		f.emitLinef("for %s in %s {", stmt.Variable, f.formatExpr(stmt.Iterable))
		f.incIndent()
		f.formatBlock(stmt.Body)
		f.decIndent()
		f.emitLine("}")

	case *ast.BreakStmt:
		f.emitLine("break;")

	case *ast.ContinueStmt:
		f.emitLine("continue;")

	case *ast.ExprStmt:
		f.emitLinef("%s;", f.formatExpr(stmt.Expr))

	case *ast.Block:
		f.formatBlock(stmt)
	}
}

func (f *formatter) formatIfStmt(stmt *ast.IfStmt, isElseIf bool) {
	if isElseIf {
		f.emitf(" else if %s {\n", f.formatExpr(stmt.Condition))
	} else {
		f.emitLinef("if %s {", f.formatExpr(stmt.Condition))
	}
	f.incIndent()
	f.formatBlock(stmt.Then)
	f.decIndent()
	if stmt.Else != nil {
		if elseIf, ok := stmt.Else.(*ast.IfStmt); ok {
			f.emit(f.indentStr() + "}")
			f.formatIfStmt(elseIf, true)
		} else if elseBlock, ok := stmt.Else.(*ast.Block); ok {
			f.emitLine("} else {")
			f.incIndent()
			f.formatBlock(elseBlock)
			f.decIndent()
			f.emitLine("}")
		}
	} else {
		f.emitLine("}")
	}
}

func (f *formatter) formatWhileStmt(stmt *ast.WhileStmt) {
	hasContracts := len(stmt.Invariants) > 0 || stmt.Decreases != nil

	if hasContracts {
		f.emitLinef("while %s", f.formatExpr(stmt.Condition))
		f.incIndent()
		for _, inv := range stmt.Invariants {
			f.emitLinef("invariant %s", f.formatExpr(inv.Expr))
		}
		if stmt.Decreases != nil {
			f.emitLinef("decreases %s", f.formatExpr(stmt.Decreases.Expr))
		}
		f.decIndent()
		f.emitLine("{")
	} else {
		f.emitLinef("while %s {", f.formatExpr(stmt.Condition))
	}
	f.incIndent()
	f.formatBlock(stmt.Body)
	f.decIndent()
	f.emitLine("}")
}

// --- expressions ---

func (f *formatter) formatExpr(e ast.Expression) string {
	return f.formatExprPrec(e, 0)
}

// formatExprPrec formats an expression, wrapping in parens if needed based on parent precedence.
func (f *formatter) formatExprPrec(e ast.Expression, parentPrec int) string {
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		prec := precedence(expr.Op)
		left := f.formatExprPrec(expr.Left, prec)
		right := f.formatExprPrec(expr.Right, prec+1) // +1 for left-associativity
		op := operatorString(expr.Op)
		result := fmt.Sprintf("%s %s %s", left, op, right)
		if prec < parentPrec {
			return "(" + result + ")"
		}
		return result

	case *ast.UnaryExpr:
		operand := f.formatExprPrec(expr.Operand, 10) // unary binds tight
		if expr.Op == lexer.NOT {
			return "not " + operand
		}
		return "-" + operand

	case *ast.CallExpr:
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = f.formatExpr(arg)
		}
		return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))

	case *ast.MethodCallExpr:
		obj := f.formatExpr(expr.Object)
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = f.formatExpr(arg)
		}
		return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))

	case *ast.FieldAccessExpr:
		obj := f.formatExpr(expr.Object)
		return fmt.Sprintf("%s.%s", obj, expr.Field)

	case *ast.IndexExpr:
		obj := f.formatExpr(expr.Object)
		idx := f.formatExpr(expr.Index)
		return fmt.Sprintf("%s[%s]", obj, idx)

	case *ast.Identifier:
		return expr.Name

	case *ast.SelfExpr:
		return "self"

	case *ast.ResultExpr:
		return "result"

	case *ast.IntLit:
		return expr.Value

	case *ast.FloatLit:
		return expr.Value

	case *ast.StringLit:
		return expr.Value

	case *ast.BoolLit:
		if expr.Value {
			return "true"
		}
		return "false"

	case *ast.ArrayLit:
		elems := make([]string, len(expr.Elements))
		for i, elem := range expr.Elements {
			elems[i] = f.formatExpr(elem)
		}
		return fmt.Sprintf("[%s]", strings.Join(elems, ", "))

	case *ast.RangeExpr:
		return fmt.Sprintf("%s..%s", f.formatExprPrec(expr.Start, 10), f.formatExprPrec(expr.End, 10))

	case *ast.ForallExpr:
		domain := fmt.Sprintf("%s..%s", f.formatExpr(expr.Domain.Start), f.formatExpr(expr.Domain.End))
		body := f.formatExpr(expr.Body)
		return fmt.Sprintf("forall %s in %s: %s", expr.Variable, domain, body)

	case *ast.ExistsExpr:
		domain := fmt.Sprintf("%s..%s", f.formatExpr(expr.Domain.Start), f.formatExpr(expr.Domain.End))
		body := f.formatExpr(expr.Body)
		return fmt.Sprintf("exists %s in %s: %s", expr.Variable, domain, body)

	case *ast.MatchExpr:
		return f.formatMatchExpr(expr)

	case *ast.TryExpr:
		inner := f.formatExpr(expr.Expr)
		return inner + "?"

	case *ast.OldExpr:
		inner := f.formatExpr(expr.Expr)
		return fmt.Sprintf("old(%s)", inner)

	default:
		return "<unknown>"
	}
}

func (f *formatter) formatMatchExpr(expr *ast.MatchExpr) string {
	var buf strings.Builder
	buf.WriteString("match ")
	buf.WriteString(f.formatExpr(expr.Scrutinee))
	buf.WriteString(" {\n")

	f.incIndent()
	for _, arm := range expr.Arms {
		buf.WriteString(f.indentStr())
		buf.WriteString(f.formatMatchPattern(arm.Pattern))
		buf.WriteString(" => ")
		buf.WriteString(f.formatExpr(arm.Body))
		buf.WriteString(",\n")
	}
	f.decIndent()

	buf.WriteString(f.indentStr())
	buf.WriteString("}")
	return buf.String()
}

func (f *formatter) formatMatchPattern(p *ast.MatchPattern) string {
	if p.IsWildcard {
		return "_"
	}
	if len(p.Bindings) == 0 {
		return p.VariantName
	}
	return fmt.Sprintf("%s(%s)", p.VariantName, strings.Join(p.Bindings, ", "))
}

// --- type references ---

func (f *formatter) formatTypeRef(t *ast.TypeRef) string {
	if t == nil {
		return "Void"
	}
	if len(t.TypeArgs) == 0 {
		return t.Name
	}
	args := make([]string, len(t.TypeArgs))
	for i, arg := range t.TypeArgs {
		args[i] = f.formatTypeRef(arg)
	}
	return fmt.Sprintf("%s<%s>", t.Name, strings.Join(args, ", "))
}

// --- operator precedence ---

// Precedence levels (higher binds tighter):
//
//	1: implies
//	2: or
//	3: and
//	5: == !=
//	6: < > <= >=
//	7: + -
//	8: * / %
func precedence(op lexer.TokenType) int {
	switch op {
	case lexer.IMPLIES:
		return 1
	case lexer.OR:
		return 2
	case lexer.AND:
		return 3
	case lexer.EQ, lexer.NEQ:
		return 5
	case lexer.LT, lexer.GT, lexer.LEQ, lexer.GEQ:
		return 6
	case lexer.PLUS, lexer.MINUS:
		return 7
	case lexer.STAR, lexer.SLASH, lexer.PERCENT:
		return 8
	default:
		return 0
	}
}

func operatorString(op lexer.TokenType) string {
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
		return "and"
	case lexer.OR:
		return "or"
	case lexer.IMPLIES:
		return "implies"
	default:
		return "?"
	}
}
