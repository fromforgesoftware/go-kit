// Package udp provides a UDP transport implementation with optional reliability.
//
// # Overview
//
// This package offers flexible UDP communication with two packet delivery modes:
//   - Unreliable: Fast, fire-and-forget (for position updates)
//   - Reliable: Guaranteed delivery with ACKs and retries (for critical events)
//
// # Client Usage
//
// For game clients sending high-frequency updates:
//
//	client, _ := udp.NewClient("server:8080",
//	    udp.WithClientReadTimeout(0),    // No timeout for games
//	    udp.WithClientWriteTimeout(0),
//	)
//	defer client.Close()
//
//	// Fast path for position updates (60+ Hz)
//	positionData := encodePosition(player)
//	client.SendRaw(positionData)  // Lock-free, no packet framing
//
//	// With packet framing but still fast
//	client.Send(positionData)  // Unreliable with 5-byte header
//
//	// Zero-copy receive
//	buf := make([]byte, 1500)  // MTU size
//	n, _ := client.ReceiveInto(buf)
//	handlePacket(buf[:n])
//
// # Server Usage with Sessions
//
// For game servers managing player sessions:
//
//	handler := udp.HandlerFunc(func(ctx context.Context, sess udp.Session, data []byte) error {
//	    switch packetType(data) {
//	    case TypePosition:
//	        // High frequency, okay to drop
//	        return sess.SendUnreliable(response)
//
//	    case TypeAttack:
//	        // Critical event, needs guarantee
//	        return sess.SendReliable(response)
//	    }
//	    return nil
//	})
//
//	server, _ := udp.NewServer(monitor,
//	    udp.WithAddress(":8080"),
//	    udp.WithHandler(handler),
//	)
//	server.Start()
//
// # Session API
//
// Sessions provide two send methods with clear semantics:
//
//   - SendUnreliable(data): Fast path, no ACK, for position updates
//   - SendReliable(data): Guaranteed delivery with retries, for combat
//
// # Performance Characteristics
//
//	Method              Latency    Memory    Use Case
//	SendRaw()          ~5-8 µs    56 B/op   Position updates (60+ Hz)
//	SendUnreliable()   ~17 µs     569 B/op  General unreliable packets
//	SendReliable()     ~17 µs     569 B/op  Critical events with retry
//
// # Reliability Features
//
// Reliable packets include:
//   - Sequence numbers for ordering
//   - ACK packets for confirmation
//   - Automatic retransmission (up to MaxRetries)
//   - Configurable timeout (ResendTimeout)
//
// # Packet Format
//
// All packets (except SendRaw) use a 5-byte header:
//
//	[Type:1][Seq:2][Ack:2][Payload...]
//
//	Type: PacketTypeUnreliable, PacketTypeReliable, PacketTypeAck
//	Seq:  Sequence number (0 for unreliable)
//	Ack:  Piggybacked acknowledgment
//
// # Best Practices for Games
//
//  1. Use SendRaw() for position updates (60-120 Hz)
//  2. Use SendReliable() for combat events, item pickups
//  3. Pre-allocate buffers for ReceiveInto() to avoid allocations
//  4. Disable timeouts (set to 0) for lowest latency
//
// # Migration from Generic API
//
// Old code:
//
//	sess.Send(data)  // Was unreliable
//
// New code (more explicit):
//
//	sess.SendUnreliable(data)  // Same behavior, clearer intent
//
// The old Send() method still works as an alias for SendUnreliable().
package udp
