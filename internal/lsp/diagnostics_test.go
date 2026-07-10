package lsp

import (
	"encoding/json"
	"testing"
	"time"
)

// Phase 18 task 18.3: parser + checker + lint diagnostics.

func TestAnalyzeCleanFileNoErrors(t *testing.T) {
	// A file with a missing-contract lint warning is still considered
	// "clean" at error level. The linter Warning surfaces but no Error
	// should appear.
	src := `module a version "1.0";
entry function main() returns Int { return 0; }
`
	diags := analyzeText(src)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("clean file should produce no error-severity diagnostics, got: %+v", d)
		}
	}
}

func TestAnalyzeParserErrorPublishedAsError(t *testing.T) {
	src := `module a version "1.0";
entry function main() returns Int { return  ;`
	diags := analyzeText(src)
	if len(diags) == 0 {
		t.Fatal("expected parser diagnostic")
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("expected Error severity, got %d", diags[0].Severity)
	}
	if diags[0].Source != "parser" {
		t.Errorf("expected source=parser, got %q", diags[0].Source)
	}
}

func TestAnalyzeCheckerErrorAfterCleanParse(t *testing.T) {
	// Type error: assigning a String to an Int-typed binding.
	src := `module a version "1.0";
entry function main() returns Int {
    let x: Int = "hi";
    return x;
}
`
	diags := analyzeText(src)
	if len(diags) == 0 {
		t.Fatal("expected checker diagnostic")
	}
	foundChecker := false
	for _, d := range diags {
		if d.Source == "checker" && d.Severity == SeverityError {
			foundChecker = true
		}
	}
	if !foundChecker {
		t.Errorf("expected checker Error diagnostic; got %+v", diags)
	}
}

func TestServerPublishesOnDidOpenAndChange(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	// Initialize.
	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	// didOpen a broken file (type error in a let binding).
	brokenText := "module a version \"1.0\";\n" +
		"entry function main() returns Int {\n" +
		"    let x: Int = \"hi\";\n" +
		"    return x;\n" +
		"}\n"
	openParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        DocumentURI("file:///tmp/a.intent"),
			LanguageID: "intent",
			Version:    1,
			Text:       brokenText,
		},
	}
	b, _ := json.Marshal(openParams)
	mustSendNotification(t, client, "textDocument/didOpen", string(b))

	notif := mustReadPublishDiagnostics(t, client)
	foundError := false
	for _, d := range notif.Diagnostics {
		if d.Severity == SeverityError {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected at least one Error diagnostic on broken file open, got %+v", notif.Diagnostics)
	}

	// didChange to a clean file. The clean file may still carry lint
	// warnings (e.g. missing contracts) — assert there are no Error-level
	// diagnostics; that's the "fix" the user just made.
	cleanText := "module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	changeParams := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     openParams.TextDocument.URI,
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{{Text: cleanText}},
	}
	b, _ = json.Marshal(changeParams)
	mustSendNotification(t, client, "textDocument/didChange", string(b))

	notif = mustReadPublishDiagnostics(t, client)
	for _, d := range notif.Diagnostics {
		if d.Severity == SeverityError {
			t.Errorf("expected no Error diagnostics after fix, got %+v", d)
		}
	}
}

func TestServerClearsDiagnosticsOnClose(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	uri := DocumentURI("file:///tmp/c.intent")
	openParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: "module c version \"1.0\";\nentry function main() returns Int { return 0; }\n"},
	}
	b, _ := json.Marshal(openParams)
	mustSendNotification(t, client, "textDocument/didOpen", string(b))
	mustReadPublishDiagnostics(t, client) // drain the open notification

	closeParams := DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}}
	b, _ = json.Marshal(closeParams)
	mustSendNotification(t, client, "textDocument/didClose", string(b))

	notif := mustReadPublishDiagnostics(t, client)
	if len(notif.Diagnostics) != 0 {
		t.Errorf("close should publish empty diagnostics, got %+v", notif.Diagnostics)
	}
	if notif.URI != uri {
		t.Errorf("close diagnostics URI: got %q want %q", notif.URI, uri)
	}
}

func TestDebouncerCollapsesRapidEdits(t *testing.T) {
	// With the default (non-zero) debounce, multiple back-to-back triggers
	// should only invoke the function once, after the delay.
	d := newDebouncer(50e6) // 50ms
	called := make(chan struct{}, 8)
	uri := DocumentURI("file:///x")
	for i := 0; i < 5; i++ {
		d.trigger(uri, func() { called <- struct{}{} })
	}
	// Wait long enough for the debounce.
	select {
	case <-called:
	case <-timeAfter(500):
		t.Fatal("debounced callback never fired")
	}
	// No subsequent fires from the cancelled timers.
	select {
	case <-called:
		t.Error("debounced callback fired more than once for collapsed edits")
	case <-timeAfter(100):
		// expected
	}
}

func timeAfter(ms int) <-chan time.Time {
	return time.After(time.Duration(ms) * time.Millisecond)
}

func mustReadPublishDiagnostics(t *testing.T, c *transport) PublishDiagnosticsParams {
	t.Helper()
	msg, err := c.readMessage()
	if err != nil {
		t.Fatalf("read publishDiagnostics: %v", err)
	}
	if msg.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got method %q", msg.Method)
	}
	var p PublishDiagnosticsParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		t.Fatalf("decode publishDiagnostics params: %v", err)
	}
	return p
}

// newTestServerWithHandle is like newTestServer but also returns the
// Server reference so tests can call SetDiagnosticsDebounce.
func newTestServerWithHandle(t *testing.T) (client *transport, close func(), srv *Server) {
	t.Helper()
	clientToServer := newPipe()
	serverToClient := newPipe()

	srv = NewServer(clientToServer.readEnd(), serverToClient.writeEnd(), "dev")
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Run()
	}()

	client = newTransport(serverToClient.readEnd(), clientToServer.writeEnd())
	close = func() {
		clientToServer.closeWrite()
		<-srvErr
	}
	return client, close, srv
}

func TestSeverityMapping(t *testing.T) {
	if severityFor(0) != SeverityError {
		t.Error("Error severity should map to LSP Error (1)")
	}
	if severityFor(1) != SeverityWarning {
		t.Error("Warning severity should map to LSP Warning (2)")
	}
	if severityFor(2) != SeverityInformation {
		t.Error("Info severity should map to LSP Information (3)")
	}
}
