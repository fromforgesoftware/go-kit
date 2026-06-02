package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/server"
)

// Config configures a Server.
type Config struct {
	// Name is the server name advertised to clients.
	Name string
	// Version is the server version advertised to clients.
	Version string
	// Transport selects the wire protocol. Defaults to Stdio.
	Transport Transport
}

// Server is a typed MCP server. Internally it owns a mark3labs/mcp-go
// MCPServer; the wrapper exists so consumers don't bind to mcp-go's
// API directly and so we can add forge concerns (typed registration,
// schema generation, lifecycle integration) without forking upstream.
type Server struct {
	cfg Config
	mcp *server.MCPServer
}

// New constructs a Server. Panics if Name is empty.
func New(cfg Config) *Server {
	if cfg.Name == "" {
		panic("mcp: Config.Name is required")
	}
	if cfg.Version == "" {
		cfg.Version = "0.0.0"
	}
	if cfg.Transport == nil {
		cfg.Transport = Stdio()
	}
	return &Server{
		cfg: cfg,
		mcp: server.NewMCPServer(cfg.Name, cfg.Version),
	}
}

// Run starts serving over the configured transport until ctx is
// cancelled or the transport returns an error.
func (s *Server) Run(ctx context.Context) error {
	return s.cfg.Transport.serve(ctx, s.mcp)
}

// MCP returns the underlying mcp-go server for advanced callers that
// need direct access (custom hooks, resources, prompts). Most code
// shouldn't need this — register tools via the typed Register helper.
func (s *Server) MCP() *server.MCPServer { return s.mcp }
