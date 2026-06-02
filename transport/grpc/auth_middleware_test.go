package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/auth/authtest"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		metadata metadata.MD
		wantErr  bool
	}{
		{
			name:     "successful authentication with valid token",
			metadata: metadata.New(map[string]string{"authorization": "Bearer valid-token"}),
			wantErr:  false,
		},
		{
			name:     "authentication requires authorization header",
			metadata: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := authtest.NewStrictGrpcAuthenticator("test-user")
			middleware := transportgrpc.NewAuthMiddleware(authenticator)

			// Create a test handler
			handlerCalled := false
			testHandler := transportgrpc.HandlerFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return "success", nil
			})

			// Apply middleware
			wrappedHandler := middleware.Intercept(testHandler)

			// Create context with metadata
			ctx := context.Background()
			if tt.metadata != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.metadata)
			}

			// Execute
			resp, err := wrappedHandler.ServeGRPC(ctx, "test-request")

			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, handlerCalled)
			} else {
				require.NoError(t, err)
				assert.True(t, handlerCalled)
				assert.Equal(t, "success", resp)
			}
		})
	}
}

func TestAuthMiddlewareWithErrorHandler(t *testing.T) {
	t.Run("custom error handler can transform errors", func(t *testing.T) {
		authenticator := authtest.NewStrictGrpcAuthenticator("test-user")

		var errorHandled bool
		customHandler := func(err error) error {
			errorHandled = true
			return errors.New("custom: " + err.Error())
		}

		middleware := transportgrpc.NewAuthMiddleware(
			authenticator,
			transportgrpc.WithAuthErrorHandler(customHandler),
		)

		testHandler := transportgrpc.HandlerFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
			return "should not reach", nil
		})

		wrappedHandler := middleware.Intercept(testHandler)
		ctx := context.Background() // No auth metadata

		_, err := wrappedHandler.ServeGRPC(ctx, "test")

		require.Error(t, err)
		assert.True(t, errorHandled)
		assert.Contains(t, err.Error(), "custom:")
	})
}

func TestClientAuthMiddleware(t *testing.T) {
	t.Run("adds authorization middleware context", func(t *testing.T) {
		middleware := transportgrpc.ClientAuthMiddleware()
		ctx := context.Background()

		// Apply middleware
		newCtx := middleware(ctx)

		// Verify context is returned
		assert.NotNil(t, newCtx)
	})
}
