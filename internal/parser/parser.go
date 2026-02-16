package parser

import (
	"strings"

	"github.com/lhaig/intent/internal/ast"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/lexer"
)

// New creates a new parser
func New(source string) *Parser {
	l := lexer.New(source)
	tokens := l.Tokenize()
	return &Parser{
		tokens: tokens,
		pos:    0,
		diags:  diagnostic.New(),
		source: source,
	}
}

// Diagnostics returns the parser's diagnostics
func (p *Parser) Diagnostics() *diagnostic.Diagnostics {
	return p.diags
}

// Parse parses the token stream into a Program AST
func (p *Parser) Parse() *ast.Program {
	prog := &ast.Program{}
	prog.Module = p.parseModuleDecl()

	// Parse import declarations
	for p.check(lexer.IMPORT) {
		prog.Imports = append(prog.Imports, p.parseImportDecl())
	}

	// Parse top-level declarations
	for !p.check(lexer.EOF) {
		// Check for public keyword
		isPublic := false
		if p.check(lexer.PUBLIC) {
			p.advance()
			isPublic = true
		}

		switch p.current().Type {
		case lexer.ENTRY:
			fn := p.parseFunctionDecl()
			fn.IsPublic = isPublic
			prog.Functions = append(prog.Functions, fn)
		case lexer.FUNCTION:
			fn := p.parseFunctionDecl()
			fn.IsPublic = isPublic
			prog.Functions = append(prog.Functions, fn)
		case lexer.ENTITY:
			ent := p.parseEntityDecl()
			ent.IsPublic = isPublic
			prog.Entities = append(prog.Entities, ent)
		case lexer.ENUM:
			enum := p.parseEnumDecl()
			enum.IsPublic = isPublic
			prog.Enums = append(prog.Enums, enum)
		case lexer.INTENT:
			if isPublic {
				p.diags.Errorf(p.current().Line, p.current().Column,
					"'public' cannot be applied to intent declarations")
			}
			prog.Intents = append(prog.Intents, p.parseIntentDecl())
		default:
			if isPublic {
				p.diags.Errorf(p.current().Line, p.current().Column,
					"expected function, entity, or enum after 'public'")
			} else {
				p.diags.Errorf(p.current().Line, p.current().Column,
					"unexpected token %s at top level", p.current().Type)
			}
			startPos := p.pos
			p.synchronize()
			if p.pos == startPos {
				p.advance() // ensure forward progress to avoid infinite loop
			}
		}
	}
	return prog
}

// parseModuleDecl parses: module <name> version "<version>";
func (p *Parser) parseModuleDecl() *ast.ModuleDecl {
	tok := p.expect(lexer.MODULE)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.VERSION)
	version := p.expect(lexer.STRING_LIT)
	p.expect(lexer.SEMICOLON)

	// Strip quotes from version string
	v := version.Literal
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}

	return &ast.ModuleDecl{
		Name:    name.Literal,
		Version: v,
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

// parseImportDecl parses: import "path";
func (p *Parser) parseImportDecl() *ast.ImportDecl {
	tok := p.expect(lexer.IMPORT)
	pathTok := p.expect(lexer.STRING_LIT)
	p.expect(lexer.SEMICOLON)

	// Strip quotes from the import path
	path := pathTok.Literal
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		path = path[1 : len(path)-1]
	}

	return &ast.ImportDecl{
		Path:   path,
		Alias:  "", // no aliasing in v1
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// parseFunctionDecl parses: [entry] function <name>(<params>) returns <type> [requires ...] [ensures ...] { ... }
func (p *Parser) parseFunctionDecl() *ast.FunctionDecl {
	isEntry := false
	tok := p.current()

	if p.match(lexer.ENTRY) {
		isEntry = true
	}

	p.expect(lexer.FUNCTION)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.LPAREN)
	params := p.parseParamList()
	p.expect(lexer.RPAREN)
	p.expect(lexer.RETURNS)
	retType := p.parseTypeRef()
	requires := p.parseContractClauses(lexer.REQUIRES)
	ensures := p.parseContractClauses(lexer.ENSURES)
	body := p.parseBlock()

	return &ast.FunctionDecl{
		Name:       name.Literal,
		IsEntry:    isEntry,
		Params:     params,
		ReturnType: retType,
		Requires:   requires,
		Ensures:    ensures,
		Body:       body,
		Line:       tok.Line,
		Column:     tok.Column,
	}
}

// parseEntityDecl parses: entity <name> { ... }
func (p *Parser) parseEntityDecl() *ast.EntityDecl {
	tok := p.expect(lexer.ENTITY)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.LBRACE)

	entity := &ast.EntityDecl{
		Name:   name.Literal,
		Line:   tok.Line,
		Column: tok.Column,
	}

	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		switch p.current().Type {
		case lexer.FIELD:
			entity.Fields = append(entity.Fields, p.parseFieldDecl())
		case lexer.INVARIANT:
			entity.Invariants = append(entity.Invariants, p.parseInvariantDecl())
		case lexer.CONSTRUCTOR:
			entity.Constructor = p.parseConstructorDecl()
		case lexer.METHOD:
			entity.Methods = append(entity.Methods, p.parseMethodDecl())
		default:
			p.diags.Errorf(p.current().Line, p.current().Column,
				"unexpected token %s in entity body", p.current().Type)
			p.synchronize()
		}
	}
	p.expect(lexer.RBRACE)
	return entity
}

