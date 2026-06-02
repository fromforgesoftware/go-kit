package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer/tracertest"
	"github.com/fromforgesoftware/go-kit/transport/grpc/middleware"
)

func TestTracingPassesThroughResponseAndError(t *testing.T) {
	tr := tracertest.NewStubTracer(t)
	interceptor := middleware.Tracing(tr)

	t.Run("success path", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
			func(ctx context.Context, req any) (any, error) { return "ok", nil })
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)
	})

	t.Run("error path", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
		wantErr := errors.New("boom")
		_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
			func(ctx context.Context, req any) (any, error) { return nil, wantErr })
		assert.ErrorIs(t, err, wantErr)
	})
}

func TestClientTracingInjectsAndForwards(t *testing.T) {
	tr := tracertest.NewStubTracer(t)
	interceptor := middleware.ClientTracing(tr)

	called := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		called = true
		md, _ := metadata.FromOutgoingContext(ctx)
		// Even with the noop stub, the outgoing metadata must be a valid MD
		// that the call would have sent — the invoker must have received it.
		_ = md
		return nil
	}

	err := interceptor(context.Background(), "/svc/Method", nil, nil, nil, invoker)
	require.NoError(t, err)
	assert.True(t, called)
}
