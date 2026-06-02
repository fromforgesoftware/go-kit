// Package writebehind is a typed non-blocking persistence queue: the
// producer (simulation loop, tick goroutine, request handler) Pushes
// operations into a bounded channel and never blocks on the slow
// downstream; a dedicated draining goroutine batches them and calls
// the consumer-supplied Flusher.
//
// Backpressure policies: DropOldest / DropNewest / Block /
// CoalesceByKey. Defaults to DropOldest, the right call for game
// shard write-behind (the freshest position wins).
package writebehind
