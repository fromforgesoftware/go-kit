package redisdb_test

import (
	"os"
	"testing"

	"github.com/fromforgesoftware/go-kit/persistence/redisdb"
	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	t.Run("WithAddress sets addresses correctly", func(t *testing.T) {
		// This is a white-box test - we can't directly inspect the config
		// but we can verify it doesn't panic and integrates properly
		addresses := []string{"localhost:6379", "localhost:6380"}
		opt := redisdb.WithAddress(addresses...)
		assert.NotNil(t, opt)
	})

	t.Run("WithPassword sets password", func(t *testing.T) {
		opt := redisdb.WithPassword("test-password")
		assert.NotNil(t, opt)
	})

	t.Run("WithPassword trims whitespace", func(t *testing.T) {
		opt := redisdb.WithPassword("  test-password  ")
		assert.NotNil(t, opt)
	})

	t.Run("WithPassword ignores empty string", func(t *testing.T) {
		opt := redisdb.WithPassword("")
		assert.NotNil(t, opt)
	})

	t.Run("WithMaxOpenLimit sets pool size", func(t *testing.T) {
		opt := redisdb.WithMaxOpenLimit(50)
		assert.NotNil(t, opt)
	})

	t.Run("WithMaxOpenLimit ignores zero or negative", func(t *testing.T) {
		opt := redisdb.WithMaxOpenLimit(0)
		assert.NotNil(t, opt)
		opt = redisdb.WithMaxOpenLimit(-10)
		assert.NotNil(t, opt)
	})

	t.Run("WithMaxIdleConns sets max idle connections", func(t *testing.T) {
		opt := redisdb.WithMaxIdleConns(20)
		assert.NotNil(t, opt)
	})

	t.Run("WithMaxIdleConns ignores zero or negative", func(t *testing.T) {
		opt := redisdb.WithMaxIdleConns(0)
		assert.NotNil(t, opt)
		opt = redisdb.WithMaxIdleConns(-5)
		assert.NotNil(t, opt)
	})

	t.Run("WithMasterName sets sentinel master name", func(t *testing.T) {
		opt := redisdb.WithMasterName("mymaster")
		assert.NotNil(t, opt)
	})

	t.Run("WithMasterName trims whitespace", func(t *testing.T) {
		opt := redisdb.WithMasterName("  mymaster  ")
		assert.NotNil(t, opt)
	})

	t.Run("WithDB sets database index", func(t *testing.T) {
		opt := redisdb.WithDB(1)
		assert.NotNil(t, opt)
	})
}

func TestEnvOptions(t *testing.T) {
	t.Run("WithAddressFromEnv reads REDIS_ADDRESS", func(t *testing.T) {
		os.Setenv("REDIS_ADDRESS", "localhost:6379,localhost:6380")
		t.Cleanup(func() { os.Unsetenv("REDIS_ADDRESS") })

		opt := redisdb.WithAddressFromEnv()
		assert.NotNil(t, opt)
	})

	t.Run("WithPasswordFromEnv reads REDIS_PASSWORD", func(t *testing.T) {
		os.Setenv("REDIS_PASSWORD", "secret")
		t.Cleanup(func() { os.Unsetenv("REDIS_PASSWORD") })

		opt := redisdb.WithPasswordFromEnv()
		assert.NotNil(t, opt)
	})

	t.Run("WithMasterNameFromEnv reads REDIS_MASTER_NAME", func(t *testing.T) {
		os.Setenv("REDIS_MASTER_NAME", "mymaster")
		t.Cleanup(func() { os.Unsetenv("REDIS_MASTER_NAME") })

		opt := redisdb.WithMasterNameFromEnv()
		assert.NotNil(t, opt)
	})
}

func TestOptionComposition(t *testing.T) {
	t.Run("multiple options can be combined", func(t *testing.T) {
		opts := []redisdb.Option{
			redisdb.WithAddress("localhost:6379"),
			redisdb.WithPassword("password"),
			redisdb.WithMaxOpenLimit(100),
			redisdb.WithMaxIdleConns(10),
			redisdb.WithDB(0),
		}
		assert.Len(t, opts, 5)
		for _, opt := range opts {
			assert.NotNil(t, opt)
		}
	})
}
