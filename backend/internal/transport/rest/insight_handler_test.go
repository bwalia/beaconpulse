package rest

import (
	"testing"

	"beacon/internal/platform/apperror"
)

func TestParseWindowHoursDefaults(t *testing.T) {
	got, err := parseWindowHours("")
	if err != nil {
		t.Fatalf("empty should default, got error: %v", err)
	}
	if got != 24 {
		t.Errorf("default = %d, want 24", got)
	}
}

func TestParseWindowHoursAllowed(t *testing.T) {
	for _, h := range []int{1, 6, 24, 168, 720} {
		got, err := parseWindowHours(itoa(h))
		if err != nil {
			t.Errorf("hours=%d rejected: %v", h, err)
			continue
		}
		if got != h {
			t.Errorf("hours=%d parsed as %d", h, got)
		}
	}
}

func TestParseWindowHoursRejectsUnlisted(t *testing.T) {
	// Anything outside the allowlist must be refused — an unbounded window would
	// let a caller make Prometheus do arbitrary work.
	for _, raw := range []string{"0", "-1", "23", "9999", "abc", "24h", "1e3", " 24"} {
		if _, err := parseWindowHours(raw); err == nil {
			t.Errorf("hours=%q was accepted, want validation error", raw)
		} else if !apperror.IsCode(err, apperror.CodeValidation) {
			t.Errorf("hours=%q gave %v, want a validation error", raw, err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
