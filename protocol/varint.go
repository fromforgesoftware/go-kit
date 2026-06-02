package protocol

import (
	"bytes"
	"errors"
	"io"
)

// WriteVarUint emits v as an unsigned LEB128 varint. Matches the encoding
// used by Google protobuf wire format (and our C++ side).
func WriteVarUint(buf *bytes.Buffer, v uint64) {
	for v >= 0x80 {
		buf.WriteByte(byte(v) | 0x80)
		v >>= 7
	}
	buf.WriteByte(byte(v))
}

// ReadVarUint reads an unsigned LEB128 varint.
func ReadVarUint(r *bytes.Reader) (uint64, error) {
	var result uint64
	var shift uint
	for i := 0; i < 10; i++ {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}
		result |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return result, nil
		}
		shift += 7
	}
	return 0, errors.New("protocol: varint too long")
}
