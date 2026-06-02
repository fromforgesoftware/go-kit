// Package app is forge's framework-style service bootstrap. A single
// app.Run(opts...) call composes the kit's standard fx modules with
// the caller's modules and blocks until SIGINT / SIGTERM.
//
// Minimal HTTP service:
//
//	func main() {
//	    app.Run(
//	        app.WithName("auth"),
//	        app.WithVersion(buildinfo.Version),
//	        gormpg.FxModule(),
//	        internal.FxModule(),
//	    )
//	}
//
// Defaults are intentionally small — only monitoring (logger + tracer)
// and HTTP (rest.FxModule) ship in. DB, auth, NATS, gRPC, outbox are
// all opt-in via the kit's existing FxModule() constructors.
//
// Option is an alias for fx.Option, so any third-party fx module
// passes through unchanged.
package app
