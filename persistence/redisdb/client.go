// Package redisdb provides a Redis client wrapper with connection pooling,
// configuration management, and FX integration.
//
// The client supports both single-instance and cluster/sentinel configurations
// via redis.UniversalClient. Configuration is primarily driven by environment
// variables, but can be overridden using functional options.
//
// Environment Variables:
//   - REDIS_ADDRESS: Comma-separated list of Redis server addresses (required)
//   - REDIS_PASSWORD: Password for authentication (optional)
//   - REDIS_MASTER_NAME: Sentinel master name for failover scenarios (optional)
//
// Example usage:
//
//	client, err := redisdb.New(monitor)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Use client
//	err = client.Set(ctx, "key", "value", 0).Err()
package redisdb

import (
	"context"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

const (
	maxOpenConns = 100
	maxIdleConns = 10
)

type config struct {
	rConfig *redis.UniversalOptions
}

func newConfig(options ...Option) *config {
	c := &config{
		rConfig: new(redis.UniversalOptions),
	}
	for _, opt := range append(defaultOptions(), options...) {
		opt(c)
	}

	return c
}

// Option defines the contract for options applied to a redis.Client.
type Option func(*config)

// WithAddress allows to set the host:port address
func WithAddress(addr ...string) Option {
	return func(c *config) {
		c.rConfig.Addrs = addr
	}
}

// WithMaxOpenLimit allows to set a maximum of open connections by the client.
func WithMaxOpenLimit(openConnsLimit int) Option {
	return func(c *config) {
		if openConnsLimit > 0 {
			c.rConfig.PoolSize = openConnsLimit
		}
	}
}

// WithMaxIdleConns allows to set a maximum of idle connections by the client.
func WithMaxIdleConns(idleConnsLimit int) Option {
	return func(c *config) {
		if idleConnsLimit > 0 {
			c.rConfig.MaxIdleConns = idleConnsLimit
		}
	}
}

// WithPassword allows to set the password.
func WithPassword(password string) Option {
	return func(c *config) {
		pass := strings.TrimSpace(password)
		if len(pass) > 0 {
			c.rConfig.Password = pass
			c.rConfig.SentinelPassword = pass
		}
	}
}

// WithMasterName allows to set the master name for the sentinel.
func WithMasterName(masterName string) Option {
	return func(c *config) {
		name := strings.TrimSpace(masterName)
		if len(name) > 0 {
			c.rConfig.MasterName = name
		}
	}
}

// WithDB allows to set the database to be selected after connecting to the server.
func WithDB(db int) Option {
	return func(c *config) {
		c.rConfig.DB = db
	}
}

// WithAddressFromEnv allows to set the host:port address
// by reading the REDIS_ADDRESS envvar (this is a default option).
func WithAddressFromEnv() Option {
	return WithAddress(strings.Split(os.Getenv("REDIS_ADDRESS"), ",")...)
}

// WithPasswordFromEnv allows to set the password
// by reading the REDIS_PASSWORD envvar (this is a default option).
func WithPasswordFromEnv() Option {
	return WithPassword(os.Getenv("REDIS_PASSWORD"))
}

// WithMasterNameFromEnv allows to set the master name for the sentinel
// This option only works with NewFailoverClusterClient.
func WithMasterNameFromEnv() Option {
	return WithMasterName(os.Getenv("REDIS_MASTER_NAME"))
}

func defaultOptions() []Option {
	return []Option{
		WithAddressFromEnv(),
		WithPasswordFromEnv(),
		WithMasterNameFromEnv(),
		WithMaxOpenLimit(maxOpenConns),
		WithMaxIdleConns(maxIdleConns),
	}
}

// Client wraps redis.UniversalClient to provide a unified interface for
// both standalone and cluster Redis deployments. The embedding allows direct
// access to all redis.UniversalClient methods while enabling future extensions.
type Client struct {
	redis.UniversalClient
}

// New creates a new Redis client with the given monitoring and options.
// It establishes a connection to Redis, verifies connectivity via PING,
// and configures keyspace notifications for pub/sub functionality.
//
// The client uses connection pooling with default limits of 100 max open
// connections and 10 max idle connections. These can be customized using
// WithMaxOpenLimit and WithMaxIdleConns options.
//
// Returns an error if connection fails, PING doesn't return PONG, or if
// keyspacenotifications configuration fails.
func New(m monitoring.Monitor, options ...Option) (*Client, error) {
	config := newConfig(options...)

	cli := redis.NewUniversalClient(config.rConfig)
	redis.SetLogger(newLogger(m))

	cmd, err := cli.Ping(context.Background()).Result()
	if err != nil {
		return nil, err
	}
	if cmd != "PONG" {
		return nil, newPingErr()
	}

	cmd, err = cli.ConfigSet(context.Background(), "notify-keyspace-events", "KEA").Result()
	if err != nil {
		return nil, err
	}
	if cmd != "OK" {
		return nil, newNotifyKeySpaceEventsErr()
	}

	return &Client{cli}, nil
}
