// Package password hashes and verifies passwords with argon2id (PHC-encoded).
// It is the shared hasher for any forge service that stores local credentials.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Hashed is the result of hashing a plaintext password: the PHC-encoded
// argon2id string plus the algorithm + params for storage/audit.
type Hashed struct {
	Encoded string
	Algo    string
	Params  map[string]any
}

// Hasher hashes and verifies passwords. The Argon2id implementation is the
// default; the interface keeps callers testable and the algorithm swappable.
type Hasher interface {
	Hash(password string) (Hashed, error)
	Verify(password, encoded string) (bool, error)
}

type argon2Params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
}

func defaultArgon2Params() argon2Params {
	return argon2Params{memory: 64 * 1024, iterations: 3, parallelism: 2, saltLen: 16, keyLen: 32}
}

// Argon2id implements Hasher with the OWASP-recommended argon2id parameters
// and the standard PHC string encoding.
type Argon2id struct {
	p argon2Params
}

func NewArgon2id() *Argon2id {
	return &Argon2id{p: defaultArgon2Params()}
}

func (h *Argon2id) Hash(password string) (Hashed, error) {
	salt := make([]byte, h.p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return Hashed{}, err
	}
	key := argon2.IDKey([]byte(password), salt, h.p.iterations, h.p.memory, h.p.parallelism, h.p.keyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.memory, h.p.iterations, h.p.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return Hashed{
		Encoded: encoded,
		Algo:    "argon2id",
		Params: map[string]any{
			"m": h.p.memory, "t": h.p.iterations, "p": h.p.parallelism,
			"saltLen": h.p.saltLen, "keyLen": h.p.keyLen,
		},
	}, nil
}

func (h *Argon2id) Verify(password, encoded string) (bool, error) {
	p, salt, key, err := decodeArgon2id(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, uint32(len(key)))
	return subtle.ConstantTimeCompare(key, other) == 1, nil
}

func decodeArgon2id(encoded string) (argon2Params, []byte, []byte, error) {
	// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, errors.New("invalid argon2id hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argon2Params{}, nil, nil, err
	}
	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return argon2Params{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}
	return p, salt, key, nil
}
