package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/google/uuid"
)

// LogSink writes each audit event as a JSON line through the kit logger. It
// stands in for a durable sink during local development, or wherever a
// deployment runs without an audit backend.
type LogSink struct {
	log logger.Logger
}

func NewLogSink() *LogSink {
	return &LogSink{log: logger.New()}
}

func (s *LogSink) Emit(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "audit", "event", json.RawMessage(line))
	return nil
}
