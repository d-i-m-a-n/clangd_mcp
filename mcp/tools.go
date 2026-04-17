// Package mcp registers MCP tools backed by the LSP proxy.
//
// Tools (names mirror LSP method names, "/" replaced with "_"):
//   - workspace_symbol                 — search symbols by query string
//   - workspace_symbolResolve          — resolve full location for a WorkspaceSymbol
//   - textDocument_references          — find all references at a position
//   - textDocument_rename              — rename a symbol and apply edits to disk
//   - textDocument_hover               — hover info (type/docs) at a position
//   - textDocument_declaration         — go to declaration
//   - textDocument_definition          — go to definition
//   - textDocument_typeDefinition      — go to type definition
//   - textDocument_implementation      — go to implementations
//   - textDocument_prepareCallHierarchy — prepare call hierarchy items at a position
//   - callHierarchy_incomingCalls      — incoming callers for a call-hierarchy item
//   - callHierarchy_outgoingCalls      — outgoing callees for a call-hierarchy item
//   - textDocument_documentSymbol      — list all symbols in a document
//   - textDocument_prepareTypeHierarchy — prepare type hierarchy items at a position
//   - typeHierarchy_supertypes         — supertypes for a type-hierarchy item
//   - typeHierarchy_subtypes           — subtypes for a type-hierarchy item
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"clangd-mcp/lsp"
)

// LSPClient is the interface the MCP tools use to communicate with the LSP server.
// *proxy.Proxy implements this interface.
type LSPClient interface {
	SendRequest(method string, params json.RawMessage) (json.RawMessage, error)
	SendNotification(method string, params json.RawMessage) error
	Workspace() string
}

// toolHandler wraps a tool handler function with logging.
func wrapToolHandler(name string, logger *SSEDebugLogger, handler func() (string, error)) func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if logger != nil {
			logger.LogRequest(name, req.Params.Arguments)
		}
		result, err := handler()
		if logger != nil {
			logger.LogResponse(name, result, err)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}
}

