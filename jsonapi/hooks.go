package jsonapi

// MarshalHook defines the interface for objects that provide BeforeMarshal hooks
type MarshalHook interface {
	// BeforeMarshal is called before an object is marshalled to JSON
	// If it returns an error, marshalling will be aborted
	BeforeMarshal() error
}

// UnmarshalHook defines the interface for objects that provide AfterUnmarshal hooks
type UnmarshalHook interface {
	// AfterUnmarshal is called after an object is unmarshalled from JSON
	// If it returns an error, unmarshalling will be aborted
	AfterUnmarshal() error
}
