package jsonapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// MarshalOptions contains options for the Marshal functions
type MarshalOptions struct {
	// IncludeRelations is a list of relationship names to include in the response.
	// If empty, all relationships will be included.
	// If non-empty, only the relationships specified will be included.
	// Supports dot notation for nested relationships (e.g., "author.comments")
	IncludeRelations []string
}

// MarshalOption is a function that modifies MarshalOptions
type MarshalOption func(*MarshalOptions)

// WithInclude specifies which relationships to include in the response
func WithInclude(relations ...string) MarshalOption {
	return func(o *MarshalOptions) {
		o.IncludeRelations = relations
	}
}

var (
	// ErrBadJSONAPIStructTag is returned when the Struct field's JSON API
	// annotation is invalid.
	ErrBadJSONAPIStructTag = errors.New("Bad jsonapi struct tag format")
	// ErrBadJSONAPIID is returned when the Struct JSON API annotated "id" field
	// was not a valid numeric type.
	ErrBadJSONAPIID = errors.New(
		"id should be either string, int(8,16,32,64) or uint(8,16,32,64)")
	// ErrExpectedSlice is returned when a variable or argument was expected to
	// be a slice of *Structs; MarshalMany will return this error when its
	// interface{} argument is invalid.
	ErrExpectedSlice = errors.New("models should be a slice of struct pointers")
	// ErrUnexpectedType is returned when marshalling an interface; the interface
	// had to be a pointer or a slice; otherwise this error is returned.
	ErrUnexpectedType = errors.New("models should be a struct pointer or slice of struct pointers")
	// ErrUnexpectedNil is returned when a slice of relation structs contains nil values
	ErrUnexpectedNil = errors.New("slice of struct pointers cannot contain nil")
)

// MarshalPayload writes a jsonapi response for one or many records. The
// related records are sideloaded into the "included" array. If this method is
// given a struct pointer as an argument it will serialize in the form
// "data": {...}. If this method is given a slice of pointers, this method will
// serialize in the form "data": [...]
//
// One Example: you could pass it, w, your http.ResponseWriter, and, models, a
// ptr to a Blog to be written to the response body:
//
//	 func ShowBlog(w http.ResponseWriter, r *http.Request) {
//		 blog := &Blog{}
//
//		 w.Header().Set("Content-Type", jsonapi.MediaType)
//		 w.WriteHeader(http.StatusOK)
//
//		 if err := jsonapi.MarshalPayload(w, blog); err != nil {
//			 http.Error(w, err.Error(), http.StatusInternalServerError)
//		 }
//	 }
//
// Many Example: you could pass it, w, your http.ResponseWriter, and, models, a
// slice of Blog struct instance pointers to be written to the response body:
//
//		 func ListBlogs(w http.ResponseWriter, r *http.Request) {
//	    blogs := []*Blog{}
//
//			 w.Header().Set("Content-Type", jsonapi.MediaType)
//			 w.WriteHeader(http.StatusOK)
//
//			 if err := jsonapi.MarshalPayload(w, blogs); err != nil {
//				 http.Error(w, err.Error(), http.StatusInternalServerError)
//			 }
//		 }
func MarshalPayload[T any](w io.Writer, models T, opts ...MarshalOption) error {
	payload, err := Marshal(models, opts...)
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(payload)
}

func MarshalManyPayloads[T any](w io.Writer, models ListResponse[T], opts ...MarshalOption) error {
	payload, err := MarshalMany(models, opts...)
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(payload)
}

