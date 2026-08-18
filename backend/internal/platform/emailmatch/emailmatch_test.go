package emailmatch

import "testing"

func TestMatch(t *testing.T) {
	list := Normalize([]string{" Workstation.CO.UK ", "boss@Example.com", ""})
	cases := []struct {
		email string
		want  bool
	}{
		{"me@workstation.co.uk", true},        // domain, exact
		{"me@team.workstation.co.uk", true},   // domain, subdomain
		{"ME@Workstation.co.uk", true},        // case-insensitive
		{"boss@example.com", true},            // full address
		{"other@example.com", false},          // address list is exact, not domain
		{"me@notworkstation.co.uk", false},    // suffix must be on a dot boundary
		{"me@evilworkstation.co.uk", false},   // ditto
		{"", false},                           // empty
		{"noatsign", false},                   // no domain
	}
	for _, c := range cases {
		if got := Match(list, c.email); got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.email, got, c.want)
		}
	}
	if Match(nil, "me@workstation.co.uk") {
		t.Error("empty list must match nothing")
	}
}
