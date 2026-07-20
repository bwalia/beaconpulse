package auth

import (
	"context"
	"net"
	"strings"
	"time"

	"beacon/internal/platform/apperror"
)

// Signup abuse is a cost problem before it is a security one. Every organization
// created is entitled to ten monitors, and every monitor is a probe every sixty
// seconds plus a Prometheus series, for as long as it exists. A junk signup is not a
// row in a table — it is recurring infrastructure spend that nobody asked for.
//
// The rate limit in front of registration bounds how FAST they can be created. This
// bounds what can be used to create them, which is the other half: a limit of five an
// hour is still forty thousand a year from one address, and rather more from a botnet.
//
// The real control is a verification email, which needs a transactional mail provider
// this deployment does not have yet. These two checks need no infrastructure at all and
// remove the cheapest routes: an address at a domain that cannot receive mail, and an
// address at a domain that exists to be thrown away.

// disposableDomains are throwaway-inbox providers.
//
// Deliberately small and not maintained as a comprehensive list — that is a losing
// game, there are thousands and new ones daily. It covers the ones a casual abuser
// reaches for first, and every entry is a real cost avoided. Someone determined will
// register a domain, and the honest answer to that is email verification plus the rate
// limit, not a longer list.
var disposableDomains = map[string]bool{
	"mailinator.com":    true,
	"guerrillamail.com": true,
	"10minutemail.com":  true,
	"tempmail.com":      true,
	"temp-mail.org":     true,
	"throwawaymail.com": true,
	"yopmail.com":       true,
	"trashmail.com":     true,
	"getnada.com":       true,
	"dispostable.com":   true,
	"maildrop.cc":       true,
	"fakeinbox.com":     true,
	"sharklasers.com":   true,
	"grr.la":            true,
	"spam4.me":          true,
	"mohmal.com":        true,
	"emailondeck.com":   true,
	"tempr.email":       true,
	"mintemail.com":     true,
	"mailnesia.com":     true,
}

// EmailPolicy vets the address a signup is made with.
type EmailPolicy struct {
	// lookupMX is injectable so tests do not depend on live DNS.
	lookupMX func(ctx context.Context, domain string) ([]*net.MX, error)
	// enabled allows a deployment to switch the checks off entirely — a private
	// install where every user is known, and an internal mail domain would fail an
	// MX lookup from wherever this happens to run.
	enabled bool
}

func NewEmailPolicy(enabled bool) *EmailPolicy {
	return &EmailPolicy{
		enabled:  enabled,
		lookupMX: func(ctx context.Context, domain string) ([]*net.MX, error) {
			return net.DefaultResolver.LookupMX(ctx, domain)
		},
	}
}

// Check rejects an address that cannot plausibly receive mail.
//
// Not validation for its own sake: an address at a domain with no mail server is one
// nobody can ever be reached at, which means the account cannot be recovered, cannot be
// warned that its card failed, and cannot be told its monitors stopped. It is also the
// shape every scripted signup takes.
func (p *EmailPolicy) Check(ctx context.Context, email string) error {
	if !p.enabled {
		return nil
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return apperror.Validation("that email address is not valid",
			apperror.FieldError{Field: "email", Message: "must be a valid email address"})
	}
	domain := strings.ToLower(email[at+1:])

	if disposableDomains[domain] {
		return apperror.Validation("that email provider is not accepted",
			apperror.FieldError{
				Field:   "email",
				Message: "please sign up with an address you can be reached at — we need it to alert you when something breaks",
			})
	}

	// Bounded tightly: this sits in the signup request path, and a slow resolver must
	// not become a slow signup.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	mx, err := p.lookupMX(ctx, domain)
	if err != nil {
		// FAIL OPEN. A resolver hiccup, a timeout, or a domain using an unusual but
		// legal mail setup must never cost us a real customer at the moment they are
		// signing up. This check exists to raise the floor on junk, not to be the
		// arbiter of whether an address is real — and turning a DNS blip into "you
		// cannot create an account" is a far worse failure than admitting one dubious
		// signup.
		return nil
	}
	if len(mx) == 0 {
		return apperror.Validation("that email domain cannot receive mail",
			apperror.FieldError{
				Field:   "email",
				Message: "we could not find a mail server for that domain — check the spelling",
			})
	}
	return nil
}