// Marshal does the same as MarshalPayload except it just returns the payload
// and doesn't write out results. Useful if you use your own JSON rendering
// library.
func Marshal[T any](model T, opts ...MarshalOption) (Payloader, error) {
	// Process options
	options := &MarshalOptions{}
	for _, opt := range opts {
		opt(options)
	}

	val := reflect.ValueOf(model)
	// Check that the pointer was to a struct
	if reflect.Indirect(val).Kind() != reflect.Struct {
		return nil, ErrUnexpectedType
	}

	payload, err := marshalOne(model, options)
	if err != nil {
		return nil, err
	}

	if linkableModels, isLinkable := val.Interface().(Linkable); isLinkable {
		jl := linkableModels.JSONAPILinks()
		if er := jl.validate(); er != nil {
			return nil, er
		}
		payload.Links = linkableModels.JSONAPILinks()
	}

	if metableModels, ok := val.Interface().(Metable); ok {
		payload.Meta = metableModels.JSONAPIMeta()
	}

	return payload, nil
}

type ListResponse[T any] interface {
	Results() []T
}

func MarshalMany[T any](models ListResponse[T], opts ...MarshalOption) (Payloader, error) {
	// Process options
	options := &MarshalOptions{}
	for _, opt := range opts {
		opt(options)
	}

	payload, err := marshalMany(models.Results(), options)
	if err != nil {
		return nil, err
	}

	if linkableModels, isLinkable := models.(Linkable); isLinkable {
		jl := linkableModels.JSONAPILinks()
		if er := jl.validate(); er != nil {
			return nil, er
		}
		payload.Links = linkableModels.JSONAPILinks()
	}

	if metableModels, ok := models.(Metable); ok {
		payload.Meta = metableModels.JSONAPIMeta()
	}

	return payload, nil
}

// marshalOne does the same as MarshalOnePayload except it just returns the
// payload and doesn't write out results. Useful is you use your JSON rendering
// library.
func marshalOne(model interface{}, options *MarshalOptions) (*OnePayload, error) {
	included := make(map[string]*Node)

	// Determine if we should include relationships based on options
	shouldSideload := false // Default is to not include relationships
	if options != nil && len(options.IncludeRelations) > 0 {
		// If specific relationships are requested, we'll handle them during visitModelNode
		shouldSideload = true
	}

	rootNode, err := visitModelNode(model, &included, shouldSideload, options)
	if err != nil {
		return nil, err
	}
	payload := &OnePayload{
		Spec: &Spec{
			Version: specVersion,
		},
		Data: rootNode,
	}

	if len(included) > 0 {
		payload.Included = nodeMapValues(&included)
	}

	return payload, nil
}

// marshalMany does the same as MarshalManyPayload except it just returns the
// payload and doesn't write out results. Useful is you use your JSON rendering
// library.
func marshalMany[T any](models []T, options *MarshalOptions) (*ManyPayload, error) {
	payload := &ManyPayload{
		Spec: &Spec{
			Version: specVersion,
		},
		Data: []*Node{},
	}
	included := map[string]*Node{}

	// Determine if we should include relationships based on options
	shouldSideload := false // Default is to not include relationships
	if options != nil && len(options.IncludeRelations) > 0 {
		// If specific relationships are requested, we'll handle them during visitModelNode
		shouldSideload = true
	}

	for _, model := range models {
		node, err := visitModelNode(model, &included, shouldSideload, options)
		if err != nil {
			return nil, err
		}
		payload.Data = append(payload.Data, node)
	}

	if len(included) > 0 {
		payload.Included = nodeMapValues(&included)
	}

	return payload, nil
}

// MarshalOnePayloadEmbedded - This method not meant to for use in
// implementation code, although feel free.  The purpose of this
// method is for use in tests.  In most cases, your request
// payloads for create will be embedded rather than sideloaded for
// related records. This method will serialize a single struct
// pointer into an embedded json response. In other words, there
// will be no, "included", array in the json all relationships will
// be serailized inline in the data.
//
// However, in tests, you may want to construct payloads to post
// to create methods that are embedded to most closely resemble
// the payloads that will be produced by the client. This is what
// this method is intended for.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayloadEmbedded(w io.Writer, model interface{}) error {
	rootNode, err := visitModelNode(model, nil, false, nil)
	if err != nil {
		return err
	}

	payload := &OnePayload{
		Spec: &Spec{
			Version: specVersion,
		},
		Data: rootNode,
	}

	return json.NewEncoder(w).Encode(payload)
}

