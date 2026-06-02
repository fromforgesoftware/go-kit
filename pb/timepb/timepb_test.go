package timepb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fromforgesoftware/go-kit/pb/timepb"
)

func TestTimePointerToTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("nil yields nil", func(t *testing.T) {
		assert.Nil(t, timepb.TimePointerToTimestamp(nil))
	})

	t.Run("zero yields nil", func(t *testing.T) {
		zero := time.Time{}
		assert.Nil(t, timepb.TimePointerToTimestamp(&zero))
	})

	t.Run("non-zero round trips", func(t *testing.T) {
		now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
		ts := timepb.TimePointerToTimestamp(&now)
		assert.True(t, ts.AsTime().Equal(now))
	})
}

func TestTimestampToTimePointer(t *testing.T) {
	t.Parallel()

	t.Run("nil yields nil", func(t *testing.T) {
		assert.Nil(t, timepb.TimestampToTimePointer(nil))
	})

	t.Run("valid round trips", func(t *testing.T) {
		now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
		got := timepb.TimestampToTimePointer(timestamppb.New(now))
		assert.NotNil(t, got)
		assert.True(t, got.Equal(now))
	})
}
