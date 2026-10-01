package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

// The server side of the client's protocol. A Hub upgrades HTTP requests, greets each
// connection with a welcome, answers pings with pongs by id, acknowledges subscribe and
// unsubscribe by id, and fans Publish out to every subscriber of a topic as `message` frames
// carrying a per-topic sequence number.

const (
	defaultServerWriteTimeout = 10 * time.Second
	defaultServerReadTimeout  = 90 * time.Second
	defaultSendBuffer         = 256
	defaultReadLimitBytes     = 64 << 10
)

type (
	// Authenticator decides whether a request may upgrade. An error refuses with 401.
	Authenticator func(r *http.Request) error

	// SubscribeHook is consulted before a subscription is acknowledged. An error refuses it and
	// is sent back to the client as an `error` frame.
	SubscribeHook func(ctx context.Context, conn ConnInfo, topic TopicType) error

	// UnsubscribeHook is told when a subscription ends, by request or because the connection
	// closed.
	UnsubscribeHook func(conn ConnInfo, topic TopicType)

	// ConnInfo identifies one connection to the hooks.
	ConnInfo struct {
		ID      string
		Request *http.Request
	}

	// SlowConsumerPolicy is what happens when a connection cannot keep up with Publish.
	SlowConsumerPolicy int

	hubOption func(*hubConfig)

	hubConfig struct {
		authenticate  Authenticator
		onSubscribe   SubscribeHook
		onUnsubscribe UnsubscribeHook
		checkOrigin   func(r *http.Request) bool
		writeTimeout  time.Duration
		readTimeout   time.Duration
		sendBuffer    int
		readLimit     int64
		slowConsumer  SlowConsumerPolicy
	}

	// Hub owns the connections and the topic subscriptions.
	Hub struct {
		cfg     hubConfig
		monitor monitoring.Monitor

		mu      sync.RWMutex
		conns   map[*serverConn]struct{}
		topics  map[TopicType]map[*serverConn]struct{}
		seqs    map[TopicType]*atomic.Int64
		closed  bool
		nextID  atomic.Int64
		dropped atomic.Int64
	}

	serverConn struct {
		hub    *Hub
		info   ConnInfo
		conn   *websocket.Conn
		send   chan *Message
		done   chan struct{}
		once   sync.Once
		topics map[TopicType]struct{}
	}
)

const (
	// SlowConsumerDisconnect closes a connection whose send buffer is full; the client
	// reconnects and resynchronises. The default.
	SlowConsumerDisconnect SlowConsumerPolicy = iota
	// SlowConsumerDrop discards the frame for that connection and counts it.
	SlowConsumerDrop
)

// WithAuthenticator gates the upgrade.
func WithAuthenticator(a Authenticator) hubOption { return func(c *hubConfig) { c.authenticate = a } }

// WithSubscribeHook lets the application accept or refuse a topic.
func WithSubscribeHook(h SubscribeHook) hubOption { return func(c *hubConfig) { c.onSubscribe = h } }

// WithUnsubscribeHook lets the application release what a subscription held.
func WithUnsubscribeHook(h UnsubscribeHook) hubOption {
	return func(c *hubConfig) { c.onUnsubscribe = h }
}

// WithCheckOrigin overrides the origin check. The default accepts same-origin and loopback.
func WithCheckOrigin(f func(r *http.Request) bool) hubOption {
	return func(c *hubConfig) { c.checkOrigin = f }
}

// WithServerWriteTimeout bounds one frame's write.
func WithServerWriteTimeout(d time.Duration) hubOption {
	return func(c *hubConfig) { c.writeTimeout = d }
}

// WithServerReadTimeout is how long a connection may stay silent; clients ping well inside it.
func WithServerReadTimeout(d time.Duration) hubOption {
	return func(c *hubConfig) { c.readTimeout = d }
}

// WithSendBuffer sizes each connection's outbound queue.
func WithSendBuffer(n int) hubOption { return func(c *hubConfig) { c.sendBuffer = n } }

// WithSlowConsumerPolicy picks what a full outbound queue does.
func WithSlowConsumerPolicy(p SlowConsumerPolicy) hubOption {
	return func(c *hubConfig) { c.slowConsumer = p }
}