// selectChoiceTypeStructField returns the first non-nil struct pointer field in the
// specified struct value that has a jsonapi type field defined within it.
// An error is returned if there are no fields matching that definition.
func selectChoiceTypeStructField(structValue reflect.Value) (reflect.Value, error) {
	for i := 0; i < structValue.NumField(); i++ {
		choiceFieldValue := structValue.Field(i)
		choiceTypeField := choiceFieldValue.Type()

		// Must be a pointer
		if choiceTypeField.Kind() != reflect.Pointer {
			continue
		}

		// Must not be nil
		if choiceFieldValue.IsNil() {
			continue
		}

		subtype := choiceTypeField.Elem()
		_, err := jsonapiTypeOfModel(subtype)
		if err == nil {
			return choiceFieldValue, nil
		}
	}

	return reflect.Value{}, errors.New("no non-nil choice field was found in the specified struct")
}

// hasJSONAPIAnnotations returns true if any of the fields of a struct type t
// has a jsonapi annotation. This function will panic if t is not a struct type.
func hasJSONAPIAnnotations(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get(annotationJSONAPI)
		if tag != "" {
			return true
		}
	}
	return false
}

func visitModelNodeAttribute(args []string, node *Node, fieldValue reflect.Value) error {
	var omitEmpty, omitZero, iso8601, rfc3339, timestamp, fmtDate, fmtTime bool

	if len(args) > 2 {
		for _, arg := range args[2:] {
			switch arg {
			case annotationOmitEmpty:
				omitEmpty = true
			case annotationOmitZero:
				omitZero = true
			case annotationISO8601:
				iso8601 = true
			case annotationRFC3339:
				rfc3339 = true
			case annotationTimestamp:
				timestamp = true
			case annotationFmtDate:
				fmtDate = true
			case annotationFmtTime:
				fmtTime = true
			}
		}
	}

	if node.Attributes == nil {
		node.Attributes = make(map[string]interface{})
	}

	// Handle NullableAttr[T]
	if strings.HasPrefix(fieldValue.Type().Name(), "NullableAttr[") {
		// handle unspecified
		if fieldValue.IsNil() {
			return nil
		}

		// handle null
		if fieldValue.MapIndex(reflect.ValueOf(false)).IsValid() {
			node.Attributes[args[1]] = json.RawMessage("null")
			return nil
		} else {

			// handle value
			fieldValue = fieldValue.MapIndex(reflect.ValueOf(true))
		}
	}

	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		t := fieldValue.Interface().(time.Time)

		if t.IsZero() {
			return nil
		}

		if timestamp {
			node.Attributes[args[1]] = t.Unix()
		} else if fmtDate {
			node.Attributes[args[1]] = t.UTC().Format(dateOnlyFormat)
		} else if fmtTime {
			node.Attributes[args[1]] = t.UTC().Format(timeOnlyFormat)
		} else if iso8601 {
			node.Attributes[args[1]] = t.UTC().Format(ISO8601TimeFormat)
		} else if rfc3339 {
			node.Attributes[args[1]] = t.UTC().Format(time.RFC3339)
		} else {
			node.Attributes[args[1]] = t.UTC().Format(ISO8601TimeFormat)
		}
	} else if fieldValue.Type() == reflect.TypeOf(new(time.Time)) {
		// A time pointer may be nil
		if fieldValue.IsNil() {
			if omitEmpty {
				return nil
			}

			node.Attributes[args[1]] = nil
		} else {
			tm := fieldValue.Interface().(*time.Time)

			if tm.IsZero() && omitEmpty {
				return nil
			}

			if timestamp {
				node.Attributes[args[1]] = tm.Unix()
			} else if fmtDate {
				node.Attributes[args[1]] = tm.UTC().Format(dateOnlyFormat)
			} else if fmtTime {
				node.Attributes[args[1]] = tm.UTC().Format(timeOnlyFormat)
			} else if iso8601 {
				node.Attributes[args[1]] = tm.UTC().Format(ISO8601TimeFormat)
			} else if rfc3339 {
				node.Attributes[args[1]] = tm.UTC().Format(time.RFC3339)
			} else {
				node.Attributes[args[1]] = tm.UTC().Format(ISO8601TimeFormat)
			}
		}
	} else {
		// Dealing with a fieldValue that is not a time
		emptyValue := reflect.Zero(fieldValue.Type())

		// See if we need to omit this field
		if omitEmpty && reflect.DeepEqual(fieldValue.Interface(), emptyValue.Interface()) {
			return nil
		}

		// Handle omitZero for numeric fields (int, float types)
		if omitZero {
			// Check if field is numeric (int, float) and zero
			switch fieldValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if fieldValue.Int() == 0 {
					return nil
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if fieldValue.Uint() == 0 {
					return nil
				}
			case reflect.Float32, reflect.Float64:
				if fieldValue.Float() == 0.0 {
					return nil
				}
			}
		}

		isStruct := fieldValue.Type().Kind() == reflect.Struct
		isPointerToStruct := fieldValue.Type().Kind() == reflect.Pointer && fieldValue.Elem().Kind() == reflect.Struct
		isSliceOfStruct := fieldValue.Type().Kind() == reflect.Slice && fieldValue.Type().Elem().Kind() == reflect.Struct
		isSliceOfPointerToStruct := fieldValue.Type().Kind() == reflect.Slice && fieldValue.Type().Elem().Kind() == reflect.Pointer && fieldValue.Type().Elem().Elem().Kind() == reflect.Struct

		if isSliceOfStruct || isSliceOfPointerToStruct {
			if fieldValue.Len() == 0 && (omitEmpty || omitZero) {
				return nil
			}

			var t reflect.Type
			if isSliceOfStruct {
				t = fieldValue.Type().Elem()
			} else {
				t = fieldValue.Type().Elem().Elem()
			}

			// This check is to maintain backwards compatibility with `json` annotated
			// nested structs, which should fall through to "primitive" handling below
			if hasJSONAPIAnnotations(t) {
				// Nested slice of object attributes
				manyNested, err := visitModelNodeRelationships(fieldValue, nil, false)
				if err != nil {
					return fmt.Errorf("failed to marshal slice of nested attribute %q: %w", args[1], err)
				}
				nestedNodes := make([]any, len(manyNested.Data))
				for i, n := range manyNested.Data {
					nestedNodes[i] = n.Attributes
				}
				node.Attributes[args[1]] = nestedNodes
				return nil
			}
		} else if isStruct || isPointerToStruct {
			var t reflect.Type
			if isStruct {
				t = fieldValue.Type()
			} else {
				t = fieldValue.Type().Elem()
			}

			// This check is to maintain backwards compatibility with `json` annotated
			// nested structs, which should fall through to "primitive" handling below
			if hasJSONAPIAnnotations(t) {
				// Nested object attribute
				nested, err := visitModelNode(fieldValue.Interface(), nil, false, nil)
				if err != nil {
					return fmt.Errorf("failed to marshal nested attribute %q: %w", args[1], err)
				}
				node.Attributes[args[1]] = nested.Attributes
				return nil
			}
		}

		// Primitive attribute
		strAttr, ok := fieldValue.Interface().(string)
		if ok {
			node.Attributes[args[1]] = strAttr
		} else {
			node.Attributes[args[1]] = fieldValue.Interface()
		}
	}

	return nil
}

