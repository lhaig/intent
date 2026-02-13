package lexer

// Lexer scans Intent source code and produces tokens
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int  // current line number
	column       int  // current column number
}

// New creates a new Lexer instance
func New(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

// readChar reads the next character and advances the position
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // ASCII code for NUL
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	l.column++
}

// peekChar returns the next character without advancing the position
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		l.readChar()
	}
}

// skipSingleLineComment skips a single-line comment (//)
func (l *Lexer) skipSingleLineComment() {
	// Skip until end of line or end of file
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

// skipMultiLineComment skips a multi-line comment (/* */)
func (l *Lexer) skipMultiLineComment() {
	// Already read '/*', now skip until '*/'
	for {
		if l.ch == 0 {
			break // End of file
		}
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			break
		}
		l.readChar()
	}
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a numeric literal (integer or float)
func (l *Lexer) readNumber() (string, TokenType) {
	position := l.position
	tokenType := INT_LIT

	// Read integer part
	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for decimal point
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokenType = FLOAT_LIT
		l.readChar() // consume '.'

		// Read fractional part
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return l.input[position:l.position], tokenType
}

// readString reads a string literal
func (l *Lexer) readString() (string, bool) {
	// Already consumed opening quote
	position := l.position
	result := ""

	for {
		l.readChar()
		if l.ch == 0 || l.ch == '\n' {
			// Unterminated string
			return "", false
		}
		if l.ch == '"' {
			break
		}
		if l.ch == '\\' {
			// Handle escape sequences
			l.readChar()
			switch l.ch {
			case 'n':
				result += "\n"
			case 't':
				result += "\t"
			case '\\':
				result += "\\"
			case '"':
				result += "\""
			default:
				// Invalid escape sequence, just include the backslash
				result += "\\" + string(l.ch)
			}
		} else {
			result += string(l.ch)
		}
	}

	// Store the raw literal including quotes for reference
	literal := l.input[position : l.position+1]
	return literal, true
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	// Save position before processing token
	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: EQ, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: ARROW, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: ASSIGN, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NEQ, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: ILLEGAL, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LEQ, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: LT, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GEQ, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: GT, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '+':
		tok = Token{Type: PLUS, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '-':
		tok = Token{Type: MINUS, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '*':
		tok = Token{Type: STAR, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '/':
		if l.peekChar() == '/' {
			l.skipSingleLineComment()
			return l.NextToken() // Recursively get next token
		} else if l.peekChar() == '*' {
			l.readChar() // consume '/'
			l.readChar() // consume '*'
			l.skipMultiLineComment()
			return l.NextToken() // Recursively get next token
		} else {
			tok = Token{Type: SLASH, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '%':
		tok = Token{Type: PERCENT, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '(':
		tok = Token{Type: LPAREN, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case ')':
		tok = Token{Type: RPAREN, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '{':
		tok = Token{Type: LBRACE, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '}':
		tok = Token{Type: RBRACE, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '[':
		tok = Token{Type: LBRACKET, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case ']':
		tok = Token{Type: RBRACKET, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case ',':
		tok = Token{Type: COMMA, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case ':':
		tok = Token{Type: COLON, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case ';':
		tok = Token{Type: SEMICOLON, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '.':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: DOTDOT, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: DOT, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	case '?':
		tok = Token{Type: QUESTION, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
	case '"':
		str, ok := l.readString()
		if !ok {
			tok = Token{Type: ILLEGAL, Literal: "unterminated string", Line: tok.Line, Column: tok.Column}
		} else {
			tok = Token{Type: STRING_LIT, Literal: str, Line: tok.Line, Column: tok.Column}
		}
	case 0:
		tok = Token{Type: EOF, Literal: "", Line: tok.Line, Column: tok.Column}
	default:
		if isLetter(l.ch) {
			ident := l.readIdentifier()
			tokenType := LookupIdent(ident)
			tok = Token{Type: tokenType, Literal: ident, Line: tok.Line, Column: tok.Column}
			return tok // Early return because readIdentifier already advanced
		} else if isDigit(l.ch) {
			literal, tokenType := l.readNumber()
			tok = Token{Type: tokenType, Literal: literal, Line: tok.Line, Column: tok.Column}
			return tok // Early return because readNumber already advanced
		} else {
			tok = Token{Type: ILLEGAL, Literal: string(l.ch), Line: tok.Line, Column: tok.Column}
		}
	}

	l.readChar()
	return tok
}

// Tokenize returns all tokens from the input
func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens
}

// Helper functions

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
