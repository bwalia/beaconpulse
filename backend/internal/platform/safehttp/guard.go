package safehttp

import (
	"context"
	"net"
)

// Guard applies this package's address policy to connections that are not HTTP.
//
// Client covers tenant-supplied URLs, but a diagnostic probe also opens raw TCP
// sockets and TLS handshakes against a tenant-supplied target, and those need the
// identical block list. Hence a Guard rather than a second copy of the ranges: both
// go through isBlockedIP, so the promise made there — one place, no drift between
// call sites — still holds once probing exists.
//
// The reason this matters more for probing than for webhooks: a blocked webhook
// tells the tenant nothing, while a probe reports back DNS answers, certificate
// subjects, and connection timings. Pointed at an internal range, that is a port
// scanner with a friendly UI, so the guard has to sit in front of every probe.
type Guard struct {
	allowPrivate bool
	// lookupIP is injectable so tests can drive the policy without real DNS.
	lookupIP func(ctx context.Context, host string) ([]net.IP, error)
}

// NewGuard builds a Guard. allowPrivate disables the block entirely and carries the
// same warning as Config.AllowPrivate: it is for single-tenant operators diagnosing
// their own internal services, and it re-opens SSRF anywhere else.
func NewGuard(allowPrivate bool) *Guard {
	return &Guard{
		allowPrivate: allowPrivate,
		lookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
	}
}

// VetHost resolves host and returns the addresses it is safe to dial, or
// ErrBlockedAddress.
//
// A host is refused if ANY of its addresses is blocked, not merely if all of them
// are. A name that answers with one public and one private address is not a
// half-safe host; it is the shape of a rebinding attack, and the caller would
// otherwise be free to dial the private one.
//
// Callers must dial a returned IP, never re-resolve the name: re-resolution is the
// window an attacker flips, and it would put the check and the connection on
// different answers.
func (g *Guard) VetHost(ctx context.Context, host string) ([]net.IP, error) {
	// A literal IP still goes through the policy — the block list is about where the
	// packet lands, and "no DNS was involved" is not a reason to trust it.
	if ip := net.ParseIP(host); ip != nil {
		if !g.allowPrivate && isBlockedIP(ip) {
			return nil, &ErrBlockedAddress{reason: "resolves to a disallowed address"}
		}
		return []net.IP{ip}, nil
	}

	ips, err := g.lookupIP(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, &ErrBlockedAddress{reason: "host did not resolve"}
	}
	if !g.allowPrivate {
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return nil, &ErrBlockedAddress{reason: "resolves to a disallowed address"}
			}
		}
	}
	return ips, nil
}
