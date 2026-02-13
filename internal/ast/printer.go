package ast

import (
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/lexer"
)

// Print returns a tree-like string representation of the AST for debugging
func Print(node Node) string {
	var sb strings.Builder
	printNode(&sb, node, 0)
	return sb.String()
}

func printNode(sb *strings.Builder, node Node, indent int) {
	if node == nil {
		return
	}

	prefix := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *Program:
		sb.WriteString(prefix + "Program\n")
		if n.Module != nil {
			printNode(sb, n.Module, indent+1)
		}
		for _, imp := range n.Imports {
			printNode(sb, imp, indent+1)
		}
		for _, fn := range n.Functions {
			printNode(sb, fn, indent+1)
		}
		for _, ent := range n.Entities {
			printNode(sb, ent, indent+1)
		}
		for _, enum := range n.Enums {
			printNode(sb, enum, indent+1)
		}
		for _, intent := range n.Intents {
			printNode(sb, intent, indent+1)
		}

	case *ModuleDecl:
		sb.WriteString(fmt.Sprintf("%sModule: %s v%s\n", prefix, n.Name, n.Version))

	case *ImportDecl:
		sb.WriteString(fmt.Sprintf("%sImport: %s\n", prefix, n.Path))

	case *FunctionDecl:
		modifiers := ""
		if n.IsPublic {
			modifiers += "public "
		}
		if n.IsEntry {
			modifiers += "entry "
		}
		if modifiers != "" {
			modifiers = " (" + strings.TrimSpace(modifiers) + ")"
		}
		sb.WriteString(fmt.Sprintf("%sFunction: %s%s\n", prefix, n.Name, modifiers))

		if len(n.Params) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Params:\n", prefix))
			for _, p := range n.Params {
				printNode(sb, p, indent+2)
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s  Params: none\n", prefix))
		}

		if n.ReturnType != nil {
			sb.WriteString(fmt.Sprintf("%s  Returns: %s\n", prefix, n.ReturnType.Name))
		}

		if len(n.Requires) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Requires:\n", prefix))
			for _, req := range n.Requires {
				printNode(sb, req, indent+2)
			}
		}

		if len(n.Ensures) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Ensures:\n", prefix))
			for _, ens := range n.Ensures {
				printNode(sb, ens, indent+2)
			}
		}

		if n.Body != nil {
			sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
			printNode(sb, n.Body, indent+2)
		}

	case *Param:
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, n.Name, n.Type.Name))

	case *ContractClause:
		sb.WriteString(fmt.Sprintf("%s%s\n", prefix, n.RawText))
		if n.Expr != nil {
			printNode(sb, n.Expr, indent+1)
		}

	case *EntityDecl:
		visibility := ""
		if n.IsPublic {
			visibility = " (public)"
		}
		sb.WriteString(fmt.Sprintf("%sEntity: %s%s\n", prefix, n.Name, visibility))

		if len(n.Fields) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Fields:\n", prefix))
			for _, f := range n.Fields {
				printNode(sb, f, indent+2)
			}
		}

		if len(n.Invariants) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Invariants:\n", prefix))
			for _, inv := range n.Invariants {
				printNode(sb, inv, indent+2)
			}
		}

		if n.Constructor != nil {
			sb.WriteString(fmt.Sprintf("%s  Constructor:\n", prefix))
			printNode(sb, n.Constructor, indent+2)
		}

		if len(n.Methods) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Methods:\n", prefix))
			for _, m := range n.Methods {
				printNode(sb, m, indent+2)
			}
		}

	case *FieldDecl:
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, n.Name, n.Type.Name))

	case *InvariantDecl:
		sb.WriteString(fmt.Sprintf("%s%s\n", prefix, n.RawText))
		if n.Expr != nil {
			printNode(sb, n.Expr, indent+1)
		}

	case *ConstructorDecl:
		sb.WriteString(fmt.Sprintf("%sConstructor\n", prefix))

		if len(n.Params) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Params:\n", prefix))
			for _, p := range n.Params {
				printNode(sb, p, indent+2)
			}
		}

		if len(n.Requires) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Requires:\n", prefix))
			for _, req := range n.Requires {
				printNode(sb, req, indent+2)
			}
		}

		if len(n.Ensures) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Ensures:\n", prefix))
			for _, ens := range n.Ensures {
				printNode(sb, ens, indent+2)
			}
		}

		if n.Body != nil {
			sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
			printNode(sb, n.Body, indent+2)
		}

	case *MethodDecl:
		sb.WriteString(fmt.Sprintf("%sMethod: %s\n", prefix, n.Name))

		if len(n.Params) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Params:\n", prefix))
			for _, p := range n.Params {
				printNode(sb, p, indent+2)
			}
		}

		if n.ReturnType != nil {
			sb.WriteString(fmt.Sprintf("%s  Returns: %s\n", prefix, n.ReturnType.Name))
		}

		if len(n.Requires) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Requires:\n", prefix))
			for _, req := range n.Requires {
				printNode(sb, req, indent+2)
			}
		}

		if len(n.Ensures) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Ensures:\n", prefix))
			for _, ens := range n.Ensures {
				printNode(sb, ens, indent+2)
			}
		}

		if n.Body != nil {
			sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
			printNode(sb, n.Body, indent+2)
		}

	case *IntentDecl:
		sb.WriteString(fmt.Sprintf("%sIntent: %s\n", prefix, n.Description))

		if len(n.Goals) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Goals:\n", prefix))
			for _, goal := range n.Goals {
				sb.WriteString(fmt.Sprintf("%s    - %s\n", prefix, goal))
			}
		}

		if len(n.Constraints) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Constraints:\n", prefix))
			for _, constraint := range n.Constraints {
				sb.WriteString(fmt.Sprintf("%s    - %s\n", prefix, constraint))
			}
		}

		if len(n.Guarantees) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Guarantees:\n", prefix))
			for _, guarantee := range n.Guarantees {
				sb.WriteString(fmt.Sprintf("%s    - %s\n", prefix, guarantee))
			}
		}

		if len(n.VerifiedBy) > 0 {
			sb.WriteString(fmt.Sprintf("%s  VerifiedBy:\n", prefix))
			for _, vb := range n.VerifiedBy {
				printNode(sb, vb, indent+2)
			}
		}

	case *VerifiedByRef:
		sb.WriteString(fmt.Sprintf("%s%s\n", prefix, strings.Join(n.Parts, ".")))

	case *EnumDecl:
		visibility := ""
		if n.IsPublic {
			visibility = " (public)"
		}
		sb.WriteString(fmt.Sprintf("%sEnum: %s%s\n", prefix, n.Name, visibility))
		if len(n.Variants) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Variants:\n", prefix))
			for _, v := range n.Variants {
				printNode(sb, v, indent+2)
			}
		}

	case *EnumVariant:
		if len(n.Fields) == 0 {
			sb.WriteString(fmt.Sprintf("%s%s (unit variant)\n", prefix, n.Name))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, n.Name))
			sb.WriteString(fmt.Sprintf("%s  Fields:\n", prefix))
			for _, f := range n.Fields {
				printNode(sb, f, indent+2)
			}
		}

	case *MatchExpr:
		sb.WriteString(fmt.Sprintf("%sMatchExpr\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  Scrutinee:\n", prefix))
		printNode(sb, n.Scrutinee, indent+2)
		if len(n.Arms) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Arms:\n", prefix))
			for _, arm := range n.Arms {
				printNode(sb, arm, indent+2)
			}
		}

	case *MatchArm:
		sb.WriteString(fmt.Sprintf("%sMatchArm\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  Pattern:\n", prefix))
		printNode(sb, n.Pattern, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
		printNode(sb, n.Body, indent+2)

	case *MatchPattern:
		if n.IsWildcard {
			sb.WriteString(fmt.Sprintf("%s_ (wildcard)\n", prefix))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s", prefix, n.VariantName))
			if len(n.Bindings) > 0 {
				sb.WriteString(fmt.Sprintf("(%s)", strings.Join(n.Bindings, ", ")))
			}
			sb.WriteString("\n")
		}

	case *Block:
		for _, stmt := range n.Statements {
			printNode(sb, stmt, indent)
		}

	case *LetStmt:
		mutable := "false"
		if n.Mutable {
			mutable = "true"
		}
		sb.WriteString(fmt.Sprintf("%sLetStmt: %s (mutable=%s)\n", prefix, n.Name, mutable))
		if n.Type != nil {
			sb.WriteString(fmt.Sprintf("%s  Type: %s\n", prefix, n.Type.Name))
		}
		if n.Value != nil {
			sb.WriteString(fmt.Sprintf("%s  Value:\n", prefix))
			printNode(sb, n.Value, indent+2)
		}

	case *AssignStmt:
		sb.WriteString(fmt.Sprintf("%sAssignStmt\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  Target:\n", prefix))
		printNode(sb, n.Target, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Value:\n", prefix))
		printNode(sb, n.Value, indent+2)

	case *ReturnStmt:
		sb.WriteString(fmt.Sprintf("%sReturnStmt\n", prefix))
		if n.Value != nil {
			sb.WriteString(fmt.Sprintf("%s  Value:\n", prefix))
			printNode(sb, n.Value, indent+2)
		}

	case *IfStmt:
		sb.WriteString(fmt.Sprintf("%sIfStmt\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  Condition:\n", prefix))
		printNode(sb, n.Condition, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Then:\n", prefix))
		printNode(sb, n.Then, indent+2)
		if n.Else != nil {
			sb.WriteString(fmt.Sprintf("%s  Else:\n", prefix))
			printNode(sb, n.Else, indent+2)
		}

	case *WhileStmt:
		sb.WriteString(fmt.Sprintf("%sWhileStmt\n", prefix))
		sb.WriteString(fmt.Sprintf("%s  Condition:\n", prefix))
		printNode(sb, n.Condition, indent+2)
		if len(n.Invariants) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Invariants:\n", prefix))
			for _, inv := range n.Invariants {
				printNode(sb, inv, indent+2)
			}
		}
		if n.Decreases != nil {
			sb.WriteString(fmt.Sprintf("%s  Decreases: %s\n", prefix, n.Decreases.RawText))
			printNode(sb, n.Decreases.Expr, indent+2)
		}
		sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
		printNode(sb, n.Body, indent+2)

	case *BreakStmt:
		sb.WriteString(fmt.Sprintf("%sBreakStmt\n", prefix))

	case *ContinueStmt:
		sb.WriteString(fmt.Sprintf("%sContinueStmt\n", prefix))

	case *ExprStmt:
		sb.WriteString(fmt.Sprintf("%sExprStmt\n", prefix))
		printNode(sb, n.Expr, indent+1)

	case *BinaryExpr:
		sb.WriteString(fmt.Sprintf("%sBinaryExpr: %s\n", prefix, tokenTypeToString(n.Op)))
		sb.WriteString(fmt.Sprintf("%s  Left:\n", prefix))
		printNode(sb, n.Left, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Right:\n", prefix))
		printNode(sb, n.Right, indent+2)

	case *UnaryExpr:
		sb.WriteString(fmt.Sprintf("%sUnaryExpr: %s\n", prefix, tokenTypeToString(n.Op)))
		sb.WriteString(fmt.Sprintf("%s  Operand:\n", prefix))
		printNode(sb, n.Operand, indent+2)

	case *CallExpr:
		sb.WriteString(fmt.Sprintf("%sCallExpr: %s\n", prefix, n.Function))
		if len(n.Args) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Args:\n", prefix))
			for _, arg := range n.Args {
				printNode(sb, arg, indent+2)
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s  Args: none\n", prefix))
		}

	case *MethodCallExpr:
		sb.WriteString(fmt.Sprintf("%sMethodCallExpr: %s\n", prefix, n.Method))
		sb.WriteString(fmt.Sprintf("%s  Object:\n", prefix))
		printNode(sb, n.Object, indent+2)
		if len(n.Args) > 0 {
			sb.WriteString(fmt.Sprintf("%s  Args:\n", prefix))
			for _, arg := range n.Args {
				printNode(sb, arg, indent+2)
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s  Args: none\n", prefix))
		}

	case *FieldAccessExpr:
		sb.WriteString(fmt.Sprintf("%sFieldAccessExpr: %s\n", prefix, n.Field))
		sb.WriteString(fmt.Sprintf("%s  Object:\n", prefix))
		printNode(sb, n.Object, indent+2)

	case *OldExpr:
		sb.WriteString(fmt.Sprintf("%sOldExpr\n", prefix))
		printNode(sb, n.Expr, indent+1)

	case *Identifier:
		sb.WriteString(fmt.Sprintf("%sIdentifier: %s\n", prefix, n.Name))

	case *SelfExpr:
		sb.WriteString(fmt.Sprintf("%sSelfExpr\n", prefix))

	case *ResultExpr:
		sb.WriteString(fmt.Sprintf("%sResultExpr\n", prefix))

	case *IntLit:
		sb.WriteString(fmt.Sprintf("%sIntLit: %s\n", prefix, n.Value))

	case *FloatLit:
		sb.WriteString(fmt.Sprintf("%sFloatLit: %s\n", prefix, n.Value))

	case *StringLit:
		sb.WriteString(fmt.Sprintf("%sStringLit: %s\n", prefix, n.Value))

	case *BoolLit:
		sb.WriteString(fmt.Sprintf("%sBoolLit: %t\n", prefix, n.Value))

	case *ForallExpr:
		sb.WriteString(fmt.Sprintf("%sForallExpr: %s\n", prefix, n.Variable))
		sb.WriteString(fmt.Sprintf("%s  Domain:\n", prefix))
		printNode(sb, n.Domain, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
		printNode(sb, n.Body, indent+2)

	case *ExistsExpr:
		sb.WriteString(fmt.Sprintf("%sExistsExpr: %s\n", prefix, n.Variable))
		sb.WriteString(fmt.Sprintf("%s  Domain:\n", prefix))
		printNode(sb, n.Domain, indent+2)
		sb.WriteString(fmt.Sprintf("%s  Body:\n", prefix))
		printNode(sb, n.Body, indent+2)

	case *TryExpr:
		sb.WriteString(fmt.Sprintf("%sTryExpr\n", prefix))
		printNode(sb, n.Expr, indent+1)

	default:
		sb.WriteString(fmt.Sprintf("%sUnknown node type: %T\n", prefix, node))
	}
}

func tokenTypeToString(tt lexer.TokenType) string {
	switch tt {
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
	case lexer.LEQ:
		return "<="
	case lexer.GT:
		return ">"
	case lexer.GEQ:
		return ">="
	case lexer.IMPLIES:
		return "implies"
	case lexer.AND:
		return "and"
	case lexer.OR:
		return "or"
	case lexer.NOT:
		return "not"
	default:
		return fmt.Sprintf("token(%d)", tt)
	}
}
