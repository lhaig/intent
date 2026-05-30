package lsp

import (
	"sync"
	"time"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/diagnostic"
	"github.com/lhaig/intent/internal/linter"
	"github.com/lhaig/intent/internal/parser"
)

// diagnosticsDebounce is how long the server waits after the most recent
// edit before re-running parse/check/lint and publishing. ADR 0032 §O6
// names ~150ms as the v1 target. Tests override this to 0 via
// Server.SetDiagnosticsDebounce.
const diagnosticsDebounce = 150 * time.Millisecond

// debouncer manages per-document timers so rapid edits collapse into a
// single analysis run. Each Reset stops any existing timer for the URI and
// schedules a fresh one.
type debouncer struct {
	mu     sync.Mutex
	timers map[DocumentURI]*time.Timer
	delay  time.Duration
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		timers: map[DocumentURI]*time.Timer{},
		delay:  delay,
	}
}

func (d *debouncer) trigger(uri DocumentURI, fn func()) {
	d.mu.Lock()
	if t, ok := d.timers[uri]; ok && t != nil {
		t.Stop()
	}
	if d.delay <= 0 {
		// Synchronous path used in tests. Run inline so the next assertion
		// sees the published diagnostics deterministically. Drop the lock
		// before invoking fn so it can re-enter trigger if it wants to.
		delete(d.timers, uri)
		d.mu.Unlock()
		fn()
		return
	}
	d.timers[uri] = time.AfterFunc(d.delay, fn)
	d.mu.Unlock()
}

func (d *debouncer) cancel(uri DocumentURI) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[uri]; ok && t != nil {
		t.Stop()
		delete(d.timers, uri)
	}
}

// analyzeAndPublish parses, type-checks, and lints the document's current
// text, then publishes the union of diagnostics over LSP. Always publishes
// (including the empty set) so the editor clears stale markers when a file
// becomes clean.
func (s *Server) analyzeAndPublish(uri DocumentURI) {
	doc, ok := s.docs.get(uri)
	if !ok {
		return
	}
	text := doc.snapshotText()

	lspDiags := analyzeText(text)
	_ = s.t.writeNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
}

// analyzeText is the pure analysis path: parse → check → lint. Each phase
// only runs if the previous phase didn't error out the way that would
// invalidate it (parser errors block checker; checker errors don't block
// lint — lint walks the AST regardless). Returns LSP diagnostics in source
// order.
func analyzeText(source string) []Diagnostic {
	var out []Diagnostic

	p := parser.New(source)
	prog := p.Parse()
	out = append(out, convertDiagnostics(p.Diagnostics().All(), "parser")...)

	if p.Diagnostics().HasErrors() {
		// Parser errors mean the AST is unreliable; don't run downstream
		// passes that might panic on it.
		return out
	}

	checkRes := checker.CheckWithResult(prog)
	out = append(out, convertDiagnostics(checkRes.Diagnostics.All(), "checker")...)

	lintDiags := linter.Lint(prog)
	out = append(out, convertDiagnostics(lintDiags.All(), "linter")...)

	return out
}

// convertDiagnostics maps compiler-side diagnostics to LSP form. Severity
// mapping per ADR 0032 §O2 → C: parser/checker = Error, lint = Warning.
// Source distinguishes "parser" vs "checker" vs "linter" for the client.
func convertDiagnostics(items []diagnostic.Diagnostic, source string) []Diagnostic {
	out := make([]Diagnostic, 0, len(items))
	for _, d := range items {
		out = append(out, Diagnostic{
			Range: Range{
				Start: lineColToPosition(d.Line, d.Column),
				End:   lineColToPosition(d.Line, d.Column+1),
			},
			Severity: severityFor(d.Severity),
			Source:   source,
			Message:  d.Message,
		})
	}
	return out
}

func severityFor(s diagnostic.Severity) DiagnosticSeverity {
	switch s {
	case diagnostic.Error:
		return SeverityError
	case diagnostic.Warning:
		return SeverityWarning
	case diagnostic.Info:
		return SeverityInformation
	default:
		return SeverityInformation
	}
}
