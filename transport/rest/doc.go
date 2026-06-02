// Package rest provides the kit's HTTP/REST server, client, and
// route-registration primitives. It is opinionated about how
// services describe their HTTP surface and how that surface maps to
// the application layer:
//
//   - Routes are declared by Controllers via Routes(Router) — a
//     grouped-routing API on top of stdlib net/http.ServeMux that
//     supports prefix groups (Router.Route), scoped middleware
//     (Router.Use and Router.With), and per-method handler
//     registration.
//   - Handler factories take usecase-shaped inputs directly:
//     usecase.Creator[R], usecase.Lister[R], usecase.Getter[R], and
//     so on. There is no intermediate "controller" adapter; the kit
//     translates HTTP query opts into search.Option internally.
//   - The JSON:API wire format is the default. Plain-JSON variants
//     live in handlers_convenience.go for the rare endpoint that
//     genuinely needs one.
//
// # Route registration
//
// A Controller implements one method:
//
//	func (c *workspaceController) Routes(r rest.Router) {
//	    r.Route("/v1/workspaces", func(r rest.Router) {
//	        r.Use(c.auth, c.actor)
//	        r.Post("/", rest.NewJsonApiCreateHandler(c.uc, api.WorkspaceFromDTO, api.WorkspaceToDTO))
//	        r.Route("/{id}", func(r rest.Router) {
//	            r.Use(c.tenancy)
//	            r.Get("/", rest.NewJsonApiGetHandler(c.uc, api.WorkspaceToDTO, nil))
//	            r.Delete("/", rest.NewJsonApiDeleteHandler(c.uc, repository.DeleteTypeSoft))
//	            r.Post("/members", rest.NewJsonApiCommandHandler(c.uc.Invite, decodeInvite, api.MemberToDTO))
//	        })
//	    })
//	}
//
// Middleware ordering is first-in-list-is-outermost — r.Use(a, b, c)
// produces a(b(c(handler))). The same applies to global middleware
// registered via WithMiddlewares.
//
// # Handler factories
//
// Resource CRUD (body equals the resource):
//
//   - NewJsonApiCreateHandler  — POST /resources
//   - NewJsonApiGetHandler     — GET /resources/{id} (or singleton via HandlerWithGetDecoderOpts(DecodeGetSkipURLPathID()))
//   - NewJsonApiListHandler    — GET /resources
//   - NewJsonApiUpdateHandler  — PUT /resources/{id}
//   - NewJsonApiPatchHandler   — PATCH /resources/{id}
//   - NewJsonApiDeleteHandler  — DELETE /resources/{id}
//
// DDD command shape (usecase takes a typed Command, input split
// across body + path):
//
//   - NewJsonApiCommandHandler — POST/PATCH /resources/{id}/action
//
// Bulk:
//
//   - NewJsonApiBulkCreateHandler       — primary-data array
//   - NewJsonApiBulkUpdateHandler       — filter + patch envelope
//   - NewJsonApiBulkDeleteHandler       — filter envelope
//   - NewJsonApiAtomicOperationsHandler — mixed add/update/remove in one transaction
//
// Streaming:
//
//   - NewMultipartUploadHandler   — multipart/form-data with one file part
//   - NewStreamingDownloadHandler — raw bytes with Range support
//   - NewServerSentEventsHandler  — text/event-stream
//
// Escape hatch for the rare handler that doesn't fit any shape:
//
//   - WriteJSONAPI       — single-resource document with buffered marshal
//   - WriteJSONAPIError  — jsonapi error document via the kit encoder
//
// # Server bootstrap
//
//	server := rest.NewServer(
//	    rest.WithAddress(":8080"),
//	    rest.WithMiddlewares(
//	        rest.RequestIDMiddleware(),
//	        rest.RecoveryMiddleware(log),
//	        rest.AccessLogMiddleware(log),
//	    ),
//	    rest.WithControllers(workspaceCtrl, memberCtrl),
//	)
//	_ = server.ListenAndServe()
//
// With fx, the wiring is:
//
//	fx.New(
//	    rest.FxModule(),
//	    rest.FxAuthenticator(),
//	    rest.NewFxController(NewWorkspaceController),
//	    rest.NewFxMiddleware(NewActorMiddleware),
//	    // ...
//	).Run()
//
// # Authentication
//
// Auth is a regular Middleware. Wire it via rest.NewAuthMiddleware:
//
//	r.Use(rest.NewAuthMiddleware(authenticator))
//
// Or scoped to a single registration:
//
//	r.With(rest.NewAuthMiddleware(authenticator)).Post("/", handler)
//
// # Client
//
// rest.NewClient and rest.NewDefaultHTTPClient cover the
// service-to-service call side. NewTracingTransport wraps an
// http.RoundTripper with OpenTelemetry trace propagation.
package rest
