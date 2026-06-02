package loggertest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
)

func TestRecordingLoggerCapturesEntries(t *testing.T) {
	t.Parallel()

	rec := loggertest.NewRecordingLogger()
	rec.Info("hello", "k", "v")
	rec.Error("oops")

	entries := rec.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, logger.LogLevelInfo, entries[0].Level)
	assert.Equal(t, "hello", entries[0].Message)
	assert.Equal(t, []any{"k", "v"}, entries[0].Args)
	assert.Equal(t, logger.LogLevelError, entries[1].Level)
}

func TestRecordingLoggerFieldsPropagateToChildren(t *testing.T) {
	t.Parallel()

	rec := loggertest.NewRecordingLogger()
	child := rec.WithKeysAndValues("request_id", "abc-123", "tenant", "acme")
	child.Info("processed")

	entries := rec.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "abc-123", entries[0].Fields["request_id"])
	assert.Equal(t, "acme", entries[0].Fields["tenant"])
}

func TestRecordingLoggerChildDoesNotMutateParent(t *testing.T) {
	t.Parallel()

	rec := loggertest.NewRecordingLogger()
	rec.WithFields(logger.LogFields{"only_in_child": true}).Info("child")
	rec.Info("parent")

	entries := rec.Entries()
	require.Len(t, entries, 2)
	assert.True(t, entries[0].Fields["only_in_child"].(bool))
	_, present := entries[1].Fields["only_in_child"]
	assert.False(t, present, "parent must not see child's fields")
}

func TestRecordingLoggerEntriesAtLevel(t *testing.T) {
	t.Parallel()

	rec := loggertest.NewRecordingLogger()
	rec.InfoContext(context.Background(), "i")
	rec.WarnContext(context.Background(), "w1")
	rec.WarnContext(context.Background(), "w2")

	warns := rec.EntriesAtLevel(logger.LogLevelWarn)
	require.Len(t, warns, 2)
	assert.Equal(t, "w1", warns[0].Message)
	assert.Equal(t, "w2", warns[1].Message)
}

func TestRecordingLoggerReset(t *testing.T) {
	t.Parallel()

	rec := loggertest.NewRecordingLogger()
	rec.Info("first")
	rec.Reset()
	rec.Info("second")

	entries := rec.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "second", entries[0].Message)
}
