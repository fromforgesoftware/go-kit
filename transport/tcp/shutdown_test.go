package tcp_test

import (
	"bufio"
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/tcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStopWaitsForHandlersToFinish is the contract a caller needs to tear down
// anything a handler touches.
//
// Stop used to join only the accept loop, so it returned while connection
// goroutines were still dispatching. A test whose mock, database or fixture
// went away on the next line then raced a handler still using it — and the
// failure surfaced against whichever test happened to be running, not the one
// that leaked.
func TestStopWaitsForHandlersToFinish(t *testing.T) {
	addr := "localhost:12041"

	var running, finished atomic.Int64
	release := make(chan struct{})

	handler := tcp.HandlerFunc(func(context.Context, tcp.Session, []byte) error {
		running.Add(1)
		<-release // hold the handler open across the Stop call
		finished.Add(1)
		return nil
	})

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("work\n"))
	require.NoError(t, err)

	require.Eventually(t, func() bool { return running.Load() == 1 },
		2*time.Second, 10*time.Millisecond, "the handler never started")

	stopped := make(chan struct{})
	go func() {
		_ = server.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a handler was still running")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the handler finished")
	}

	assert.EqualValues(t, 1, finished.Load(), "the handler must have completed before Stop returned")
}

// TestStopClosesIdleConnections pins the other half: a connection with nothing
// to say is blocked in Scan, and only its socket closing will wake it. Without
// that, waiting for the connection goroutines would hang instead of returning.
func TestStopClosesIdleConnections(t *testing.T) {
	addr := "localhost:12042"

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(tcp.HandlerFunc(func(context.Context, tcp.Session, []byte) error { return nil })),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())

	time.Sleep(100 * time.Millisecond)

	// Three connections that never send anything.
	for range 3 {
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()
	}
	time.Sleep(100 * time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		_ = server.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop hung on idle connections")
	}
}

// TestStopRunsDisconnectHooksBeforeReturning matters because the hook is where
// a caller releases what the session owned.
func TestStopRunsDisconnectHooksBeforeReturning(t *testing.T) {
	addr := "localhost:12043"

	var disconnected atomic.Int64
	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(tcp.HandlerFunc(func(context.Context, tcp.Session, []byte) error { return nil })),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
		tcp.WithOnDisconnect(func(tcp.Session) { disconnected.Add(1) }),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, server.Stop())
	assert.EqualValues(t, 1, disconnected.Load(),
		"the disconnect hook must have run before Stop returned")
}
