// Package loggertest ...
package loggertest

import (
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// TestingT is the interface wrapper around *testing.T and *testing.B
type TestingT interface {
	mock.TestingT
	Cleanup(func())
}

func NewStubLogger(t TestingT) logger.Logger {
	log := NewLogger(t)
	// Allow any calls
	log.EXPECT().Enabled(mock.Anything).Return(false).Maybe()
	log.EXPECT().IsLevelEnabled(mock.Anything).Return(false).Maybe()
	log.EXPECT().WithContext(mock.Anything).Return(log).Maybe()
	log.EXPECT().WithFields(mock.Anything).Return(log).Maybe()
	// Allow generic key-values (up to 10 args for simplicity)
	kvArgs := []any{}
	for i := 0; i < 24; i++ {
		log.EXPECT().WithKeysAndValues(kvArgs...).Return(log).Maybe()
		kvArgs = append(kvArgs, mock.Anything)
	}

	// Log methods with variadic args
	args := []any{}
	for i := 0; i < 24; i++ {
		log.EXPECT().Debug(mock.Anything, args...).Maybe()
		log.EXPECT().Info(mock.Anything, args...).Maybe()
		log.EXPECT().Warn(mock.Anything, args...).Maybe()
		log.EXPECT().Error(mock.Anything, args...).Maybe()
		log.EXPECT().Critical(mock.Anything, args...).Maybe()

		log.EXPECT().DebugContext(mock.Anything, mock.Anything, args...).Maybe()
		log.EXPECT().InfoContext(mock.Anything, mock.Anything, args...).Maybe()
		log.EXPECT().WarnContext(mock.Anything, mock.Anything, args...).Maybe()
		log.EXPECT().ErrorContext(mock.Anything, mock.Anything, args...).Maybe()
		log.EXPECT().CriticalContext(mock.Anything, mock.Anything, args...).Maybe()

		args = append(args, mock.Anything)
	}

	return log
}
