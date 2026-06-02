//go:build integration
// +build integration

package redistest

import (
	"context"
	"sync"
	"testing"

	"github.com/orlangure/gnomock"
	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
	"github.com/fromforgesoftware/go-kit/persistence/redisdb"

	"github.com/orlangure/gnomock/preset/redis"
)

var (
	//nolint: gochecknoglobals // singleton
	database *db
	//nolint: gochecknoglobals // singleton
	once sync.Once
)

type db struct {
	*redisdb.Client
	ConnAddr string
}

func GetDB(t *testing.T) *db {
	t.Helper()

	once.Do(
		func() {
			loggerInstance := logger.New(
				logger.WithType(logger.ZapLogger),
				logger.WithLevel(logger.LogLevelDebug),
			)
			tracerInstance, err := tracer.New(tracer.WithType(tracer.NoopTracer))
			assert.NoError(t, err)
			monitor := monitoring.New(loggerInstance, tracerInstance)
			database = helperCreateRedisContainer(t, monitor)
		})

	return database
}

func helperCreateRedisContainer(t *testing.T, m monitoring.Monitor) *db {
	t.Helper()

	p := redis.Preset(redis.WithVersion("7.0.11"))
	container, err := gnomock.Start(p)
	assert.NoError(t, err)

	addr := container.DefaultAddress()
	client, err := redisdb.New(m, redisdb.WithAddress(addr))
	assert.NoError(t, err)
	assert.NotNil(t, client)
	cmd, err := client.ConfigSet(context.TODO(), "notify-keyspace-events", "KEA").Result()
	assert.NoError(t, err)
	assert.Equal(t, "OK", cmd)

	return &db{
		Client:   client,
		ConnAddr: addr,
	}
}