// RegisterTools adds all tools to an MCPServer.
func RegisterTools(s *server.MCPServer, p LSPClient, logger *SSEDebugLogger) {
	s.AddTool(
		mcp.NewTool("workspace_symbol",
			mcp.WithDescription("Search for symbols in the workspace by name or partial name (workspace/symbol)."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Symbol name or partial name to search for (e.g. 'Plugin' or 'Potap::Plugin').")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("workspace_symbol", req.Params.Arguments)
			}
			result, err := workspaceSymbolTool(p, stringArg(req, "query"))
			if logger != nil {
				logger.LogResponse("workspace_symbol", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_references",
			mcp.WithDescription("Find all references to the symbol at a given position (textDocument/references)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_references", req.Params.Arguments)
			}
			result, err := textDocumentReferencesTool(p,
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_references", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_rename",
			mcp.WithDescription("Rename the symbol at a given position across the whole workspace and apply edits to disk (textDocument/rename)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithString("newName", mcp.Required(), mcp.Description("New name for the symbol.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{DestructiveHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_rename", req.Params.Arguments)
			}
			result, err := textDocumentRenameTool(p,
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
				stringArg(req, "newName"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_rename", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_hover",
			mcp.WithDescription("Return hover information (type signature, documentation) for the symbol at a given position (textDocument/hover)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_hover", req.Params.Arguments)
			}
			result, err := textDocumentHoverTool(p,
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_hover", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_declaration",
			mcp.WithDescription("Go to the declaration of the symbol at a given position (textDocument/declaration)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_declaration", req.Params.Arguments)
			}
			result, err := textDocumentLocationTool(p, "textDocument/declaration", "Declarations",
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_declaration", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_definition",
			mcp.WithDescription("Go to the definition of the symbol at a given position (textDocument/definition)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_definition", req.Params.Arguments)
			}
			result, err := textDocumentLocationTool(p, "textDocument/definition", "Definitions",
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_definition", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_typeDefinition",
			mcp.WithDescription("Go to the type definition of the symbol at a given position (textDocument/typeDefinition)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_typeDefinition", req.Params.Arguments)
			}
			result, err := textDocumentLocationTool(p, "textDocument/typeDefinition", "Type definitions",
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_typeDefinition", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_implementation",
			mcp.WithDescription("Find implementations of the symbol at a given position (textDocument/implementation)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_implementation", req.Params.Arguments)
			}
			result, err := textDocumentLocationTool(p, "textDocument/implementation", "Implementations",
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_implementation", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_prepareCallHierarchy",
			mcp.WithDescription("Prepare call hierarchy items for the symbol at a given position; pass an item's JSON to callHierarchy_incomingCalls or callHierarchy_outgoingCalls (textDocument/prepareCallHierarchy)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_prepareCallHierarchy", req.Params.Arguments)
			}
			result, err := textDocumentPrepareCallHierarchyTool(p,
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_prepareCallHierarchy", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("callHierarchy_incomingCalls",
			mcp.WithDescription("List all callers of a call-hierarchy item (callHierarchy/incomingCalls). Pass the JSON object of a CallHierarchyItem as returned by textDocument_prepareCallHierarchy."),
			mcp.WithString("item", mcp.Required(), mcp.Description("JSON object of a CallHierarchyItem.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("callHierarchy_incomingCalls", req.Params.Arguments)
			}
			result, err := callHierarchyIncomingCallsTool(p, stringArg(req, "item"))
			if logger != nil {
				logger.LogResponse("callHierarchy_incomingCalls", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("callHierarchy_outgoingCalls",
			mcp.WithDescription("List all callees of a call-hierarchy item (callHierarchy/outgoingCalls). Pass the JSON object of a CallHierarchyItem as returned by textDocument_prepareCallHierarchy."),
			mcp.WithString("item", mcp.Required(), mcp.Description("JSON object of a CallHierarchyItem.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("callHierarchy_outgoingCalls", req.Params.Arguments)
			}
			result, err := callHierarchyOutgoingCallsTool(p, stringArg(req, "item"))
			if logger != nil {
				logger.LogResponse("callHierarchy_outgoingCalls", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_documentSymbol",
			mcp.WithDescription("List all symbols (functions, classes, variables, …) defined in a document (textDocument/documentSymbol)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_documentSymbol", req.Params.Arguments)
			}
			result, err := textDocumentDocumentSymbolTool(p, stringArg(req, "filePath"))
			if logger != nil {
				logger.LogResponse("textDocument_documentSymbol", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("workspace_symbolResolve",
			mcp.WithDescription("Resolve additional information (e.g. full location) for a WorkspaceSymbol (workspace/symbolResolve). Pass the JSON object of a WorkspaceSymbol."),
			mcp.WithString("symbol", mcp.Required(), mcp.Description("JSON object of a WorkspaceSymbol.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("workspace_symbolResolve", req.Params.Arguments)
			}
			result, err := workspaceSymbolResolveTool(p, stringArg(req, "symbol"))
			if logger != nil {
				logger.LogResponse("workspace_symbolResolve", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("textDocument_prepareTypeHierarchy",
			mcp.WithDescription("Prepare type hierarchy items for the symbol at a given position; pass an item's JSON to typeHierarchy_supertypes or typeHierarchy_subtypes (textDocument/prepareTypeHierarchy)."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the source file.")),
			mcp.WithNumber("line", mcp.Required(), mcp.Description("1-based line number.")),
			mcp.WithNumber("column", mcp.Required(), mcp.Description("1-based column number.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("textDocument_prepareTypeHierarchy", req.Params.Arguments)
			}
			result, err := textDocumentPrepareTypeHierarchyTool(p,
				stringArg(req, "filePath"),
				intArg(req, "line"),
				intArg(req, "column"),
			)
			if logger != nil {
				logger.LogResponse("textDocument_prepareTypeHierarchy", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("typeHierarchy_supertypes",
			mcp.WithDescription("List all supertypes of a type-hierarchy item (typeHierarchy/supertypes). Pass the JSON object of a TypeHierarchyItem as returned by textDocument_prepareTypeHierarchy."),
			mcp.WithString("item", mcp.Required(), mcp.Description("JSON object of a TypeHierarchyItem.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("typeHierarchy_supertypes", req.Params.Arguments)
			}
			result, err := typeHierarchySupertypesTool(p, stringArg(req, "item"))
			if logger != nil {
				logger.LogResponse("typeHierarchy_supertypes", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("typeHierarchy_subtypes",
			mcp.WithDescription("List all subtypes of a type-hierarchy item (typeHierarchy/subtypes). Pass the JSON object of a TypeHierarchyItem as returned by textDocument_prepareTypeHierarchy."),
			mcp.WithString("item", mcp.Required(), mcp.Description("JSON object of a TypeHierarchyItem.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: true}),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if logger != nil {
				logger.LogRequest("typeHierarchy_subtypes", req.Params.Arguments)
			}
			result, err := typeHierarchySubtypesTool(p, stringArg(req, "item"))
			if logger != nil {
				logger.LogResponse("typeHierarchy_subtypes", result, err)
			}
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)
}

// NewSSEServer creates an SSE-based MCP server and registers all tools.
func NewSSEServer(p LSPClient, port int) *server.SSEServer {
	s := server.NewMCPServer("clangd-mcp", "1.0.0")
	RegisterTools(s, p, nil)
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	return server.NewSSEServer(s, server.WithBaseURL(baseURL))
}

// NewSSEServerWithLogger creates an SSE-based MCP server with debug logging.
func NewSSEServerWithLogger(p LSPClient, port int, logger *SSEDebugLogger) *server.SSEServer {
	s := server.NewMCPServer("clangd-mcp", "1.0.0")
	RegisterTools(s, p, logger)
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	return server.NewSSEServer(s, server.WithBaseURL(baseURL))
}

// ─── Tool implementations ─────────────────────────────────────────────────────

func workspaceSymbolTool(p LSPClient, query string) (string, error) {
	// Strip namespace prefix for a broader search, keep original for filtering.
	searchTerm := query
	if i := strings.LastIndex(query, "::"); i >= 0 {
		searchTerm = query[i+2:]
	}

	params, _ := json.Marshal(map[string]string{"query": searchTerm})
	raw, err := p.SendRequest("workspace/symbol", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return fmt.Sprintf("No symbols found for %q.", query), nil
	}

	symbols, err := parseSymbols(raw)
	if err != nil {
		return "", err
	}
	if len(symbols) == 0 {
		return fmt.Sprintf("No symbols found for %q.", query), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Symbols matching %q (%d):\n", query, len(symbols))
	for _, sym := range symbols {
		path := uriToPath(sym.Location.URI)
		rel := relativePath(p.Workspace(), path)
		fmt.Fprintf(&sb, "  [%s] %s  %s:%d:%d\n",
			symbolKindName(sym.Kind),
			sym.Name,
			rel,
			sym.Location.Range.Start.Line+1,
			sym.Location.Range.Start.Character+1,
		)
	}
	return sb.String(), nil
}

func textDocumentReferencesTool(p LSPClient, filePath string, line, col int) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
		"context":      map[string]bool{"includeDeclaration": true},
	})
	raw, err := p.SendRequest("textDocument/references", params)
	if err != nil {
		return "", err
	}
	locs, err := parseLocations(raw)
	if err != nil {
		return "", err
	}
	return formatLocations(p.Workspace(), locs), nil
}

func textDocumentRenameTool(p LSPClient, filePath string, line, col int, newName string) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
		"newName":      newName,
	})
	raw, err := p.SendRequest("textDocument/rename", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No rename edits returned (symbol may not be renameable at this position).", nil
	}

	edits, err := parseWorkspaceEdit(raw)
	if err != nil {
		return "", fmt.Errorf("parse WorkspaceEdit: %w", err)
	}
	if len(edits) == 0 {
		return "No edits to apply.", nil
	}

	return applyWorkspaceEdits(edits)
}

func textDocumentHoverTool(p LSPClient, filePath string, line, col int) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
	})
	raw, err := p.SendRequest("textDocument/hover", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No hover information available at this position.", nil
	}
	var hover lsp.Hover
	if err := json.Unmarshal(raw, &hover); err != nil {
		return "", fmt.Errorf("hover parse: %w", err)
	}
	return extractHoverText(hover.Contents), nil
}

func textDocumentLocationTool(p LSPClient, method, label, filePath string, line, col int) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
	})
	raw, err := p.SendRequest(method, params)
	if err != nil {
		return "", err
	}
	locs, err := parseLocationsOrLinks(raw)
	if err != nil {
		return "", err
	}
	return formatLocationList(p.Workspace(), locs, label), nil
}

func textDocumentPrepareCallHierarchyTool(p LSPClient, filePath string, line, col int) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
	})
	raw, err := p.SendRequest("textDocument/prepareCallHierarchy", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No call hierarchy items at this position.", nil
	}
	var items []lsp.CallHierarchyItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", fmt.Errorf("prepareCallHierarchy parse: %w", err)
	}
	if len(items) == 0 {
		return "No call hierarchy items at this position.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Call hierarchy items (%d):\n", len(items))
	for i, item := range items {
		rel := relativePath(p.Workspace(), uriToPath(item.URI))
		fmt.Fprintf(&sb, "  [%d] [%s] %s  %s:%d:%d\n",
			i, symbolKindName(item.Kind), item.Name,
			rel, item.SelectionRange.Start.Line+1, item.SelectionRange.Start.Character+1,
		)
		if item.Detail != "" {
			fmt.Fprintf(&sb, "      detail: %s\n", item.Detail)
		}
		itemJSON, _ := json.Marshal(item)
		fmt.Fprintf(&sb, "      json: %s\n", itemJSON)
	}
	return sb.String(), nil
}

func callHierarchyIncomingCallsTool(p LSPClient, itemJSON string) (string, error) {
	params, _ := json.Marshal(map[string]json.RawMessage{
		"item": json.RawMessage(itemJSON),
	})
	raw, err := p.SendRequest("callHierarchy/incomingCalls", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No incoming calls found.", nil
	}
	var calls []lsp.CallHierarchyIncomingCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return "", fmt.Errorf("incomingCalls parse: %w", err)
	}
	if len(calls) == 0 {
		return "No incoming calls found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Incoming calls (%d):\n", len(calls))
	for _, c := range calls {
		rel := relativePath(p.Workspace(), uriToPath(c.From.URI))
		fmt.Fprintf(&sb, "  [%s] %s  %s:%d:%d  (%d call site(s))\n",
			symbolKindName(c.From.Kind), c.From.Name,
			rel, c.From.SelectionRange.Start.Line+1, c.From.SelectionRange.Start.Character+1,
			len(c.FromRanges),
		)
	}
	return sb.String(), nil
}

func callHierarchyOutgoingCallsTool(p LSPClient, itemJSON string) (string, error) {
	params, _ := json.Marshal(map[string]json.RawMessage{
		"item": json.RawMessage(itemJSON),
	})
	raw, err := p.SendRequest("callHierarchy/outgoingCalls", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No outgoing calls found.", nil
	}
	var calls []lsp.CallHierarchyOutgoingCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return "", fmt.Errorf("outgoingCalls parse: %w", err)
	}
	if len(calls) == 0 {
		return "No outgoing calls found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Outgoing calls (%d):\n", len(calls))
	for _, c := range calls {
		rel := relativePath(p.Workspace(), uriToPath(c.To.URI))
		fmt.Fprintf(&sb, "  [%s] %s  %s:%d:%d  (%d call site(s))\n",
			symbolKindName(c.To.Kind), c.To.Name,
			rel, c.To.SelectionRange.Start.Line+1, c.To.SelectionRange.Start.Character+1,
			len(c.FromRanges),
		)
	}
	return sb.String(), nil
}

func textDocumentDocumentSymbolTool(p LSPClient, filePath string) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
	})
	raw, err := p.SendRequest("textDocument/documentSymbol", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No symbols found.", nil
	}

	rel := relativePath(p.Workspace(), filePath)

	// Distinguish DocumentSymbol[] (has "selectionRange") from SymbolInformation[] (has "location").
	if isDocumentSymbolArray(raw) {
		var docSyms []lsp.DocumentSymbol
		if err := json.Unmarshal(raw, &docSyms); err != nil {
			return "", fmt.Errorf("documentSymbol parse: %w", err)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Document symbols in %s (%d):\n", rel, len(docSyms))
		printDocumentSymbols(docSyms, "", &sb)
		return sb.String(), nil
	}

	// Fall back to flat SymbolInformation[].
	symbols, err := parseSymbols(raw)
	if err != nil {
		return "", fmt.Errorf("documentSymbol parse: %w", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Document symbols in %s (%d):\n", rel, len(symbols))
	for _, sym := range symbols {
		fmt.Fprintf(&sb, "  [%s] %s  %d:%d\n",
			symbolKindName(sym.Kind), sym.Name,
			sym.Location.Range.Start.Line+1, sym.Location.Range.Start.Character+1,
		)
	}
	return sb.String(), nil
}

func workspaceSymbolResolveTool(p LSPClient, symbolJSON string) (string, error) {
	raw, err := p.SendRequest("workspace/symbolResolve", json.RawMessage(symbolJSON))
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "Symbol could not be resolved.", nil
	}
	// Pretty-print the resolved symbol.
	var pretty interface{}
	if err := json.Unmarshal(raw, &pretty); err != nil {
		return string(raw), nil
	}
	out, _ := json.MarshalIndent(pretty, "", "  ")
	return string(out), nil
}

func textDocumentPrepareTypeHierarchyTool(p LSPClient, filePath string, line, col int) (string, error) {
	params, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": pathToURI(filePath)},
		"position":     map[string]int{"line": line - 1, "character": col - 1},
	})
	raw, err := p.SendRequest("textDocument/prepareTypeHierarchy", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No type hierarchy items at this position.", nil
	}
	var items []lsp.TypeHierarchyItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", fmt.Errorf("prepareTypeHierarchy parse: %w", err)
	}
	if len(items) == 0 {
		return "No type hierarchy items at this position.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Type hierarchy items (%d):\n", len(items))
	for i, item := range items {
		rel := relativePath(p.Workspace(), uriToPath(item.URI))
		fmt.Fprintf(&sb, "  [%d] [%s] %s  %s:%d:%d\n",
			i, symbolKindName(item.Kind), item.Name,
			rel, item.SelectionRange.Start.Line+1, item.SelectionRange.Start.Character+1,
		)
		if item.Detail != "" {
			fmt.Fprintf(&sb, "      detail: %s\n", item.Detail)
		}
		itemJSON, _ := json.Marshal(item)
		fmt.Fprintf(&sb, "      json: %s\n", itemJSON)
	}
	return sb.String(), nil
}

func typeHierarchySupertypesTool(p LSPClient, itemJSON string) (string, error) {
	params, _ := json.Marshal(map[string]json.RawMessage{
		"item": json.RawMessage(itemJSON),
	})
	raw, err := p.SendRequest("typeHierarchy/supertypes", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No supertypes found.", nil
	}
	var supertypes []lsp.TypeHierarchySupertype
	if err := json.Unmarshal(raw, &supertypes); err != nil {
		return "", fmt.Errorf("supertypes parse: %w", err)
	}
	if len(supertypes) == 0 {
		return "No supertypes found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Supertypes (%d):\n", len(supertypes))
	for _, st := range supertypes {
		rel := relativePath(p.Workspace(), uriToPath(st.Type.URI))
		fmt.Fprintf(&sb, "  [%s] %s  %s:%d:%d\n",
			symbolKindName(st.Type.Kind), st.Type.Name,
			rel, st.Type.SelectionRange.Start.Line+1, st.Type.SelectionRange.Start.Character+1,
		)
	}
	return sb.String(), nil
}

func typeHierarchySubtypesTool(p LSPClient, itemJSON string) (string, error) {
	params, _ := json.Marshal(map[string]json.RawMessage{
		"item": json.RawMessage(itemJSON),
	})
	raw, err := p.SendRequest("typeHierarchy/subtypes", params)
	if err != nil {
		return "", err
	}
	if raw == nil || string(raw) == "null" {
		return "No subtypes found.", nil
	}
	var subtypes []lsp.TypeHierarchySubtype
	if err := json.Unmarshal(raw, &subtypes); err != nil {
		return "", fmt.Errorf("subtypes parse: %w", err)
	}
	if len(subtypes) == 0 {
		return "No subtypes found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Subtypes (%d):\n", len(subtypes))
	for _, st := range subtypes {
		rel := relativePath(p.Workspace(), uriToPath(st.Type.URI))
		fmt.Fprintf(&sb, "  [%s] %s  %s:%d:%d\n",
			symbolKindName(st.Type.Kind), st.Type.Name,
			rel, st.Type.SelectionRange.Start.Line+1, st.Type.SelectionRange.Start.Character+1,
		)
	}
	return sb.String(), nil
}

// ─── WorkspaceEdit application ────────────────────────────────────────────────

type fileEdits struct {
	path  string
	edits []lsp.TextEdit
}

// parseWorkspaceEdit extracts per-file TextEdits from a WorkspaceEdit response.
func parseWorkspaceEdit(raw json.RawMessage) ([]fileEdits, error) {
	var we struct {
		Changes         map[string][]lsp.TextEdit `json:"changes"`
		DocumentChanges []json.RawMessage          `json:"documentChanges"`
	}
	if err := json.Unmarshal(raw, &we); err != nil {
		return nil, err
	}

	result := make([]fileEdits, 0)

	// Prefer documentChanges (versioned).
	if len(we.DocumentChanges) > 0 {
		for _, dc := range we.DocumentChanges {
			var tde struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				Edits []lsp.TextEdit `json:"edits"`
			}
			if err := json.Unmarshal(dc, &tde); err != nil {
				continue // may be a CreateFile/RenameFile/DeleteFile — skip for now
			}
			if tde.TextDocument.URI == "" {
				continue
			}
			result = append(result, fileEdits{
				path:  uriToPath(tde.TextDocument.URI),
				edits: tde.Edits,
			})
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// Fall back to changes map.
	for uri, edits := range we.Changes {
		result = append(result, fileEdits{path: uriToPath(uri), edits: edits})
	}
	// Stable order.
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result, nil
}

// applyWorkspaceEdits writes all edits to disk and returns a summary.
func applyWorkspaceEdits(all []fileEdits) (string, error) {
	var sb strings.Builder
	totalEdits := 0

	for _, fe := range all {
		data, err := os.ReadFile(fe.path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", fe.path, err)
		}
		lines := splitLines(string(data))
		// Apply edits in reverse order so earlier offsets stay valid.
		edits := fe.edits
		sort.Slice(edits, func(i, j int) bool {
			if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
				return edits[i].Range.Start.Line > edits[j].Range.Start.Line
			}
			return edits[i].Range.Start.Character > edits[j].Range.Start.Character
		})
		for _, e := range edits {
			lines = applyTextEdit(lines, e)
		}
		if err := os.WriteFile(fe.path, []byte(strings.Join(lines, "")), 0644); err != nil {
			return "", fmt.Errorf("write %s: %w", fe.path, err)
		}
		fmt.Fprintf(&sb, "  %s (%d edit(s))\n", fe.path, len(fe.edits))
		totalEdits += len(fe.edits)
	}

	header := fmt.Sprintf("Renamed: %d file(s), %d edit(s) applied.\n", len(all), totalEdits)
	return header + sb.String(), nil
}

// splitLines splits text into lines, preserving each line's terminator.
func splitLines(text string) []string {
	var lines []string
	for len(text) > 0 {
		idx := strings.Index(text, "\n")
		if idx < 0 {
			lines = append(lines, text)
			break
		}
		lines = append(lines, text[:idx+1])
		text = text[idx+1:]
	}
	return lines
}

// applyTextEdit applies a single TextEdit to a slice of lines (each with its terminator).
func applyTextEdit(lines []string, e lsp.TextEdit) []string {
	startLine := e.Range.Start.Line
	endLine := e.Range.End.Line
	startChar := e.Range.Start.Character
	endChar := e.Range.End.Character

	if startLine >= len(lines) {
		return lines
	}

	startLineText := lines[startLine]
	prefix := safeSlice(startLineText, 0, startChar)

	var suffix string
	if endLine < len(lines) {
		endLineText := lines[endLine]
		suffix = safeSlice(endLineText, endChar, len(endLineText))
	}

	merged := prefix + e.NewText + suffix

	newLines := make([]string, 0, len(lines)-(endLine-startLine))
	newLines = append(newLines, lines[:startLine]...)
	newLines = append(newLines, merged)
	if endLine+1 < len(lines) {
		newLines = append(newLines, lines[endLine+1:]...)
	}
	return newLines
}

func safeSlice(s string, start, end int) string {
	r := []rune(s)
	if start > len(r) {
		start = len(r)
	}
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

// ─── Parse helpers ────────────────────────────────────────────────────────────

func parseSymbols(raw json.RawMessage) ([]lsp.SymbolInformation, error) {
	var symbols []lsp.SymbolInformation
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("workspace/symbol parse: %w", err)
	}
	return symbols, nil
}

func parseLocationsOrLinks(raw json.RawMessage) ([]lsp.Location, error) {
	if raw == nil || string(raw) == "null" {
		return nil, nil
	}
	// Try []Location.
	var locs []lsp.Location
	if err := json.Unmarshal(raw, &locs); err == nil && len(locs) > 0 && locs[0].URI != "" {
		return locs, nil
	}
	// Try single Location.
	var single lsp.Location
	if err := json.Unmarshal(raw, &single); err == nil && single.URI != "" {
		return []lsp.Location{single}, nil
	}
	// Try []LocationLink.
	var links []lsp.LocationLink
	if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 {
		result := make([]lsp.Location, len(links))
		for i, l := range links {
			result[i] = lsp.Location{URI: l.TargetURI, Range: l.TargetSelectionRange}
		}
		return result, nil
	}
	return nil, nil
}

func parseLocations(raw json.RawMessage) ([]lsp.Location, error) {
	if raw == nil || string(raw) == "null" {
		return nil, nil
	}
	var locs []lsp.Location
	if err := json.Unmarshal(raw, &locs); err != nil {
		var single lsp.Location
		if err2 := json.Unmarshal(raw, &single); err2 == nil {
			return []lsp.Location{single}, nil
		}
		return nil, fmt.Errorf("location parse: %w", err)
	}
	return locs, nil
}

// ─── Formatting helpers ───────────────────────────────────────────────────────

func formatLocationList(workspace string, locs []lsp.Location, label string) string {
	if len(locs) == 0 {
		return fmt.Sprintf("No %s found.", strings.ToLower(label))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s (%d):\n", label, len(locs))
	for _, loc := range locs {
		path := uriToPath(loc.URI)
		rel := relativePath(workspace, path)
		fmt.Fprintf(&sb, "  %s:%d:%d\n", rel, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
	}
	return sb.String()
}

// extractHoverText converts any LSP hover contents variant to plain text.
func extractHoverText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	// MarkupContent: {"kind":"markdown","value":"..."}
	var mc struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &mc); err == nil && mc.Value != "" {
		return mc.Value
	}
	// Plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Array of MarkedString/MarkupContent.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			parts = append(parts, extractHoverText(item))
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

// isDocumentSymbolArray returns true when raw looks like DocumentSymbol[] rather than SymbolInformation[].
// DocumentSymbol has "selectionRange"; SymbolInformation has "location".
func isDocumentSymbolArray(raw json.RawMessage) bool {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(arr[0], &obj); err != nil {
		return false
	}
	_, has := obj["selectionRange"]
	return has
}

// printDocumentSymbols recursively writes a DocumentSymbol tree to sb.
func printDocumentSymbols(syms []lsp.DocumentSymbol, indent string, sb *strings.Builder) {
	for _, sym := range syms {
		fmt.Fprintf(sb, "%s[%s] %s", indent, symbolKindName(sym.Kind), sym.Name)
		if sym.Detail != "" {
			fmt.Fprintf(sb, "  (%s)", sym.Detail)
		}
		fmt.Fprintf(sb, "  %d:%d\n", sym.Range.Start.Line+1, sym.Range.Start.Character+1)
		if len(sym.Children) > 0 {
			printDocumentSymbols(sym.Children, indent+"  ", sb)
		}
	}
}

func formatLocations(workspace string, locs []lsp.Location) string {
	if len(locs) == 0 {
		return "No references found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "References (%d):\n", len(locs))
	for _, loc := range locs {
		path := uriToPath(loc.URI)
		rel := relativePath(workspace, path)
		fmt.Fprintf(&sb, "  %s:%d:%d\n", rel, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
	}
	return sb.String()
}

func symbolKindName(kind int) string {
	names := map[int]string{
		1: "File", 2: "Module", 3: "Namespace", 4: "Package",
		5: "Class", 6: "Method", 7: "Property", 8: "Field",
		9: "Constructor", 10: "Enum", 11: "Interface", 12: "Function",
		13: "Variable", 14: "Constant", 15: "String", 16: "Number",
		17: "Boolean", 18: "Array", 19: "Object", 20: "Key",
		21: "Null", 22: "EnumMember", 23: "Struct", 24: "Event",
		25: "Operator", 26: "TypeParameter",
	}
	if n, ok := names[kind]; ok {
		return n
	}
	return "Symbol"
}

// ─── URI / path helpers ───────────────────────────────────────────────────────

func pathToURI(path string) string {
	abs, _ := filepath.Abs(path)
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return "file://" + abs
}

func uriToPath(uri string) string {
	path := strings.TrimPrefix(uri, "file://")
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

func relativePath(workspace, path string) string {
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return path
	}
	return rel
}

// ─── Argument helpers ─────────────────────────────────────────────────────────

func stringArg(req mcp.CallToolRequest, name string) string {
	if v, ok := req.Params.Arguments[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intArg(req mcp.CallToolRequest, name string) int {
	if v, ok := req.Params.Arguments[name]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}
