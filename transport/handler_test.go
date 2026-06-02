package transport_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/transport"
)

type handlerPayload struct {
	Field string `json:"field"`
}

func TestHandlerFuncImplementsHandler(t *testing.T) {
	wantErr := errors.New("boom")
	var h transport.Handler[handlerPayload] = transport.HandlerFunc[handlerPayload](
		func(ctx context.Context, event handlerPayload) error { return wantErr },
	)
	err := h.Handle(context.Background(), handlerPayload{})
	assert.ErrorIs(t, err, wantErr)
}
