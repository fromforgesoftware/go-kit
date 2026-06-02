package tcp

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"time"
)

// Client represents a reusable TCP client
type Client struct {
	addr         string
	conn         net.Conn
	splitter     bufio.SplitFunc
	reader       *bufio.Scanner
	writeTimeout time.Duration
	readTimeout  time.Duration

	mu      sync.Mutex
	closed  bool
	closing chan struct{}
}

// ClientOption allows configuring the client
type clientOption func(*Client)

// WithClientSplitter sets the packet splitter for the client
func WithClientSplitter(splitter bufio.SplitFunc) clientOption {
	return func(c *Client) {
		c.splitter = splitter
	}
}

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

// NewClient creates a new TCP client and connects to the address
func NewClient(addr string, opts ...clientOption) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		addr: addr,
		conn: conn,
		// Default splitter: ScanLines (matching server default)
		splitter:     bufio.ScanLines,
		writeTimeout: 5 * time.Second,
		readTimeout:  0, // No timeout by default for reading
		closing:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.reader = bufio.NewScanner(conn)
	c.reader.Split(c.splitter)
	// Larger buffer for client reading if needed, can be parameterized later
	c.reader.Buffer(make([]byte, 4096), bufio.MaxScanTokenSize)

	return c, nil
}

// Send sends data to the server
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return net.ErrClosed
	}

	if c.writeTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	} else {
		c.conn.SetWriteDeadline(time.Time{})
	}

	_, err := c.conn.Write(data)
	return err
}

// Receive blocks until a packet is received or error occurs
func (c *Client) Receive(ctx context.Context) ([]byte, error) {
	// Scanner is synchronous, so we need a goroutine or just block.
	// For simple client usage, blocking with a check on context/closing is tricky with Scanner.
	// However, we can set ReadDeadline based on context or timeout logic if we exposed it.
	// Since bufio.Scanner doesn't support Context cancellation easily without closing the connection,
	// we will rely on ReadTimeout or manual closing.

	// Ideally, tests calling Receive() expect a packet soon.

	if c.readTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	} else {
		c.conn.SetReadDeadline(time.Time{})
	}

	if c.reader.Scan() {
		// Return copy of bytes
		b := c.reader.Bytes()
		out := make([]byte, len(b))
		copy(out, b)
		return out, nil
	}

	if err := c.reader.Err(); err != nil {
		return nil, err
	}

	// EOF
	return nil, io.EOF
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closing)
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
