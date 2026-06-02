package rest

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/fromforgesoftware/go-kit/search/query"
)

type DecodeRequestFunc func(context.Context, *http.Request) (request interface{}, err error)

func NewHTTPClientDecoder[O any](ctx context.Context, r *http.Response) (response O, err error) {
	var reader io.ReadCloser
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(r.Body)
		if err != nil {
			var zero O
			return zero, fmt.Errorf("gzip reader: %w", err)
		}
		defer func() { _ = reader.Close() }()
	default:
		reader = r.Body
	}

	out := new(O)
	if err := json.NewDecoder(reader).Decode(&out); err != nil {
		var zero O
		return zero, err
	}

	return *out, nil
}

func NewHTTPDecoder[O any](
	f func(context.Context, *http.Request) (request O, err error),
) func(context.Context, *http.Request) (request any, err error) {
	return func(ctx context.Context, r *http.Request) (request any, err error) {
		out, err := f(ctx, r)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
}

func QueryOptsFromReq(parseOpts ...query.ParseOpt) func(ctx context.Context, r *http.Request) ([]query.Option, error) {
	return func(_ context.Context, r *http.Request) ([]query.Option, error) {
		opts, err := query.ParseOptsFromHTTPReq(r, parseOpts...)
		if err != nil {
			return nil, err
		}

		return opts, nil
	}
}

type getEncoderConfig struct {
	skipURLPathID bool
}

// GetDecoderOpt configures the Get-request decoder used by
// NewJsonApiGetHandler. Plumbed through HandlerWithGetDecoderOpts so
// callers can compose options at the handler-factory level.
type GetDecoderOpt func(c *getEncoderConfig)

// DecodeGetSkipURLPathID disables the "{id}" path-segment requirement
// in the Get-request decoder. Used by singleton endpoints (/me,
// /config, /session) where identity is resolved from the context
// rather than passed as a path id.
func DecodeGetSkipURLPathID() GetDecoderOpt {
	return func(c *getEncoderConfig) {
		c.skipURLPathID = true
	}
}

func DecodeGetReq(parseOpts []query.ParseOpt, opts ...GetDecoderOpt) func(_ context.Context, req *http.Request) ([]query.Option, error) {
	return func(ctx context.Context, req *http.Request) ([]query.Option, error) {
		c := new(getEncoderConfig)
		for _, opt := range opts {
			opt(c)
		}

		var id string
		if !c.skipURLPathID {
			id = req.PathValue("id")
			if id == "" {
				return nil, errors.InvalidArgument("missing url-path-id")
			}
		}

		opts, err := QueryOptsFromReq(parseOpts...)(ctx, req)
		if err != nil {
			return nil, err
		}

		if id != "" {
			opts = append(opts, query.FilterBy(filter.OpEq, "id", id))
		}

		return opts, nil
	}
}

func decodeResourceReq[R any, O any](
	mapper func(O) R,
) func(_ context.Context, req *http.Request) (R, error) {
	return func(_ context.Context, req *http.Request) (R, error) {
		createReq := new(O)
		err := json.NewDecoder(req.Body).Decode(&createReq)
		var zero R
		if err != nil || reflect.DeepEqual(createReq, new(O)) {
			return zero, errors.InvalidArgument("invalid request body")
		}

		return mapper(*createReq), nil
	}
}

func DecodeTokenFromCtx(ctx context.Context, r *http.Request) (interface{}, error) {
	token := auth.TokenFromCtx(ctx)
	if token == nil {
		return nil, errors.Unauthorized("missing or invalid auth token")
	}
	return token.Value(), nil
}

func DecodeEmptyReq(ctx context.Context, r *http.Request) (interface{}, error) {
	return nil, nil
}

func decodePatchReq[T resource.Resource](
	kind resource.Type,
	mapper func(T) []repository.PatchOption,
) func(_ context.Context, req *http.Request) ([]repository.PatchOption, error) {
	return func(_ context.Context, req *http.Request) ([]repository.PatchOption, error) {
		updateReq := new(T)
		dec := json.NewDecoder(req.Body)
		// dec.DisallowUnknownFields()
		err := dec.Decode(&updateReq)
		if err != nil {
			return nil, errors.InvalidArgument("invalid request body")
		}

		r := func(t *T) T { return *t }(updateReq)

		if err := validateUpdateReqData(kind, r); err != nil {
			return nil, err
		}

		if pathID := req.PathValue("id"); r.ID() != pathID {
			return nil, errors.InvalidArgument("request ID mismatch")
		}

		return mapper(*updateReq), nil
	}
}

func validateUpdateReqData[R resource.Resource](kind resource.Type, data R) error {
	if data.ID() == "" {
		return errors.InvalidArgument("missing resource ID")
	}

	if data.Type() != kind {
		return errors.InvalidArgument("resource type mismatch")
	}
	return nil
}
