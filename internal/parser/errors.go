package parser

import (
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/lexer"
)

// syncTokens are tokens the parser can synchronize to after an error
var syncTokens = map[lexer.TokenType]bool{
	lexer.FUNCTION:    true,
	lexer.ENTRY:       true,
	lexer.ENTITY:      true,
	lexer.INTENT:      true,
	lexer.LET:         true,
	lexer.RETURN:      true,
	lexer.IF:          true,
	lexer.RBRACE:      true,
	lexer.SEMICOLON:   true,
	lexer.METHOD:      true,
	lexer.CONSTRUCTOR: true,
	lexer.FIELD:       true,
	lexer.INVARIANT:   true,
	lexer.EOF:         true,
}

// Parser holds the parser state
type Parser struct {
	tokens      []lexer.Token
	pos         int
	diags       *diagnostic.Diagnostics
	source      string // raw source for extracting contract text
}

// current returns the current token
func (p *Parser) current() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.EOF}
	}
	return p.tokens[p.pos]
}

// peek returns the next token without consuming
func (p *Parser) peek() lexer.Token {
	if p.pos+1 >= len(p.tokens) {
		return lexer.Token{Type: lexer.EOF}
	}
	return p.tokens[p.pos+1]
}

// advance moves to the next token and returns the consumed token
func (p *Parser) advance() lexer.Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// expect consumes the current token if it matches the expected type,
// otherwise reports an error
func (p *Parser) expect(tt lexer.TokenType) lexer.Token {
	tok := p.current()
	if tok.Type != tt {
		p.diags.Errorf(tok.Line, tok.Column, "expected %s, got %s", tt, tok.Type)
		return tok
	}
	return p.advance()
}

// check returns true if the current token is of the given type
func (p *Parser) check(tt lexer.TokenType) bool {
	return p.current().Type == tt
}

// match consumes the current token if it matches, returns true if consumed
func (p *Parser) match(tt lexer.TokenType) bool {
	if p.check(tt) {
		p.advance()
		return true
	}
	return false
}

// synchronize skips tokens until a sync point is found.
// If skipSemicolon is true, also advance past a semicolon sync point.
func (p *Parser) synchronize() {
	for !p.check(lexer.EOF) {
		if p.current().Type == lexer.SEMICOLON {
			p.advance() // consume the semicolon and continue
			return
		}
		if syncTokens[p.current().Type] {
			return
		}
		p.advance()
	}
}
