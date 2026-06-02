package jsonapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/instance"
)

const (
	unsupportedStructTagMsg = "Unsupported jsonapi tag annotation, %s"
	dateOnlyFormat          = "2006-01-02"
	timeOnlyFormat          = "15:04:05"
)

var (
	// ErrInvalidTime is returned when a struct has a time.Time type field, but
	// the JSON value was not a unix timestamp integer.
	ErrInvalidTime = errors.New("Only numbers can be parsed as dates, unix timestamps")
	// ErrInvalidISO8601 is returned when a struct has a time.Time type field and includes
	// "iso8601" in the tag spec, but the JSON value was not an ISO8601 timestamp string.
	ErrInvalidISO8601 = errors.New("Only strings can be parsed as dates, ISO8601 timestamps")
	// ErrInvalidRFC3339 is returned when a struct has a time.Time type field and includes
	// "rfc3339" in the tag spec, but the JSON value was not an RFC3339 timestamp string.
	ErrInvalidRFC3339 = errors.New("Only strings can be parsed as dates, RFC3339 timestamps")
	// ErrInvalidDateFormat is returned when a struct has a time.Time type field and includes
	// "fmt:date" in the tag spec, but the JSON value was not a date string (YYYY-MM-DD).
	ErrInvalidDateFormat = errors.New("Only strings can be parsed as dates, format: YYYY-MM-DD")
	// ErrInvalidTimeFormat is returned when a struct has a time.Time type field and includes
	// "fmt:time" in the tag spec, but the JSON value was not a time string (HH:MM:SS).
	ErrInvalidTimeFormat = errors.New("Only strings can be parsed as times, format: HH:MM:SS")
	// ErrUnknownFieldNumberType is returned when the JSON value was a float
	// (numeric) but the Struct field was a non numeric type (i.e. not int, uint,
	// float, etc)
	ErrUnknownFieldNumberType = errors.New("The struct field was not of a known number type")
	// ErrInvalidType is returned when the given type is incompatible with the expected type.
	ErrInvalidType = errors.New("Invalid type provided") // I wish we used punctuation.
	// ErrTypeNotFound is returned when the given type not found on the model.
	ErrTypeNotFound = errors.New("no primary type annotation found on model")
)

// ErrUnsupportedPtrType is returned when the Struct field was a pointer but
// the JSON value was of a different type
type ErrUnsupportedPtrType struct {
	rf          reflect.Value
	t           reflect.Type
	structField reflect.StructField
}

func (eupt ErrUnsupportedPtrType) Error() string {
	typeName := eupt.t.Elem().Name()
	kind := eupt.t.Elem().Kind()
	if kind.String() != "" && kind.String() != typeName {
		typeName = fmt.Sprintf("%s (%s)", typeName, kind.String())
	}
	return fmt.Sprintf(
		"jsonapi: Can't unmarshal %+v (%s) to struct field `%s`, which is a pointer to `%s`",
		eupt.rf, eupt.rf.Type().Kind(), eupt.structField.Name, typeName,
	)
}

func newErrUnsupportedPtrType(rf reflect.Value, t reflect.Type, structField reflect.StructField) error {
	return ErrUnsupportedPtrType{rf, t, structField}
}

type UnmarshalResult[T any] struct {
	Data T
	Meta any
}

// UnmarshalPayload converts an io into a struct instance using jsonapi tags on
// struct fields. This method supports single request payloads only, at the
// moment. Bulk creates and updates are not supported yet.
//
// Will Unmarshal embedded and sideloaded payloads.  The latter is only possible if the
// object graph is complete.  That is, in the "relationships" data there are type and id,
// keys that correspond to records in the "included" array.
//
// For example you could pass it, in, req.Body and, model, a BlogPost
// struct instance to populate in an http handler,
//
//	func CreateBlog(w http.ResponseWriter, r *http.Request) {
//		blog := new(Blog)
//
//		if err := jsonapi.UnmarshalPayload(r.Body, blog); err != nil {
//			http.Error(w, err.Error(), 500)
//			return
//		}
//
//		// ...do stuff with your blog...
//
//		w.Header().Set("Content-Type", jsonapi.MediaType)
//		w.WriteHeader(201)
//
//		if err := jsonapi.MarshalPayload(w, blog); err != nil {
//			http.Error(w, err.Error(), 500)
//		}
//	}
//
// Visit https://github.com/google/jsonapi#create for more info.
//
// model interface{} should be a pointer to a struct.
func UnmarshalPayload[T any](in io.Reader) (*UnmarshalResult[T], error) {
	res := &UnmarshalResult[T]{}

	payload := new(OnePayload)

	if err := json.NewDecoder(in).Decode(payload); err != nil {
		return nil, err
	}

	model := instance.New[T]()

	includedMap := make(map[string]*Node)
	if payload.Included != nil {
		for _, included := range payload.Included {
			key := fmt.Sprintf("%s,%s", included.Type, included.ID)
			includedMap[key] = included
		}
	}

	err := unmarshalNode(payload.Data, reflect.ValueOf(model), &includedMap)
	if err != nil {
		return nil, err
	}

	res.Data = model

	return res, nil
}

