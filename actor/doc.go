// Package actor provides the owner-goroutine + inbox actor pattern: a
// single-writer aggregate that receives typed messages from any sender,
// drains them at known points, and never shares state across goroutines.
//
// Three primitives compose: Mailbox (channel-based MPSC), Actor (Mailbox
// + Behavior loop), Supervisor (crash-isolated restart policy).
package actor
