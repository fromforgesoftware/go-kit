package internal

import (
	"context"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapConfig holds the private configuration for zap logger
type zapConfig struct {
	zap.Config
	level  int
	output io.Writer
}

// ZapOption defines a function that can modify the zap logger configuration
type ZapOption func(*zapConfig)

// defaultZapConfig returns the default zap configuration
func defaultZapConfig() []ZapOption {
	zapConfig := WithZapDevelopmentConfig()
	defaultLevel := LogLevelDebug
	if os.Getenv("ENV") != "" && os.Getenv("ENV") != "dev" {
		zapConfig = WithZapProductionConfig()
		defaultLevel = LogLevelInfo
	}

	return []ZapOption{
		zapConfig,
		WithZapLevel(defaultLevel),
		WithZapOutput(os.Stdout),
	}
}

// WithZapProductionConfig sets the production configuration for zap logger
func WithZapProductionConfig() ZapOption {
	return func(cfg *zapConfig) {
		cfg.Config = zap.NewProductionConfig()
	}
}

// WithZapDevelopmentConfig sets the development configuration for zap logger
func WithZapDevelopmentConfig() ZapOption {
	return func(cfg *zapConfig) {
		cfg.Config = zap.NewDevelopmentConfig()
	}
}

// WithZapLevel sets the log level
func WithZapLevel(level int) ZapOption {
	return func(c *zapConfig) {
		c.level = level
	}
}

// WithZapOutput sets the output writer
func WithZapOutput(output io.Writer) ZapOption {
	return func(c *zapConfig) {
		c.output = output
	}
}

type zapLogger struct {
	sugar *zap.SugaredLogger
	ctx   context.Context
}

// NewZapLogger creates a new zap logger with optional configuration
func NewZapLogger(opts ...ZapOption) *zapLogger {
	cfg := &zapConfig{}
	for _, opt := range append(defaultZapConfig(), opts...) {
		opt(cfg)
	}

	stackSkip := 1
	zapOptions := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(stackSkip),
	}
	zapOptions = append(zapOptions,
		zap.ErrorOutput(zapcore.AddSync(cfg.output)),
	)
	zl, err := cfg.Config.Build(zapOptions...)
	if err != nil {
		panic(err)
	}

	return &zapLogger{
		sugar: zl.Sugar(),
		ctx:   context.Background(),
	}
}

func (l *zapLogger) Enabled(level int) bool {
	zapLevel := convertLevelToZapLevel(level)
	return l.sugar.Desugar().Core().Enabled(zapLevel)
}

func (l *zapLogger) WithContext(ctx context.Context) Logger {
	return &zapLogger{
		sugar: l.sugar,
		ctx:   ctx,
	}
}

func (l *zapLogger) WithFields(fields map[string]any) Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &zapLogger{
		sugar: l.sugar.With(args...),
		ctx:   l.ctx,
	}
}

func (l *zapLogger) WithKeysAndValues(keysAndValues ...any) Logger {
	return &zapLogger{
		sugar: l.sugar.With(keysAndValues...),
		ctx:   l.ctx,
	}
}

// Non-context methods
func (l *zapLogger) Debug(msg string, args ...any) {
	l.sugar.Debugw(msg, args...)
}

func (l *zapLogger) Info(msg string, args ...any) {
	l.sugar.Infow(msg, args...)
}

func (l *zapLogger) Warn(msg string, args ...any) {
	l.sugar.Warnw(msg, args...)
}

func (l *zapLogger) Error(msg string, args ...any) {
	l.sugar.Errorw(msg, args...)
}

func (l *zapLogger) Critical(msg string, args ...any) {
	l.sugar.Errorw(msg, args...)
}

func convertLevelToZapLevel(level int) zapcore.Level {
	switch level {
	case LogLevelDebug:
		return zapcore.DebugLevel
	case LogLevelInfo:
		return zapcore.InfoLevel
	case LogLevelWarn:
		return zapcore.WarnLevel
	case LogLevelError, LogLevelCritical:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
