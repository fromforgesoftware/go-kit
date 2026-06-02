package nats_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/nats"
	"github.com/stretchr/testify/assert"
)

type testObject struct {
	TestField string `json:"test_field"`
}

func encodeTestObject(ctx context.Context, v testObject) ([]byte, error) {
	return json.Marshal(v)
}

func helperNewProducer(t *testing.T, conn nats.Connection) nats.Producer[testObject] {
	t.Helper()

	log := loggertest.NewStubLogger(t)
	prod, err := nats.NewProducer(
		conn,
		log,
		"test.subject",
		encodeTestObject,
	)
	assert.NoError(t, err)
	assert.NotNil(t, prod)
	return prod
}
