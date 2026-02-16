package lexer

import (
	"testing"
)

func TestNextToken_Operators(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:     "arithmetic operators",
			input:    "+ - * / %",
			expected: []TokenType{PLUS, MINUS, STAR, SLASH, PERCENT, EOF},
		},
		{
			name:     "comparison operators",
			input:    "== != < > <= >=",
			expected: []TokenType{EQ, NEQ, LT, GT, LEQ, GEQ, EOF},
		},
		{
			name:     "assignment operator",
			input:    "=",
			expected: []TokenType{ASSIGN, EOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, expectedType := range tt.expected {
				tok := l.NextToken()
				if tok.Type != expectedType {
					t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
						i, expectedType, tok.Type)
				}
			}
		})
	}
}

func TestNextToken_Delimiters(t *testing.T) {
	input := "( ) { } [ ] , : ; ."
	expected := []TokenType{
		LPAREN, RPAREN, LBRACE, RBRACE, LBRACKET, RBRACKET,
		COMMA, COLON, SEMICOLON, DOT, EOF,
	}

	l := New(input)
	for i, expectedType := range expected {
		tok := l.NextToken()
		if tok.Type != expectedType {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
				i, expectedType, tok.Type)
		}
	}
}

func TestNextToken_Keywords(t *testing.T) {
	tests := []struct {
		keyword  string
		expected TokenType
	}{
		{"module", MODULE},
		{"version", VERSION},
		{"function", FUNCTION},
		{"entry", ENTRY},
		{"returns", RETURNS},
		{"requires", REQUIRES},
		{"ensures", ENSURES},
		{"entity", ENTITY},
		{"field", FIELD},
		{"invariant", INVARIANT},
		{"constructor", CONSTRUCTOR},
		{"method", METHOD},
		{"intent", INTENT},
		{"goal", GOAL},
		{"constraint", CONSTRAINT},
		{"guarantee", GUARANTEE},
		{"verified_by", VERIFIED_BY},
		{"let", LET},
		{"mutable", MUTABLE},
		{"if", IF},
		{"else", ELSE},
		{"return", RETURN},
		{"self", SELF},
		{"old", OLD},
		{"result", RESULT},
		{"and", AND},
		{"or", OR},
		{"not", NOT},
		{"implies", IMPLIES},
		{"true", TRUE},
		{"false", FALSE},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			l := New(tt.keyword)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("keyword %q - wrong type. expected=%q, got=%q",
					tt.keyword, tt.expected, tok.Type)
			}
			if tok.Literal != tt.keyword {
				t.Errorf("keyword %q - wrong literal. expected=%q, got=%q",
					tt.keyword, tt.keyword, tok.Literal)
			}
		})
	}
}

func TestNextToken_TypeKeywords(t *testing.T) {
	tests := []struct {
		keyword  string
		expected TokenType
	}{
		{"Int", INT_TYPE},
		{"Float", FLOAT_TYPE},
		{"String", STRING_TYPE},
		{"Bool", BOOL_TYPE},
		{"Void", VOID_TYPE},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			l := New(tt.keyword)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("type keyword %q - wrong type. expected=%q, got=%q",
					tt.keyword, tt.expected, tok.Type)
			}
		})
	}
}

func TestNextToken_IntegerLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0", "0"},
		{"123", "123"},
		{"456789", "456789"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != INT_LIT {
				t.Errorf("expected INT_LIT, got %q", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("expected literal %q, got %q", tt.expected, tok.Literal)
			}
		})
	}
}

func TestNextToken_FloatLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0.0", "0.0"},
		{"123.45", "123.45"},
		{"3.14159", "3.14159"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != FLOAT_LIT {
				t.Errorf("expected FLOAT_LIT, got %q", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("expected literal %q, got %q", tt.expected, tok.Literal)
			}
		})
	}
}

