package lsp

import (
	"net/url"
	"strings"
	"sync"
)

// Document holds the cached state for an open .intent file. Per ADR 0032 §O6
// (Full re-check on didChange), the AST/checker fields are populated on each
// change rather than incrementally. Per-document mutex protects the cache
// (the analysis run is fast — tens of milliseconds typical — so we hold the
// lock for the duration of a re-check).
type Document struct {
	URI     DocumentURI
	Text    string
	Version int

	// lineOffsets is the byte offset of the start of each line. Populated
	// lazily by buildLineOffsets when position conversion needs it.
	lineOffsets []int

	// Diagnostic sources are tracked separately so the verify goroutine
	// (18.4) and the synchronous parse/check/lint pass (18.3) can publish
	// independently. publishUnion sends the merged list. Per-URI publish
	// in LSP replaces all diagnostics for the document, so we must merge
	// before sending.
	analyzeDiags []Diagnostic
	verifyDiags  []Diagnostic

	// saveSeq is bumped on each didSave. The async verifier captures it
	// before launch and discards results if a newer save has bumped it
	// before they arrive — ADR 0032 §18.4's cancellation contract.
	saveSeq uint64

	mu sync.Mutex
}

// documentStore is the server's open-document map. It is the canonical view
// of what the client has open; handlers consult it instead of re-reading
// from disk.
type documentStore struct {
	mu   sync.Mutex
	docs map[DocumentURI]*Document
}

func newDocumentStore() *documentStore {
	return &documentStore{docs: map[DocumentURI]*Document{}}
}

func (s *documentStore) open(uri DocumentURI, version int, text string) *Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := &Document{
		URI:     uri,
		Text:    text,
		Version: version,
	}
	s.docs[uri] = d
	return d
}

func (s *documentStore) update(uri DocumentURI, version int, text string) *Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[uri]
	if !ok {
		// Spec violation by the client (change before open), but recover.
		d = &Document{URI: uri}
		s.docs[uri] = d
	}
	d.mu.Lock()
	d.Text = text
	d.Version = version
	d.lineOffsets = nil // invalidate
	d.mu.Unlock()
	return d
}

func (s *documentStore) close(uri DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *documentStore) get(uri DocumentURI) (*Document, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[uri]
	return d, ok
}

// snapshotText returns the current text under a brief lock — useful when a
// caller wants a consistent view without holding the doc lock across heavy
// work.
func (d *Document) snapshotText() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Text
}

// setAnalyzeDiags stores the latest parser/checker/lint diagnostics. Called
// after each debounced re-analysis. Returns the union of analyze + verify
// diagnostics so the caller can publish in one notification.
func (d *Document) setAnalyzeDiags(diags []Diagnostic) []Diagnostic {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.analyzeDiags = diags
	return appendCopy(d.analyzeDiags, d.verifyDiags)
}

// setVerifyDiags stores the latest Z3 verification diagnostics. Returns the
// union for publication.
func (d *Document) setVerifyDiags(diags []Diagnostic) []Diagnostic {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.verifyDiags = diags
	return appendCopy(d.analyzeDiags, d.verifyDiags)
}

// bumpSaveSeq increments and returns the document's save sequence. The
// async verifier captures the returned value before launching and only
// publishes if d.saveSeq matches when results arrive.
func (d *Document) bumpSaveSeq() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.saveSeq++
	return d.saveSeq
}

// currentSaveSeq returns d.saveSeq without modifying it.
func (d *Document) currentSaveSeq() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.saveSeq
}

func appendCopy(a, b []Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// buildLineOffsets computes byte offsets of line starts in the document
// text. Cached lazily and invalidated on every update. Position helpers
// use it for fast (line, col) ↔ byte conversion.
func (d *Document) buildLineOffsets() []int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lineOffsets != nil {
		return d.lineOffsets
	}
	offsets := []int{0}
	for i := 0; i < len(d.Text); i++ {
		if d.Text[i] == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	d.lineOffsets = offsets
	return offsets
}

// positionToLineCol converts an LSP Position (0-indexed line + UTF-16 char)
// to a 1-indexed (line, col) tuple matching the rest of the compiler
// pipeline. ASCII-only assumption is documented in the PRD; multi-byte
// support is deferred. Returns (1, 1) for an out-of-range position so
// callers can't index OOB on the cached AST.
func (d *Document) positionToLineCol(p Position) (line, col int) {
	if p.Line < 0 || p.Character < 0 {
		return 1, 1
	}
	return p.Line + 1, p.Character + 1
}

// lineColToPosition is the inverse: 1-indexed (line, col) → 0-indexed
// Position. Used when translating compiler diagnostics into LSP ranges.
func lineColToPosition(line, col int) Position {
	if line < 1 {
		line = 1
	}
	if col < 1 {
		col = 1
	}
	return Position{Line: line - 1, Character: col - 1}
}

// uriToPath converts a file:// URI to an absolute filesystem path. Returns
// the empty string for non-file URIs (which v1 doesn't try to handle).
func uriToPath(uri DocumentURI) string {
	s := string(uri)
	if !strings.HasPrefix(s, "file://") {
		return ""
	}
	rest := strings.TrimPrefix(s, "file://")
	// Some clients send "file:///path" (extra slash for the empty authority);
	// the leading slash is the path itself, so we keep it.
	decoded, err := url.PathUnescape(rest)
	if err != nil {
		return rest
	}
	return decoded
}

// pathToURI is the inverse of uriToPath for go-to-definition results.
func pathToURI(path string) DocumentURI {
	// Encode any path characters that aren't safe in a URI. url.PathEscape
	// is too aggressive (it escapes /); building the URI manually mirrors
	// what the standard library does internally.
	u := url.URL{Scheme: "file", Path: path}
	return DocumentURI(u.String())
}
