// Package extractor is a parallel tile-based ETL framework: split a
// large input into units, dispatch across a worker pool, track
// progress, and skip already-completed units on resume.
//
// Three composable layers:
//   - Worker[T,R]  — bounded-concurrency worker pool with cancellation
//     and per-error recovery policy.
//   - Progress     — atomic counters + JSON heartbeat snapshots.
//   - Checkpoint   — completed-unit ledger (file or DB) for resumption.
//
// A Pipeline[T,R] composes all three; each layer is independently
// usable.
package extractor
