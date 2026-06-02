package rest

import (
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/openapi"
)

// jsonAPIDocsOverrideSingle returns a HandlerOpt that replaces the
// kit's auto-generated request/response reflection (bare DTO) with
// the JSON:API single-resource envelope shape:
//
//	{ "data": { ... DTO ... }, "included": [...], "links": {...}, "meta": {...} }
//
// Applied by every NewJsonApi*Handler factory that returns a single
// resource (Create, Get, Update, Patch, Command-as-resource).
func jsonAPIDocsOverrideSingle[DTO any](successStatus int) HandlerOpt {
	return handlerWithDocsOverride(func(oc openapi.OperationContext) error {
		var doc jsonapi.Document[DTO]
		oc.AddRequest(doc)
		oc.AddResponse(successStatus, doc)
		return nil
	})
}

// jsonAPIDocsOverrideRespOnly is the override for handlers that decode
// from URL/query rather than the body (Get, List, Delete). Only the
// response is shaped as a JSON:API envelope.
func jsonAPIDocsOverrideRespOnly[DTO any](successStatus int) HandlerOpt {
	return handlerWithDocsOverride(func(oc openapi.OperationContext) error {
		var doc jsonapi.Document[DTO]
		oc.AddResponse(successStatus, doc)
		return nil
	})
}

// jsonAPIDocsOverrideList returns the override for list handlers —
// JSON:API multi-resource envelope on the response. Request is parsed
// from query string, so no request body is declared.
func jsonAPIDocsOverrideList[DTO any](successStatus int) HandlerOpt {
	return handlerWithDocsOverride(func(oc openapi.OperationContext) error {
		var doc jsonapi.ListDocument[DTO]
		oc.AddResponse(successStatus, doc)
		return nil
	})
}

// jsonAPIDocsOverrideNoBody is the override for delete (204 No Content).
// No request body, no response body; the override just declares the
// success status with an empty schema.
func jsonAPIDocsOverrideNoBody(successStatus int) HandlerOpt {
	return handlerWithDocsOverride(func(oc openapi.OperationContext) error {
		oc.AddResponse(successStatus, struct{}{})
		return nil
	})
}
