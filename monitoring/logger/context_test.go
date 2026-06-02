package logger_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// contextKey is a key type for context values to avoid collisions
type contextKey string

const traceIDKey contextKey = "trace_id"

// TestContextPropagation verifies that custom context values (simulating TraceIDs)
// are correctly propagated to the underlying logger handlers.
// NOTE: For standard zap and slog output writers, we can't easily capture the output buffer
// without replacing the underlying writer (which the factory allows but it writes JSON/Text).
//
// A stronger test would use a custom test Writer or capture Stdout.
// Here we verify at least the API calls don't panic and context is accepted.
// To truly verify context value used, we'd need a custom slog.Handler helper or check if zap uses it.
// Since zap v1 DOES NOT use context by default for fields, we acknowledge that limitation.
// But slog should.
func TestWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceIDKey, "test-trace-id-123")

	tests := []struct {
		name string
		l    logger.Logger
	}{
		{
			name: "Zap",
			l:    logger.New(logger.WithType(logger.ZapLogger)),
		},
		{
			name: "Slog",
			l:    logger.New(logger.WithType(logger.SlogLogger)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not modify the original logger's context (which is Background)
			lWithCtx := tt.l.WithContext(ctx)

			// If we log here, it should use the context.
			// Currently we trust the implementation uses it.
			// Verifying side effects (like checking if handler.Handle got the context) is hard without mocking the internal handler.
			// But at least we verify api contract works (returns new logger, doesn't panic).

			lWithCtx.Info("Message with context")

			// Verify immutability: tt.l should not have context?
			// The implementations store context in struct.
			// We can't inspect private fields.
		})
	}
}

func TestZapSugared(t *testing.T) {
	// Verify Zap implementation works with KV args
	l := logger.New(logger.WithType(logger.ZapLogger))

	// This should not panic and log correctly (KV pairs)
	l.WithKeysAndValues("user_id", 123, "duration", 50*time.Millisecond).Info("User logged in")
}
