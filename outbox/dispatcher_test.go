package outbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer/tracertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/outbox"
)

// stubRepo is a minimal in-memory Repository for testing the
// dispatcher logic without standing up a database.
type stubRepo struct {
	mu      sync.Mutex
	rows    []outbox.Message
	done    []string
	failed  map[string]string
	retryAt map[string]time.Time
}

func newStubRepo(rows ...outbox.Message) *stubRepo {
	return &stubRepo{rows: rows, failed: map[string]string{}, retryAt: map[string]time.Time{}}
}

func (r *stubRepo) Enqueue(_ context.Context, drafts ...outbox.MessageDraft) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range drafts {
		r.rows = append(r.rows, outbox.Message{ID: d.Kind + "-id", Kind: d.Kind, Payload: d.Payload})
	}
	return nil
}

func (r *stubRepo) Claim(_ context.Context, batch int) ([]outbox.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if batch > len(r.rows) {
		batch = len(r.rows)
	}
	out := r.rows[:batch]
	r.rows = r.rows[batch:]
	return out, nil
}

func (r *stubRepo) MarkDone(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = append(r.done, id)
	return nil
}

func (r *stubRepo) MarkFailed(_ context.Context, id string, err error, retryAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.failed[id] = err.Error()
	}
	r.retryAt[id] = retryAt
	return nil
}

func newTestMonitor(t *testing.T) monitoring.Monitor {
	return monitoring.New(loggertest.NewStubLogger(t), tracertest.NewStubTracer(t))
}

func TestDrainer_HappyPath(t *testing.T) {
	repo := newStubRepo(
		outbox.Message{ID: "a", Kind: "ping"},
		outbox.Message{ID: "b", Kind: "ping"},
	)
	handled := []string{}
	handlers := map[string]outbox.Handler{
		"ping": outbox.HandlerFunc(func(_ context.Context, m outbox.Message) error {
			handled = append(handled, m.ID)
			return nil
		}),
	}

	d := outbox.NewDrainer(repo, handlers, newTestMonitor(t), outbox.Config{BatchSize: 10})
	n, err := d.Drain(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.ElementsMatch(t, []string{"a", "b"}, handled)
	assert.ElementsMatch(t, []string{"a", "b"}, repo.done)
	assert.Empty(t, repo.failed)
}

func TestDrainer_HandlerError_MarksFailed(t *testing.T) {
	repo := newStubRepo(outbox.Message{ID: "a", Kind: "ping"})
	handlers := map[string]outbox.Handler{
		"ping": outbox.HandlerFunc(func(_ context.Context, _ outbox.Message) error {
			return errors.New("boom")
		}),
	}

	d := outbox.NewDrainer(repo, handlers, newTestMonitor(t), outbox.Config{BatchSize: 10})
	n, err := d.Drain(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, repo.done)
	assert.Equal(t, "boom", repo.failed["a"])
}

func TestDrainer_UnknownKind_MarksFailed(t *testing.T) {
	repo := newStubRepo(outbox.Message{ID: "a", Kind: "unknown"})
	handlers := map[string]outbox.Handler{}

	d := outbox.NewDrainer(repo, handlers, newTestMonitor(t), outbox.Config{BatchSize: 10})
	n, err := d.Drain(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, repo.failed["a"], "no handler registered")
}

func TestDrainer_MultiplePasses(t *testing.T) {
	repo := newStubRepo(
		outbox.Message{ID: "a", Kind: "ping"},
		outbox.Message{ID: "b", Kind: "ping"},
		outbox.Message{ID: "c", Kind: "ping"},
	)
	handled := []string{}
	handlers := map[string]outbox.Handler{
		"ping": outbox.HandlerFunc(func(_ context.Context, m outbox.Message) error {
			handled = append(handled, m.ID)
			return nil
		}),
	}

	d := outbox.NewDrainer(repo, handlers, newTestMonitor(t), outbox.Config{BatchSize: 1})
	n, err := d.Drain(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []string{"a", "b", "c"}, handled)
}