// parseFieldDecl parses: field <name>: <type>;
func (p *Parser) parseFieldDecl() *ast.FieldDecl {
	tok := p.expect(lexer.FIELD)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.COLON)
	fieldType := p.parseTypeRef()
	p.expect(lexer.SEMICOLON)

	return &ast.FieldDecl{
		Name:   name.Literal,
		Type:   fieldType,
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// parseInvariantDecl parses: invariant <expr>;
func (p *Parser) parseInvariantDecl() *ast.InvariantDecl {
	tok := p.expect(lexer.INVARIANT)
	startPos := p.pos
	expr := p.parseExpression()
	rawText := p.extractRawText(startPos)
	p.expect(lexer.SEMICOLON)

	return &ast.InvariantDecl{
		Expr:    expr,
		RawText: rawText,
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

// parseConstructorDecl parses: constructor(<params>) [requires ...] [ensures ...] { ... }
func (p *Parser) parseConstructorDecl() *ast.ConstructorDecl {
	tok := p.expect(lexer.CONSTRUCTOR)
	p.expect(lexer.LPAREN)
	params := p.parseParamList()
	p.expect(lexer.RPAREN)
	requires := p.parseContractClauses(lexer.REQUIRES)
	ensures := p.parseContractClauses(lexer.ENSURES)
	body := p.parseBlock()

	return &ast.ConstructorDecl{
		Params:   params,
		Requires: requires,
		Ensures:  ensures,
		Body:     body,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// parseMethodDecl parses: method <name>(<params>) returns <type> [requires ...] [ensures ...] { ... }
func (p *Parser) parseMethodDecl() *ast.MethodDecl {
	tok := p.expect(lexer.METHOD)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.LPAREN)
	params := p.parseParamList()
	p.expect(lexer.RPAREN)
	p.expect(lexer.RETURNS)
	retType := p.parseTypeRef()
	requires := p.parseContractClauses(lexer.REQUIRES)
	ensures := p.parseContractClauses(lexer.ENSURES)
	body := p.parseBlock()

	return &ast.MethodDecl{
		Name:       name.Literal,
		Params:     params,
		ReturnType: retType,
		Requires:   requires,
		Ensures:    ensures,
		Body:       body,
		Line:       tok.Line,
		Column:     tok.Column,
	}
}

// parseEnumDecl parses: enum <name> { Variant1, Variant2(field: Type), ... }
func (p *Parser) parseEnumDecl() *ast.EnumDecl {
	tok := p.expect(lexer.ENUM)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.LBRACE)

	enum := &ast.EnumDecl{
		Name:   name.Literal,
		Line:   tok.Line,
		Column: tok.Column,
	}

	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		variantTok := p.expect(lexer.IDENT)
		variant := &ast.EnumVariant{
			Name:   variantTok.Literal,
			Line:   variantTok.Line,
			Column: variantTok.Column,
		}

		// Check for data-carrying variant: VariantName(field: Type, ...)
		if p.match(lexer.LPAREN) {
			variant.Fields = p.parseVariantFields()
			p.expect(lexer.RPAREN)
		}

		enum.Variants = append(enum.Variants, variant)

		// Allow optional trailing comma
		if !p.check(lexer.RBRACE) {
			p.expect(lexer.COMMA)
		}
	}

	p.expect(lexer.RBRACE)
	return enum
}

// parseVariantFields parses the fields of a variant: field1: Type1, field2: Type2, ...
func (p *Parser) parseVariantFields() []*ast.FieldDecl {
	var fields []*ast.FieldDecl

	if p.check(lexer.RPAREN) {
		return fields
	}

	for {
		fieldTok := p.expect(lexer.IDENT)
		p.expect(lexer.COLON)
		fieldType := p.parseTypeRef()

		fields = append(fields, &ast.FieldDecl{
			Name:   fieldTok.Literal,
			Type:   fieldType,
			Line:   fieldTok.Line,
			Column: fieldTok.Column,
		})

		if !p.match(lexer.COMMA) {
			break
		}
	}

	return fields
}

// parseIntentDecl parses: intent "<desc>" { ... }
func (p *Parser) parseIntentDecl() *ast.IntentDecl {
	tok := p.expect(lexer.INTENT)
	desc := p.expect(lexer.STRING_LIT)
	p.expect(lexer.LBRACE)

	intent := &ast.IntentDecl{
		Description: stripQuotes(desc.Literal),
		Line:        tok.Line,
		Column:      tok.Column,
	}

	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		switch p.current().Type {
		case lexer.GOAL:
			p.advance()
			if p.check(lexer.COLON) {
				p.advance()
			}
			s := p.expect(lexer.STRING_LIT)
			intent.Goals = append(intent.Goals, stripQuotes(s.Literal))
			p.expect(lexer.SEMICOLON)
		case lexer.CONSTRAINT:
			p.advance()
			if p.check(lexer.COLON) {
				p.advance()
			}
			s := p.expect(lexer.STRING_LIT)
			intent.Constraints = append(intent.Constraints, stripQuotes(s.Literal))
			p.expect(lexer.SEMICOLON)
		case lexer.GUARANTEE:
			p.advance()
			if p.check(lexer.COLON) {
				p.advance()
			}
			s := p.expect(lexer.STRING_LIT)
			intent.Guarantees = append(intent.Guarantees, stripQuotes(s.Literal))
			p.expect(lexer.SEMICOLON)
		case lexer.VERIFIED_BY:
			p.advance()
			if p.check(lexer.COLON) {
				p.advance()
			}
			// Can be a single ref or a bracketed list
			if p.check(lexer.LBRACKET) {
				p.advance()
				for !p.check(lexer.RBRACKET) && !p.check(lexer.EOF) {
					intent.VerifiedBy = append(intent.VerifiedBy, p.parseVerifiedByRef())
					if !p.match(lexer.COMMA) {
						break
					}
				}
				p.expect(lexer.RBRACKET)
			} else {
				intent.VerifiedBy = append(intent.VerifiedBy, p.parseVerifiedByRef())
			}
			p.expect(lexer.SEMICOLON)
		default:
			p.diags.Errorf(p.current().Line, p.current().Column,
				"unexpected token %s in intent block", p.current().Type)
			p.synchronize()
		}
	}
	p.expect(lexer.RBRACE)
	return intent
}

// parseVerifiedByRef parses: Ident.Ident.Ident...
func (p *Parser) parseVerifiedByRef() *ast.VerifiedByRef {
	tok := p.current()
	var parts []string
	name := p.expect(lexer.IDENT)
	parts = append(parts, name.Literal)
	for p.match(lexer.DOT) {
		next := p.current()
		// Allow keywords as parts (e.g., "requires", "ensures", "invariant")
		switch next.Type {
		case lexer.IDENT, lexer.REQUIRES, lexer.ENSURES, lexer.INVARIANT, lexer.CONSTRUCTOR:
			parts = append(parts, next.Literal)
			p.advance()
		default:
			p.diags.Errorf(next.Line, next.Column, "expected identifier in verified_by path")
			break
		}
	}
	return &ast.VerifiedByRef{
		Parts:  parts,
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// parseParamList parses a comma-separated list of parameters
func (p *Parser) parseParamList() []*ast.Param {
	var params []*ast.Param
	if p.check(lexer.RPAREN) {
		return params
	}

	params = append(params, p.parseParam())
	for p.match(lexer.COMMA) {
		params = append(params, p.parseParam())
	}
	return params
}

// parseParam parses: <name>: <type>
func (p *Parser) parseParam() *ast.Param {
	name := p.expect(lexer.IDENT)
	p.expect(lexer.COLON)
	paramType := p.parseTypeRef()
	return &ast.Param{
		Name:   name.Literal,
		Type:   paramType,
		Line:   name.Line,
		Column: name.Column,
	}
}

// parseTypeRef parses a type reference
func (p *Parser) parseTypeRef() *ast.TypeRef {
	tok := p.current()
	var name string
	switch tok.Type {
	case lexer.INT_TYPE:
		p.advance()
		name = "Int"
	case lexer.FLOAT_TYPE:
		p.advance()
		name = "Float"
	case lexer.STRING_TYPE:
		p.advance()
		name = "String"
	case lexer.BOOL_TYPE:
		p.advance()
		name = "Bool"
	case lexer.VOID_TYPE:
		p.advance()
		name = "Void"
	case lexer.IDENT:
		p.advance()
		name = tok.Literal
	default:
		p.diags.Errorf(tok.Line, tok.Column, "expected type, got %s", tok.Type)
		return &ast.TypeRef{Name: "<error>", Line: tok.Line, Column: tok.Column}
	}

	// Parse type arguments: Array<Int>, Array<Array<Int>>, etc.
	// Context-based disambiguation: after a type name, '<' means type arguments
	var typeArgs []*ast.TypeRef
	if p.check(lexer.LT) {
		p.advance() // consume '<'
		for {
			typeArgs = append(typeArgs, p.parseTypeRef()) // recursive
			if !p.match(lexer.COMMA) {
				break
			}
		}
		p.expect(lexer.GT)
	}

	return &ast.TypeRef{
		Name:     name,
		TypeArgs: typeArgs,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// parseContractClauses parses zero or more requires/ensures clauses
func (p *Parser) parseContractClauses(keyword lexer.TokenType) []*ast.ContractClause {
	var clauses []*ast.ContractClause
	for p.check(keyword) {
		tok := p.advance()
		startPos := p.pos
		expr := p.parseExpression()
		rawText := p.extractRawText(startPos)
		clauses = append(clauses, &ast.ContractClause{
			Expr:    expr,
			RawText: rawText,
			Line:    tok.Line,
			Column:  tok.Column,
		})
	}
	return clauses
}

// parseBlock parses: { statement* }
func (p *Parser) parseBlock() *ast.Block {
	tok := p.expect(lexer.LBRACE)
	block := &ast.Block{
		Line:   tok.Line,
		Column: tok.Column,
	}
	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}
	p.expect(lexer.RBRACE)
	return block
}

// parseStatement parses a statement
func (p *Parser) parseStatement() ast.Statement {
	switch p.current().Type {
	case lexer.LET:
		return p.parseLetStmt()
	case lexer.RETURN:
		return p.parseReturnStmt()
	case lexer.IF:
		return p.parseIfStmt()
	case lexer.WHILE:
		return p.parseWhileStmt()
	case lexer.FOR:
		return p.parseForStmt()
	case lexer.BREAK:
		return p.parseBreakStmt()
	case lexer.CONTINUE:
		return p.parseContinueStmt()
	default:
		return p.parseExprStmtOrAssign()
	}
}

// parseLetStmt parses: let [mutable] <name>: <type> = <expr>;
func (p *Parser) parseLetStmt() *ast.LetStmt {
	tok := p.expect(lexer.LET)
	mutable := p.match(lexer.MUTABLE)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.COLON)
	varType := p.parseTypeRef()
	p.expect(lexer.ASSIGN)
	value := p.parseExpression()
	p.expect(lexer.SEMICOLON)

	return &ast.LetStmt{
		Name:    name.Literal,
		Mutable: mutable,
		Type:    varType,
		Value:   value,
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

// parseReturnStmt parses: return [expr];
func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	tok := p.expect(lexer.RETURN)
	var value ast.Expression
	if !p.check(lexer.SEMICOLON) {
		value = p.parseExpression()
	}
	p.expect(lexer.SEMICOLON)

	return &ast.ReturnStmt{
		Value:  value,
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// parseIfStmt parses: if <expr> { ... } [else { ... }]
func (p *Parser) parseIfStmt() *ast.IfStmt {
	tok := p.expect(lexer.IF)
	condition := p.parseExpression()
	then := p.parseBlock()

	var elseStmt ast.Statement
	if p.match(lexer.ELSE) {
		if p.check(lexer.IF) {
			elseStmt = p.parseIfStmt()
		} else {
			elseStmt = p.parseBlock()
		}
	}

	return &ast.IfStmt{
		Condition: condition,
		Then:      then,
		Else:      elseStmt,
		Line:      tok.Line,
		Column:    tok.Column,
	}
}

// parseWhileStmt parses: while <expr> { ... }
func (p *Parser) parseWhileStmt() *ast.WhileStmt {
	tok := p.expect(lexer.WHILE)
	condition := p.parseExpression()

	// Parse optional invariant clauses (reuse parseContractClauses with INVARIANT)
	invariants := p.parseContractClauses(lexer.INVARIANT)

	// Parse optional decreases clause (at most one)
	var decreases *ast.DecreaseClause
	if p.check(lexer.DECREASES) {
		decTok := p.advance()
		startPos := p.pos
		expr := p.parseExpression()
		rawText := p.extractRawText(startPos)
		decreases = &ast.DecreaseClause{
			Expr:    expr,
			RawText: rawText,
			Line:    decTok.Line,
			Column:  decTok.Column,
		}
	}

	body := p.parseBlock()

	return &ast.WhileStmt{
		Condition:  condition,
		Invariants: invariants,
		Decreases:  decreases,
		Body:       body,
		Line:       tok.Line,
		Column:     tok.Column,
	}
}

// parseForStmt parses: for <variable> in <iterable> { ... }
func (p *Parser) parseForStmt() *ast.ForInStmt {
	tok := p.expect(lexer.FOR)
	varName := p.expect(lexer.IDENT)
	p.expect(lexer.IN)

	// Parse iterable -- could be array expression or range (start..end)
	iterable := p.parseForIterable()

	body := p.parseBlock()

	return &ast.ForInStmt{
		Variable: varName.Literal,
		Iterable: iterable,
		Body:     body,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// parseForIterable parses the iterable expression in a for-in loop
// This can be either a range expression (start..end) or a regular expression
func (p *Parser) parseForIterable() ast.Expression {
	// Parse the first expression (could be start of range or entire iterable)
	expr := p.parseExpression()

	// Check if this is a range expression: expr..expr
	if p.check(lexer.DOTDOT) {
		p.advance() // consume '..'
		end := p.parseExpression()
		line, col := expr.Pos()
		return &ast.RangeExpr{
			Start:  expr,
			End:    end,
			Line:   line,
			Column: col,
		}
	}

	return expr
}

// parseBreakStmt parses: break;
func (p *Parser) parseBreakStmt() *ast.BreakStmt {
	tok := p.expect(lexer.BREAK)
	p.expect(lexer.SEMICOLON)
	return &ast.BreakStmt{Line: tok.Line, Column: tok.Column}
}

// parseContinueStmt parses: continue;
func (p *Parser) parseContinueStmt() *ast.ContinueStmt {
	tok := p.expect(lexer.CONTINUE)
	p.expect(lexer.SEMICOLON)
	return &ast.ContinueStmt{Line: tok.Line, Column: tok.Column}
}

// parseExprStmtOrAssign parses an expression statement or assignment
func (p *Parser) parseExprStmtOrAssign() ast.Statement {
	tok := p.current()
	expr := p.parseExpression()

	if p.check(lexer.ASSIGN) {
		p.advance()
		value := p.parseExpression()
		p.expect(lexer.SEMICOLON)
		return &ast.AssignStmt{
			Target: expr,
			Value:  value,
			Line:   tok.Line,
			Column: tok.Column,
		}
	}

	p.expect(lexer.SEMICOLON)
	return &ast.ExprStmt{
		Expr:   expr,
		Line:   tok.Line,
		Column: tok.Column,
	}
}

// Expression parsing - Pratt parser / precedence climbing

// Precedence levels (lowest to highest):
// 1. implies      (right-associative)
// 2. or           (left-associative)
// 3. and          (left-associative)
// 4. == !=        (left-associative)
// 5. < > <= >=    (left-associative)
// 6. + -          (left-associative)
// 7. * / %        (left-associative)
// 8. unary (- not)
// 9. postfix (. ())

const (
	precNone       = 0
	precImplies    = 1
	precOr         = 2
	precAnd        = 3
	precEquality   = 4
	precComparison = 5
	precAdditive   = 6
	precMulti      = 7
	precUnary      = 8
	precPostfix    = 9
)

func tokenPrecedence(tt lexer.TokenType) int {
	switch tt {
	case lexer.IMPLIES:
		return precImplies
	case lexer.OR:
		return precOr
	case lexer.AND:
		return precAnd
	case lexer.EQ, lexer.NEQ:
		return precEquality
	case lexer.LT, lexer.GT, lexer.LEQ, lexer.GEQ:
		return precComparison
	case lexer.PLUS, lexer.MINUS:
		return precAdditive
	case lexer.STAR, lexer.SLASH, lexer.PERCENT:
		return precMulti
	default:
		return precNone
	}
}

func (p *Parser) parseExpression() ast.Expression {
	return p.parsePrecedence(precImplies)
}

func (p *Parser) parsePrecedence(minPrec int) ast.Expression {
	left := p.parseUnary()

	for {
		prec := tokenPrecedence(p.current().Type)
		if prec < minPrec {
			break
		}

		op := p.advance()

		// Right-associative for implies
		nextPrec := prec + 1
		if op.Type == lexer.IMPLIES {
			nextPrec = prec
		}

		right := p.parsePrecedence(nextPrec)
		left = &ast.BinaryExpr{
			Left:   left,
			Op:     op.Type,
			Right:  right,
			Line:   op.Line,
			Column: op.Column,
		}
	}

	return left
}

func (p *Parser) parseUnary() ast.Expression {
	if p.check(lexer.MINUS) {
		op := p.advance()
		operand := p.parseUnary()
		return &ast.UnaryExpr{
			Op:      op.Type,
			Operand: operand,
			Line:    op.Line,
			Column:  op.Column,
		}
	}
	if p.check(lexer.NOT) {
		op := p.advance()
		operand := p.parseUnary()
		return &ast.UnaryExpr{
			Op:      op.Type,
			Operand: operand,
			Line:    op.Line,
			Column:  op.Column,
		}
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() ast.Expression {
	expr := p.parsePrimary()
	line, col := expr.Pos()

	for {
		if p.check(lexer.LBRACKET) {
			// Index access: expr[index]
			p.advance() // consume '['
			index := p.parseExpression()
			p.expect(lexer.RBRACKET)
			expr = &ast.IndexExpr{
				Object: expr,
				Index:  index,
				Line:   line,
				Column: col,
			}
		} else if p.check(lexer.DOT) {
			p.advance()
			name := p.expect(lexer.IDENT)
			if p.check(lexer.LPAREN) {
				// method call
				p.advance()
				args := p.parseArgList()
				p.expect(lexer.RPAREN)
				expr = &ast.MethodCallExpr{
					Object: expr,
					Method: name.Literal,
					Args:   args,
					Line:   name.Line,
					Column: name.Column,
				}
			} else {
				// field access
				expr = &ast.FieldAccessExpr{
					Object: expr,
					Field:  name.Literal,
					Line:   name.Line,
					Column: name.Column,
				}
			}
		} else if p.check(lexer.LPAREN) {
			// function call - only valid if expr is an identifier
			if ident, ok := expr.(*ast.Identifier); ok {
				p.advance()
				args := p.parseArgList()
				p.expect(lexer.RPAREN)
				expr = &ast.CallExpr{
					Function: ident.Name,
					Args:     args,
					Line:     ident.Line,
					Column:   ident.Column,
				}
			} else {
				break
			}
		} else if p.check(lexer.QUESTION) {
			// Try operator: expr?
			p.advance()
			expr = &ast.TryExpr{
				Expr:   expr,
				Line:   line,
				Column: col,
			}
		} else {
			break
		}
	}

	return expr
}

func (p *Parser) parsePrimary() ast.Expression {
	tok := p.current()

	switch tok.Type {
	case lexer.INT_LIT:
		p.advance()
		return &ast.IntLit{Value: tok.Literal, Line: tok.Line, Column: tok.Column}
	case lexer.FLOAT_LIT:
		p.advance()
		return &ast.FloatLit{Value: tok.Literal, Line: tok.Line, Column: tok.Column}
	case lexer.STRING_LIT:
		p.advance()
		return &ast.StringLit{Value: tok.Literal, Line: tok.Line, Column: tok.Column}
	case lexer.TRUE:
		p.advance()
		return &ast.BoolLit{Value: true, Line: tok.Line, Column: tok.Column}
	case lexer.FALSE:
		p.advance()
		return &ast.BoolLit{Value: false, Line: tok.Line, Column: tok.Column}
	case lexer.SELF:
		p.advance()
		return &ast.SelfExpr{Line: tok.Line, Column: tok.Column}
	case lexer.RESULT:
		p.advance()
		return &ast.ResultExpr{Line: tok.Line, Column: tok.Column}
	case lexer.OLD:
		p.advance()
		p.expect(lexer.LPAREN)
		expr := p.parseExpression()
		p.expect(lexer.RPAREN)
		return &ast.OldExpr{Expr: expr, Line: tok.Line, Column: tok.Column}
	case lexer.IDENT:
		p.advance()
		return &ast.Identifier{Name: tok.Literal, Line: tok.Line, Column: tok.Column}
	case lexer.LPAREN:
		p.advance()
		expr := p.parseExpression()
		p.expect(lexer.RPAREN)
		return expr
	case lexer.LBRACKET:
		return p.parseArrayLit()
	case lexer.FORALL:
		return p.parseForallExpr()
	case lexer.EXISTS:
		return p.parseExistsExpr()
	case lexer.MATCH:
		return p.parseMatchExpr()
	default:
		p.diags.Errorf(tok.Line, tok.Column, "unexpected token %s in expression", tok.Type)
		p.advance()
		return &ast.Identifier{Name: "<error>", Line: tok.Line, Column: tok.Column}
	}
}

func (p *Parser) parseArgList() []ast.Expression {
	var args []ast.Expression
	if p.check(lexer.RPAREN) {
		return args
	}
	args = append(args, p.parseExpression())
	for p.match(lexer.COMMA) {
		args = append(args, p.parseExpression())
	}
	return args
}

func (p *Parser) parseArrayLit() *ast.ArrayLit {
	tok := p.expect(lexer.LBRACKET)
	var elements []ast.Expression

	// Handle empty array: []
	if !p.check(lexer.RBRACKET) {
		elements = append(elements, p.parseExpression())
		for p.match(lexer.COMMA) {
			// Allow trailing comma
			if p.check(lexer.RBRACKET) {
				break
			}
			elements = append(elements, p.parseExpression())
		}
	}
	p.expect(lexer.RBRACKET)

	return &ast.ArrayLit{
		Elements: elements,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// extractRawText extracts the raw source text for a range of tokens
func (p *Parser) extractRawText(startPos int) string {
	if startPos >= len(p.tokens) || p.pos <= 0 {
		return ""
	}
	endPos := p.pos - 1
	if endPos >= len(p.tokens) {
		endPos = len(p.tokens) - 1
	}

	var parts []string
	for i := startPos; i <= endPos; i++ {
		parts = append(parts, p.tokens[i].Literal)
	}
	return strings.Join(parts, " ")
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseForallExpr parses: forall <var> in <start>..<end>: <expr>
func (p *Parser) parseForallExpr() *ast.ForallExpr {
	tok := p.expect(lexer.FORALL)
	varName := p.expect(lexer.IDENT).Literal
	p.expect(lexer.IN)

	// Parse range domain -- must be a range expression (start..end)
	startExpr := p.parseExpression()
	if !p.check(lexer.DOTDOT) {
		p.diags.Errorf(tok.Line, tok.Column,
			"forall requires bounded range domain (e.g., 0..n)")
		return &ast.ForallExpr{Variable: varName, Line: tok.Line, Column: tok.Column}
	}
	p.advance() // consume DOTDOT
	endExpr := p.parseExpression()
	domain := &ast.RangeExpr{Start: startExpr, End: endExpr, Line: tok.Line, Column: tok.Column}

	p.expect(lexer.COLON)
	body := p.parseExpression()

	return &ast.ForallExpr{
		Variable: varName,
		Domain:   domain,
		Body:     body,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// parseExistsExpr parses: exists <var> in <start>..<end>: <expr>
func (p *Parser) parseExistsExpr() *ast.ExistsExpr {
	tok := p.expect(lexer.EXISTS)
	varName := p.expect(lexer.IDENT).Literal
	p.expect(lexer.IN)

	// Parse range domain -- must be a range expression (start..end)
	startExpr := p.parseExpression()
	if !p.check(lexer.DOTDOT) {
		p.diags.Errorf(tok.Line, tok.Column,
			"exists requires bounded range domain (e.g., 0..n)")
		return &ast.ExistsExpr{Variable: varName, Line: tok.Line, Column: tok.Column}
	}
	p.advance() // consume DOTDOT
	endExpr := p.parseExpression()
	domain := &ast.RangeExpr{Start: startExpr, End: endExpr, Line: tok.Line, Column: tok.Column}

	p.expect(lexer.COLON)
	body := p.parseExpression()

	return &ast.ExistsExpr{
		Variable: varName,
		Domain:   domain,
		Body:     body,
		Line:     tok.Line,
		Column:   tok.Column,
	}
}

// parseMatchExpr parses: match <expr> { <pattern> => <expr>, ... }
func (p *Parser) parseMatchExpr() *ast.MatchExpr {
	tok := p.expect(lexer.MATCH)
	scrutinee := p.parseExpression()
	p.expect(lexer.LBRACE)

	var arms []*ast.MatchArm
	for !p.check(lexer.RBRACE) && !p.check(lexer.EOF) {
		arms = append(arms, p.parseMatchArm())
		// Allow optional comma between arms (and optional trailing comma)
		if p.check(lexer.COMMA) {
			p.advance()
		}
	}
	p.expect(lexer.RBRACE)

	return &ast.MatchExpr{
		Scrutinee: scrutinee,
		Arms:      arms,
		Line:      tok.Line,
		Column:    tok.Column,
	}
}

// parseMatchArm parses: <pattern> => <expr>
func (p *Parser) parseMatchArm() *ast.MatchArm {
	tok := p.current()
	pattern := p.parseMatchPattern()
	p.expect(lexer.ARROW)
	body := p.parseExpression()

	return &ast.MatchArm{
		Pattern: pattern,
		Body:    body,
		Line:    tok.Line,
		Column:  tok.Column,
	}
}

// parseMatchPattern parses a match pattern: _ | VariantName | VariantName(binding1, binding2, ...)
func (p *Parser) parseMatchPattern() *ast.MatchPattern {
	tok := p.current()

	// Check for wildcard pattern "_"
	if p.check(lexer.IDENT) && tok.Literal == "_" {
		p.advance()
		return &ast.MatchPattern{
			IsWildcard: true,
			Line:       tok.Line,
			Column:     tok.Column,
		}
	}

	// Otherwise expect a variant name
	variantName := p.expect(lexer.IDENT)

	// Check if this is a data-carrying variant with bindings
	if p.check(lexer.LPAREN) {
		p.advance()
		var bindings []string

		// Parse binding names (comma-separated identifiers)
		if !p.check(lexer.RPAREN) {
			bindings = append(bindings, p.expect(lexer.IDENT).Literal)
			for p.match(lexer.COMMA) {
				bindings = append(bindings, p.expect(lexer.IDENT).Literal)
			}
		}
		p.expect(lexer.RPAREN)

		return &ast.MatchPattern{
			VariantName: variantName.Literal,
			Bindings:    bindings,
			Line:        tok.Line,
			Column:      tok.Column,
		}
	}

	// Unit variant (no bindings)
	return &ast.MatchPattern{
		VariantName: variantName.Literal,
		Bindings:    nil,
		Line:        tok.Line,
		Column:      tok.Column,
	}
}
