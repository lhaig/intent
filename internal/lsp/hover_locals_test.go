package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 19 task 19.2: hover for locals/params/methods/fields/self.

func runHoverAt(t *testing.T, src string, line, col int) (Hover, string) {
	t.Helper()
	client, closeFn, srv := newTestServerWithHandle(t)
	t.Cleanup(closeFn)
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/hov.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client) // drain

	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line - 1, Character: col - 1},
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("hover: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("hover rpc error: %+v", resp.Error)
	}
	if string(resp.Result) == "null" {
		return Hover{}, "null"
	}
	var h Hover
	if err := json.Unmarshal(resp.Result, &h); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	return h, ""
}

func TestHoverLocalLetShowsTypedDeclaration(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"function f() returns Int {\n" +
		"    let xx: Int = 42;\n" +
		"    return xx;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'xx' on the return line — line 4, 'xx' at column 12.
	h, raw := runHoverAt(t, src, 4, 13)
	if raw == "null" {
		t.Fatal("expected non-null hover on local let")
	}
	if !strings.Contains(h.Contents.Value, "let xx: Int") {
		t.Errorf("expected 'let xx: Int' in hover, got:\n%s", h.Contents.Value)
	}
}

func TestHoverFunctionParam(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"function f(yy: String) returns Int {\n" +
		"    return 0;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'yy' in 'f(yy: String)' on line 2 — but we test in the body. Add a
	// reference of yy in the body for cleaner positioning.
	src = "module a version \"1.0\";\n" +
		"function f(yy: String) returns Int {\n" +
		"    let n: Int = 0;\n" +
		"    return n + len(yy);\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'yy' on return line — `    return n + len(yy);`, 'yy' at columns 20-21.
	h, raw := runHoverAt(t, src, 4, 20)
	if raw == "null" {
		t.Fatal("expected non-null hover on param")
	}
	if !strings.Contains(h.Contents.Value, "yy: String") {
		t.Errorf("expected 'yy: String' in hover, got:\n%s", h.Contents.Value)
	}
}

func TestHoverSelfInsideMethod(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity E {\n" +
		"    field n: Int;\n" +
		"    method get() returns Int { return self.n; }\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'self' on line 4 col 39.
	h, raw := runHoverAt(t, src, 4, 39)
	if raw == "null" {
		t.Fatal("expected non-null hover on self")
	}
	if !strings.Contains(h.Contents.Value, "self: E") {
		t.Errorf("expected 'self: E' in hover, got:\n%s", h.Contents.Value)
	}
}

func TestHoverFieldAccess(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity E {\n" +
		"    field n: Int;\n" +
		"    method get() returns Int { return self.n; }\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'n' after 'self.' on line 4, col 44.
	h, raw := runHoverAt(t, src, 4, 44)
	if raw == "null" {
		t.Fatal("expected non-null hover on field access")
	}
	if !strings.Contains(h.Contents.Value, "field n: Int") {
		t.Errorf("expected 'field n: Int' in hover, got:\n%s", h.Contents.Value)
	}
}
