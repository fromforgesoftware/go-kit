package inheritance

import "encoding/json"

// Optional is a "set or unset" wrapper used in Layer structs.
type Optional[T any] struct {
	set   bool
	value T
}

// Set wraps a value as "explicitly set".
func Set[T any](v T) Optional[T] { return Optional[T]{set: true, value: v} }

// Unset returns the zero Optional[T] representing "no value provided".
func Unset[T any]() Optional[T] { return Optional[T]{} }

// Get returns the value and whether it was set.
func (o Optional[T]) Get() (T, bool) { return o.value, o.set }

// OrDefault returns the value if set, otherwise d.
func (o Optional[T]) OrDefault(d T) T {
	if o.set {
		return o.value
	}
	return d
}

// IsSet reports whether the value was provided.
func (o Optional[T]) IsSet() bool { return o.set }

// MarshalJSON emits the underlying value when set, JSON null when
// unset, so round-trips don't accidentally promote unset → zero.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.set {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON treats JSON null as Unset; any other value as Set.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.set = false
		return nil
	}
	if err := json.Unmarshal(data, &o.value); err != nil {
		return err
	}
	o.set = true
	return nil
}
