package lexer

import "fmt"

// TokenType represents the type of a token
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF

	// Literals
	IDENT      // x, y, myVariable
	INT_LIT    // 123
	FLOAT_LIT  // 123.45
	STRING_LIT // "hello"

	// Keywords
	MODULE
	VERSION
	FUNCTION
	ENTRY
	RETURNS
	REQUIRES
	ENSURES
	ENTITY
	FIELD
	INVARIANT
	CONSTRUCTOR
	METHOD
	INTENT
	GOAL
	CONSTRAINT
	GUARANTEE
	VERIFIED_BY
	LET
	MUTABLE
	IF
	ELSE
	RETURN
	SELF
	OLD
	RESULT
	AND
	OR
	NOT
	IMPLIES
	TRUE
	FALSE
	WHILE
	BREAK
	CONTINUE
	FOR
	IN
	DECREASES
	FORALL
	EXISTS
	ENUM
	MATCH
	ARROW
	IMPORT
	PUBLIC

	// Type keywords
	INT_TYPE
	FLOAT_TYPE
	STRING_TYPE
	BOOL_TYPE
	VOID_TYPE

	// Operators
	PLUS    // +
	MINUS   // -
	STAR    // *
	SLASH   // /
	PERCENT // %
	EQ      // ==
	NEQ     // !=
	LT      // <
	GT      // >
	LEQ     // <=
	GEQ     // >=
	ASSIGN  // =

	// Delimiters
	LPAREN    // (
	RPAREN    // )
	LBRACE    // {
	RBRACE    // }
	LBRACKET  // [
	RBRACKET  // ]
	COMMA     // ,
	COLON     // :
	SEMICOLON // ;
	DOT       // .
	DOTDOT    // ..
	QUESTION  // ?
)

// Token represents a lexical token
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// String returns a string representation of the token type
func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case IDENT:
		return "IDENT"
	case INT_LIT:
		return "INT_LIT"
	case FLOAT_LIT:
		return "FLOAT_LIT"
	case STRING_LIT:
		return "STRING_LIT"
	case MODULE:
		return "MODULE"
	case VERSION:
		return "VERSION"
	case FUNCTION:
		return "FUNCTION"
	case ENTRY:
		return "ENTRY"
	case RETURNS:
		return "RETURNS"
	case REQUIRES:
		return "REQUIRES"
	case ENSURES:
		return "ENSURES"
	case ENTITY:
		return "ENTITY"
	case FIELD:
		return "FIELD"
	case INVARIANT:
		return "INVARIANT"
	case CONSTRUCTOR:
		return "CONSTRUCTOR"
	case METHOD:
		return "METHOD"
	case INTENT:
		return "INTENT"
	case GOAL:
		return "GOAL"
	case CONSTRAINT:
		return "CONSTRAINT"
	case GUARANTEE:
		return "GUARANTEE"
	case VERIFIED_BY:
		return "VERIFIED_BY"
	case LET:
		return "LET"
	case MUTABLE:
		return "MUTABLE"
	case IF:
		return "IF"
	case ELSE:
		return "ELSE"
	case RETURN:
		return "RETURN"
	case SELF:
		return "SELF"
	case OLD:
		return "OLD"
	case RESULT:
		return "RESULT"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	case IMPLIES:
		return "IMPLIES"
	case TRUE:
		return "TRUE"
	case FALSE:
		return "FALSE"
	case WHILE:
		return "WHILE"
	case BREAK:
		return "BREAK"
	case CONTINUE:
		return "CONTINUE"
	case FOR:
		return "FOR"
	case IN:
		return "IN"
	case DECREASES:
		return "DECREASES"
	case FORALL:
		return "FORALL"
	case EXISTS:
		return "EXISTS"
	case ENUM:
		return "ENUM"
	case MATCH:
		return "MATCH"
	case ARROW:
		return "ARROW"
	case IMPORT:
		return "IMPORT"
	case PUBLIC:
		return "PUBLIC"
	case INT_TYPE:
		return "INT_TYPE"
	case FLOAT_TYPE:
		return "FLOAT_TYPE"
	case STRING_TYPE:
		return "STRING_TYPE"
	case BOOL_TYPE:
		return "BOOL_TYPE"
	case VOID_TYPE:
		return "VOID_TYPE"
	case PLUS:
		return "PLUS"
	case MINUS:
		return "MINUS"
	case STAR:
		return "STAR"
	case SLASH:
		return "SLASH"
	case PERCENT:
		return "PERCENT"
	case EQ:
		return "EQ"
	case NEQ:
		return "NEQ"
	case LT:
		return "LT"
	case GT:
		return "GT"
	case LEQ:
		return "LEQ"
	case GEQ:
		return "GEQ"
	case ASSIGN:
		return "ASSIGN"
	case LPAREN:
		return "LPAREN"
	case RPAREN:
		return "RPAREN"
	case LBRACE:
		return "LBRACE"
	case RBRACE:
		return "RBRACE"
	case LBRACKET:
		return "LBRACKET"
	case RBRACKET:
		return "RBRACKET"
	case COMMA:
		return "COMMA"
	case COLON:
		return "COLON"
	case SEMICOLON:
		return "SEMICOLON"
	case DOT:
		return "DOT"
	case DOTDOT:
		return "DOTDOT"
	case QUESTION:
		return "QUESTION"
	default:
		return fmt.Sprintf("TokenType(%d)", t)
	}
}

// keywords maps keyword strings to their token types
var keywords = map[string]TokenType{
	"module":      MODULE,
	"version":     VERSION,
	"function":    FUNCTION,
	"entry":       ENTRY,
	"returns":     RETURNS,
	"requires":    REQUIRES,
	"ensures":     ENSURES,
	"entity":      ENTITY,
	"field":       FIELD,
	"invariant":   INVARIANT,
	"constructor": CONSTRUCTOR,
	"method":      METHOD,
	"intent":      INTENT,
	"goal":        GOAL,
	"constraint":  CONSTRAINT,
	"guarantee":   GUARANTEE,
	"verified_by": VERIFIED_BY,
	"let":         LET,
	"mutable":     MUTABLE,
	"if":          IF,
	"else":        ELSE,
	"return":      RETURN,
	"self":        SELF,
	"old":         OLD,
	"result":      RESULT,
	"and":         AND,
	"or":          OR,
	"not":         NOT,
	"implies":     IMPLIES,
	"true":        TRUE,
	"false":       FALSE,
	"while":       WHILE,
	"break":       BREAK,
	"continue":    CONTINUE,
	"for":         FOR,
	"in":          IN,
	"decreases":   DECREASES,
	"forall":      FORALL,
	"exists":      EXISTS,
	"enum":        ENUM,
	"match":       MATCH,
	"import":      IMPORT,
	"public":      PUBLIC,
	"Int":         INT_TYPE,
	"Float":       FLOAT_TYPE,
	"String":      STRING_TYPE,
	"Bool":        BOOL_TYPE,
	"Void":        VOID_TYPE,
}

// LookupIdent checks if an identifier is a keyword
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
