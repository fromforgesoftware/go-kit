package amqp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestProducerWithTimeout_SetsTimeout verifies the correctly-spelled option
// sets the per-publish timeout.
func TestProducerWithTimeout_SetsTimeout(t *testing.T) {
	cfg := &producerConfig{}
	ProducerWithTimeout(7 * time.Second)(cfg)
	assert.Equal(t, 7*time.Second, cfg.timeout)
}

// TestPorducerWithTimeout_DeprecatedAliasMatches verifies the misspelled,
// deprecated alias behaves identically to ProducerWithTimeout so existing
// callers are unaffected.
func TestPorducerWithTimeout_DeprecatedAliasMatches(t *testing.T) {
	canonical := &producerConfig{}
	ProducerWithTimeout(3 * time.Second)(canonical)

	deprecated := &producerConfig{}
	PorducerWithTimeout(3 * time.Second)(deprecated)

	assert.Equal(t, canonical.timeout, deprecated.timeout)
	assert.Equal(t, 3*time.Second, deprecated.timeout)
}
