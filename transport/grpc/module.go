package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/monitoring"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FxConfig holds optional configuration for the gRPC module
type FxConfig struct {
	Controllers []Controller `ignored:"true"`
}

// ============================================================================
// Server Module
// ============================================================================

// FxModule creates an Fx module for a gRPC server with automatic lifecycle management.
// It wires up controllers and middlewares from the dependency graph and starts/stops
// the server automatically using Fx hooks.
//
// Example usage:
//
//	fx.New(
//	    grpc.FxModule(),
//	    grpc.NewFxController(myController),
//	    grpc.NewFxMiddleware(myMiddleware),
//	)
func FxModule(opts ...serverOption) fx.Option {
	return fx.Module("grpc-gateway",
		// Collect all controllers from dependency graph
		fx.Provide(
			fx.Annotate(
				WithControllers,
				fx.ParamTags(`group:"grpcControllers"`),
				fx.ResultTags(`group:"grpcGatewayOptions"`),
			),
		),
		// Collect all middlewares from dependency graph
		fx.Provide(
			fx.Annotate(
				WithMiddlewares,
				fx.ParamTags(`group:"grpcMiddlewares"`),
				fx.ResultTags(`group:"grpcGatewayOptions"`),
			),
		),
		// Provide user-supplied options
		fx.Supply(
			fx.Annotate(
				opts,
				fx.ResultTags(`group:"grpcGatewayOptions,flatten"`),
			),
		),
		// Create and lifecycle-manage the server
		fx.Provide(
			fx.Annotate(
				startServer,
				fx.ParamTags(``, `optional:"true"`, ``, ``, `group:"grpcGatewayOptions"`),
			),
		),
		// Force server creation
		fx.Invoke(func(*Server) {}),
	)
}

// startServer creates and starts the gRPC server with lifecycle hooks
func startServer(
	lc fx.Lifecycle,
	cfg *FxConfig,
	monitor monitoring.Monitor,
	shutdowner fx.Shutdowner,
	opts []serverOption,
) (*Server, error) {
	// Merge config controllers with options
	if cfg != nil && len(cfg.Controllers) > 0 {
		opts = append(opts, WithControllers(cfg.Controllers...))
	}

	// Create server
	server, err := NewServer(monitor, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Channel that closes when Serve returns — used to coordinate Graceful
	// vs hard Stop on shutdown so we don't block fx forever on a misbehaved
	// long-running RPC.
	serveDone := make(chan struct{})

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return startServerAsync(server, monitor, shutdowner, serveDone)
		},
		OnStop: func(ctx context.Context) error {
			// GracefulStop drains in-flight RPCs but blocks until they
			// finish. If that takes longer than the fx stop ctx allows,
			// fall back to hard Stop so the process can actually exit.
			graceful := make(chan struct{})
			go func() {
				server.GracefulStop()
				close(graceful)
			}()
			select {
			case <-graceful:
			case <-ctx.Done():
				monitor.Logger().Warn("gRPC GracefulStop exceeded ctx deadline; forcing Stop")
				server.Stop()
				<-graceful
			}
			<-serveDone
			return nil
		},
	})

	return server, nil
}

// startServerAsync starts the server in a goroutine and checks for early
// startup failures. The 100ms probe catches synchronous bind / TLS errors
// without the leaked goroutine that the previous implementation had:
// errCh is consumed by a follower goroutine that surfaces late errors via
// fx.Shutdowner instead of blocking on an unread channel forever.
func startServerAsync(server *Server, monitor monitoring.Monitor, shutdowner fx.Shutdowner, done chan struct{}) error {
	errCh := make(chan error, 1)

	go func() {
		defer close(done)
		if err := server.Start(); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// Give the server a moment to fail synchronously (bind errors, TLS
	// handshake config errors). After this the goroutine below takes over
	// surfacing later errors.
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("gRPC server failed to start: %w", err)
		}
		// Serve returned immediately with nil — rare but valid (graceful
		// stop during startup probe).
		return nil
	case <-time.After(100 * time.Millisecond):
		// Server is up. Spawn a watcher that surfaces late Serve errors
		// via fx instead of leaking on an unread channel.
		go func() {
			if err := <-errCh; err != nil {
				monitor.Logger().Error("gRPC Serve returned error after startup", "error", err)
				_ = shutdowner.Shutdown(fx.ExitCode(1))
			}
		}()
		return nil
	}
}

// NewFxController registers a controller in the Fx dependency graph.
// The controller will be automatically picked up by FxModule.
//
// Example:
//
//	grpc.NewFxController(func() grpc.Controller {
//	    return myController
//	})
func NewFxController(ctrl any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			ctrl,
			fx.ResultTags(`group:"grpcControllers"`),
			fx.As(new(Controller)),
		),
	)
}

// NewFxMiddleware registers a middleware in the Fx dependency graph.
// The middleware will be automatically picked up by FxModule.
//
// Example:
//
//	grpc.NewFxMiddleware(func(logger logger.Logger) grpc.Middleware {
//	    return middleware.Logging(logger)
//	})
func NewFxMiddleware(middleware any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			middleware,
			fx.ResultTags(`group:"grpcMiddlewares"`),
			fx.As(new(Middleware)),
		),
	)
}

