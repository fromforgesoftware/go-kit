package rest

import (
	"context"
	"io"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
)

// The factories in this file consume the usecase.* generic interfaces
// directly — there is no intermediate ctrl adapter. Translation
// between the HTTP layer's []query.Option (parsed by DecodeGetReq)
// and the usecase layer's variadic search.Option happens inline
// inside each handler. The fewer indirections, the better.

// NewJsonApiCreateHandler builds the POST handler for a jsonapi
// resource. When HandlerAllowsEmptyReq(true) is passed, the body
// decoder short-circuits on a zero-length body and feeds the creator
// a zero R — useful for token-driven creates (e.g. POST /v1/users
// bootstrapping from a Firebase token) where all inputs come from
// ctx and there is no meaningful body to send.
func NewJsonApiCreateHandler[R, DTO resource.Resource, C usecase.Creator[R]](
	creator C,
	decoder func(DTO) R, encoder func(res R) DTO,
	opts ...HandlerOpt,
) http.Handler {
	cfg := new(handlerConfig)
	for _, opt := range opts {
		opt(cfg)
	}

	return WithJSONAPIIncludes(
		NewHandler(
			creator.Create,
			NewHTTPDecoder(jsonApiDecodeResourceReq(decoder, cfg.allowsEmptyReq)),
			jsonApiEncoder(encoder, http.StatusCreated),
			append(opts,
				HandlerWithErrorEncoder(JsonApiErrorEncoder),
				jsonAPIDocsOverrideSingle[DTO](http.StatusCreated),
			)...,
		),
	)
}

func NewJsonApiListHandler[R, DTO resource.Resource, C usecase.Lister[R]](
	lister C, resItemMapper func(res R) DTO, opts ...HandlerOpt,
) http.Handler {
	// Adapt the usecase.Lister shape (...search.Option) to the
	// transport.Endpoint[[]query.Option, ListResponse[R]] shape kit's
	// NewHandler expects.
	endpoint := func(ctx context.Context, opts []query.Option) (resource.ListResponse[R], error) {
		return lister.List(ctx, search.WithQueryOpts(opts...))
	}
	listResponseMapper := func(res resource.ListResponse[R]) jsonapi.ListResponse[DTO] {
		return resource.ListResponseToDTO(resItemMapper)(res)
	}

	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(QueryOptsFromReq()),
			jsonApiListEncoder(listResponseMapper, http.StatusOK),
			append(opts,
				HandlerWithErrorEncoder(JsonApiErrorEncoder),
				jsonAPIDocsOverrideList[DTO](http.StatusOK),
			)...,
		),
	)
}

// NewJsonApiGetHandler serves both the canonical "/{id}" Get and the
// singleton variant (e.g. /me, /config). Pass
// HandlerWithGetDecoderOpts(DecodeGetSkipURLPathID()) to skip the
// {id} path-segment requirement when identity comes from context
// instead.
func NewJsonApiGetHandler[R, DTO resource.Resource, C usecase.Getter[R]](
	getter C, encoder func(res R) DTO,
	parseOpts []query.ParseOpt,
	opts ...HandlerOpt,
) http.Handler {
	cfg := new(handlerConfig)
	for _, opt := range opts {
		opt(cfg)
	}

	parseOpts = append(parseOpts, query.SkipDefaultPagination())
	endpoint := func(ctx context.Context, opts []query.Option) (R, error) {
		return getter.Get(ctx, search.WithQueryOpts(opts...))
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(DecodeGetReq(parseOpts, cfg.getDecoderOpts...)),
			jsonApiEncoder(encoder, http.StatusOK),
			append(opts,
				HandlerWithErrorEncoder(JsonApiErrorEncoder),
				jsonAPIDocsOverrideRespOnly[DTO](http.StatusOK),
			)...,
		),
	)
}

func NewJsonApiUpdateHandler[R, DTO resource.Resource, C usecase.Updater[R]](
	updater C,
	decoder func(DTO) R, encoder func(res R) DTO,
	opts ...HandlerOpt,
) http.Handler {
	cfg := new(handlerConfig)
	for _, opt := range opts {
		opt(cfg)
	}

	return NewHandler(
		updater.Update,
		NewHTTPDecoder(jsonApiDecodeResourceReq(decoder, cfg.allowsEmptyReq)),
		jsonApiEncoder(encoder, http.StatusOK),
		append(opts,
			HandlerWithErrorEncoder(JsonApiErrorEncoder),
			jsonAPIDocsOverrideSingle[DTO](http.StatusOK),
		)...,
	)
}