func TestNextToken_StringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    `"hello"`,
			expected: `"hello"`,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: `""`,
		},
		{
			name:     "string with newline escape",
			input:    `"hello\nworld"`,
			expected: `"hello\nworld"`,
		},
		{
			name:     "string with tab escape",
			input:    `"hello\tworld"`,
			expected: `"hello\tworld"`,
		},
		{
			name:     "string with backslash escape",
			input:    `"path\\to\\file"`,
			expected: `"path\\to\\file"`,
		},
		{
			name:     "string with quote escape",
			input:    `"say \"hello\""`,
			expected: `"say \"hello\""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != STRING_LIT {
				t.Errorf("expected STRING_LIT, got %q", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("expected literal %q, got %q", tt.expected, tok.Literal)
			}
		})
	}
}

func TestNextToken_StringInterp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
		literal  string
	}{
		{
			name:     "simple interpolation",
			input:    `"hello {name}"`,
			expected: STRING_INTERP,
			literal:  `"hello {name}"`,
		},
		{
			name:     "multiple interpolations",
			input:    `"x={x}, y={y}"`,
			expected: STRING_INTERP,
			literal:  `"x={x}, y={y}"`,
		},
		{
			name:     "no interpolation",
			input:    `"hello world"`,
			expected: STRING_LIT,
			literal:  `"hello world"`,
		},
		{
			name:     "field access in interpolation",
			input:    `"Balance: {self.balance}"`,
			expected: STRING_INTERP,
			literal:  `"Balance: {self.balance}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tok.Type)
			}
			if tok.Literal != tt.literal {
				t.Errorf("expected literal %q, got %q", tt.literal, tok.Literal)
			}
		})
	}
}

func TestNextToken_Identifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"x", "x"},
		{"myVar", "myVar"},
		{"my_variable", "my_variable"},
		{"_private", "_private"},
		{"var123", "var123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != IDENT {
				t.Errorf("expected IDENT, got %q", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("expected literal %q, got %q", tt.expected, tok.Literal)
			}
		})
	}
}

func TestNextToken_IdentifiersVsKeywords(t *testing.T) {
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"if", IF},
		{"ifx", IDENT},
		{"return", RETURN},
		{"returns", RETURNS},
		{"returnValue", IDENT},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expectedType {
				t.Errorf("input %q - expected %q, got %q",
					tt.input, tt.expectedType, tok.Type)
			}
		})
	}
}

func TestNextToken_LineAndColumnTracking(t *testing.T) {
	input := `x = 5
y = 10`

	expected := []struct {
		tokenType TokenType
		line      int
		column    int
	}{
		{IDENT, 1, 1},
		{ASSIGN, 1, 3},
		{INT_LIT, 1, 5},
		{IDENT, 2, 1},
		{ASSIGN, 2, 3},
		{INT_LIT, 2, 5},
		{EOF, 2, 7},
	}

	l := New(input)
	for i, exp := range expected {
		tok := l.NextToken()
		if tok.Type != exp.tokenType {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		if tok.Line != exp.line {
			t.Errorf("token[%d] - wrong line. expected=%d, got=%d",
				i, exp.line, tok.Line)
		}
		if tok.Column != exp.column {
			t.Errorf("token[%d] - wrong column. expected=%d, got=%d",
				i, exp.column, tok.Column)
		}
	}
}

func TestNextToken_SingleLineComments(t *testing.T) {
	input := `x // this is a comment
y`

	expected := []TokenType{IDENT, IDENT, EOF}

	l := New(input)
	for i, expectedType := range expected {
		tok := l.NextToken()
		if tok.Type != expectedType {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
				i, expectedType, tok.Type)
		}
	}
}

func TestNextToken_MultiLineComments(t *testing.T) {
	input := `x /* this is a
multi-line comment */ y`

	expected := []TokenType{IDENT, IDENT, EOF}

	l := New(input)
	for i, expectedType := range expected {
		tok := l.NextToken()
		if tok.Type != expectedType {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
				i, expectedType, tok.Type)
		}
	}
}

