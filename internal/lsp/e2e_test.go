package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 18 task 18.9: end-to-end smoke test. One scripted LSP session
// drives the server through every v1 surface — initialize, didOpen
// (with a broken file), didChange (to a clean file), hover, definition,
// shutdown, exit — and asserts the wire-level responses are sane.
// Locks down the v1 protocol contract so 18.10 docs can reference it.

func TestE2ELspSession(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	// --- initialize ---
	mustSend(t, client, 1, "initialize", `{"rootUri":"file:///tmp"}`)
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if !initResult.Capabilities.HoverProvider || !initResult.Capabilities.DefinitionProvider {
		t.Fatalf("missing v1 capabilities: %+v", initResult.Capabilities)
	}

	mustSendNotification(t, client, "initialized", `{}`)

	// --- didOpen: broken file ---
	uri := DocumentURI("file:///tmp/e2e.intent")
	brokenText := "module e2e version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int\n" +
		"    requires x >= 0\n" +
		"    ensures result >= x\n" +
		"{\n" +
		"    return x + y;\n" +
		"}\n" +
		"entry function main() returns Int {\n" +
		"    let x: Int = \"oops\";\n" + // type mismatch — checker error
		"    return add(x, 2);\n" +
		"}\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: brokenText},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))

	notif := mustReadPublishDiagnostics(t, client)
	if notif.URI != uri {
		t.Errorf("publish URI: got %q want %q", notif.URI, uri)
	}
	if !hasErrorDiagnostic(notif.Diagnostics) {
		t.Errorf("expected at least one Error diagnostic on broken open, got %+v", notif.Diagnostics)
	}

	// --- didChange: fix the type error ---
	cleanText := strings.Replace(brokenText, `let x: Int = "oops";`, `let x: Int = 1;`, 1)
	changeP, _ := json.Marshal(DidChangeTextDocumentParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{{Text: cleanText}},
	})
	mustSendNotification(t, client, "textDocument/didChange", string(changeP))

	notif = mustReadPublishDiagnostics(t, client)
	if hasErrorDiagnostic(notif.Diagnostics) {
		t.Errorf("expected no Error diagnostics after fix, got %+v", notif.Diagnostics)
	}

	// --- hover on 'add' inside main's call site ---
	// Locate 'add(' in cleanText by line.
	addLine, addCol := findInText(cleanText, "add(")
	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addLine, addCol+1), // middle of 'add'
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("hover: %v", err)
	}
	var hov Hover
	if err := json.Unmarshal(resp.Result, &hov); err != nil {
		t.Fatalf("decode hover: %v (raw %s)", err, resp.Result)
	}
	if !strings.Contains(hov.Contents.Value, "function add(x: Int, y: Int) returns Int") {
		t.Errorf("hover missing signature, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "requires x >= 0") {
		t.Errorf("hover missing requires, got:\n%s", hov.Contents.Value)
	}

	// --- definition on the same 'add' ---
	defP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addLine, addCol+1),
	})
	mustSend(t, client, 3, "textDocument/definition", string(defP))
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err != nil {
		t.Fatalf("decode definition: %v (raw %s)", err, resp.Result)
	}
	if loc.URI != uri {
		t.Errorf("definition URI: got %q want %q", loc.URI, uri)
	}
	// 'add' is declared on line 2 of cleanText (1-indexed) → LSP line 1.
	if loc.Range.Start.Line != 1 {
		t.Errorf("definition start line: got %d want 1", loc.Range.Start.Line)
	}

	// --- shutdown / exit ---
	mustSend(t, client, 4, "shutdown", `null`)
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("shutdown error: %+v", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Errorf("shutdown result: got %s want null", resp.Result)
	}
	mustSendNotification(t, client, "exit", `null`)
}

func hasErrorDiagnostic(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// findInText returns 1-indexed (line, col) of the first occurrence of needle
// in haystack. Used by the E2E test to locate identifiers without
// hardcoding magic offsets. Returns (0, 0) on no match — caller can check.
func findInText(haystack, needle string) (line, col int) {
	idx := strings.Index(haystack, needle)
	if idx < 0 {
		return 0, 0
	}
	line = 1
	lineStart := 0
	for i := 0; i < idx; i++ {
		if haystack[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	col = idx - lineStart + 1
	return line, col
}
