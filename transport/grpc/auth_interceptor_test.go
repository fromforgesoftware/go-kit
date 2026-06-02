package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fromforgesoftware/go-kit/auth/authtest"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
)

func TestAuthInterceptorSuccessForwardsAuthenticatedContext(t *testing.T) {
	authenticator := authtest.NewStrictGrpcAuthenticator("test-user")
	interceptor := transportgrpc.AuthInterceptor(authenticator)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		"authorization": "Bearer valid-token",
	}))

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called, "handler should run after successful auth")
}

func TestAuthInterceptorFailureReturnsErrorWithoutCallingHandler(t *testing.T) {
	authenticator := authtest.NewStrictGrpcAuthenticator("test-user")
	interceptor := transportgrpc.AuthInterceptor(authenticator)

	ctx := context.Background()
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not run when auth fails")
		return nil, nil
	}

	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	assert.Nil(t, resp)
	require.Error(t, err)

	// Default error handler maps to codes.Unauthenticated.
	st, ok := status.FromError(err)
	require.True(t, ok, "auth error should be a gRPC status error")
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptorRespectsCustomErrorHandler(t *testing.T) {
	authenticator := authtest.NewStrictGrpcAuthenticator("test-user")
	custom := errors.New("custom auth error")
	interceptor := transportgrpc.AuthInterceptor(authenticator,
		transportgrpc.WithAuthErrorHandler(func(err error) error { return custom }),
	)

	ctx := context.Background()
	handler := func(ctx context.Context, req any) (any, error) { return nil, nil }

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	assert.ErrorIs(t, err, custom)
}
