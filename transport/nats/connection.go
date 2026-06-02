package nats

import (
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

type Connection interface {
	Conn() *nats.Conn
	JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error)
	Close()
}

type config struct {
	connURL string
	opts    []nats.Option
}

type connOption func(*config)

func WithConnURLFromEnv() connOption {
	addr := os.Getenv("NATS_URL")
	if addr == "" {
		addr = nats.DefaultURL
	}
	return WithConnURL(addr)
}

func WithConnURL(url string) connOption {
	return func(c *config) {
		c.connURL = url
	}
}

func WithNatsOptions(opts ...nats.Option) connOption {
	return func(c *config) {
		c.opts = append(c.opts, opts...)
	}
}

func defaultOpts() []connOption {
	return []connOption{
		WithConnURLFromEnv(),
	}
}

type connection struct {
	nc *nats.Conn
}

func NewConnection(opts ...connOption) (*connection, error) {
	config := &config{}
	for _, opt := range append(defaultOpts(), opts...) {
		opt(config)
	}

	// Default NATS options for resilience
	natsOpts := []nats.Option{
		nats.MaxReconnects(-1),
	}
	natsOpts = append(natsOpts, config.opts...)

	nc, err := nats.Connect(config.connURL, natsOpts...)
	if err != nil {
		return nil, err
	}

	return &connection{nc: nc}, nil
}

func (c *connection) Conn() *nats.Conn {
	return c.nc
}

func (c *connection) JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error) {
	return c.nc.JetStream(opts...)
}

// Close drains the connection — calls Drain() and waits for the connection
// to actually finish closing. The previous implementation called Drain()
// (async) and then Close() immediately, which yanked the connection out
// from under the in-flight drain. Drain alone is the supported graceful
// shutdown path in nats.go.
//
// Use CloseImmediately when the caller wants an abrupt teardown (tests,
// fatal-error paths).
func (c *connection) Close() {
	if c.nc == nil {
		return
	}
	// Drain returns immediately; the connection's internal goroutine
	// closes the conn once subscriptions finish. We poll IsClosed so the
	// caller can rely on the connection being fully torn down on return.
	_ = c.nc.Drain()
	for !c.nc.IsClosed() {
		time.Sleep(10 * time.Millisecond)
	}
}

// CloseImmediately tears down the connection without draining
// in-flight subscriptions. Use for tests and fatal-error paths.
func (c *connection) CloseImmediately() {
	if c.nc != nil {
		c.nc.Close()
	}
}
