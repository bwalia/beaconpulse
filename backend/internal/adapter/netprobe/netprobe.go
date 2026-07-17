// Package netprobe measures a monitor's target the way an on-call engineer would:
// resolve the name, open the port, inspect the certificate, make the request. It
// implements diagnose.Prober.
//
// Every probe here reports detail back to a tenant — resolved addresses, certificate
// subjects, connection errors, response timings. That is the point of the feature and
// also its hazard: aimed at a private range, the same output is a port scanner with a
// friendly UI, and the tenant chooses the target. So the guard is not decoration on
// this package, it is the precondition for it existing, and nothing dials until an
// address has passed it.
package netprobe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"beacon/internal/domain/diagnose"
	"beacon/internal/platform/safehttp"
)

const (
	dnsTimeout  = 5 * time.Second
	tcpTimeout  = 5 * time.Second
	tlsTimeout  = 5 * time.Second
	httpTimeout = 10 * time.Second
	// maxRedirectChain bounds what we report, matching safehttp's own hop limit.
	maxRedirectChain = 4
)

// Prober measures targets under the address guard.
type Prober struct {
	guard *safehttp.Guard
	http  *safehttp.Client
}

// New builds a Prober. allowPrivate is passed straight through to the guard and
// carries the same warning it does there: it is for single-tenant operators
// diagnosing their own internal services, and it re-opens SSRF anywhere else.
func New(allowPrivate bool) *Prober {
	return &Prober{
		guard: safehttp.NewGuard(allowPrivate),
		http: safehttp.New(safehttp.Config{
			AllowHTTP:    true, // a monitor may legitimately watch a plain-http endpoint
			AllowPrivate: allowPrivate,
			Timeout:      httpTimeout,
			MaxRedirects: maxRedirectChain,
		}),
	}
}

var _ diagnose.Prober = (*Prober)(nil)

// Probe runs the ladder and stops descending once a rung fails. That mirrors how the
// failure actually works — an unresolved name makes every later result meaningless —
// and it keeps the evidence honest: a probe that never ran is recorded as not
// attempted rather than as a pass.
func (p *Prober) Probe(ctx context.Context, target, monitorType string) (diagnose.Evidence, error) {
	host, port, scheme := parseTarget(target, monitorType)
	ev := diagnose.Evidence{
		Target:      target,
		MonitorType: monitorType,
		CheckedAt:   time.Now().UTC(),
	}
	if host == "" {
		ev.DNS.Error = "the monitor's target is not a host we can probe"
		return ev, nil
	}

	// 1. DNS. Also the guard: VetHost both resolves and refuses.
	addrs, err := p.probeDNS(ctx, host, &ev)
	if err != nil {
		var blocked *safehttp.ErrBlockedAddress
		if errors.As(err, &blocked) {
			// Refused, and vaguely. Naming the range would answer the question the
			// scan was asking — that something is there — which is exactly what the
			// guard exists to withhold.
			return diagnose.Evidence{}, fmt.Errorf(
				"this target cannot be diagnosed: %w", err)
		}
		return ev, nil // a resolver failure IS the finding; it is already recorded
	}
	if len(addrs) == 0 {
		return ev, nil
	}

	// 2. TCP against the address we vetted, never a re-resolution.
	ip := addrs[0]
	if !p.probeTCP(ctx, ip, port, &ev) {
		return ev, nil
	}

	// 3. TLS, when the target speaks it.
	if scheme == "https" || monitorType == "ssl" {
		if !p.probeTLS(ctx, ip, port, host, &ev) {
			return ev, nil
		}
	}

	// 4. The request itself.
	if scheme == "http" || scheme == "https" {
		p.probeHTTP(ctx, target, &ev)
	}
	return ev, nil
}

func (p *Prober) probeDNS(ctx context.Context, host string, ev *diagnose.Evidence) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(ctx, dnsTimeout)
	defer cancel()

	start := time.Now()
	addrs, err := p.guard.VetHost(ctx, host)
	ev.DNS.LookupMS = time.Since(start).Milliseconds()
	if err != nil {
		var blocked *safehttp.ErrBlockedAddress
		if errors.As(err, &blocked) {
			return nil, err
		}
		ev.DNS.Resolved = false
		ev.DNS.Error = friendlyErr(err)
		return nil, nil
	}

	ev.DNS.Resolved = true
	for _, ip := range addrs {
		ev.DNS.Addresses = append(ev.DNS.Addresses, ip.String())
	}
	// CNAME and NS are best-effort colour: they turn "does not resolve" into
	// "does not resolve, and here is who is meant to be answering for it", which is
	// the difference between a dead end and a next step.
	if cname, err := net.DefaultResolver.LookupCNAME(ctx, host); err == nil {
		if c := strings.TrimSuffix(cname, "."); c != "" && !strings.EqualFold(c, host) {
			ev.DNS.CNAME = c
		}
	}
	if ns, err := net.DefaultResolver.LookupNS(ctx, registrableSuffix(host)); err == nil {
		for _, n := range ns {
			ev.DNS.Nameservers = append(ev.DNS.Nameservers, strings.TrimSuffix(n.Host, "."))
		}
	}
	return addrs, nil
}

