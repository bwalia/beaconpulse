package monitor

import (
	"strings"
	"testing"

	"beacon/internal/platform/apperror"
)

func TestNormalizeHTTPDefaults(t *testing.T) {
	target, s, err := normalizeAndValidate(TypeHTTPS, "example.com", Settings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "https://example.com" {
		t.Errorf("target = %q, want scheme prepended", target)
	}
	if s.Method != "GET" {
		t.Errorf("method default = %q, want GET", s.Method)
	}
	if len(s.ValidStatusCodes) == 0 {
		t.Error("expected default valid status codes")
	}
	if s.SSLExpiryWarningDays != 30 {
		t.Errorf("ssl warning default = %d, want 30", s.SSLExpiryWarningDays)
	}
}

func TestNormalizeHTTPRejectsBadMethod(t *testing.T) {
	_, _, err := normalizeAndValidate(TypeHTTP, "http://x.com", Settings{Method: "FETCH"})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestNormalizeTCPRequiresHostPort(t *testing.T) {
	if _, _, err := normalizeAndValidate(TypeTCP, "example.com", Settings{}); err == nil {
		t.Fatal("expected error for TCP target without port")
	}
	if _, _, err := normalizeAndValidate(TypeTCP, "example.com:5432", Settings{}); err != nil {
		t.Fatalf("valid host:port rejected: %v", err)
	}
}

func TestNormalizeDNSDefaults(t *testing.T) {
	_, s, err := normalizeAndValidate(TypeDNS, "example.com", Settings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DNSQueryName != "example.com" {
		t.Errorf("dns query name = %q, want example.com", s.DNSQueryName)
	}
	if s.DNSQueryType != "A" {
		t.Errorf("dns query type = %q, want A", s.DNSQueryType)
	}
}

func TestNormalizeRejectsUnsupportedType(t *testing.T) {
	if _, _, err := normalizeAndValidate(Type("server"), "host", Settings{}); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error for unsupported type, got %v", err)
	}
}

func TestNormalizeICMPRejectsURL(t *testing.T) {
	if _, _, err := normalizeAndValidate(TypeICMP, "https://example.com", Settings{}); err == nil {
		t.Fatal("expected ICMP to reject a URL target")
	}
}

func TestNormalizeGitHubActions(t *testing.T) {
	cases := map[string]string{
		"bwalia/beaconpulse":                              "bwalia/beaconpulse",
		"https://github.com/bwalia/beaconpulse":           "bwalia/beaconpulse",
		"https://github.com/bwalia/beaconpulse/":          "bwalia/beaconpulse",
		"github.com/bwalia/beaconpulse.git":               "bwalia/beaconpulse",
		"https://github.com/bwalia/beaconpulse/tree/main": "bwalia/beaconpulse",
	}
	for in, want := range cases {
		got, _, err := normalizeAndValidate(TypeGitHubActions, in, Settings{})
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("%q normalized to %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeGitHubActionsRejectsBadRepo(t *testing.T) {
	for _, in := range []string{"", "just-owner", "/", "owner/"} {
		if _, _, err := normalizeAndValidate(TypeGitHubActions, in, Settings{}); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Errorf("%q: expected validation error, got %v", in, err)
		}
	}
}

// TestMaskedHeadersKeepSavedValues: the API returns HeaderMask for every header
// value, so a client that sends settings back unchanged must keep the secret, and
// a mask with nothing saved behind it is refused rather than stored.
func TestMaskedHeadersKeepSavedValues(t *testing.T) {
	saved := map[string]string{"Authorization": "Bearer s3cret"}
	next := map[string]string{"Authorization": HeaderMask, "X-Trace": "on"}
	if err := keepMaskedHeaders(next, saved); err != nil {
		t.Fatal(err)
	}
	if next["Authorization"] != "Bearer s3cret" || next["X-Trace"] != "on" {
		t.Errorf("got %v", next)
	}
	if err := keepMaskedHeaders(map[string]string{"X-Api-Key": HeaderMask}, saved); err == nil {
		t.Error("a mask with no saved value was accepted")
	}
}

func TestRedactedMasksHeaderValues(t *testing.T) {
	s := Settings{Headers: map[string]string{"Authorization": "Bearer s3cret"}}
	r := s.Redacted()
	if r.Headers["Authorization"] != HeaderMask {
		t.Errorf("redacted value = %q", r.Headers["Authorization"])
	}
	if s.Headers["Authorization"] != "Bearer s3cret" {
		t.Error("Redacted mutated the original settings")
	}
}

func TestValidateRequestRejectsUnsafeInput(t *testing.T) {
	for name, s := range map[string]Settings{
		"bad header name": {Headers: map[string]string{"Bad Header": "x"}},
		"CRLF in value":   {Headers: map[string]string{"X-A": "a\r\nInjected: 1"}},
		"oversized body":  {Body: strings.Repeat("x", maxBody+1)},
	} {
		if _, _, err := normalizeAndValidate(TypeHTTPS, "https://example.com", s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := Settings{Method: "POST", Body: `{"q":1}`, Headers: map[string]string{"Authorization": "Bearer x", "X-API-Key": "k"}}
	if _, _, err := normalizeAndValidate(TypeHTTPS, "https://example.com/checkout?step=1", ok); err != nil {
		t.Errorf("valid request refused: %v", err)
	}
}
