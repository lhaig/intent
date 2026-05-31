package lsp

import (
	"encoding/json"
	"testing"

	"github.com/lhaig/intent/internal/parser"
)

// Phase 20 task 20.2: semantic tokens.

func TestSemanticTokensFunctionDeclaration(t *testing.T) {
	src := `module a version "1.0";
function add(x: Int, y: Int) returns Int { return x + y; }
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	if len(toks) == 0 {
		t.Fatal("expected at least one token")
	}
	// First token should be 'add' as function declaration on line 1 (LSP 0-indexed).
	if toks[0].Type != tokFunction {
		t.Errorf("first token type: got %d want function(%d)", toks[0].Type, tokFunction)
	}
	if toks[0].Modifiers&modDeclaration == 0 {
		t.Errorf("first token should carry declaration modifier, got %b", toks[0].Modifiers)
	}
	if toks[0].Line != 1 {
		t.Errorf("function decl line: got %d want 1", toks[0].Line)
	}
	// Length should match 'add'.
	if toks[0].Length != 3 {
		t.Errorf("function decl length: got %d want 3", toks[0].Length)
	}
}

func TestSemanticTokensBuiltinHasDefaultLibraryModifier(t *testing.T) {
	src := `module a version "1.0";
entry function main() returns Int {
    print("hello");
    return 0;
}
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	var printTok *semanticToken
	for i := range toks {
		if toks[i].Type == tokFunction && toks[i].Modifiers&modDefaultLibrary != 0 {
			printTok = &toks[i]
			break
		}
	}
	if printTok == nil {
		t.Fatal("expected to find print() with defaultLibrary modifier")
	}
}

func TestSemanticTokensEntityAndFields(t *testing.T) {
	src := `module a version "1.0";
entity Account {
    field balance: Int;
    field owner: String;
}
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	var classToken *semanticToken
	var fieldTokens []semanticToken
	for i := range toks {
		switch toks[i].Type {
		case tokClass:
			classToken = &toks[i]
		case tokProperty:
			fieldTokens = append(fieldTokens, toks[i])
		}
	}
	if classToken == nil {
		t.Fatal("expected a class token for Account")
	}
	if classToken.Length != len("Account") {
		t.Errorf("class length: got %d want %d", classToken.Length, len("Account"))
	}
	if len(fieldTokens) != 2 {
		t.Errorf("expected 2 field tokens, got %d", len(fieldTokens))
	}
}

func TestSemanticTokensMethodCall(t *testing.T) {
	src := `module a version "1.0";
entity E {
    method ping() returns Int { return 1; }
    method call_ping() returns Int { return self.ping(); }
}
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	declCount, callCount := 0, 0
	for _, t := range toks {
		if t.Type == tokMethod {
			if t.Modifiers&modDeclaration != 0 {
				declCount++
			} else {
				callCount++
			}
		}
	}
	if declCount != 2 {
		t.Errorf("expected 2 method declarations, got %d", declCount)
	}
	if callCount < 1 {
		t.Errorf("expected at least 1 method call token, got %d", callCount)
	}
}

func TestSemanticTokensAsyncModifier(t *testing.T) {
	src := `module a version "1.0";
async function fetch_data() returns Int { return 0; }
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	var asyncFn *semanticToken
	for i := range toks {
		if toks[i].Type == tokFunction && toks[i].Modifiers&modAsync != 0 {
			asyncFn = &toks[i]
			break
		}
	}
	if asyncFn == nil {
		t.Fatal("expected async modifier on fetch_data")
	}
}

func TestSemanticTokensEnumAndVariants(t *testing.T) {
	src := `module a version "1.0";
enum Color { Red, Green, Blue }
entry function main() returns Int { return 0; }
`
	prog := parser.New(src).Parse()
	toks := collectSemanticTokens(prog)
	var enumTok *semanticToken
	variantCount := 0
	for i := range toks {
		switch toks[i].Type {
		case tokEnum:
			enumTok = &toks[i]
		case tokEnumMember:
			variantCount++
		}
	}
	if enumTok == nil {
		t.Fatal("expected enum token")
	}
	if variantCount != 3 {
		t.Errorf("expected 3 enum member tokens, got %d", variantCount)
	}
}

func TestEncodeSemanticTokensDeltas(t *testing.T) {
	// Three tokens; assert the delta encoding matches the LSP spec.
	tokens := []semanticToken{
		{Line: 2, StartChar: 5, Length: 3, Type: tokFunction, Modifiers: modDeclaration},
		{Line: 2, StartChar: 10, Length: 1, Type: tokParameter, Modifiers: 0},
		{Line: 5, StartChar: 0, Length: 4, Type: tokVariable, Modifiers: 0},
	}
	got := encodeSemanticTokens(tokens)
	want := []uint32{
		2, 5, 3, tokFunction, modDeclaration, // first: absolute line + char
		0, 5, 1, tokParameter, 0, // same line: delta char = 10 - 5
		3, 0, 4, tokVariable, 0, // new line: deltaLine=3, deltaStart=absolute(0)
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("encoded[%d]: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestSemanticTokensHandlerEndToEnd(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	uri := DocumentURI("file:///tmp/st.intent")
	src := "module a version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int { return x + y; }\n" +
		"entry function main() returns Int { return add(1, 2); }\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	stP, _ := json.Marshal(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	mustSend(t, client, 2, "textDocument/semanticTokens/full", string(stP))
	resp, _ := client.readMessage()
	var st SemanticTokens
	if err := json.Unmarshal(resp.Result, &st); err != nil {
		t.Fatalf("decode: %v (raw %s)", err, resp.Result)
	}
	if len(st.Data)%5 != 0 {
		t.Fatalf("token data length not a multiple of 5: %d", len(st.Data))
	}
	if len(st.Data) == 0 {
		t.Error("expected at least one semantic token in response")
	}
}
