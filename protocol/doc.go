// Package protocol provides a wire-protocol foundation for custom binary
// protocols: framed messages with versioned magic + sequence + opcode,
// encoding helpers (varint, quantised float), and a typed opcode
// registry where projects bind their per-opcode payload codecs.
//
// Domain-agnostic; common applications:
//   - Custom UDP / TCP application protocols.
//   - Market-data feeds with fixed-shape per-tick messages.
//   - Sensor / IoT telemetry with bandwidth-constrained encodings.
//   - Cross-language protocols requiring byte-for-byte parity (the
//     frame format is stable; golden vectors under kit/protocol/golden/
//     support cross-language parity testing — for example, against a
//     matching C++ implementation in a paired client SDK).
package protocol
