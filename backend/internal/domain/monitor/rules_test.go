package monitor

import (
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
