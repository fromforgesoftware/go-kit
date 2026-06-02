//go:build integration
// +build integration

package amqptest

import (
	"fmt"
	"sync"
	"testing"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/rabbitmq"
	"github.com/stretchr/testify/assert"
)

var (
	//nolint: gochecknoglobals // singleton
	container *gnomock.Container
	//nolint: gochecknoglobals // singleton
	once sync.Once
)

func GetRabbitMQURL(t *testing.T) string {
	t.Helper()

	once.Do(func() {
		rmq := rabbitmq.Preset(rabbitmq.WithUser("guest", "guest"))

		var err error
		container, err = gnomock.Start(rmq)
		assert.NoError(t, err)
	})

	return fmt.Sprintf("amqp://guest:guest@%s:%d/", container.Host, container.DefaultPort())
}

func RestartRabbitMQ(t *testing.T) {
	t.Helper()

	if container == nil {
		return
	}

	err := gnomock.Stop(container)
	assert.NoError(t, err)

	rmq := rabbitmq.Preset(rabbitmq.WithUser("guest", "guest"))
	defaultPorts := rmq.Ports()
	defaultPorts["default"] = gnomock.Port{Protocol: "tcp", Port: 5672, HostPort: container.DefaultPort()}
	container, err = gnomock.Start(rmq, gnomock.WithCustomNamedPorts(defaultPorts))
	assert.NoError(t, err)
}
