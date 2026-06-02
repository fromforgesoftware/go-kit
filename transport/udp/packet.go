package udp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PacketType identifies the reliability mode or control message
type PacketType uint8

const (
	// PacketTypeUnreliable is fire-and-forget. Used for movement updates.
	// We still use Seq to discard out-of-order/old packets.
	PacketTypeUnreliable PacketType = iota

	// PacketTypeReliable requires an ACK. Retransmitted if not acked.
	// Used for combat events, interactions, etc. where state must match.
	PacketTypeReliable

	// PacketTypeAck is a control packet acknowledging a Reliable packet.
	PacketTypeAck
)

const (
	// HeaderSize: [Type:1][Seq:2][Ack:2]
	HeaderSize = 5
)

var (
	ErrPacketTooSmall = errors.New("packet too small for header")
)

// Packet represents a decoded UDP packet
type Packet struct {
	Type    PacketType // 1 byte
	Seq     uint16     // 2 bytes (Sequence Number of THIS packet)
	Ack     uint16     // 2 bytes (The Sequence Number we are ACKing, or piggybacking)
	Payload []byte
}

// Marshal encodes the packet into a byte slice
func (p Packet) Marshal() []byte {
	size := HeaderSize + len(p.Payload)
	buf := make([]byte, size)

	buf[0] = byte(p.Type)
	binary.LittleEndian.PutUint16(buf[1:3], p.Seq)
	binary.LittleEndian.PutUint16(buf[3:5], p.Ack)

	if len(p.Payload) > 0 {
		copy(buf[5:], p.Payload)
	}

	return buf
}

// Unmarshal decodes a byte slice into a Packet
func Unmarshal(data []byte) (Packet, error) {
	if len(data) < HeaderSize {
		return Packet{}, ErrPacketTooSmall
	}

	p := Packet{
		Type: PacketType(data[0]),
		Seq:  binary.LittleEndian.Uint16(data[1:3]),
		Ack:  binary.LittleEndian.Uint16(data[3:5]),
	}

	if len(data) > HeaderSize {
		// Copy payload to avoid memory aliasing if buffer is reused
		p.Payload = make([]byte, len(data)-HeaderSize)
		copy(p.Payload, data[HeaderSize:])
	}

	return p, nil
}

// String returns a string representation of the packet
func (p Packet) String() string {
	typeStr := "UNKNOWN"
	switch p.Type {
	case PacketTypeUnreliable:
		typeStr = "UNREL"
	case PacketTypeReliable:
		typeStr = "REL"
	case PacketTypeAck:
		typeStr = "ACK"
	}
	return fmt.Sprintf("[%s Seq:%d Ack:%d Size:%d]", typeStr, p.Seq, p.Ack, len(p.Payload))
}
