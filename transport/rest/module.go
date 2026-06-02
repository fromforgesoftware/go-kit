package rest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"
	"golang.org/x/net/netutil"

	"github.com/fromforgesoftware/go-kit/auth"
)

type FxConfig struct {
	HTTPAddress string `required:"true" envconfig:"HTTP_ADDRESS"`
}

const defaultShutdownTimeout = 5 * time.Second

func FxModule(opts ...serverOption) fx.Option {
	return fx.Module("http-gateway",
		fx.Provide(NewReadiness),
		fx.Provide(
			fx.Annotate(
				WithControllers,
				fx.ParamTags(`group:"restControllers"`),
				fx.ResultTags(`group:"restGatewayOptions"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				WithMiddlewares,
				fx.ParamTags(`group:"restMiddlewares"`),
				fx.ResultTags(`group:"restGatewayOptions"`),
			),
		),
		fx.Supply(
			fx.Annotate(
				opts,
				fx.ResultTags(`group:"restGatewayOptions,flatten"`),
			),
		),
		fx.Invoke(
			fx.Annotate(func(lc fx.Lifecycle, shutdowner fx.Shutdowner, readiness *Readiness, opts []serverOption) *http.Server {
				cfg := &serverConfig{
					shutdownTimeout: defaultShutdownTimeout,
				}

				// Parse twice: once locally to read maxConnections /
				// shutdownTimeout, once again inside NewServer to build the
				// *http.Server. Options are pure functions so this is safe.
				for _, opt := range append(defaultServerOpts(), opts...) {
					opt(cfg)
				}

				// Auto-register /readyz so callers get a separate
				// liveness (/healthz) vs readiness (/readyz) endpoint
				// without having to wire it themselves.
				opts = append(opts, WithEndpoints(ReadinessEndpoint(readiness)))

				g := NewServer(opts...)

				lc.Append(fx.Hook{
					OnStart: func(context.Context) error {
						ln, err := net.Listen("tcp", g.Addr)
						if err != nil {
							return err
						}
						if cfg.maxConnections > 0 {
							ln = netutil.LimitListener(ln, cfg.maxConnections)
						}

						go func() {
							err := g.Serve(ln)
							if err != nil && !errors.Is(err, http.ErrServerClosed) {
								// Surface fatal Serve errors through fx instead of
								// panicking from a detached goroutine — fx will run
								// OnStop hooks and exit cleanly.
								_ = shutdowner.Shutdown(fx.ExitCode(1))
							}
						}()

						return nil
					},
					OnStop: func(ctx context.Context) error {
						// Flip readiness to NOT ready before draining so
						// load balancers stop sending new traffic while
						// in-flight requests finish. /healthz keeps
						// returning 200 — the pod is still alive.
						readiness.SetReady(false)

						newCtx, cancel := context.WithTimeout(ctx, cfg.shutdownTimeout)
						defer cancel()
						if err := g.Shutdown(newCtx); err != nil {
							return err
						}
						return nil
					},
				})

				return g
			}, fx.ParamTags(``, ``, ``, `group:"restGatewayOptions"`)),
		),
	)
}

// NewFxController registers a Controller constructor into the fx
// graph. The Controller is collected via the `restControllers` fx
// group and its Routes(Router) method is invoked at NewServer time
// against the server's root router.
func NewFxController(controller any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			controller,
			fx.ResultTags(`group:"restControllers"`),
			fx.As(new(Controller)),
		),
	)
}

// NewFxMiddleware is a helper function that given a middleware constructor builds a compatible and annotated fx module.
func NewFxMiddleware(middleware any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			middleware,
			fx.ResultTags(`group:"restMiddlewares"`),
			fx.As(new(Middleware)),
		),
	)
}

func FxAuthenticator() fx.Option {
	return fx.Provide(
		fx.Annotate(auth.NewHttpAuthenticator, fx.As(new(HTTPAuthenticator))),
	)
}
