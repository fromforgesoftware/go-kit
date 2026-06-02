package logger_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

func TestLoggerFactory(t *testing.T) {
	tests := []struct {
		name     string
		options  []logger.Option
		expected logger.LoggerType
	}{
		{
			name: "Zap Logger",
			options: []logger.Option{
				logger.WithType(logger.ZapLogger),
				logger.WithLevel(logger.LogLevelInfo),
			},
			expected: logger.ZapLogger,
		},
		{
			name: "Slog Logger",
			options: []logger.Option{
				logger.WithType(logger.SlogLogger),
				logger.WithLevel(logger.LogLevelDebug),
			},
			expected: logger.SlogLogger,
		},
		{
			name: "Default Logger (Zap)",
			options: []logger.Option{
				logger.WithLevel(logger.LogLevelWarn),
			},
			expected: logger.ZapLogger, // Default should be Zap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger using the factory
			l := logger.New(tt.options...)

			// Verify logger is created
			if l == nil {
				t.Fatal("Logger should not be nil")
			}

			// Test basic functionality
			l.Info("Test message")
			l.WithKeysAndValues("test", "value").Debug("Debug message")
			l.WithKeysAndValues("key", "value").Error("Error message")

			// Test level checking
			if !l.IsLevelEnabled(logger.LogLevelError) {
				t.Errorf("Error level should always be enabled")
			}

			// Test WithFields
			l2 := l.WithFields(logger.LogFields{"request_id": "123"})
			l2.Info("Message with fields")

			// Test WithKeysAndValues
			l3 := l.WithKeysAndValues("user", "test", "action", "login")
			l3.Info("Message with key-value pairs")
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected logger.LogLevel
	}{
		{"debug", logger.LogLevelDebug},
		{"DEBUG", logger.LogLevelDebug},
		{"info", logger.LogLevelInfo},
		{"INFO", logger.LogLevelInfo},
		{"warn", logger.LogLevelWarn},
		{"warning", logger.LogLevelWarn},
		{"error", logger.LogLevelError},
		{"critical", logger.LogLevelCritical},
		{"unknown", logger.LogLevelInfo}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := logger.ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
