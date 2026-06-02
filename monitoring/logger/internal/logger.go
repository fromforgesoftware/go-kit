package internal

import "context"

// Logger is the interface for internal logger implementations to avoid circular dependencies
type Logger interface {
	Enabled(level int) bool
	WithContext(ctx context.Context) Logger
	WithFields(fields map[string]any) Logger
	WithKeysAndValues(keysAndValues ...any) Logger
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Critical(msg string, args ...any)
}
