package rest

import (
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"github.com/fromforgesoftware/go-kit/openapi"
)

const (
	// defaultReadHeaderTimeout caps the time a slow client can take to
	// send request headers. Defends against Slowloris-style stalls.
	defaultReadHeaderTimeout = 5 * time.Second

	// defaultReadTimeout caps the time the full request body can take to
	// arrive. Generous enough for normal uploads, short enough to drop
	// hung connections.
	defaultReadTimeout = 30 * time.Second

	// defaultWriteTimeout caps the response duration. 0 disables it
	// entirely — required for streaming / SSE endpoints. Configure via
	// WithWriteTimeout(0) when registering streaming handlers.
	defaultWriteTimeout = 30 * time.Second

	// defaultIdleTimeout reaps keep-alive connections that sit silent.
	// Without it idle conns linger until the OS reaps them and a sustained
	// keep-alive client base monotonically grows the fd count.
	defaultIdleTimeout = 120 * time.Second

	// defaultMaxHeaderBytes mirrors http.DefaultMaxHeaderBytes but is set
	// explicitly so the value is visible to operators and a deviation is
	// always intentional.
	defaultMaxHeaderBytes = 1 << 20
)

// NewServer creates a new HTTP server with the given options.
//
// Route registration is unified through a single root Router:
//
//   - WithControllers(...) — each Controller's Routes(Router) method
//     registers grouped routes + scoped middleware on the root router.
//   - WithEndpoints(...) — singleton handlers (health, readiness,
//     anything truly route-less). Registered at the root.
//
// Global cfg.middlewares wrap the entire mux as the outermost layer,
// so they run before routing — important for RequestID, Tracing,
// Recovery which need to see 404s and 405s too. Ordering matches the
// Router's Use(): the first middleware in the slice is the outermost
// runtime layer (it runs first, returns last).
func NewServer(opts ...serverOption) *http.Server {
	cfg := new(serverConfig)

	for _, opt := range append(defaultServerOpts(), opts...) {
		opt(cfg)
	}

	root := NewRouter().(*router)

	// Controllers declare their grouped routes via the Router API.
	for _, c := range cfg.controllers {
		c.Routes(root)
	}

	// Standalone endpoints (health, readiness, ...).
	for _, e := range cfg.endpoints {
		root.Method(e.Method(), e.Path(), e)
	}

	// Build the OpenAPI collector if WithOpenAPI was supplied, then
	// register the spec + UI routes. They're registered like any
	// other route, but the Collector is told to skip them so they
	// don't appear in the published spec as API endpoints.
	var collector *openapi.Collector
	if cfg.openAPI != nil {
		c, err := openapi.NewCollector(openapi.NewReflector(), *cfg.openAPI)
		if err == nil {
			collector = c
			specPath := c.SpecConfig().SpecPath
			uiPath := c.SpecConfig().UIPath
			title := c.SpecConfig().Title
			root.Method(http.MethodGet, specPath, c)
			// Swagger UI's static-asset bundler expects to see the
			// full request path (it routes its own CSS/JS sub-paths
			// internally), so we don't strip the prefix the way
			// Router.Mount would. Two registrations cover both the
			// "/docs" and "/docs/anything" cases.
			uiHandler := openapi.NewUIHandler(title, specPath, uiPath, c.SpecConfig().UIRenderer)
			root.Method(http.MethodGet, uiPath, uiHandler)
			root.Method(http.MethodGet, uiPath+"/", uiHandler)

			// Hide kit infrastructure from the published API surface.
			c.SkipPath(specPath)
			c.SkipPath(uiPath)
			c.SkipPath(uiPath + "/")
			// Default health endpoints (see defaultEndpoints).
			c.SkipPath("/healthz")
			c.SkipPath("/healthz/")
		}
	}

	mux := http.NewServeMux()
	root.build(mux, collector)

	// Global middlewares wrap the whole mux so RequestID / Tracing /
	// Recovery cover 404 + 405 responses too. First in slice =
	// outermost runtime layer — same as Router.Use(), so behaviour
	// is consistent whether middleware is declared at server level or
	// inside a controller's Routes().
	handler := wrapOuterFirst(http.Handler(mux), cfg.middlewares)

	return &http.Server{
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		ReadTimeout:       cfg.readTimeout,
		WriteTimeout:      cfg.writeTimeout,
		IdleTimeout:       cfg.idleTimeout,
		MaxHeaderBytes:    cfg.maxHeaderBytes,
		Addr:              cfg.address,
		Handler:           handler,
		TLSConfig:         cfg.tlsConfig,
	}
}

type serverOption func(*serverConfig)

type serverConfig struct {
	tlsConfig         *tls.Config
	address           string
	shutdownTimeout   time.Duration
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	maxHeaderBytes    int
	maxConnections    int
	controllers       []Controller
	endpoints         []Endpoint
	middlewares       []Middleware
	openAPI           *openapi.SpecConfig
}

