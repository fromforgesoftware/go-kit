package amqp

import (
	"errors"
	"os"
	"sync/atomic"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/retry"
)

const (
	maxReconnectionAttempts = 5
	reconnectionDelay       = 3 * time.Second
)

// ErrConnectionClosed is returned by Connection operations after Close().
var ErrConnectionClosed = errors.New("amqp: connection closed")

// Connection is the contract every producer/consumer in the package depends
// on. Implementations must survive transparent broker reconnects: Channel()
// always returns a channel opened against the *current* underlying
// connection.
type Connection interface {
	Channel() (*amqp091.Channel, error)
	Config() amqp091.Config
	Close() error
}

type config struct {
	connURL     string
	vhost       string
	maxChannels uint16
	properties  amqp091.Table
}

type connOption func(*config)

func WithConnURLFromEnv() connOption {
	return WithConnURL(os.Getenv("AMQP_URL"))
}

func WithConnURL(url string) connOption {
	return func(c *config) {
		c.connURL = url
	}
}

func WithVhost(vhost string) connOption {
	return func(c *config) {
		c.vhost = vhost
	}
}

func WithMaxChannels(maxChannels uint16) connOption {
	return func(c *config) {
		c.maxChannels = maxChannels
	}
}

func WithProperties(properties amqp091.Table) connOption {
	return func(c *config) {
		c.properties = properties
	}
}

func defaultOpts() []connOption {
	return []connOption{
		WithConnURLFromEnv(),
		WithVhost("/"),
		WithMaxChannels(0),
		WithProperties(amqp091.NewConnectionProperties()),
	}
}

// connection wraps *amqp091.Connection so a reconnect can swap the
// underlying connection without invalidating callers who already hold
// pointers to us. The previous implementation copied the live struct
// (lock + internal goroutines included) which corrupted both copies.
type connection struct {
	inner  atomic.Pointer[amqp091.Connection]
	cfg    config
	log    logger.Logger
	closed atomic.Bool
}

// NewConnection dials the broker and starts a background goroutine that
// transparently reconnects on broker close events.
func NewConnection(log logger.Logger, opts ...connOption) (*connection, error) {
	cfg := config{}
	for _, opt := range append(defaultOpts(), opts...) {
		opt(&cfg)
	}

	conn, err := dial(cfg)
	if err != nil {
		return nil, err
	}

	c := &connection{cfg: cfg, log: log}
	c.inner.Store(conn)

	go c.watch()
	return c, nil
}

func dial(cfg config) (*amqp091.Connection, error) {
	return amqp091.DialConfig(cfg.connURL, amqp091.Config{
		Vhost:      cfg.vhost,
		ChannelMax: cfg.maxChannels,
		Properties: cfg.properties,
	})
}

// Channel opens a fresh AMQP channel against the current connection. Callers
// should treat the returned channel as owned by them: close it when done.
func (c *connection) Channel() (*amqp091.Channel, error) {
	if c.closed.Load() {
		return nil, ErrConnectionClosed
	}
	conn := c.inner.Load()
	if conn == nil {
		return nil, ErrConnectionClosed
	}
	return conn.Channel()
}

// Config returns the dialed amqp091 config of the current connection.
func (c *connection) Config() amqp091.Config {
	if conn := c.inner.Load(); conn != nil {
		return conn.Config
	}
	return amqp091.Config{}
}

// Close releases the connection and stops the reconnect goroutine.
func (c *connection) Close() error {
	c.closed.Store(true)
	conn := c.inner.Swap(nil)
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// watch listens for broker-side close events and atomically swaps in a
// freshly-dialed connection on disconnect. Reconnect failures are logged
// rather than panicking — the process stays alive and subsequent Channel()
// calls return ErrConnectionClosed until the next NotifyClose event lands.
func (c *connection) watch() {
	for {
		conn := c.inner.Load()
		if conn == nil || c.closed.Load() {
			return
		}

		// NotifyClose closes the returned channel once the underlying
		// connection terminates; for normal Close() it sends nil.
		notifyCh := conn.NotifyClose(make(chan *amqp091.Error, 1))
		closeErr, ok := <-notifyCh
		if !ok || c.closed.Load() {
			return
		}

		if c.log != nil && closeErr != nil {
			c.log.Warn("amqp connection lost, reconnecting: %v", closeErr)
		}

		var fresh *amqp091.Connection
		err := retry.Retry(func() error {
			var dialErr error
			fresh, dialErr = dial(c.cfg)
			return dialErr
		},
			retry.WithExponentialPolicy(),
			retry.WithMaxRetries(maxReconnectionAttempts),
			retry.WithInitialInterval(reconnectionDelay),
		)
		if err != nil {
			if c.log != nil {
				c.log.Error("amqp reconnect failed after %d attempts: %v", maxReconnectionAttempts, err)
			}
			return
		}

		c.inner.Store(fresh)
		if c.log != nil {
			c.log.Info("amqp connection re-established")
		}
	}
}
