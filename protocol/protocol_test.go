package protocol_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/protocol"
)

func TestFrameRoundTrip(t *testing.T) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	in := protocol.Frame{
		Magic:   [2]byte{'F', 'G'},
		Version: 1,
		Flags:   0x42,
		Seq:     12345,
		Opcode:  0x0010,
		Payload: []byte{1, 2, 3, 4, 5},
	}

	var buf bytes.Buffer
	require.NoError(t, c.Encode(in, &buf))

	got, err := c.Decode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestCodecRejectsMagic(t *testing.T) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	// Manually craft a frame with wrong magic.
	bad := []byte{'X', 'X', 1, 0, 0, 0, 0, 0, 0, 0}
	_, err := c.Decode(bytes.NewReader(bad))
	assert.Error(t, err)
}

func TestCodecRejectsVersion(t *testing.T) {
	c := protocol.NewCodec([2]byte{'F', 'G'}, 1)
	bad := []byte{'F', 'G', 2, 0, 0, 0, 0, 0, 0, 0}
	_, err := c.Decode(bytes.NewReader(bad))
	assert.Error(t, err)
}

func TestVarUintRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 127, 128, 16383, 16384, 1 << 32, 1 << 63} {
		var buf bytes.Buffer
		protocol.WriteVarUint(&buf, v)
		got, err := protocol.ReadVarUint(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, v, got)
	}
}

func TestQuantFloat32(t *testing.T) {
	var buf bytes.Buffer
	protocol.WriteQuantFloat(&buf, 3.14159, 32)
	got, err := protocol.ReadQuantFloat(bytes.NewReader(buf.Bytes()), 32)
	require.NoError(t, err)
	assert.InDelta(t, float32(3.14159), got, 1e-6)
}

func TestQuantFloat16Bfloat(t *testing.T) {
	var buf bytes.Buffer
	protocol.WriteQuantFloat(&buf, 1.5, 16)
	got, err := protocol.ReadQuantFloat(bytes.NewReader(buf.Bytes()), 16)
	require.NoError(t, err)
	// bfloat16 has ~8 bits of mantissa; small power-of-two values are exact.
	assert.InDelta(t, float32(1.5), got, 0.01)
}

type spawnPayload struct {
	EntityID uint64
	X, Y, Z  float32
}

func TestRegistryRoundTrip(t *testing.T) {
	r := protocol.NewRegistry()
	protocol.Register[spawnPayload](r, 0x10, "spawn",
		func(p spawnPayload, buf *bytes.Buffer) {
			protocol.WriteVarUint(buf, p.EntityID)
			protocol.WriteQuantFloat(buf, p.X, 32)
			protocol.WriteQuantFloat(buf, p.Y, 32)
			protocol.WriteQuantFloat(buf, p.Z, 32)
		},
		func(rd *bytes.Reader) (spawnPayload, error) {
			id, err := protocol.ReadVarUint(rd)
			if err != nil {
				return spawnPayload{}, err
			}
			x, _ := protocol.ReadQuantFloat(rd, 32)
			y, _ := protocol.ReadQuantFloat(rd, 32)
			z, _ := protocol.ReadQuantFloat(rd, 32)
			return spawnPayload{EntityID: id, X: x, Y: y, Z: z}, nil
		})

	payload, err := r.EncodePayload(0x10, spawnPayload{EntityID: 99, X: 1, Y: 2, Z: 3})
	require.NoError(t, err)

	got, err := r.DecodePayload(0x10, payload)
	require.NoError(t, err)
	assert.Equal(t, spawnPayload{EntityID: 99, X: 1, Y: 2, Z: 3}, got)
}

func TestRegistryUnknownOpcode(t *testing.T) {
	r := protocol.NewRegistry()
	_, err := r.EncodePayload(0xFFFF, struct{}{})
	assert.ErrorIs(t, err, protocol.ErrUnknownOpcode)
}