func defaultEndpoints() []Endpoint {
	return []Endpoint{
		NewEndpoint(http.MethodGet, "/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
		NewEndpoint(http.MethodGet, "/healthz/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	}
}

func defaultServerOpts() []serverOption {
	return []serverOption{
		WithEndpoints(defaultEndpoints()...),
		WithReadHeaderTimeout(defaultReadHeaderTimeout),
		WithReadTimeout(defaultReadTimeout),
		WithWriteTimeout(defaultWriteTimeout),
		WithIdleTimeout(defaultIdleTimeout),
		WithMaxHeaderBytes(defaultMaxHeaderBytes),
		withAddrFromEnv(),
	}
}

// WithTLSConfig sets the TLS configuration of the inner *http.Server
func WithTLSConfig(config *tls.Config) serverOption {
	return func(cfg *serverConfig) {
		cfg.tlsConfig = config
	}
}

// WithAddress sets the address the inner *http.Server will listen to
func WithAddress(address string) serverOption {
	return func(cfg *serverConfig) {
		cfg.address = address
	}
}

func withAddrFromEnv() serverOption {
	return WithAddress(os.Getenv("REST_ADDRESS"))
}

// WithShutdownTimeout sets the shutdown deadline
func WithShutdownTimeout(shutdownTimeout time.Duration) serverOption {
	return func(cfg *serverConfig) {
		cfg.shutdownTimeout = shutdownTimeout
	}
}

// WithReadHeaderTimeout caps the time a client may take to send the
// request line + headers. Defaults to defaultReadHeaderTimeout.
func WithReadHeaderTimeout(d time.Duration) serverOption {
	return func(cfg *serverConfig) {
		cfg.readHeaderTimeout = d
	}
}

// WithReadTimeout caps the time taken to read the full request body.
// Defaults to defaultReadTimeout. Set 0 to disable.
func WithReadTimeout(d time.Duration) serverOption {
	return func(cfg *serverConfig) {
		cfg.readTimeout = d
	}
}

// WithWriteTimeout caps the time taken to write the response. Defaults
// to defaultWriteTimeout. Set 0 to disable — required for SSE, long-poll
// or any other streaming response.
func WithWriteTimeout(d time.Duration) serverOption {
	return func(cfg *serverConfig) {
		cfg.writeTimeout = d
	}
}

// WithIdleTimeout caps how long an idle keep-alive connection can sit
// before the server closes it.
func WithIdleTimeout(d time.Duration) serverOption {
	return func(cfg *serverConfig) {
		cfg.idleTimeout = d
	}
}

// WithMaxHeaderBytes overrides the maximum size of request headers.
func WithMaxHeaderBytes(n int) serverOption {
	return func(cfg *serverConfig) {
		cfg.maxHeaderBytes = n
	}
}

// WithMaxConnections caps the number of concurrent accepted connections.
// 0 disables the cap. Read by the fx module wrapper which wraps the
// listener with netutil.LimitListener.
func WithMaxConnections(n int) serverOption {
	return func(cfg *serverConfig) {
		cfg.maxConnections = n
	}
}

// WithMiddlewares adds the provided rest middlewares to the middleware list
func WithMiddlewares(middlewares ...Middleware) serverOption {
	return func(cfg *serverConfig) {
		cfg.middlewares = append(cfg.middlewares, middlewares...)
	}
}

// WithControllers adds Controllers whose Routes(Router) methods
// register grouped routes + scoped middleware on the server's root
// router.
func WithControllers(controllers ...Controller) serverOption {
	return func(cfg *serverConfig) {
		cfg.controllers = append(cfg.controllers, controllers...)
	}
}

// WithEndpoints adds the provided endpoints to the endpoint list
func WithEndpoints(endpoints ...Endpoint) serverOption {
	return func(cfg *serverConfig) {
		cfg.endpoints = append(cfg.endpoints, endpoints...)
	}
}

// WithOpenAPI activates OpenAPI 3.1 spec generation for this server.
// Every route registered through the kit's Router will be reflected
// into the spec, and the spec + Swagger UI will be served at the
// configured paths (defaults: /openapi.json and /docs).
//
// Compose with the openapi.Spec* helpers:
//
//	rest.WithOpenAPI(
//	    openapi.SpecTitle("auth"),
//	    openapi.SpecVersion("0.1.0"),
//	    openapi.SpecSecurityScheme("bearerAuth", openapi.BearerJWT()),
//	    openapi.DefaultSecurity("bearerAuth"),
//	)
//
// Calling WithOpenAPI multiple times is additive — each call appends
// further opts to the same SpecConfig.
func WithOpenAPI(opts ...openapi.SpecOpt) serverOption {
	return func(cfg *serverConfig) {
		if cfg.openAPI == nil {
			cfg.openAPI = &openapi.SpecConfig{}
		}
		for _, opt := range opts {
			opt(cfg.openAPI)
		}
	}
}
