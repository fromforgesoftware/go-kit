package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Opcode identifies a payload type within a frame.
type Opcode uint16

// Frame is the wire-level message: a 10-byte header followed by Payload.
//
//	bytes  | field
//	-------+----------
//	0..1   | Magic (uint8 × 2, e.g. "FG")
//	2      | Version (uint8)
//	3      | Flags   (uint8)
//	4..7   | Seq     (uint32, little-endian)
//	8..9   | Opcode  (uint16, little-endian)
//	...    | Payload (Length implicit from outer frame size)
type Frame struct {
	Magic   [2]byte
	Version uint8
	Flags   uint8
	Seq     uint32
	Opcode  Opcode
	Payload []byte
}

const headerSize = 10

// Codec encodes / decodes Frames against a fixed Magic + Version. Wrong
// magic or version on decode returns an error.
type Codec struct {
	magic   [2]byte
	version uint8
}

// NewCodec returns a Codec for the given magic + version pair.
func NewCodec(magic [2]byte, version uint8) *Codec {
	return &Codec{magic: magic, version: version}
}

// Encode writes a Frame into out using little-endian byte order.
func (c *Codec) Encode(f Frame, out *bytes.Buffer) error {
	if out == nil {
		return errors.New("protocol: nil buffer")
	}
	var hdr [headerSize]byte
	hdr[0] = c.magic[0]
	hdr[1] = c.magic[1]
	hdr[2] = c.version
	hdr[3] = f.Flags
	binary.LittleEndian.PutUint32(hdr[4:8], f.Seq)
	binary.LittleEndian.PutUint16(hdr[8:10], uint16(f.Opcode))
	if _, err := out.Write(hdr[:]); err != nil {
		return err
	}
	if len(f.Payload) > 0 {
		if _, err := out.Write(f.Payload); err != nil {
			return err
		}
	}
	return nil
}

// Decode reads a Frame from in. Consumes the entire remaining buffer as
// payload (caller controls outer framing for stream transports). The
// returned Frame.Payload is a freshly-allocated slice; callers that need
// zero-allocation should use DecodeInto.
func (c *Codec) Decode(in *bytes.Reader) (Frame, error) {
	return c.DecodeInto(in, nil)
}

// DecodeInto reads a Frame from in, reusing payloadBuf for the payload
// allocation. If payloadBuf has sufficient capacity, no allocation
// occurs; otherwise a new slice grows behind it. Pass nil to behave like
// Decode (allocates a fresh slice each call).
func (c *Codec) DecodeInto(in *bytes.Reader, payloadBuf []byte) (Frame, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(in, hdr[:]); err != nil {
		return Frame{}, fmt.Errorf("protocol: read header: %w", err)
	}
	if hdr[0] != c.magic[0] || hdr[1] != c.magic[1] {
		return Frame{}, fmt.Errorf("protocol: magic mismatch")
	}
	if hdr[2] != c.version {
		return Frame{}, fmt.Errorf("protocol: version mismatch: got %d want %d", hdr[2], c.version)
	}
	f := Frame{
		Magic:   c.magic,
		Version: c.version,
		Flags:   hdr[3],
		Seq:     binary.LittleEndian.Uint32(hdr[4:8]),
		Opcode:  Opcode(binary.LittleEndian.Uint16(hdr[8:10])),
	}
	if remaining := in.Len(); remaining > 0 {
		if cap(payloadBuf) >= remaining {
			f.Payload = payloadBuf[:remaining]
		} else {
			f.Payload = make([]byte, remaining)
		}
		if _, err := io.ReadFull(in, f.Payload); err != nil {
			return Frame{}, fmt.Errorf("protocol: read payload: %w", err)
		}
	}
	return f, nil
}

// ReaderPool pools *bytes.Reader instances so callers can decode many
// frames without allocating a new Reader per call. Pair with DecodeInto
// + a recycled payload buffer for fully zero-alloc decoding.
type ReaderPool struct {
	pool sync.Pool
}

// NewReaderPool returns an empty pool. The pool is safe for concurrent use.
func NewReaderPool() *ReaderPool {
	return &ReaderPool{pool: sync.Pool{
		New: func() any { return bytes.NewReader(nil) },
	}}
}

// Get returns a Reader positioned over data. Always return it via Put.
func (p *ReaderPool) Get(data []byte) *bytes.Reader {
	r := p.pool.Get().(*bytes.Reader)
	r.Reset(data)
	return r
}

// Put returns a Reader to the pool.
func (p *ReaderPool) Put(r *bytes.Reader) {
	r.Reset(nil)
	p.pool.Put(r)
}
