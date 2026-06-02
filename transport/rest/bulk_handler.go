package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fromforgesoftware/go-kit/application/usecase"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
)

// Bulk handlers implement the JSON:API 1.1+ "primary data is an array"
// shape (BulkCreate) and the Atomic Operations extension
// (AtomicOperations). Filter-based bulk update/delete don't have a
// JSON:API extension — they're modelled as ordinary Commands with
// well-known body schemas: see NewJsonApiBulkUpdateHandler /
// NewJsonApiBulkDeleteHandler below.

// NewJsonApiBulkCreateHandler wires `POST /resources` whose body is
// `{ "data": [ {...}, {...} ] }`. The usecase implements
// usecase.CreatorBatch[R] (one transactional batch insert); the
// response is a primary-data array with the created resources, status
// 201.
func NewJsonApiBulkCreateHandler[R, DTO resource.Resource, C usecase.CreatorBatch[R]](
	creator C,
	decoder func(DTO) R, encoder func(res R) DTO,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, in []R) ([]R, error) {
		return creator.CreateBatch(ctx, in)
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(jsonApiDecodeManyResourceReq(decoder)),
			jsonApiManyEncoder(encoder, http.StatusCreated),
			append(opts, HandlerWithErrorEncoder(JsonApiErrorEncoder))...,
		),
	)
}

// BulkUpdateRequest is the canonical wire shape for
// `POST /resources:batchUpdate`: a filter (server-sided selector) plus
// a patch to apply. Services consume `BulkUpdateRequest[F, P]` via a
// Command-style decoder that builds the usecase command.
//
// The kit doesn't impose a Filter or Patch struct shape — that lives
// in each service's app layer. F and P are whatever the service
// defines; this struct just nails down the envelope on the wire.
type BulkUpdateRequest[F any, P any] struct {
	Filter F `json:"filter"`
	Patch  P `json:"patch"`
}

// BulkUpdateResponse reports how many rows the bulk update touched.
// Services that need richer responses (per-row results, IDs of
// updated rows) should compose a custom Command handler instead.
type BulkUpdateResponse struct {
	resource.RestDTO
	RAffectedCount int `jsonapi:"attr,affectedCount"`
}

// NewJsonApiBulkUpdateHandler wires `POST /resources:batchUpdate`.
// The cmdFn signature is open enough to fit any (filter, patch)
// command shape; the kit provides the wire-envelope contract via
// BulkUpdateRequest. Returns 200 + a BulkUpdateResponse with the
// affected count.
//
//	type BulkMarkPaidCommand struct {
//	    Filter InvoiceFilter
//	    Patch  InvoicePatch
//	}
//	func (uc *invoiceUsecase) MarkPaid(ctx context.Context, cmd BulkMarkPaidCommand) (int, error)
//
//	r.Post("/invoices:batchUpdate",
//	    kitrest.NewJsonApiBulkUpdateHandler[InvoiceFilter, InvoicePatch](
//	        uc.MarkPaid,
//	        func(f InvoiceFilter, p InvoicePatch) BulkMarkPaidCommand {
//	            return BulkMarkPaidCommand{Filter: f, Patch: p}
//	        },
//	    ),
//	)
func NewJsonApiBulkUpdateHandler[F any, P any, Cmd any](
	cmdFn func(ctx context.Context, cmd Cmd) (int, error),
	cmdFromReq func(filter F, patch P) Cmd,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, cmd Cmd) (*BulkUpdateResponse, error) {
		n, err := cmdFn(ctx, cmd)
		if err != nil {
			return nil, err
		}
		out := &BulkUpdateResponse{RAffectedCount: n}
		out.RType = "bulkUpdates"
		return out, nil
	}
	decoder := func(_ context.Context, req *http.Request) (Cmd, error) {
		// Plain JSON, not a JSON:API document — the bulk envelope
		// describes a *command*, not a resource. Inventing a fake
		// "type": "bulkUpdates" resource would muddy the wire contract.
		var body BulkUpdateRequest[F, P]
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			var zero Cmd
			return zero, errors.InvalidArgument("invalid bulk-update body")
		}
		return cmdFromReq(body.Filter, body.Patch), nil
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(decoder),
			jsonApiEncoder(func(r *BulkUpdateResponse) *BulkUpdateResponse { return r }, http.StatusOK),
			append(opts, HandlerWithErrorEncoder(JsonApiErrorEncoder))...,
		),
	)
}

// BulkDeleteRequest is the canonical wire shape for
// `POST /resources:batchDelete`. Filter-only; no patch.
type BulkDeleteRequest[F any] struct {
	Filter F `json:"filter"`
}

// BulkDeleteResponse mirrors BulkUpdateResponse.
type BulkDeleteResponse struct {
	resource.RestDTO
	RAffectedCount int `jsonapi:"attr,affectedCount"`
}

