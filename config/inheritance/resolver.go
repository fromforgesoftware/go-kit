package inheritance

import (
	"fmt"
	"reflect"
	"strings"
)

// Layer is one priority level in a hierarchical config. Higher-index
// layers (added later via AddLayer) override lower ones.
type Layer[T any] struct {
	Name string
	Data T
}

// ResolutionError is one structured per-field problem from Resolve.
type ResolutionError struct {
	Field  string
	Reason string
}

// Error implements the error interface.
func (e ResolutionError) Error() string {
	return fmt.Sprintf("inheritance: field %q: %s", e.Field, e.Reason)
}

// Resolver merges layers of T into a final T.
type Resolver[T any] struct {
	layers []Layer[T]
}

// NewResolver constructs an empty Resolver.
func NewResolver[T any]() *Resolver[T] { return &Resolver[T]{} }

// AddLayer appends a layer. Later AddLayer calls have higher priority.
func (r *Resolver[T]) AddLayer(l Layer[T]) { r.layers = append(r.layers, l) }

// Resolve produces a fully-realised T plus any per-field errors
// (required fields unset, type mismatches).
func (r *Resolver[T]) Resolve() (T, []ResolutionError) {
	var out T
	var errs []ResolutionError
	outVal := reflect.ValueOf(&out).Elem()
	outType := outVal.Type()
	if outType.Kind() != reflect.Struct {
		errs = append(errs, ResolutionError{Field: "_root", Reason: "T must be a struct"})
		return out, errs
	}

	for i := 0; i < outType.NumField(); i++ {
		field := outType.Field(i)
		if !field.IsExported() {
			continue
		}
		name, mode, required := parseTag(field)

		// Find highest-priority layer that set this field.
		var (
			setSomewhere bool
			merged       any
		)
		for li := len(r.layers) - 1; li >= 0; li-- {
			lv := reflect.ValueOf(r.layers[li].Data)
			f := lv.Field(i)
			val, ok := optionalValue(f)
			if !ok {
				continue
			}
			if !setSomewhere {
				merged = val
				setSomewhere = true
				if mode == mergeOverride {
					break // highest wins; we walked top-down so this is highest
				}
				continue
			}
			// Lower-priority layer — merge underneath the existing merged value.
			merged = mergeValues(merged, val, mode)
		}

		if !setSomewhere {
			if required {
				errs = append(errs, ResolutionError{Field: name, Reason: "required but unset in every layer"})
			}
			continue
		}
		// Write the merged Optional[T] value back into out.
		assignOptional(outVal.Field(i), merged)
	}
	return out, errs
}

type mergeMode uint8

const (
	mergeOverride mergeMode = iota
	mergeAppend
	mergeUnion
)

func parseTag(f reflect.StructField) (name string, mode mergeMode, required bool) {
	tag := f.Tag.Get("cfg")
	name = f.Name
	mode = mergeOverride
	if tag == "" {
		return
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		name = parts[0]
	}
	for _, opt := range parts[1:] {
		switch opt {
		case "required":
			required = true
		case "merge=append":
			mode = mergeAppend
		case "merge=union":
			mode = mergeUnion
		}
	}
	return
}

// optionalValue extracts the underlying value from an Optional[T] field.
// Returns (zero, false) when the field is not Optional or is Unset.
// Bypasses reflect's unexported-field guard via reflect.NewAt; safe
// because Optional lives in this package and we control its layout.
func optionalValue(v reflect.Value) (any, bool) {
	if v.Kind() != reflect.Struct {
		return nil, false
	}
	if !v.CanAddr() {
		// Take an addressable copy so FieldByName().UnsafeAddr() works.
		copy := reflect.New(v.Type()).Elem()
		copy.Set(v)
		v = copy
	}
	setField := v.FieldByName("set")
	if !setField.IsValid() || setField.Kind() != reflect.Bool {
		return nil, false
	}
	setReadable := reflect.NewAt(setField.Type(), unsafePointer(setField)).Elem()
	if !setReadable.Bool() {
		return nil, false
	}
	valField := v.FieldByName("value")
	if !valField.IsValid() {
		return nil, false
	}
	valReadable := reflect.NewAt(valField.Type(), unsafePointer(valField)).Elem()
	return valReadable.Interface(), true
}

// assignOptional writes `value` into the Optional[T] field at `dst`.
func assignOptional(dst reflect.Value, value any) {
	if dst.Kind() != reflect.Struct {
		return
	}
	setField := dst.FieldByName("set")
	valField := dst.FieldByName("value")
	if !setField.IsValid() || !valField.IsValid() {
		return
	}
	// reflect.Value.Set on an unexported field requires UnsafePointer
	// gymnastics in normal code; the Optional type lives in the same
	// package so we can use reflect.NewAt to bypass the export check.
	setPtr := reflect.NewAt(setField.Type(), unsafePointer(setField)).Elem()
	valPtr := reflect.NewAt(valField.Type(), unsafePointer(valField)).Elem()
	setPtr.SetBool(true)
	valPtr.Set(reflect.ValueOf(value).Convert(valField.Type()))
}

func mergeValues(higher, lower any, mode mergeMode) any {
	switch mode {
	case mergeAppend:
		return appendMerge(higher, lower)
	case mergeUnion:
		return unionMerge(higher, lower)
	default:
		return higher
	}
}

func appendMerge(higher, lower any) any {
	h := reflect.ValueOf(higher)
	l := reflect.ValueOf(lower)
	if h.Kind() != reflect.Slice || l.Kind() != reflect.Slice {
		return higher
	}
	out := reflect.MakeSlice(h.Type(), 0, h.Len()+l.Len())
	out = reflect.AppendSlice(out, l)
	out = reflect.AppendSlice(out, h)
	return out.Interface()
}

func unionMerge(higher, lower any) any {
	h := reflect.ValueOf(higher)
	l := reflect.ValueOf(lower)
	if h.Kind() != reflect.Map || l.Kind() != reflect.Map {
		return higher
	}
	out := reflect.MakeMapWithSize(h.Type(), h.Len()+l.Len())
	for _, k := range l.MapKeys() {
		out.SetMapIndex(k, l.MapIndex(k))
	}
	for _, k := range h.MapKeys() {
		out.SetMapIndex(k, h.MapIndex(k))
	}
	return out.Interface()
}
