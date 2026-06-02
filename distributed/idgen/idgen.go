// Package idgen provides distributed ID generation without requiring central coordination.
//
// IDs are 64-bit integers with the following structure:
//   - 41 bits: Timestamp in milliseconds since epoch
//   - 10 bits: Region ID (supports 1024 regions)
//   - 12 bits: Sequence number (supports 4096 IDs per millisecond per region)
//
// This design ensures globally unique IDs across multiple regions while maintaining
// the benefits of integer primary keys (smaller storage, faster indexes, compatibility
// with existing protocols).
//
// Example usage:
//
//	generator := idgen.NewGenerator(1) // Region ID = 1
//	characterID := generator.NextID()   // 7123456789012345678
package idgen

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrClockBackward is returned by NextIDErr when the system clock has moved
// backward relative to the last observed timestamp. Generating an ID in this
// situation could yield duplicate IDs, so the generator refuses instead.
var ErrClockBackward = errors.New("idgen: clock moved backward, refusing to generate ID")

const (
	// Bit allocations
	timestampBits = 41
	regionBits    = 10
	sequenceBits  = 12

	// Max values
	maxRegionID = (1 << regionBits) - 1   // 1023
	maxSequence = (1 << sequenceBits) - 1 // 4095

	// Bit shifts
	regionShift    = sequenceBits
	timestampShift = sequenceBits + regionBits
)

// Generator generates globally unique 64-bit IDs for a specific region.
// It is safe for concurrent use.
type Generator struct {
	regionID int64
	sequence int64
	lastTime int64
	mu       sync.Mutex

	// nowFn returns the current time in milliseconds since epoch. It exists
	// as a field so tests can simulate clock movement; production code always
	// uses time.Now().UnixMilli.
	nowFn func() int64
}

// NewGenerator creates a new ID generator for the specified region.
// Region ID must be between 0 and 1023 (inclusive).
func NewGenerator(regionID int64) *Generator {
	if regionID < 0 || regionID > maxRegionID {
		panic(fmt.Sprintf("region ID must be between 0 and %d, got %d", maxRegionID, regionID))
	}

	return &Generator{
		regionID: regionID,
		sequence: 0,
		lastTime: 0,
		nowFn:    func() int64 { return time.Now().UnixMilli() },
	}
}

// NextIDErr generates the next unique ID.
//
// It returns ErrClockBackward if the system clock has moved backward relative
// to the last observed timestamp, as continuing could produce duplicate IDs.
// This is the error-returning counterpart of NextID and is the preferred entry
// point for callers that can handle a transient clock anomaly at runtime
// instead of crashing the process.
func (g *Generator) NextIDErr() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()

	// Clock moved backward check
	if now < g.lastTime {
		return 0, fmt.Errorf("%w (last=%d, now=%d)", ErrClockBackward, g.lastTime, now)
	}

	if now == g.lastTime {
		// Same millisecond, increment sequence
		g.sequence++
		if g.sequence > maxSequence {
			// Sequence overflow, wait for next millisecond
			for now <= g.lastTime {
				now = g.now()
			}
			g.sequence = 0
		}
	} else {
		// New millisecond, reset sequence
		g.sequence = 0
	}

	g.lastTime = now

	// Pack into 64 bits: [timestamp][regionID][sequence]
	id := (now << timestampShift) | (g.regionID << regionShift) | g.sequence
	return id, nil
}

// NextID generates the next unique ID.
//
// Deprecated: NextID panics if the system clock moves backward, which can crash
// a service at runtime. Use NextIDErr instead, which returns ErrClockBackward
// so the caller can decide how to handle the anomaly. NextID is retained for
// backward compatibility and will be removed in a future major release.
func (g *Generator) NextID() int64 {
	id, err := g.NextIDErr()
	if err != nil {
		panic(err.Error())
	}
	return id
}

// now returns the current time in milliseconds. It falls back to time.Now for
// generators constructed without NewGenerator (e.g. a zero-value Generator),
// preserving behaviour for any such existing callers.
func (g *Generator) now() int64 {
	if g.nowFn != nil {
		return g.nowFn()
	}
	return time.Now().UnixMilli()
}

// RegionID returns the region ID this generator was created with.
func (g *Generator) RegionID() int64 {
	return g.regionID
}

// ParseID extracts the timestamp, region ID, and sequence from a generated ID.
func ParseID(id int64) (timestamp, regionID, sequence int64) {
	timestamp = id >> timestampShift
	regionID = (id >> regionShift) & maxRegionID
	sequence = id & maxSequence
	return
}
