package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// Phase 18 task 18.1: scaffold tests — transport framing, initialize
// handshake, shutdown/exit, lifecycle gating.

func TestTransportFraming(t *testing.T) {
	// Encode a message via writeMessage, decode it via readMessage, expect
	// fidelity.
	var buf bytes.Buffer
	tw := newTransport(&bytes.Buffer{}, &buf)
	if err := tw.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "ping",
		Params:  json.RawMessage(`{"k":"v"}`),
	}); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}

	// The framed bytes should start with the Content-Length header.
	out := buf.String()
	if !strings.HasPrefix(out, "Content-Length: ") {
		t.Fatalf("expected Content-Length header, got %q", out)
	}

	tr := newTransport(&buf, io.Discard)
	got, err := tr.readMessage()
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if got.Method != "ping" || string(got.ID) != "1" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestTransportMissingContentLength(t *testing.T) {
	tr := newTransport(strings.NewReader("\r\n{\"jsonrpc\":\"2.0\"}"), io.Discard)
	if _, err := tr.readMessage(); err == nil {
		t.Error("expected error for missing Content-Length")
	}
}

// Helper: run a Server against in-memory pipes; the test acts as the client.
// Returns the client-side transport so the test can drive the server with
// requests/notifications, plus a closer to terminate the server.
func newTestServer(t *testing.T) (client *transport, close func()) {
	t.Helper()
	clientToServer := newPipe()
	serverToClient := newPipe()

	srv := NewServer(clientToServer.readEnd(), serverToClient.writeEnd())
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.Run()
	}()

	client = newTransport(serverToClient.readEnd(), clientToServer.writeEnd())
	close = func() {
		// Closing the client's write side EOFs the server's read; Run() returns nil.
		clientToServer.closeWrite()
		<-srvErr
	}
	return client, close
}

func TestInitializeHandshake(t *testing.T) {
	client, closeFn := newTestServer(t)
	defer closeFn()

	if err := client.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"rootUri":"file:///tmp"}`),
	}); err != nil {
		t.Fatalf("send initialize: %v", err)
	}

	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("read initialize response: %v", err)
	}
	if string(resp.ID) != "1" {
		t.Errorf("response id: got %s want 1", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}
	var got InitializeResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !got.Capabilities.HoverProvider {
		t.Error("expected HoverProvider true")
	}
	if !got.Capabilities.DefinitionProvider {
		t.Error("expected DefinitionProvider true")
	}
	if got.Capabilities.TextDocumentSync == nil || got.Capabilities.TextDocumentSync.Change != SyncFull {
		t.Errorf("expected Full sync, got %+v", got.Capabilities.TextDocumentSync)
	}
}

func TestRequestBeforeInitializeRejected(t *testing.T) {
	client, closeFn := newTestServer(t)
	defer closeFn()

	if err := client.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`9`),
		Method:  "textDocument/hover",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("send hover: %v", err)
	}
	resp, err := client.readMessage()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32002 {
		t.Errorf("expected ServerNotInitialized (-32002), got %+v", resp.Error)
	}
}

func TestShutdownExitFlow(t *testing.T) {
	client, closeFn := newTestServer(t)
	defer closeFn()

	// Initialize first.
	mustSend(t, client, 1, "initialize", `{}`)
	resp, err := client.readMessage()
	if err != nil || resp.Error != nil {
		t.Fatalf("initialize failed: err=%v rpcErr=%+v", err, resp.Error)
	}

	// Shutdown returns null result.
	mustSend(t, client, 2, "shutdown", `null`)
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("shutdown response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("shutdown rpc error: %+v", resp.Error)
	}
	if string(resp.Result) != "null" {
		t.Errorf("shutdown result: got %s want null", resp.Result)
	}

	// Post-shutdown requests must error.
	mustSend(t, client, 3, "textDocument/hover", `{}`)
	resp, err = client.readMessage()
	if err != nil {
		t.Fatalf("post-shutdown response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != errInvalidRequest {
		t.Errorf("expected InvalidRequest after shutdown, got %+v", resp.Error)
	}

	// exit is a notification — no response. Sending it then closing the
	// pipe drives Run() to return nil.
	mustSendNotification(t, client, "exit", `null`)
}

func mustSend(t *testing.T, c *transport, id int, method string, paramsJSON string) {
	t.Helper()
	if err := c.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
		Params:  json.RawMessage(paramsJSON),
	}); err != nil {
		t.Fatalf("send %s: %v", method, err)
	}
}

func mustSendNotification(t *testing.T, c *transport, method string, paramsJSON string) {
	t.Helper()
	if err := c.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  json.RawMessage(paramsJSON),
	}); err != nil {
		t.Fatalf("send notification %s: %v", method, err)
	}
}

// pipe is a thin wrapper combining a bytes.Buffer with a sync mechanism for
// in-process LSP transport tests. Closing the write end signals EOF to the
// reader.
type pipe struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	cond   *sync.Cond
	closed bool
}

func newPipe() *pipe {
	p := &pipe{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

type pipeReader struct{ p *pipe }

func (r *pipeReader) Read(b []byte) (int, error) {
	r.p.mu.Lock()
	defer r.p.mu.Unlock()
	for r.p.buf.Len() == 0 && !r.p.closed {
		r.p.cond.Wait()
	}
	if r.p.buf.Len() == 0 && r.p.closed {
		return 0, io.EOF
	}
	return r.p.buf.Read(b)
}

type pipeWriter struct{ p *pipe }

func (w *pipeWriter) Write(b []byte) (int, error) {
	w.p.mu.Lock()
	defer w.p.mu.Unlock()
	n, err := w.p.buf.Write(b)
	w.p.cond.Broadcast()
	return n, err
}

func (p *pipe) readEnd() io.Reader  { return &pipeReader{p} }
func (p *pipe) writeEnd() io.Writer { return &pipeWriter{p} }
func (p *pipe) closeWrite() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.cond.Broadcast()
}
