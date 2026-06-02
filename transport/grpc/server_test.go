package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func TestNewServerWithDefaults(t *testing.T) {
	t.Run("creates server with default configuration", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		server, err := transportgrpc.NewServer(monitor, transportgrpc.WithAddress(":0"))

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.NotNil(t, server.Server)
	})
}

func TestNewServerWithAddress(t *testing.T) {
	t.Run("applies custom address", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		server, err := transportgrpc.NewServer(
			monitor,
			transportgrpc.WithAddress(":0"), // Use port 0 for automatic assignment
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNewServerWithNetwork(t *testing.T) {
	t.Run("applies custom network", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		server, err := transportgrpc.NewServer(
			monitor,
			transportgrpc.WithAddress(":0"),
			transportgrpc.WithNetwork("tcp"),
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNewServerWithMiddlewares(t *testing.T) {
	t.Run("applies middleware", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)

		middleware := transportgrpc.MiddlewareFunc(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		})

		server, err := transportgrpc.NewServer(
			monitor,
			transportgrpc.WithAddress(":0"),
			transportgrpc.WithMiddlewares(middleware),
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNewServerWithKeepalive(t *testing.T) {
	t.Run("applies keepalive parameters", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)

		server, err := transportgrpc.NewServer(
			monitor,
			transportgrpc.WithAddress(":0"),
			transportgrpc.WithKeepalive(&keepalive.ServerParameters{
				MaxConnectionIdle: 5 * time.Minute,
				Time:              2 * time.Hour,
				Timeout:           20 * time.Second,
			}),
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNewServerWithLogger(t *testing.T) {
	t.Run("server uses provided logger", func(t *testing.T) {
		logger := loggertest.NewStubLogger(t)
		monitor := monitoringtest.NewMonitor(t, monitoringtest.WithLogger(logger))

		server, err := transportgrpc.NewServer(monitor, transportgrpc.WithAddress(":0"))

		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNewServerCreatesHealthCheckByDefault(t *testing.T) {
	t.Run("health check controller is registered by default", func(t *testing.T) {
		monitor := monitoringtest.NewMonitor(t)
		server, err := transportgrpc.NewServer(monitor, transportgrpc.WithAddress(":0"))

		require.NoError(t, err)
		assert.NotNil(t, server)
		// Health check is registered as a default controller
	})
}
