package lsp

import (
	"encoding/json"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// Phase 19 task 19.6: completion.

func collectLabels(items []CompletionItem) map[string]CompletionItemKind {
	out := map[string]CompletionItemKind{}
	for _, it := range items {
		out[it.Label] = it.Kind
	}
	return out
}

func TestCompletionTopLevelDeclsPresent(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"function helper() returns Int { return 0; }\n" +
		"entity Account { field balance: Int; }\n" +
		"entry function main() returns Int { return 0; }\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	items := buildCompletionItems(prog, scope, 1, 1, nil)
	labels := collectLabels(items)
	if labels["helper"] != CompletionFunction {
		t.Errorf("expected helper function in completions, got %v", labels["helper"])
	}
	if labels["Account"] != CompletionClass {
		t.Errorf("expected Account class in completions, got %v", labels["Account"])
	}
	if labels["main"] != CompletionFunction {
		t.Errorf("expected main function in completions, got %v", labels["main"])
	}
}

func TestCompletionKeywordsAndTypes(t *testing.T) {
	src := "module a version \"1.0\";\nentry function main() returns Int { return 0; }\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	items := buildCompletionItems(prog, scope, 1, 1, nil)
	labels := collectLabels(items)
	for _, kw := range []string{"if", "while", "let", "return", "match"} {
		if labels[kw] != CompletionKeyword {
			t.Errorf("expected %q as Keyword, got %v", kw, labels[kw])
		}
	}
	for _, ty := range []string{"Int", "Float", "String", "Bool", "Result"} {
		if labels[ty] != CompletionStruct {
			t.Errorf("expected %q as built-in type, got %v", ty, labels[ty])
		}
	}
}

func TestCompletionLocalsAndParams(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"function f(yy: Int) returns Int {\n" +
		"    let xx: Int = 1;\n" +
		"    return xx + yy;\n" +
		"}\n" +
		"entry function main() returns Int { return 0; }\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	// Cursor on the return line — both xx and yy are in scope.
	items := buildCompletionItems(prog, scope, 4, 5, nil)
	labels := collectLabels(items)
	if labels["xx"] != CompletionVariable {
		t.Errorf("expected xx as Variable, got %v", labels["xx"])
	}
	if labels["yy"] != CompletionVariable {
		t.Errorf("expected yy as Variable, got %v", labels["yy"])
	}
}

func TestCompletionHandlerEndToEnd(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	uri := DocumentURI("file:///tmp/cp.intent")
	src := "module a version \"1.0\";\nfunction helper() returns Int { return 0; }\nentry function main() returns Int { return helper(); }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	cpP, _ := json.Marshal(CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 50},
	})
	mustSend(t, client, 2, "textDocument/completion", string(cpP))
	resp, _ := client.readMessage()
	var items []CompletionItem
	if err := json.Unmarshal(resp.Result, &items); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	labels := collectLabels(items)
	if labels["helper"] == 0 {
		t.Errorf("expected helper in completions, got %v keys", len(labels))
	}
}
