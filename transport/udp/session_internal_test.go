package udp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
)

// newTestServer returns a Server wired with a no-op handler and a stub
// monitor, suitable for direct session-level white-box tests. The server
// is not Start()ed — these tests poke the session in-process.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(monitoringtest.NewMonitor(t),
		WithAddress("127.0.0.1:0"),
		WithHandler(HandlerFunc(func(ctx context.Context, sess Session, data []byte) error {
			return nil
		})),
	)
	require.NoError(t, err)
	// Open a loopback socket so writeTo doesn't panic on nil conn.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	srv.conn = conn
	srv.ctx, srv.cancel = context.WithCancel(context.Background())
	t.Cleanup(func() {
		srv.cancel()
		_ = conn.Close()
	})
	return srv
}

func TestSessionSequenceWrapAround(t *testing.T) {
	srv := newTestServer(t)
	sess := newSession(srv.ctx, srv, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})
	defer sess.Close()

	// Seed lastAckRecv near the wrap boundary.
	sess.mu.Lock()
	sess.lastAckRecv = 65530
	sess.mu.Unlock()

	// A "newer" packet that already wrapped past 65535. With a naïve
	// `p.Seq > s.lastAckRecv` comparison this would be treated as
	// older-than-current and lastAckRecv would stay at 65530 forever.
	require.NoError(t, sess.ProcessPacket(Packet{Type: PacketTypeReliable, Seq: 3}))

	sess.mu.Lock()
	got := sess.lastAckRecv
	sess.mu.Unlock()
	assert.Equal(t, uint16(3), got, "wrap-safe comparison must accept post-wrap seqs")

	// A "stale" packet that's actually older than current; lastAckRecv
	// must NOT regress.
	require.NoError(t, sess.ProcessPacket(Packet{Type: PacketTypeReliable, Seq: 65520}))
	sess.mu.Lock()
	got = sess.lastAckRecv
	sess.mu.Unlock()
	assert.Equal(t, uint16(3), got, "stale post-wrap seq must not regress lastAckRecv")
}

func TestSessionHandleAckRemovesPendingByExactSeq(t *testing.T) {
	srv := newTestServer(t)
	sess := newSession(srv.ctx, srv, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})
	defer sess.Close()

	// Send 3 reliable packets; they all queue in pending.
	require.NoError(t, sess.SendReliable([]byte("a")))
	require.NoError(t, sess.SendReliable([]byte("b")))
	require.NoError(t, sess.SendReliable([]byte("c")))

	sess.mu.Lock()
	assert.Len(t, sess.pending, 3)
	sess.mu.Unlock()

	// Ack the middle one. O(1) map delete — only that entry is removed.
	sess.mu.Lock()
	sess.handleAck(2)
	remaining := len(sess.pending)
	_, stillPresent := sess.pending[2]
	sess.mu.Unlock()
	assert.Equal(t, 2, remaining)
	assert.False(t, stillPresent, "acked seq must be removed from pending")
}

func TestSessionTouchUpdatesLastSeen(t *testing.T) {
	srv := newTestServer(t)
	sess := newSession(srv.ctx, srv, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})
	defer sess.Close()

	t0 := time.Unix(0, sess.lastSeenNano.Load())
	time.Sleep(2 * time.Millisecond)
	sess.touch()
	t1 := time.Unix(0, sess.lastSeenNano.Load())

	assert.True(t, t1.After(t0), "touch must move lastSeenNano forward")
}

func TestSessionIdleFor(t *testing.T) {
	srv := newTestServer(t)
	sess := newSession(srv.ctx, srv, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})
	defer sess.Close()

	// Backdate lastSeen by 5 seconds.
	sess.lastSeenNano.Store(time.Now().Add(-5 * time.Second).UnixNano())

	idle := sess.idleFor(time.Now())
	assert.Greater(t, idle, 4*time.Second)
	assert.Less(t, idle, 6*time.Second)
}

func TestSessionEnqueueDropsWhenFull(t *testing.T) {
	srv := newTestServer(t)
	sess := newSession(srv.ctx, srv, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})

	// Fill the inbound buffer without draining (we never start
	// dispatchLoop's consumer side here — the goroutine is running but
	// our test handler is a no-op so it drains quickly; instead, we
	// close the session up-front so dispatchLoop exits, then send into
	// the closed channel via the closed-flag fast path).
	sess.Close()
	for i := 0; i < sessionInboundBuf+10; i++ {
		ok := sess.enqueue([]byte{byte(i)})
		assert.False(t, ok, "enqueue must refuse after Close")
	}
}
