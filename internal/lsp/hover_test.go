package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 18 task 18.5: hover.

func TestWordAtPositionMidIdent(t *testing.T) {
	text := "module a version \"1.0\";\nfunction helper() returns Int { return 0; }\n"
	got := wordAtPosition(text, 2, 12) // 'helper' starts at column 10
	if got != "helper" {
		t.Errorf("wordAtPosition: got %q want helper", got)
	}
}

func TestWordAtPositionAtBoundary(t *testing.T) {
	text := "function helper()"
	// Cursor immediately after the identifier still resolves it.
	got := wordAtPosition(text, 1, 16) // ')' position; previous char is 'r'
	if got != "helper" {
		t.Errorf("wordAtPosition: got %q want helper (boundary)", got)
	}
}

func TestWordAtPositionWhitespaceReturnsEmpty(t *testing.T) {
	// Two consecutive spaces; cursor on the second space has no identifier
	// to either side.
	text := "function  helper()"
	got := wordAtPosition(text, 1, 10) // second space
	if got != "" {
		t.Errorf("wordAtPosition on whitespace: got %q want empty", got)
	}
}

func TestHoverFunctionRendersSignatureAndContracts(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int\n" +
		"    requires x >= 0\n" +
		"    ensures result >= x\n" +
		"{\n" +
		"    return x + y;\n" +
		"}\n" +
		"entry function main() returns Int { return add(1, 2); }\n"
	uri := DocumentURI("file:///tmp/a.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client) // drain

	// Hover on the 'add' identifier inside main's body.
	// Line index (0-based): "entry function main..." is line 7. The 'add'
	// substring starts at character offset ... let's compute:
	// "entry function main() returns Int { return add(1, 2); }"
	//  0         1         2         3         4
	//  0123456789012345678901234567890123456789012345
	//                                              ^ 'add' starts at col 43 (1-indexed) = char 42 (0-indexed)
	pos := Position{Line: 7, Character: 44} // middle of 'add'
	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("hover response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("hover error: %+v", resp.Error)
	}
	var hov Hover
	if err := json.Unmarshal(resp.Result, &hov); err != nil {
		t.Fatalf("decode hover: %v (raw: %s)", err, resp.Result)
	}
	if hov.Contents.Kind != "markdown" {
		t.Errorf("expected markdown content, got %q", hov.Contents.Kind)
	}
	if !strings.Contains(hov.Contents.Value, "function add(x: Int, y: Int) returns Int") {
		t.Errorf("hover should include signature, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "requires x >= 0") {
		t.Errorf("hover should include requires clause, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "ensures result >= x") {
		t.Errorf("hover should include ensures clause, got:\n%s", hov.Contents.Value)
	}
}

func TestHoverEntityRendersFieldsAndInvariants(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    invariant balance >= 0;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	uri := DocumentURI("file:///tmp/b.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Hover on 'Account' in the entity declaration itself (line 2,
	// 0-indexed line 1, 'Account' starts at column 8 0-indexed = char 8).
	pos := Position{Line: 1, Character: 10}
	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, _ := client.readMessage()
	var hov Hover
	if err := json.Unmarshal(resp.Result, &hov); err != nil {
		t.Fatalf("decode hover: %v (raw: %s)", err, resp.Result)
	}
	if !strings.Contains(hov.Contents.Value, "entity Account") {
		t.Errorf("hover should include entity name, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "field balance: Int") {
		t.Errorf("hover should include field, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "invariant balance >= 0") {
		t.Errorf("hover should include invariant, got:\n%s", hov.Contents.Value)
	}
}

func TestHoverUnknownPositionReturnsNull(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	uri := DocumentURI("file:///tmp/n.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Hover on the digit '0' inside 'return 0;' — not an identifier.
	pos := Position{Line: 1, Character: 41}
	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, _ := client.readMessage()
	if string(resp.Result) != "null" {
		t.Errorf("hover on non-identifier should return null, got %s", resp.Result)
	}
}