// NewJsonApiBulkDeleteHandler wires `POST /resources:batchDelete`.
func NewJsonApiBulkDeleteHandler[F any, Cmd any](
	cmdFn func(ctx context.Context, cmd Cmd) (int, error),
	cmdFromReq func(filter F) Cmd,
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, cmd Cmd) (*BulkDeleteResponse, error) {
		n, err := cmdFn(ctx, cmd)
		if err != nil {
			return nil, err
		}
		out := &BulkDeleteResponse{RAffectedCount: n}
		out.RType = "bulkDeletes"
		return out, nil
	}
	decoder := func(_ context.Context, req *http.Request) (Cmd, error) {
		var body BulkDeleteRequest[F]
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			var zero Cmd
			return zero, errors.InvalidArgument("invalid bulk-delete body")
		}
		return cmdFromReq(body.Filter), nil
	}
	return WithJSONAPIIncludes(
		NewHandler(
			endpoint,
			NewHTTPDecoder(decoder),
			jsonApiEncoder(func(r *BulkDeleteResponse) *BulkDeleteResponse { return r }, http.StatusOK),
			append(opts, HandlerWithErrorEncoder(JsonApiErrorEncoder))...,
		),
	)
}

// AtomicOperation is one entry in a JSON:API Atomic Operations
// request body. Services that need true cross-resource atomicity wire
// a NewJsonApiAtomicOperationsHandler with a dispatcher that maps
// each operation to the right usecase call inside a single
// transaction.
//
// See https://jsonapi.org/ext/atomic/ for the wire spec. We follow
// the spec's `op` field (add | update | remove); `data` carries the
// resource payload (for add / update) and the `ref` field can be
// used by callers to address an existing row by id.
type AtomicOperation struct {
	Op   string         `json:"op"`
	Ref  *AtomicRef     `json:"ref,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

// AtomicRef points at an existing resource by type + id, used by the
// `update` and `remove` ops.
type AtomicRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AtomicOperationsRequest is the top-level envelope under the
// `urn:ietf:params:jsonapi:ext:atomic` extension namespace. The wire
// field is literally `atomic:operations`.
type AtomicOperationsRequest struct {
	Operations []AtomicOperation `json:"atomic:operations"`
}

// AtomicOperationResult is the per-operation result returned to the
// client. The spec says the response is `atomic:results` with the
// same length and order as the request's `atomic:operations`.
type AtomicOperationResult struct {
	Data map[string]any `json:"data,omitempty"`
}

// AtomicOperationsResponse is the top-level response envelope.
type AtomicOperationsResponse struct {
	Results []AtomicOperationResult `json:"atomic:results"`
}

// NewJsonApiAtomicOperationsHandler wires the JSON:API Atomic
// Operations endpoint. The dispatcher receives the parsed operations
// array; it's the dispatcher's job to run them inside a single
// transaction and produce results in the same order.
//
// This is the most flexible (and most dangerous) bulk shape — any
// mix of add/update/remove across any resource type in one request.
// Use only when true cross-resource atomicity is required.
func NewJsonApiAtomicOperationsHandler(
	dispatcher func(ctx context.Context, ops []AtomicOperation) ([]AtomicOperationResult, error),
	opts ...HandlerOpt,
) http.Handler {
	endpoint := func(ctx context.Context, in *AtomicOperationsRequest) (*AtomicOperationsResponse, error) {
		results, err := dispatcher(ctx, in.Operations)
		if err != nil {
			return nil, err
		}
		return &AtomicOperationsResponse{Results: results}, nil
	}
	decoder := func(_ context.Context, req *http.Request) (*AtomicOperationsRequest, error) {
		var body AtomicOperationsRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, errors.InvalidArgument("invalid atomic:operations body")
		}
		return &body, nil
	}
	encoder := func(_ context.Context, w http.ResponseWriter, in *AtomicOperationsResponse) error {
		return writeBuffered(w, "application/vnd.api+json; ext=\"https://jsonapi.org/ext/atomic\"; charset=utf-8", http.StatusOK, func(buf io.Writer) error {
			return json.NewEncoder(buf).Encode(in)
		})
	}
	return NewHandler(
		endpoint,
		NewHTTPDecoder(decoder),
		NewHTTPEncoder(encoder),
		append(opts, HandlerWithErrorEncoder(JsonApiErrorEncoder))...,
	)
}

// jsonApiDecodeManyResourceReq parses a JSON:API document whose
// primary data is an array (the BulkCreate shape).
func jsonApiDecodeManyResourceReq[R, DTO resource.Resource](
	mapper func(DTO) R,
) func(_ context.Context, req *http.Request) ([]R, error) {
	return func(_ context.Context, req *http.Request) ([]R, error) {
		res, err := jsonapi.UnmarshalManyPayload[DTO](req.Body)
		if err != nil {
			return nil, errors.InvalidArgument("invalid request body")
		}
		out := make([]R, len(res.Data))
		for i, d := range res.Data {
			out[i] = mapper(d)
		}
		return out, nil
	}
}

// jsonApiManyEncoder mirrors jsonApiListEncoder but takes a flat
// slice of R rather than a resource.ListResponse[R]. Used by
// BulkCreate where we return exactly the items we just inserted, no
// pagination meta.
func jsonApiManyEncoder[R, DTO resource.Resource](
	itemMapper func(R) DTO,
	successCode int,
) func(context.Context, http.ResponseWriter, any) error {
	return NewHTTPEncoder(
		func(ctx context.Context, w http.ResponseWriter, in []R) error {
			return writeBuffered(w, "application/vnd.api+json; charset=utf-8", successCode, func(buf io.Writer) error {
				items := make([]DTO, len(in))
				for i, r := range in {
					items[i] = itemMapper(r)
				}
				return jsonapi.MarshalManyPayloads(buf,
					resource.NewListResponse(items, len(items)),
					jsonapi.WithInclude(GetJSONAPIIncludes(ctx)...))
			})
		},
	)
}
