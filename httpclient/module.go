package httpclient

import (
	"fmt"
	"net/http"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// FxModule registers a named *Client in the fx graph so a service
// can hold several clients (one per upstream) and inject the right
// one by name.
//
// Usage:
//
//	app.Run(
//	    app.WithName("billing"),
//	    httpclient.FxModule("stripe",
//	        httpclient.WithBaseURL("https://api.stripe.com"),
//	        httpclient.WithRetries(3),
//	        httpclient.WithBreakerPerHost(5, 30*time.Second)),
//	    httpclient.FxModule("slack",
//	        httpclient.WithBaseURL("https://slack.com/api")),
//	    internal.FxModule(),
//	)
//
//	// Inject by name:
//	type StripeDeps struct {
//	    fx.In
//	    Client *httpclient.Client `name:"stripe"`
//	}
//
// The module name doubles as the fx group key so duplicate
// registrations panic at boot.
func FxModule(name string, opts ...Option) fx.Option {
	if name == "" {
		panic("httpclient.FxModule: name is required")
	}
	tag := fmt.Sprintf(`name:"%s"`, name)
	return fx.Module("httpclient-"+name,
		fx.Provide(
			fx.Annotate(
				func(log logger.Logger, tr tracer.Tracer) *Client {
					// Auto-wire the observability transport so every
					// fx-built client gets OTel spans + log lines per
					// outbound call without the caller having to ask.
					// User-supplied WithTransport still wins because
					// New() applies options in order.
					full := append([]Option{
						WithTransport(NewObservabilityTransport(http.DefaultTransport, log, tr)),
					}, opts...)
					return New(full...)
				},
				fx.ResultTags(tag),
			),
		),
	)
}
