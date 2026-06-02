package tcp_test

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/tcp"
)

func TestDefaultOpcodeExtractor(t *testing.T) {
	t.Parallel()

	t.Run("reads little-endian uint16 from first 2 bytes", func(t *testing.T) {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint16(buf, 0x1234)
		op, err := tcp.DefaultOpcodeExtractor(buf)
		require.NoError(t, err)
		assert.Equal(t, uint16(0x1234), op)
	})

	t.Run("errors on short packet", func(t *testing.T) {
		_, err := tcp.DefaultOpcodeExtractor([]byte{0x01})
		assert.Error(t, err)
	})
}

func TestMuxRoutesByOpcode(t *testing.T) {
	t.Parallel()

	m := tcp.NewMux()

	var got1, got2 atomic.Int32
	m.RegisterFunc(1, func(ctx context.Context, sess tcp.Session, payload []byte) error {
		got1.Add(1)
		return nil
	})
	m.RegisterFunc(2, func(ctx context.Context, sess tcp.Session, payload []byte) error {
		got2.Add(1)
		return nil
	})

	mk := func(op uint16) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint16(b, op)
		return b
	}

	require.NoError(t, m.Handle(context.Background(), nil, mk(1)))
	require.NoError(t, m.Handle(context.Background(), nil, mk(1)))
	require.NoError(t, m.Handle(context.Background(), nil, mk(2)))

	assert.Equal(t, int32(2), got1.Load())
	assert.Equal(t, int32(1), got2.Load())
}

func TestMuxUnknownOpcodeReturnsError(t *testing.T) {
	t.Parallel()

	m := tcp.NewMux()
	m.RegisterFunc(1, func(ctx context.Context, sess tcp.Session, payload []byte) error {
		t.Fatal("handler for opcode 1 should not run for opcode 99")
		return nil
	})

	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b, 99)
	err := m.Handle(context.Background(), nil, b)
	assert.Error(t, err)
}

func TestMuxCustomExtractor(t *testing.T) {
	t.Parallel()

	// Opcode at offset 4 (e.g. a fixed-size envelope header before the
	// routing bytes).
	extractor := func(p []byte) (uint16, error) {
		if len(p) < 6 {
			return 0, errors.New("too short")
		}
		return binary.LittleEndian.Uint16(p[4:6]), nil
	}

	m := tcp.NewMuxWithExtractor(extractor)
	var called atomic.Bool
	m.RegisterFunc(42, func(ctx context.Context, sess tcp.Session, payload []byte) error {
		called.Store(true)
		return nil
	})

	pkt := make([]byte, 8)
	binary.LittleEndian.PutUint16(pkt[4:6], 42)
	require.NoError(t, m.Handle(context.Background(), nil, pkt))
	assert.True(t, called.Load())
}

func TestMuxRegisterIsRaceFree(t *testing.T) {
	// Concurrent Register + Handle calls — the read lock on Handle and
	// write lock on Register must not deadlock or race.
	m := tcp.NewMux()
	pkt := make([]byte, 4)
	binary.LittleEndian.PutUint16(pkt, 1)
	m.RegisterFunc(1, func(ctx context.Context, sess tcp.Session, payload []byte) error { return nil })

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		op := uint16(i + 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.RegisterFunc(op, func(ctx context.Context, sess tcp.Session, payload []byte) error { return nil })
		}()
		go func() {
			defer wg.Done()
			_ = m.Handle(context.Background(), nil, pkt)
		}()
	}
	wg.Wait()
}
