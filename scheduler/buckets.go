package scheduler

import "time"

// Bucket declares a tick frequency for items bucketed into it.
type Bucket struct {
	Name     string
	Interval time.Duration
}

// Buckets groups items by relevance and ticks each group at a different
// frequency. Useful for game AI / observability polling where "hot" items
// need fast updates and "cold" ones don't.
type Buckets[T any] struct {
	buckets  []bucketState[T]
	byID     map[uint64]int // id → bucket index
	bucketer func(T) string
}

type bucketState[T any] struct {
	name     string
	interval time.Duration
	items    map[uint64]T
	lastTick time.Time
}

// NewBuckets constructs a Buckets given the bucket definitions and a
// bucketer that maps each item to a bucket name. If the bucketer returns
// a name not in the configured set, the item is silently dropped.
func NewBuckets[T any](buckets []Bucket, bucketer func(T) string) *Buckets[T] {
	b := &Buckets[T]{
		buckets:  make([]bucketState[T], len(buckets)),
		byID:     make(map[uint64]int),
		bucketer: bucketer,
	}
	for i, def := range buckets {
		b.buckets[i] = bucketState[T]{
			name:     def.Name,
			interval: def.Interval,
			items:    make(map[uint64]T),
		}
	}
	return b
}

// Add inserts or re-buckets an item under its bucketer-derived bucket.
func (b *Buckets[T]) Add(id uint64, item T) {
	name := b.bucketer(item)
	target := -1
	for i := range b.buckets {
		if b.buckets[i].name == name {
			target = i
			break
		}
	}
	if target < 0 {
		return
	}
	if existing, ok := b.byID[id]; ok && existing != target {
		delete(b.buckets[existing].items, id)
	}
	b.buckets[target].items[id] = item
	b.byID[id] = target
}

// Remove deletes an item.
func (b *Buckets[T]) Remove(id uint64) {
	idx, ok := b.byID[id]
	if !ok {
		return
	}
	delete(b.buckets[idx].items, id)
	delete(b.byID, id)
}

// Tick invokes fn on every item whose bucket interval has elapsed since
// its last tick.
func (b *Buckets[T]) Tick(now time.Time, fn func(item T)) {
	for i := range b.buckets {
		if now.Sub(b.buckets[i].lastTick) < b.buckets[i].interval {
			continue
		}
		b.buckets[i].lastTick = now
		for _, item := range b.buckets[i].items {
			fn(item)
		}
	}
}
