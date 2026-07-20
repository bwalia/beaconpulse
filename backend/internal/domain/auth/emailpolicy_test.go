package auth

import (
	"context"
	"errors"
	"net"
	"testing"
)

func policy(mx []*net.MX, err error) *EmailPolicy {
	p := NewEmailPolicy(true)
	p.lookupMX = func(context.Context, string) ([]*net.MX, error) { return mx, err }
	return p
}

func TestRejectsDisposableProviders(t *testing.T) {
	p := policy([]*net.MX{{Host: "mx.mailinator.com"}}, nil)
	// Note the MX lookup SUCCEEDS here — a throwaway provider does receive mail. The
	// point is that the inbox is not one anyone can be reached at tomorrow.
	if err := p.Check(context.Background(), "someone@mailinator.com"); err == nil {
		t.Fatal("a throwaway provider was accepted")
	}
}

func TestRejectsADomainThatCannotReceiveMail(t *testing.T) {
	p := policy(nil, nil) // resolves, but no MX records
	if err := p.Check(context.Background(), "someone@no-mail-here.example"); err == nil {
		t.Fatal("a domain with no mail server was accepted; that account can never be recovered or warned")
	}
}

// TestFailsOpenWhenDNSIsUnavailable is the one that protects revenue. This check exists
// to raise the floor on junk, not to arbitrate whether an address is real — and turning
// a resolver hiccup into "you cannot create an account" would cost a genuine customer at
// the exact moment they were signing up.
func TestFailsOpenWhenDNSIsUnavailable(t *testing.T) {
	p := policy(nil, errors.New("dns timeout"))
	if err := p.Check(context.Background(), "someone@example.com"); err != nil {
		t.Fatalf("a DNS failure blocked a signup: %v", err)
	}
}

func TestAcceptsANormalAddress(t *testing.T) {
	p := policy([]*net.MX{{Host: "aspmx.l.google.com"}}, nil)
	if err := p.Check(context.Background(), "founder@realcompany.com"); err != nil {
		t.Fatalf("a normal address was rejected: %v", err)
	}
}

func TestDisabledPolicyChecksNothing(t *testing.T) {
	p := NewEmailPolicy(false)
	p.lookupMX = func(context.Context, string) ([]*net.MX, error) {
		t.Fatal("a disabled policy performed a DNS lookup")
		return nil, nil
	}
	if err := p.Check(context.Background(), "x@mailinator.com"); err != nil {
		t.Fatalf("disabled policy rejected: %v", err)
	}
}
