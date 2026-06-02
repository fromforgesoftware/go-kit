package transport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport"
)

type codecPayload struct {
	Field string `json:"field"`
}

func TestJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	in := codecPayload{Field: "hello"}

	data, err := transport.JSONEncoder[codecPayload](ctx, in)
	require.NoError(t, err)

	got, err := transport.JSONDecoder[codecPayload](ctx, data)
	require.NoError(t, err)
	assert.Equal(t, in, got)
}