func TestNextToken_CompleteMiniProgram(t *testing.T) {
	input := `module MyModule
version "1.0.0"

function add(x: Int, y: Int) returns Int {
    return x + y;
}

entity Counter {
    field value: Int;

    constructor(initial: Int)
    requires initial >= 0
    {
        self.value = initial;
    }
}`

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{MODULE, "module"},
		{IDENT, "MyModule"},
		{VERSION, "version"},
		{STRING_LIT, `"1.0.0"`},
		{FUNCTION, "function"},
		{IDENT, "add"},
		{LPAREN, "("},
		{IDENT, "x"},
		{COLON, ":"},
		{INT_TYPE, "Int"},
		{COMMA, ","},
		{IDENT, "y"},
		{COLON, ":"},
		{INT_TYPE, "Int"},
		{RPAREN, ")"},
		{RETURNS, "returns"},
		{INT_TYPE, "Int"},
		{LBRACE, "{"},
		{RETURN, "return"},
		{IDENT, "x"},
		{PLUS, "+"},
		{IDENT, "y"},
		{SEMICOLON, ";"},
		{RBRACE, "}"},
		{ENTITY, "entity"},
		{IDENT, "Counter"},
		{LBRACE, "{"},
		{FIELD, "field"},
		{IDENT, "value"},
		{COLON, ":"},
		{INT_TYPE, "Int"},
		{SEMICOLON, ";"},
		{CONSTRUCTOR, "constructor"},
		{LPAREN, "("},
		{IDENT, "initial"},
		{COLON, ":"},
		{INT_TYPE, "Int"},
		{RPAREN, ")"},
		{REQUIRES, "requires"},
		{IDENT, "initial"},
		{GEQ, ">="},
		{INT_LIT, "0"},
		{LBRACE, "{"},
		{SELF, "self"},
		{DOT, "."},
		{IDENT, "value"},
		{ASSIGN, "="},
		{IDENT, "initial"},
		{SEMICOLON, ";"},
		{RBRACE, "}"},
		{RBRACE, "}"},
		{EOF, ""},
	}

	l := New(input)
	for i, exp := range expected {
		tok := l.NextToken()
		if tok.Type != exp.tokenType {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q (literal: %q)",
				i, exp.tokenType, tok.Type, tok.Literal)
		}
		if tok.Literal != exp.literal {
			t.Errorf("token[%d] - wrong literal. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestNextToken_UnterminatedString(t *testing.T) {
	input := `"unterminated`

	l := New(input)
	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Errorf("expected ILLEGAL for unterminated string, got %q", tok.Type)
	}
}

func TestNextToken_IllegalCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected byte
	}{
		{"@", '@'},
		{"#", '#'},
		{"$", '$'},
		{"&", '&'},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != ILLEGAL {
				t.Errorf("expected ILLEGAL, got %q", tok.Type)
			}
			if tok.Literal != string(tt.expected) {
				t.Errorf("expected literal %q, got %q", string(tt.expected), tok.Literal)
			}
		})
	}
}

func TestNextToken_ExclamationWithoutEquals(t *testing.T) {
	input := "!"

	l := New(input)
	tok := l.NextToken()
	if tok.Type != ILLEGAL {
		t.Errorf("expected ILLEGAL for standalone '!', got %q", tok.Type)
	}
}

func TestTokenize(t *testing.T) {
	input := "x = 5"

	expected := []TokenType{IDENT, ASSIGN, INT_LIT, EOF}

	l := New(input)
	tokens := l.Tokenize()

	if len(tokens) != len(expected) {
		t.Fatalf("wrong number of tokens. expected=%d, got=%d",
			len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
				i, exp, tokens[i].Type)
		}
	}
}

func TestTokenType_String(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		expected  string
	}{
		{ILLEGAL, "ILLEGAL"},
		{EOF, "EOF"},
		{IDENT, "IDENT"},
		{INT_LIT, "INT_LIT"},
		{FLOAT_LIT, "FLOAT_LIT"},
		{STRING_LIT, "STRING_LIT"},
		{MODULE, "MODULE"},
		{FUNCTION, "FUNCTION"},
		{PLUS, "PLUS"},
		{LPAREN, "LPAREN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.tokenType.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNextToken_ImportAndPublicKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:     "import keyword",
			input:    "import",
			expected: []TokenType{IMPORT, EOF},
		},
		{
			name:     "public keyword",
			input:    "public",
			expected: []TokenType{PUBLIC, EOF},
		},
		{
			name:     "import statement",
			input:    `import "math.intent";`,
			expected: []TokenType{IMPORT, STRING_LIT, SEMICOLON, EOF},
		},
		{
			name:     "public function",
			input:    "public function",
			expected: []TokenType{PUBLIC, FUNCTION, EOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, expectedType := range tt.expected {
				tok := l.NextToken()
				if tok.Type != expectedType {
					t.Errorf("token[%d] - wrong type. expected=%q, got=%q",
						i, expectedType, tok.Type)
				}
			}
		})
	}
}
