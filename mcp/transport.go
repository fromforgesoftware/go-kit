package mcp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Transport abstracts the wire on which the server speaks MCP.
// Implementations are produced by Stdio(), HTTP(), etc.
type Transport interface {
	serve(ctx context.Context, mcp *server.MCPServer) error
}

// Stdio returns the standard MCP transport — JSON-RPC framed over
// stdin/stdout. The default transport when no other is specified.
func Stdio() Transport { return stdioTransport{} }

type stdioTransport struct{}

func (stdioTransport) serve(ctx context.Context, mcp *server.MCPServer) error {
	srv := server.NewStdioServer(mcp)
	// mcp-go's ServeStdio doesn't accept a context directly; we wrap
	// it by closing stdin on ctx.Done. The library will return as the
	// stream EOFs.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Listen(ctx, nil, nil)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// HTTP returns the streamable-HTTP transport bound to `addr`. The
// resulting server is shut down gracefully when the supplied context
// is cancelled.
func HTTP(addr string) Transport { return httpTransport{addr: addr} }

type httpTransport struct {
	addr string
}

// ErrHTTPClosed is returned by HTTP transport when the server stops
// because the context was cancelled. Wraps net/http's
// ErrServerClosed semantics so callers can branch cleanly.
var ErrHTTPClosed = errors.New("mcp: HTTP transport closed")

func (h httpTransport) serve(ctx context.Context, mcp *server.MCPServer) error {
	srv := server.NewStreamableHTTPServer(mcp)
	httpSrv := &http.Server{
		Addr:              h.addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		err := httpSrv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		<-errCh
		return ErrHTTPClosed
	case err := <-errCh:
		return err
	}
}
