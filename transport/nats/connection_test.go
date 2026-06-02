package nats_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/nats"
	"github.com/fromforgesoftware/go-kit/transport/nats/natstest"
	"github.com/stretchr/testify/assert"
)

func helperNewConnection(t *testing.T) nats.Connection {
	t.Helper()

	s := natstest.StartEmbeddedServer(t)
	conn, err := nats.NewConnection(nats.WithConnURL(s.ClientURL()))
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

func TestNewConnectionWithDefault(t *testing.T) {
	// Since default reads from env, we can't easily test without setting env
	// but we can test if it doesn't panic and returns a valid error if NATS is not running
	// or we set the env to our embedded server
	s := natstest.StartEmbeddedServer(t)
	t.Setenv("NATS_URL", s.ClientURL())

	conn, err := nats.NewConnection()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnectionWithOptions(t *testing.T) {
	s := natstest.StartEmbeddedServer(t)
	conn, err := nats.NewConnection(
		nats.WithConnURL(s.ClientURL()),
	)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnectionWithInvalidURL(t *testing.T) {
	conn, err := nats.NewConnection(nats.WithConnURL("invalid://url"))
	assert.Error(t, err)
	assert.Nil(t, conn)
}
