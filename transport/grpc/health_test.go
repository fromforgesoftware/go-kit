package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthService(t *testing.T) {
	tests := []struct {
		name           string
		service        string
		expectedStatus grpc_health_v1.HealthCheckResponse_ServingStatus
	}{
		{
			name:           "reports serving status for empty service name",
			service:        "",
			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
		{
			name:           "reports serving status for specific service",
			service:        "test.Service",
			expectedStatus: grpc_health_v1.HealthCheckResponse_SERVING,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := monitoringtest.NewMonitor(t)
			healthService := transportgrpc.NewHealthServer(monitor)

			req := &grpc_health_v1.HealthCheckRequest{
				Service: tt.service,
			}

			resp, err := healthService.Check(context.Background(), req)

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.Status)
		})
	}
}

func TestHealthServiceWatch(t *testing.T) {
	t.Run("watch sends initial serving status", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		healthService := transportgrpc.NewHealthServer(monitor)

		req := &grpc_health_v1.HealthCheckRequest{
			Service: "",
		}

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a mock watch server
		mockServer := &mockHealthWatchServer{
			ctx: ctx,
		}

		// Run Watch in a goroutine since it blocks
		done := make(chan error, 1)
		go func() {
			done <- healthService.Watch(req, mockServer)
		}()

		// Give it a moment to send the initial status
		time.Sleep(50 * time.Millisecond)

		// Cancel context to stop watching
		cancel()

		// Wait for Watch to return
		err := <-done

		// Watch should return context.Canceled error
		assert.Error(t, err)
		assert.True(t, mockServer.sendCalled, "Watch should send status update")
		if mockServer.lastResponse != nil {
			assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, mockServer.lastResponse.Status)
		}
	})
}

// Mock types for health service testing

type mockHealthWatchServer struct {
	grpc_health_v1.Health_WatchServer
	ctx          context.Context
	sendCalled   bool
	lastResponse *grpc_health_v1.HealthCheckResponse
}

func (m *mockHealthWatchServer) Send(resp *grpc_health_v1.HealthCheckResponse) error {
	m.sendCalled = true
	m.lastResponse = resp
	return nil
}

func (m *mockHealthWatchServer) Context() context.Context {
	return m.ctx
}
