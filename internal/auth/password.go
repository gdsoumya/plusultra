package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime      = 3
	argonMemory    = 64 * 1024 // KiB
	argonThreads   = 2
	argonKeyLen    = 32
	saltLen        = 16
	maxArgonMemory = 256 * 1024 // KiB
	minSaltLen     = 8
)

var b64 = base64.RawStdEncoding

// Each hash holds 64 MiB, so a flood of logins must queue rather than exhaust memory.
var hashSlots = make(chan struct{}, 4)

func argonKey(ctx context.Context, pw string, salt []byte, t, m uint32, p uint8, keyLen uint32) ([]byte, error) {
	select {
	case hashSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-hashSlots }()
	return argon2.IDKey([]byte(pw), salt, t, m, p, keyLen), nil
}

// HashPassword returns an argon2id hash of pw in PHC string format.
func HashPassword(ctx context.Context, pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := argonKey(ctx, pw, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether pw matches hash, using the parameters stored in
// hash. It reports false if ctx ends while waiting for a hashing slot.
func VerifyPassword(ctx context.Context, hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}
	var m, t uint32
	var p uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
	// Printing the values back rejects the trailing junk Sscanf ignores.
	if err != nil || parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", m, t, p) || t == 0 || p == 0 || m > maxArgonMemory {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < minSaltLen {
		return false
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := argonKey(ctx, pw, salt, t, m, p, uint32(len(want))) //nolint:gosec // len of a decoded hash, far below 2^32
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
