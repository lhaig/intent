package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhaig/intent/internal/formatter"
	"github.com/lhaig/intent/internal/parser"
)

// Formatting via LSP. Phase 19 task 19.4.
//
// textDocument/formatting parses the open document, runs the existing
// internal/formatter, and returns one TextEdit that replaces the whole
// document with the canonical form. Edits are skipped (empty slice) when:
//   - the file fails to parse — we don't want to write garbage on top
//   - the formatted output equals the input — clients can short-circuit

// TextEdit is the LSP type returned by formatting. Range + new text.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// DocumentFormattingParams is the request body for textDocument/formatting.
// Options (tab size, insert spaces) are part of the spec but the Intent
// formatter is non-configurable in v1 — we ignore Options here.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

func (s *Server) handleFormatting(id json.RawMessage, params json.RawMessage) {
	var p DocumentFormattingParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode formatting params: %v", err))
		return
	}
	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, []TextEdit{})
		return
	}
	text := doc.snapshotText()
	edits := computeFormattingEdits(text)
	s.t.writeResponse(id, edits)
}

// computeFormattingEdits is the pure formatter-driver. Public to the
// test file (lowercase but same package) — exercised without spinning
// up a server.
func computeFormattingEdits(text string) []TextEdit {
	pp := parser.New(text)
	prog := pp.Parse()
	if pp.Diagnostics().HasErrors() {
		// Don't format unparseable text.
		return []TextEdit{}
	}
	formatted := formatter.Format(prog)
	if formatted == text {
		// No-op edit; let the client skip the round-trip.
		return []TextEdit{}
	}
	return []TextEdit{{
		Range:   Range{Start: Position{Line: 0, Character: 0}, End: endOfText(text)},
		NewText: formatted,
	}}
}

// endOfText returns the Position one past the last character of text —
// the half-open range end LSP expects when replacing the whole document.
func endOfText(text string) Position {
	if text == "" {
		return Position{Line: 0, Character: 0}
	}
	lastNewline := strings.LastIndexByte(text, '\n')
	if lastNewline == len(text)-1 {
		// File ends with a newline. End position is (numLines, 0).
		lines := strings.Count(text, "\n")
		return Position{Line: lines, Character: 0}
	}
	// No trailing newline. End is on the last line, after its last char.
	lines := strings.Count(text, "\n")
	tail := text[lastNewline+1:]
	return Position{Line: lines, Character: len(tail)}
}
