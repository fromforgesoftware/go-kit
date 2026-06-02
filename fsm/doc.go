// Package fsm provides a generic finite state machine over typed State
// and Event values. Declarative configuration (Spec) over a fluent
// builder — config is a value, easier to introspect, validate, and test.
//
// Concurrency-safe by default via a single internal mutex; consumers
// needing lock-free single-writer semantics (tick loops) can drive the
// machine from a single goroutine and treat it as plain state.
package fsm
