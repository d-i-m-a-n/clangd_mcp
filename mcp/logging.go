package mcp

import (
	"encoding/json"
	"log"
	"strings"
)

// SSEDebugLogger logs detailed SSE requests and responses for debugging.
type SSEDebugLogger struct {
	enabled bool
}

// NewSSEDebugLogger creates a new SSE debug logger.
func NewSSEDebugLogger(enabled bool) *SSEDebugLogger {
	return &SSEDebugLogger{enabled: enabled}
}

// LogRequest logs an incoming MCP tool call request.
func (l *SSEDebugLogger) LogRequest(toolName string, args map[string]interface{}) {
	if !l.enabled {
		return
	}
	argsJSON, _ := json.MarshalIndent(args, "  ", "  ")
	log.Printf("[SSE-DEBUG] → Request: %s\n%s", toolName, string(argsJSON))
}

// LogResponse logs an outgoing MCP tool call response.
func (l *SSEDebugLogger) LogResponse(toolName string, result string, err error) {
	if !l.enabled {
		return
	}
	if err != nil {
		log.Printf("[SSE-DEBUG] ← Response: %s\n  error: %v", toolName, err)
		return
	}

	result = strings.TrimSpace(result)
	if len(result) > 500 {
		log.Printf("[SSE-DEBUG] ← Response: %s\n  result (truncated): %s...", toolName, result[:500])
	} else {
		log.Printf("[SSE-DEBUG] ← Response: %s\n  result: %s", toolName, result)
	}
}

// LogLSPRequest logs an LSP request being sent from an MCP tool.
func (l *SSEDebugLogger) LogLSPRequest(method string, params json.RawMessage) {
	if !l.enabled {
		return
	}
	var paramsObj interface{}
	if err := json.Unmarshal(params, &paramsObj); err == nil {
		paramsJSON, _ := json.MarshalIndent(paramsObj, "  ", "  ")
		log.Printf("[SSE-DEBUG]   → LSP: %s\n%s", method, string(paramsJSON))
	} else {
		log.Printf("[SSE-DEBUG]   → LSP: %s", method)
	}
}

// LogLSPResponse logs an LSP response received for an MCP tool request.
func (l *SSEDebugLogger) LogLSPResponse(method string, result json.RawMessage, err error) {
	if !l.enabled {
		return
	}
	if err != nil {
		log.Printf("[SSE-DEBUG]   ← LSP: %s\n  error: %v", method, err)
		return
	}

	if result == nil {
		log.Printf("[SSE-DEBUG]   ← LSP: %s\n  result: null", method)
		return
	}

	resultStr := string(result)
	if len(resultStr) > 500 {
		log.Printf("[SSE-DEBUG]   ← LSP: %s\n  result (truncated): %s...", method, resultStr[:500])
	} else {
		var obj interface{}
		if err := json.Unmarshal(result, &obj); err == nil {
			resultJSON, _ := json.MarshalIndent(obj, "  ", "  ")
			log.Printf("[SSE-DEBUG]   ← LSP: %s\n%s", method, string(resultJSON))
		} else {
			log.Printf("[SSE-DEBUG]   ← LSP: %s\n  result: %s", method, resultStr)
		}
	}
}

// LogError logs an error that occurred during MCP tool execution.
func (l *SSEDebugLogger) LogError(toolName string, msg string) {
	if !l.enabled {
		return
	}
	log.Printf("[SSE-DEBUG] ✗ Error in %s: %s", toolName, msg)
}

// LogDuration logs the execution time of an MCP tool call.
func (l *SSEDebugLogger) LogDuration(toolName string, durationMs float64) {
	if !l.enabled {
		return
	}
	log.Printf("[SSE-DEBUG] ⏱ %s completed in %.2fms", toolName, durationMs)
}