func defaultHubConfig() hubConfig {
	return hubConfig{
		writeTimeout: defaultServerWriteTimeout,
		readTimeout:  defaultServerReadTimeout,
		sendBuffer:   defaultSendBuffer,
		readLimit:    defaultReadLimitBytes,
		checkOrigin:  sameOriginOrLoopback,
	}
}

// NewHub builds a hub.
func NewHub(m monitoring.Monitor, opts ...hubOption) *Hub {
	cfg := defaultHubConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Hub{
		cfg:     cfg,
		monitor: m,
		conns:   map[*serverConn]struct{}{},
		topics:  map[TopicType]map[*serverConn]struct{}{},
		seqs:    map[TopicType]*atomic.Int64{},
	}
}

// Handler upgrades requests and serves them until they close or the hub does.
func (h *Hub) Handler() http.Handler {
	upgrader := websocket.Upgrader{CheckOrigin: h.cfg.checkOrigin}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.authenticate != nil {
			if err := h.cfg.authenticate(r); err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
		}
		h.mu.RLock()
		closed := h.closed
		h.mu.RUnlock()
		if closed {
			http.Error(w, "hub is closed", http.StatusServiceUnavailable)
			return
		}
		raw, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		sc := &serverConn{
			hub:    h,
			info:   ConnInfo{ID: IntToString(h.nextID.Add(1)), Request: r},
			conn:   raw,
			send:   make(chan *Message, h.cfg.sendBuffer),
			done:   make(chan struct{}),
			topics: map[TopicType]struct{}{},
		}
		h.add(sc)
		sc.serve(r.Context())
	})
}

// Publish sends data to every subscriber of topic and returns how many received it.
func (h *Hub) Publish(topic TopicType, subject string, data any) int {
	h.mu.RLock()
	seq, ok := h.seqs[topic]
	subs := make([]*serverConn, 0, len(h.topics[topic]))
	for sc := range h.topics[topic] {
		subs = append(subs, sc)
	}
	h.mu.RUnlock()
	if !ok || len(subs) == 0 {
		return 0
	}
	msg := &Message{
		ID: IntToString(time.Now().UnixNano()), Type: MessageTypeMessage,
		SequenceNumber: seq.Add(1), Topic: topic, Subject: subject, Data: data,
	}
	delivered := 0
	for _, sc := range subs {
		if sc.enqueue(msg) {
			delivered++
		}
	}
	return delivered
}

// Subscribers counts the connections on a topic.
func (h *Hub) Subscribers(topic TopicType) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics[topic])
}

// Connections counts open connections.
func (h *Hub) Connections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Dropped counts frames discarded under the drop policy.
func (h *Hub) Dropped() int64 { return h.dropped.Load() }

// Close closes every connection and refuses new ones.
func (h *Hub) Close() {
	h.mu.Lock()
	h.closed = true
	conns := make([]*serverConn, 0, len(h.conns))
	for sc := range h.conns {
		conns = append(conns, sc)
	}
	h.mu.Unlock()
	for _, sc := range conns {
		sc.close()
	}
}

