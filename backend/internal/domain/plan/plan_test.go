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

// TestCatalogCopy checks the operator-editable card copy: numeric bullets are always
// generated from the live limits (never drift), and tagline/highlights fall back to the
// built-in defaults when not customised but honour an operator override.
func TestCatalogCopy(t *testing.T) {
	t.Cleanup(func() { Apply(DefaultConfig()) })

	// Defaults: no stored tagline/features → built-in defaults, numeric bullets derived.
	Apply(DefaultConfig())
	free := catalogFor(t, Free)
	if free.Tagline != defaultTaglines[Free] {
		t.Errorf("default tagline = %q, want %q", free.Tagline, defaultTaglines[Free])
	}
	if got := free.Features[0]; got != "10 monitors" {
		t.Errorf("free first bullet = %q, want %q", got, "10 monitors")
	}
	// Free has 0 monthly diagnoses → no AI bullet; Pro has 1000 → one.
	pro := catalogFor(t, Pro)
	if !hasBullet(pro.Features, "1000 AI summaries / month") {
		t.Errorf("pro features missing AI bullet: %v", pro.Features)
	}
	if hasBullet(free.Features, "AI summaries / month") {
		t.Errorf("free should have no AI bullet: %v", free.Features)
	}

	// Operator override: custom tagline + highlights win; a re-tuned interval shows in
	// the generated bullet as readable text (1800s → "30m minimum interval").
	c := DefaultConfig()
	c.Limits[Free] = Limits{MaxMonitors: 5, MinIntervalSeconds: 1800, MonthlyDiagnoses: 0}
	c.Taglines[Free] = "Kick the tyres."
	c.Features[Free] = []string{"Custom bullet"}
	Apply(c)
	free = catalogFor(t, Free)
	if free.Tagline != "Kick the tyres." {
		t.Errorf("custom tagline = %q", free.Tagline)
	}
	if free.Features[0] != "5 monitors" || free.Features[1] != "30m minimum interval" {
		t.Errorf("generated bullets not updated: %v", free.Features)
	}
	if free.Highlights[0] != "Custom bullet" {
		t.Errorf("custom highlight lost: %v", free.Highlights)
	}
}

func catalogFor(t *testing.T, p Plan) Info {
	t.Helper()
	for _, info := range Catalog() {
		if info.Plan == p {
			return info
		}
	}
	t.Fatalf("plan %q not in catalog", p)
	return Info{}
}

func hasBullet(bullets []string, sub string) bool {
	for _, b := range bullets {
		if b == sub || (len(sub) < len(b) && contains(b, sub)) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
