package udp

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

const (
	// ResendTimeout is how long we wait before resending a reliable packet
	ResendTimeout = 300 * time.Millisecond
	// MaxRetries is how many times we retry before dropping/disconnecting
	MaxRetries = 5

	// sessionInboundBuf is the per-session bounded inbound queue depth used
	// by the worker goroutine that dispatches to user Handler code. Bounded
	// to keep slow-handler memory blow-up local to one session rather than
	// the whole server.
	sessionInboundBuf = 64
)

// Session represents a connection-like state over UDP
type Session interface {
	ID() string
	RemoteAddr() net.Addr
	Context() context.Context

	// SendUnreliable sends data without reliability guarantees (for position updates)
	// Uses packet framing but no ACK/retry. Fast path for high-frequency updates.
	SendUnreliable(data []byte) error

	// SendReliable sends data with ACK and retry (for critical events)
	// Guaranteed delivery with retransmission on packet loss.
	SendReliable(data []byte) error

	// Send is an alias for SendUnreliable (backwards compatibility)
	Send(data []byte) error

	// Internal methods called by Server
	ProcessPacket(p Packet) error
	CheckResends()
	Close()
}

type session struct {
	id         string
	server     *Server
	remoteAddr *net.UDPAddr
	ctx        context.Context
	cancel     context.CancelFunc

	// Reliability State — guarded by mu
	mu          sync.Mutex
	nextSeq     uint16
	lastAckRecv uint16 // The highest Seq we've seen from remote (to Ack back)
	// pending is indexed by sequence number for O(1) ack removal — the
	// previous list-based scan was O(n) per ACK and allocated a new slice
	// on every call.
	pending map[uint16]*pendingPacket

	// lastSeenNano is updated on every received packet and inspected by the
	// server's reaper. Atomic so the reaper doesn't have to take mu.
	lastSeenNano atomic.Int64

	// inbound feeds the per-session dispatch worker. Bounded so a slow
	// handler can't allocate unbounded goroutines for the same session,
	// and per-session ordering is preserved (the old code spawned a fresh
	// goroutine per packet, randomising order).
	inbound chan []byte
	loopWG  sync.WaitGroup

	closed atomic.Bool
}

type pendingPacket struct {
	seq      uint16
	data     []byte
	sentAt   time.Time
	attempts int
}

func newSession(ctx context.Context, server *Server, addr *net.UDPAddr) *session {
	ctx, cancel := context.WithCancel(ctx)
	s := &session{
		id:         uuid.New().String(),
		server:     server,
		remoteAddr: addr,
		ctx:        ctx,
		cancel:     cancel,
		nextSeq:    1, // Start at 1
		pending:    make(map[uint16]*pendingPacket),
		inbound:    make(chan []byte, sessionInboundBuf),
	}
	s.lastSeenNano.Store(time.Now().UnixNano())

	s.loopWG.Add(1)
	go s.dispatchLoop()
	return s
}

func (s *session) ID() string {
	return s.id
}

func (s *session) RemoteAddr() net.Addr {
	return s.remoteAddr
}

func (s *session) Context() context.Context {
	return s.ctx
}

// touch records that we just heard from this peer; the reaper uses it to
// decide whether the session is idle.
func (s *session) touch() {
	s.lastSeenNano.Store(time.Now().UnixNano())
}

// idleFor returns how long it's been since the last received packet.
func (s *session) idleFor(now time.Time) time.Duration {
	last := s.lastSeenNano.Load()
	if last == 0 {
		return 0
	}
	return now.Sub(time.Unix(0, last))
}

// SendUnreliable sends data without reliability (packet framing, no ACK)
// Best for high-frequency position updates where newest data matters most.
func (s *session) SendUnreliable(data []byte) error {
	p := Packet{
		Type:    PacketTypeUnreliable,
		Seq:     0, // Unreliable doesn't track Seq
		Ack:     0,
		Payload: data,
	}
	// Piggyback ACK to help reliability on other side
	s.mu.Lock()
	p.Ack = s.lastAckRecv
	s.mu.Unlock()

	return s.server.writeTo(p.Marshal(), s.remoteAddr)
}

// Send is an alias for SendUnreliable for backwards compatibility
func (s *session) Send(data []byte) error {
	return s.SendUnreliable(data)
}

