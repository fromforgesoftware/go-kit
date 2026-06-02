package rest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type streamCmd struct {
	Topic string
}

func TestNewServerSentEventsHandlerFramesEvents(t *testing.T) {
	decoder := func(req *http.Request) (streamCmd, error) {
		return streamCmd{Topic: req.URL.Query().Get("topic")}, nil
	}
	producer := func(_ context.Context, cmd streamCmd, out chan<- rest.SSEEvent) error {
		assert.Equal(t, "signals", cmd.Topic)
		out <- rest.SSEEvent{ID: "1", Event: "signal", Data: map[string]string{"sym": "AAPL"}}
		out <- rest.SSEEvent{ID: "2", Event: "signal", Data: map[string]string{"sym": "MSFT"}}
		return nil
	}
	h := rest.NewServerSentEventsHandler(decoder, producer,
		rest.SSEWithHeartbeatInterval(0)) // no heartbeats during the test

	req := httptest.NewRequest(http.MethodGet, "/stream?topic=signals", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	assert.Contains(t, body, "id: 1\nevent: signal\ndata: {\"sym\":\"AAPL\"}\n\n")
	assert.Contains(t, body, "id: 2\nevent: signal\ndata: {\"sym\":\"MSFT\"}\n\n")
}

func TestNewServerSentEventsHandlerDecoderError(t *testing.T) {
	decoder := func(_ *http.Request) (streamCmd, error) {
		return streamCmd{}, errStub("invalid topic")
	}
	producer := func(_ context.Context, _ streamCmd, _ chan<- rest.SSEEvent) error {
		t.Fatal("producer should not start when decoder errors")
		return nil
	}
	h := rest.NewServerSentEventsHandler(decoder, producer)

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Plain error → 500 (decoder didn't return a kit error).
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNewServerSentEventsHandlerContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Producer runs until ctx is cancelled.
	producerExited := make(chan struct{})
	producer := func(ctx context.Context, _ streamCmd, out chan<- rest.SSEEvent) error {
		defer close(producerExited)
		<-ctx.Done()
		return nil
	}
	decoder := func(_ *http.Request) (streamCmd, error) { return streamCmd{}, nil }
	h := rest.NewServerSentEventsHandler(decoder, producer,
		rest.SSEWithHeartbeatInterval(0))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() { h.ServeHTTP(rec, req); close(done) }()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not exit after context cancel")
	}
	select {
	case <-producerExited:
	case <-time.After(time.Second):
		t.Fatal("producer did not exit after context cancel")
	}
}

// errStub is a tiny error type so the decoder-error test doesn't have
// to drag in kit/errors just for one assertion.
type errStub string

func (e errStub) Error() string { return string(e) }

// keep imports tidy
var _ = strings.Builder{}