// UnmarshalManyPayload converts an io into a set of struct instances using
// jsonapi tags on the type's struct fields.
func UnmarshalManyPayload[T any](in io.Reader) (*UnmarshalResult[[]T], error) {
	res := &UnmarshalResult[[]T]{}

	payload := new(ManyPayload)
	if err := json.NewDecoder(in).Decode(payload); err != nil {
		return nil, err
	}
	if payload.Data == nil {
		return res, nil
	}

	models := []T{}                   // will be populated from the "data"
	includedMap := map[string]*Node{} // will be populate from the "included"

	if payload.Included != nil {
		for _, included := range payload.Included {
			key := fmt.Sprintf("%s,%s", included.Type, included.ID)
			includedMap[key] = included
		}
	}

	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Pointer {
		t = reflect.PointerTo(t)
	}
	for _, data := range payload.Data {
		model := reflect.New(t.Elem())
		err := unmarshalNode(data, model, &includedMap)
		if err != nil {
			return nil, err
		}
		models = append(models, model.Interface().(T))
	}

	res.Data = models
	res.Meta = payload.Meta

	return res, nil
}

func jsonapiTypeOfModel(structModel reflect.Type) (string, error) {
	for i := 0; i < structModel.NumField(); i++ {
		fieldType := structModel.Field(i)
		args, err := getStructTags(fieldType)

		// A jsonapi tag was found, but it was improperly structured
		if err != nil {
			return "", err
		}

		if len(args) < 2 {
			continue
		}

		if args[0] == annotationPrimary {
			return args[1], nil
		}
	}

	return "", ErrTypeNotFound
}

// structFieldIndex holds a bit of information about a type found at a struct field index
type structFieldIndex struct {
	Type     reflect.Type
	FieldNum int
}

// choiceStructMapping reflects on a value that may be a slice
// of choice type structs or a choice type struct. A choice type
// struct is a struct comprised of pointers to other jsonapi models,
// only one of which is populated with a value by the decoder.
//
// The specified type is probed and a map is generated that maps the
// underlying model type (its 'primary' type) to the field number
// within the choice type struct. This data can then be used to correctly
// assign each data relationship node to the correct choice type
// struct field.
//
// For example, if the `choice` type was
//
//	type OneOfMedia struct {
//		Video *Video
//		Image *Image
//	}
//
// then the resulting map would be
//
//	{
//	  "videos" => {Video, 0}
//	  "images" => {Image, 1}
//	}
//
// where `"videos"` is the value of the `primary` annotation on the `Video` model
func choiceStructMapping(choice reflect.Type) (result map[string]structFieldIndex) {
	result = make(map[string]structFieldIndex)

	for choice.Kind() != reflect.Struct {
		choice = choice.Elem()
	}

	for i := 0; i < choice.NumField(); i++ {
		fieldType := choice.Field(i)

		// Must be a pointer
		if fieldType.Type.Kind() != reflect.Pointer {
			continue
		}

		subtype := fieldType.Type.Elem()

		// Must be a pointer to struct
		if subtype.Kind() != reflect.Struct {
			continue
		}

		if t, err := jsonapiTypeOfModel(subtype); err == nil {
			result[t] = structFieldIndex{
				Type:     subtype,
				FieldNum: i,
			}
		}
	}

	return result
}

func getStructTags(field reflect.StructField) ([]string, error) {
	tag := field.Tag.Get("jsonapi")
	if tag == "" {
		return []string{}, nil
	}

	args := strings.Split(tag, ",")
	if len(args) < 1 {
		return nil, ErrBadJSONAPIStructTag
	}

	return args, nil
}

// unmarshalNodeMaybeChoice populates a model that may or may not be
// a choice type struct that corresponds to a polyrelation or relation
func unmarshalNodeMaybeChoice(m *reflect.Value, data *Node, annotation string, choiceTypeMapping map[string]structFieldIndex, included *map[string]*Node) error {
	// This will hold either the value of the choice type model or the actual
	// model, depending on annotation
	var actualModel = *m
	var choiceElem *structFieldIndex = nil

	if annotation == annotationPolyRelation {
		c, ok := choiceTypeMapping[data.Type]
		if !ok {
			// If there is no valid choice field to assign this type of relation,
			// this shouldn't necessarily be an error because a newer version of
			// the API could be communicating with an older version of the client
			// library, in which case all choice variants would be nil.
			return nil
		}
		choiceElem = &c
		actualModel = reflect.New(choiceElem.Type)
	}

	if err := unmarshalNode(
		fullNode(data, included),
		actualModel,
		included,
	); err != nil {
		return err
	}

	if choiceElem != nil {
		// actualModel is a pointer to the model type
		// m is a pointer to a struct that should hold the actualModel
		// at choiceElem.FieldNum
		v := m.Elem()
		v.Field(choiceElem.FieldNum).Set(actualModel)
	}
	return nil
}

