package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Phase 26 / ADR 0035: textDocument/references — find all usages of the
// symbol at the cursor.

func TestReferencesFunctionCalls(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/r1.intent")
	src := "module r version \"1.0\";\n" +
		"function helper() returns Int { return 42; }\n" +
		"function caller() returns Int { return helper() + helper(); }\n" +
		"entry function main() returns Int { return helper(); }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on `helper` in its declaration on line 1 (0-indexed).
	// Source col 10 (1-indexed) → LSP char 9.
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 10},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	mustSend(t, client, 2, "textDocument/references", string(refP))
	resp, _ := client.readMessage()
	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		t.Fatalf("decode: %v (%s)", err, resp.Result)
	}
	// Expect: declaration (line 1) + 3 call sites (lines 2 twice, 3 once).
	if len(locs) != 4 {
		t.Errorf("expected 4 references with includeDeclaration, got %d: %+v", len(locs), locs)
	}

	// includeDeclaration: false should drop the decl.
	refP2, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 10},
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	mustSend(t, client, 3, "textDocument/references", string(refP2))
	resp, _ = client.readMessage()
	var locs2 []Location
	if err := json.Unmarshal(resp.Result, &locs2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(locs2) != 3 {
		t.Errorf("expected 3 references without includeDeclaration, got %d", len(locs2))
	}
}

func TestReferencesEntityType(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/r2.intent")
	src := "module r version \"1.0\";\n" +
		"entity Account { field balance: Int; }\n" +
		"function takesAccount(a: Account) returns Int { return 0; }\n" +
		"entry function main() returns Int {\n" +
		"    let acc: Account = Account();\n" +
		"    return takesAccount(acc);\n" +
		"}\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on `Account` in the entity declaration (line 1).
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 8},
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	mustSend(t, client, 2, "textDocument/references", string(refP))
	resp, _ := client.readMessage()
	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		t.Fatalf("decode: %v (%s)", err, resp.Result)
	}
	// Expect: 3 references — `a: Account` (line 2), `let acc: Account` (line 4),
	// `Account()` constructor (line 4).
	if len(locs) != 3 {
		t.Errorf("expected 3 entity references, got %d: %+v", len(locs), locs)
	}
}

func TestReferencesLocal(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/r3.intent")
	src := "module r version \"1.0\";\n" +
		"function f() returns Int {\n" +
		"    let xx: Int = 10;\n" +
		"    let yy: Int = xx + xx;\n" +
		"    return xx + yy;\n" +
		"}\n" +
		"entry function main() returns Int { return f(); }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on `xx` at let on line 2 (0-indexed). Source line 3 col 9 → LSP (2, 8).
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 9},
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	mustSend(t, client, 2, "textDocument/references", string(refP))
	resp, _ := client.readMessage()
	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		t.Fatalf("decode: %v (%s)", err, resp.Result)
	}
	// Expect: 3 uses of xx (line 3: xx + xx is two uses; line 4: xx).
	if len(locs) != 3 {
		t.Errorf("expected 3 local references, got %d: %+v", len(locs), locs)
	}
	// Sanity-check that every match is in the body, not in main().
	for _, l := range locs {
		if l.Range.Start.Line < 2 || l.Range.Start.Line > 4 {
			t.Errorf("local reference outside f()'s body: %+v", l)
		}
	}
}

func TestReferencesNonIdentifierReturnsEmpty(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/r4.intent")
	src := "module r version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on whitespace (line 0, column 0).
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	mustSend(t, client, 2, "textDocument/references", string(refP))
	resp, _ := client.readMessage()
	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(locs) != 0 {
		t.Errorf("expected 0 references on whitespace, got %d", len(locs))
	}
}

func TestReferencesCrossPackage(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app_pkg")
	typesDir := filepath.Join(root, "types_pkg")
	for _, d := range []string{appDir, typesDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(appDir, "intent.toml"),
		[]byte("[package]\nname=\"app_pkg\"\nversion=\"0.1.0\"\n\n[dependencies]\ntypes_pkg = { path = \"../types_pkg\" }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(typesDir, "intent.toml"),
		[]byte("[package]\nname=\"types_pkg\"\nversion=\"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	typesSrc := "module types_pkg version \"1.0\";\n" +
		"public entity Point { field x: Float; field y: Float; }\n"
	if err := os.WriteFile(filepath.Join(typesDir, "types.intent"), []byte(typesSrc), 0644); err != nil {
		t.Fatal(err)
	}
	// Two uses of Point in the app: type-position and constructor call.
	mainSrc := "module main version \"1.0\";\n" +
		"import types_pkg;\n" +
		"entry function main() returns Int {\n" +
		"    let p: Point = types_pkg.Point(0.0, 0.0);\n" +
		"    return 0;\n" +
		"}\n"
	mainPath := filepath.Join(appDir, "main.intent")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}

	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(mainPath)
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: mainSrc},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	// Cursor on the `Point` in the let-type on line 3 (0-indexed). Source
	// "    let p: Point = types_pkg.Point(0.0, 0.0);" — `Point` starts at
	// column 12 (1-indexed).
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 12},
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	mustSend(t, client, 2, "textDocument/references", string(refP))
	resp, _ := client.readMessage()
	var locs []Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil {
		t.Fatalf("decode: %v (%s)", err, resp.Result)
	}
	// Expect at least 2 references in app/main.intent: type position +
	// constructor call. (The CallExpr `Point(...)` matches refType too.)
	if len(locs) < 2 {
		t.Errorf("expected at least 2 references in app/main, got %d: %+v", len(locs), locs)
	}
}
