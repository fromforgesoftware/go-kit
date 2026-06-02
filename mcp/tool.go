package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tool is a typed MCP tool. The input/output schemas exposed to
// clients are derived from In and Out via reflection (see SchemaFor).
// The handler is invoked with the decoded In and its returned Out is
// JSON-marshalled into a structured CallToolResult.
type Tool[In, Out any] struct {
	// Name is the MCP tool identifier. Required.
	Name string
	// Description is shown to MCP clients (and to the LLM). Required.
	Description string
	// Handler runs the tool. Errors are mapped to MCP error responses.
	Handler func(ctx context.Context, in In) (Out, error)
}

// Register installs a typed Tool on a Server.
func Register[In, Out any](s *Server, t Tool[In, Out]) {
	if t.Name == "" {
		panic("mcp: Tool.Name is required")
	}
	if t.Handler == nil {
		panic("mcp: Tool.Handler is required for " + t.Name)
	}
	schema := SchemaFor[In]()
	tool := mcp.NewToolWithRawSchema(t.Name, t.Description, schema)
	s.mcp.AddTool(tool, wrapHandler(t.Handler))
}

func wrapHandler[In, Out any](h func(ctx context.Context, in In) (Out, error)) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in In
		// BindArguments is forgiving: missing inputs leave fields at
		// their zero value. The user's handler decides what to do.
		if err := req.BindArguments(&in); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %s", err)), nil
		}
		out, err := h(ctx, in)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("tool execution failed", err), nil
		}
		// Render the typed output as both structured + text fallback so
		// non-structured-aware clients still see something useful.
		fallback := fmt.Sprintf("%+v", out)
		return mcp.NewToolResultStructured(out, fallback), nil
	}
}
