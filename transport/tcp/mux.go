package tcp

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
)

// OpcodeExtractor reads the routing opcode out of a packet. Default
// implementation (DefaultOpcodeExtractor) treats the first two bytes as a
// little-endian uint16, matching the UDP mux.
type OpcodeExtractor func([]byte) (uint16, error)

// DefaultOpcodeExtractor reads opcode from bytes [0:2] little-endian.
// Matches udp.Mux so a service can swap transports without changing its
// per-opcode routing code.
func DefaultOpcodeExtractor(packet []byte) (uint16, error) {
	if len(packet) < 2 {
		return 0, fmt.Errorf("tcp: packet too short for opcode (len=%d)", len(packet))
	}
	return binary.LittleEndian.Uint16(packet[:2]), nil
}

// Mux dispatches packets to handlers keyed by a uint16 opcode. The
// previous interface used `key interface{}` which differed from
// udp.Mux's `Register(opcode uint16, ...)` shape and required runtime
// type assertions; standardising on uint16 keeps the two transports
// interchangeable for per-opcode routing.
type Mux struct {
	mu        sync.RWMutex
	extractor OpcodeExtractor
	handlers  map[uint16]Handler
}

// NewMux creates a new Mux that uses DefaultOpcodeExtractor.
func NewMux() *Mux {
	return NewMuxWithExtractor(DefaultOpcodeExtractor)
}

// NewMuxWithExtractor creates a Mux that pulls the opcode via a custom
// extractor — useful when the frame layout puts the opcode at a
// different offset.
func NewMuxWithExtractor(extractor OpcodeExtractor) *Mux {
	if extractor == nil {
		extractor = DefaultOpcodeExtractor
	}
	return &Mux{
		extractor: extractor,
		handlers:  make(map[uint16]Handler),
	}
}

// Handle implements Handler interface
func (m *Mux) Handle(ctx context.Context, session Session, packet []byte) error {
	opcode, err := m.extractor(packet)
	if err != nil {
		return fmt.Errorf("failed to extract opcode: %w", err)
	}

	m.mu.RLock()
	handler, ok := m.handlers[opcode]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for opcode %d", opcode)
	}

	return handler.Handle(ctx, session, packet)
}

// Register registers a handler for the given opcode.
func (m *Mux) Register(opcode uint16, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[opcode] = handler
}

// RegisterFunc registers a handler function for the given opcode.
func (m *Mux) RegisterFunc(opcode uint16, handler func(context.Context, Session, []byte) error) {
	m.Register(opcode, HandlerFunc(handler))
}
