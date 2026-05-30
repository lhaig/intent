package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 19 task 19.5: signature help.

func TestFindEnclosingCallBareFunction(t *testing.T) {
	text := "function f() returns Int { return add(1, 2); }\n"
	// cursor right after the comma — line 1, col 41 (0-indexed offset 40)
	callee, active, ok := findEnclosingCall(text, 1, 41)
	if !ok {
		t.Fatal("expected call detection")
	}
	if callee != "add" {
		t.Errorf("callee: got %q want add", callee)
	}
	if active != 1 {
		t.Errorf("active: got %d want 1", active)
	}
}

func TestFindEnclosingCallNoCall(t *testing.T) {
	text := "let x: Int = 1;"
	if _, _, ok := findEnclosingCall(text, 1, 5); ok {
		t.Error("expected no call detection outside any call")
	}
}

func TestFindEnclosingCallMethod(t *testing.T) {
	text := "function f() returns Int { return account.deposit(100); }\n"
	// Cursor inside deposit's arg.
	callee, active, ok := findEnclosingCall(text, 1, 53)
	if !ok {
		t.Fatal("expected call detection")
	}
	if callee != "account.deposit" {
		t.Errorf("callee: got %q want account.deposit", callee)
	}
	if active != 0 {
		t.Errorf("active: got %d want 0", active)
	}
}

func TestSignatureHelpFunctionCall(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int { return x + y; }\n" +
		"entry function main() returns Int { return add(1, 2); }\n"
	uri := DocumentURI("file:///tmp/sig.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor between '1,' and ' 2' — inside add's call. Line 3, around col 50.
	sigP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 49}, // after the comma
	})
	mustSend(t, client, 2, "textDocument/signatureHelp", string(sigP))
	resp, _ := client.readMessage()
	if string(resp.Result) == "null" {
		t.Fatalf("expected signature help, got null")
	}
	var sh SignatureHelp
	if err := json.Unmarshal(resp.Result, &sh); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	if len(sh.Signatures) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(sh.Signatures))
	}
	if !strings.Contains(sh.Signatures[0].Label, "add(x: Int, y: Int) returns Int") {
		t.Errorf("label: got %q", sh.Signatures[0].Label)
	}
	if sh.ActiveParameter != 1 {
		t.Errorf("active param: got %d want 1", sh.ActiveParameter)
	}
}

func TestSignatureHelpOutsideCallReturnsNull(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	uri := DocumentURI("file:///tmp/no.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	sigP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
	})
	mustSend(t, client, 2, "textDocument/signatureHelp", string(sigP))
	resp, _ := client.readMessage()
	if string(resp.Result) != "null" {
		t.Errorf("expected null outside call, got %s", resp.Result)
	}
}