func NewJsonApiPatchHandler[T, R, DTO resource.Resource, C usecase.Patcher[R]](
	patcher C, kind resource.Type,
	decoder func(T) []repository.PatchOption, encoder func(res R) DTO,
	opts ...HandlerOpt,
) http.Handler {
	// Adapt usecase.Patcher (...PatchOption) to the slice-typed
	// endpoint signature kit's NewHandler wants.
	endpoint := func(ctx context.Context, optList []repository.PatchOption) (R, error) {
		return patcher.Patch(ctx, optList...)
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(jsonApiDecodePatchReq(kind, decoder)),
			jsonApiEncoder(encoder, http.StatusOK),
			append(opts,
				HandlerWithErrorEncoder(JsonApiErrorEncoder),
				jsonAPIDocsOverrideSingle[DTO](http.StatusOK),
			)...,
		),
	)
}

// NewJsonApiDeleteHandler wires a DELETE endpoint. The deleteType
// (soft vs hard) is fixed per handler at registration time — usually
// soft, since hard-deletes are a privileged operation that goes
// through a separate code path.
func NewJsonApiDeleteHandler(
	deleter usecase.Deleter, deleteType repository.DeleteType,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, qOpts []query.Option) (struct{}, error) {
		return struct{}{}, deleter.Delete(ctx, deleteType, search.WithQueryOpts(qOpts...))
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(DecodeGetReq([]query.ParseOpt{query.SkipDefaultPagination()})),
			NewEmptyHTTPEncoder(http.StatusNoContent),
			append(opts,
				HandlerWithErrorEncoder(JsonApiErrorEncoder),
				jsonAPIDocsOverrideNoBody(http.StatusNoContent),
			)...,
		),
	)
}

func jsonApiEncoder[I, O any](
	itemMapper func(in I) O,
	successCode int,
) func(context.Context, http.ResponseWriter, any) error {
	return NewHTTPEncoder(
		func(ctx context.Context, w http.ResponseWriter, in I) error {
			return writeBuffered(w, "application/vnd.api+json; charset=utf-8", successCode, func(buf io.Writer) error {
				return jsonapi.MarshalPayload(buf, itemMapper(in), jsonapi.WithInclude(GetJSONAPIIncludes(ctx)...))
			})
		},
	)
}

func jsonApiListEncoder[I, O any](
	itemMapper func(in I) jsonapi.ListResponse[O],
	successCode int,
) func(context.Context, http.ResponseWriter, any) error {
	return NewHTTPEncoder(
		func(ctx context.Context, w http.ResponseWriter, in I) error {
			return writeBuffered(w, "application/vnd.api+json; charset=utf-8", successCode, func(buf io.Writer) error {
				return jsonapi.MarshalManyPayloads(buf, itemMapper(in), jsonapi.WithInclude(GetJSONAPIIncludes(ctx)...))
			})
		},
	)
}

func jsonApiDecodeResourceReq[R, DTO resource.Resource](mapper func(DTO) R, allowEmpty bool) func(_ context.Context, req *http.Request) (R, error) {
	return func(_ context.Context, req *http.Request) (R, error) {
		// allowEmpty: zero-length body produces a zero R rather than a
		// 400. Used by token-driven creates where ctx, not the body,
		// carries the input. Mirrors the handler-layer behavior of
		// HandlerAllowsEmptyReq(true) — keep the two in sync.
		if allowEmpty && req.ContentLength == 0 {
			var zero R
			return zero, nil
		}
		res, err := jsonapi.UnmarshalPayload[DTO](req.Body)
		if err != nil {
			var zero R
			return zero, errors.InvalidArgument("invalid request body")
		}

		return mapper(res.Data), nil
	}
}

func jsonApiDecodePatchReq[T resource.Resource](
	kind resource.Type,
	mapper func(T) []repository.PatchOption,
) func(_ context.Context, req *http.Request) ([]repository.PatchOption, error) {
	return func(_ context.Context, req *http.Request) ([]repository.PatchOption, error) {

		res, err := jsonapi.UnmarshalPayload[T](req.Body)
		if err != nil {
			return nil, errors.InvalidArgument("invalid request body")
		}

		if res.Data.ID() == "" {
			return nil, errors.InvalidArgument("missing resource ID")
		}

		if res.Data.Type() != kind {
			return nil, errors.InvalidArgument("resource type mismatch")
		}

		if pathID := req.PathValue("id"); res.Data.ID() != pathID {
			return nil, errors.InvalidArgument("request ID mismatch")
		}

		return mapper(res.Data), nil
	}
}
