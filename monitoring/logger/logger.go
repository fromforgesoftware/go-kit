// Package logger describes interfaces for a logger.Logger and other support elements
package logger

import (
	"context"
	"strings"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/internal"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota - 1
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelCritical
)

func ParseLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "critical":
		return LogLevelCritical
	default:
		return LogLevelInfo
	}
}

type LogFields map[string]any

type Logger interface {
	Enabled(level int) bool

	WithContext(ctx context.Context) Logger
	WithFields(fields LogFields) Logger
	WithKeysAndValues(keysAndValues ...any) Logger

	IsLevelEnabled(level LogLevel) bool

	// Context-aware methods
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	CriticalContext(ctx context.Context, msg string, args ...any)

	// Non-context methods
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Critical(msg string, args ...any)
}

// LoggerType defines the type of logger implementation
type LoggerType string

const (
	ZapLogger  LoggerType = "zap"
	SlogLogger LoggerType = "slog"
)

// config holds the private configuration for logger creation
type config struct {
	loggerType LoggerType
	level      LogLevel
}

// Option defines a function that can modify the logger configuration
type Option func(*config)

// WithType sets the logger type
func WithType(loggerType LoggerType) Option {
	return func(c *config) {
		c.loggerType = loggerType
	}
}

// WithLevel sets the log level
func WithLevel(level LogLevel) Option {
	return func(c *config) {
		c.level = level
	}
}

// defaultConfig returns the default logger configuration options
func defaultConfig() []Option {
	return []Option{
		WithType(ZapLogger),
		WithLevel(LogLevelInfo),
	}
}

// loggerAdapter wraps internal.Logger and adds IsLevelEnabled method
type loggerAdapter struct {
	internal.Logger
}

func (l *loggerAdapter) IsLevelEnabled(level LogLevel) bool {
	return l.Enabled(int(level))
}

func (l *loggerAdapter) WithContext(ctx context.Context) Logger {
	return &loggerAdapter{Logger: l.Logger.WithContext(ctx)}
}

func (l *loggerAdapter) WithFields(fields LogFields) Logger {
	return &loggerAdapter{Logger: l.Logger.WithFields(fields)}
}

func (l *loggerAdapter) WithKeysAndValues(keysAndValues ...any) Logger {
	return &loggerAdapter{Logger: l.Logger.WithKeysAndValues(keysAndValues...)}
}

func (l *loggerAdapter) DebugContext(ctx context.Context, msg string, args ...any) {
	if l.Logger != nil {
		l.Logger.WithContext(ctx).Debug(msg, args...)
	}
}

func (l *loggerAdapter) InfoContext(ctx context.Context, msg string, args ...any) {
	if l.Logger != nil {
		l.Logger.WithContext(ctx).Info(msg, args...)
	}
}

func (l *loggerAdapter) WarnContext(ctx context.Context, msg string, args ...any) {
	if l.Logger != nil {
		l.Logger.WithContext(ctx).Warn(msg, args...)
	}
}

func (l *loggerAdapter) ErrorContext(ctx context.Context, msg string, args ...any) {
	if l.Logger != nil {
		// Try to cast to internal logger if it supports context, or just fallback
		// internal/logger.go interface has WithContext which returns a Logger.
		// We should use that.
		l.Logger.WithContext(ctx).Error(msg, args...)
	}
}

func (l *loggerAdapter) CriticalContext(ctx context.Context, msg string, args ...any) {
	if l.Logger != nil {
		l.Logger.WithContext(ctx).Critical(msg, args...)
	}
}

// New creates a new logger with optional configuration
func New(opts ...Option) Logger {
	cfg := &config{}

	// Apply default options first
	defaultOpts := defaultConfig()
	for _, opt := range defaultOpts {
		opt(cfg)
	}

	// Apply user-provided options (which can override defaults)
	for _, opt := range opts {
		opt(cfg)
	}

	var internalLogger internal.Logger
	switch cfg.loggerType {
	case SlogLogger:
		internalLogger = internal.NewSlogLogger(internal.WithSlogLevel(int(cfg.level)))
	case ZapLogger:
		fallthrough
	default:
		internalLogger = internal.NewZapLogger(internal.WithZapLevel(int(cfg.level)))
	}

	return &loggerAdapter{Logger: internalLogger}
}
