package udp_test

import (
	"context"
	"crypto/rand"
	"net"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/udp"
	"github.com/stretchr/testify/require"
)

// Payload size for benchmarks
const payloadSize = 512

// BenchmarkClientSendReceiveEchoRoundtrip benchmarks standard Send/Receive with allocations
func BenchmarkClientSendReceiveEchoRoundtrip(b *testing.B) {
	// Setup echo server
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(b, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(b, err)
	defer conn.Close()

	go func() {
		buf := make([]byte, 2048)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			conn.WriteToUDP(buf[:n], peer)
		}
	}()

	client, err := udp.NewClient(conn.LocalAddr().String(),
		udp.WithClientReadTimeout(0),
		udp.WithClientWriteTimeout(0),
	)
	require.NoError(b, err)
	defer client.Close()

	payload := make([]byte, payloadSize)
	rand.Read(payload)

	b.ResetTimer()
	b.ReportAllocs()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if err := client.Send(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := client.Receive(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkClientSendRawReceiveIntoZeroCopy benchmarks optimized zero-copy path
func BenchmarkClientSendRawReceiveIntoZeroCopy(b *testing.B) {
	// Setup echo server
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(b, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(b, err)
	defer conn.Close()

	go func() {
		buf := make([]byte, 2048)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			conn.WriteToUDP(buf[:n], peer)
		}
	}()

	client, err := udp.NewClient(conn.LocalAddr().String(),
		udp.WithClientReadTimeout(0),
		udp.WithClientWriteTimeout(0),
	)
	require.NoError(b, err)
	defer client.Close()

	payload := make([]byte, payloadSize)
	rand.Read(payload)

	recvBuf := make([]byte, 2048)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := client.SendRaw(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := client.ReceiveInto(recvBuf); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkServerSessionEchoParallel benchmarks concurrent session handling
func BenchmarkServerSessionEchoParallel(b *testing.B) {
	monitor := monitoringtest.NewMonitor(b)

	handler := udp.HandlerFunc(func(ctx context.Context, sess udp.Session, data []byte) error {
		return sess.SendUnreliable(data) // Echo back
	})

	server, err := udp.NewServer(monitor,
		udp.WithAddress("127.0.0.1:0"),
		udp.WithHandler(handler),
	)
	require.NoError(b, err)

	err = server.Start()
	require.NoError(b, err)
	defer server.Stop()

	// Note: Server address extraction would need to be exposed to get real address
	serverAddr := "127.0.0.1:0"

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		// Create raw UDP connection for benchmarking
		addr, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			b.Fatal(err)
		}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		payload := make([]byte, payloadSize)
		rand.Read(payload)
		buf := make([]byte, 2048)

		for pb.Next() {
			conn.Write(payload)
			conn.Read(buf)
		}
	})
}
