package tcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSessionClosed = errors.New("session closed")
	ErrSendQueueFull = errors.New("session send queue full")
)

// SendOverflowPolicy controls Session.Send behaviour when the internal
// queue is full.
type SendOverflowPolicy int

const (
	// SendOverflowError returns ErrSendQueueFull immediately. Default —
	// blocking on a slow consumer stalls the caller and is rarely what
	// upstream code wants.
	SendOverflowError SendOverflowPolicy = iota
	// SendOverflowBlock waits until the queue has room or the session is
	// closed. Matches the original behaviour; opt-in for callers that
	// prefer back-pressure over drops.
	SendOverflowBlock
	// SendOverflowDropOldest discards the oldest queued message to make
	// room for the new one. Useful for "newest wins" telemetry streams.
	SendOverflowDropOldest
)

// Session represents a connected client
type Session interface {
	ID() uuid.UUID
	RemoteAddr() net.Addr
	Send(data []byte) error
	Close() error
	// Context returns the session context
	Context() context.Context
	// SetContext sets the session context
	SetContext(ctx context.Context)
}

type session struct {
	id   uuid.UUID
	conn net.Conn
	// ctx is held in an atomic.Pointer so SetContext is safe to call from
	// middleware concurrently with the read loop calling Context().
	ctx atomic.Pointer[context.Context]

	writeTimeout   time.Duration
	overflowPolicy SendOverflowPolicy

	sendCh chan []byte
	doneCh chan struct{}

	closeOnce sync.Once
	closed    atomic.Bool
}

func newSession(conn net.Conn, writeBufferSize int, writeTimeout time.Duration, policy SendOverflowPolicy) *session {
	s := &session{
		id:             uuid.New(),
		conn:           conn,
		writeTimeout:   writeTimeout,
		overflowPolicy: policy,
		sendCh:         make(chan []byte, writeBufferSize),
		doneCh:         make(chan struct{}),
	}
	bg := context.Background()
	s.ctx.Store(&bg)
	return s
}

func (s *session) Start() {
	go s.writeLoop()
}

func (s *session) ID() uuid.UUID {
	return s.id
}

func (s *session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *session) Send(data []byte) error {
	if s.closed.Load() {
		return ErrSessionClosed
	}

	switch s.overflowPolicy {
	case SendOverflowBlock:
		select {
		case s.sendCh <- data:
			return nil
		case <-s.doneCh:
			return ErrSessionClosed
		}

	case SendOverflowDropOldest:
		for {
			select {
			case s.sendCh <- data:
				return nil
			case <-s.doneCh:
				return ErrSessionClosed
			default:
				// Drop the oldest queued message and retry — non-blocking
				// drain to avoid spinning when the writer is keeping up.
				select {
				case <-s.sendCh:
				default:
				}
			}
		}

	default: // SendOverflowError
		select {
		case s.sendCh <- data:
			return nil
		case <-s.doneCh:
			return ErrSessionClosed
		default:
			return ErrSendQueueFull
		}
	}
}

func (s *session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		close(s.doneCh)
		err = s.conn.Close()
	})
	return err
}

func (s *session) Context() context.Context {
	if p := s.ctx.Load(); p != nil {
		return *p
	}
	return context.Background()
}

func (s *session) SetContext(ctx context.Context) {
	s.ctx.Store(&ctx)
}

func (s *session) writeLoop() {
	for {
		select {
		case data := <-s.sendCh:
			if s.writeTimeout > 0 {
				s.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			}

			if _, err := s.conn.Write(data); err != nil {
				s.Close()
				return
			}

		case <-s.doneCh:
			return
		}
	}
}
