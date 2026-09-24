package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the hex-encoded SHA-256 of s. It is the stored representation
// of a high-entropy capability token (e.g. a GitHub Actions ingest token): the same
// function is used to hash the token when it is minted and to look it up when a
// request presents it, so the two can never disagree about "the hash of this token".
//
// SHA-256 and not bcrypt, deliberately — the token is 256 bits of CSPRNG output, so
// there is nothing to brute-force, and a slow hash would be paid on every ingest.
// Same reasoning as the api_keys table.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
