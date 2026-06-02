package grpc

import (
	"context"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// healthServer implements the standard gRPC Health Checking Protocol
// https://github.com/grpc/grpc/blob/master/doc/health-checking.md
type healthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	monitor monitoring.Monitor
}

// NewHealthServer creates a new health check server
// It always returns SERVING status since the server is running
func NewHealthServer(monitor monitoring.Monitor) *healthServer {
	return &healthServer{
		monitor: monitor,
	}
}

// Check implements the health check RPC
// Returns SERVING status if the service is healthy
func (s *healthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// Always return SERVING - if the server is running, it's healthy
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch implements the health check streaming RPC
// Immediately sends SERVING status and keeps the stream open
func (s *healthServer) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	// Send initial SERVING status
	if err := stream.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}); err != nil {
		return err
	}

	// Keep stream open - in a real implementation, you'd monitor service health
	// and send updates when status changes
	<-stream.Context().Done()
	return stream.Context().Err()
}

// SD returns the service descriptor for registration
func (s *healthServer) SD() ServiceDesc {
	return &grpc_health_v1.Health_ServiceDesc
}
