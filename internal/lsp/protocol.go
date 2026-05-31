package lsp

import "encoding/json"

// Minimal LSP 3.17 message types. Only the subset of fields the v1 server
// actually reads/writes is modelled here. Unused fields stay as
// json.RawMessage so the wire format survives without us having to track
// every spec change.
//
// See ADR 0032 and ops/plans/phase-18-lsp-server.md for the v1 surface.

// JSON-RPC 2.0 envelope.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// JSON-RPC error codes (LSP-defined values).
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

// DocumentURI is the LSP file identifier. Always `file://...` in practice.
type DocumentURI string

// Position is 0-indexed line + 0-indexed UTF-16 character. V1 treats the
// character field as a byte offset; this is correct for ASCII content and
// documented as a limitation in ops/plans/phase-18-lsp-server.md.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// InitializeParams carries the fields v1 actually inspects. RootURI is the
// workspace root (file://... form); the server falls back to per-document
// directories if absent.
type InitializeParams struct {
	ProcessID *int        `json:"processId,omitempty"`
	RootURI   DocumentURI `json:"rootUri,omitempty"`
	// Capabilities, ClientInfo, etc. — not consumed in v1.
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ServerCapabilities struct {
	TextDocumentSync           *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	HoverProvider              bool                     `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool                     `json:"definitionProvider,omitempty"`
	DocumentFormattingProvider bool                     `json:"documentFormattingProvider,omitempty"`
	DocumentSymbolProvider     bool                     `json:"documentSymbolProvider,omitempty"`
	SignatureHelpProvider      *SignatureHelpOptions    `json:"signatureHelpProvider,omitempty"`
	CompletionProvider         *CompletionOptions       `json:"completionProvider,omitempty"`
	SemanticTokensProvider     *SemanticTokensOptions   `json:"semanticTokensProvider,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

type TextDocumentSyncOptions struct {
	OpenClose bool                 `json:"openClose"`
	Change    TextDocumentSyncKind `json:"change"`
	Save      *SaveOptions         `json:"save,omitempty"`
}

type SaveOptions struct {
	IncludeText bool `json:"includeText"`
}

// TextDocumentSyncKind: 0 = none, 1 = full, 2 = incremental.
// V1 uses Full (ADR 0032 §O6 → A).
type TextDocumentSyncKind int

const (
	SyncNone        TextDocumentSyncKind = 0
	SyncFull        TextDocumentSyncKind = 1
	SyncIncremental TextDocumentSyncKind = 2
)

type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

type VersionedTextDocumentIdentifier struct {
	URI     DocumentURI `json:"uri"`
	Version int         `json:"version"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams uses Full sync — each change carries the
// complete document text (Range/RangeLength absent).
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         string                 `json:"text,omitempty"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// PublishDiagnosticsParams is the notification body sent to the client when
// diagnostics change.
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// DiagnosticSeverity per LSP spec.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// SymbolKind per LSP spec — only the values v1 emits are listed.
type SymbolKind int

const (
	SymbolFile        SymbolKind = 1
	SymbolModule      SymbolKind = 2
	SymbolNamespace   SymbolKind = 3
	SymbolPackage     SymbolKind = 4
	SymbolClass       SymbolKind = 5
	SymbolMethod      SymbolKind = 6
	SymbolProperty    SymbolKind = 7
	SymbolField       SymbolKind = 8
	SymbolConstructor SymbolKind = 9
	SymbolEnum        SymbolKind = 10
	SymbolInterface   SymbolKind = 11
	SymbolFunction    SymbolKind = 12
	SymbolVariable    SymbolKind = 13
	SymbolEnumMember  SymbolKind = 22
	SymbolStruct      SymbolKind = 23
)

// DocumentSymbol is the LSP outline-tree entry for textDocument/documentSymbol.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SignatureHelp is the response for textDocument/signatureHelp.
type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type SignatureInformation struct {
	Label      string                 `json:"label"`
	Parameters []ParameterInformation `json:"parameters,omitempty"`
}

type ParameterInformation struct {
	Label string `json:"label"`
}

// CompletionItem is a single completion suggestion.
type CompletionItem struct {
	Label  string             `json:"label"`
	Kind   CompletionItemKind `json:"kind,omitempty"`
	Detail string             `json:"detail,omitempty"`
}

// SemanticTokensLegend names the token types and modifiers the server
// emits. The client decodes tokens by indexing into TokenTypes /
// TokenModifiers, so order is part of the wire contract — adding a new
// type appends to TokenTypes; renaming or reordering would break
// already-running clients.
type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Range  bool                 `json:"range,omitempty"`
	Full   bool                 `json:"full,omitempty"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

type CompletionItemKind int

const (
	CompletionText        CompletionItemKind = 1
	CompletionMethod      CompletionItemKind = 2
	CompletionFunction    CompletionItemKind = 3
	CompletionConstructor CompletionItemKind = 4
	CompletionField       CompletionItemKind = 5
	CompletionVariable    CompletionItemKind = 6
	CompletionClass       CompletionItemKind = 7
	CompletionInterface   CompletionItemKind = 8
	CompletionModule      CompletionItemKind = 9
	CompletionProperty    CompletionItemKind = 10
	CompletionEnum        CompletionItemKind = 13
	CompletionKeyword     CompletionItemKind = 14
	CompletionStruct      CompletionItemKind = 22
	CompletionTypeParam   CompletionItemKind = 25
)

// Hover response (markdown).
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" or "markdown"
	Value string `json:"value"`
}
