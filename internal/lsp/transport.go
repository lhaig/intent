package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// transport handles LSP's Content-Length framed JSON-RPC 2.0 envelope over a
// reader/writer pair (typically stdin/stdout). It's intentionally small —
// ADR 0002's zero-Go-deps stance means no third-party LSP library; LSP
// framing is simple enough to roll by hand.

type transport struct {
	reader *bufio.Reader
	writer io.Writer
	wmu    sync.Mutex
}

func newTransport(in io.Reader, out io.Writer) *transport {
	return &transport{
		reader: bufio.NewReader(in),
		writer: out,
	}
}

// readMessage reads one Content-Length-framed JSON-RPC message. Returns
// io.EOF cleanly when the client closes the connection.
func (t *transport) readMessage() (*rpcMessage, error) {
	contentLength := -1
	// Read headers until blank line.
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		// Headers are "Name: Value". The only one we care about is Content-Length.
		if i := strings.Index(line, ":"); i >= 0 {
			name := strings.TrimSpace(line[:i])
			value := strings.TrimSpace(line[i+1:])
			if strings.EqualFold(name, "Content-Length") {
				n, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid Content-Length %q: %w", value, err)
				}
				contentLength = n
			}
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return nil, fmt.Errorf("read body (length %d): %w", contentLength, err)
	}

	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}
	return &msg, nil
}

// writeMessage writes the message with the LSP framing. Safe for concurrent
// callers (the verifier goroutine and the main dispatch loop both publish).
func (t *transport) writeMessage(msg *rpcMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if _, err := fmt.Fprintf(t.writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := t.writer.Write(body); err != nil {
		return err
	}
	return nil
}

// writeResponse sends a successful response to a request.
func (t *transport) writeResponse(id json.RawMessage, result interface{}) error {
	var raw json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		raw = b
	} else {
		raw = json.RawMessage("null")
	}
	return t.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	})
}

// writeError sends a JSON-RPC error response.
func (t *transport) writeError(id json.RawMessage, code int, message string) error {
	return t.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

// writeNotification sends a server-initiated notification (no id).
func (t *transport) writeNotification(method string, params interface{}) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = b
	}
	return t.writeMessage(&rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	})
}
