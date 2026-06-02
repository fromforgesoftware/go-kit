package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// slogConfig holds the private configuration for slog logger
type slogConfig struct {
	level  int
	output io.Writer
	format OutputFormat
}

// SlogOption defines a function that can modify the slog logger configuration
type SlogOption func(*slogConfig)

// defaultSlogConfig returns the default slog configuration
func defaultSlogConfig() []SlogOption {
	return []SlogOption{
		WithSlogLevel(LogLevelInfo),
		WithSlogOutput(os.Stdout),
		WithSlogFormat(TextFormat),
	}
}

// WithSlogLevel sets the log level
func WithSlogLevel(level int) SlogOption {
	return func(c *slogConfig) {
		c.level = level
	}
}

// WithSlogOutput sets the output writer
func WithSlogOutput(output io.Writer) SlogOption {
	return func(c *slogConfig) {
		c.output = output
	}
}

// WithSlogFormat sets the output format
func WithSlogFormat(format OutputFormat) SlogOption {
	return func(c *slogConfig) {
		c.format = format
	}
}

type slogLogger struct {
	slog *slog.Logger
	ctx  context.Context
}

// NewSlogLogger creates a new slog logger with optional configuration
func NewSlogLogger(opts ...SlogOption) *slogLogger {
	cfg := &slogConfig{}
	for _, opt := range append(defaultSlogConfig(), opts...) {
		opt(cfg)
	}

	level := convertLevelToSlogLevel(cfg.level)

	var handler slog.Handler
	switch cfg.format {
	case JSONFormat:
		handler = slog.NewJSONHandler(cfg.output, &slog.HandlerOptions{
			Level: level,
		})
	case TextFormat:
		handler = slog.NewTextHandler(cfg.output, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)

	return &slogLogger{
		slog: logger,
		ctx:  context.Background(),
	}
}

func (l *slogLogger) Enabled(level int) bool {
	slogLevel := convertLevelToSlogLevel(level)
	return l.slog.Enabled(l.ctx, slogLevel)
}

func (l *slogLogger) WithContext(ctx context.Context) Logger {
	return &slogLogger{
		slog: l.slog,
		ctx:  ctx,
	}
}

func (l *slogLogger) WithFields(fields map[string]any) Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &slogLogger{
		slog: l.slog.With(args...),
		ctx:  l.ctx,
	}
}

func (l *slogLogger) WithKeysAndValues(keysAndValues ...any) Logger {
	return &slogLogger{
		slog: l.slog.With(keysAndValues...),
		ctx:  l.ctx,
	}
}

// Non-context methods
func (l *slogLogger) Debug(msg string, args ...any) {
	l.slog.DebugContext(l.ctx, msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.slog.InfoContext(l.ctx, msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.slog.WarnContext(l.ctx, msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.slog.ErrorContext(l.ctx, msg, args...)
}

func (l *slogLogger) Critical(msg string, args ...any) {
	l.slog.ErrorContext(l.ctx, msg, args...)
}

func convertLevelToSlogLevel(level int) slog.Level {
	switch level {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError, LogLevelCritical:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
