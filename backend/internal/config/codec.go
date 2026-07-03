package config

import (
	"encoding/base64"
	"encoding/hex"
)

// hexDecode decodes a hex string, returning errBadEncoding on failure so the
// caller can fall through to other encodings.
func hexDecode(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, errBadEncoding
	}
	return b, nil
}

// base64Decode decodes a standard base64 string.
func base64Decode(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadEncoding
	}
	return b, nil
}
