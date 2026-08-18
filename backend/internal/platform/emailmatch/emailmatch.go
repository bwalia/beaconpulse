// Package emailmatch matches an email address against a list of allow patterns.
// A pattern is either a full address (contains "@") or a bare domain. It backs two
// operator allowlists that share the same shape: the platform-admin list (who may
// change pricing) and the premium-grant list (which accounts get the top tier free).
package emailmatch

import "strings"

// Normalize lowercases and trims each pattern, dropping blanks. Stored patterns are
// kept normalized so Match never has to re-clean them on the hot path.
func Normalize(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Match reports whether email matches any pattern. A pattern containing "@" matches
// that exact address; a bare pattern is a domain and matches that domain and its
// subdomains — so "workstation.co.uk" matches both "me@workstation.co.uk" and
// "me@team.workstation.co.uk". Comparison is case-insensitive.
func Match(patterns []string, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	domain := ""
	if at := strings.LastIndex(email, "@"); at >= 0 {
		domain = email[at+1:]
	}
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.Contains(p, "@") {
			if p == email {
				return true
			}
			continue
		}
		if domain != "" && (domain == p || strings.HasSuffix(domain, "."+p)) {
			return true
		}
	}
	return false
}
