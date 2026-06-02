package extractor_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/extractor"
)

func TestWorkerProcessesAllUnits(t *testing.T) {
	var done atomic.Int64
	w, err := extractor.NewWorker(extractor.WorkerConfig[int, int]{
		Concurrency: 4,
		Process: func(_ context.Context, n int) (int, error) {
			done.Add(1)
			return n * 2, nil
		},
		OnResult: func(_ int, r int) { _ = r },
	})
	require.NoError(t, err)
	units := []int{1, 2, 3, 4, 5, 6, 7, 8}
	require.NoError(t, w.Run(context.Background(), units))
	assert.Equal(t, int64(8), done.Load())
}

func TestWorkerAbortOnError(t *testing.T) {
	target := errors.New("boom")
	w, err := extractor.NewWorker(extractor.WorkerConfig[int, int]{
		Process: func(_ context.Context, n int) (int, error) {
			if n == 3 {
				return 0, target
			}
			return n, nil
		},
		OnError: func(_ int, _ error) extractor.Recovery { return extractor.Abort },
	})
	require.NoError(t, err)
	err = w.Run(context.Background(), []int{1, 2, 3, 4})
	assert.ErrorIs(t, err, extractor.ErrAborted)
	assert.ErrorIs(t, err, target)
}

func TestWorkerSkipOnError(t *testing.T) {
	var processed atomic.Int64
	w, err := extractor.NewWorker(extractor.WorkerConfig[int, int]{
		Process: func(_ context.Context, n int) (int, error) {
			if n%2 == 0 {
				return 0, errors.New("even")
			}
			processed.Add(1)
			return n, nil
		},
		OnError: func(_ int, _ error) extractor.Recovery { return extractor.Skip },
	})
	require.NoError(t, err)
	require.NoError(t, w.Run(context.Background(), []int{1, 2, 3, 4, 5, 6, 7}))
	assert.Equal(t, int64(4), processed.Load())
}

func TestWorkerProcessRequired(t *testing.T) {
	_, err := extractor.NewWorker[int, int](extractor.WorkerConfig[int, int]{})
	assert.ErrorIs(t, err, extractor.ErrProcessRequired)
}

func TestProgressSnapshot(t *testing.T) {
	p := extractor.NewProgress("test", 10)
	p.Inc(3)
	p.Error()
	p.IncBytes(1024)
	s := p.Snapshot()
	assert.Equal(t, int64(3), s.Done)
	assert.Equal(t, int64(1), s.Errors)
	assert.Equal(t, int64(1024), s.Bytes)
	assert.InDelta(t, 0.3, s.Pct, 0.001)
}

func TestMemoryCheckpoint(t *testing.T) {
	c := extractor.NewMemoryCheckpoint()
	done, _ := c.Done("a")
	assert.False(t, done)
	require.NoError(t, c.Mark("a"))
	done, _ = c.Done("a")
	assert.True(t, done)
	all, _ := c.All()
	assert.Equal(t, []string{"a"}, all)
}

func TestFileCheckpointPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cp.json")
	c1, err := extractor.FileCheckpoint(path)
	require.NoError(t, err)
	require.NoError(t, c1.Mark("a"))
	require.NoError(t, c1.Mark("b"))

	c2, err := extractor.FileCheckpoint(path)
	require.NoError(t, err)
	done, _ := c2.Done("a")
	assert.True(t, done)
	all, _ := c2.All()
	assert.Equal(t, []string{"a", "b"}, all)
}

func TestPipelineSkipsCheckpointedUnits(t *testing.T) {
	cp := extractor.NewMemoryCheckpoint()
	require.NoError(t, cp.Mark("3"))
	var processed atomic.Int64
	p, err := extractor.NewPipeline(extractor.PipelineConfig[int, int]{
		Worker: extractor.WorkerConfig[int, int]{
			Process: func(_ context.Context, n int) (int, error) {
				processed.Add(1)
				return n, nil
			},
		},
		Checkpoint: cp,
		UnitID:     func(n int) string { return strconv.Itoa(n) },
	})
	require.NoError(t, err)
	require.NoError(t, p.Run(context.Background(), []int{1, 2, 3, 4}))
	assert.Equal(t, int64(3), processed.Load())
}

func TestPipelineMissingUnitID(t *testing.T) {
	cp := extractor.NewMemoryCheckpoint()
	_, err := extractor.NewPipeline(extractor.PipelineConfig[int, int]{
		Worker:     extractor.WorkerConfig[int, int]{Process: func(_ context.Context, n int) (int, error) { return n, nil }},
		Checkpoint: cp,
	})
	assert.ErrorIs(t, err, extractor.ErrUnitIDRequired)
}

func TestProgressStreamCancels(t *testing.T) {
	p := extractor.NewProgress("x", 1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.Stream(ctx, 10*time.Millisecond, &nullWriter{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
