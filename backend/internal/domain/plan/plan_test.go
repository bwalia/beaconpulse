package plan

import "testing"

// TestResolve covers the free-Pro paths and the normal effective-plan fallback, since
// this decides who gets premium access and must never leak it to "everyone".
func TestResolve(t *testing.T) {
	// Isolate global state: an operator + a premium domain, then restore.
	SetAdmins([]string{"boss@ops.com"})
	Apply(Config{Grants: []string{"acme.com"}})
	t.Cleanup(func() { SetAdmins(nil); Apply(DefaultConfig()) })

	cases := []struct {
		name       string
		subscribed Plan
		active     bool
		credit     int64
		ownerEmail string
		want       Plan
	}{
		{"platform admin → pro", Free, false, 0, "boss@ops.com", Pro},
		{"granted domain → pro", Free, false, 0, "jo@acme.com", Pro},
		{"granted subdomain → pro", Free, false, 0, "jo@team.acme.com", Pro},
		{"plain free stays free", Free, false, 0, "someone@else.com", Free},
		{"credit → payg", Free, false, 100, "someone@else.com", PayAsYouGo},
		{"active subscription honored", Starter, true, 0, "someone@else.com", Starter},
		{"empty owner never premium", Free, false, 0, "", Free},
	}
	for _, c := range cases {
		if got := Resolve(c.subscribed, c.active, c.credit, c.ownerEmail); got != c.want {
			t.Errorf("%s: Resolve(%q) = %q, want %q", c.name, c.ownerEmail, got, c.want)
		}
	}
}
