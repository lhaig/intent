package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// Phase 18 task 18.2: document state + text sync.

func TestDocumentStoreOpenUpdateClose(t *testing.T) {
	s := newDocumentStore()
	uri := DocumentURI("file:///tmp/a.intent")

	s.open(uri, 1, "first")
	d, ok := s.get(uri)
	if !ok || d.snapshotText() != "first" {
		t.Fatalf("expected open to store text; got %+v", d)
	}

	s.update(uri, 2, "second")
	d, _ = s.get(uri)
	if d.snapshotText() != "second" || d.Version != 2 {
		t.Errorf("update did not replace: %+v", d)
	}

	s.close(uri)
	if _, ok := s.get(uri); ok {
		t.Error("close did not remove document")
	}
}

func TestDocumentLineOffsets(t *testing.T) {
	s := newDocumentStore()
	uri := DocumentURI("file:///tmp/x.intent")
	d := s.open(uri, 1, "line1\nline2\n\nline4")

	got := d.buildLineOffsets()
	want := []int{0, 6, 12, 13}
	if len(got) != len(want) {
		t.Fatalf("line offsets: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("offset[%d]: got %d want %d", i, got[i], want[i])
		}
	}

	// After update, offsets should be invalidated and recomputed.
	s.update(uri, 2, "x")
	got2 := d.buildLineOffsets()
	if len(got2) != 1 || got2[0] != 0 {
		t.Errorf("post-update offsets: got %v want [0]", got2)
	}
}

func TestPositionRoundTrip(t *testing.T) {
	s := newDocumentStore()
	d := s.open(DocumentURI("file:///x"), 1, "hello\nworld")
	// LSP position {Line: 1, Character: 2} → 1-indexed (2, 3).
	line, col := d.positionToLineCol(Position{Line: 1, Character: 2})
	if line != 2 || col != 3 {
		t.Errorf("positionToLineCol: got (%d, %d) want (2, 3)", line, col)
	}
	// Inverse:
	p := lineColToPosition(2, 3)
	if p.Line != 1 || p.Character != 2 {
		t.Errorf("lineColToPosition: got %+v want {1, 2}", p)
	}
}

func TestURIPathRoundTrip(t *testing.T) {
	cases := []string{"/tmp/foo.intent", "/home/user/dir with space/main.intent"}
	for _, path := range cases {
		uri := pathToURI(path)
		got := uriToPath(uri)
		if got != path {
			t.Errorf("uri round-trip: %q -> %q -> %q", path, uri, got)
		}
	}
}

func TestServerDidOpenChangeClose(t *testing.T) {
	client, closeFn := newTestServer(t)
	defer closeFn()

	// Initialize first (required by lifecycle gate).
	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// didOpen
	openParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        DocumentURI("file:///tmp/a.intent"),
			LanguageID: "intent",
			Version:    1,
			Text:       "module a version \"1.0\";",
		},
	}
	b, _ := json.Marshal(openParams)
	mustSendNotification(t, client, "textDocument/didOpen", string(b))

	// didChange (Full sync)
	changeParams := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     openParams.TextDocument.URI,
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: "module a version \"2.0\";"},
		},
	}
	b, _ = json.Marshal(changeParams)
	mustSendNotification(t, client, "textDocument/didChange", string(b))

	// didClose
	closeParams := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: openParams.TextDocument.URI},
	}
	b, _ = json.Marshal(closeParams)
	mustSendNotification(t, client, "textDocument/didClose", string(b))

	// Should be no responses (all notifications); subsequent shutdown should
	// still work, confirming the server processed those notifications without
	// erroring out.
	mustSend(t, client, 2, "shutdown", `null`)
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("shutdown rpc error: %+v", resp.Error)
	}
}

func TestServerDidOpenWrongShapeIgnored(t *testing.T) {
	// Structurally valid JSON but wrong shape (params is a string, not an
	// object). The handler unmarshals into a struct, fails, and silently
	// drops the notification — no crash, no error response.
	client, closeFn := newTestServer(t)
	defer closeFn()
	mustSend(t, client, 1, "initialize", `{}`)
	if _, err := client.readMessage(); err != nil {
		t.Fatal(err)
	}
	mustSendNotification(t, client, "textDocument/didOpen", `"not an object"`)

	mustSend(t, client, 2, "shutdown", `null`)
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("server died on wrong-shape didOpen: %+v", resp.Error)
	}
}

func TestUriToPathNonFile(t *testing.T) {
	if uriToPath(DocumentURI("http://example.com/x")) != "" {
		t.Error("non-file URI should return empty path")
	}
}

func TestPositionOutOfRangeFallsBack(t *testing.T) {
	s := newDocumentStore()
	d := s.open(DocumentURI("file:///x"), 1, "x")
	line, col := d.positionToLineCol(Position{Line: -1, Character: -1})
	if line != 1 || col != 1 {
		t.Errorf("negative position should fall back to (1,1), got (%d,%d)", line, col)
	}
}

// Sanity: the package builds without the deferred handler imports leaking.
func TestDocumentTextNoTrailingNul(t *testing.T) {
	s := newDocumentStore()
	d := s.open(DocumentURI("file:///x"), 1, "abc\n")
	if strings.Contains(d.snapshotText(), "\x00") {
		t.Error("doc text contains NUL byte")
	}
}