func visitModelNodeRelation(model any, annotation string, args []string, node *Node, fieldValue reflect.Value, included *map[string]*Node, sideload bool, options *MarshalOptions) error {
	var omitEmpty bool

	//add support for 'omitempty' struct tag for marshaling as absent
	if len(args) > 2 {
		omitEmpty = args[2] == annotationOmitEmpty
	}

	// Check if this relationship should be included based on options
	relationName := args[1]
	var nestedOptions *MarshalOptions

	if options != nil && len(options.IncludeRelations) > 0 {
		shouldInclude := false
		isExactMatch := false
		hasNestedPaths := false
		nestedPaths := []string{}

		for _, includePath := range options.IncludeRelations {
			// Support dot notation for nested relationships
			parts := strings.Split(includePath, ".")
			if parts[0] == relationName {
				// This relationship is referenced in at least one include path
				shouldInclude = true

				// Check if this is an exact match (e.g., "coauthors")
				if len(parts) == 1 {
					isExactMatch = true
				}

				// If there are nested parts, add them to the nested options
				if len(parts) > 1 {
					hasNestedPaths = true
					nestedPath := strings.Join(parts[1:], ".")
					nestedPaths = append(nestedPaths, nestedPath)
				}
			}
		}

		// Create nested options with only the relevant paths for this relationship
		if len(nestedPaths) > 0 {
			nestedOptions = &MarshalOptions{
				IncludeRelations: nestedPaths,
			}
		}

		// If we have specific includes and this one isn't in the list at all,
		// don't sideload it
		if !shouldInclude {
			sideload = false
		} else if !isExactMatch && !hasNestedPaths {
			// Not explicitly included and not a parent of any nested path
			sideload = false
		}
	}

	if node.Relationships == nil {
		node.Relationships = make(map[string]interface{})
	}

	// Handle NullableRelationship[T]
	if strings.HasPrefix(fieldValue.Type().Name(), "NullableRelationship[") {

		if fieldValue.MapIndex(reflect.ValueOf(false)).IsValid() {
			innerTypeIsSlice := fieldValue.MapIndex(reflect.ValueOf(false)).Type().Kind() == reflect.Slice
			// handle explicit null
			if innerTypeIsSlice {
				node.Relationships[args[1]] = json.RawMessage("[]")
			} else {
				node.Relationships[args[1]] = json.RawMessage("{\"data\":null}")
			}
			return nil
		} else if fieldValue.MapIndex(reflect.ValueOf(true)).IsValid() {
			// handle value
			fieldValue = fieldValue.MapIndex(reflect.ValueOf(true))
		}
	}

	isSlice := fieldValue.Type().Kind() == reflect.Slice
	if omitEmpty &&
		(isSlice && fieldValue.Len() < 1 ||
			(!isSlice && fieldValue.IsNil())) {
		return nil
	}

	if annotation == annotationPolyRelation {
		// for polyrelation, we'll snoop out the actual relation model
		// through the choice type value by choosing the first non-nil
		// field that has a jsonapi type annotation and overwriting
		// `fieldValue` so normal annotation-assisted marshaling
		// can continue
		if !isSlice {
			choiceValue := fieldValue

			// must be a pointer type
			if choiceValue.Type().Kind() != reflect.Pointer {
				return ErrUnexpectedType
			}

			if choiceValue.IsNil() {
				fieldValue = reflect.ValueOf(nil)
			}
			structValue := choiceValue.Elem()

			// Short circuit if field is omitted from model
			if !structValue.IsValid() {
				return nil
			}

			if found, err := selectChoiceTypeStructField(structValue); err == nil {
				fieldValue = found
			}
		} else {
			// A slice polyrelation field can be... polymorphic... meaning
			// that we might snoop different types within each slice element.
			// Each snooped value will added to this collection and then
			// the recursion will take care of the rest. The only special case
			// is nil. For that, we'll just choose the first
			collection := make([]interface{}, 0)

			for i := 0; i < fieldValue.Len(); i++ {
				itemValue := fieldValue.Index(i)
				// Once again, must be a pointer type
				if itemValue.Type().Kind() != reflect.Pointer {
					return ErrUnexpectedType
				}

				if itemValue.IsNil() {
					return ErrUnexpectedNil
				}

				structValue := itemValue.Elem()

				if found, err := selectChoiceTypeStructField(structValue); err == nil {
					collection = append(collection, found.Interface())
				}
			}

			fieldValue = reflect.ValueOf(collection)
		}
	}

	var relLinks *Links
	if linkableModel, ok := model.(RelationshipLinkable); ok {
		relLinks = linkableModel.JSONAPIRelationshipLinks(args[1])
	}

	var relMeta *Meta
	if metableModel, ok := model.(RelationshipMetable); ok {
		relMeta = metableModel.JSONAPIRelationshipMeta(args[1])
	}

	if isSlice {
		// to-many relationship

		// For nested resources like "subarticles.subarticles":
		// 1. Pass down the nested options for further processing
		// 2. Make sure intermediate resources are properly included
		optsToUse := nestedOptions
		if optsToUse == nil {
			optsToUse = options
		}

		relationship, err := visitModelNodeRelationshipsWithOptions(
			fieldValue,
			included,
			sideload,
			optsToUse,
		)
		if err != nil {
			return err
		}
		relationship.Links = relLinks
		relationship.Meta = relMeta

		shallowNodes := []*Node{}
		for _, n := range relationship.Data {
			if sideload {
				appendIncluded(included, n)
			}
			shallowNodes = append(shallowNodes, toShallowNode(n))
		}

		node.Relationships[args[1]] = &RelationshipManyNode{
			Data:  shallowNodes,
			Links: relationship.Links,
			Meta:  relationship.Meta,
		}
	} else {
		// to-one relationships

		// Handle null relationship case
		if fieldValue.IsNil() {
			node.Relationships[args[1]] = &RelationshipOneNode{Data: nil}
			return nil
		}

		// Pass down nested options for to-one relationships as well
		optsToUse := nestedOptions
		if optsToUse == nil {
			optsToUse = options
		}

		relationship, err := visitModelNode(
			fieldValue.Interface(),
			included,
			sideload,
			optsToUse,
		)

		if err != nil {
			return err
		}

		if sideload {
			appendIncluded(included, relationship)
		}
		node.Relationships[args[1]] = &RelationshipOneNode{
			Data:  toShallowNode(relationship),
			Links: relLinks,
			Meta:  relMeta,
		}
	}
	return nil
}

