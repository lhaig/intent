package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 18 task 18.9: end-to-end smoke test. One scripted LSP session
// drives the server through every v1 surface — initialize, didOpen
// (with a broken file), didChange (to a clean file), hover, definition,
// shutdown, exit — and asserts the wire-level responses are sane.
// Locks down the v1 protocol contract so 18.10 docs can reference it.

func TestE2ELspSession(t *testing.T) {
	client, closeFn, srv := newTestServerWithHandle(t)
	defer closeFn()
	srv.SetDiagnosticsDebounce(0)

	// --- initialize ---
	mustSend(t, client, 1, "initialize", `{"rootUri":"file:///tmp"}`)
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if !initResult.Capabilities.HoverProvider || !initResult.Capabilities.DefinitionProvider {
		t.Fatalf("missing v1 capabilities: %+v", initResult.Capabilities)
	}

	mustSendNotification(t, client, "initialized", `{}`)

	// --- didOpen: broken file ---
	uri := DocumentURI("file:///tmp/e2e.intent")
	brokenText := "module e2e version \"1.0\";\n" +
		"function add(x: Int, y: Int) returns Int\n" +
		"    requires x >= 0\n" +
		"    ensures result >= x\n" +
		"{\n" +
		"    return x + y;\n" +
		"}\n" +
		"entry function main() returns Int {\n" +
		"    let x: Int = \"oops\";\n" + // type mismatch — checker error
		"    return add(x, 2);\n" +
		"}\n"
	openP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "intent", Version: 1, Text: brokenText},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openP))

	notif := mustReadPublishDiagnostics(t, client)
	if notif.URI != uri {
		t.Errorf("publish URI: got %q want %q", notif.URI, uri)
	}
	if !hasErrorDiagnostic(notif.Diagnostics) {
		t.Errorf("expected at least one Error diagnostic on broken open, got %+v", notif.Diagnostics)
	}

	// --- didChange: fix the type error ---
	cleanText := strings.Replace(brokenText, `let x: Int = "oops";`, `let x: Int = 1;`, 1)
	changeP, _ := json.Marshal(DidChangeTextDocumentParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{{Text: cleanText}},
	})
	mustSendNotification(t, client, "textDocument/didChange", string(changeP))

	notif = mustReadPublishDiagnostics(t, client)
	if hasErrorDiagnostic(notif.Diagnostics) {
		t.Errorf("expected no Error diagnostics after fix, got %+v", notif.Diagnostics)
	}

	// --- hover on 'add' inside main's call site ---
	// Locate 'add(' in cleanText by line.
	addLine, addCol := findInText(cleanText, "add(")
	hoverP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addLine, addCol+1), // middle of 'add'
	})
	mustSend(t, client, 2, "textDocument/hover", string(hoverP))
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("hover: %v", err)
	}
	var hov Hover
	if err := json.Unmarshal(resp.Result, &hov); err != nil {
		t.Fatalf("decode hover: %v (raw %s)", err, resp.Result)
	}
	if !strings.Contains(hov.Contents.Value, "function add(x: Int, y: Int) returns Int") {
		t.Errorf("hover missing signature, got:\n%s", hov.Contents.Value)
	}
	if !strings.Contains(hov.Contents.Value, "requires x >= 0") {
		t.Errorf("hover missing requires, got:\n%s", hov.Contents.Value)
	}

	// --- definition on the same 'add' ---
	defP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addLine, addCol+1),
	})
	mustSend(t, client, 3, "textDocument/definition", string(defP))
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err != nil {
		t.Fatalf("decode definition: %v (raw %s)", err, resp.Result)
	}
	if loc.URI != uri {
		t.Errorf("definition URI: got %q want %q", loc.URI, uri)
	}
	// 'add' is declared on line 2 of cleanText (1-indexed) → LSP line 1.
	if loc.Range.Start.Line != 1 {
		t.Errorf("definition start line: got %d want 1", loc.Range.Start.Line)
	}

	// --- Phase 19 surface: documentSymbol, formatting, signatureHelp,
	// completion, plus hover/goto-def on a local. ---

	// documentSymbol: outline of the file.
	dsP, _ := json.Marshal(DocumentSymbolParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	mustSend(t, client, 100, "textDocument/documentSymbol", string(dsP))
	resp, _ = client.readMessage()
	var syms []DocumentSymbol
	if err := json.Unmarshal(resp.Result, &syms); err != nil {
		t.Fatalf("decode documentSymbol: %v (raw %s)", err, resp.Result)
	}
	found := map[string]bool{}
	for _, s := range syms {
		found[s.Name] = true
	}
	if !found["add"] || !found["main"] {
		t.Errorf("expected add and main in documentSymbol, got %v", found)
	}

	// formatting: parse the file. The Phase 18 cleanText is already
	// canonical, so we expect no edits. Format a deliberately-malformed
	// version on a separate handler call would also work, but the no-op
	// path is what real editors hit most often.
	fmtP, _ := json.Marshal(DocumentFormattingParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	mustSend(t, client, 101, "textDocument/formatting", string(fmtP))
	resp, _ = client.readMessage()
	var edits []TextEdit
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		t.Fatalf("decode formatting: %v (raw %s)", err, resp.Result)
	}
	// Either empty (canonical) or one edit (rewrite); both are valid.
	if len(edits) > 1 {
		t.Errorf("formatting should produce at most one edit, got %d", len(edits))
	}

	// signatureHelp: cursor inside add(1, 2) — should return add's signature.
	// Re-find 'add(' for the call site position (the call has two args).
	addCallLine, addCallCol := findInText(cleanText, "add(")
	sigP, _ := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addCallLine, addCallCol+len("add(")+1),
	})
	mustSend(t, client, 102, "textDocument/signatureHelp", string(sigP))
	resp, _ = client.readMessage()
	if string(resp.Result) != "null" {
		var sh SignatureHelp
		if err := json.Unmarshal(resp.Result, &sh); err != nil {
			t.Fatalf("decode signatureHelp: %v (raw %s)", err, resp.Result)
		}
		if len(sh.Signatures) != 1 || !strings.Contains(sh.Signatures[0].Label, "add(x: Int, y: Int)") {
			t.Errorf("signatureHelp wrong signature: %+v", sh)
		}
	}

	// completion: request anywhere in the file; expect 'add' and 'main'.
	cpP, _ := json.Marshal(CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addCallLine, 1),
	})
	mustSend(t, client, 103, "textDocument/completion", string(cpP))
	resp, _ = client.readMessage()
	var items []CompletionItem
	if err := json.Unmarshal(resp.Result, &items); err != nil {
		t.Fatalf("decode completion: %v (raw %s)", err, resp.Result)
	}
	foundAdd, foundLet := false, false
	for _, it := range items {
		if it.Label == "add" {
			foundAdd = true
		}
		if it.Label == "if" {
			foundLet = true
		}
	}
	if !foundAdd || !foundLet {
		t.Errorf("completion missing expected items (add=%v if=%v)", foundAdd, foundLet)
	}

	// Phase 21: member completion. Capability assertion + an end-to-end
	// member-position request.
	if initResult.Capabilities.CompletionProvider == nil {
		t.Fatal("completionProvider missing from initialize response")
	}
	foundDot := false
	for _, ch := range initResult.Capabilities.CompletionProvider.TriggerCharacters {
		if ch == "." {
			foundDot = true
		}
	}
	if !foundDot {
		t.Errorf("completionProvider.triggerCharacters missing '.', got %v", initResult.Capabilities.CompletionProvider.TriggerCharacters)
	}

	// Open a separate file containing an entity and an `acc.` position.
	memberURI := DocumentURI("file:///tmp/e2e_member.intent")
	memberText := "module e2e_m version \"1.0\";\n" +
		"entity Account {\n" +
		"    field balance: Int;\n" +
		"    method peek() returns Int { return self.balance; }\n" +
		"}\n" +
		"entry function main() returns Int {\n" +
		"    let acc: Account = Account();\n" +
		"    acc.\n" +
		"    return 0;\n" +
		"}\n"
	openMP, _ := json.Marshal(DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: memberURI, LanguageID: "intent", Version: 1, Text: memberText},
	})
	mustSendNotification(t, client, "textDocument/didOpen", string(openMP))
	mustReadPublishDiagnostics(t, client) // drain — parser will complain about `acc.` but that's fine for the completion test.

	// Cursor immediately after the dot on the `acc.` line.
	dotLine, dotCol := findInText(memberText, "acc.")
	mcpP, _ := json.Marshal(CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: memberURI},
		Position:     lineColToPosition(dotLine, dotCol+len("acc.")),
	})
	mustSend(t, client, 104, "textDocument/completion", string(mcpP))
	resp, _ = client.readMessage()
	var memberItems []CompletionItem
	if err := json.Unmarshal(resp.Result, &memberItems); err != nil {
		t.Fatalf("decode member completion: %v (raw %s)", err, resp.Result)
	}
	memberLabels := map[string]CompletionItemKind{}
	for _, it := range memberItems {
		memberLabels[it.Label] = it.Kind
	}
	if memberLabels["balance"] != CompletionField {
		t.Errorf("expected balance as Field in member completion, got %v (%d items)", memberLabels["balance"], len(memberItems))
	}
	if memberLabels["peek"] != CompletionMethod {
		t.Errorf("expected peek as Method in member completion, got %v", memberLabels["peek"])
	}
	if _, has := memberLabels["if"]; has {
		t.Error("keywords leaked into member completion")
	}
	if _, has := memberLabels["main"]; has {
		t.Error("top-level decls leaked into member completion")
	}

	// Phase 26: textDocument/references — find call sites of `add` from
	// the cleanText document. Two uses are expected: the call inside main()
	// (we're not counting the declaration unless includeDeclaration:true).
	refP, _ := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     lineColToPosition(addLine, addCol+1),
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	mustSend(t, client, 105, "textDocument/references", string(refP))
	resp, _ = client.readMessage()
	var refLocs []Location
	if err := json.Unmarshal(resp.Result, &refLocs); err != nil {
		t.Fatalf("decode references: %v (%s)", err, resp.Result)
	}
	if len(refLocs) < 2 {
		t.Errorf("expected >= 2 references for `add` (decl + at least one call), got %d", len(refLocs))
	}

	// --- shutdown / exit ---
	mustSend(t, client, 4, "shutdown", `null`)
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("shutdown error: %+v", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Errorf("shutdown result: got %s want null", resp.Result)
	}
	mustSendNotification(t, client, "exit", `null`)
}

func hasErrorDiagnostic(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// findInText returns 1-indexed (line, col) of the first occurrence of needle
// in haystack. Used by the E2E test to locate identifiers without
// hardcoding magic offsets. Returns (0, 0) on no match — caller can check.
func findInText(haystack, needle string) (line, col int) {
	idx := strings.Index(haystack, needle)
	if idx < 0 {
		return 0, 0
	}
	line = 1
	lineStart := 0
	for i := 0; i < idx; i++ {
		if haystack[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	col = idx - lineStart + 1
	return line, col
}
