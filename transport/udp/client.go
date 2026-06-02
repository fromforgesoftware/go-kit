package udp

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Client represents a reusable UDP client
type Client struct {
	addr         string
	conn         *net.UDPConn
	writeTimeout time.Duration
	readTimeout  time.Duration

	closed atomic.Bool // Lock-free closed flag
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		// 64k buffer
		b := make([]byte, 65535)
		return &b
	},
}

// ClientOption allows configuring the client
type clientOption func(*Client)

// WithClientWriteTimeout sets the write timeout
func WithClientWriteTimeout(d time.Duration) clientOption {
	return func(c *Client) {
		c.writeTimeout = d
	}
}

// WithClientReadTimeout sets the read timeout
func WithClientReadTimeout(d time.Duration) clientOption {
	return func(c *Client) {
		c.readTimeout = d
	}
}

// NewClient creates a new UDP client and connects to the address.
// Creating a "connected" UDP socket allows using Write and Read instead of WriteTo/ReadFrom,
// and filters incoming packets to only those from the server.
func NewClient(addr string, opts ...clientOption) (*Client, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		addr:         addr,
		conn:         conn,
		writeTimeout: 5 * time.Second,
		readTimeout:  5 * time.Second, // Default read timeout for UDP to avoid blocking forever
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Send sends data to the server with timeout handling
func (c *Client) Send(data []byte) error {
	if c.closed.Load() {
		return net.ErrClosed
	}

	// Only set deadline if needed - avoids syscall overhead
	if c.writeTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	_, err := c.conn.Write(data)
	return err
}

// SendRaw sends data without any checks or timeout handling.
// This is the fastest path for high-frequency unreliable packets (e.g., position updates).
// Use this when you don't need timeout handling and can tolerate errors.
func (c *Client) SendRaw(data []byte) error {
	if c.closed.Load() {
		return net.ErrClosed
	}
	_, err := c.conn.Write(data)
	return err
}

// Receive blocks until a packet is received or error occurs.
// It returns a copy of the data.
func (c *Client) Receive(ctx context.Context) ([]byte, error) {
	// Use buffer from pool to avoid allocation per packet
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)
	buf := *bufPtr

	// Only set deadline if needed - avoids syscall overhead
	if c.readTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	// We can't easily select on context with Read, so we rely on Deadlines.
	// If context is already cancelled, return immediately.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	n, _, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}

	// Return exactly the bytes read.
	// Note: We still allocate the result buffer here because we return a []byte that the caller owns.
	// To be truly zero-alloc, the API would need to accept a buffer from the caller (Receive(buf []byte)).
	// But using the pool for the read buffer avoids allocating the 64k buffer on every call if the stack isn't enough or it escapes.
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}

// ReceiveInto reads a packet directly into the provided buffer (zero-copy).
// Returns the number of bytes read. This is the fastest receive path.
// The caller must provide a buffer large enough for the expected packet size (typically MTU size ~1500 bytes).
func (c *Client) ReceiveInto(buf []byte) (int, error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	n, _, err := c.conn.ReadFromUDP(buf)
	return n, err
}

// Close closes the connection
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		// Already closed
		return nil
	}
	return c.conn.Close()
}

// LocalAddr returns the local address
func (c *Client) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote address
func (c *Client) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
