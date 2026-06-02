package udp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientSendReceiveEchoServer(t *testing.T) {
	// Start echo server
	serverAddr := "127.0.0.1:0"
	conn, err := NewRawListener(serverAddr)
	require.NoError(t, err)
	defer conn.Close()

	realAddr := conn.LocalAddr().String()

	// Server loop: echo back
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()

	// Test client connection
	client, err := NewClient(realAddr, WithClientReadTimeout(1*time.Second))
	require.NoError(t, err)
	defer client.Close()

	assert.NotNil(t, client.LocalAddr())
	assert.Equal(t, realAddr, client.RemoteAddr().String())

	// Test Send
	msg := []byte("hello udp")
	err = client.Send(msg)
	require.NoError(t, err)

	// Test Receive
	ctx := context.Background()
	resp, err := client.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, msg, resp)
}

func TestClientReceiveTimeoutNoResponse(t *testing.T) {
	// Listen but don't reply
	serverAddr := "127.0.0.1:0"
	conn, err := NewRawListener(serverAddr)
	require.NoError(t, err)
	defer conn.Close()
	realAddr := conn.LocalAddr().String()

	// Dial with short timeout
	client, err := NewClient(realAddr, WithClientReadTimeout(10*time.Millisecond))
	require.NoError(t, err)
	defer client.Close()

	err = client.Send([]byte("ping"))
	require.NoError(t, err)

	// Receive should timeout since server doesn't respond
	_, err = client.Receive(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// Helper to start raw listener
func NewRawListener(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}