func unmarshalNode(data *Node, model reflect.Value, included *map[string]*Node) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("data is not a jsonapi representation of '%v'", model.Type())
		}
	}()

	modelValue := model.Elem()
	modelType := modelValue.Type()
	polyrelationFields := map[string]reflect.Type{}

	var er error

	// preprocess the model to find polyrelation fields
	for i := 0; i < modelValue.NumField(); i++ {
		fieldValue := modelValue.Field(i)
		fieldType := modelType.Field(i)

		args, err := getStructTags(fieldType)
		if err != nil {
			er = err
			break
		}

		if len(args) < 2 {
			continue
		}

		annotation := args[0]
		name := args[1]

		if annotation == annotationPolyRelation {
			polyrelationFields[name] = fieldValue.Type()
		}
	}

	for i := 0; i < modelValue.NumField(); i++ {
		fieldValue := modelValue.Field(i)
		fieldType := modelType.Field(i)

		args, err := getStructTags(fieldType)
		if err != nil {
			er = err
			break
		}
		if len(args) == 0 {
			if fieldValue.Kind() == reflect.Struct && fieldType.Anonymous {
				fieldPtr := fieldValue.Addr()
				// Recursively unmarshal the embedded struct
				err := unmarshalNode(data, fieldPtr, included)
				if err != nil {
					er = err
					break
				}
			}
			continue
		}
		annotation := args[0]

		if annotation == annotationPrimary {
			if data.ID == "" {
				continue
			}

			// ID will have to be transmitted as astring per the JSON API spec
			v := reflect.ValueOf(data.ID)

			// Deal with PTRS
			var kind reflect.Kind
			if fieldValue.Kind() == reflect.Pointer {
				kind = fieldType.Type.Elem().Kind()
			} else {
				kind = fieldType.Type.Kind()
			}

			// Handle String case
			if kind == reflect.String {
				assign(fieldValue, v)
				continue
			}

			// Value was not a string... only other supported type was a numeric,
			// which would have been sent as a float value.
			floatValue, err := strconv.ParseFloat(data.ID, 64)
			if err != nil {
				// Could not convert the value in the "id" attr to a float
				er = ErrBadJSONAPIID
				break
			}

			// Convert the numeric float to one of the supported ID numeric types
			// (int[8,16,32,64] or uint[8,16,32,64])
			idValue, err := handleNumeric(floatValue, fieldType.Type, fieldValue)
			if err != nil {
				// We had a JSON float (numeric), but our field was not one of the
				// allowed numeric types
				er = ErrBadJSONAPIID
				break
			}

			assign(fieldValue, idValue)
		} else if annotation == annotationType {
			if data.Type == "" {
				continue
			}

			assign(fieldValue, reflect.ValueOf(data.Type))
		} else if annotation == annotationClientID {
			if data.ClientID == "" {
				continue
			}

			fieldValue.Set(reflect.ValueOf(data.ClientID))
		} else if annotation == annotationAttribute {
			attributes := data.Attributes

			if attributes == nil || len(data.Attributes) == 0 {
				continue
			}

			attribute := attributes[args[1]]

			// continue if the attribute was not included in the request
			if attribute == nil {
				continue
			}

			structField := fieldType
			value, err := unmarshalAttribute(attribute, args, structField, fieldValue)
			if err != nil {
				er = err
				break
			}

			assign(fieldValue, value)
		} else if annotation == annotationRelation || annotation == annotationPolyRelation || strings.HasPrefix(annotation, annotationRelation+":") || strings.HasPrefix(annotation, annotationPolyRelation+":") {
			// Extract the relationship name if using the "rel:name" format
			var relName string
			if strings.HasPrefix(annotation, annotationRelation+":") || strings.HasPrefix(annotation, annotationPolyRelation+":") {
				parts := strings.Split(annotation, ":")
				relName = parts[1]
				annotation = parts[0] // Set to just "rel" or "polyrelation"
			} else {
				relName = args[1]
			}
			isSlice := fieldValue.Type().Kind() == reflect.Slice

			// No relations of the given name were provided
			if data.Relationships == nil || data.Relationships[relName] == nil {
				continue
			}

			// If this is a polymorphic relation, each data relationship needs to be assigned
			// to it's appropriate choice field and fieldValue should be a choice
			// struct type field.
			var choiceMapping map[string]structFieldIndex = nil
			if annotation == annotationPolyRelation {
				choiceMapping = choiceStructMapping(fieldValue.Type())
			}

			if isSlice {
				// to-many relationship
				relationship := new(RelationshipManyNode)
				sliceType := fieldValue.Type()

				buf := bytes.NewBuffer(nil)

				json.NewEncoder(buf).Encode(data.Relationships[relName]) //nolint:errcheck
				json.NewDecoder(buf).Decode(relationship)                //nolint:errcheck

				data := relationship.Data

				// This will hold either the value of the slice of choice type models or
				// the slice of models, depending on the annotation
				models := reflect.New(sliceType).Elem()

				for _, n := range data {
					// This will hold either the value of the choice type model or the actual
					// model, depending on annotation
					m := reflect.New(sliceType.Elem().Elem())

					err = unmarshalNodeMaybeChoice(&m, n, annotation, choiceMapping, included)
					if err != nil {
						er = err
						break
					}

					models = reflect.Append(models, m)
				}

				fieldValue.Set(models)
			} else {
				// to-one relationships
				relationship := new(RelationshipOneNode)

				buf := bytes.NewBuffer(nil)
				relDataStr := data.Relationships[relName]
				json.NewEncoder(buf).Encode(relDataStr) //nolint:errcheck

				isExplicitNull := false
				relationshipDecodeErr := json.NewDecoder(buf).Decode(relationship)
				if relationshipDecodeErr == nil && relationship.Data == nil {
					// If the relationship was a valid node and relationship data was null
					// this indicates disassociating the relationship
					isExplicitNull = true
				} else if relationshipDecodeErr != nil {
					er = fmt.Errorf("could not unmarshal json: %w", relationshipDecodeErr)
				}

				// This will hold either the value of the choice type model or the actual
				// model, depending on annotation
				m := reflect.New(fieldValue.Type().Elem())

				// Nullable relationships have an extra pointer indirection
				// unwind that here
				if strings.HasPrefix(fieldType.Type.Name(), "NullableRelationship[") {
					if m.Kind() == reflect.Pointer {
						m = reflect.New(fieldValue.Type().Elem().Elem())
					}
				}
				/*
					http://jsonapi.org/format/#document-resource-object-relationships
					http://jsonapi.org/format/#document-resource-object-linkage
					relationship can have a data node set to null (e.g. to disassociate the relationship)
					so unmarshal and set fieldValue only if data obj is not null
				*/
				if relationship.Data == nil {
					// Explicit null supplied for the field value
					// If a nullable relationship we set the field value to a map with a single entry
					if isExplicitNull && strings.HasPrefix(fieldType.Type.Name(), "NullableRelationship[") {
						fieldValue.Set(reflect.MakeMapWithSize(fieldValue.Type(), 1))
						fieldValue.SetMapIndex(reflect.ValueOf(false), m)
					}
					continue
				}

				// If the field is also a polyrelation field, then prefer the polyrelation.
				// Otherwise stop processing this node.
				// This is to allow relation and polyrelation fields to coexist, supporting deprecation for consumers
				if pFieldType, ok := polyrelationFields[relName]; ok && fieldValue.Type() != pFieldType {
					continue
				}

				err = unmarshalNodeMaybeChoice(&m, relationship.Data, annotation, choiceMapping, included)
				if err != nil {
					er = err
					break
				}

				if strings.HasPrefix(fieldType.Type.Name(), "NullableRelationship[") {
					fieldValue.Set(reflect.MakeMapWithSize(fieldValue.Type(), 1))
					fieldValue.SetMapIndex(reflect.ValueOf(true), m)
				} else {
					fieldValue.Set(m)
				}
			}
		} else if annotation == annotationLinks {
			if data.Links == nil {
				continue
			}

			links := make(Links, len(*data.Links))

			for k, v := range *data.Links {
				link := v // default case (including string urls)

				// Unmarshal link objects to Link
				if t, ok := v.(map[string]interface{}); ok {
					unmarshaledHref := ""
					href, ok := t["href"].(string)
					if ok {
						unmarshaledHref = href
					}

					unmarshaledMeta := make(Meta)
					if meta, ok := t["meta"].(map[string]interface{}); ok {
						for metaK, metaV := range meta {
							unmarshaledMeta[metaK] = metaV
						}
					}

					link = Link{
						Href: unmarshaledHref,
						Meta: unmarshaledMeta,
					}
				}

				links[k] = link
			}

			if err != nil {
				er = err
				break
			}

			assign(fieldValue, reflect.ValueOf(links))
		} else if strings.HasPrefix(annotation, annotationMeta+":") {
			// Handle meta fields (e.g., meta:print)
			metaName := strings.TrimPrefix(annotation, annotationMeta+":")

			if data.Meta == nil {
				continue
			}

			metaData, ok := (*data.Meta)[metaName]
			if !ok {
				continue
			}

			// Handle meta field as struct
			if fieldValue.Kind() == reflect.Pointer {
				model := reflect.New(fieldValue.Type().Elem())
				node := &Node{
					Attributes: metaData.(map[string]interface{}),
				}

				if err := unmarshalNode(node, model, nil); err != nil {
					er = err
					break
				}

				fieldValue.Set(model)
			}
		} else {
			er = fmt.Errorf(unsupportedStructTagMsg, annotation)
		}
	}

	// If there's no error and the model can be interfaced, check for AfterUnmarshal hook
	if er == nil && model.CanInterface() {
		// Get the model as an interface
		modelInterface := model.Interface()
		if modelInterface != nil {
			// Check if it implements the UnmarshalHook interface
			if hook, ok := modelInterface.(UnmarshalHook); ok {
				// Call the AfterUnmarshal method
				if err := hook.AfterUnmarshal(); err != nil {
					return err
				}
				// Update the model value if needed
				model = reflect.ValueOf(modelInterface)
			}
		}
	}

	return er
}

