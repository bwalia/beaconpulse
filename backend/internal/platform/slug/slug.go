// Package slug converts human names into URL- and DNS-safe slugs matching the
// database CHECK constraint ^[a-z0-9][a-z0-9-]{0,62}$. It is deterministic and
// dependency-free so it can be unit tested in isolation.
package slug

import (
	"strings"
	"unicode"
)

// Make returns a slug derived from s: lowercased, non-alphanumeric runs
// collapsed to single hyphens, trimmed to 63 characters, with no leading or
// trailing hyphen. If nothing usable remains it returns fallback.
func Make(s, fallback string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII, unicode.IsDigit(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	// Leading char must be alphanumeric; the trim above guarantees this unless
	// the string is empty.
	if out == "" {
		return fallback
	}
	return out
}
