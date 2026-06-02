package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSE (Server-Sent Events) is one-way over HTTP with a long-lived
// response body and `Content-Type: text/event-stream`. It's still
// HTTP, so it lives in the rest package rather than a separate
// transport.

// SSEEvent is one frame sent to the client. ID is optional; clients
// reuse the last seen id when reconnecting via the `Last-Event-ID`
// header. Event is the optional event name (clients listen via
// `eventSource.addEventListener("name", ...)`); Data is the payload.
type SSEEvent struct {
	ID    string
	Event string
	Data  any
}

// SSEHandlerOpt configures NewServerSentEventsHandler.
type SSEHandlerOpt func(*sseConfig)

type sseConfig struct {
	heartbeatInterval time.Duration
}

func defaultSSEConfig() *sseConfig {
	// 25s heartbeat: under most proxy idle-timeouts (commonly 30s+)
	// so the connection stays alive without surprising the client.
	return &sseConfig{heartbeatInterval: 25 * time.Second}
}

// SSEWithHeartbeatInterval overrides the keep-alive comment cadence
// (default 25s). Set 0 to disable heartbeats entirely.
func SSEWithHeartbeatInterval(d time.Duration) SSEHandlerOpt {
	return func(c *sseConfig) { c.heartbeatInterval = d }
}

// NewServerSentEventsHandler wires a GET endpoint that streams events
// to the client. The producer receives a context (cancelled when the
// client disconnects) and a channel to write events into; the handler
// frames each event per the SSE spec and writes a keep-alive comment
// every heartbeat interval.
//
//	r.Get("/signals/stream",
//	    kitrest.NewServerSentEventsHandler(
//	        func(req *http.Request) (StreamCommand, error) {
//	            return StreamCommand{Topic: req.URL.Query().Get("topic")}, nil
//	        },
//	        uc.Stream,
//	    ),
//	)
//
//	// usecase
//	func (uc *signalsUsecase) Stream(ctx context.Context, cmd StreamCommand, out chan<- rest.SSEEvent) error {
//	    sub := uc.bus.Subscribe(cmd.Topic)
//	    defer sub.Close()
//	    for {
//	        select {
//	        case <-ctx.Done(): return nil
//	        case sig := <-sub.C: out <- rest.SSEEvent{Event: "signal", Data: sig}
//	        }
//	    }
//	}
//
// The handler closes the events channel when the producer returns or
// the client disconnects; the producer should not close it. Note that
// the http.Server's WriteTimeout must be 0 (disabled) for the route
// — otherwise long-lived streams are cut off. Wire with
// WithWriteTimeout(0) at server construction or behind a per-route
// opt-out if support for that lands.
func NewServerSentEventsHandler[Cmd any](
	decoder func(*http.Request) (Cmd, error),
	producer func(ctx context.Context, cmd Cmd, out chan<- SSEEvent) error,
	opts ...SSEHandlerOpt,
) http.Handler {
	cfg := defaultSSEConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		cmd, err := decoder(req)
		if err != nil {
			JsonApiErrorEncoder(ctx, err, w)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			// Misconfigured server (writer doesn't support flushing) —
			// SSE wouldn't ever push frames. Surface a 500 instead of
			// silently buffering forever.
			JsonApiErrorEncoder(ctx, errInternal("response writer does not support flushing"), w)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		events := make(chan SSEEvent, 8)
		producerErr := make(chan error, 1)
		go func() {
			defer close(events)
			producerErr <- producer(ctx, cmd, events)
		}()

		var heartbeat <-chan time.Time
		if cfg.heartbeatInterval > 0 {
			t := time.NewTicker(cfg.heartbeatInterval)
			defer t.Stop()
			heartbeat = t.C
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat:
				// SSE comment line — clients ignore it but proxies see traffic.
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case ev, ok := <-events:
				if !ok {
					// Producer finished. Surface its error (if any) — but
					// headers are already sent, so we can only log/close;
					// the client sees an EOF. We could surface via a final
					// "event: error" frame but most clients ignore that.
					<-producerErr
					return
				}
				if err := writeSSEEvent(w, ev); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

// writeSSEEvent frames one event per the spec:
//
//	id: <ID>\n          (optional)
//	event: <Event>\n    (optional)
//	data: <Data>\n
//	\n
//
// Data is JSON-encoded so the wire is parseable by every SSE client.
// Multi-line data is split into multiple `data:` lines per spec.
func writeSSEEvent(w http.ResponseWriter, ev SSEEvent) error {
	if ev.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", ev.ID); err != nil {
			return err
		}
	}
	if ev.Event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", ev.Event); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	// Spec requires every newline in data to start its own "data:" line.
	for _, line := range strings.Split(string(payload), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err = fmt.Fprint(w, "\n")
	return err
}

// errInternal is a small kit-error helper kept local so we don't
// have to import the whole errors package just for one call site.
func errInternal(msg string) error { return errInternalErr{msg: msg} }

type errInternalErr struct{ msg string }

func (e errInternalErr) Error() string { return e.msg }