// FxAuthenticator provides a gRPC authenticator from the auth package.
// This is a convenience function for common authentication setups.
func FxAuthenticator() fx.Option {
	return fx.Provide(
		fx.Annotate(
			auth.NewGrpcAuthenticator,
			fx.As(new(GRPCAuthenticator)),
		),
	)
}

// ============================================================================
// Client Module
// ============================================================================

// clientConfig holds configuration for creating a gRPC client module
type clientConfig struct {
	url                    string
	grpcOpts               []grpc.DialOption
	extraProviders         []any
	clientName             string
	constructorAnnotations []fx.Annotation
}

// ClientOption configures a gRPC client module
type clientOption func(*clientConfig)

// WithClientDialOptions adds gRPC dial options to the client
func WithClientDialOptions(opts ...grpc.DialOption) clientOption {
	return func(c *clientConfig) {
		c.grpcOpts = append(c.grpcOpts, opts...)
	}
}

// WithClientProviders registers additional client constructors that
// share the SAME underlying *grpc.ClientConn dialled by the module.
// Use this when one gRPC server hosts multiple service interfaces a
// service consumes (e.g. auth's AuthorizerService + ResourceRegistry +
// ACLService all served from the same auth process):
//
//	grpc.FxClientModule("auth", url,
//	    func(c *grpc.ClientConn) AuthorizerServiceClient { return NewAuthorizerServiceClient(c) },
//	    grpc.WithClientProviders(
//	        func(c *grpc.ClientConn) ResourceRegistryServiceClient { return NewResourceRegistryServiceClient(c) },
//	        func(c *grpc.ClientConn) ACLServiceClient { return NewACLServiceClient(c) },
//	    ),
//	)
//
// Each extra constructor is auto-annotated to receive the named
// connection — callers don't repeat the fx tag plumbing.
func WithClientProviders(providers ...any) clientOption {
	return func(c *clientConfig) {
		c.extraProviders = append(c.extraProviders, providers...)
	}
}

// WithClientConstructorAnnotations adds Fx annotations to the client constructor
func WithClientConstructorAnnotations(annotations ...fx.Annotation) clientOption {
	return func(c *clientConfig) {
		c.constructorAnnotations = append(c.constructorAnnotations, annotations...)
	}
}

// FxClientModule creates an Fx module for a gRPC client connection.
// It provides a named client connection that can be injected into other components.
//
// Parameters:
//   - name: Module name and client identifier
//   - url: gRPC server URL (e.g., "localhost:50051")
//   - constructor: Function to create the client from a connection
//   - opts: Additional client options
//
// Example:
//
//	grpc.FxClientModule(
//	    "auth",
//	    "localhost:50051",
//	    func(conn *grpc.ClientConn) AuthServiceClient {
//	        return NewAuthServiceClient(conn)
//	    },
//	)
func FxClientModule(name, url string, constructor any, opts ...clientOption) fx.Option {
	// Generate consistent naming
	clientName := fmt.Sprintf("%sGRPCClient", name)
	urlName := fmt.Sprintf("%sGRPCUrl", name)
	optsName := fmt.Sprintf("%sGRPCOpts", name)

	// Build config
	cfg := &clientConfig{
		url:        url,
		clientName: clientName,
		grpcOpts:   []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return fx.Module(name,
		// Supply URL
		fx.Supply(
			fx.Annotate(url, fx.ResultTags(fmt.Sprintf(`name:%q`, urlName))),
		),
		// Supply dial options
		fx.Supply(
			fx.Annotate(
				cfg.grpcOpts,
				fx.ResultTags(fmt.Sprintf(`group:"%s,flatten"`, optsName)),
			),
		),
		// Provide connection
		fx.Provide(
			fx.Annotate(
				dial,
				fx.ParamTags(
					fmt.Sprintf(`name:%q`, urlName),
					fmt.Sprintf(`group:%q`, optsName),
				),
				fx.ResultTags(fmt.Sprintf(`name:%q`, clientName)),
			),
		),
		// Provide client + any extras (all share the named conn).
		fx.Provide(buildClientProviders(constructor, cfg, clientName)...),
	)
}

// buildClientProviders wraps the primary constructor + every extra
// provider with the ParamTags that point at the module's named conn,
// so callers passing plain func(*grpc.ClientConn) X get the right
// connection wired without spelling out the fx tag each time.
func buildClientProviders(primary any, cfg *clientConfig, clientName string) []any {
	annotate := func(ctor any) any {
		return fx.Annotate(
			ctor,
			append(cfg.constructorAnnotations,
				fx.ParamTags(fmt.Sprintf(`name:%q`, clientName)),
			)...,
		)
	}
	out := make([]any, 0, 1+len(cfg.extraProviders))
	out = append(out, annotate(primary))
	for _, p := range cfg.extraProviders {
		out = append(out, annotate(p))
	}
	return out
}