func visitModelNode(model interface{}, included *map[string]*Node,
	sideload bool, options *MarshalOptions) (*Node, error) {
	node := new(Node)

	// Check if the model implements MarshalHook and call BeforeMarshal if it does
	if model != nil {
		if hook, ok := model.(MarshalHook); ok {
			if err := hook.BeforeMarshal(); err != nil {
				return nil, err
			}
		}
	}

	var er error
	var modelValue reflect.Value
	var modelType reflect.Type
	value := reflect.ValueOf(model)

	if value.Type().Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		modelValue = value.Elem()
		modelType = value.Type().Elem()
	} else {
		modelValue = value
		modelType = value.Type()
	}

	for i := 0; i < modelValue.NumField(); i++ {
		fieldValue := modelValue.Field(i)
		structField := modelValue.Type().Field(i)
		tag := structField.Tag.Get(annotationJSONAPI)

		if tag == "" && structField.Anonymous && fieldValue.Kind() == reflect.Struct {
			var model interface{}
			if fieldValue.CanInterface() {
				model = fieldValue.Interface()
			} else {
				// Create a new instance and copy all field values from the unexported embedded struct
				newValue := reflect.New(fieldValue.Type()).Elem()
				for j := 0; j < fieldValue.NumField(); j++ {
					if fieldValue.Field(j).CanSet() && newValue.Field(j).CanSet() {
						newValue.Field(j).Set(fieldValue.Field(j))
					}
				}
				model = newValue.Interface()
			}

			embeddedNode, err := visitModelNode(model, included, sideload, options)

			if err != nil {
				return nil, err
			}

			// Merge the embedded node's properties into the current node
			if embeddedNode != nil {
				if embeddedNode.ID != "" {
					node.ID = embeddedNode.ID
				}
				if embeddedNode.Type != "" {
					node.Type = embeddedNode.Type
				}
				if embeddedNode.ClientID != "" {
					node.ClientID = embeddedNode.ClientID
				}
				if embeddedNode.Attributes != nil {
					if node.Attributes == nil {
						node.Attributes = make(map[string]interface{})
					}
					for k, v := range embeddedNode.Attributes {
						node.Attributes[k] = v
					}
				}
				if embeddedNode.Relationships != nil {
					if node.Relationships == nil {
						node.Relationships = make(map[string]interface{})
					}
					for k, v := range embeddedNode.Relationships {
						node.Relationships[k] = v
					}
				}
				if embeddedNode.Links != nil {
					node.Links = embeddedNode.Links
				}
				if embeddedNode.Meta != nil {
					node.Meta = embeddedNode.Meta
				}
			}
			continue
		}

		if tag == "" {
			continue
		}

		fieldType := modelType.Field(i)

		args := strings.Split(tag, annotationSeparator)

		if len(args) < 1 {
			er = ErrBadJSONAPIStructTag
			break
		}

		annotation := args[0]

		if annotation == annotationPrimary {
			v := fieldValue

			// Deal with PTRS
			var kind reflect.Kind
			if fieldValue.Kind() == reflect.Pointer {
				kind = fieldType.Type.Elem().Kind()
				v = reflect.Indirect(fieldValue)
			} else {
				kind = fieldType.Type.Kind()
			}

			// Handle allowed types
			switch kind {
			case reflect.String:
				node.ID = v.Interface().(string)
			case reflect.Int:
				node.ID = strconv.FormatInt(int64(v.Interface().(int)), 10)
			case reflect.Int8:
				node.ID = strconv.FormatInt(int64(v.Interface().(int8)), 10)
			case reflect.Int16:
				node.ID = strconv.FormatInt(int64(v.Interface().(int16)), 10)
			case reflect.Int32:
				node.ID = strconv.FormatInt(int64(v.Interface().(int32)), 10)
			case reflect.Int64:
				node.ID = strconv.FormatInt(v.Interface().(int64), 10)
			case reflect.Uint:
				node.ID = strconv.FormatUint(uint64(v.Interface().(uint)), 10)
			case reflect.Uint8:
				node.ID = strconv.FormatUint(uint64(v.Interface().(uint8)), 10)
			case reflect.Uint16:
				node.ID = strconv.FormatUint(uint64(v.Interface().(uint16)), 10)
			case reflect.Uint32:
				node.ID = strconv.FormatUint(uint64(v.Interface().(uint32)), 10)
			case reflect.Uint64:
				node.ID = strconv.FormatUint(v.Interface().(uint64), 10)
			default:
				// We had a JSON float (numeric), but our field was not one of the
				// allowed numeric types
				er = ErrBadJSONAPIID
			}

			if er != nil {
				break
			}

			// Set type from args if provided (backward compatibility)
			if len(args) >= 2 {
				node.Type = args[1]
			}
		} else if annotation == annotationType {
			node.Type = fieldValue.String()
		} else if annotation == annotationClientID {
			clientID := fieldValue.String()
			if clientID != "" {
				node.ClientID = clientID
			}
		} else if annotation == annotationAttribute {
			er = visitModelNodeAttribute(args, node, fieldValue)
			if er != nil {
				break
			}
		} else if annotation == annotationRelation || annotation == annotationPolyRelation ||
			strings.HasPrefix(annotation, annotationRelation+":") {
			// If it has rel: prefix, adjust the args to work with the existing implementation
			if strings.HasPrefix(annotation, annotationRelation+":") {
				// Create new args with the relationship name
				relName := strings.TrimPrefix(annotation, annotationRelation+":")
				newArgs := []string{annotationRelation, relName}
				if len(args) > 1 {
					newArgs = append(newArgs, args[1:]...)
				}
				args = newArgs
				annotation = annotationRelation
			}
			er = visitModelNodeRelation(model, annotation, args, node, fieldValue, included, sideload, options)
			if er != nil {
				break
			}
		} else if annotation == annotationLinks {
			// Nothing. Ignore this field, as Links fields are only for unmarshaling requests.
			// The Linkable interface methods are used for marshaling data in a response.
		} else if strings.HasPrefix(annotation, annotationMeta+":") {
			// Handle meta fields (e.g., meta:print)
			metaName := strings.TrimPrefix(annotation, annotationMeta+":")

			// Create Meta if it doesn't exist
			if node.Meta == nil {
				node.Meta = &Meta{}
			}

			// Deserialize value into node.Meta[metaName]
			if !fieldValue.IsNil() {
				meta := fieldValue.Interface()

				// Check for omitempty
				omitEmpty := false
				if len(args) > 1 {
					for _, arg := range args[1:] {
						if arg == annotationOmitEmpty {
							omitEmpty = true
							break
						}
					}
				}

				// Skip if field should be omitted when empty
				if omitEmpty {
					metaValue := reflect.ValueOf(meta)
					if metaValue.IsZero() {
						continue
					}
				}

				(*node.Meta)[metaName] = meta
			}
		} else {
			er = ErrBadJSONAPIStructTag
			break
		}
	}

	if er != nil {
		return nil, er
	}

	if linkableModel, isLinkable := model.(Linkable); isLinkable {
		jl := linkableModel.JSONAPILinks()
		if er := jl.validate(); er != nil {
			return nil, er
		}
		node.Links = linkableModel.JSONAPILinks()
	}

	if metableModel, ok := model.(Metable); ok {
		node.Meta = metableModel.JSONAPIMeta()
	}

	return node, nil
}

