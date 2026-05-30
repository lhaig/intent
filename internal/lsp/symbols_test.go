package lsp

import (
	"encoding/json"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// Phase 19 task 19.3: document symbols.

func TestDocumentSymbolsTopLevel(t *testing.T) {
	src := `module a version "1.0";
function helper() returns Int { return 1; }
entity Account { field balance: Int; }
enum Color { Red, Blue }
test "smoke" { assert(true); }
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	syms := buildDocumentSymbols(prog)
	names := map[string]SymbolKind{}
	for _, s := range syms {
		names[s.Name] = s.Kind
	}
	want := map[string]SymbolKind{
		"helper":  SymbolFunction,
		"Account": SymbolClass,
		"Color":   SymbolEnum,
		"smoke":   SymbolFunction, // tests
		"main":    SymbolFunction,
	}
	for k, v := range want {
		if got, ok := names[k]; !ok || got != v {
			t.Errorf("symbol %q: got kind %d (present=%v), want %d", k, got, ok, v)
		}
	}
}

func TestDocumentSymbolsEntityChildren(t *testing.T) {
	src := `module a version "1.0";
entity E {
    field x: Int;
    field y: Int;
    constructor(a: Int, b: Int)
        ensures self.x == a
    {
        self.x = a;
        self.y = b;
    }
    method get_sum() returns Int { return self.x + self.y; }
}
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	syms := buildDocumentSymbols(prog)
	var ent *DocumentSymbol
	for i := range syms {
		if syms[i].Name == "E" {
			ent = &syms[i]
			break
		}
	}
	if ent == nil {
		t.Fatal("expected E in top-level symbols")
	}
	childNames := map[string]SymbolKind{}
	for _, c := range ent.Children {
		childNames[c.Name] = c.Kind
	}
	for _, want := range []struct {
		name string
		kind SymbolKind
	}{
		{"x", SymbolField},
		{"y", SymbolField},
		{"constructor", SymbolConstructor},
		{"get_sum", SymbolMethod},
	} {
		if got, ok := childNames[want.name]; !ok || got != want.kind {
			t.Errorf("entity child %q: got %d present=%v want %d", want.name, got, ok, want.kind)
		}
	}
}

func TestDocumentSymbolsEnumVariants(t *testing.T) {
	src := `module a version "1.0";
enum Direction { North, South, East, West }
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	syms := buildDocumentSymbols(prog)
	var en *DocumentSymbol
	for i := range syms {
		if syms[i].Name == "Direction" {
			en = &syms[i]
			break
		}
	}
	if en == nil {
		t.Fatal("expected Direction enum")
	}
	if len(en.Children) != 4 {
		t.Fatalf("expected 4 variants, got %d", len(en.Children))
	}
	for _, c := range en.Children {
		if c.Kind != SymbolEnumMember {
			t.Errorf("variant %q kind: got %d want %d", c.Name, c.Kind, SymbolEnumMember)
		}
	}
}

func TestDocumentSymbolsEmptyProgramSafe(t *testing.T) {
	syms := buildDocumentSymbols(nil)
	if syms == nil || len(syms) != 0 {
		t.Errorf("nil prog should return empty slice, got %+v", syms)
	}
}

func TestDocumentSymbolHandlerEndToEnd(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	uri := DocumentURI("file:///tmp/ds.intent")
	src := "module a version \"1.0\";\nfunction f() returns Int { return 0; }\nentry function main() returns Int { return 0; }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	dsP, _ := json.Marshal(DocumentSymbolParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	mustSend(t, client, 2, "textDocument/documentSymbol", string(dsP))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatal(err)
	}
	var syms []DocumentSymbol
	if err := json.Unmarshal(resp.Result, &syms); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	if len(syms) < 2 {
		t.Errorf("expected at least 2 symbols (f, main), got %d", len(syms))
	}
}
