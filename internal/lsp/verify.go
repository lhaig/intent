package lsp

import (
	"fmt"
	"os/exec"

	"github.com/lhaig/intent/internal/checker"
	"github.com/lhaig/intent/internal/ir"
	"github.com/lhaig/intent/internal/parser"
	"github.com/lhaig/intent/internal/verify"
)

// Verification diagnostics from Z3, surfaced asynchronously on save.
// ADR 0032 §18.4 contract:
//
//   - Z3 unavailable on PATH → no diagnostic, silent degradation.
//   - Verified contracts → no diagnostic.
//   - Unverified → Information severity at (1,1) with the qualified
//     contract name in the message. The PRD notes that real per-contract
//     line positions are a v1.1 follow-up requiring source positions to
//     thread through the verify package.
//   - Timeout / error → Hint severity.
//   - Sequence-based cancellation: each didSave bumps doc.saveSeq; if a
//     newer save bumps it before this goroutine finishes, results are
//     dropped on publish.
func (s *Server) runVerifyAsync(uri DocumentURI) {
	doc, ok := s.docs.get(uri)
	if !ok {
		return
	}
	if _, err := exec.LookPath("z3"); err != nil {
		// Z3 absent — clear any stale verify diagnostics (e.g. if Z3 was
		// uninstalled mid-session) and stop.
		union := doc.setVerifyDiags(nil)
		_ = s.t.writeNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: union,
		})
		return
	}

	seq := doc.bumpSaveSeq()
	text := doc.snapshotText()

	go func() {
		diags := runVerifyOnText(text)

		// Drop results if a newer save bumped saveSeq while we were
		// running. The newer save's goroutine will publish its own
		// results.
		if doc.currentSaveSeq() != seq {
			return
		}

		union := doc.setVerifyDiags(diags)
		_ = s.t.writeNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: union,
		})
	}()
}

// runVerifyOnText runs the full pipeline up to verify and converts the
// VerifyResults to LSP diagnostics. Parse / check errors abort the
// pipeline silently — the user is already seeing those diagnostics from
// the synchronous analyze path; double-publishing them under
// Source="verify" would be noise.
func runVerifyOnText(source string) []Diagnostic {
	p := parser.New(source)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		return nil
	}
	checkRes := checker.CheckWithResult(prog)
	if checkRes.Diagnostics.HasErrors() {
		return nil
	}
	mod := ir.Lower(prog, checkRes)
	results := verify.Verify(mod)
	return verifyResultsToDiagnostics(results)
}

func verifyResultsToDiagnostics(results []*verify.VerifyResult) []Diagnostic {
	var out []Diagnostic
	for _, r := range results {
		if r.Status == "verified" {
			continue
		}
		// "error" status from the verifier with Message="z3 not found"
		// is filtered out by the caller's LookPath check; other "error"
		// values (translation errors, etc.) become Hint diagnostics.
		var sev DiagnosticSeverity
		switch r.Status {
		case "unverified":
			sev = SeverityInformation
		case "timeout", "error":
			sev = SeverityHint
		default:
			sev = SeverityInformation
		}
		// ADR 0034: anchor the diagnostic on the failing clause. The
		// parser records 1-indexed positions; LSP is 0-indexed.
		// Toolchain-error rows (no clause origin) leave Line=0 and fall
		// back to the file-start anchor.
		var rng Range
		if r.Line > 0 {
			rng = Range{
				Start: Position{Line: r.Line - 1, Character: r.Column - 1},
				End:   Position{Line: r.Line - 1, Character: r.Column},
			}
		} else {
			rng = Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			}
		}
		out = append(out, Diagnostic{
			Range:    rng,
			Severity: sev,
			Source:   "verify",
			Message:  fmt.Sprintf("%s: %s — %s", r.Status, r.QualifiedName(), r.Message),
		})
	}
	return out
}
