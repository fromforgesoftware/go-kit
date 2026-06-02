package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/retry"
	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	tests := []struct {
		name           string
		setupOperation func() func() error
		options        []retry.Option
		expectError    bool
		expectedCalls  int
		minDuration    time.Duration
	}{
		{
			name: "success_on_first_attempt",
			setupOperation: func() func() error {
				callCount := 0
				return func() error {
					callCount++
					return nil
				}
			},
			options:       []retry.Option{},
			expectError:   false,
			expectedCalls: 1,
		},
		{
			name: "success_after_failures",
			setupOperation: func() func() error {
				callCount := 0
				return func() error {
					callCount++
					if callCount < 3 {
						return errors.New("temporary error")
					}
					return nil
				}
			},
			options: []retry.Option{
				retry.WithConstantPolicy(10 * time.Millisecond),
				retry.WithMaxRetries(5),
			},
			expectError:   false,
			expectedCalls: 3,
			minDuration:   20 * time.Millisecond, // 2 retries * 10ms
		},
		{
			name: "max_retries_exceeded",
			setupOperation: func() func() error {
				callCount := 0
				return func() error {
					callCount++
					return errors.New("persistent error")
				}
			},
			options: []retry.Option{
				retry.WithConstantPolicy(10 * time.Millisecond),
				retry.WithMaxRetries(2),
			},
			expectError:   true,
			expectedCalls: 2,
		},
		{
			name: "zero_max_retries",
			setupOperation: func() func() error {
				callCount := 0
				return func() error {
					callCount++
					return errors.New("should not be called")
				}
			},
			options: []retry.Option{
				retry.WithMaxRetries(0),
			},
			expectError:   true,
			expectedCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			operation := func() error {
				callCount++
				switch tt.name {
				case "success_after_failures":
					if callCount < 3 {
						return errors.New("temporary error")
					}
					return nil
				case "max_retries_exceeded":
					return errors.New("persistent error")
				case "zero_max_retries":
					return errors.New("should not be called")
				default: // success_on_first_attempt
					return nil
				}
			}

			start := time.Now()
			err := retry.Retry(operation, tt.options...)
			duration := time.Since(start)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedCalls, callCount)

			if tt.minDuration > 0 {
				assert.GreaterOrEqual(t, duration, tt.minDuration)
			}
		})
	}
}

func TestRetryWithContext(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		operationDelay time.Duration
		expectError    bool
	}{
		{
			name:           "context_timeout",
			timeout:        50 * time.Millisecond,
			operationDelay: 30 * time.Millisecond,
			expectError:    true,
		},
		{
			name:           "success_before_timeout",
			timeout:        100 * time.Millisecond,
			operationDelay: 10 * time.Millisecond,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			callCount := 0
			operation := func() error {
				callCount++
				if tt.expectError {
					time.Sleep(tt.operationDelay)
					return errors.New("slow error")
				}
				if callCount < 2 {
					return errors.New("temporary error")
				}
				return nil
			}

			err := retry.RetryWithContext(ctx, operation, retry.WithConstantPolicy(20*time.Millisecond))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRetryWithData(t *testing.T) {
	tests := []struct {
		name           string
		expectedResult string
		expectedCalls  int
		expectError    bool
	}{
		{
			name:           "success_with_data",
			expectedResult: "success",
			expectedCalls:  2,
			expectError:    false,
		},
		{
			name:           "failure_with_data",
			expectedResult: "",
			expectedCalls:  2,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			operation := func() (string, error) {
				callCount++
				if tt.expectError {
					return "", errors.New("persistent error")
				}
				if callCount < 2 {
					return "", errors.New("temporary error")
				}
				return "success", nil
			}

			result, err := retry.RetryWithData(operation,
				retry.WithConstantPolicy(10*time.Millisecond),
				retry.WithMaxRetries(2))

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
			assert.Equal(t, tt.expectedCalls, callCount)
		})
	}
}

func TestRetryWithDataAndContext(t *testing.T) {
	t.Run("context_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		callCount := 0
		operation := func() (int, error) {
			callCount++
			// Always fail so we keep retrying until context cancels
			time.Sleep(15 * time.Millisecond) // Sleep to consume time
			return 0, errors.New("always fails")
		}

		result, err := retry.RetryWithDataAndContext(ctx, operation,
			retry.WithConstantPolicy(5*time.Millisecond))

		// Should have an error due to context cancellation
		assert.Error(t, err)
		assert.Equal(t, 0, result)
		// Should have made at least one call
		assert.GreaterOrEqual(t, callCount, 1)
	})

	t.Run("success_before_timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		callCount := 0
		operation := func() (int, error) {
			callCount++
			if callCount == 1 {
				return 0, errors.New("first error")
			}
			return 42, nil
		}

		result, err := retry.RetryWithDataAndContext(ctx, operation,
			retry.WithConstantPolicy(10*time.Millisecond))

		assert.NoError(t, err)
		assert.Equal(t, 42, result)
		assert.Equal(t, 2, callCount)
	})
}

func TestPolicyOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     []retry.Option
		minDuration time.Duration
		maxDuration time.Duration
	}{
		{
			name: "constant_policy",
			options: []retry.Option{
				retry.WithConstantPolicy(50 * time.Millisecond),
				retry.WithMaxRetries(3),
			},
			minDuration: 100 * time.Millisecond, // 2 retries * 50ms
			maxDuration: 200 * time.Millisecond, // Allow some variance
		},
		{
			name: "exponential_policy",
			options: []retry.Option{
				retry.WithExponentialPolicy(),
				retry.WithInitialInterval(10 * time.Millisecond),
				retry.WithMaxRetries(3),
			},
			minDuration: 10 * time.Millisecond,  // At least initial interval
			maxDuration: 500 * time.Millisecond, // Allow for exponential growth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			operation := func() error {
				callCount++
				if callCount < 3 {
					return errors.New("temporary error")
				}
				return nil
			}

			start := time.Now()
			err := retry.Retry(operation, tt.options...)
			duration := time.Since(start)

			assert.NoError(t, err)
			assert.Equal(t, 3, callCount)
			assert.GreaterOrEqual(t, duration, tt.minDuration)
			assert.LessOrEqual(t, duration, tt.maxDuration)
		})
	}
}