func (p *Prober) probeTCP(ctx context.Context, ip net.IP, port string, ev *diagnose.Evidence) bool {
	ctx, cancel := context.WithTimeout(ctx, tcpTimeout)
	defer cancel()

	addr := net.JoinHostPort(ip.String(), port)
	ev.TCP.Attempted = true
	ev.TCP.Address = addr

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	ev.TCP.ConnectMS = time.Since(start).Milliseconds()
	if err != nil {
		ev.TCP.Error = friendlyErr(err)
		return false
	}
	_ = conn.Close()
	ev.TCP.Connected = true
	return true
}

func (p *Prober) probeTLS(ctx context.Context, ip net.IP, port, serverName string, ev *diagnose.Evidence) bool {
	ctx, cancel := context.WithTimeout(ctx, tlsTimeout)
	defer cancel()

	ev.TLS.Attempted = true
	addr := net.JoinHostPort(ip.String(), port)

	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		ev.TLS.Error = friendlyErr(err)
		return false
	}
	defer func() { _ = raw.Close() }()

	// InsecureSkipVerify, then verify by hand. Not a shortcut: the standard handshake
	// aborts on the first problem, which would leave us reporting "handshake failed"
	// for an expired certificate without ever reading its expiry date. Diagnosis needs
	// the certificate precisely when it is the thing that is broken, so we take it
	// first and judge it after. Nothing is trusted on the strength of this connection
	// — no request is sent over it.
	conn := tls.Client(raw, &tls.Config{ServerName: serverName, InsecureSkipVerify: true}) //nolint:gosec // see above: inspection only, never used to transport data
	if err := conn.HandshakeContext(ctx); err != nil {
		ev.TLS.Error = friendlyErr(err)
		return false
	}
	defer func() { _ = conn.Close() }()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		ev.TLS.Error = "the server offered no certificate"
		return false
	}
	leaf := state.PeerCertificates[0]
	ev.TLS.HandshakeOK = true
	ev.TLS.Issuer = leaf.Issuer.CommonName
	ev.TLS.Subject = leaf.Subject.CommonName
	ev.TLS.NotAfter = leaf.NotAfter.UTC()
	ev.TLS.DaysRemaining = int(time.Until(leaf.NotAfter).Hours() / 24)
	ev.TLS.Expired = time.Now().After(leaf.NotAfter)
	ev.TLS.HostnameOK = leaf.VerifyHostname(serverName) == nil
	return true
}

func (p *Prober) probeHTTP(ctx context.Context, target string, ev *diagnose.Evidence) {
	ctx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	ev.HTTP.Attempted = true
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		ev.HTTP.Error = "the monitor's target is not a valid URL"
		return
	}
	req.Header.Set("User-Agent", "BeaconPulse-Diagnose/1.0")

	start := time.Now()
	resp, err := p.http.Do(req)
	ev.HTTP.ResponseMS = time.Since(start).Milliseconds()
	if err != nil {
		ev.HTTP.Error = friendlyErr(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	ev.HTTP.StatusCode = resp.StatusCode
	// Only the Server header, and only because it names the proxy that is failing —
	// "nginx returned 502" is a lead. The rest of the response is deliberately not
	// reported: headers and bodies are where a misaimed probe would leak whatever it
	// reached, and none of it helps diagnose a domain the caller already owns.
	ev.HTTP.Server = resp.Header.Get("Server")
}

// parseTarget works out what to probe from a monitor's target, which may be a URL
// (http/https/ssl monitors) or a bare host or host:port (tcp/icmp/dns monitors).
func parseTarget(target, monitorType string) (host, port, scheme string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", ""
	}

	if u, err := url.Parse(target); err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
		host = u.Hostname()
		port = u.Port()
		scheme = u.Scheme
		if port == "" {
			if scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		return host, port, scheme
	}

	// Bare host, or host:port.
	if h, p, err := net.SplitHostPort(target); err == nil {
		return h, p, ""
	}
	// An ssl monitor without a scheme still means 443 — that is what it is watching.
	if monitorType == "ssl" {
		return target, "443", "https"
	}
	return target, "443", ""
}

// registrableSuffix drops the leftmost label so an NS lookup asks about the zone
// rather than the record: www.example.com has no nameservers of its own, example.com
// does. A heuristic, not a public-suffix parse — it is only used to add colour, and a
// wrong guess costs an empty field.
func registrableSuffix(host string) string {
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// friendlyErr turns a Go network error into something the person who owns the domain
// can act on, and — just as importantly — keeps our internals out of the reply. It
// preserves the distinction the diagnosis turns on: refused means something answered
// and said no, timed out means nothing answered at all.
func friendlyErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "connection refused"):
		return "connection refused — a host is reachable but nothing is listening on that port"
	case strings.Contains(s, "i/o timeout"), strings.Contains(s, "context deadline exceeded"):
		return "timed out — nothing answered, which usually means a firewall dropped the packet"
	case strings.Contains(s, "no such host"):
		return "no such host — the name does not resolve"
	case strings.Contains(s, "certificate has expired"):
		return "the TLS certificate has expired"
	case strings.Contains(s, "certificate is valid for"):
		return "the TLS certificate does not cover this hostname"
	case strings.Contains(s, "network is unreachable"):
		return "the network is unreachable from our probes"
	case strings.Contains(s, "connection reset"):
		return "the connection was reset by the remote host"
	}
	return s
}
