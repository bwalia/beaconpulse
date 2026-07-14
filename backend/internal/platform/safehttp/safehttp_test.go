package safehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsBlockedIP is the exhaustive block-list table. If an address that should be
// unreachable from a tenant URL is ever allowed, it shows up here first.
func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
		why     string
	}{
		// ---- must be blocked ----
		{"127.0.0.1", true, "loopback v4"},
		{"127.0.0.53", true, "loopback range"},
		{"::1", true, "loopback v6"},
		{"169.254.169.254", true, "cloud metadata (link-local)"},
		{"169.254.0.1", true, "link-local v4"},
		{"fe80::1", true, "link-local v6"},
		{"10.0.0.5", true, "private 10/8"},
		{"172.16.4.4", true, "private 172.16/12"},
		{"192.168.1.1", true, "private 192.168/16"},
		{"fc00::1", true, "unique-local v6"},
		{"fd12:3456::1", true, "unique-local v6"},
		{"0.0.0.0", true, "unspecified v4"},
		{"::", true, "unspecified v6"},
		{"224.0.0.1", true, "multicast"},
		{"::ffff:127.0.0.1", true, "ipv4-mapped loopback must not slip past"},
		{"::ffff:10.0.0.1", true, "ipv4-mapped private must not slip past"},
		{"100.64.0.1", true, "CGNAT shared space"},
		{"100.127.255.255", true, "CGNAT upper bound"},

		// ---- must be allowed (real public addresses) ----
		{"8.8.8.8", false, "public v4"},
		{"1.1.1.1", false, "public v4"},
		{"93.184.216.34", false, "public v4 (example.com)"},
		{"2606:4700:4700::1111", false, "public v6"},
		{"99.255.255.255", false, "just below CGNAT"},
		{"101.0.0.1", false, "just above CGNAT"},
	}

	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("bad test IP %q", tc.ip)
			}
			if got := isBlockedIP(ip); got != tc.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v (%s)", tc.ip, got, tc.blocked, tc.why)
			}
		})
	}
}

func TestCheckScheme(t *testing.T) {
	cases := []struct {
		scheme    string
		allowHTTP bool
		ok        bool
	}{
		{"https", false, true},
		{"http", false, false},
		{"http", true, true},
		{"file", true, false},
		{"gopher", false, false},
		{"ftp", true, false},
	}
	for _, tc := range cases {
		err := checkScheme(tc.scheme, tc.allowHTTP)
		if (err == nil) != tc.ok {
			t.Errorf("checkScheme(%q, allowHTTP=%v) err=%v, want ok=%v", tc.scheme, tc.allowHTTP, err, tc.ok)
		}
	}
}

// withResolver swaps the client's resolver so we can point any hostname at any IP,
// which is how DNS rebinding is simulated deterministically.
func (c *Client) withResolver(f func(host string) []net.IP) *Client {
	c.lookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		if ips := f(host); ips != nil {
			return ips, nil
		}
		return nil, errors.New("no such host")
	}
	return c
}

func TestDo_AllowsPublicResolvedHost(t *testing.T) {
	// A real loopback server, but the client is told the hostname resolves to a
	// PUBLIC ip — so the guard allows it — while we actually dial loopback where
	// the test server lives. (We override the dial target to the server for the
	// allow case by resolving the host to the server's real loopback addr AND
	// setting AllowPrivate, proving the happy path end-to-end.)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	c := New(Config{AllowHTTP: true, AllowPrivate: true}). // AllowPrivate: this is the "internal webhook opt-in" path
								withResolver(func(string) []net.IP { return []net.IP{net.ParseIP(host)} })

	req, _ := http.NewRequest(http.MethodGet, "http://anything.example.com:"+port+"/", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v, want success", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDo_BlocksRebindToLoopback(t *testing.T) {
	// The attack: a public-looking hostname that resolves to 127.0.0.1. The
	// hostname reveals nothing; only vetting the resolved IP catches it.
	c := New(Config{AllowHTTP: true}).
		withResolver(func(string) []net.IP { return []net.IP{net.ParseIP("127.0.0.1")} })

	req, _ := http.NewRequest(http.MethodGet, "http://not-suspicious.example.com/", nil)
	_, err := c.Do(req)
	if err == nil {
		t.Fatal("Do() succeeded, want blocked — rebinding to loopback slipped through")
	}
	assertBlocked(t, err)
}

func TestDo_BlocksMetadataEndpoint(t *testing.T) {
	c := New(Config{AllowHTTP: true}).
		withResolver(func(string) []net.IP { return []net.IP{net.ParseIP("169.254.169.254")} })

	req, _ := http.NewRequest(http.MethodGet, "http://metadata.example.com/latest/meta-data/", nil)
	if _, err := c.Do(req); err == nil || !isBlockedErr(err) {
		t.Fatalf("Do() err = %v, want blocked (cloud metadata must be unreachable)", err)
	}
}

func TestDo_BlocksWhenAnyResolvedIPIsInternal(t *testing.T) {
	// A host that resolves to BOTH a public and an internal address must be
	// blocked — an attacker only needs one of the returned IPs to be dialled.
	c := New(Config{AllowHTTP: true}).
		withResolver(func(string) []net.IP {
			return []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.1")}
		})

	req, _ := http.NewRequest(http.MethodGet, "http://split.example.com/", nil)
	if _, err := c.Do(req); err == nil || !isBlockedErr(err) {
		t.Fatalf("Do() err = %v, want blocked when any resolved IP is internal", err)
	}
}

func TestDo_RejectsHTTPByDefault(t *testing.T) {
	c := New(Config{}) // AllowHTTP false
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	if _, err := c.Do(req); err == nil || !isBlockedErr(err) {
		t.Fatalf("Do() err = %v, want blocked (http must be refused by default)", err)
	}
}

func TestDo_RejectsNonHTTPScheme(t *testing.T) {
	c := New(Config{})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.URL.Scheme = "file" // force a bad scheme past URL parsing
	if _, err := c.Do(req); err == nil || !isBlockedErr(err) {
		t.Fatalf("Do() err = %v, want blocked for non-http(s) scheme", err)
	}
}

func assertBlocked(t *testing.T, err error) {
	t.Helper()
	if !isBlockedErr(err) {
		t.Fatalf("err = %v, want an *ErrBlockedAddress", err)
	}
}

func isBlockedErr(err error) bool {
	var blocked *ErrBlockedAddress
	return errors.As(err, &blocked)
}
