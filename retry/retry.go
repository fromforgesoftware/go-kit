package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
)

// PolicyType denotes if the back off delay should be constant or exponential.
type PolicyType int

const (
	// PolicyConstant is a backoff policy that always returns the same backoff delay.
	PolicyConstant PolicyType = iota
	// PolicyExponential is a backoff implementation that increases the backoff period
	// for each retry attempt using a randomization function that grows exponentially.
	PolicyExponential
)

// Option represents a functional option for configuring retry behavior.
type Option func(*config)

// config encapsulates the back off policy configuration.
type config struct {
	policy PolicyType

	// Constant back off
	duration time.Duration

	// Exponential back off
	initialInterval     time.Duration
	randomizationFactor float32
	multiplier          float32
	maxInterval         time.Duration
	maxElapsedTime      time.Duration

	// Additional options
	maxRetries int64
}

// WithConstantPolicy configures constant backoff policy with the specified duration.
func WithConstantPolicy(duration time.Duration) Option {
	return func(c *config) {
		c.policy = PolicyConstant
		c.duration = duration
	}
}

// WithExponentialPolicy configures exponential backoff policy.
func WithExponentialPolicy() Option {
	return func(c *config) {
		c.policy = PolicyExponential
	}
}

// WithInitialInterval sets the initial interval for exponential backoff.
func WithInitialInterval(interval time.Duration) Option {
	return func(c *config) {
		c.initialInterval = interval
	}
}

// WithRandomizationFactor sets the randomization factor for exponential backoff.
func WithRandomizationFactor(factor float32) Option {
	return func(c *config) {
		c.randomizationFactor = factor
	}
}

// WithMultiplier sets the multiplier for exponential backoff.
func WithMultiplier(multiplier float32) Option {
	return func(c *config) {
		c.multiplier = multiplier
	}
}

// WithMaxInterval sets the maximum interval for exponential backoff.
func WithMaxInterval(interval time.Duration) Option {
	return func(c *config) {
		c.maxInterval = interval
	}
}

// WithMaxElapsedTime sets the maximum elapsed time for exponential backoff.
func WithMaxElapsedTime(elapsedTime time.Duration) Option {
	return func(c *config) {
		c.maxElapsedTime = elapsedTime
	}
}

// WithMaxRetries sets the maximum number of tries (note: misnamed — it's
// total attempts, not retries). Kept for backwards compatibility; prefer
// WithMaxAttempts in new code.
func WithMaxRetries(maxRetries int64) Option {
	return func(c *config) {
		c.maxRetries = maxRetries
	}
}

// WithMaxAttempts caps the total number of attempts. The natural-meaning
// replacement for WithMaxRetries: n=1 means no retry (single attempt),
// n=3 retries up to twice. Values below 1 are clamped to 1.
func WithMaxAttempts(n int) Option {
	if n < 1 {
		n = 1
	}
	return func(c *config) {
		c.maxRetries = int64(n)
	}
}

// defaultOptions returns the default configuration options with exponential backoff.
func defaultOptions() []Option {
	return []Option{
		WithExponentialPolicy(),
		WithInitialInterval(backoff.DefaultInitialInterval),
		WithRandomizationFactor(backoff.DefaultRandomizationFactor),
		WithMultiplier(backoff.DefaultMultiplier),
		WithMaxInterval(backoff.DefaultMaxInterval),
		WithMaxRetries(-1),
	}
}

// BackOff returns a BackOff instance based on the current configuration.

// BackOff returns a BackOff instance based on the current configuration.
// Additional options can be provided to override specific settings.
// The instance will not stop due to context cancellation. To support
// cancellation (recommended), use `BackOffWithContext`.
//
// Since the underlying backoff implementations are not always thread safe,
// `BackOff` or `BackOffWithContext` should be called each time
// a retry operation is performed.
func (c *config) BackOff(opts ...Option) backoff.BackOff {
	// Apply additional options to a copy of the config
	config := *c
	for _, opt := range opts {
		opt(&config)
	}

	var b backoff.BackOff
	switch config.policy {
	case PolicyConstant:
		b = backoff.NewConstantBackOff(config.duration)
	case PolicyExponential:
		eb := backoff.NewExponentialBackOff()
		eb.InitialInterval = config.initialInterval
		eb.RandomizationFactor = float64(config.randomizationFactor)
		eb.Multiplier = float64(config.multiplier)
		eb.MaxInterval = config.maxInterval
		// In v5, MaxElapsedTime is handled via RetryOption, not on the BackOff
		b = eb
	}

	return b
}

// BackOffWithContext returns a BackOff instance based on the current configuration.
// Additional options can be provided to override specific settings.
// The provided context is used to cancel retries if it is canceled.
//
// Since the underlying backoff implementations are not always thread safe,
// `BackOff` or `BackOffWithContext` should be called each time
// a retry operation is performed.
func (c *config) BackOffWithContext(ctx context.Context, opts ...Option) backoff.BackOff {
	// In v5, context is not applied to BackOff directly
	return c.BackOff(opts...)
}

// Retry executes operation repeatedly until it succeeds or the backoff stops.
// In v5, this uses the new generic Retry function.
func Retry(operation func() error, opts ...Option) error {
	_, err := RetryWithData(func() (struct{}, error) {
		return struct{}{}, operation()
	}, opts...)
	return err
}

// RetryWithContext executes operation repeatedly with context until it succeeds or the backoff stops.
func RetryWithContext(ctx context.Context, operation func() error, opts ...Option) error {
	_, err := RetryWithDataAndContext(ctx, func() (struct{}, error) {
		return struct{}{}, operation()
	}, opts...)
	return err
}

// RetryWithData executes operation repeatedly until it succeeds or the backoff stops.
// Returns the result of the operation and any error.
func RetryWithData[T any](operation func() (T, error), opts ...Option) (T, error) {
	return RetryWithDataAndContext(context.Background(), operation, opts...)
}

// RetryWithDataAndContext executes operation repeatedly with context until it succeeds or the backoff stops.
// Returns the result of the operation and any error.
func RetryWithDataAndContext[T any](ctx context.Context, operation func() (T, error), opts ...Option) (T, error) {
	config := config{}
	for _, opt := range append(defaultOptions(), opts...) {
		opt(&config)
	}

	var retryOpts []backoff.RetryOption
	retryOpts = append(retryOpts, backoff.WithBackOff(config.BackOff()))

	if config.maxElapsedTime > 0 {
		retryOpts = append(retryOpts, backoff.WithMaxElapsedTime(config.maxElapsedTime))
	}

	if config.maxRetries >= 0 {
		if config.maxRetries == 0 {
			var zero T
			return zero, fmt.Errorf("max retries set to 0")
		}
		retryOpts = append(retryOpts, backoff.WithMaxTries(uint(config.maxRetries)))
	}

	return backoff.Retry(ctx, operation, retryOpts...)
}