// toShallowNode takes a node and returns a shallow version of the node.
// If the ID is empty, we include attributes into the shallow version.
//
// An example of where this is useful would be if an object
// within a relationship can be created at the same time as
// the root node.
//
// This is not 1.0 jsonapi spec compliant--it's a bespoke variation on
// resource object identifiers discussed in the pending 1.1 spec.
func toShallowNode(node *Node) *Node {
	ret := &Node{Type: node.Type}
	if node.ID == "" {
		ret.Attributes = node.Attributes
	} else {
		ret.ID = node.ID
	}
	return ret
}

func visitModelNodeRelationships(models reflect.Value, included *map[string]*Node,
	sideload bool) (*RelationshipManyNode, error) {
	return visitModelNodeRelationshipsWithOptions(models, included, sideload, nil)
}

func visitModelNodeRelationshipsWithOptions(models reflect.Value, included *map[string]*Node,
	sideload bool, options *MarshalOptions) (*RelationshipManyNode, error) {
	nodes := []*Node{}

	for i := 0; i < models.Len(); i++ {
		model := models.Index(i)
		if !model.IsValid() || (model.Kind() == reflect.Pointer && model.IsNil()) {
			return nil, ErrUnexpectedNil
		}

		n := model.Interface()

		node, err := visitModelNode(n, included, sideload, options)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, node)
	}

	return &RelationshipManyNode{Data: nodes}, nil
}

func appendIncluded(m *map[string]*Node, nodes ...*Node) {
	included := *m

	for _, n := range nodes {
		k := fmt.Sprintf("%s,%s", n.Type, n.ID)

		if _, hasNode := included[k]; hasNode {
			continue
		}

		included[k] = n
	}
}

func nodeMapValues(m *map[string]*Node) []*Node {
	mp := *m
	nodes := make([]*Node, len(mp))

	i := 0
	for _, n := range mp {
		nodes[i] = n
		i++
	}

	// Sort nodes by type and then by ID for consistent output
	slices.SortFunc(nodes, func(a, b *Node) int {
		// First compare by type
		if a.Type > b.Type {
			return 1
		}
		if a.Type < b.Type {
			return -1
		}
		// Same type, compare by ID
		if a.ID > b.ID {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		return 0
	})

	return nodes
}
