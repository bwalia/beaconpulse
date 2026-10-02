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

func TestValidPublicText(t *testing.T) {
	for in, want := range map[string]bool{
		"":                                true,
		"Acme Ltd":                        true,
		"1 High St, London, EC1A 1AA, UK": true,
		"Line one\nLine two":              false, // single line only — it's inline in a sentence
		"Acme\u0000Ltd":                   false,
		string(make([]byte, 201)):         false, // NULs and too long
	} {
		if got := validPublicText(in, 200); got != want {
			t.Errorf("validPublicText(%q) = %v, want %v", in, got, want)
		}
	}
}
