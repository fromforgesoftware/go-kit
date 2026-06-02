package actor

import (
	"context"
	"errors"
)

// ErrMailboxFull is returned when a non-blocking Send is attempted on a
// full mailbox.
var ErrMailboxFull = errors.New("actor: mailbox full")

// Mailbox is an MPSC queue of messages of type M. Senders call Send
// (returns ErrMailboxFull if blocked); the owner calls Drain to bulk-pull.
type Mailbox[M any] struct {
	ch chan M
}

// NewMailbox returns a Mailbox with the given buffer capacity.
func NewMailbox[M any](capacity int) *Mailbox[M] {
	return &Mailbox[M]{ch: make(chan M, capacity)}
}

// Send delivers a message. Returns ErrMailboxFull if the mailbox is full
// and the context is not blocking, or ctx.Err() if cancelled.
func (mb *Mailbox[M]) Send(ctx context.Context, msg M) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case mb.ch <- msg:
		return nil
	default:
	}
	// Mailbox full: respect context.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case mb.ch <- msg:
		return nil
	}
}

// TrySend attempts a non-blocking send.
func (mb *Mailbox[M]) TrySend(msg M) error {
	select {
	case mb.ch <- msg:
		return nil
	default:
		return ErrMailboxFull
	}
}

// Drain pulls all currently-available messages into the caller-supplied
// slice (reused for allocation-free polling). Returns when the channel is
// drained or ctx is cancelled.
func (mb *Mailbox[M]) Drain(buf *[]M) {
	*buf = (*buf)[:0]
	for {
		select {
		case msg := <-mb.ch:
			*buf = append(*buf, msg)
		default:
			return
		}
	}
}

// Close signals senders no more messages will be processed. After Close,
// Send panics; Drain still drains remaining messages.
func (mb *Mailbox[M]) Close() { close(mb.ch) }

// Cap returns the mailbox capacity.
func (mb *Mailbox[M]) Cap() int { return cap(mb.ch) }
