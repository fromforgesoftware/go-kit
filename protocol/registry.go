package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// ErrUnknownOpcode is returned when decoding a frame whose opcode has no
// registered codec.
var ErrUnknownOpcode = errors.New("protocol: unknown opcode")

// Registry maps Opcodes to per-type encode/decode functions. Callers
// register their typed payload codecs via the generic Register helper.
type Registry struct {
	mu   sync.RWMutex
	enc  map[Opcode]func(v any, buf *bytes.Buffer) error
	dec  map[Opcode]func(r *bytes.Reader) (any, error)
	name map[Opcode]string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		enc:  make(map[Opcode]func(any, *bytes.Buffer) error),
		dec:  make(map[Opcode]func(*bytes.Reader) (any, error)),
		name: make(map[Opcode]string),
	}
}

// Register binds typed encode + decode functions to an opcode. Name is
// used for diagnostics.
func Register[T any](r *Registry, op Opcode, name string, enc func(T, *bytes.Buffer), dec func(*bytes.Reader) (T, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enc[op] = func(v any, buf *bytes.Buffer) error {
		t, ok := v.(T)
		if !ok {
			return fmt.Errorf("protocol: opcode %d: wrong payload type", op)
		}
		enc(t, buf)
		return nil
	}
	r.dec[op] = func(buf *bytes.Reader) (any, error) {
		t, err := dec(buf)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	r.name[op] = name
}

// EncodePayload looks up the codec for op and encodes v into a new buffer.
func (r *Registry) EncodePayload(op Opcode, v any) ([]byte, error) {
	r.mu.RLock()
	fn, ok := r.enc[op]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownOpcode, op)
	}
	var buf bytes.Buffer
	if err := fn(v, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePayload looks up the codec for op and decodes from payload bytes.
func (r *Registry) DecodePayload(op Opcode, payload []byte) (any, error) {
	r.mu.RLock()
	fn, ok := r.dec[op]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownOpcode, op)
	}
	return fn(bytes.NewReader(payload))
}

// Name returns the opcode's registered name, or empty string if unknown.
func (r *Registry) Name(op Opcode) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name[op]
}
