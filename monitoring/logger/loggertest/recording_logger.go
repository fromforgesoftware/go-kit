package loggertest

import (
	"context"
	"sync"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// Entry is one captured log record. Fields are populated by the
// WithFields / WithKeysAndValues calls that came before the terminal
// Debug/Info/Warn/Error/Critical call.
type Entry struct {
	Level   logger.LogLevel
	Message string
	Args    []any
	Fields  map[string]any
}

// recorderState is the shared store. Cloned loggers (produced by
// WithFields / WithKeysAndValues / WithContext) keep a pointer to the
// SAME state so all entries end up in one slice the caller can inspect.
type recorderState struct {
	mu      sync.Mutex
	entries []Entry
}

// RecordingLogger is a logger.Logger that captures every emitted entry
// in memory so tests can assert on log output. The previous stub mock
// set up `.Maybe()` expectations and discarded everything, which meant
// logging middleware could not be unit-tested at all.
type RecordingLogger struct {
	state  *recorderState
	fields map[string]any
}

// NewRecordingLogger returns an empty recorder.
func NewRecordingLogger() *RecordingLogger {
	return &RecordingLogger{
		state:  &recorderState{},
		fields: map[string]any{},
	}
}

// Entries returns a snapshot of all entries recorded so far.
func (l *RecordingLogger) Entries() []Entry {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	out := make([]Entry, len(l.state.entries))
	copy(out, l.state.entries)
	return out
}

// EntriesAtLevel filters captured entries by level.
func (l *RecordingLogger) EntriesAtLevel(level logger.LogLevel) []Entry {
	out := []Entry{}
	for _, e := range l.Entries() {
		if e.Level == level {
			out = append(out, e)
		}
	}
	return out
}

// Reset clears the captured entries.
func (l *RecordingLogger) Reset() {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	l.state.entries = nil
}

func (l *RecordingLogger) record(level logger.LogLevel, msg string, args []any) {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	cp := make(map[string]any, len(l.fields))
	for k, v := range l.fields {
		cp[k] = v
	}
	l.state.entries = append(l.state.entries, Entry{
		Level:   level,
		Message: msg,
		Args:    append([]any(nil), args...),
		Fields:  cp,
	})
}

func (l *RecordingLogger) Enabled(_ int) bool                    { return true }
func (l *RecordingLogger) IsLevelEnabled(_ logger.LogLevel) bool { return true }

func (l *RecordingLogger) WithContext(_ context.Context) logger.Logger {
	return l.clone(nil)
}

func (l *RecordingLogger) WithFields(fields logger.LogFields) logger.Logger {
	extra := make(map[string]any, len(fields))
	for k, v := range fields {
		extra[k] = v
	}
	return l.clone(extra)
}

func (l *RecordingLogger) WithKeysAndValues(kv ...any) logger.Logger {
	extra := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		extra[key] = kv[i+1]
	}
	return l.clone(extra)
}

// clone returns a child logger that shares the parent's recorder state
// but has its own merged fields snapshot.
func (l *RecordingLogger) clone(extra map[string]any) *RecordingLogger {
	cp := &RecordingLogger{
		state:  l.state,
		fields: make(map[string]any, len(l.fields)+len(extra)),
	}
	for k, v := range l.fields {
		cp.fields[k] = v
	}
	for k, v := range extra {
		cp.fields[k] = v
	}
	return cp
}

func (l *RecordingLogger) Debug(msg string, args ...any) { l.record(logger.LogLevelDebug, msg, args) }
func (l *RecordingLogger) Info(msg string, args ...any)  { l.record(logger.LogLevelInfo, msg, args) }
func (l *RecordingLogger) Warn(msg string, args ...any)  { l.record(logger.LogLevelWarn, msg, args) }
func (l *RecordingLogger) Error(msg string, args ...any) { l.record(logger.LogLevelError, msg, args) }
func (l *RecordingLogger) Critical(msg string, args ...any) {
	l.record(logger.LogLevelCritical, msg, args)
}

func (l *RecordingLogger) DebugContext(_ context.Context, msg string, args ...any) {
	l.record(logger.LogLevelDebug, msg, args)
}
func (l *RecordingLogger) InfoContext(_ context.Context, msg string, args ...any) {
	l.record(logger.LogLevelInfo, msg, args)
}
func (l *RecordingLogger) WarnContext(_ context.Context, msg string, args ...any) {
	l.record(logger.LogLevelWarn, msg, args)
}
func (l *RecordingLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	l.record(logger.LogLevelError, msg, args)
}
func (l *RecordingLogger) CriticalContext(_ context.Context, msg string, args ...any) {
	l.record(logger.LogLevelCritical, msg, args)
}
