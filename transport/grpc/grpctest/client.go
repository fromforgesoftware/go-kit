package grpctest

import (
	"context"
	"log"
	"net"
	"testing"

	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

type serverConfig struct {
	clientOptions []grpc.DialOption
}

type serverConfigOption func(*serverConfig)

func WithClientOptions(opts ...grpc.DialOption) serverConfigOption {
	return func(cfg *serverConfig) {
		cfg.clientOptions = append(cfg.clientOptions, opts...)
	}
}

func NewServer(t *testing.T, controller transportgrpc.Controller, opts ...serverConfigOption) *grpc.ClientConn {
	t.Helper()

	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Use real TCP listener to avoid bufconn deadlocks
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	s.RegisterService(controller.SD(), controller)
	reflection.Register(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			// Do not fatal here as it kills the test process. Just log.
			// s.Serve returns nil on Stop(), so this only catches actual errors.
			log.Printf("Server exited with error: %v", err)
		}
	}()

	// No custom dialer needed for real TCP
	cfg.clientOptions = append(
		cfg.clientOptions,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	conn, err := grpc.NewClient(lis.Addr().String(), cfg.clientOptions...)
	assert.NoError(t, err)

	t.Cleanup(func() {
		// Close client connection first to ensure no new requests specific to this test
		// log.Printf("Closing client connection...")
		_ = conn.Close()

		// Explicitly close listener to ensure Serve returns immediately
		// log.Printf("Closing listener...")
		_ = lis.Close()

		// Stop the server in a goroutine to prevent blocking if it waits for connections
		// log.Printf("Stopping server...")
		go s.Stop()
	})

	return conn
}

func ClientWithAuthInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Apply client middleware
		ctx = transportgrpc.ClientAuthMiddleware()(ctx)

		// Call the original invoker with the modified context
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}
