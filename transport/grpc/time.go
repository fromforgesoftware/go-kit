package grpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fromforgesoftware/go-kit/pb/timepb"
)

// TimePointerToTimestamp is deprecated: use kit/pb/timepb.TimePointerToTimestamp.
func TimePointerToTimestamp(t *time.Time) *timestamppb.Timestamp {
	return timepb.TimePointerToTimestamp(t)
}

// TimestampToTimePointer is deprecated: use kit/pb/timepb.TimestampToTimePointer.
func TimestampToTimePointer(t *timestamppb.Timestamp) *time.Time {
	return timepb.TimestampToTimePointer(t)
}
