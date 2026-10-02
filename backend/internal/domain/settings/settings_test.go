package settings

import "testing"

func TestValidSupportEmail(t *testing.T) {
	for in, want := range map[string]bool{
		"":                              true, // unset — the web app uses its default
		"support@sysops247.com":         true,
		"  help@example.co.uk ":         true,
		"not-an-email":                  false,
		"Support <support@example.com>": false, // display names don't belong in a mailto:
		"mailto:support@example.com":    false,
		"a@b.com\r\nBcc: x@evil.com":    false,
	} {
		if got := validSupportEmail(in); got != want {
			t.Errorf("validSupportEmail(%q) = %v, want %v", in, got, want)
		}
	}
}
