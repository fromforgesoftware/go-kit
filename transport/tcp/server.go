package tcp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// SplitterFactory builds a bufio.SplitFunc for a single connection. Use it
// when framing depends on per-connection state — a stream cipher over the
// packet header, a negotiated frame size — which a shared SplitFunc cannot
// hold. It is called once per accepted connection, before the read loop
// starts, so the returned closure owns its state exclusively.
type SplitterFactory func(Session) bufio.SplitFunc

// Server represents a TCP server
type Server struct {
	address           string
	monitor           monitoring.Monitor
	handler           Handler
	packetSplitter    bufio.SplitFunc
	splitterFactory   SplitterFactory
	splitterOwnsToken bool

	readTimeout     time.Duration
	writeTimeout    time.Duration
	readBufferSize  int
	writeBufferSize int
	maxConnections  int
	sendPolicy      SendOverflowPolicy

	// Hooks
	onConnect    func(Session)
	onDisconnect func(Session)

	listener net.Listener
	mu       sync.RWMutex
	sessions map[uuid.UUID]*session
	readPool *sync.Pool
	connCnt  atomic.Int64

	shutdown     chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup

	controllers []Controller
	middlewares []Middleware
}

type serverConfig struct {
	packetSplitter    bufio.SplitFunc
	splitterFactory   SplitterFactory
	splitterOwnsToken bool
	readBufferSize    int
	writeBufferSize   int
	readTimeout       time.Duration
	writeTimeout      time.Duration
	maxConnections    int
	sendPolicy        SendOverflowPolicy
	onConnect         func(Session)
	onDisconnect      func(Session)
	address           string
	handler           Handler
	controllers       []Controller
	middlewares       []Middleware
}

// serverOption allows configuring the server
type serverOption func(*serverConfig)

// defaultServerOpts returns the default options
func defaultServerOpts() []serverOption {
	return []serverOption{
		WithPacketSplitter(bufio.ScanLines),
		WithReadBufferSize(4096),
		WithWriteBufferSize(128),
		withAddrFromEnv(),
	}
}

// WithPacketSplitter sets the packet splitter
func WithPacketSplitter(splitter bufio.SplitFunc) serverOption {
	return func(s *serverConfig) {
		s.packetSplitter = splitter
	}
}

// WithPacketSplitterFactory installs a per-connection splitter. It takes
// precedence over WithPacketSplitter, which cannot express framing that
// depends on connection state.
func WithPacketSplitterFactory(f SplitterFactory) serverOption {
	return func(s *serverConfig) {
		s.splitterFactory = f
	}
}

// WithSplitterOwnedTokens declares that the splitter returns tokens backed by
// memory it allocated itself rather than by the scanner's reusable buffer, so
// the server can hand them to the handler without copying. Setting this for a
// splitter that returns sub-slices of its input corrupts packets under load.
func WithSplitterOwnedTokens() serverOption {
	return func(s *serverConfig) {
		s.splitterOwnsToken = true
	}
}

