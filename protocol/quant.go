package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
)

// WriteQuantFloat writes f as a quantised value using bits ∈ {16, 32}.
// 32-bit is IEEE 754 raw; 16-bit is bfloat16 (top 16 bits of float32),
// chosen because both endpoints round-trip identically.
func WriteQuantFloat(buf *bytes.Buffer, f float32, bits uint8) {
	switch bits {
	case 32:
		_ = binary.Write(buf, binary.LittleEndian, math.Float32bits(f))
	case 16:
		raw := math.Float32bits(f)
		// bfloat16: take the top 16 bits.
		bf := uint16(raw >> 16)
		_ = binary.Write(buf, binary.LittleEndian, bf)
	default:
		panic("protocol: WriteQuantFloat: bits must be 16 or 32")
	}
}

// ReadQuantFloat reads a value previously written with WriteQuantFloat.
func ReadQuantFloat(r *bytes.Reader, bits uint8) (float32, error) {
	switch bits {
	case 32:
		var raw uint32
		if err := binary.Read(r, binary.LittleEndian, &raw); err != nil {
			if err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		return math.Float32frombits(raw), nil
	case 16:
		var bf uint16
		if err := binary.Read(r, binary.LittleEndian, &bf); err != nil {
			if err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		return math.Float32frombits(uint32(bf) << 16), nil
	default:
		panic("protocol: ReadQuantFloat: bits must be 16 or 32")
	}
}
