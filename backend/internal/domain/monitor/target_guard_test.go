package monitor

import (
	"context"
	"net"
	"strings"
	"testing"

	"beacon/internal/platform/safehttp"
)

type fakeGuard struct{ blocked map[string]bool }

func (g fakeGuard) VetHost(_ context.Context, host string) ([]net.IP, error) {
	if g.blocked[host] {
		return nil, &safehttp.ErrBlockedAddress{}
	}
	if host == "unresolvable.example" {
		return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
	}
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func guarded(blocked ...string) *Service {
	m := map[string]bool{}
	for _, h := range blocked {
		m[h] = true
	}
	return (&Service{}).WithTargetGuard(fakeGuard{blocked: m})
}

// TestVetTargetRefusesInternalAddresses is the leak this closes. Blackbox probes from
// inside the cluster, so a TCP monitor on the Kubernetes API reports — one bit at a
// time, free, unattended — whether it is listening.
func TestVetTargetRefusesInternalAddresses(t *testing.T) {
	s := guarded("169.254.169.254", "10.0.0.5", "localhost")

	for _, tc := range []struct{ typ Type; target string }{
		{TypeHTTPS, "https://169.254.169.254/latest/meta-data/"}, // cloud credentials
		{TypeTCP, "10.0.0.5:6432"},                               // internal service
		{TypeHTTP, "http://localhost:8080"},                      // ourselves
	} {
		if err := s.vetTarget(context.Background(), tc.typ, tc.target); err == nil {
			t.Errorf("%s target %q was accepted", tc.typ, tc.target)
		}
	}
}

// TestVetTargetRefusalSaysNothingUseful — "10.0.0.5 is internal" confirms something is
// there, which is the answer a scan wants. The refusal must not be an oracle.
func TestVetTargetRefusalSaysNothingUseful(t *testing.T) {
	s := guarded("10.0.0.5")
	err := s.vetTarget(context.Background(), TypeTCP, "10.0.0.5:6432")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, leak := range []string{"10.0.0.5", "internal", "private", "loopback", "metadata"} {
		if strings.Contains(strings.ToLower(err.Error()), leak) {
			t.Errorf("the refusal leaks %q: %v", leak, err)
		}
	}
}

func TestVetTargetAllowsPublicTargets(t *testing.T) {
	s := guarded()
	for _, target := range []string{"https://example.com", "example.com:443", "example.com"} {
		if err := s.vetTarget(context.Background(), TypeHTTPS, target); err != nil {
			t.Errorf("public target %q was refused: %v", target, err)
		}
	}
}

// TestVetTargetAllowsUnresolvableHosts — monitoring a domain that is not live yet is
// ordinary, a DNS blip must not fail a legitimate create, and an unresolvable target
// leaks nothing because the probe fails too.
func TestVetTargetAllowsUnresolvableHosts(t *testing.T) {
	s := guarded()
	if err := s.vetTarget(context.Background(), TypeHTTPS, "https://unresolvable.example"); err != nil {
		t.Fatalf("an unresolvable host was refused: %v", err)
	}
}

// TestVetTargetSkipsHeartbeats — a heartbeat has no target; it waits to be pinged.
func TestVetTargetSkipsHeartbeats(t *testing.T) {
	s := guarded("10.0.0.5")
	if err := s.vetTarget(context.Background(), TypeHeartbeat, ""); err != nil {
		t.Fatalf("heartbeat was refused: %v", err)
	}
}

// TestNoGuardAllowsEverything — the single-tenant escape hatch, where the operator's
// own internal services are exactly what they want to watch.
func TestNoGuardAllowsEverything(t *testing.T) {
	s := &Service{}
	if err := s.vetTarget(context.Background(), TypeTCP, "10.0.0.5:6432"); err != nil {
		t.Fatalf("with no guard configured the target should be allowed: %v", err)
	}
}
