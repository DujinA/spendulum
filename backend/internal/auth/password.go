package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// AC 12 of issue 28: argon2id with m=64 MiB, t=3, p=4.
const (
	argonMemoryKiB = 64 * 1024
	argonTime      = 3
	argonThreads   = 4
	argonSaltLen   = 16
	argonKeyLen    = 32
)

// Each hash takes 64 MiB, and the pod's memory limit is small, so only two run at once.
const maxConcurrentHashes = 2

var errMalformedHash = errors.New("auth: malformed password hash")

type Hasher struct {
	slots chan struct{}
}

func NewHasher() *Hasher {
	return &Hasher{slots: make(chan struct{}, maxConcurrentHashes)}
}

func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read never returns an error

	key, err := h.key(ctx, password, salt, argonMemoryKiB, argonTime, argonThreads, argonKeyLen)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func (h *Hasher) Verify(ctx context.Context, hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false, errMalformedHash
	}

	var (
		memory, time uint32
		threads      uint8
	)

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errMalformedHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errMalformedHash
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errMalformedHash
	}

	got, err := h.key(ctx, password, salt, memory, time, threads, uint32(len(want))) //nolint:gosec // decoded keys are 32 bytes
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func (h *Hasher) key(ctx context.Context, password string, salt []byte, memory, time uint32, threads uint8, keyLen uint32) ([]byte, error) {
	select {
	case h.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("auth: wait to hash: %w", ctx.Err())
	}
	defer func() { <-h.slots }()

	return argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen), nil
}

// dummyHash has the real parameters, so verifying against it costs what a real check costs.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
