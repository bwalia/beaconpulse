package ratelimit

import "testing"

// Pins the diagnose endpoint's setting: a burst of 3, then refusal, and per-key
// isolation so one org hammering cannot starve another's outage.
func TestDiagnoseLimiterShape(t *testing.T) {
	l := New(1.0/6.0, 3, 10000)

	for i := 0; i < 3; i++ {
		if !l.Allow("org-a") {
			t.Fatalf("burst request %d was refused; a human checking a few monitors must not be blocked", i+1)
		}
	}
	if l.Allow("org-a") {
		t.Fatal("a 4th immediate request was allowed — the burst is not capped")
	}
	// A different org is unaffected: the one on fire must still get through.
	if !l.Allow("org-b") {
		t.Fatal("one org's hammering starved another org")
	}
}
