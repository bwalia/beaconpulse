// Package safehttp provides an HTTP client hardened against server-side request
// forgery (SSRF) for fetching URLs that a TENANT controls.
//
// Beacon lets customers configure webhook and Slack notification channels, i.e.
// URLs that the Beacon SERVER will POST to when an alert fires. Beacon runs inside
// a cluster with an internal network, so an unguarded fetch is a loaded weapon: a
// tenant could aim a webhook at the cloud metadata endpoint (169.254.169.254) to
// steal instance credentials, at internal services (postgres, the API, Vault), or
// at loopback — none of which should ever be reachable from a tenant-supplied URL.
//
// The defence that actually works is to vet the IP we CONNECT TO, not the hostname
// we were given. A hostname check is defeated by DNS rebinding: `evil.example.com`
// resolves to a public IP when validated and to 127.0.0.1 when dialled. So the
// guard lives in the dialer: resolve the host, reject the connection if any
// resolved address is in a blocked range, and dial the vetted IP. Redirects are
// re-vetted the same way, because a 302 to an internal address is the same attack
// wearing a hat.
//
// This package is the security boundary for outbound tenant-triggered requests and
// is tested as one (see safehttp_test.go).
package safehttp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Config tunes the guard. The zero value is safe and production-appropriate.
type Config struct {
	// AllowHTTP permits plain http:// URLs. Off by default — tenant webhooks
	// should be https. Operators enable it for local development only.
	AllowHTTP bool
	// AllowPrivate disables the private/loopback/link-local block entirely. This
	// re-opens the SSRF hole and exists ONLY for single-tenant on-prem operators
	// who deliberately want internal webhooks. Never enable in a multi-tenant
	// deployment.
	AllowPrivate bool
	// Timeout bounds the whole request. Defaults to 10s.
	Timeout time.Duration
	// MaxResponseBytes caps how much of the response body Read will return. We only
	// read the body to surface an error message, so it is small. Defaults to 64KiB.
	MaxResponseBytes int64
	// MaxRedirects caps redirect following. Each hop is re-vetted. Defaults to 3.
	MaxRedirects int
}

func (c Config) withDefaults() Config {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	if c.MaxResponseBytes == 0 {
		c.MaxResponseBytes = 64 << 10
	}
	if c.MaxRedirects == 0 {
		c.MaxRedirects = 3
	}
	return c
}

// Client is a guarded HTTP client. Safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
	// lookupIP is the resolver, injectable so tests can exercise the dial guard
	// (incl. DNS rebinding) deterministically without real DNS. Defaults to the
	// system resolver.
	lookupIP func(ctx context.Context, host string) ([]net.IP, error)
}

// ErrBlockedAddress is returned when a request targets a scheme or address the
// guard refuses. It is deliberately vague to the caller-facing message so we do
// not turn the notifier into a network scanner that reports which internal hosts
// exist.
type ErrBlockedAddress struct{ reason string }

func (e *ErrBlockedAddress) Error() string { return "request blocked: " + e.reason }

// New builds a guarded Client.
func New(cfg Config) *Client {
	cfg = cfg.withDefaults()
	c := &Client{
		cfg: cfg,
		lookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}

	transport := &http.Transport{
		// The guard. control runs AFTER the address is resolved and BEFORE the
		// socket is opened, for every address the resolver returns — including the
		// resolved IPs of the redirect targets, since the same Transport handles
		// them. This is the line that defeats DNS rebinding.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := c.lookupIP(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve %q: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, &ErrBlockedAddress{reason: "host did not resolve"}
			}
			for _, ip := range ips {
				if !cfg.AllowPrivate && isBlockedIP(ip) {
					return nil, &ErrBlockedAddress{reason: "resolves to a disallowed address"}
				}
			}
			// Dial the vetted IP directly so we connect to exactly what we checked,
			// not to a re-resolution that a rebinding attacker could have flipped.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		// No connection reuse across requests: pooled connections to a
		// tenant-controlled host are not worth the isolation risk here.
		DisableKeepAlives:   true,
		MaxIdleConns:        0,
		IdleConnTimeout:     time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	c.http = &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
			}
			// The scheme must survive redirects too — a https→http downgrade to an
			// internal plaintext service is a classic bypass.
			if err := checkScheme(req.URL.Scheme, cfg.AllowHTTP); err != nil {
				return err
			}
			return nil
		},
	}

	return c
}

// Do sends req through the guard and returns the response with a body already
// capped at MaxResponseBytes. The caller owns closing the body.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if err := checkScheme(req.URL.Scheme, c.cfg.AllowHTTP); err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, c.cfg.MaxResponseBytes), resp.Body}
	return resp, nil
}

func checkScheme(scheme string, allowHTTP bool) error {
	switch scheme {
	case "https":
		return nil
	case "http":
		if allowHTTP {
			return nil
		}
		return &ErrBlockedAddress{reason: "http is not allowed (use https)"}
	default:
		return &ErrBlockedAddress{reason: "unsupported url scheme"}
	}
}

// isBlockedIP reports whether ip is in a range a tenant-supplied URL must never
// reach. Kept as one place so the block list cannot drift between call sites.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || // 127/8, ::1
		ip.IsLinkLocalUnicast() || // 169.254/16 (incl. cloud metadata), fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() || // 0.0.0.0, ::
		ip.IsPrivate() { // 10/8, 172.16/12, 192.168/16, fc00::/7
		return true
	}
	// IPv4-mapped IPv6 (::ffff:a.b.c.d) — normalise and re-check so a mapped
	// private address cannot slip past as "not private".
	if v4 := ip.To4(); v4 != nil && !ip.Equal(v4) {
		return isBlockedIP(v4)
	}
	// 100.64.0.0/10 (carrier-grade NAT / shared address space) and IPv6 ULA are
	// covered by IsPrivate on modern Go; guard the CGNAT range explicitly for
	// belt-and-braces on the IPv4 path.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}
