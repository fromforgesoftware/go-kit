package tcp_test

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/tcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerGenericHandler(t *testing.T) {
	port := "12001"
	addr := "localhost:" + port

	endpoint := func(ctx context.Context, req string) (string, error) {
		return req + "ACK", nil
	}
	dec := func(ctx context.Context, p []byte) (string, error) { return strings.TrimSpace(string(p)), nil }
	enc := func(ctx context.Context, r string) ([][]byte, error) { return [][]byte{[]byte(r + "\n")}, nil }

	handler := tcp.NewHandler(endpoint, dec, enc)

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
		tcp.WithWriteBufferSize(10),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("HELLO\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)
	res, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HELLOACK\n", res)
}

func TestServerLifecycle(t *testing.T) {
	port := "12009"
	addr := "localhost:" + port

	connectCh := make(chan bool, 1)
	disconnectCh := make(chan bool, 1)

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(&mockHandler{}),
		tcp.WithAddress(addr),
		tcp.WithOnConnect(func(s tcp.Session) {
			connectCh <- true
		}),
		tcp.WithOnDisconnect(func(s tcp.Session) {
			disconnectCh <- true
		}),
	)
	require.NoError(t, err)

	require.NoError(t, server.Start())
	defer server.Stop()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	select {
	case <-connectCh:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for onConnect")
	}

	conn.Close()

	select {
	case <-disconnectCh:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for onDisconnect")
	}
}

func TestNewMultiHandler(t *testing.T) {
	port := "12010"
	addr := "localhost:" + port

	// Endpoint returns multiple items
	endpoint := func(ctx context.Context, req string) ([]string, error) {
		return []string{req + "-PACKET1", req + "-PACKET2"}, nil
	}
	dec := func(ctx context.Context, p []byte) (string, error) { return strings.TrimSpace(string(p)), nil }

	// Encoder converts response to multiple byte slices (packets)
	enc := func(ctx context.Context, r []string) ([][]byte, error) {
		packets := make([][]byte, len(r))
		for i, s := range r {
			packets[i] = []byte(s + "\n")
		}
		return packets, nil
	}

	handler := tcp.NewHandler(endpoint, dec, enc)

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
		tcp.WithWriteBufferSize(10),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("HELLO\n"))
	require.NoError(t, err)

	reader := bufio.NewReader(conn)

	// Read first response
	res1, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HELLO-PACKET1\n", res1)

	// Read second response
	res2, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HELLO-PACKET2\n", res2)
}

func TestErrorHandler(t *testing.T) {
	port := "12011"
	addr := "localhost:" + port

	errCh := make(chan error, 1)

	// Endpoint returns error
	endpoint := func(ctx context.Context, req string) (string, error) {
		return "", assert.AnError
	}
	dec := func(ctx context.Context, p []byte) (string, error) { return string(p), nil }
	enc := func(ctx context.Context, r string) ([][]byte, error) { return nil, nil }

	// Custom ErrorHandler
	errorHandler := &mockErrorHandler{
		handleFunc: func(ctx context.Context, err error) {
			errCh <- err
		},
	}

	handler := tcp.NewHandler(
		endpoint,
		dec,
		enc,
		tcp.HandlerErrorHandler(errorHandler),
	)

	server, err := tcp.NewServer(
		monitoringtest.NewMonitor(t),
		tcp.WithHandler(handler),
		tcp.WithAddress(addr),
		tcp.WithPacketSplitter(bufio.ScanLines),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("TRIGGER_ERROR\n"))
	require.NoError(t, err)

	select {
	case err := <-errCh:
		assert.Equal(t, assert.AnError, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for error handler")
	}
}

type mockErrorHandler struct {
	handleFunc func(ctx context.Context, err error)
}

func (h *mockErrorHandler) Handle(ctx context.Context, err error) {
	if h.handleFunc != nil {
		h.handleFunc(ctx, err)
	}
}

type mockHandler struct{}

func (h *mockHandler) Handle(ctx context.Context, s tcp.Session, payload []byte) error { return nil }