func (h *Hub) add(sc *serverConn) {
	h.mu.Lock()
	h.conns[sc] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(sc *serverConn) {
	h.mu.Lock()
	delete(h.conns, sc)
	released := make([]TopicType, 0, len(sc.topics))
	for topic := range sc.topics {
		if subs := h.topics[topic]; subs != nil {
			delete(subs, sc)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		}
		released = append(released, topic)
	}
	sc.topics = map[TopicType]struct{}{}
	h.mu.Unlock()
	if h.cfg.onUnsubscribe != nil {
		for _, topic := range released {
			h.cfg.onUnsubscribe(sc.info, topic)
		}
	}
}

func (h *Hub) subscribe(ctx context.Context, sc *serverConn, topic TopicType) error {
	if topic == "" {
		return errors.New("a subscription needs a topic")
	}
	h.mu.RLock()
	_, already := sc.topics[topic]
	h.mu.RUnlock()
	if already {
		return nil
	}
	if h.cfg.onSubscribe != nil {
		if err := h.cfg.onSubscribe(ctx, sc.info, topic); err != nil {
			return err
		}
	}
	h.mu.Lock()
	if h.topics[topic] == nil {
		h.topics[topic] = map[*serverConn]struct{}{}
	}
	if h.seqs[topic] == nil {
		h.seqs[topic] = &atomic.Int64{}
	}
	h.topics[topic][sc] = struct{}{}
	sc.topics[topic] = struct{}{}
	h.mu.Unlock()
	return nil
}

func (h *Hub) unsubscribe(sc *serverConn, topic TopicType) {
	h.mu.Lock()
	_, had := sc.topics[topic]
	delete(sc.topics, topic)
	if subs := h.topics[topic]; subs != nil {
		delete(subs, sc)
		if len(subs) == 0 {
			delete(h.topics, topic)
		}
	}
	h.mu.Unlock()
	if had && h.cfg.onUnsubscribe != nil {
		h.cfg.onUnsubscribe(sc.info, topic)
	}
}

func (sc *serverConn) serve(ctx context.Context) {
	defer sc.hub.remove(sc)
	defer sc.close()
	sc.conn.SetReadLimit(sc.hub.cfg.readLimit)

	go sc.writePump()
	if !sc.enqueue(&Message{ID: sc.info.ID, Type: MessageTypeWelcome}) {
		return
	}
	for {
		if err := sc.conn.SetReadDeadline(time.Now().Add(sc.hub.cfg.readTimeout)); err != nil {
			return
		}
		var m Message
		if err := sc.conn.ReadJSON(&m); err != nil {
			return
		}
		sc.handle(ctx, &m)
	}
}

func (sc *serverConn) handle(ctx context.Context, m *Message) {
	switch m.Type {
	case MessageTypePing:
		sc.enqueue(&Message{ID: m.ID, Type: MessageTypePong})
	case MessageTypeSubscribe:
		if err := sc.hub.subscribe(ctx, sc, m.Topic); err != nil {
			sc.enqueue(&Message{ID: m.ID, Type: MessageTypeError, Topic: m.Topic, Data: err.Error()})
			return
		}
		sc.enqueue(&Message{ID: m.ID, Type: MessageTypeAck, Topic: m.Topic})
	case MessageTypeUnsubscribe:
		sc.hub.unsubscribe(sc, m.Topic)
		sc.enqueue(&Message{ID: m.ID, Type: MessageTypeAck, Topic: m.Topic})
	case MessageTypeWelcome, MessageTypePong, MessageTypeAck, MessageTypeError, MessageTypeMessage:
		sc.enqueue(&Message{ID: m.ID, Type: MessageTypeError, Data: fmt.Sprintf("a client does not send %q", m.Type)})
	default:
		sc.enqueue(&Message{ID: m.ID, Type: MessageTypeError, Data: fmt.Sprintf("unknown message type %q", m.Type)})
	}
}

// enqueue offers a frame to the connection's writer and reports whether it was taken.
func (sc *serverConn) enqueue(m *Message) bool {
	select {
	case <-sc.done:
		return false
	default:
	}
	select {
	case sc.send <- m:
		return true
	default:
		if sc.hub.cfg.slowConsumer == SlowConsumerDrop {
			sc.hub.dropped.Add(1)
			return false
		}
		sc.hub.monitor.Logger().Warn("websocket: closing a slow consumer", "conn", sc.info.ID)
		sc.close()
		return false
	}
}

func (sc *serverConn) writePump() {
	for {
		select {
		case <-sc.done:
			return
		case m := <-sc.send:
			if err := sc.conn.SetWriteDeadline(time.Now().Add(sc.hub.cfg.writeTimeout)); err != nil {
				sc.close()
				return
			}
			raw, err := json.Marshal(m)
			if err != nil {
				sc.hub.monitor.Logger().Error("websocket: encode frame", "error", err)
				continue
			}
			if err := sc.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				sc.close()
				return
			}
		}
	}
}

func (sc *serverConn) close() {
	sc.once.Do(func() {
		close(sc.done)
		_ = sc.conn.Close()
	})
}

func sameOriginOrLoopback(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == "http://"+r.Host || origin == "https://"+r.Host {
		return true
	}
	for _, prefix := range []string{"http://127.0.0.1", "http://localhost", "http://[::1]", "wails://", "file://"} {
		if len(origin) >= len(prefix) && origin[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
