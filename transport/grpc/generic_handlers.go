package grpc

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
)

// NewCreateHandler wires a gRPC RPC that consumes a Creator[R]. The
// decoder builds R from the proto request; the encoder maps the
// inserted entity back to the proto response.
//
// Symmetric to rest.NewJsonApiCreateHandler. The variadic adapter
// isn't needed for Create (Creator.Create takes R directly), but the
// helper exists so callers don't have to manually wire
// `NewHandler(creator.Create, …)` and pick the right type params.
func NewCreateHandler[R resource.Resource, C repository.Creator[R], DI, EO any](
	creator C,
	decoder DecodeRequestFunc[DI, R],
	encoder EncodeResponseFunc[R, EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(creator.Create, decoder, encoder, monitor, opts...)
}

// NewUpdateHandler wires a gRPC RPC that consumes an Updater[R]. Same
// shape as NewCreateHandler but binds to Update.
//
// Symmetric to rest.NewJsonApiUpdateHandler.
func NewUpdateHandler[R resource.Resource, C repository.Updater[R], DI, EO any](
	updater C,
	decoder DecodeRequestFunc[DI, R],
	encoder EncodeResponseFunc[R, EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(updater.Update, decoder, encoder, monitor, opts...)
}

// NewPatchHandler wires a gRPC RPC that consumes a Patcher[R]. The
// decoder produces []repository.PatchOption from the proto request;
// the encoder maps the patched entities back to the proto response.
//
// Symmetric to rest.NewJsonApiPatchHandler.
func NewPatchHandler[R resource.Resource, C repository.Patcher[R], DI, EO any](
	patcher C,
	decoder DecodeRequestFunc[DI, []repository.PatchOption],
	encoder EncodeResponseFunc[[]R, EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(
		func(ctx context.Context, optList []repository.PatchOption) ([]R, error) {
			return patcher.Patch(ctx, optList...)
		},
		decoder, encoder, monitor, opts...,
	)
}

// NewCommandHandler wires a gRPC RPC whose usecase method takes a
// typed Command struct rather than a resource. Fits the DDD shape
// where the function signature carries the intent of the operation:
//
//	func (uc *workspaceUsecase) Invite(ctx context.Context, cmd InviteMemberCommand) (Member, error)
//	func (uc *orderUsecase) Cancel(ctx context.Context, cmd CancelOrderCommand) (Order, error)
//
// Identity (actor, tenancy) is read from ctx by the usecase — the
// decoder only produces the Command payload from the proto request.
//
// Symmetric to rest.NewJsonApiCommandHandler.
func NewCommandHandler[Cmd any, R any, DI, EO any](
	cmdFn func(ctx context.Context, cmd Cmd) (R, error),
	decoder DecodeRequestFunc[DI, Cmd],
	encoder EncodeResponseFunc[R, EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(cmdFn, decoder, encoder, monitor, opts...)
}

// NewGetHandler wires a gRPC RPC that consumes a Getter[R]. The
// decoder produces []search.Option from the proto request (typically
// one filter on the id field); the encoder maps the domain entity to
// the proto response.
//
// Symmetric to rest.NewJsonApiGetHandler: the variadic-to-slice
// adapter for the kit usecase shape lives here so every RPC isn't
// re-writing the same closure.
func NewGetHandler[R resource.Resource, C repository.Getter[R], DI, EO any](
	getter C,
	decoder DecodeRequestFunc[DI, []search.Option],
	encoder EncodeResponseFunc[R, EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(
		func(ctx context.Context, opts []search.Option) (R, error) {
			return getter.Get(ctx, opts...)
		},
		decoder, encoder, monitor, opts...,
	)
}

// NewListHandler wires a gRPC RPC that consumes a Lister[R]. The
// decoder produces []search.Option (commonly via
// QueryOptsFromProto on a tb.search.QueryOptions field); the encoder
// maps the kit ListResponse to the proto list message.
//
// Symmetric to rest.NewJsonApiListHandler.
func NewListHandler[R resource.Resource, C repository.Lister[R], DI, EO any](
	lister C,
	decoder DecodeRequestFunc[DI, []search.Option],
	encoder EncodeResponseFunc[resource.ListResponse[R], EO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewHandler(
		func(ctx context.Context, opts []search.Option) (resource.ListResponse[R], error) {
			return lister.List(ctx, opts...)
		},
		decoder, encoder, monitor, opts...,
	)
}

// NewDeleteHandler wires a gRPC RPC that consumes a Deleter. delType
// is fixed at wire time (typically DeleteTypeSoft); the decoder
// produces the id filter as []search.Option. Returns an empty response
// — callers usually wrap this with a *emptypb.Empty in Serve.
//
// Symmetric to rest.NewJsonApiDeleteHandler.
func NewDeleteHandler[C repository.Deleter, DI any](
	deleter C,
	delType repository.DeleteType,
	decoder DecodeRequestFunc[DI, []search.Option],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	return NewEmptyResHandler(
		func(ctx context.Context, opts []search.Option) error {
			return deleter.Delete(ctx, delType, opts...)
		},
		decoder, monitor, opts...,
	)
}
