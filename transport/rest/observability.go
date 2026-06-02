package rest

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// HeaderRequestID is the canonical request-id header read and written by the
// REST middleware stack.
const HeaderRequestID = "X-Request-ID"

// requestIDKey is the context key for the per-request id. Unexported so
// callers go through RequestIDFromContext.
type requestIDKey struct{}

// RequestIDFromContext returns the request id installed by the
// RequestIDMiddleware, or "" if no middleware ran.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// httpHeaderCarrier adapts http.Header to tracer.Carrier. Header.Get and
// Header.Set already match Carrier's signatures; Keys() is the only thing
// we add.
type httpHeaderCarrier http.Header

func (c httpHeaderCarrier) Get(key string) string { return http.Header(c).Get(key) }
func (c httpHeaderCarrier) Set(key, value string) { http.Header(c).Set(key, value) }
func (c httpHeaderCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}

// responseRecorder captures status code + bytes-written so the access log
// and tracing middleware can read them after the handler returns.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush forwards to the underlying ResponseWriter if it supports it, so
// streaming endpoints continue to work behind the middleware chain.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RequestIDMiddleware ensures every request has an id, propagates it through
// ctx and into the response header. Pairs with the logger and tracer
// middlewares so log lines and spans share the same correlation id.
type RequestIDMiddleware struct{}

func NewRequestIDMiddleware() *RequestIDMiddleware { return &RequestIDMiddleware{} }

func (m *RequestIDMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TracingMiddleware extracts trace context from request headers, starts a
// server span around the handler, and records status + error attributes.
type TracingMiddleware struct {
	monitor monitoring.Monitor
}

func NewTracingMiddleware(monitor monitoring.Monitor) *TracingMiddleware {
	return &TracingMiddleware{monitor: monitor}
}

func (m *TracingMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := m.monitor.Tracer().Extract(r.Context(), httpHeaderCarrier(r.Header))
		spanName := r.Method + " " + r.URL.Path
		ctx, span := m.monitor.Tracer().Start(ctx, spanName,
			tracer.WithSpanKind(tracer.SpanKindServer),
			tracer.WithAttributes(
				tracer.String("http.method", r.Method),
				tracer.String("http.target", r.URL.Path),
				tracer.String("http.scheme", schemeOf(r)),
				tracer.String("net.peer.addr", r.RemoteAddr),
			),
		)
		defer span.End()

		if rid := RequestIDFromContext(ctx); rid != "" {
			span.SetAttributes(tracer.String("request.id", rid))
		}

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		span.SetAttributes(tracer.Int("http.status_code", rec.status))
		if rec.status >= 500 {
			span.SetStatus(tracer.StatusError, http.StatusText(rec.status))
		}
	})
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// RecoveryMiddleware turns handler panics into 500 responses and records the
// panic on the active span. Without it a panicking handler kills only the
// goroutine but leaves the client hanging until ReadTimeout.
type RecoveryMiddleware struct {
	monitor monitoring.Monitor
}

func NewRecoveryMiddleware(monitor monitoring.Monitor) *RecoveryMiddleware {
	return &RecoveryMiddleware{monitor: monitor}
}

func (m *RecoveryMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			err := asError(rec)
			m.monitor.Logger().
				WithKeysAndValues(
					"request_id", RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", err.Error(),
					"stack", string(debug.Stack()),
				).
				ErrorContext(r.Context(), "rest handler panic")

			sp := m.monitor.Tracer().SpanFromContext(r.Context())
			sp.RecordError(err)
			sp.SetStatus(tracer.StatusError, "handler panic")

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

func asError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("%v", v)
}

// AccessLogMiddleware logs one line per request after the response is
// written. Place it close to the outside of the chain so duration covers
// the whole pipeline including auth + decoders.
type AccessLogMiddleware struct {
	monitor monitoring.Monitor
}

func NewAccessLogMiddleware(monitor monitoring.Monitor) *AccessLogMiddleware {
	return &AccessLogMiddleware{monitor: monitor}
}

func (m *AccessLogMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		m.monitor.Logger().
			WithKeysAndValues(
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", strconv.Itoa(rec.status),
				"bytes", strconv.Itoa(rec.bytes),
				"duration_ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10),
			).
			InfoContext(r.Context(), "http request")
	})
}

// DefaultObservabilityMiddlewares returns the recommended middleware stack
// (outermost first): RequestID → AccessLog → Tracing → Recovery. Apply
// with WithMiddlewares(...).
func DefaultObservabilityMiddlewares(monitor monitoring.Monitor) []Middleware {
	return []Middleware{
		NewRequestIDMiddleware(),
		NewAccessLogMiddleware(monitor),
		NewTracingMiddleware(monitor),
		NewRecoveryMiddleware(monitor),
	}
}

// NewTracingTransport returns an http.RoundTripper that injects the current
// span's trace context into outbound request headers. Wrap an existing
// transport (default http.DefaultTransport) with WithTransport. Use it via
// WithHTTPClient(&http.Client{Transport: NewTracingTransport(...)}).
func NewTracingTransport(monitor monitoring.Monitor, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &tracingTransport{base: base, monitor: monitor}
}

type tracingTransport struct {
	base    http.RoundTripper
	monitor monitoring.Monitor
}

func (t *tracingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, span := t.monitor.Tracer().Start(r.Context(),
		r.Method+" "+r.URL.Path,
		tracer.WithSpanKind(tracer.SpanKindClient),
		tracer.WithAttributes(
			tracer.String("http.method", r.Method),
			tracer.String("http.url", r.URL.String()),
		),
	)
	defer span.End()

	t.monitor.Tracer().Inject(ctx, httpHeaderCarrier(r.Header))

	resp, err := t.base.RoundTrip(r.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		return resp, err
	}
	span.SetAttributes(tracer.Int("http.status_code", resp.StatusCode))
	if resp.StatusCode >= 500 {
		span.SetStatus(tracer.StatusError, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}
