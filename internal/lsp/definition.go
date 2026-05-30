package lsp

import (
	"encoding/json"
	"fmt"

	"github.com/lhaig/intent/internal/parser"
)

// definition handler: returns the declaration Location for the identifier
// at the cursor. ADR 0032 §O4 → B — same-file plus same-package only;
// cross-package and `extern function` jumps to the Rust crate are out of
// scope (extern jumps land on the local extern declaration, which is what
// the user sees when they navigate).

func (s *Server) handleDefinition(id json.RawMessage, params json.RawMessage) {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode definition params: %v", err))
		return
	}

	doc, ok := s.docs.get(p.TextDocument.URI)
	if !ok {
		s.t.writeResponse(id, nil)
		return
	}

	text := doc.snapshotText()
	line, col := doc.positionToLineCol(p.Position)
	name := wordAtPosition(text, line, col)
	if name == "" {
		s.t.writeResponse(id, nil)
		return
	}

	pp := parser.New(text)
	prog := pp.Parse()

	ownPath := uriToPath(p.TextDocument.URI)
	ws := s.workspaces.workspaceForURI(p.TextDocument.URI)
	siblings := ws.siblingModules(ownPath)

	hit := resolveAcrossWorkspace(prog, ownPath, name, siblings)
	if hit == nil {
		s.t.writeResponse(id, nil)
		return
	}

	loc := Location{
		URI: pathToURI(hit.Path),
		Range: Range{
			Start: lineColToPosition(hit.Line, hit.Column),
			End:   lineColToPosition(hit.Line, hit.Column+len(hit.Name)),
		},
	}
	s.t.writeResponse(id, loc)
}
