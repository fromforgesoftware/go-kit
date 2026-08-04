package tcp_test

import (
	"bufio"
	"context"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/tcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rollingSplitter frames [len uint16][payload] where len is obfuscated by a
// per-connection counter, so a shared SplitFunc cannot decode it: reading the
// same header twice yields different values. It stands in for the stream
// ciphers real protocols put over their headers.
type rollingSplitter struct {
	seq  uint16
	seen int
}

func (r *rollingSplitter) split(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if len(data) < 2 {
		return 0, nil, nil
	}
	n := int(binary.BigEndian.Uint16(data[:2]) - r.seq)
	if len(data) < 2+n {
		return 0, nil, nil
	}
	r.seq++
	r.seen++
	// Token is freshly allocated, so it stays valid past the next Scan.
	out := make([]byte, n)
	copy(out, data[2:2+n])
	return 2 + n, out, nil
}

func writeRolling(t *testing.T, conn net.Conn, seq uint16, payload string) {
	t.Helper()
	hdr := make([]byte, 2)
	binary.BigEndian.PutUint16(hdr, uint16(len(payload))+seq)
	_, err := conn.Write(append(hdr, payload...))
	require.NoError(t, err)
}

func TestPacketSplitterFactoryHoldsPerConnectionState(t *testing.T) {
	addr := "localhost:12031"

	var mu sync.Mutex
	got := map[string][]string{}

	handler := tcp.HandlerFunc(func(_ context.Context, sess tcp.Session, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		id := sess.ID().String()
		got[id] = append(got[id], string(data))
		return nil
	})

	var built atomic.Int64
	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitterFactory(func(tcp.Session) bufio.SplitFunc {
			built.Add(1)
			return (&rollingSplitter{}).split
		}),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Two connections, each with its own sequence: if the splitter state were
	// shared, the second connection's frames would decode at the wrong length.
	for range 2 {
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		writeRolling(t, conn, 0, "alpha")
		writeRolling(t, conn, 1, "bravo")
		writeRolling(t, conn, 2, "charlie")
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		if len(got) != 2 {
			return false
		}
		for _, msgs := range got {
			if len(msgs) != 3 {
				return false
			}
		}
		return true
	}, 2*time.Second, 20*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.EqualValues(t, 2, built.Load(), "one splitter per connection")
	for _, msgs := range got {
		assert.Equal(t, []string{"alpha", "bravo", "charlie"}, msgs)
	}
}

func TestPacketSplitterFactoryTakesPrecedence(t *testing.T) {
	addr := "localhost:12032"

	received := make(chan string, 1)
	handler := tcp.HandlerFunc(func(_ context.Context, _ tcp.Session, data []byte) error {
		received <- string(data)
		return nil
	})

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
		tcp.WithPacketSplitterFactory(func(tcp.Session) bufio.SplitFunc {
			return (&rollingSplitter{}).split
		}),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	// Contains a newline: ScanLines would cut it in half, the factory splitter
	// delivers it whole.
	writeRolling(t, conn, 0, "one\ntwo")

	select {
	case msg := <-received:
		assert.Equal(t, "one\ntwo", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the framed packet")
	}
}

func TestSplitterOwnedTokensReachHandlerIntact(t *testing.T) {
	addr := "localhost:12033"

	var mu sync.Mutex
	var msgs [][]byte

	// Retains the payload past the next Scan: with the copy skipped, this is
	// only sound because the splitter allocates its tokens.
	handler := tcp.HandlerFunc(func(_ context.Context, _ tcp.Session, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		msgs = append(msgs, data)
		return nil
	})

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitterFactory(func(tcp.Session) bufio.SplitFunc {
			return (&rollingSplitter{}).split
		}),
		tcp.WithSplitterOwnedTokens(),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	want := []string{"first", "second", "third", "fourth"}
	for i, p := range want {
		writeRolling(t, conn, uint16(i), p)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(msgs) == len(want)
	}, 2*time.Second, 20*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for i, w := range want {
		assert.Equal(t, w, string(msgs[i]), "payload %d survived later scans", i)
	}
}
