package idgen_test

import (
	"sync"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/distributed/idgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	t.Run("creates generator with valid region ID", func(t *testing.T) {
		gen := idgen.NewGenerator(1)
		require.NotNil(t, gen)
		assert.Equal(t, int64(1), gen.RegionID())
	})

	t.Run("panics with negative region ID", func(t *testing.T) {
		assert.Panics(t, func() {
			idgen.NewGenerator(-1)
		})
	})

	t.Run("panics with region ID too large", func(t *testing.T) {
		assert.Panics(t, func() {
			idgen.NewGenerator(1024) // Max is 1023
		})
	})
}

func TestNextID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		gen := idgen.NewGenerator(1)

		ids := make(map[int64]bool)
		for i := 0; i < 1000; i++ {
			id := gen.NextID()
			assert.False(t, ids[id], "duplicate ID generated: %d", id)
			ids[id] = true
		}
	})

	t.Run("IDs are monotonically increasing", func(t *testing.T) {
		gen := idgen.NewGenerator(1)

		prev := gen.NextID()
		for i := 0; i < 100; i++ {
			curr := gen.NextID()
			assert.Greater(t, curr, prev, "ID should be greater than previous")
			prev = curr
		}
	})

	t.Run("different regions produce different IDs", func(t *testing.T) {
		gen1 := idgen.NewGenerator(1)
		gen2 := idgen.NewGenerator(2)

		id1 := gen1.NextID()
		id2 := gen2.NextID()

		assert.NotEqual(t, id1, id2, "IDs from different regions should differ")

		// Verify region IDs are encoded correctly
		_, region1, _ := idgen.ParseID(id1)
		_, region2, _ := idgen.ParseID(id2)
		assert.Equal(t, int64(1), region1)
		assert.Equal(t, int64(2), region2)
	})

	t.Run("concurrent generation is safe", func(t *testing.T) {
		gen := idgen.NewGenerator(1)
		const goroutines = 10
		const idsPerGoroutine = 100

		ids := make(chan int64, goroutines*idsPerGoroutine)
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < idsPerGoroutine; j++ {
					ids <- gen.NextID()
				}
			}()
		}

		wg.Wait()
		close(ids)

		// Check all IDs are unique
		seen := make(map[int64]bool)
		for id := range ids {
			assert.False(t, seen[id], "duplicate ID in concurrent test")
			seen[id] = true
		}
		assert.Len(t, seen, goroutines*idsPerGoroutine)
	})
}

func TestParseID(t *testing.T) {
	t.Run("correctly parses generated ID", func(t *testing.T) {
		gen := idgen.NewGenerator(42)
		id := gen.NextID()

		timestamp, regionID, sequence := idgen.ParseID(id)

		assert.Equal(t, int64(42), regionID)
		assert.GreaterOrEqual(t, timestamp, int64(0))
		assert.GreaterOrEqual(t, sequence, int64(0))
		assert.LessOrEqual(t, sequence, int64(4095))
	})

	t.Run("timestamp is reasonable", func(t *testing.T) {
		gen := idgen.NewGenerator(1)
		id := gen.NextID()

		timestamp, _, _ := idgen.ParseID(id)

		// Timestamp should be within a few seconds of now
		now := time.Now().UnixMilli()
		diff := now - timestamp
		assert.LessOrEqual(t, diff, int64(5000), "timestamp should be recent")
	})
}

func BenchmarkNextID(b *testing.B) {
	gen := idgen.NewGenerator(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NextID()
	}
}

func BenchmarkNextIDParallel(b *testing.B) {
	gen := idgen.NewGenerator(1)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gen.NextID()
		}
	})
}