func fullNode(n *Node, included *map[string]*Node) *Node {
	includedKey := fmt.Sprintf("%s,%s", n.Type, n.ID)

	if included != nil && (*included)[includedKey] != nil {
		return (*included)[includedKey]
	}

	return n
}

// assign will take the value specified and assign it to the field; if
// field is expecting a ptr assign will assign a ptr.
func assign(field, value reflect.Value) {
	value = reflect.Indirect(value)

	if field.Kind() == reflect.Pointer {
		// initialize pointer so it's value
		// can be set by assignValue
		field.Set(reflect.New(field.Type().Elem()))
		field = field.Elem()

	}

	assignValue(field, value)
}

// assign assigns the specified value to the field,
// expecting both values not to be pointer types.
func assignValue(field, value reflect.Value) {
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		field.SetInt(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		field.SetUint(value.Uint())
	case reflect.Float32, reflect.Float64:
		field.SetFloat(value.Float())
	case reflect.String:
		field.SetString(value.String())
	case reflect.Bool:
		field.SetBool(value.Bool())
	default:
		field.Set(value)
	}
}

func unmarshalAttribute(
	attribute interface{},
	args []string,
	structField reflect.StructField,
	fieldValue reflect.Value) (value reflect.Value, err error) {
	value = reflect.ValueOf(attribute)
	fieldType := structField.Type

	if value.CanInterface() {
		_, isCustomUnmarshal := value.Interface().(interface{ UnmarshalJSONAPIField(data []byte) error })
		if isCustomUnmarshal {
			return handleUnmarshallJSONAPIField(attribute, fieldValue)
		}
		// Also check if the pointer to the value implements the interface
		if fieldValue.CanAddr() {
			_, isCustomUnmarshal := fieldValue.Addr().Interface().(interface{ UnmarshalJSONAPIField(data []byte) error })
			if isCustomUnmarshal {
				return handleUnmarshallJSONAPIField(attribute, fieldValue)
			}
		}
	}

	// Handle NullableAttr[T]
	if strings.HasPrefix(fieldValue.Type().Name(), "NullableAttr[") {
		value, err = handleNullable(attribute, args, structField, fieldValue)
		return
	}

	// Handle field of type []string
	if fieldValue.Type() == reflect.TypeOf([]string{}) {
		value, err = handleStringSlice(attribute)
		return
	}

	// Handle field of type time.Time
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) ||
		fieldValue.Type() == reflect.TypeOf(new(time.Time)) {
		value, err = handleTime(attribute, args, fieldValue)
		return
	}

	if fieldValue.Type().Kind() == reflect.Interface {
		return reflect.ValueOf(attribute), nil
	}

	// Handle field of type struct
	if fieldValue.Type().Kind() == reflect.Struct {
		value, err = handleStruct(attribute, fieldValue)
		return
	}

	// Handle field containing slice of structs
	if fieldValue.Type().Kind() == reflect.Slice &&
		reflect.TypeOf(fieldValue.Interface()).Elem().Kind() == reflect.Struct {
		value, err = handleStructSlice(attribute, fieldValue)
		return
	}

	if fieldValue.Type().Kind() == reflect.Slice &&
		reflect.TypeOf(fieldValue.Interface()).Elem().Kind() == reflect.Pointer {
		value, err = handleStructPointerSlice(attribute, args, fieldValue)
		return
	}

	// JSON value was a float (numeric)
	if value.Kind() == reflect.Float64 {
		value, err = handleNumeric(attribute, fieldType, fieldValue)
		return
	}

	// Field was a Pointer type
	if fieldValue.Kind() == reflect.Pointer {
		value, err = handlePointer(attribute, fieldType, fieldValue, structField)
		return
	}

	// Handle time.Duration
	if fieldValue.Type() == reflect.TypeOf(time.Duration(0)) {
		value, err = handleDuration(attribute)
		return
	}

	// Handle []int slice
	if fieldValue.Type() == reflect.TypeOf([]int{}) {
		value, err = handleIntSlice(attribute)
		return
	}

	// Handle map[string]string
	if fieldValue.Type() == reflect.TypeOf(map[string]string{}) {
		value, err = handleStringMap(attribute)
		return
	}

	// Handle map[string][]string
	if fieldValue.Type() == reflect.TypeOf(map[string][]string{}) {
		value, err = handleStringSliceMap(attribute)
		return
	}

	// Handle map[string]interface{}
	if fieldValue.Type() == reflect.TypeOf(map[string]interface{}{}) {
		value, err = handleInterfaceMap(attribute)
		return
	}

	// Handle custom string-based types (like enums)
	if fieldValue.Type().Kind() == reflect.String && fieldValue.Type() != reflect.TypeOf("") {
		if strVal, ok := attribute.(string); ok {
			value = reflect.ValueOf(strVal).Convert(fieldValue.Type())
			return
		}
	}

	// Handle json.RawMessage
	if fieldValue.Type() == reflect.TypeOf(json.RawMessage{}) {
		data, marshalErr := json.Marshal(attribute)
		if marshalErr != nil {
			err = marshalErr
			return
		}
		value = reflect.ValueOf(json.RawMessage(data))
		return
	}

	// As a final catch-all, ensure types line up to avoid a runtime panic.
	if fieldValue.Kind() != value.Kind() {
		err = ErrInvalidType
		return
	}

	return
}

