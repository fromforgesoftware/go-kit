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
	"fmt"
	"sync"
	"time"
)

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
	}
}

// NextID generates the next unique ID.
// It panics if the system clock moves backward, as this could cause duplicate IDs.
func (g *Generator) NextID() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	// Clock moved backward check
	if now < g.lastTime {
		panic(fmt.Sprintf("clock moved backward: refusing to generate ID (last=%d, now=%d)", g.lastTime, now))
	}

	if now == g.lastTime {
		// Same millisecond, increment sequence
		g.sequence++
		if g.sequence > maxSequence {
			// Sequence overflow, wait for next millisecond
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
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
	return id
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
