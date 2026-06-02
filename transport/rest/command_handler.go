package rest

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
)

// NewJsonApiCommandHandler wires a write endpoint whose usecase
// method takes a typed Command struct rather than the resource
// directly. Fits the DDD pattern where the function signature carries
// the intent of the operation alongside its inputs:
//
//	func (uc *workspaceUsecase) Invite(ctx context.Context, cmd InviteMemberCommand) (Member, error)
//	func (uc *orderUsecase) Cancel(ctx context.Context, cmd CancelOrderCommand) (Order, error)
//
// The decoder receives the *http.Request so it can read URL path
// params (req.PathValue("id")), query params, and body. Identity
// (actor, tenancy) is NEVER extracted by the decoder — middleware
// puts those on ctx and the usecase reads them when applying
// business rules.
//
// The default success status is 201 Created. Override with
// HandlerWithCreatedStatus(http.StatusOK) for a Command that doesn't
// produce a new resource (e.g. POST /orders/{id}/cancel returns the
// updated order, not a new one).
func NewJsonApiCommandHandler[Cmd any, R resource.Resource, DTO resource.Resource](
	cmdFn func(ctx context.Context, cmd Cmd) (R, error),
	decoder func(*http.Request) (Cmd, error),
	encoder func(R) DTO,
	opts ...HandlerOpt,
) http.Handler {
	cfg := new(handlerConfig)
	for _, opt := range opts {
		opt(cfg)
	}
	status := cfg.successStatus
	if status == 0 {
		status = http.StatusCreated
	}
	return WithJSONAPIIncludes(
		NewHandler(
			cmdFn,
			NewHTTPDecoder(adaptCommandDecoder(decoder)),
			jsonApiEncoder(encoder, status),
			append(opts, HandlerWithErrorEncoder(JsonApiErrorEncoder))...,
		),
	)
}

// adaptCommandDecoder lifts a `func(*http.Request) (Cmd, error)` into
// the kit decoder shape `func(ctx, *http.Request) (Cmd, error)`. The
// caller doesn't need ctx today, but exposing both shapes upstream
// would be over-engineering for the rare case it might.
func adaptCommandDecoder[Cmd any](
	decoder func(*http.Request) (Cmd, error),
) func(context.Context, *http.Request) (Cmd, error) {
	return func(_ context.Context, req *http.Request) (Cmd, error) {
		return decoder(req)
	}
}

// UnmarshalPayloadFromRequest decodes a JSON:API document from the
// request body into DTO. Convenient one-liner for Command-handler
// decoders so they don't need to plumb `bytes.NewReader(io.ReadAll(req.Body))`
// or remember the kit error shape on a malformed body. Returns the
// decoded primary-data DTO; an InvalidArgument kit error on parse
// failure.
//
//	func decodeInviteMember(req *http.Request) (app.InviteMemberCommand, error) {
//	    body, err := kitrest.UnmarshalPayloadFromRequest[api.InviteMemberRequest](req)
//	    if err != nil {
//	        return app.InviteMemberCommand{}, err
//	    }
//	    return app.InviteMemberCommand{
//	        WorkspaceID: req.PathValue("id"),
//	        AccountID:   body.AccountID(),
//	        Role:        body.Role(),
//	    }, nil
//	}
func UnmarshalPayloadFromRequest[DTO any](req *http.Request) (DTO, error) {
	res, err := jsonapi.UnmarshalPayload[DTO](req.Body)
	if err != nil {
		var zero DTO
		return zero, errors.InvalidArgument("invalid request body")
	}
	return res.Data, nil
}