func handleUnmarshallJSONAPIField(
	attr any, dtoAttr reflect.Value,
) (val reflect.Value, err error) {
	bs, err := json.Marshal(attr)
	if err != nil {
		return reflect.ValueOf(nil), err
	}

	t := dtoAttr.Type()
	isPointer := t.Kind() == reflect.Pointer
	if isPointer {
		t = t.Elem()
	}

	newInstance := reflect.New(t)

	if ptrIface := newInstance.Interface(); ptrIface != nil {
		if unmarshaler, ok := ptrIface.(interface{ UnmarshalJSONAPIField(data []byte) error }); ok {
			err = unmarshaler.UnmarshalJSONAPIField(bs)
			if err != nil {
				return reflect.ValueOf(nil), err
			}

			if isPointer {
				return newInstance, nil
			}
			return newInstance.Elem(), nil
		}
	}

	return reflect.ValueOf(nil), nil
}

func handleStringSlice(attribute interface{}) (reflect.Value, error) {
	v := reflect.ValueOf(attribute)
	values := make([]string, v.Len())
	for i := 0; i < v.Len(); i++ {
		values[i] = v.Index(i).Interface().(string)
	}

	return reflect.ValueOf(values), nil
}

func handleNullable(
	attribute interface{},
	args []string,
	structField reflect.StructField,
	fieldValue reflect.Value) (reflect.Value, error) {

	if a, ok := attribute.(string); ok && a == "null" {
		return reflect.ValueOf(nil), nil
	}

	innerType := fieldValue.Type().Elem()
	zeroValue := reflect.Zero(innerType)

	attrVal, err := unmarshalAttribute(attribute, args, structField, zeroValue)
	if err != nil {
		return reflect.ValueOf(nil), err
	}

	fieldValue.Set(reflect.MakeMapWithSize(fieldValue.Type(), 1))
	fieldValue.SetMapIndex(reflect.ValueOf(true), attrVal)

	return fieldValue, nil
}

