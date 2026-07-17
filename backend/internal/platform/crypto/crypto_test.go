package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, err := NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

func TestCipherRoundTrip(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("telegram-bot-token:123456:secret")

	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, dec) {
		t.Fatalf("round trip mismatch: got %q", dec)
	}
}

func TestCipherProducesDistinctCiphertexts(t *testing.T) {
	c := newTestCipher(t)
	a, _ := c.EncryptString("same")
	b, _ := c.EncryptString("same")
	if a == b {
		t.Fatal("expected distinct ciphertexts due to random nonce")
	}
}

func TestCipherRejectsTampering(t *testing.T) {
	c := newTestCipher(t)
	enc, _ := c.EncryptString("secret")
	// Flip the first character to one it is definitely not. Hardcoding "A" made this
	// tamper with nothing whenever the ciphertext already began with 'A' — about one
	// run in 64, since it opens with a random nonce — and the test then decrypted the
	// UNCHANGED string, failed, and got re-run. A security assertion that passes 98%
	// of the time is one people learn to retry, which is how a real regression hides.
	flip := byte('A')
	if enc[0] == flip {
		flip = 'B'
	}
	tampered := string(flip) + enc[1:]
	if _, err := c.DecryptString(tampered); err == nil {
		t.Fatal("expected decryption of tampered ciphertext to fail")
	}
}

func TestNewCipherRejectsWrongKeySize(t *testing.T) {
	if _, err := NewCipher([]byte("short")); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}

func TestPasswordHasher(t *testing.T) {
	h := NewPasswordHasher(4) // low cost for fast tests
	hash, err := h.Hash("s3cret-password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := h.Verify(hash, "s3cret-password")
	if err != nil || !ok {
		t.Fatalf("Verify correct password: ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify(hash, "wrong-password")
	if err != nil {
		t.Fatalf("Verify wrong password errored: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail verification")
	}
}
