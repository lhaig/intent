package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 19 task 19.4: formatting via LSP.

func TestComputeFormattingEditsCleanFileNoOp(t *testing.T) {
	// Round-trip the unformatted source once to get canonical form, then
	// re-format and assert idempotence.
	rough := "module a version \"1.0\";\nentry function main() returns Int {\nreturn 0;\n}\n"
	firstPass := computeFormattingEdits(rough)
	if len(firstPass) != 1 {
		t.Fatalf("expected one edit on first pass, got %d", len(firstPass))
	}
	canonical := firstPass[0].NewText
	secondPass := computeFormattingEdits(canonical)
	if len(secondPass) != 0 {
		t.Errorf("expected no edits on canonical input, got %d:\nfirst:\n%s\nsecond:\n%v", len(secondPass), canonical, secondPass)
	}
}

func TestComputeFormattingEditsUnparseable(t *testing.T) {
	// Garbage — no edits, no panic.
	edits := computeFormattingEdits("module a version \"1.0\";\nentry function (")
	if len(edits) != 0 {
		t.Errorf("expected no edits on unparseable file, got %+v", edits)
	}
}

func TestComputeFormattingEditsRewritesUnformatted(t *testing.T) {
	// Force unformatted source by mangling indentation.
	src := "module a version \"1.0\";\nentry function main() returns Int {\nreturn 0;\n}\n"
	edits := computeFormattingEdits(src)
	if len(edits) != 1 {
		t.Fatalf("expected one edit, got %d", len(edits))
	}
	if !strings.Contains(edits[0].NewText, "    return 0;") {
		t.Errorf("expected indented return in formatted output, got:\n%s", edits[0].NewText)
	}
}

func TestEndOfTextNoTrailingNewline(t *testing.T) {
	got := endOfText("abc\ndef")
	want := Position{Line: 1, Character: 3}
	if got != want {
		t.Errorf("endOfText: got %+v want %+v", got, want)
	}
}

func TestEndOfTextTrailingNewline(t *testing.T) {
	got := endOfText("abc\ndef\n")
	want := Position{Line: 2, Character: 0}
	if got != want {
		t.Errorf("endOfText: got %+v want %+v", got, want)
	}
}

func TestEndOfTextEmpty(t *testing.T) {
	got := endOfText("")
	want := Position{Line: 0, Character: 0}
	if got != want {
		t.Errorf("endOfText: got %+v want %+v", got, want)
	}
}

func TestFormattingHandlerEndToEnd(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}

	uri := DocumentURI("file:///tmp/fmt.intent")
	src := "module a version \"1.0\";\nentry function main() returns Int {\nreturn 0;\n}\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: src},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))
	mustReadPublishDiagnostics(t, client)

	fmtP, _ := json.Marshal(DocumentFormattingParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	mustSend(t, client, 2, "textDocument/formatting", string(fmtP))
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("format response: %v", err)
	}
	var edits []TextEdit
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		t.Fatalf("decode edits: %v (raw %s)", err, resp.Result)
	}
	if len(edits) != 1 {
		t.Fatalf("expected one edit, got %d", len(edits))
	}
	if !strings.Contains(edits[0].NewText, "    return 0;") {
		t.Errorf("expected reformatted body, got:\n%s", edits[0].NewText)
	}
}
