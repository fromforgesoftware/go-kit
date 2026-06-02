package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FromGRPCError extracts our custom error from a gRPC status error
func FromGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// Get the status from the error
	s, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, return as is
		return err
	}

	// Extract the error message which should contain our original error
	msg := s.Message()

	// Check for known error types based on the message and code
	switch s.Code() {
	case codes.NotFound:
		return NotFound("", "", WithMessage(msg))
	case codes.InvalidArgument:
		return InvalidArgument(msg)
	case codes.AlreadyExists:
		return AlreadyExists("", "", WithMessage(msg))
	case codes.PermissionDenied:
		return Forbidden(msg)
	case codes.Unauthenticated:
		return Unauthenticated(msg)
	case codes.Internal:
		fallthrough
	default:
		return InternalError(msg)
	}
}
