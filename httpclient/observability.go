package httpclient

import (
	"net/http"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// NewObservabilityTransport wraps base so every outbound request gets
// an OTel span (propagated via W3C tracecontext headers) and a single
// structured log line on completion with method / host / status /
// duration. Pass it to WithTransport, or rely on FxModule to wire it
// automatically when logger + tracer are available in the fx graph.
func NewObservabilityTransport(base http.RoundTripper, log logger.Logger, tr tracer.Tracer) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &observabilityTransport{base: base, log: log, tracer: tr}
}

type observabilityTransport struct {
	base   http.RoundTripper
	log    logger.Logger
	tracer tracer.Tracer
}

func (o *observabilityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if o.tracer != nil {
		var span tracer.Span
		ctx, span = o.tracer.Start(ctx, "HTTP "+req.Method)
		defer span.End()
		span.SetAttributes(
			tracer.String("http.method", req.Method),
			tracer.String("http.url", req.URL.String()),
			tracer.String("http.host", req.URL.Host),
		)
		o.tracer.Inject(ctx, headerCarrier(req.Header))
		req = req.WithContext(ctx)
	}

	start := time.Now()
	res, err := o.base.RoundTrip(req)
	durMs := time.Since(start).Milliseconds()

	if o.tracer != nil {
		span := o.tracer.SpanFromContext(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusError, err.Error())
		} else if res != nil {
			span.SetAttributes(tracer.Int("http.status_code", res.StatusCode))
			if res.StatusCode >= 500 {
				span.SetStatus(tracer.StatusError, http.StatusText(res.StatusCode))
			}
		}
	}

	if o.log != nil {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		o.log.WithKeysAndValues(
			"outbound.method", req.Method,
			"outbound.host", req.URL.Host,
			"outbound.status", status,
			"outbound.duration_ms", durMs,
		).InfoContext(ctx, "outbound HTTP call")
	}

	return res, err
}

// headerCarrier adapts http.Header to the tracer.Carrier interface
// so the tracer can inject W3C tracecontext headers without us
// touching the propagator directly.
type headerCarrier http.Header

func (h headerCarrier) Get(key string) string { return http.Header(h).Get(key) }
func (h headerCarrier) Set(key, value string) { http.Header(h).Set(key, value) }
func (h headerCarrier) Keys() []string {
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	return out
}
