// Package scheduler provides three orthogonal in-process scheduling
// primitives: EventMap (lightweight time + group + phase event queue),
// TaskScheduler (callback-based delayed/repeating tasks), and Buckets
// (frequency-bucketed scheduling by relevance). All share a Clock
// interface for time-source injection in tests.
package scheduler
