package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/fromforgesoftware/go-kit/mcp"
)

type echoIn struct {
	Message string `json:"message" desc:"text to echo"`
	Loud    bool   `json:"loud,omitempty"`
}

type echoOut struct {
	Reply string `json:"reply"`
}

func newClient(t *testing.T, s *mcp.Server) *mcpclient.Client {
	t.Helper()
	c, err := mcpclient.NewInProcessClient(s.MCP())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	require.NoError(t, c.Start(context.Background()))
	initReq := mcpgo.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpgo.Implementation{Name: "test", Version: "0"}
	_, err = c.Initialize(context.Background(), initReq)
	require.NoError(t, err)
	return c
}

func callTool(t *testing.T, c *mcpclient.Client, name string, args map[string]any) *mcpgo.CallToolResult {
	t.Helper()
	req := mcpgo.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := c.CallTool(context.Background(), req)
	require.NoError(t, err)
	return res
}

func TestRegisterAndCallTypedTool(t *testing.T) {
	s := mcp.New(mcp.Config{Name: "test", Version: "1.0"})
	mcp.Register(s, mcp.Tool[echoIn, echoOut]{
		Name:        "echo",
		Description: "Echoes back the input.",
		Handler: func(_ context.Context, in echoIn) (echoOut, error) {
			reply := in.Message
			if in.Loud {
				reply = strings.ToUpper(reply)
			}
			return echoOut{Reply: reply}, nil
		},
	})

	c := newClient(t, s)
	res := callTool(t, c, "echo", map[string]any{"message": "hi", "loud": true})
	assert.False(t, res.IsError)
	require.NotEmpty(t, res.Content)
	var out echoOut
	require.NotNil(t, res.StructuredContent)
	raw, _ := json.Marshal(res.StructuredContent)
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "HI", out.Reply)
}

func TestToolHandlerErrorMapsToToolError(t *testing.T) {
	s := mcp.New(mcp.Config{Name: "test", Version: "1.0"})
	mcp.Register(s, mcp.Tool[echoIn, echoOut]{
		Name:        "boom",
		Description: "Always fails.",
		Handler: func(_ context.Context, _ echoIn) (echoOut, error) {
			return echoOut{}, errors.New("simulated failure")
		},
	})
	c := newClient(t, s)
	res := callTool(t, c, "boom", map[string]any{"message": "x"})
	assert.True(t, res.IsError)
}

func TestRegisterPanicsOnMissingName(t *testing.T) {
	s := mcp.New(mcp.Config{Name: "test"})
	assert.Panics(t, func() {
		mcp.Register(s, mcp.Tool[echoIn, echoOut]{Handler: func(_ context.Context, _ echoIn) (echoOut, error) { return echoOut{}, nil }})
	})
}

func TestRegisterPanicsOnMissingHandler(t *testing.T) {
	s := mcp.New(mcp.Config{Name: "test"})
	assert.Panics(t, func() {
		mcp.Register(s, mcp.Tool[echoIn, echoOut]{Name: "x"})
	})
}

func TestNewPanicsOnMissingName(t *testing.T) {
	assert.Panics(t, func() { mcp.New(mcp.Config{}) })
}

func TestListToolsReturnsRegisteredSchema(t *testing.T) {
	s := mcp.New(mcp.Config{Name: "test"})
	mcp.Register(s, mcp.Tool[echoIn, echoOut]{
		Name:        "echo",
		Description: "echo",
		Handler:     func(_ context.Context, in echoIn) (echoOut, error) { return echoOut{Reply: in.Message}, nil },
	})
	c := newClient(t, s)
	res, err := c.ListTools(context.Background(), mcpgo.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)
	assert.Equal(t, "echo", res.Tools[0].Name)
	// The raw schema flows through into a real JSON Schema with
	// `message` required (no omitempty) and `loud` not required.
	schemaJSON, _ := json.Marshal(res.Tools[0])
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(schemaJSON, &decoded))
	is, ok := decoded["inputSchema"].(map[string]any)
	require.True(t, ok, "inputSchema present, got %T", decoded["inputSchema"])
	required, _ := is["required"].([]any)
	assert.Contains(t, required, "message")
	assert.NotContains(t, required, "loud")
}
