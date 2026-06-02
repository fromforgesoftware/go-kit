package redisdb

import (
	"context"
	"fmt"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

// wrappedLogger adapts our monitoring.Logger to the redis library's logging interface.
// The redis library expects a Printf(context.Context, string, ...any) method,
// which we implement by forwarding to our logger's DebugContext method.
type wrappedLogger struct{ logger.Logger }

func (l wrappedLogger) Printf(ctx context.Context, format string, v ...any) {
	l.DebugContext(ctx, fmt.Sprintf(format, v...))
}

func newLogger(m monitoring.Monitor) wrappedLogger {
	return wrappedLogger{m.Logger()}
}
