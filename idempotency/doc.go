// Package idempotency makes value-bearing RPCs safe to retry: on
// first request the handler runs and the response is stored under
// the client-supplied key; on replay with the same key + same request
// hash the cached response is returned without re-invoking the
// handler.
//
// Concurrent same-key requests block on the in-flight Reserve and
// reuse the eventual cached response, so duplicate writes from racing
// retries collapse to a single handler invocation.
//
// Backends behind the Store interface: in-memory (tests), Redis
// (production, fast), Postgres (production, durable).
package idempotency
