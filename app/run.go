package app

import (
	"context"
	"errors"
	"os"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
)

// Run boots a forge service with the kit's standard defaults and the
// caller-supplied modules, then blocks until SIGINT / SIGTERM.
//
// Defaults are intentionally small — DB / auth / NATS / gRPC are all
// opt-in via the kit's existing FxModule() constructors passed in
// alongside internal.FxModule().
func Run(opts ...Option) {
	cfg := resolve(opts)
	if handled, code := runCommandIfMatched(os.Args, cfg); handled {
		os.Exit(code)
	}
	app := buildAppFromConfig(cfg)
	app.Run()
}

// RunWorker is Run with HTTP disabled. Use it for background workers
// — services that consume queues or run cron jobs but don't serve
// requests. Equivalent to app.Run(append(opts, app.WithoutHTTP())...)
// but the dedicated entry point makes intent obvious at the call site.
func RunWorker(opts ...Option) {
	Run(append(opts, WithoutHTTP())...)
}

// RunContextWorker is RunWorker with a caller-supplied context — the
// test/lifecycle-driven counterpart to RunContext.
func RunContextWorker(ctx context.Context, opts ...Option) error {
	return RunContext(ctx, append(opts, WithoutHTTP())...)
}

// RunWithConfig is the common-case shorthand for services that load
// a typed config struct from env vars at startup. The pointer's
// contents are populated via envconfig.Process before fx wiring
// begins, then the same pointer is supplied to the fx graph so
// handlers can inject *T.
func RunWithConfig[T any](cfg *T, opts ...Option) {
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}
	opts = append(opts, fx.Supply(cfg))
	Run(opts...)
}

// RunContext is Run with a caller-supplied context. When the context
// cancels, the fx app shuts down — useful for tests that need to
// drive the lifecycle without sending real OS signals.
func RunContext(ctx context.Context, opts ...Option) error {
	app := buildApp(opts)

	// Start with its own timeout — not derived from ctx — so that a
	// caller cancelling ctx mid-Start means "shut down once started"
	// rather than "abort Start".
	startCtx, cancelStart := context.WithTimeout(context.Background(), app.StartTimeout())
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		// Caller cancelled — initiate graceful shutdown.
	case sig := <-app.Wait():
		// Real signal arrived; only treat non-zero exits as errors.
		if sig.ExitCode != 0 {
			stopCtx, cancelStop := context.WithTimeout(context.Background(), app.StopTimeout())
			defer cancelStop()
			_ = app.Stop(stopCtx)
			return errors.New("app exited with non-zero code")
		}
	}

	stopCtx, cancelStop := context.WithTimeout(context.Background(), app.StopTimeout())
	defer cancelStop()
	return app.Stop(stopCtx)
}

// RunContextWithConfig is RunContext + envconfig loading.
func RunContextWithConfig[T any](ctx context.Context, cfg *T, opts ...Option) error {
	if err := envconfig.Process("", cfg); err != nil {
		return err
	}
	opts = append(opts, fx.Supply(cfg))
	return RunContext(ctx, opts...)
}

// buildApp composes the default modules + the caller's options into
// an *fx.App ready to Run() or Start()/Stop().
func buildApp(opts []Option) *fx.App {
	cfg := resolve(opts)
	return buildAppFromConfig(cfg)
}

// buildAppFromConfig is buildApp's body — kept separate so the
// command dispatcher (cmd.go) can reuse it after mutating the
// resolved config (e.g. forcing withoutHTTP and appending the
// command's fx.Invoke).
func buildAppFromConfig(cfg *config) *fx.App {
	mods := make([]fx.Option, 0, len(cfg.userOptions)+4)

	mods = append(mods, fx.Supply(Info{Name: cfg.name, Version: cfg.version}))

	if !cfg.withoutTele {
		mods = append(mods, monitoring.FxModule())
	}

	if !cfg.withoutHTTP {
		// rest.FxModule's serverOption type is unexported, so the
		// kit's compose-the-options pattern means we branch inline
		// rather than build a typed slice. The single OpenAPI opt
		// is the only one we forward today; richer composition lands
		// as new app.WithRest* options when needed.
		if len(cfg.openAPI) > 0 {
			mods = append(mods, kitrest.FxModule(kitrest.WithOpenAPI(cfg.openAPI...)))
		} else {
			mods = append(mods, kitrest.FxModule())
		}
	}

	mods = append(mods, cfg.userOptions...)

	return fx.New(mods...)
}
