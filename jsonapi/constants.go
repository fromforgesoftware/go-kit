package jsonapi

const (
	specVersion = "1.1"
	// StructTag annotation strings
	annotationJSONAPI      = "jsonapi"
	annotationPrimary      = "primary"
	annotationType         = "type"
	annotationClientID     = "client-id"
	annotationAttribute    = "attr"
	annotationRelation     = "rel"
	annotationPolyRelation = "polyrelation"
	annotationLinks        = "links"
	annotationMeta         = "meta"
	annotationOmitEmpty    = "omitempty"
	annotationOmitZero     = "omitzero"
	annotationISO8601      = "fmt:iso8601"
	annotationRFC3339      = "fmt:rfc3339"
	annotationTimestamp    = "fmt:timestamp"
	annotationFmtDate      = "fmt:date"
	annotationFmtTime      = "fmt:time"
	annotationSeparator    = ","

	ISO8601TimeFormat = "2006-01-02T15:04:05.999Z"

	// MediaType is the identifier for the JSON API media type
	//
	// see http://jsonapi.org/format/#document-structure
	MediaType = "application/vnd.api+json"

	// Pagination Constants
	//
	// http://jsonapi.org/format/#fetching-pagination

	// KeyFirstPage is the key to the links object whose value contains a link to
	// the first page of data
	KeyFirstPage = "first"
	// KeyLastPage is the key to the links object whose value contains a link to
	// the last page of data
	KeyLastPage = "last"
	// KeyPreviousPage is the key to the links object whose value contains a link
	// to the previous page of data
	KeyPreviousPage = "prev"
	// KeyNextPage is the key to the links object whose value contains a link to
	// the next page of data
	KeyNextPage = "next"

	// QueryParamPageNumber is a JSON API query parameter used in a page based
	// pagination strategy in conjunction with QueryParamPageSize
	QueryParamPageNumber = "page[number]"
	// QueryParamPageSize is a JSON API query parameter used in a page based
	// pagination strategy in conjunction with QueryParamPageNumber
	QueryParamPageSize = "page[size]"

	// QueryParamPageOffset is a JSON API query parameter used in an offset based
	// pagination strategy in conjunction with QueryParamPageLimit
	QueryParamPageOffset = "page[offset]"
	// QueryParamPageLimit is a JSON API query parameter used in an offset based
	// pagination strategy in conjunction with QueryParamPageOffset
	QueryParamPageLimit = "page[limit]"

	// QueryParamPageCursor is a JSON API query parameter used with a cursor-based
	// strategy
	QueryParamPageCursor = "page[cursor]"

	// KeySelfLink is the key within a top-level links object that denotes the link that
	// generated the current response document.
	KeySelfLink = "self"
)