func handleTime(attribute interface{}, args []string, fieldValue reflect.Value) (reflect.Value, error) {
	var isISO8601, isRFC3339, isFmtDate, isFmtTime bool
	v := reflect.ValueOf(attribute)

	// If attribute is nil or null, return a zero value
	if attribute == nil {
		if fieldValue.Kind() == reflect.Pointer {
			return reflect.Zero(fieldValue.Type()), nil
		}
		return reflect.Zero(reflect.TypeOf(time.Time{})), nil
	}

	if len(args) > 2 {
		for _, arg := range args[2:] {
			switch arg {
			case annotationISO8601:
				isISO8601 = true
			case annotationRFC3339:
				isRFC3339 = true
			case annotationFmtDate:
				isFmtDate = true
			case annotationFmtTime:
				isFmtTime = true
			}
		}
	}

	// First, check if attribute is a string
	if v.Kind() == reflect.String {
		dateStr := v.Interface().(string)

		// Empty string means null for time fields
		if dateStr == "" {
			if fieldValue.Kind() == reflect.Pointer {
				return reflect.Zero(fieldValue.Type()), nil
			}
			return reflect.Zero(reflect.TypeOf(time.Time{})), nil
		}

		// Try to parse as ISO8601 if specified
		if isISO8601 {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				if fieldValue.Kind() == reflect.Pointer {
					return reflect.ValueOf(&t), nil
				}
				return reflect.ValueOf(t), nil
			}
		}

		// Try to parse as RFC3339 if specified
		if isRFC3339 {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				if fieldValue.Kind() == reflect.Pointer {
					return reflect.ValueOf(&t), nil
				}
				return reflect.ValueOf(t), nil
			}
		}

		// Try to parse as date-only if specified
		if isFmtDate {
			if t, err := time.Parse(dateOnlyFormat, dateStr); err == nil {
				if fieldValue.Kind() == reflect.Pointer {
					return reflect.ValueOf(&t), nil
				}
				return reflect.ValueOf(t), nil
			} else {
				// If we explicitly requested date format but couldn't parse, return error
				return reflect.ValueOf(time.Time{}), fmt.Errorf("%v: %v (input: %q)", ErrInvalidDateFormat, err, dateStr)
			}
		}

		// Try to parse as time-only if specified
		if isFmtTime {
			// Since we only have time part, add a dummy date part for parsing
			timeStr := "0001-01-01T" + dateStr
			if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
				if fieldValue.Kind() == reflect.Pointer {
					return reflect.ValueOf(&t), nil
				}
				return reflect.ValueOf(t), nil
			} else {
				return reflect.ValueOf(time.Time{}), ErrInvalidTimeFormat
			}
		}

		// If no specific format annotation, try the formats in order
		formats := []string{
			ISO8601TimeFormat,
			time.RFC3339,
			dateOnlyFormat,
		}

		for _, format := range formats {
			if t, err := time.Parse(format, dateStr); err == nil {
				if fieldValue.Kind() == reflect.Pointer {
					return reflect.ValueOf(&t), nil
				}
				return reflect.ValueOf(t), nil
			}
		}
	}

	// For timestamps
	if v.Kind() == reflect.Float64 || v.Kind() == reflect.Int {
		var at int64
		if v.Kind() == reflect.Float64 {
			at = int64(v.Interface().(float64))
		} else {
			at = v.Int()
		}

		t := time.Unix(at, 0)
		if fieldValue.Kind() == reflect.Pointer {
			return reflect.ValueOf(&t), nil
		}
		return reflect.ValueOf(t), nil
	}

	// If we couldn't parse the time in any expected format
	return reflect.ValueOf(time.Time{}), fmt.Errorf("could not parse %v as time", attribute)
}

