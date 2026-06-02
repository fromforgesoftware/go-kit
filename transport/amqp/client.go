package amqp

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/slicesx"
)

type ExchangeType string

const (
	ExchangeTypeDirect  ExchangeType = "direct"
	ExchangeTypeFanout  ExchangeType = "fanout"
	ExchangeTypeTopic   ExchangeType = "topic"
	ExchangeTypeHeaders ExchangeType = "headers"

	FieldNameQueue = "queue"
)

func (e ExchangeType) String() string {
	return string(e)
}

type Exchange struct {
	name       string
	kind       ExchangeType
	durable    bool
	autoDelete bool
	internal   bool
	noWait     bool
}

type exchangeOption func(*Exchange)

func ExchangeDurable(durable bool) exchangeOption {
	return func(e *Exchange) {
		e.durable = durable
	}
}

func ExchangeAutoDelete(autoDelete bool) exchangeOption {
	return func(e *Exchange) {
		e.autoDelete = autoDelete
	}
}

func ExchangeInternal(internal bool) exchangeOption {
	return func(e *Exchange) {
		e.internal = internal
	}
}

func ExchangeNoWait(noWait bool) exchangeOption {
	return func(e *Exchange) {
		e.noWait = noWait
	}
}

func defaultExchangeOpts() []exchangeOption {
	return []exchangeOption{
		ExchangeDurable(true),
		ExchangeAutoDelete(false),
		ExchangeInternal(false),
		ExchangeNoWait(false),
	}
}

func NewExchange(name string, kind ExchangeType, opts ...exchangeOption) *Exchange {
	e := &Exchange{
		name: name,
		kind: kind,
	}
	for _, opt := range append(defaultExchangeOpts(), opts...) {
		opt(e)
	}
	return e
}

type Queue struct {
	name       string
	durable    bool
	autoDelete bool
	exclusive  bool
	noWait     bool
}

type queueOption func(*Queue)

func QueueDurable(durable bool) queueOption {
	return func(q *Queue) {
		q.durable = durable
	}
}

func QueueAutoDelete(autoDelete bool) queueOption {
	return func(q *Queue) {
		q.autoDelete = autoDelete
	}
}

func QueueExclusive(exclusive bool) queueOption {
	return func(q *Queue) {
		q.exclusive = exclusive
	}
}

func QueueNoWait(noWait bool) queueOption {
	return func(q *Queue) {
		q.noWait = noWait
	}
}

func QueueName(name string) queueOption {
	return func(q *Queue) {
		q.name = name
	}
}

func defaultQueueOpts(consumerName string) []queueOption {
	return []queueOption{
		QueueName(consumerName),
		QueueDurable(true),
		QueueAutoDelete(false),
		QueueExclusive(false),
		QueueNoWait(false),
	}
}

func NewQueue(consumerName string, opts ...queueOption) *Queue {
	q := new(Queue)

	if len(consumerName) < 1 {
		panic(errors.New("name cannot be empty"))
	}
	for _, opt := range append(defaultQueueOpts(consumerName), opts...) {
		opt(q)
	}
	if len(q.name) < 1 {
		panic(errors.New("queue.name cannot be empty"))
	}

	return q
}

type (
	RoutingKeyPart string
	routingKey     []RoutingKeyPart
)

func (rkp RoutingKeyPart) String() string {
	return string(rkp)
}

const (
	RoutingKeyPartMatchAnyWord  RoutingKeyPart = "*"
	RoutingKeyPartMatchAnyWords RoutingKeyPart = "#"
)

func (rk routingKey) String() string {
	if len(rk) < 1 {
		return ""
	}

	return strings.Join(
		slicesx.Map(rk, func(rkp RoutingKeyPart) string { return rkp.String() }), ".",
	)
}

func RoutingKey(parts ...RoutingKeyPart) routingKey {
	return routingKey(parts)
}

// client owns one AMQP channel that is atomically replaced on reconnect.
// All publish/consume operations route through channel() so callers never
// see a stale channel pointer after a broker restart.
type client struct {
	conn Connection
	ch   atomic.Pointer[amqp091.Channel]
	log  logger.Logger

	exchange *Exchange

	// publishMu serializes Publish frames on the underlying channel.
	// amqp091.Channel is not safe for concurrent Publish calls — concurrent
	// publishers interleave AMQP frames and break the wire protocol.
	publishMu sync.Mutex
}

func newClient(
	conn Connection,
	log logger.Logger,
	e *Exchange,
) (*client, error) {
	cli := &client{
		conn:     conn,
		log:      log,
		exchange: e,
	}
	if err := cli.openChannel(); err != nil {
		return nil, err
	}
	return cli, nil
}

// openChannel opens a fresh channel against the current connection and
// declares the exchange on it. Safe to call multiple times — callers use it
// during construction and during reconnect.
func (c *client) openChannel() error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(
		c.exchange.name, c.exchange.kind.String(),
		c.exchange.durable, c.exchange.autoDelete,
		c.exchange.internal, c.exchange.noWait, nil,
	); err != nil {
		_ = ch.Close()
		return err
	}
	if old := c.ch.Swap(ch); old != nil {
		_ = old.Close()
	}
	c.log.Debug("exchange with name: %q, type: %q, declared", c.exchange.name, c.exchange.kind.String())
	return nil
}

// channel returns the current channel; nil if the client has been closed.
func (c *client) channel() *amqp091.Channel {
	return c.ch.Load()
}

// close releases the held channel.
func (c *client) close() error {
	if old := c.ch.Swap(nil); old != nil {
		return old.Close()
	}
	return nil
}

// reconnect re-opens the channel against the current Connection. Used by
// the consumer loop when the broker drops the delivery channel.
func (c *client) reconnect() error {
	return c.openChannel()
}
