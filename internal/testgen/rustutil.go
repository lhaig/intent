package testgen

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/lexer"
)

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
	g := &rustHelper{
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

// MapType converts an Intent TypeRef to its Rust type string.
func MapType(t *ast.TypeRef) string {
	g := &rustHelper{}
	return g.mapType(t)
}

// EscapeRustString escapes a string for use in Rust string literals.
func EscapeRustString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// rustHelper holds state for converting AST expressions to Rust strings.
type rustHelper struct {
	entities          map[string]*ast.EntityDecl
	enums             map[string]*ast.EnumDecl
	functions         map[string]*ast.FunctionDecl
	oldExprs          map[string]string
	ensuresContext    bool
	selfVarOverride   string
	resultVarOverride string
}

func (g *rustHelper) generateExpr(e ast.Expression) string {
	switch expr := e.(type) {
	case *ast.BinaryExpr:
		left := g.generateExpr(expr.Left)
		right := g.generateExpr(expr.Right)
		op := g.mapOperator(expr.Op)

		if expr.Op == lexer.PLUS {
			if _, ok := expr.Left.(*ast.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
			if _, ok := expr.Right.(*ast.StringLit); ok {
				return fmt.Sprintf("format!(\"{}{}\", %s, %s)", left, right)
			}
		}

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
		if expr.Function == "print" && len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("println!(\"{}\", %s)", arg)
		}
		if expr.Function == "len" && len(expr.Args) == 1 {
			arg := g.generateExpr(expr.Args[0])
			return fmt.Sprintf("(%s.len() as i64)", arg)
		}
		if expr.Function == "Ok" || expr.Function == "Err" || expr.Function == "Some" {
			if len(expr.Args) == 1 {
				arg := g.generateExpr(expr.Args[0])
				return fmt.Sprintf("%s(%s)", expr.Function, arg)
			}
		}
		if enumDecl, variantDecl := g.lookupVariant(expr.Function); enumDecl != nil {
			return g.generateVariantConstructor(expr, enumDecl, variantDecl)
		}
		args := make([]string, len(expr.Args))
		funcDecl := g.functions[expr.Function]
		for i, arg := range expr.Args {
			argStr := g.generateExpr(arg)
			if funcDecl != nil && i < len(funcDecl.Params) {
				paramType := funcDecl.Params[i].Type
				if paramType.Name == "Array" || g.isEntityType(paramType) {
					if _, ok := arg.(*ast.Identifier); ok {
						argStr = "&" + argStr
					} else if _, ok := arg.(*ast.IndexExpr); ok {
						argStr = "&" + argStr
					}
				} else if paramType.Name == "String" {
					if _, isLit := arg.(*ast.StringLit); !isLit {
						argStr += ".clone()"
					}
				}
			}
			args[i] = argStr
		}
		return fmt.Sprintf("%s(%s)", expr.Function, strings.Join(args, ", "))

	case *ast.MethodCallExpr:
		obj := g.generateExpr(expr.Object)
		if expr.Method == "is_ok" || expr.Method == "is_err" || expr.Method == "is_some" || expr.Method == "is_none" {
			return fmt.Sprintf("%s.%s()", obj, expr.Method)
		}
		args := make([]string, len(expr.Args))
		for i, arg := range expr.Args {
			args[i] = g.generateExpr(arg)
		}
		return fmt.Sprintf("%s.%s(%s)", obj, expr.Method, strings.Join(args, ", "))

	case *ast.FieldAccessExpr:
		obj := g.generateExpr(expr.Object)
		fieldExpr := fmt.Sprintf("%s.%s", obj, expr.Field)
		if g.fieldIsString(expr) {
			fieldExpr += ".clone()"
		}
		return fieldExpr

	case *ast.OldExpr:
		if g.ensuresContext {
			mangledName := g.mangleOldExpr(expr.Expr)
			return mangledName
		}
		return g.generateExpr(expr.Expr)

	case *ast.Identifier:
		if expr.Name == "None" {
			return "None"
		}
		if enumDecl, variantDecl := g.lookupVariant(expr.Name); enumDecl != nil && len(variantDecl.Fields) == 0 {
			return fmt.Sprintf("%s::%s", enumDecl.Name, expr.Name)
		}
		return expr.Name

	case *ast.SelfExpr:
		if g.selfVarOverride != "" {
			return g.selfVarOverride
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
		return expr.Value + ".to_string()"

	case *ast.StringInterp:
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

func (g *rustHelper) mapType(t *ast.TypeRef) string {
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
		return "Vec<_>"
	case "Result":
		if len(t.TypeArgs) == 2 {
			return "Result<" + g.mapType(t.TypeArgs[0]) + ", " + g.mapType(t.TypeArgs[1]) + ">"
		}
		return "Result<_, _>"
	case "Option":
		if len(t.TypeArgs) == 1 {
			return "Option<" + g.mapType(t.TypeArgs[0]) + ">"
		}
		return "Option<_>"
	default:
		return t.Name
	}
}

func (g *rustHelper) mapOperator(op lexer.TokenType) string {
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

func (g *rustHelper) lookupVariant(name string) (*ast.EnumDecl, *ast.EnumVariant) {
	for _, enumDecl := range g.enums {
		for _, variant := range enumDecl.Variants {
			if variant.Name == name {
				return enumDecl, variant
			}
		}
	}
	return nil, nil
}

func (g *rustHelper) generateVariantConstructor(expr *ast.CallExpr, enumDecl *ast.EnumDecl, variantDecl *ast.EnumVariant) string {
	if len(variantDecl.Fields) == 0 {
		return fmt.Sprintf("%s::%s", enumDecl.Name, expr.Function)
	}
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

func (g *rustHelper) isEntityType(t *ast.TypeRef) bool {
	_, ok := g.entities[t.Name]
	return ok
}

func (g *rustHelper) fieldIsString(expr *ast.FieldAccessExpr) bool {
	entityName := g.resolveEntityName(expr.Object)
	if entityName == "" {
		return false
	}
	entity, ok := g.entities[entityName]
	if !ok {
		return false
	}
	for _, f := range entity.Fields {
		if f.Name == expr.Field {
			return f.Type.Name == "String"
		}
	}
	return false
}

func (g *rustHelper) resolveEntityName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.IndexExpr:
		if ident, ok := e.Object.(*ast.Identifier); ok {
			return g.resolveArrayElementEntity(ident.Name)
		}
	}
	return ""
}

func (g *rustHelper) resolveArrayElementEntity(varName string) string {
	for _, fn := range g.functions {
		for _, p := range fn.Params {
			if p.Name == varName && p.Type.Name == "Array" && len(p.Type.TypeArgs) == 1 {
				elemTypeName := p.Type.TypeArgs[0].Name
				if _, ok := g.entities[elemTypeName]; ok {
					return elemTypeName
				}
			}
		}
	}
	return ""
}

func (g *rustHelper) mangleOldExpr(e ast.Expression) string {
	text := g.exprToText(e)
	text = strings.ReplaceAll(text, ".", "_")
	text = strings.ReplaceAll(text, "(", "")
	text = strings.ReplaceAll(text, ")", "")
	text = strings.ReplaceAll(text, " ", "_")
	return "__old_" + text
}

func (g *rustHelper) exprToText(e ast.Expression) string {
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

func (g *rustHelper) generateForallExpr(expr *ast.ForallExpr) string {
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

func (g *rustHelper) generateExistsExpr(expr *ast.ExistsExpr) string {
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

func (g *rustHelper) generateMatchExpr(expr *ast.MatchExpr) string {
	var buf strings.Builder
	buf.WriteString("match ")
	buf.WriteString(g.generateExpr(expr.Scrutinee))
	buf.WriteString(" {\n")

	for _, arm := range expr.Arms {
		buf.WriteString("    ")
		buf.WriteString(g.generateMatchPattern(arm.Pattern))
		buf.WriteString(" => ")
		buf.WriteString(g.generateExpr(arm.Body))
		buf.WriteString(",\n")
	}

	buf.WriteString("}")
	return buf.String()
}

func (g *rustHelper) generateMatchPattern(pattern *ast.MatchPattern) string {
	if pattern.IsWildcard {
		return "_"
	}

	if pattern.VariantName == "Ok" || pattern.VariantName == "Err" || pattern.VariantName == "Some" {
		if len(pattern.Bindings) == 1 {
			return fmt.Sprintf("%s(%s)", pattern.VariantName, pattern.Bindings[0])
		}
		return pattern.VariantName
	}
	if pattern.VariantName == "None" {
		return "None"
	}

	enumName := g.resolveEnumNameForVariant(pattern.VariantName)

	if len(pattern.Bindings) == 0 {
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	enumDecl := g.enums[enumName]
	if enumDecl == nil {
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
		return fmt.Sprintf("%s::%s", enumName, pattern.VariantName)
	}

	var fields []string
	for i, binding := range pattern.Bindings {
		if i < len(variant.Fields) {
			fieldName := variant.Fields[i].Name
			fields = append(fields, fmt.Sprintf("%s: %s", fieldName, binding))
		}
	}

	return fmt.Sprintf("%s::%s { %s }", enumName, pattern.VariantName, strings.Join(fields, ", "))
}

func (g *rustHelper) resolveEnumNameForVariant(variantName string) string {
	for enumName, enumDecl := range g.enums {
		for _, v := range enumDecl.Variants {
			if v.Name == variantName {
				return enumName
			}
		}
	}
	return "UnknownEnum"
}
