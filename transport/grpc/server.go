// Package grpc provides everything needed to build a gRPC server and client
package grpc

import (
	"crypto/tls"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

const (
	// defaultMaxRecvMsgSize matches the gRPC stdlib (4 MiB) but is set
	// explicitly so operators see the value in code and a deviation is
	// always intentional.
	defaultMaxRecvMsgSize = 4 * 1024 * 1024
	// defaultMaxSendMsgSize defaults to a generous 16 MiB for typical
	// service responses. grpc-go default is math.MaxInt32; capping
	// prevents accidental gigantic responses from blowing through TCP
	// send buffers.
	defaultMaxSendMsgSize = 16 * 1024 * 1024
	// defaultMaxConcurrentStreams caps HTTP/2 streams per connection.
	defaultMaxConcurrentStreams = 1024
)

type serverOption func(*serverConfig)

type serverConfig struct {
	tlsConfig            *tls.Config
	network              string
	address              string
	controllers          []Controller
	middlewares          []Middleware
	streamMiddlewares    []grpc.StreamServerInterceptor
	keepalive            *keepalive.ServerParameters
	enforcementPolicy    *keepalive.EnforcementPolicy
	reflectionEnabled    bool
	maxRecvMsgSize       int
	maxSendMsgSize       int
	maxConcurrentStreams uint32
}

func defaultServerControllers(monitor monitoring.Monitor) []Controller {
	return []Controller{NewHealthServer(monitor)}
}

func defaultServerMiddlewares() []Middleware {
	return []Middleware{}
}

// defaultKeepalive picks values appropriate for typical microservice
// deployments. The previous defaults (15s idle / 5s ping / 1s timeout)
// caused HTTP/2 ping storms under low traffic and churned connections on
// otherwise-healthy networks.
func defaultKeepalive() *keepalive.ServerParameters {
	return &keepalive.ServerParameters{
		MaxConnectionIdle: 15 * time.Minute,
		Time:              30 * time.Second,
		Timeout:           10 * time.Second,
	}
}

// defaultEnforcementPolicy refuses clients that ping faster than once
// every 5s, which would otherwise be cheap DoS against the server.
func defaultEnforcementPolicy() *keepalive.EnforcementPolicy {
	return &keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: false,
	}
}

func defaultServerOpts(monitor monitoring.Monitor) []serverOption {
	return []serverOption{
		WithControllers(defaultServerControllers(monitor)...),
		WithMiddlewares(defaultServerMiddlewares()...),
		WithKeepalive(defaultKeepalive()),
		WithEnforcementPolicy(defaultEnforcementPolicy()),
		WithMaxRecvMsgSize(defaultMaxRecvMsgSize),
		WithMaxSendMsgSize(defaultMaxSendMsgSize),
		WithMaxConcurrentStreams(defaultMaxConcurrentStreams),
		withAddrFromEnv(),
	}
}

// WithTLSConfig sets the TLS configuration for the server
func WithTLSConfig(config *tls.Config) serverOption {
	return func(cfg *serverConfig) {
		cfg.tlsConfig = config
	}
}

// WithNetwork sets the network the server will listen to (tcp, tcp4, tcp6, unix)
func WithNetwork(network string) serverOption {
	return func(cfg *serverConfig) {
		cfg.network = network
	}
}

// WithAddress sets the address the server will listen to (e.g., ":50051", "localhost:50051")
func WithAddress(address string) serverOption {
	return func(cfg *serverConfig) {
		cfg.address = address
	}
}

// WithKeepalive sets keepalive parameters to prevent zombie connections
func WithKeepalive(params *keepalive.ServerParameters) serverOption {
	return func(cfg *serverConfig) {
		cfg.keepalive = params
	}
}

// WithEnforcementPolicy sets the server-side keepalive enforcement policy
// that controls how aggressively misbehaving clients can ping.
func WithEnforcementPolicy(p *keepalive.EnforcementPolicy) serverOption {
	return func(cfg *serverConfig) {
		cfg.enforcementPolicy = p
	}
}

// WithReflection toggles the gRPC server reflection API. Defaults to off
// because reflection exposes the entire service surface — useful in dev,
// risky in prod.
func WithReflection(enabled bool) serverOption {
	return func(cfg *serverConfig) {
		cfg.reflectionEnabled = enabled
	}
}

// WithMaxRecvMsgSize caps the size of incoming messages the server will
// accept. Messages larger than this fail with ResourceExhausted.
func WithMaxRecvMsgSize(n int) serverOption {
	return func(cfg *serverConfig) {
		cfg.maxRecvMsgSize = n
	}
}

// WithMaxSendMsgSize caps the size of outgoing messages.
func WithMaxSendMsgSize(n int) serverOption {
	return func(cfg *serverConfig) {
		cfg.maxSendMsgSize = n
	}
}

