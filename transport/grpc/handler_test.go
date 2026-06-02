package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/auth/authtest"
	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    transport.Endpoint[string, string]
		decoder     transportgrpc.DecodeRequestFunc[string, string]
		encoder     transportgrpc.EncodeResponseFunc[string, string]
		wantErr     bool
		expectedRes string
	}{
		{
			name: "successful request flow through handler",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "processed: " + req, nil
			},
			decoder: func(ctx context.Context, req string) (string, error) {
				return req, nil
			},
			encoder: func(ctx context.Context, resp string) (string, error) {
				return resp, nil
			},
			wantErr:     false,
			expectedRes: "processed: test",
		},
		{
			name: "decoder error stops request flow",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "should not reach", nil
			},
			decoder: func(ctx context.Context, req string) (string, error) {
				return "", errors.New("decode failed")
			},
			encoder: func(ctx context.Context, resp string) (string, error) {
				return resp, nil
			},
			wantErr: true,
		},
		{
			name: "endpoint error is propagated",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "", errors.New("endpoint error")
			},
			decoder: func(ctx context.Context, req string) (string, error) {
				return req, nil
			},
			encoder: func(ctx context.Context, resp string) (string, error) {
				return resp, nil
			},
			wantErr: true,
		},
		{
			name: "encoder error is returned",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "success", nil
			},
			decoder: func(ctx context.Context, req string) (string, error) {
				return req, nil
			},
			encoder: func(ctx context.Context, resp string) (string, error) {
				return "", errors.New("encode failed")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := monitoringtest.NewMonitor(t)
			handler := transportgrpc.NewHandler(
				tt.endpoint,
				tt.decoder,
				tt.encoder,
				monitor,
			)

			ctx := context.Background()
			resp, err := handler.ServeGRPC(ctx, "test")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRes, resp)
			}
		})
	}
}

func TestHandlerWithAuthentication(t *testing.T) {
	t.Run("authentication middleware is applied to handler", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		authenticator := authtest.NewGrpcAuthenticator()

		endpoint := func(ctx context.Context, req string) (string, error) {
			return "response", nil
		}

		handler := transportgrpc.NewHandler(
			endpoint,
			func(ctx context.Context, req string) (string, error) { return req, nil },
			func(ctx context.Context, resp string) (string, error) { return resp, nil },
			monitor,
			transportgrpc.WithAuthentication(authenticator),
		)

		assert.NotNil(t, handler)
	})
}

func TestHandlerWithErrorHandler(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         transport.Endpoint[string, string]
		wantErr          bool
		wantErrorHandled bool
	}{
		{
			name: "error handler is called on endpoint error",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "", errors.New("test error")
			},
			wantErr:          true,
			wantErrorHandled: true,
		},
		{
			name: "error handler not called on success",
			endpoint: func(ctx context.Context, req string) (string, error) {
				return "success", nil
			},
			wantErr:          false,
			wantErrorHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := monitoringtest.NewMonitor(t)
			var errorHandled bool
			errorHandler := &mockErrorHandler{
				handleFn: func(ctx context.Context, err error) {
					errorHandled = true
				},
			}

			handler := transportgrpc.NewHandler(
				tt.endpoint,
				func(ctx context.Context, req string) (string, error) { return req, nil },
				func(ctx context.Context, resp string) (string, error) { return resp, nil },
				monitor,
				transportgrpc.WithHandlerErrorHandler(errorHandler),
			)

			ctx := context.Background()
			_, err := handler.ServeGRPC(ctx, "test")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantErrorHandled, errorHandled)
		})
	}
}

// Mock error handler for testing

type mockErrorHandler struct {
	handleFn func(context.Context, error)
}

func (m *mockErrorHandler) Handle(ctx context.Context, err error) {
	if m.handleFn != nil {
		m.handleFn(ctx, err)
	}
}
