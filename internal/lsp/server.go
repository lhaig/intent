package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Server is the LSP 3.17 server for Intent. It speaks JSON-RPC over a
// reader/writer pair (typically os.Stdin / os.Stdout when launched via
// `intentc lsp`).
//
// Lifecycle: initialize → initialized → ... → shutdown → exit. Per LSP, any
// request received before initialize must return error code -32002
// (ServerNotInitialized), and any request other than exit received after
// shutdown must return -32600 (InvalidRequest). We implement these strictly
// so well-behaved clients see clean errors.
type Server struct {
	t *transport

	mu          sync.Mutex
	initialized bool
	shutdown    bool
	rootURI     DocumentURI
}

// NewServer constructs a Server bound to the given reader/writer pair.
func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{t: newTransport(in, out)}
}

// Run reads messages until EOF or shutdown+exit. Returns nil on clean exit,
// non-nil on transport or framing errors.
func (s *Server) Run() error {
	for {
		msg, err := s.t.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		s.dispatch(msg)
	}
}

// dispatch routes a single inbound message. Requests get responses;
// notifications don't.
func (s *Server) dispatch(msg *rpcMessage) {
	isRequest := len(msg.ID) > 0

	// Lifecycle gates per LSP spec.
	s.mu.Lock()
	initialized := s.initialized
	shutdown := s.shutdown
	s.mu.Unlock()

	switch msg.Method {
	case "initialize":
		if !isRequest {
			return
		}
		s.handleInitialize(msg.ID, msg.Params)
		return
	case "initialized":
		// Client notification after initialize result. No-op for v1.
		return
	case "shutdown":
		if !isRequest {
			return
		}
		s.handleShutdown(msg.ID)
		return
	case "exit":
		// Notification. The Run() loop terminates when the client closes
		// stdin; we set a flag so any caller that wants to assert post-exit
		// state can. The spec also says we should exit the process here, but
		// `intentc lsp` is a subcommand and the caller (CLI main) is
		// responsible for the process lifecycle.
		s.mu.Lock()
		s.shutdown = true
		s.mu.Unlock()
		return
	}

	if !initialized {
		if isRequest {
			s.t.writeError(msg.ID, -32002, "server not initialized")
		}
		return
	}
	if shutdown {
		if isRequest {
			s.t.writeError(msg.ID, errInvalidRequest, "server has been shut down")
		}
		return
	}

	// Post-initialize, pre-shutdown handlers. Filled in by later 18.x tasks.
	switch msg.Method {
	default:
		if isRequest {
			s.t.writeError(msg.ID, errMethodNotFound, fmt.Sprintf("method not found: %s", msg.Method))
		}
		// Unknown notifications are silently ignored per JSON-RPC convention.
	}
}

func (s *Server) handleInitialize(id json.RawMessage, params json.RawMessage) {
	var p InitializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			s.t.writeError(id, errInvalidParams, fmt.Sprintf("decode initialize params: %v", err))
			return
		}
	}

	s.mu.Lock()
	s.initialized = true
	s.rootURI = p.RootURI
	s.mu.Unlock()

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    SyncFull,
				Save:      &SaveOptions{IncludeText: false},
			},
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		ServerInfo: &ServerInfo{
			Name:    "intentc-lsp",
			Version: "0.1.0",
		},
	}
	if err := s.t.writeResponse(id, result); err != nil {
		// Connection broken — Run() will return the next read error.
		return
	}
}

func (s *Server) handleShutdown(id json.RawMessage) {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()
	// LSP requires the result to be null.
	s.t.writeResponse(id, nil)
}
