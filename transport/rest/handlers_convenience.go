package rest

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/transport"
)

// Plain-JSON sibling of jsonapi_handler.go. Same handler factories,
// same usecase-typed inputs, but the response goes out as
// application/json instead of application/vnd.api+json. Use these
// when you genuinely need to expose a non-JSON:API endpoint (webhook
// receivers, admin tools); default to the jsonapi variants for any
// resource-shaped surface.

func NewResourceHandler[R resource.Resource, O any](
	e transport.Endpoint[R, R],
	decoder func(O) R, encoder func(res R) O,
	successCode int, opts ...HandlerOpt,
) http.Handler {
	return NewHandler(
		e,
		NewHTTPDecoder(decodeResourceReq(decoder)),
		RestJSONEncoder(encoder, successCode),
		opts...,
	)
}

func NewCreateHandler[R resource.Resource, C usecase.Creator[R], O any](
	creator C,
	decoder func(O) R, encoder func(res R) O,
	opts ...HandlerOpt,
) http.Handler {
	return NewHandler(
		creator.Create,
		NewHTTPDecoder(decodeResourceReq(decoder)),
		RestJSONEncoder(encoder, http.StatusCreated),
		opts...,
	)
}

func NewListHandler[R resource.Resource, C usecase.Lister[R], O any](
	lister C, resItemMapper func(res R) O, opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, qOpts []query.Option) (resource.ListResponse[R], error) {
		return lister.List(ctx, search.WithQueryOpts(qOpts...))
	}
	return NewHandler(
		endpoint,
		NewHTTPDecoder(QueryOptsFromReq()),
		RestJSONEncoder(
			resource.ListResponseToDTO(resItemMapper),
			http.StatusOK,
		),
		opts...,
	)
}

func NewGetHandler[R resource.Resource, C usecase.Getter[R], O any](
	getter C, encoder func(res R) O,
	parseOpts []query.ParseOpt,
	opts ...HandlerOpt,
) http.Handler {
	cfg := new(handlerConfig)
	for _, opt := range opts {
		opt(cfg)
	}

	parseOpts = append(parseOpts, query.SkipDefaultPagination())
	endpoint := func(ctx context.Context, qOpts []query.Option) (R, error) {
		return getter.Get(ctx, search.WithQueryOpts(qOpts...))
	}
	return NewHandler(
		endpoint,
		NewHTTPDecoder(DecodeGetReq(parseOpts, cfg.getDecoderOpts...)),
		RestJSONEncoder(encoder, http.StatusOK),
		opts...,
	)
}

func NewUpdateHandler[R resource.Resource, C usecase.Updater[R], O any](
	updater C,
	decoder func(O) R, encoder func(res R) O,
	opts ...HandlerOpt,
) http.Handler {
	return NewHandler(
		updater.Update,
		NewHTTPDecoder(decodeResourceReq(decoder)),
		RestJSONEncoder(encoder, http.StatusOK),
		opts...,
	)
}

func NewPatchHandler[T, R resource.Resource, C usecase.Patcher[R], O any](
	patcher C, kind resource.Type,
	decoder func(T) []repository.PatchOption, encoder func(res R) O,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, optList []repository.PatchOption) (R, error) {
		return patcher.Patch(ctx, optList...)
	}
	return NewHandler(
		endpoint,
		NewHTTPDecoder(decodePatchReq(kind, decoder)),
		RestJSONEncoder(encoder, http.StatusOK),
		opts...,
	)
}

func NewDeleteHandler(
	deleter usecase.Deleter, deleteType repository.DeleteType,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, qOpts []query.Option) (struct{}, error) {
		return struct{}{}, deleter.Delete(ctx, deleteType, search.WithQueryOpts(qOpts...))
	}
	return NewHandler(
		endpoint,
		NewHTTPDecoder(DecodeGetReq([]query.ParseOpt{})),
		NewEmptyHTTPEncoder(http.StatusNoContent),
		opts...,
	)
}
