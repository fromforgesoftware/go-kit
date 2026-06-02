package httpclient_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/httpclient"
)

func TestBreaker_StartsClosed(t *testing.T) {
	b := httpclient.NewBreaker()
	allow, state := b.Allow()
	assert.True(t, allow)
	assert.Equal(t, httpclient.StateClosed, state)
}

func TestBreaker_TripsAfterThresholdFailures(t *testing.T) {
	b := httpclient.NewBreaker(httpclient.WithThreshold(3))
	for i := 0; i < 3; i++ {
		b.RecordFailure()
	}
	allow, state := b.Allow()
	assert.False(t, allow)
	assert.Equal(t, httpclient.StateOpen, state)
}

func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := httpclient.NewBreaker(httpclient.WithThreshold(3))
	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess()
	b.RecordFailure()
	allow, _ := b.Allow()
	assert.True(t, allow, "success between failures should reset the counter")
}

func TestBreaker_HalfOpenAfterCooldown(t *testing.T) {
	b := httpclient.NewBreaker(httpclient.WithThreshold(1), httpclient.WithCooldown(50*time.Millisecond))
	clock := time.Now()
	b.SetClock(func() time.Time { return clock })

	b.RecordFailure()
	allow, state := b.Allow()
	assert.False(t, allow)
	assert.Equal(t, httpclient.StateOpen, state)

	clock = clock.Add(60 * time.Millisecond)
	allow, state = b.Allow()
	assert.True(t, allow, "after cooldown, one probe is allowed")
	assert.Equal(t, httpclient.StateHalfOpen, state)
}

func TestBreaker_HalfOpenFailureReopens(t *testing.T) {
	b := httpclient.NewBreaker(httpclient.WithThreshold(1), httpclient.WithCooldown(10*time.Millisecond))
	clock := time.Now()
	b.SetClock(func() time.Time { return clock })

	b.RecordFailure()
	clock = clock.Add(20 * time.Millisecond)
	b.Allow() // moves to half-open + consumes the probe
	b.RecordFailure()

	assert.Equal(t, httpclient.StateOpen, b.State())
}

func TestBreaker_HalfOpenSuccessClosesAgain(t *testing.T) {
	b := httpclient.NewBreaker(httpclient.WithThreshold(1), httpclient.WithCooldown(10*time.Millisecond))
	clock := time.Now()
	b.SetClock(func() time.Time { return clock })

	b.RecordFailure()
	clock = clock.Add(20 * time.Millisecond)
	b.Allow()
	b.RecordSuccess()

	assert.Equal(t, httpclient.StateClosed, b.State())
}
