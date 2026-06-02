// Package httpclient wraps net/http with retry, circuit breaker, and tracing
// so consumers stop reimplementing the same outbound-call boilerplate.
//
//	c := httpclient.New(
//	    httpclient.WithBaseURL("https://api.example.com"),
//	    httpclient.WithTimeout(5 * time.Second),
//	    httpclient.WithRetries(3),
//	    httpclient.WithBreaker(httpclient.NewBreaker()),
//	)
//	res, err := c.Do(ctx, req)
package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fromforgesoftware/go-kit/retry"
)

// Client is an http.Client wrapper with retry + breaker + per-attempt timeout.
type Client struct {
	base           *http.Client
	baseURL        string
	timeout        time.Duration
	retries        int
	backoff        []retry.Option
	breaker        *Breaker
	perHostBreaker *PerHostBreaker
	idempotentOnly bool
	headers        func() http.Header
	transport      http.RoundTripper
}

type Option func(*Client)

// WithHTTPClient overrides the underlying http.Client. Useful when callers
// want shared connection pooling. Resets timeout to the supplied client's.
func WithHTTPClient(c *http.Client) Option {
	return func(cli *Client) { cli.base = c }
}

// WithBaseURL prepends a base URL to relative request paths.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithTimeout sets the per-attempt timeout. The http.Client's own timeout is
// left alone; this wraps the request context.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithRetries sets the total number of attempts (1 = no retry, default 1).
func WithRetries(n int) Option {
	return func(c *Client) { c.retries = n }
}

// WithBackoff overrides the retry policy options. Defaults to exponential
// backoff starting at 100ms.
func WithBackoff(opts ...retry.Option) Option {
	return func(c *Client) { c.backoff = opts }
}

// WithBreaker installs a circuit breaker. When the breaker is open, Do
// returns ErrBreakerOpen immediately without contacting the server.
func WithBreaker(b *Breaker) Option {
	return func(c *Client) { c.breaker = b }
}

// WithBreakerPerHost installs a manager that builds a separate
// *Breaker per upstream host. One bad downstream stops poisoning
// unrelated calls. Mutually exclusive with WithBreaker; the per-host
// manager wins if both are supplied.
func WithBreakerPerHost(threshold int, cooldown time.Duration) Option {
	return func(c *Client) { c.perHostBreaker = NewPerHostBreaker(threshold, cooldown) }
}

// WithRetryAll opts every HTTP method into the retry policy. By
// default retries are gated to idempotent methods (GET, HEAD, PUT,
// DELETE, OPTIONS) per RFC 9110 §9.2.2 — replaying POST/PATCH can
// double-charge / double-create. Use this for endpoints you know
// are idempotent on the server side despite the verb.
func WithRetryAll() Option {
	return func(c *Client) { c.idempotentOnly = false }
}

// WithHeaders installs a header factory invoked per attempt. Headers from
// the inbound request are preserved; factory headers are added on top
// (typical use: Authorization, X-Tenant-ID).
func WithHeaders(fn func() http.Header) Option {
	return func(c *Client) { c.headers = fn }
}

// WithTransport overrides the RoundTripper. Combine with NewTracingTransport
// from go/kit/transport/rest for OTEL spans.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.transport = rt }
}

// New constructs a Client with the supplied options.
func New(opts ...Option) *Client {
	c := &Client{
		base:           http.DefaultClient,
		timeout:        30 * time.Second,
		retries:        1,
		idempotentOnly: true,
		backoff: []retry.Option{
			retry.WithExponentialPolicy(),
			retry.WithInitialInterval(100 * time.Millisecond),
			retry.WithMaxInterval(2 * time.Second),
		},
	}
	for _, o := range opts {
		o(c)
	}
	if c.transport != nil {
		// Swap the base client to apply the custom transport.
		c.base = &http.Client{Transport: c.transport, Timeout: c.base.Timeout}
	}
	return c
}

// Do sends the request with the configured retry + breaker semantics. The
// request body is buffered so retries can replay it.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c.baseURL != "" && req.URL != nil && !req.URL.IsAbs() {
		full, err := joinURL(c.baseURL, req.URL.String())
		if err != nil {
			return nil, err
		}
		req.URL = full
		req.Host = full.Host
	}

	breaker := c.breakerFor(req)
	if breaker != nil {
		if allow, _ := breaker.Allow(); !allow {
			return nil, ErrBreakerOpen
		}
	}

	if c.headers != nil {
		for k, vs := range c.headers() {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("httpclient: read body: %w", err)
		}
		_ = req.Body.Close()
		bodyBytes = b
	}

	// Idempotent methods retry; others get a single attempt unless
	// the caller opted into WithRetryAll().
	attempts := c.retries
	if c.idempotentOnly && !isIdempotent(req.Method) {
		attempts = 1
	}

	res, err := retry.RetryWithDataAndContext(ctx, func() (*http.Response, error) {
		return c.attempt(ctx, req, bodyBytes)
	}, append(c.backoff, retry.WithMaxAttempts(attempts))...)
	if IsRetriable(err) {
		// Retries exhausted but the final attempt still returned a
		// retriable status — surface the response, not the marker error.
		err = nil
	}

	if breaker != nil {
		if err != nil || isFailureStatus(res) {
			breaker.RecordFailure()
		} else {
			breaker.RecordSuccess()
		}
	}
	return res, err
}

// breakerFor returns the breaker to consult for req — either the
// per-host one (preferred when configured), the single global one,
// or nil if neither is set.
func (c *Client) breakerFor(req *http.Request) *Breaker {
	if c.perHostBreaker != nil && req.URL != nil && req.URL.Host != "" {
		return c.perHostBreaker.For(req.URL.Host)
	}
	return c.breaker
}

// isIdempotent reports whether the HTTP method is safe to retry per
// RFC 9110 §9.2.2.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodTrace, "":
		return true
	}
	return false
}

func (c *Client) attempt(ctx context.Context, req *http.Request, bodyBytes []byte) (*http.Response, error) {
	attemptCtx := ctx
	var cancel context.CancelFunc
	if c.timeout > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	r := req.Clone(attemptCtx)
	if bodyBytes != nil {
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		r.ContentLength = int64(len(bodyBytes))
	}

	res, err := c.base.Do(r)
	if err != nil {
		return nil, err
	}
	if isRetriableStatus(res) {
		return res, errRetriable{status: res.StatusCode}
	}
	return res, nil
}

func isRetriableStatus(res *http.Response) bool {
	if res == nil {
		return false
	}
	switch res.StatusCode {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func isFailureStatus(res *http.Response) bool {
	return res != nil && res.StatusCode >= 500
}

type errRetriable struct{ status int }

func (e errRetriable) Error() string {
	return fmt.Sprintf("httpclient: retriable status %d", e.status)
}

// IsRetriable reports whether an error is one the client retried on.
func IsRetriable(err error) bool {
	var e errRetriable
	return errors.As(err, &e)
}
