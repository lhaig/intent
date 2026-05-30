package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Phase 18 task 18.6: textDocument/definition.

func TestDefinitionSameFile(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	// 'add' is declared on line 2 (1-indexed); the call site is on line 4.
	src := "module a version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int { return x + y; }\n" +
		"\n" +
		"entry function main() returns Int { return add(1, 2); }\n"
	uri := DocumentURI("file:///tmp/d.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client) // drain

	// Cursor on the 'add' at the call site (line index 3, char index ~44).
	defParams, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 44},
	})
	mustSend(t, client, 2, "textDocument/definition", string(defParams))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("read definition response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("definition error: %+v", resp.Error)
	}
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err != nil {
		t.Fatalf("decode location: %v (raw %s)", err, resp.Result)
	}
	if loc.URI != uri {
		t.Errorf("expected URI %q, got %q", uri, loc.URI)
	}
	// Declaration starts on the 'function' keyword of line 2 (LSP line 1).
	if loc.Range.Start.Line != 1 {
		t.Errorf("expected declaration line 1 (0-indexed), got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionUnknownReturnsNull(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	src := "module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	uri := DocumentURI("file:///tmp/u.intent")
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on the digit '0' — not an identifier.
	defParams, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 41},
	})
	mustSend(t, client, 2, "textDocument/definition", string(defParams))
	resp, _ := client.readMessage()
	if string(resp.Result) != "null" {
		t.Errorf("expected null Location for non-identifier, got %s", resp.Result)
	}
}

func TestDefinitionCrossFileSamePackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "intent.toml"), []byte("[package]\nname=\"d\"\nversion=\"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	libPath := filepath.Join(dir, "lib.intent")
	mainPath := filepath.Join(dir, "main.intent")
	if err := os.WriteFile(libPath, []byte("module lib version \"1.0\";\npublic function helper() returns Int { return 1; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte("module main version \"1.0\";\nimport \"lib.intent\";\nentry function main() returns Int { return helper(); }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	mainSrc, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(mainPath)
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: string(mainSrc)},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// 'helper' in main.intent: line index 2 (0-indexed) of main.intent.
	// Find an offset inside 'helper' — the substring starts after
	// "return ".
	pos := Position{Line: 2, Character: 47}
	defParams, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	})
	mustSend(t, client, 2, "textDocument/definition", string(defParams))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("read definition response: %v", err)
	}
	if resp.Error != nil || string(resp.Result) == "null" {
		t.Fatalf("expected cross-file location, got error=%+v result=%s", resp.Error, resp.Result)
	}
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if loc.URI != pathToURI(libPath) {
		t.Errorf("expected URI to point at lib.intent, got %q", loc.URI)
	}
}
