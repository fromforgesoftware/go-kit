package app

import (
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/openapi"
)

// Option is the unit of app configuration. Every Option is just an
// fx.Option so any third-party fx module composes seamlessly:
//
//	app.Run(
//	    app.WithName("auth"),
//	    gormpg.FxModule(),            // any fx.Option works as an Option
//	    internal.FxModule(),
//	)
type Option = fx.Option

// appOption is the internal marker for forge-owned options that
// configure app behaviour (name / version / openapi / runtime
// toggles) before any user modules run. Distinct from raw fx.Option
// so the Run resolver can separate "app config" from "modules to
// install".
type appOption interface {
	fx.Option
	applyApp(*config)
}

type config struct {
	name        string
	version     string
	openAPI     []openapi.SpecOpt
	withoutHTTP bool
	withoutTele bool
	commands    []CommandDef
	userOptions []fx.Option
}

// raw fx.Options end up in userOptions; appOptions get applied to the
// config struct and then dropped from the install set.
func resolve(opts []Option) *config {
	c := &config{}
	for _, o := range opts {
		if ao, ok := o.(appOption); ok {
			ao.applyApp(c)
			continue
		}
		c.userOptions = append(c.userOptions, o)
	}
	return c
}

// markerOption wraps a no-op fx.Option (fx.Options() with no inner
// args) together with the apply hook. Implements both fx.Option and
// appOption.
type markerOption struct {
	fx.Option
	apply func(*config)
}

func (m markerOption) applyApp(c *config) { m.apply(c) }

func newAppOption(apply func(*config)) appOption {
	return markerOption{Option: fx.Options(), apply: apply}
}

// WithName sets the service name. Surfaces in the logger ("svc"
// field), tracer (service.name attribute), and the app.Info value
// provided to the fx graph.
func WithName(name string) Option {
	return newAppOption(func(c *config) { c.name = name })
}

// WithVersion sets the service version. Surfaces alongside the name
// in observability and OpenAPI info.
func WithVersion(version string) Option {
	return newAppOption(func(c *config) { c.version = version })
}

// WithOpenAPI activates OpenAPI 3.1 spec generation by threading the
// given openapi.SpecOpt values through rest.WithOpenAPI.
//
// Calling multiple times is additive; the SpecConfig builds up across
// calls.
func WithOpenAPI(opts ...openapi.SpecOpt) Option {
	return newAppOption(func(c *config) {
		c.openAPI = append(c.openAPI, opts...)
	})
}

// WithModules is a convenience for grouping several fx.Options
// inline; rarely needed since each fx.Option can be passed directly
// to Run.
func WithModules(mods ...fx.Option) Option {
	return fx.Options(mods...)
}

// WithoutHTTP skips the default rest.FxModule. Used by background
// workers and other non-HTTP services that don't need a server.
func WithoutHTTP() Option {
	return newAppOption(func(c *config) { c.withoutHTTP = true })
}

// WithoutTelemetry skips the default monitoring.FxModule. Rare; used
// only by trivial scripts where the logger/tracer overhead isn't
// wanted.
func WithoutTelemetry() Option {
	return newAppOption(func(c *config) { c.withoutTele = true })
}
