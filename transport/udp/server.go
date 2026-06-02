package udp

import (
	"context"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

const (
	// DefaultSessionIdleTimeout reaps sessions silent for longer than this.
	// UDP "sessions" are address-keyed, so without expiry a spoofed-address
	// spray is a trivial OOM. 60s is short enough to bound memory but long
	// enough to survive a brief client pause.
	DefaultSessionIdleTimeout = 60 * time.Second

	// DefaultMaxSessions caps the in-memory session table to bound memory
	// in worst-case DoS scenarios. Pick a number above your real peak.
	DefaultMaxSessions = 100_000

	reaperInterval = 5 * time.Second
)

// OnConnectHook is a callback when a new session is established
type OnConnectHook func(context.Context, Session)

type serverConfig struct {
	addrStr            string
	handler            Handler
	onConnect          OnConnectHook
	readBufferSize     int
	controllers        []Controller
	middlewares        []Middleware
	sessionIdleTimeout time.Duration
	maxSessions        int
}

// serverOption represents a functional option for configuring the server
type serverOption func(*serverConfig)

// defaultServerOpts returns the default options
func defaultServerOpts() []serverOption {
	return []serverOption{
		WithReadBufferSize(4096),
		WithSessionIdleTimeout(DefaultSessionIdleTimeout),
		WithMaxSessions(DefaultMaxSessions),
	}
}

// WithAddress sets the address for the server
func WithAddress(addr string) serverOption {
	return func(c *serverConfig) {
		c.addrStr = addr
	}
}

// WithAddressFromEnv sets the address from environment variable
func WithAddressFromEnv(envVar string) serverOption {
	return func(c *serverConfig) {
		if addr := os.Getenv(envVar); addr != "" {
			c.addrStr = addr
		}
	}
}

// WithHandler sets the handler for the server
func WithHandler(h Handler) serverOption {
	return func(c *serverConfig) {
		c.handler = h
	}
}

// WithControllers adds controllers to the server
func WithControllers(controllers ...Controller) serverOption {
	return func(c *serverConfig) {
		c.controllers = append(c.controllers, controllers...)
	}
}

// WithMiddlewares adds middlewares to the server
func WithMiddlewares(middlewares ...Middleware) serverOption {
	return func(c *serverConfig) {
		c.middlewares = append(c.middlewares, middlewares...)
	}
}

// WithOnConnect sets the callback for new connections
func WithOnConnect(hook OnConnectHook) serverOption {
	return func(c *serverConfig) {
		c.onConnect = hook
	}
}

// WithReadBufferSize sets the read buffer size
func WithReadBufferSize(size int) serverOption {
	return func(c *serverConfig) {
		c.readBufferSize = size
	}
}

// WithSessionIdleTimeout sets how long a session may sit silent before the
// reaper closes it. Zero disables reaping.
func WithSessionIdleTimeout(d time.Duration) serverOption {
	return func(c *serverConfig) {
		c.sessionIdleTimeout = d
	}
}

// WithMaxSessions caps the number of concurrent sessions. Excess packets
// from new addresses are dropped. Zero disables the cap.
func WithMaxSessions(n int) serverOption {
	return func(c *serverConfig) {
		c.maxSessions = n
	}
}

// Server represents a UDP server
type Server struct {
	addrStr string
	conn    *net.UDPConn
	handler Handler
	monitor monitoring.Monitor

	sessions    sync.Map // map[string]*session (key: remoteAddr.String())
	sessionCnt  atomic.Int64
	maxSessions int

	sessionIdleTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Options
	readBufferSize int
	onConnect      OnConnectHook
}

// NewServer creates a new UDP Server
func NewServer(monitor monitoring.Monitor, opts ...serverOption) (*Server, error) {
	cfg := &serverConfig{}
	for _, opt := range append(defaultServerOpts(), opts...) {
		opt(cfg)
	}

	// Default Handler if none provided
	if cfg.handler == nil {
		cfg.handler = NewMux()
	}

	// Register Controllers if Handler is a Registry
	if registry, ok := cfg.handler.(Registry); ok {
		for _, c := range cfg.controllers {
			c.Register(registry)
		}
	}

	// Apply Middlewares
	h := cfg.handler
	for i := len(cfg.middlewares) - 1; i >= 0; i-- {
		h = cfg.middlewares[i](h)
	}

	s := &Server{
		addrStr:            cfg.addrStr,
		handler:            h,
		monitor:            monitor,
		readBufferSize:     cfg.readBufferSize,
		onConnect:          cfg.onConnect,
		sessionIdleTimeout: cfg.sessionIdleTimeout,
		maxSessions:        cfg.maxSessions,
	}

	// Default logging hook
	if s.onConnect == nil && s.monitor != nil {
		s.onConnect = func(ctx context.Context, sess Session) {
			s.monitor.Logger().Debug("UDP Session created", "addr", sess.RemoteAddr().String())
		}
	}

	return s, nil
}

// Start starts the UDP listener
func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.addrStr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	s.conn = conn
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.monitor.Logger().Info("UDP server started", "address", addr.String())

	s.wg.Add(2)
	go s.readLoop()
	go s.resendLoop()

	if s.sessionIdleTimeout > 0 {
		s.wg.Add(1)
		go s.reapLoop()
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	s.wg.Wait()

	// Close all live sessions so their dispatch goroutines exit cleanly.
	s.sessions.Range(func(_, value any) bool {
		value.(*session).Close()
		return true
	})
	return nil
}

func (s *Server) readLoop() {
	defer s.wg.Done()

	buf := make([]byte, s.readBufferSize)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// ReadFromUDP
			n, remoteAddr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				// Check if closed
				select {
				case <-s.ctx.Done():
					return
				default:
					s.monitor.Logger().Error("udp read error", "error", err)
					continue
				}
			}

			// Copy data to avoid buffer overwrite
			data := make([]byte, n)
			copy(data, buf[:n])

			// Decode Packet
			pkt, err := Unmarshal(data)
			if err != nil {
				// Malformed packet
				continue
			}

			// Get/Create Session — may be nil if we're over the cap.
			sess := s.getSession(remoteAddr)
			if sess == nil {
				continue
			}

			// Process Reliability (Update ACKs, etc)
			if err := sess.ProcessPacket(pkt); err != nil {
				continue
			}

			// If it's pure data (Unreliable or Reliable), hand off to the
			// per-session dispatch worker. enqueue is non-blocking — drops
			// rather than stalling the read loop on a slow client.
			if len(pkt.Payload) > 0 {
				sess.enqueue(pkt.Payload)
			}
		}
	}
}

