package password

import "testing"

func TestArgon2id_HashThenVerify(t *testing.T) {
	h := NewArgon2id()

	hp, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hp.Algo != "argon2id" {
		t.Fatalf("algo = %q, want argon2id", hp.Algo)
	}
	if hp.Encoded == "" {
		t.Fatal("encoded hash is empty")
	}

	ok, err := h.Verify("correct horse battery staple", hp.Encoded)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("Verify returned false for the correct password")
	}

	bad, err := h.Verify("wrong password", hp.Encoded)
	if err != nil {
		t.Fatalf("Verify(wrong): %v", err)
	}
	if bad {
		t.Fatal("Verify returned true for the wrong password")
	}
}

func TestArgon2id_SaltIsRandom(t *testing.T) {
	h := NewArgon2id()
	a, err := h.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	b, err := h.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash b: %v", err)
	}
	if a.Encoded == b.Encoded {
		t.Fatal("two hashes of the same password are identical — salt is not random")
	}
}