// WithMaxConcurrentStreams caps the number of concurrent HTTP/2 streams
// per connection. 0 means use the gRPC default (math.MaxUint32).
func WithMaxConcurrentStreams(n uint32) serverOption {
	return func(cfg *serverConfig) {
		cfg.maxConcurrentStreams = n
	}
}

// WithStreamMiddlewares appends stream interceptors to the chain applied
// to every streaming RPC. Unary handlers are unaffected.
func WithStreamMiddlewares(interceptors ...grpc.StreamServerInterceptor) serverOption {
	return func(cfg *serverConfig) {
		cfg.streamMiddlewares = append(cfg.streamMiddlewares, interceptors...)
	}
}

func withAddrFromEnv() serverOption {
	addr := os.Getenv("GRPC_ADDRESS")
	if addr == "" {
		return func(cfg *serverConfig) {} // No-op if env var not set
	}
	return WithAddress(addr)
}

// WithMiddlewares adds the provided middlewares to the middleware list
func WithMiddlewares(middlewares ...Middleware) serverOption {
	return func(cfg *serverConfig) {
		cfg.middlewares = append(cfg.middlewares, middlewares...)
	}
}

// WithControllers adds the provided controllers to the controllers list
func WithControllers(controllers ...Controller) serverOption {
	return func(cfg *serverConfig) {
		cfg.controllers = append(cfg.controllers, controllers...)
	}
}

// Server wraps grpc.Server with additional functionality
type Server struct {
	*grpc.Server // Embedded as pointer (not value)
	listener     net.Listener
}

// Start starts serving gRPC requests
func (s *Server) Start() error {
	return s.Serve(s.listener)
}

// Addr returns the listener's network address
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// NewServer creates a new gRPC server with the given options
func NewServer(monitor monitoring.Monitor, opts ...serverOption) (*Server, error) {
	grpcOpts := make([]grpc.ServerOption, 0)
	cfg := &serverConfig{
		network: "tcp",
		address: ":50051", // gRPC standard port
	}

	for _, opt := range append(defaultServerOpts(monitor), opts...) {
		opt(cfg)
	}

	// Add keepalive if configured
	if cfg.keepalive != nil {
		grpcOpts = append(grpcOpts, grpc.KeepaliveParams(*cfg.keepalive))
	}
	if cfg.enforcementPolicy != nil {
		grpcOpts = append(grpcOpts, grpc.KeepaliveEnforcementPolicy(*cfg.enforcementPolicy))
	}
	if cfg.maxRecvMsgSize > 0 {
		grpcOpts = append(grpcOpts, grpc.MaxRecvMsgSize(cfg.maxRecvMsgSize))
	}
	if cfg.maxSendMsgSize > 0 {
		grpcOpts = append(grpcOpts, grpc.MaxSendMsgSize(cfg.maxSendMsgSize))
	}
	if cfg.maxConcurrentStreams > 0 {
		grpcOpts = append(grpcOpts, grpc.MaxConcurrentStreams(cfg.maxConcurrentStreams))
	}

	// Setup listener
	var lis net.Listener
	var err error

	if cfg.tlsConfig == nil {
		lis, err = net.Listen(cfg.network, cfg.address)
		if err != nil {
			return nil, err
		}
	} else {
		lis, err = tls.Listen(cfg.network, cfg.address, cfg.tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	// Build interceptor chain once at construction. grpc.ChainUnaryInterceptor
	// composes the chain inside grpc-go so we don't allocate a fresh closure
	// chain per RPC the way the bespoke ChainUnaryServer did.
	if len(cfg.middlewares) > 0 {
		grpcOpts = append(grpcOpts, grpc.ChainUnaryInterceptor(adaptMiddlewares(cfg.middlewares)...))
	}
	if len(cfg.streamMiddlewares) > 0 {
		grpcOpts = append(grpcOpts, grpc.ChainStreamInterceptor(cfg.streamMiddlewares...))
	}

	s := &Server{
		Server:   grpc.NewServer(grpcOpts...),
		listener: lis,
	}

	// Register controllers
	for _, c := range cfg.controllers {
		s.RegisterService(c.SD(), c)
	}

	if cfg.reflectionEnabled {
		reflection.Register(s.Server)
	}

	return s, nil
}

// adaptMiddlewares turns the package's Middleware interface into the
// grpc.UnaryServerInterceptor signature so the stdlib's chain builder can
// compose them.
func adaptMiddlewares(ms []Middleware) []grpc.UnaryServerInterceptor {
	out := make([]grpc.UnaryServerInterceptor, len(ms))
	for i, m := range ms {
		m := m
		out[i] = m.Intercept
	}
	return out
}
