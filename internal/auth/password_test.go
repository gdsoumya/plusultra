package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// phc builds a hash with arbitrary parameters, bypassing HashPassword's fixed ones.
func phc(pw string, salt []byte, m, t uint32, p uint8) string {
	key := argon2.IDKey([]byte(pw), salt, t, m, p, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", m, t, p,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func TestHashVerify(t *testing.T) {
	const pw = "correct horse battery"
	ctx := t.Context()
	h, err := HashPassword(ctx, pw)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "$argon2id$v=19$m=65536,t=3,p=2$"
	if !strings.HasPrefix(h, prefix) {
		t.Fatalf("hash %q is not PHC argon2id with the required params", h)
	}
	salt, _, _ := strings.Cut(strings.TrimPrefix(h, prefix), "$")
	if b, err := base64.RawStdEncoding.DecodeString(salt); err != nil || len(b) != 16 {
		t.Fatalf("salt %q: %d bytes, err %v; want 16 bytes", salt, len(b), err)
	}
	if !VerifyPassword(ctx, h, pw) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(ctx, h, pw+"!") {
		t.Fatal("wrong password accepted")
	}
	if h2, _ := HashPassword(ctx, pw); h2 == h {
		t.Fatal("two hashes of the same password are equal; salt not random")
	}

	salt16 := []byte("0123456789abcdef")
	cheap := phc(pw, salt16, 64, 1, 1)
	if !VerifyPassword(ctx, cheap, pw) {
		t.Fatal("parameters are not read from the stored hash")
	}
	for name, bad := range map[string]string{
		"empty":         "",
		"plain":         "plain",
		"bad base64":    "$argon2id$v=19$m=65536,t=3,p=2$!!$!!",
		"argon2i":       strings.Replace(cheap, "$argon2id$", "$argon2i$", 1),
		"trailing junk": strings.Replace(cheap, "p=1$", "p=1x$", 1),
		"short salt":    phc(pw, []byte("salt"), 64, 1, 1),
		"memory > cap":  phc(pw, salt16, 256*1024+4, 1, 1),
	} {
		if VerifyPassword(ctx, bad, pw) {
			t.Fatalf("%s: hash %q verified", name, bad)
		}
	}
}

func TestHashWaitsRespectContext(t *testing.T) {
	for range cap(hashSlots) {
		hashSlots <- struct{}{}
	}
	defer func() {
		for range cap(hashSlots) {
			<-hashSlots
		}
	}()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := HashPassword(ctx, "correct horse battery"); err == nil {
		t.Fatal("HashPassword ignored a cancelled context while all slots were busy")
	}
	if VerifyPassword(ctx, phc("x", []byte("0123456789abcdef"), 64, 1, 1), "x") {
		t.Fatal("VerifyPassword succeeded on a cancelled context while all slots were busy")
	}
}
