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
	// Flip a character in the base64 to simulate tampering.
	tampered := "A" + enc[1:]
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