// SendReliable sends data with ACK and retry guarantees
// Best for critical events like combat, item pickups, state changes.
// Will retry up to MaxRetries times with ResendTimeout delay.
func (s *session) SendReliable(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	seq := s.nextSeq
	s.nextSeq++

	p := Packet{
		Type:    PacketTypeReliable,
		Seq:     seq,
		Ack:     s.lastAckRecv,
		Payload: data,
	}

	bytes := p.Marshal()

	s.pending[seq] = &pendingPacket{
		seq:      seq,
		data:     bytes,
		sentAt:   time.Now(),
		attempts: 1,
	}

	return s.server.writeTo(bytes, s.remoteAddr)
}

func (s *session) ProcessPacket(p Packet) error {
	s.touch()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Handle ACKs
	if p.Ack > 0 {
		s.handleAck(p.Ack)
	}

	// 2. Handle Payload
	// If it is Reliable, we update lastAckRecv.
	if p.Type == PacketTypeReliable {
		// Wrap-safe comparison: uint16 sequence numbers wrap around at
		// 65536. A naïve `p.Seq > s.lastAckRecv` would treat seq=3 as
		// older than seq=65530 forever after the first wrap. Casting the
		// difference to int16 produces the signed distance, which is
		// positive for "newer" and negative for "older" across the wrap.
		if int16(p.Seq-s.lastAckRecv) > 0 {
			s.lastAckRecv = p.Seq
		}
		// Send immediate ACK
		s.sendAck(p.Seq)
	}

	return nil
}

func (s *session) handleAck(ackSeq uint16) {
	delete(s.pending, ackSeq)
}

func (s *session) sendAck(seq uint16) {
	// Send a control ACK packet
	ackPkt := Packet{
		Type: PacketTypeAck,
		Seq:  0,
		Ack:  seq,
	}
	_ = s.server.writeTo(ackPkt.Marshal(), s.remoteAddr)
}

func (s *session) CheckResends() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, pkt := range s.pending {
		if now.Sub(pkt.sentAt) > ResendTimeout {
			if pkt.attempts >= MaxRetries {
				// Failed. Log or Disconnect.
				s.server.monitor.Logger().
					WithKeysAndValues("id", s.id, "seq", pkt.seq).
					ErrorContext(s.ctx, "Dropped packet after max retries")
				// Reset timer to avoid spam
				pkt.sentAt = now
				continue
			}

			// Resend
			pkt.attempts++
			pkt.sentAt = now
			_ = s.server.writeTo(pkt.data, s.remoteAddr)
		}
	}
}

// enqueue hands a payload to the dispatch loop without blocking the
// server's read loop. If the inbound queue is full the packet is dropped
// — preferable to head-of-line blocking the whole UDP read loop on one
// slow client.
func (s *session) enqueue(payload []byte) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case s.inbound <- payload:
		return true
	default:
		s.server.monitor.Logger().
			WithKeysAndValues("id", s.id, "addr", s.remoteAddr.String()).
			Warn("udp session inbound queue full, dropping packet")
		return false
	}
}

// dispatchLoop drains inbound and calls the user handler with panic
// recovery. One worker per session preserves per-session ordering.
func (s *session) dispatchLoop() {
	defer s.loopWG.Done()
	for payload := range s.inbound {
		s.dispatchOne(payload)
	}
}

func (s *session) dispatchOne(payload []byte) {
	ctx, span := s.server.monitor.Tracer().Start(s.ctx, "udp.handle",
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithAttributes(
			tracer.String("net.transport", "udp"),
			tracer.String("net.peer.addr", s.remoteAddr.String()),
			tracer.Int("net.payload.bytes", len(payload)),
		),
	)
	defer span.End()

	defer func() {
		if r := recover(); r != nil {
			s.server.monitor.Logger().
				WithKeysAndValues("id", s.id, "addr", s.remoteAddr.String(), "panic", r).
				ErrorContext(ctx, "udp handler panic")
			span.SetStatus(tracer.StatusError, "handler panic")
		}
	}()
	if err := s.server.handler.Handle(ctx, s, payload); err != nil {
		s.server.monitor.Logger().
			WithKeysAndValues("id", s.id, "error", err).
			Error("udp handler error")
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
	}
}

func (s *session) Close() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	close(s.inbound)
	s.loopWG.Wait()
	s.cancel()
	s.server.sessions.Delete(s.remoteAddr.String())
}
