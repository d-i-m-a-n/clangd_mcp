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

// Hover is the result of a textDocument/hover request.
type Hover struct {
	Contents json.RawMessage `json:"contents"`
	Range    *Range          `json:"range,omitempty"`
}

// LocationLink links an origin selection range to a target location.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// CallHierarchyItem represents a node in a call hierarchy.
type CallHierarchyItem struct {
	Name           string          `json:"name"`
	Kind           int             `json:"kind"`
	Detail         string          `json:"detail,omitempty"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}

// CallHierarchyIncomingCall is a single entry returned by callHierarchy/incomingCalls.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCall is a single entry returned by callHierarchy/outgoingCalls.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// DocumentSymbol represents a symbol in a document, possibly with nested children.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// TypeHierarchyItem represents a node in a type hierarchy.
type TypeHierarchyItem struct {
	Name           string          `json:"name"`
	Kind           int             `json:"kind"`
	Detail         string          `json:"detail,omitempty"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}

// TypeHierarchySupertype is a single entry returned by typeHierarchy/supertypes.
type TypeHierarchySupertype struct {
	Type  TypeHierarchyItem `json:"type"`
	From  []Range           `json:"from,omitempty"`
}

// TypeHierarchySubtype is a single entry returned by typeHierarchy/subtypes.
type TypeHierarchySubtype struct {
	Type TypeHierarchyItem `json:"type"`
	From []Range           `json:"from,omitempty"`
}
