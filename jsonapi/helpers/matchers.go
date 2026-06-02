package helpers

import (
	"time"
)

func MatchNullableTime(want *time.Time) func(*time.Time) bool {
	return func(got *time.Time) bool {
		return (want == nil && got == nil) ||
			want.Equal(*got)
	}
}
