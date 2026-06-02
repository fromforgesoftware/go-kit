package transport

import (
	"context"
	"encoding/json"
)

// Encoder turns an application value into the byte payload that goes on
// the wire. Returns an error if the value cannot be marshalled.
type Encoder[T any] func(ctx context.Context, v T) ([]byte, error)

// Decoder is the inverse of Encoder: given a payload (and any broker
// metadata the transport extracted into ctx) it returns the decoded
// value or an error.
type Decoder[T any] func(ctx context.Context, payload []byte) (T, error)

// JSONEncoder is a ready-made Encoder for JSON payloads. Most services
// can use this directly instead of writing a per-type encoder.
func JSONEncoder[T any](_ context.Context, v T) ([]byte, error) {
	return json.Marshal(v)
}

// JSONDecoder is a ready-made Decoder for JSON payloads. Mirror of
// JSONEncoder for the consumer side.
func JSONDecoder[T any](_ context.Context, payload []byte) (T, error) {
	var v T
	err := json.Unmarshal(payload, &v)
	return v, err
}
