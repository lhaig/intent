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
	items := buildCompletionItems(prog, scope, src, 1, 1, nil)
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
	items := buildCompletionItems(prog, scope, src, 1, 1, nil)
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
	items := buildCompletionItems(prog, scope, src, 4, 5, nil)
	labels := collectLabels(items)
	if labels["xx"] != CompletionVariable {
		t.Errorf("expected xx as Variable, got %v", labels["xx"])
	}
	if labels["yy"] != CompletionVariable {
		t.Errorf("expected yy as Variable, got %v", labels["yy"])
	}
}

// Phase 21: member completion.

func TestMemberCompletionContext(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		line     int
		col      int
		wantRecv string
		wantOK   bool
	}{
		{"just after dot", "account.", 1, 9, "account", true},
		{"partial member", "account.bal", 1, 12, "account", true},
		{"self dot", "self.balance", 1, 13, "self", true},
		{"self bare dot", "self.", 1, 6, "self", true},
		{"whitespace around dot", "account . bal", 1, 14, "account", true},
		{"no dot", "account", 1, 8, "", false},
		{"chained access", "a.b.c", 1, 6, "", false},
		{"call result", "foo().bar", 1, 10, "", false},
		{"empty text", "", 1, 1, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recv, ok := memberCompletionContext(tc.text, tc.line, tc.col)
			if ok != tc.wantOK || recv != tc.wantRecv {
				t.Errorf("memberCompletionContext(%q, %d, %d) = (%q, %v); want (%q, %v)", tc.text, tc.line, tc.col, recv, ok, tc.wantRecv, tc.wantOK)
			}
		})
	}
}

func TestMemberCompletionOnTypedLocal(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field owner: String;\n" +
		"    field balance: Int;\n" +
		"    method deposit(amount: Int) returns Void { self.balance = self.balance + amount; }\n" +
		"}\n" +
		"entry function main() returns Int {\n" +
		"    let acc: Account = Account();\n" +
		"    return 0;\n" +
		"}\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	// Position: line containing `return 0;` is line 9; pretend the user
	// inserted `acc.` and the cursor is right after the dot. Use a fresh
	// text for memberCompletionContext.
	text := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field owner: String;\n" +
		"    field balance: Int;\n" +
		"    method deposit(amount: Int) returns Void { self.balance = self.balance + amount; }\n" +
		"}\n" +
		"entry function main() returns Int {\n" +
		"    let acc: Account = Account();\n" +
		"    acc.\n" +
		"}\n"
	items := buildCompletionItems(prog, scope, text, 9, 9, nil)
	labels := collectLabels(items)
	if labels["owner"] != CompletionField {
		t.Errorf("expected owner as Field, got %v", labels["owner"])
	}
	if labels["balance"] != CompletionField {
		t.Errorf("expected balance as Field, got %v", labels["balance"])
	}
	if labels["deposit"] != CompletionMethod {
		t.Errorf("expected deposit as Method, got %v", labels["deposit"])
	}
	// Keywords / top-level names must NOT be present in member position.
	if _, ok := labels["if"]; ok {
		t.Error("keywords leaked into member completion list")
	}
	if _, ok := labels["main"]; ok {
		t.Error("top-level decls leaked into member completion list")
	}
}

func TestMemberCompletionOnSelfInMethod(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    method peek() returns Int {\n" +
		"        return self.balance;\n" +
		"    }\n" +
		"}\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	text := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    method peek() returns Int {\n" +
		"        self.\n" +
		"    }\n" +
		"}\n"
	items := buildCompletionItems(prog, scope, text, 5, 14, nil)
	labels := collectLabels(items)
	if labels["balance"] != CompletionField {
		t.Errorf("expected balance as Field inside method, got %v", labels["balance"])
	}
	if labels["peek"] != CompletionMethod {
		t.Errorf("expected peek as Method inside method, got %v", labels["peek"])
	}
}

func TestMemberCompletionOnSelfInConstructor(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    constructor(initial: Int) ensures self.balance == initial { self.balance = initial; }\n" +
		"}\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	text := "module a version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    constructor(initial: Int) ensures self.balance == initial { self. }\n" +
		"}\n"
	items := buildCompletionItems(prog, scope, text, 4, 70, nil)
	labels := collectLabels(items)
	if labels["balance"] != CompletionField {
		t.Errorf("expected balance as Field inside constructor, got %v", labels["balance"])
	}
}

func TestMemberCompletionUnresolvableReturnsEmpty(t *testing.T) {
	src := "module a version \"1.0\";\n" +
		"entry function main() returns Int {\n" +
		"    let n: Int = 0;\n" +
		"    return n;\n" +
		"}\n"
	prog := parser.New(src).Parse()
	scope := newScopeResolver(prog, "")
	text := "module a version \"1.0\";\n" +
		"entry function main() returns Int {\n" +
		"    let n: Int = 0;\n" +
		"    n.\n" +
		"}\n"
	items := buildCompletionItems(prog, scope, text, 4, 7, nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items for receiver of non-entity type, got %d (%v)", len(items), collectLabels(items))
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