func (s *Server) resendLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.sessions.Range(func(key, value interface{}) bool {
				sess := value.(*session)
				sess.CheckResends()
				return true
			})
		}
	}
}

// reapLoop closes sessions that haven't been seen recently. Without this,
// an address-spoofing spray fills the session table until the process
// runs out of memory.
func (s *Server) reapLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.sessions.Range(func(_, value any) bool {
				sess := value.(*session)
				if sess.idleFor(now) > s.sessionIdleTimeout {
					s.monitor.Logger().Debug("reaping idle UDP session",
						"id", sess.id, "addr", sess.remoteAddr.String())
					sess.Close()
					s.sessionCnt.Add(-1)
				}
				return true
			})
		}
	}
}

// getSession returns the session for addr, creating one if necessary. Returns
// nil if the per-server session cap would be exceeded.
func (s *Server) getSession(addr *net.UDPAddr) *session {
	key := addr.String()
	if val, ok := s.sessions.Load(key); ok {
		return val.(*session)
	}

	if s.maxSessions > 0 && s.sessionCnt.Load() >= int64(s.maxSessions) {
		s.monitor.Logger().Warn("udp session cap reached, refusing new session",
			"cap", s.maxSessions, "addr", key)
		return nil
	}

	sess := newSession(s.ctx, s, addr)
	if existing, loaded := s.sessions.LoadOrStore(key, sess); loaded {
		// Lost the race — close the loser and use the winner.
		sess.Close()
		return existing.(*session)
	}
	s.sessionCnt.Add(1)

	// Execute Hook
	if s.onConnect != nil {
		s.onConnect(s.ctx, sess)
	}

	return sess
}

func (s *Server) writeTo(data []byte, addr *net.UDPAddr) error {
	s.monitor.Logger().Debug("UDP Write", "dest", addr.String(), "len", len(data))
	_, err := s.conn.WriteToUDP(data, addr)
	return err
}
