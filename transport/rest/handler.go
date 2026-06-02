package rest

import (
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/fromforgesoftware/go-kit/transport"
)

type (
	HandlerOpt    func(c *handlerConfig)
	handlerConfig struct {
		allowsEmptyReq bool
		opts           []serverOption
		errorEncoder   ErrorEncoder
		getDecoderOpts []GetDecoderOpt
		successStatus  int

		// docs is the composed setup-function built from the
		// HandlerWithOpenAPI opt. nil if the caller never annotated.
		docs func(oc openapi.OperationContext) error

		// docsOverride replaces the auto-generated request/response
		// schema reflection. Used by the JsonApi handler factories to
		// emit the JSON:API envelope instead of the bare DTO.
		docsOverride func(oc openapi.OperationContext) error
	}
)

func defaultHandlerOpts() []HandlerOpt {
	return []HandlerOpt{
		HandlerAllowsEmptyReq(false),
		HandlerWithErrorEncoder(DefaultErrorEncoder),
	}
}

func HandlerAllowsEmptyReq(emptyReqAllowed bool) HandlerOpt {
	return func(c *handlerConfig) {
		c.allowsEmptyReq = emptyReqAllowed
	}
}

func HandlerWithErrorEncoder(ee ErrorEncoder) HandlerOpt {
	return func(c *handlerConfig) {
		c.errorEncoder = ee
	}
}

// HandlerWithGetDecoderOpts threads decoder options through the
// jsonapi Get-handler factory. The canonical use case is opting a
// singleton endpoint (/me, /config) out of the {id} path-segment
// requirement:
//
//	r.Get("/me",
//	    kitrest.NewJsonApiGetHandler(uc, api.UserToDTO, nil,
//	        kitrest.HandlerWithGetDecoderOpts(kitrest.DecodeGetSkipURLPathID()),
//	    ),
//	)
func HandlerWithGetDecoderOpts(opts ...GetDecoderOpt) HandlerOpt {
	return func(c *handlerConfig) {
		c.getDecoderOpts = append(c.getDecoderOpts, opts...)
	}
}

// HandlerWithSuccessStatus overrides the success status code for
// handlers that support it (currently Command). Defaults vary per
// factory (Create = 201, Get/List/Update/Patch = 200, Delete = 204);
// Command defaults to 201 but a "operation on existing resource" command
// (POST /orders/{id}/cancel) often wants 200.
func HandlerWithSuccessStatus(code int) HandlerOpt {
	return func(c *handlerConfig) {
		c.successStatus = code
	}
}

// HandlerWithOpenAPI threads per-operation OpenAPI annotations through
// the handler factory. Composes any number of openapi.OpOpt values
// (Summary, Description, Tags, Security, Errors, Response, …) into a
// single setup function applied during spec collection.
func HandlerWithOpenAPI(opts ...openapi.OpOpt) HandlerOpt {
	return func(c *handlerConfig) {
		prev := c.docs
		c.docs = func(oc openapi.OperationContext) error {
			if prev != nil {
				if err := prev(oc); err != nil {
					return err
				}
			}
			for _, opt := range opts {
				if err := opt(oc); err != nil {
					return err
				}
			}
			return nil
		}
	}
}

// handlerWithDocsOverride installs the JSON:API envelope override for
// jsonapi handler factories. Unexported because callers should never
// need it — JsonApi factories apply it themselves.
func handlerWithDocsOverride(fn func(oc openapi.OperationContext) error) HandlerOpt {
	return func(c *handlerConfig) {
		c.docsOverride = fn
	}
}

type handler[I, O any] struct {
	e          transport.Endpoint[I, O]
	reqDecoder DecodeRequestFunc
	resEncoder EncodeResponseFunc

	allowsEmptyReq bool
	opts           []serverOption
	errorEncoder   ErrorEncoder

	successStatus int
	docs          func(oc openapi.OperationContext) error
	docsOverride  func(oc openapi.OperationContext) error
}

// SetupOperation implements openapi.Preparer. The factory bakes in
// reflection of the request type I and the success response type O;
// jsonapi factories override that via handlerWithDocsOverride; per-route
// HandlerWithOpenAPI opts run last so they always win.
func (h handler[I, O]) SetupOperation(oc openapi.OperationContext) error {
	if h.docsOverride != nil {
		if err := h.docsOverride(oc); err != nil {
			return err
		}
	} else {
		var in I
		var out O
		oc.AddRequest(in)
		status := h.successStatus
		if status == 0 {
			status = http.StatusOK
		}
		oc.AddResponse(status, out)
	}
	if h.docs != nil {
		return h.docs(oc)
	}
	return nil
}

func (h handler[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request, err := h.reqDecoder(ctx, r)
	if err != nil {
		h.errorEncoder(ctx, err, w)
		return
	}

	if request == nil {
		if !h.allowsEmptyReq {
			h.errorEncoder(ctx, http.ErrBodyNotAllowed, w)
			return
		}
	}

	var in I
	if request != nil {
		var ok bool
		in, ok = request.(I)
		if !ok {
			h.errorEncoder(ctx, http.ErrBodyNotAllowed, w)
			return
		}
	}

	response, err := h.e(r.Context(), in)
	if err != nil {
		h.errorEncoder(ctx, err, w)
		return
	}

	if err := h.resEncoder(ctx, w, response); err != nil {
		h.errorEncoder(ctx, err, w)
		return
	}
}

func NewHandler[I, O any](
	e transport.Endpoint[I, O],
	reqDecoder DecodeRequestFunc,
	resEncoder EncodeResponseFunc,
	opts ...HandlerOpt,
) handler[I, O] {
	c := new(handlerConfig)
	for _, opt := range append(defaultHandlerOpts(), opts...) {
		opt(c)
	}

	return handler[I, O]{
		e:              e,
		reqDecoder:     reqDecoder,
		resEncoder:     resEncoder,
		allowsEmptyReq: c.allowsEmptyReq,
		opts:           c.opts,
		errorEncoder:   c.errorEncoder,
		successStatus:  c.successStatus,
		docs:           c.docs,
		docsOverride:   c.docsOverride,
	}
}
