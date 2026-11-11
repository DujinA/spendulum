package auth_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/DujinA/spendulum/backend/internal/auth"
)

func TestPasswordHashUsesArgon2idWithM64MiBT3P4(t *testing.T) {
	hash, err := auth.NewHasher().Hash(t.Context(), "correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=4" {
		t.Fatalf("hash = %q, want $argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>", hash)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 {
		t.Fatalf("salt %q: %v, want at least 16 bytes", parts[4], err)
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		t.Fatalf("key %q: %v", parts[5], err)
	}

	if want := argon2.IDKey([]byte("correct horse battery"), salt, 3, 64*1024, 4, uint32(len(key))); !bytes.Equal(key, want) {
		t.Error("the key is not argon2id(m=64 MiB, t=3, p=4) of the password and salt")
	}
}

func TestVerifyAcceptsOnlyThePasswordThatWasHashed(t *testing.T) {
	h := auth.NewHasher()

	hash, err := h.Hash(t.Context(), "correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	for pw, want := range map[string]bool{"correct horse battery": true, "correct horse batterY": false, "": false} {
		if got, err := h.Verify(t.Context(), hash, pw); err != nil || got != want {
			t.Errorf("Verify(%q) = %v, %v; want %v", pw, got, err, want)
		}
	}
}

func TestTwoHashesOfOnePasswordDiffer(t *testing.T) {
	h := auth.NewHasher()

	a, _ := h.Hash(t.Context(), "correct horse battery")
	b, _ := h.Hash(t.Context(), "correct horse battery")

	if a == b {
		t.Error("two hashes of one password are equal, want a fresh salt each time")
	}
}
