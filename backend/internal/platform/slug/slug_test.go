package slug

import "testing"

func TestMake(t *testing.T) {
	cases := []struct {
		in, fallback, want string
	}{
		{"Acme Inc.", "org", "acme-inc"},
		{"  Hello   World  ", "x", "hello-world"},
		{"Production Website!!!", "x", "production-website"},
		{"---weird---", "x", "weird"},
		{"", "fallback", "fallback"},
		{"!!!", "fallback", "fallback"},
		{"CamelCase123", "x", "camelcase123"},
		{"a/b\\c", "x", "a-b-c"},
	}
	for _, c := range cases {
		if got := Make(c.in, c.fallback); got != c.want {
			t.Errorf("Make(%q, %q) = %q, want %q", c.in, c.fallback, got, c.want)
		}
	}
}

func TestMakeTruncatesTo63(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	got := Make(long, "x")
	if len(got) > 63 {
		t.Errorf("slug length = %d, want <= 63", len(got))
	}
}

func TestMakeMatchesConstraint(t *testing.T) {
	// The DB CHECK requires ^[a-z0-9][a-z0-9-]{0,62}$ with no trailing hyphen.
	got := Make("Trailing hyphens---", "x")
	if got == "" || got[len(got)-1] == '-' || got[0] == '-' {
		t.Errorf("slug %q violates the constraint", got)
	}
}
