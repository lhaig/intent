package lsp

import (
	"encoding/json"
	"testing"
)

// Phase 19 task 19.2: go-to-definition for locals/params/methods/fields.

func runDefinitionAt(t *testing.T, src string, line, col int) (Location, string) {
	t.Helper()
	client, closeFn, srv := newTestServerWithHandle(t)
	t.Cleanup(closeFn)
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/def.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	defP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line - 1, Character: col - 1},
	})
	mustSend(t, client, 2, "textDocument/definition", string(defP))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if string(resp.Result) == "null" {
		return Location{}, "null"
	}
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	return loc, ""
}

func TestDefinitionLocalLet(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"function f() returns Int {\n" +
		"    let xx: Int = 42;\n" +
		"    return xx;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'xx' on line 4 col 12 (the reference).
	loc, raw := runDefinitionAt(t, src, 4, 13)
	if raw == "null" {
		t.Fatal("expected non-null location")
	}
	// Declaration: line 3 of the source (1-indexed) → LSP line 2.
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected declaration on LSP line 2, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionMethodCall(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity E {\n" +
		"    method ping() returns Int { return 1; }\n" +
		"    method ping_caller() returns Int { return self.ping(); }\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'ping' as a method call on line 4 — `return self.ping()`. 'ping'
	// is at col 53.
	loc, raw := runDefinitionAt(t, src, 4, 53)
	if raw == "null" {
		t.Fatal("expected non-null location for method call")
	}
	// Method 'ping' declaration is on line 3 (1-indexed) → LSP line 2.
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected method ping on LSP line 2, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionFieldAccess(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity E {\n" +
		"    field n: Int;\n" +
		"    method get() returns Int { return self.n; }\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	// 'n' after self. on line 4 col 44.
	loc, raw := runDefinitionAt(t, src, 4, 44)
	if raw == "null" {
		t.Fatal("expected non-null location for field")
	}
	// Field 'n' declared on line 3 → LSP line 2.
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected field n on LSP line 2, got %d", loc.Range.Start.Line)
	}
}