func handleNumeric(
	attribute interface{},
	fieldType reflect.Type,
	fieldValue reflect.Value) (reflect.Value, error) {
	v := reflect.ValueOf(attribute)
	floatValue := v.Interface().(float64)

	var kind reflect.Kind
	if fieldValue.Kind() == reflect.Pointer {
		kind = fieldType.Elem().Kind()
	} else {
		kind = fieldType.Kind()
	}

	var numericValue reflect.Value

	switch kind {
	case reflect.Int:
		n := int(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Int8:
		n := int8(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Int16:
		n := int16(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Int32:
		n := int32(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Int64:
		n := int64(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Uint:
		n := uint(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Uint8:
		n := uint8(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Uint16:
		n := uint16(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Uint32:
		n := uint32(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Uint64:
		n := uint64(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Float32:
		n := float32(floatValue)
		numericValue = reflect.ValueOf(&n)
	case reflect.Float64:
		n := floatValue
		numericValue = reflect.ValueOf(&n)
	default:
		return reflect.Value{}, ErrUnknownFieldNumberType
	}

	return numericValue, nil
}

func handlePointer(
	attribute interface{},
	fieldType reflect.Type,
	fieldValue reflect.Value,
	structField reflect.StructField) (reflect.Value, error) {
	t := fieldValue.Type()
	var concreteVal reflect.Value

	switch cVal := attribute.(type) {
	case string:
		// Special case for pointer to custom string type
		if t.Elem().Kind() == reflect.String && t.Elem() != reflect.TypeOf("") {
			// Create a new instance of the custom string type
			customStringType := reflect.New(t.Elem()).Elem()
			// Convert the string to the custom type
			customStringType.SetString(cVal)
			// Get address of the custom string type value
			concreteVal = customStringType.Addr()
		} else {
			concreteVal = reflect.ValueOf(&cVal)
		}
	case bool:
		concreteVal = reflect.ValueOf(&cVal)
	case complex64, complex128, uintptr:
		concreteVal = reflect.ValueOf(&cVal)
	case map[string]interface{}:
		var err error
		concreteVal, err = handleStruct(attribute, fieldValue)
		if err != nil {
			return reflect.Value{}, newErrUnsupportedPtrType(
				reflect.ValueOf(attribute), fieldType, structField)
		}
		return concreteVal, err
	default:
		return reflect.Value{}, newErrUnsupportedPtrType(
			reflect.ValueOf(attribute), fieldType, structField)
	}

	if t != concreteVal.Type() {
		return reflect.Value{}, newErrUnsupportedPtrType(
			reflect.ValueOf(attribute), fieldType, structField)
	}

	return concreteVal, nil
}

func handleStruct(
	attribute interface{},
	fieldValue reflect.Value) (reflect.Value, error) {

	var model reflect.Value
	if fieldValue.Kind() == reflect.Pointer {
		model = reflect.New(fieldValue.Type().Elem())
	} else {
		model = reflect.New(fieldValue.Type())
	}

	attributeMap, ok := attribute.(map[string]interface{})
	if !ok {
		return reflect.Value{}, fmt.Errorf("expected map[string]interface{}, got %T", attribute)
	}

	node := &Node{
		Attributes: attributeMap,
	}

	if err := unmarshalNode(node, model, nil); err != nil {
		return reflect.Value{}, err
	}

	return model, nil
}

func handleStructSlice(
	attribute interface{},
	fieldValue reflect.Value) (reflect.Value, error) {
	models := reflect.New(fieldValue.Type()).Elem()
	dataMap := reflect.ValueOf(attribute).Interface().([]interface{})
	for _, data := range dataMap {
		model := reflect.New(fieldValue.Type().Elem()).Elem()

		value, err := handleStruct(data, model)

		if err != nil {
			continue
		}

		models = reflect.Append(models, reflect.Indirect(value))
	}

	return models, nil
}

func handleStructPointerSlice(
	attribute interface{},
	args []string,
	fieldValue reflect.Value) (reflect.Value, error) {

	dataMap := reflect.ValueOf(attribute).Interface().([]interface{})
	models := reflect.New(fieldValue.Type()).Elem()
	for _, data := range dataMap {
		model := reflect.New(fieldValue.Type().Elem()).Elem()
		value, err := handleStruct(data, model)
		if err != nil {
			continue
		}

		models = reflect.Append(models, value)
	}
	return models, nil
}

func handleDuration(attribute interface{}) (reflect.Value, error) {
	v := reflect.ValueOf(attribute)

	switch v.Kind() {
	case reflect.String:
		// Handle duration strings like "1h0s", "5m", "30s", etc.
		durationStr := v.String()
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot parse duration string %q: %v", durationStr, err)
		}
		return reflect.ValueOf(duration), nil

	case reflect.Float64:
		// Handle numeric nanoseconds (JSON numbers are float64 by default)
		nanoseconds := int64(v.Float())
		duration := time.Duration(nanoseconds)
		return reflect.ValueOf(duration), nil

	case reflect.Int64:
		// Handle int64 nanoseconds
		nanoseconds := v.Int()
		duration := time.Duration(nanoseconds)
		return reflect.ValueOf(duration), nil

	case reflect.Int:
		// Handle int nanoseconds
		nanoseconds := v.Int()
		duration := time.Duration(nanoseconds)
		return reflect.ValueOf(duration), nil

	default:
		return reflect.Value{}, fmt.Errorf("cannot convert %T to time.Duration (expected string, float64, int64, or int)", attribute)
	}
}

func handleIntSlice(attribute interface{}) (reflect.Value, error) {
	v := reflect.ValueOf(attribute)
	if v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("expected slice for []int, got %T", attribute)
	}

	ints := make([]int, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemInterface := elem.Interface()

		switch val := elemInterface.(type) {
		case float64:
			ints[i] = int(val)
		case int:
			ints[i] = val
		case int64:
			ints[i] = int(val)
		default:
			return reflect.Value{}, fmt.Errorf("cannot convert element %T to int", elemInterface)
		}
	}

	return reflect.ValueOf(ints), nil
}

func handleStringMap(attribute interface{}) (reflect.Value, error) {
	attrMap, ok := attribute.(map[string]interface{})
	if !ok {
		return reflect.Value{}, fmt.Errorf("expected map[string]interface{} for map[string]string, got %T", attribute)
	}

	result := make(map[string]string)
	for key, val := range attrMap {
		if strVal, ok := val.(string); ok {
			result[key] = strVal
		} else {
			return reflect.Value{}, fmt.Errorf("map value must be string, got %T", val)
		}
	}

	return reflect.ValueOf(result), nil
}

func handleStringSliceMap(attribute interface{}) (reflect.Value, error) {
	attrMap, ok := attribute.(map[string]interface{})
	if !ok {
		return reflect.Value{}, fmt.Errorf("expected map[string]interface{} for map[string][]string, got %T", attribute)
	}

	result := make(map[string][]string)
	for key, val := range attrMap {
		if slice, ok := val.([]interface{}); ok {
			strSlice := make([]string, len(slice))
			for i, item := range slice {
				if strItem, ok := item.(string); ok {
					strSlice[i] = strItem
				} else {
					return reflect.Value{}, fmt.Errorf("slice element must be string, got %T", item)
				}
			}
			result[key] = strSlice
		} else {
			return reflect.Value{}, fmt.Errorf("map value must be []interface{}, got %T", val)
		}
	}

	return reflect.ValueOf(result), nil
}

func handleInterfaceMap(attribute interface{}) (reflect.Value, error) {
	attrMap, ok := attribute.(map[string]interface{})
	if !ok {
		return reflect.Value{}, fmt.Errorf("expected map[string]interface{}, got %T", attribute)
	}

	// For map[string]interface{}, we can directly use the attribute as-is
	return reflect.ValueOf(attrMap), nil
}
