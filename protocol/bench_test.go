package protocol_test

import (
	"bytes"
	"testing"

	"github.com/fromforgesoftware/go-kit/protocol"
)

func BenchmarkCodecEncodeNoPayload(b *testing.B) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	f := protocol.Frame{Magic: [2]byte{'F', 'G'}, Version: 1, Seq: 42, Opcode: 0x10}
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = c.Encode(f, buf)
	}
}

func BenchmarkCodecEncodeWithPayload(b *testing.B) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	payload := make([]byte, 64)
	f := protocol.Frame{Magic: [2]byte{'F', 'G'}, Version: 1, Seq: 42, Opcode: 0x10, Payload: payload}
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = c.Encode(f, buf)
	}
}

func BenchmarkCodecDecodeNoPayload(b *testing.B) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	var buf bytes.Buffer
	_ = c.Encode(protocol.Frame{Magic: [2]byte{'F', 'G'}, Version: 1, Seq: 42, Opcode: 0x10}, &buf)
	raw := buf.Bytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Decode(bytes.NewReader(raw))
	}
}

func BenchmarkCodecDecodeIntoPooled(b *testing.B) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	pool := protocol.NewReaderPool()
	var buf bytes.Buffer
	_ = c.Encode(protocol.Frame{Magic: [2]byte{'F', 'G'}, Version: 1, Seq: 42, Opcode: 0x10, Payload: make([]byte, 32)}, &buf)
	raw := buf.Bytes()
	payload := make([]byte, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := pool.Get(raw)
		_, _ = c.DecodeInto(r, payload)
		pool.Put(r)
	}
}

func BenchmarkVarUintRoundTrip(b *testing.B) {
	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		protocol.WriteVarUint(&buf, uint64(i))
		_, _ = protocol.ReadVarUint(bytes.NewReader(buf.Bytes()))
	}
}