// WithReadTimeout sets the read timeout
func WithReadTimeout(d time.Duration) serverOption {
	return func(s *serverConfig) {
		s.readTimeout = d
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(d time.Duration) serverOption {
	return func(s *serverConfig) {
		s.writeTimeout = d
	}
}

// WithWriteDuration is deprecated: use WithWriteTimeout. Kept as an alias
// to avoid breaking existing callers.
func WithWriteDuration(d time.Duration) serverOption {
	return WithWriteTimeout(d)
}

// WithSendOverflowPolicy controls what Session.Send does when the internal
// queue is full. Default is SendOverflowError.
func WithSendOverflowPolicy(p SendOverflowPolicy) serverOption {
	return func(s *serverConfig) {
		s.sendPolicy = p
	}
}

// WithReadBufferSize sets the read buffer size
func WithReadBufferSize(size int) serverOption {
	return func(s *serverConfig) {
		s.readBufferSize = size
	}
}

// WithWriteBufferSize sets the write channel buffer size
func WithWriteBufferSize(size int) serverOption {
	return func(s *serverConfig) {
		s.writeBufferSize = size
	}
}

// WithMaxConnections sets the maximum number of concurrent connections
func WithMaxConnections(max int) serverOption {
	return func(s *serverConfig) {
		s.maxConnections = max
	}
}

// WithOnConnect sets the onConnect hook
func WithOnConnect(f func(Session)) serverOption {
	return func(s *serverConfig) {
		s.onConnect = f
	}
}

// WithOnDisconnect sets the onDisconnect hook
func WithOnDisconnect(f func(Session)) serverOption {
	return func(s *serverConfig) {
		s.onDisconnect = f
	}
}

// WithAddress sets the server address
func WithAddress(addr string) serverOption {
	return func(s *serverConfig) {
		s.address = addr
	}
}

// WithHandler sets the packet handler
func WithHandler(handler Handler) serverOption {
	return func(s *serverConfig) {
		s.handler = handler
	}
}

// WithControllers registers controllers with the server
func WithControllers(controllers ...Controller) serverOption {
	return func(s *serverConfig) {
		s.controllers = append(s.controllers, controllers...)
	}
}

// WithMiddlewares adds middlewares to the server
func WithMiddlewares(middlewares ...Middleware) serverOption {
	return func(s *serverConfig) {
		s.middlewares = append(s.middlewares, middlewares...)
	}
}

func withAddrFromEnv() serverOption {
	addr := os.Getenv("TCP_ADDRESS")
	if addr == "" {
		return func(s *serverConfig) {}
	}
	return WithAddress(addr)
}

// NewServer creates a new TCP server
func NewServer(monitor monitoring.Monitor, opts ...serverOption) (*Server, error) {
	cfg := &serverConfig{}
	for _, opt := range append(defaultServerOpts(), opts...) {
		opt(cfg)
	}

	s := &Server{
		monitor:           monitor,
		sessions:          make(map[uuid.UUID]*session),
		shutdown:          make(chan struct{}),
		packetSplitter:    cfg.packetSplitter,
		splitterFactory:   cfg.splitterFactory,
		splitterOwnsToken: cfg.splitterOwnsToken,
		readBufferSize:    cfg.readBufferSize,
		writeBufferSize:   cfg.writeBufferSize,
		readTimeout:       cfg.readTimeout,
		writeTimeout:      cfg.writeTimeout,
		maxConnections:    cfg.maxConnections,
		sendPolicy:        cfg.sendPolicy,
		onConnect:         cfg.onConnect,
		onDisconnect:      cfg.onDisconnect,
		address:           cfg.address,
		handler:           cfg.handler,
		controllers:       cfg.controllers,
		middlewares:       cfg.middlewares,
	}

	// sync.Pool stores pointers — passing the slice value directly causes
	// the slice header to be allocated on every Put. Boxing as *[]byte is
	// the canonical workaround (see staticcheck SA6002).
	s.readPool = &sync.Pool{
		New: func() interface{} {
			b := make([]byte, s.readBufferSize)
			return &b
		},
	}

	// Validate required dependencies
	if s.handler == nil {
		return nil, fmt.Errorf("TCP server requires a Handler")
	}

	// Register controllers if handler supports it
	if len(s.controllers) > 0 {
		registry, ok := s.handler.(Registry)
		if !ok {
			return nil, fmt.Errorf("TCP Controllers provided but Handler (%T) does not implement tcp.Registry", s.handler)
		}
		for _, c := range s.controllers {
			c.Register(registry)
		}
	}

	// Apply Middlewares
	h := s.handler
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		h = s.middlewares[i](h)
	}
	s.handler = h

	return s, nil
}

// SetOnConnect registers a hook fired after each new Session is accepted.
func (s *Server) SetOnConnect(f func(Session)) {
	s.onConnect = f
}

func (s *Server) SetOnDisconnect(f func(Session)) {
	s.onDisconnect = f
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	s.monitor.Logger().Info("TCP server started", "address", s.listener.Addr().String())

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *Server) Addr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

func (s *Server) Stop() error {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
	})

	s.mu.RLock()
	listener := s.listener
	s.mu.RUnlock()
	if listener != nil {
		listener.Close()
	}

	// Close the live sessions before waiting: a connection with nothing to say
	// is blocked in Scan, and nothing but its socket closing will wake it.
	s.mu.RLock()
	live := make([]*session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		live = append(live, sess)
	}
	s.mu.RUnlock()
	for _, sess := range live {
		sess.Close()
	}

	s.wg.Wait()
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	// backoff mirrors http.Server.Serve's strategy for transient accept
	// errors (typically EMFILE / ECONNABORTED). Without it a runaway fd
	// exhaustion pins one core at 100% spinning on Accept.
	var tempDelay time.Duration
	const maxDelay = time.Second

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
			}

			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if tempDelay > maxDelay {
					tempDelay = maxDelay
				}
				s.monitor.Logger().Warn("accept temporary error, backing off",
					"error", err, "delay", tempDelay)
				select {
				case <-time.After(tempDelay):
				case <-s.shutdown:
					return
				}
				continue
			}

			s.monitor.Logger().Error("accept error", "error", err)
			continue
		}
		tempDelay = 0

		if s.maxConnections > 0 && s.connCnt.Load() >= int64(s.maxConnections) {
			s.monitor.Logger().Warn("tcp connection cap reached, closing accepted conn",
				"cap", s.maxConnections, "remote", conn.RemoteAddr())
			conn.Close()
			continue
		}

		// Registered on the WaitGroup so Stop waits for the connection to
		// finish dispatching. Without it Stop returns while handlers are still
		// running, and a caller that tears down what a handler is using — a
		// test's mock, a repository — races it.
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()

	s.connCnt.Add(1)
	defer s.connCnt.Add(-1)

	session := newSession(conn, s.writeBufferSize, s.writeTimeout, s.sendPolicy)
	session.Start()

	s.mu.Lock()
	s.sessions[session.id] = session
	s.mu.Unlock()

	if s.onConnect != nil {
		s.onConnect(session)
	}

	defer func() {
		// Close the session first so Sends issued from the hook return
		// ErrSessionClosed instead of buffering into a queue that's about
		// to be reclaimed.
		session.Close()

		s.mu.Lock()
		delete(s.sessions, session.id)
		s.mu.Unlock()

		if s.onDisconnect != nil {
			s.onDisconnect(session)
		}
	}()

	// Read Loop — readPool holds *[]byte so Put doesn't allocate.
	bufPtr := s.readPool.Get().(*[]byte)
	defer s.readPool.Put(bufPtr)
	buf := *bufPtr

	// A factory-built splitter owns state for this connection alone; the
	// shared splitter is the fallback. Built before the first Scan so the
	// closure is in place for the very first packet.
	splitter := s.packetSplitter
	if s.splitterFactory != nil {
		splitter = s.splitterFactory(session)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(buf, bufio.MaxScanTokenSize)
	scanner.Split(splitter)

	for {
		if s.readTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(s.readTimeout))
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
					s.monitor.Logger().Debug("scan error", "error", err, "remote", conn.RemoteAddr())
				}
			}
			return
		}

		// Handle packet
		payload := scanner.Bytes()
		if !s.splitterOwnsToken {
			// The scanner reuses its buffer, so the token is only valid until
			// the next Scan — copy unless the splitter allocated it.
			cp := make([]byte, len(payload))
			copy(cp, payload)
			payload = cp
		}

		s.dispatchPacket(session, payload)
	}
}

// dispatchPacket runs the user handler under a per-packet server span and
// panic recovery. A panicking handler used to take down the whole conn
// goroutine; now it logs + records the panic on the span and continues.
func (s *Server) dispatchPacket(session *session, payload []byte) {
	ctx, span := s.monitor.Tracer().Start(session.Context(), "tcp.handle",
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithAttributes(
			tracer.String("net.transport", "tcp"),
			tracer.String("net.peer.addr", session.RemoteAddr().String()),
			tracer.Int("net.payload.bytes", len(payload)),
		),
	)
	defer span.End()

	defer func() {
		if r := recover(); r != nil {
			s.monitor.Logger().
				WithKeysAndValues("session", session.id.String(), "panic", r, "stack", string(debug.Stack())).
				ErrorContext(ctx, "tcp handler panic")
			span.RecordError(asError(r))
			span.SetStatus(tracer.StatusError, "handler panic")
		}
	}()

	if err := s.handler.Handle(ctx, session, payload); err != nil {
		s.monitor.Logger().Error("handle error", "error", err)
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
	}
}

func asError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("%v", v)
}
