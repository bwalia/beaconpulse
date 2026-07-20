package rest

import (
	"testing"
)

// The limits are a product decision as much as a security one: set too loose they do
// not bound the bill, set too tight they break a real customer. These pin both edges,
// so a future tweak has to be deliberate rather than accidental.

// TestSignupLimitStopsAFarmWithoutStoppingAnOffice is the important one. Registration
// is the only endpoint where a single request buys permanent recurring cost — ten
// monitors probing every minute, forever — so it carries the tightest limit we apply.
func TestSignupLimitStopsAFarmWithoutStoppingAnOffice(t *testing.T) {
	const ip = "203.0.113.7"

	// A small team behind one office address signing up together must get through.
	for i := 0; i < 3; i++ {
		if !signupLimiter.Allow(ip) {
			t.Fatalf("signup %d from one address was refused; a real team shares an IP", i+1)
		}
	}
	// A script continuing past that is creating orgs we pay to monitor.
	if signupLimiter.Allow(ip) {
		t.Fatal("a 4th immediate signup was allowed — the burst is not capped")
	}
	// And it must not have limited anybody else.
	if !signupLimiter.Allow("198.51.100.9") {
		t.Fatal("one address hitting the signup limit starved every other visitor")
	}
}

// TestLoginLimitAllowsFatFingersAndStopsAList — a person mistyping a password a few
// times is normal; working through a credential dump is not.
func TestLoginLimitAllowsFatFingersAndStopsAList(t *testing.T) {
	const ip = "203.0.113.8"
	for i := 0; i < 10; i++ {
		if !loginLimiter.Allow(ip) {
			t.Fatalf("login attempt %d refused — too tight for someone mistyping", i+1)
		}
	}
	if loginLimiter.Allow(ip) {
		t.Fatal("an 11th consecutive login attempt was allowed")
	}
}

// TestBaselineDoesNotBlockADashboard — the dashboard fires a dozen requests when a
// page loads. A baseline that trips on ordinary use would be worse than none, because
// it would be turned off.
func TestBaselineDoesNotBlockADashboard(t *testing.T) {
	const ip = "203.0.113.10"
	for i := 0; i < 60; i++ {
		if !baselineLimiter.Allow(ip) {
			t.Fatalf("baseline refused request %d — a page load fires many at once", i+1)
		}
	}
}

// TestSyncLimitIsPerOrgNotPerAddress — a workflow runs from a different GitHub runner
// every time, so an address key would limit nobody. This pins that the key we chose
// actually isolates one org from another.
func TestSyncLimitIsPerOrgNotPerAddress(t *testing.T) {
	const orgA, orgB = "org-a", "org-b"
	for i := 0; i < 5; i++ {
		if !syncLimiter.Allow(orgA) {
			t.Fatalf("sync %d refused; a push that touches several files may retry", i+1)
		}
	}
	if syncLimiter.Allow(orgA) {
		t.Fatal("a runaway loop was allowed to keep rewriting the control plane")
	}
	if !syncLimiter.Allow(orgB) {
		t.Fatal("one org's loop starved another org's deploy")
	}
}

// TestPublicStatusSurvivesBeingLinked — a status page is read most when it is linked
// from an incident, which is exactly when it must not rate-limit real readers.
func TestPublicStatusSurvivesBeingLinked(t *testing.T) {
	for i := 0; i < 20; i++ {
		if !publicLimiter.Allow("203.0.113.11") {
			t.Fatalf("status page refused reader %d from one address", i+1)
		}
	}
}
