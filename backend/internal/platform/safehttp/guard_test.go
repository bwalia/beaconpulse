package safehttp

import (
	"context"
	"errors"
	"net"
	"testing"
)

// TestGuard_RefusesInternalRanges pins the addresses a tenant-supplied target must
// never reach. These are not hypothetical: 169.254.169.254 is where cloud instance
// credentials live, and a diagnostic probe reports what it finds back to whoever
// asked for it.
func TestGuard_RefusesInternalRanges(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"169.254.169.254", // cloud metadata
		"10.0.0.5",        // private
		"172.16.4.1",      // private
		"192.168.1.10",    // private
		"0.0.0.0",         // unspecified
		"100.64.1.1",      // CGNAT
		"fd00::1",         // IPv6 ULA
		"::ffff:10.0.0.5", // IPv4-mapped private, the classic bypass
	}
	g := NewGuard(false)
	for _, addr := range blocked {
		t.Run(addr, func(t *testing.T) {
			_, err := g.VetHost(context.Background(), addr)
			var b *ErrBlockedAddress
			if !errors.As(err, &b) {
				t.Fatalf("VetHost(%q) = %v, want it refused", addr, err)
			}
		})
	}
}

func TestGuard_AllowsPublicAddresses(t *testing.T) {
	g := NewGuard(false)
	for _, addr := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if _, err := g.VetHost(context.Background(), addr); err != nil {
			t.Fatalf("VetHost(%q) = %v, want allowed", addr, err)
		}
	}
}

// TestGuard_RefusesAHostThatResolvesToAnyBlockedAddress — a name answering with one
// public and one private address is not half-safe, it is the shape of a rebinding
// attack. Allowing it would let the caller dial the private one.
func TestGuard_RefusesAHostThatResolvesToAnyBlockedAddress(t *testing.T) {
	g := NewGuard(false)
	g.lookupIP = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.5")}, nil
	}
	_, err := g.VetHost(context.Background(), "rebind.test")
	var b *ErrBlockedAddress
	if !errors.As(err, &b) {
		t.Fatalf("a host resolving to one private address was allowed: %v", err)
	}
}

// TestGuard_BlockedErrorNamesNothing — the refusal must not confirm what is there.
// "10.0.0.5 is an internal host" answers exactly the question a scan is asking.
func TestGuard_BlockedErrorNamesNothing(t *testing.T) {
	g := NewGuard(false)
	_, err := g.VetHost(context.Background(), "169.254.169.254")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, leak := range []string{"169.254", "metadata", "internal", "private", "loopback"} {
		if contains(err.Error(), leak) {
			t.Fatalf("the refusal leaks %q, turning the guard into an oracle: %q", leak, err)
		}
	}
}

// TestGuard_AllowPrivateIsTheEscapeHatch — single-tenant operators diagnosing their
// own internal services.
func TestGuard_AllowPrivateIsTheEscapeHatch(t *testing.T) {
	g := NewGuard(true)
	if _, err := g.VetHost(context.Background(), "10.0.0.5"); err != nil {
		t.Fatalf("AllowPrivate should permit internal targets: %v", err)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
