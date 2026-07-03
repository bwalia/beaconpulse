package monitor

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"beacon/internal/platform/apperror"
)

// validDNSTypes are the DNS record types Beacon can query.
var validDNSTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "MX": true,
	"TXT": true, "NS": true, "SOA": true, "CAA": true,
}

// validHTTPMethods for HTTP probes.
var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "HEAD": true, "PUT": true, "DELETE": true, "PATCH": true,
}

// normalizeAndValidate cleans up user input for a monitor of type t and returns
// the normalized target and settings, or a validation error. It is the single
// place monitor invariants are enforced, shared by create and update.
func normalizeAndValidate(t Type, target string, s Settings) (string, Settings, error) {
	if !SupportedTypes[t] {
		return "", s, apperror.Validation(
			fmt.Sprintf("monitor type %q is not available yet", t),
			apperror.FieldError{Field: "type", Message: "unsupported monitor type"})
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", s, apperror.Validation("target is required",
			apperror.FieldError{Field: "target", Message: "is required"})
	}

	// Alert sensitivity applies to every monitor type.
	if s.AlertSensitivity == "" {
		s.AlertSensitivity = SensitivityBalanced
	}
	if !ValidSensitivity(s.AlertSensitivity) {
		return "", s, apperror.Validation("invalid alert sensitivity",
			apperror.FieldError{Field: "settings.alert_sensitivity", Message: "must be immediate, balanced or relaxed"})
	}

	switch t {
	case TypeHTTP, TypeHTTPS, TypeSSL:
		return validateHTTP(t, target, s)
	case TypeTCP:
		return validateTCP(target, s)
	case TypeICMP:
		return validateICMP(target, s)
	case TypeDNS:
		return validateDNS(target, s)
	default:
		return "", s, apperror.Validation("unsupported monitor type")
	}
}

func validateHTTP(t Type, target string, s Settings) (string, Settings, error) {
	// Allow bare hosts by defaulting a scheme based on type.
	if !strings.Contains(target, "://") {
		if t == TypeHTTP {
			target = "http://" + target
		} else {
			target = "https://" + target
		}
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", s, apperror.Validation("target must be a valid http(s) URL",
			apperror.FieldError{Field: "target", Message: "must be a valid URL"})
	}

	if s.Method == "" {
		s.Method = "GET"
	}
	s.Method = strings.ToUpper(s.Method)
	if !validHTTPMethods[s.Method] {
		return "", s, apperror.Validation("invalid HTTP method",
			apperror.FieldError{Field: "settings.method", Message: "must be a valid HTTP method"})
	}
	if len(s.ValidStatusCodes) == 0 {
		s.ValidStatusCodes = []int{200, 201, 202, 203, 204, 206, 300, 301, 302, 303, 304, 307, 308}
	}
	for _, code := range s.ValidStatusCodes {
		if code < 100 || code > 599 {
			return "", s, apperror.Validation("invalid status code in valid_status_codes",
				apperror.FieldError{Field: "settings.valid_status_codes", Message: "codes must be 100-599"})
		}
	}
	if (t == TypeSSL || u.Scheme == "https") && s.SSLExpiryWarningDays == 0 {
		s.SSLExpiryWarningDays = 30
	}
	return target, s, nil
}

func validateTCP(target string, s Settings) (string, Settings, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil || host == "" || port == "" {
		return "", s, apperror.Validation("target must be host:port for TCP monitors",
			apperror.FieldError{Field: "target", Message: "must be in host:port form"})
	}
	return target, s, nil
}

func validateICMP(target string, s Settings) (string, Settings, error) {
	// ICMP target is a bare host or IP; reject anything with a scheme or port.
	if strings.Contains(target, "://") || strings.Contains(target, "/") {
		return "", s, apperror.Validation("ICMP target must be a hostname or IP",
			apperror.FieldError{Field: "target", Message: "must be a hostname or IP"})
	}
	return target, s, nil
}

func validateDNS(target string, s Settings) (string, Settings, error) {
	// For DNS, target is the resolver-independent query name; DNSQueryName may
	// override it. The probe queries the system resolver.
	if s.DNSQueryName == "" {
		s.DNSQueryName = target
	}
	if s.DNSQueryType == "" {
		s.DNSQueryType = "A"
	}
	s.DNSQueryType = strings.ToUpper(s.DNSQueryType)
	if !validDNSTypes[s.DNSQueryType] {
		return "", s, apperror.Validation("invalid DNS query type",
			apperror.FieldError{Field: "settings.dns_query_type", Message: "must be a valid DNS record type"})
	}
	return target, s, nil
}
