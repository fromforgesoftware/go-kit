// Package timepb provides conversions between Go time.Time and the
// protobuf Timestamp type. It lives outside any transport package so
// every consumer can import it without pulling in transport code.
package timepb

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// TimePointerToTimestamp converts a *time.Time to a *timestamppb.Timestamp,
// returning nil for nil or zero-time input so the wire encoding omits the
// field entirely.
func TimePointerToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// TimestampToTimePointer converts a *timestamppb.Timestamp to *time.Time,
// returning nil for nil / invalid / zero inputs.
func TimestampToTimePointer(t *timestamppb.Timestamp) *time.Time {
	if t == nil || !t.IsValid() {
		return nil
	}
	out := t.AsTime()
	if out.IsZero() {
		return nil
	}
	return &out
}
