package lsp

import "encoding/json"

// Message is a JSON-RPC 2.0 message (request, response, or notification).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError is a JSON-RPC error object.
type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (m *Message) hasID() bool {
	return len(m.ID) > 0 && string(m.ID) != "null"
}

// IsRequest returns true for a request (has non-null ID and a method).
func (m *Message) IsRequest() bool { return m.hasID() && m.Method != "" }

// IsResponse returns true for a response (has non-null ID, no method).
func (m *Message) IsResponse() bool { return m.hasID() && m.Method == "" }

// IsNotification returns true for a notification (method, no ID).
func (m *Message) IsNotification() bool { return m.Method != "" && !m.hasID() }

// Position is a zero-based LSP position.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a zero-based LSP range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a file URI + range.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// SymbolInformation is returned by workspace/symbol.
type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}

// TextEdit is a textual edit applicable to a text document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}
