package crypto

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher hashes and verifies passwords using bcrypt. The cost is
// configurable to trade off security against CPU under load; the default is a
// sensible production value.
type PasswordHasher struct {
	cost int
}

// DefaultBcryptCost balances security and login latency for a web application.
const DefaultBcryptCost = 12

// NewPasswordHasher builds a hasher. A cost < bcrypt.MinCost falls back to the
// default.
func NewPasswordHasher(cost int) *PasswordHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = DefaultBcryptCost
	}
	return &PasswordHasher{cost: cost}
}

// Hash returns the bcrypt hash of the given password.
func (h *PasswordHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify reports whether password matches the stored hash. It returns false
// (not an error) on mismatch so callers can treat it as a boolean, and an error
// only for malformed hashes.
func (h *PasswordHasher) Verify(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, err
}
