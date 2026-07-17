// Package diagnose answers "why is this monitor down?" for a paying organization.
//
// An alert says a thing is down. This says why, and what to do about it, by probing
// the target the way an engineer would — resolve the name, open the port, inspect the
// certificate, make the request — and then having a model read the results back in
// plain English.
//
// The division of labour is the whole design, and it is not negotiable: the PROBES
// gather facts, the MODEL only interprets them. A language model cannot resolve a
// name or read a certificate; asked to, it produces a fluent, well-formatted, wholly
// invented answer. Confidently wrong root-cause analysis is worse than none at all
// during an outage, because it is acted on. So every fact here is measured, the model
// receives only measurements, and it is told to reason from them and nothing else.
package diagnose

import (
	"context"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

// Actor is the authenticated caller.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}

// DNSFinding is what the resolver said about the target's name.
type DNSFinding struct {
	// Resolved is false when the name did not resolve at all — on its own, usually
	// the whole answer.
	Resolved bool          `json:"resolved"`
	Addresses []string     `json:"addresses,omitempty"`
	CNAME     string       `json:"cname,omitempty"`
	Nameservers []string   `json:"nameservers,omitempty"`
	LookupMS  int64        `json:"lookup_ms"`
	Error     string       `json:"error,omitempty"`
	took      time.Duration
}

// TCPFinding is whether the port accepts connections, which separates "the host is
// gone" from "the service on it is not listening".
type TCPFinding struct {
	Attempted bool   `json:"attempted"`
	Connected bool   `json:"connected"`
	Address   string `json:"address,omitempty"`
	ConnectMS int64  `json:"connect_ms"`
	Error     string `json:"error,omitempty"`
}

// TLSFinding covers the certificate — the quiet cause of a great many outages that
// look like nothing else.
type TLSFinding struct {
	Attempted     bool      `json:"attempted"`
	HandshakeOK   bool      `json:"handshake_ok"`
	Issuer        string    `json:"issuer,omitempty"`
	Subject       string    `json:"subject,omitempty"`
	NotAfter      time.Time `json:"not_after,omitzero"`
	DaysRemaining int       `json:"days_remaining,omitempty"`
	Expired       bool      `json:"expired"`
	HostnameOK    bool      `json:"hostname_ok"`
	Error         string    `json:"error,omitempty"`
}

// HTTPFinding is the request itself, when the target speaks HTTP.
type HTTPFinding struct {
	Attempted    bool     `json:"attempted"`
	StatusCode   int      `json:"status_code,omitempty"`
	ResponseMS   int64    `json:"response_ms"`
	RedirectChain []string `json:"redirect_chain,omitempty"`
	Server       string   `json:"server,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Evidence is everything the probes measured. It is returned to the caller alongside
// the model's reading of it, deliberately: the facts are checkable, the prose is not,
// and an engineer mid-incident deserves to see what was actually observed rather than
// being asked to trust a summary.
type Evidence struct {
	Target      string      `json:"target"`
	MonitorType string      `json:"monitor_type"`
	CheckedAt   time.Time   `json:"checked_at"`
	DNS         DNSFinding  `json:"dns"`
	TCP         TCPFinding  `json:"tcp"`
	TLS         TLSFinding  `json:"tls"`
	HTTP        HTTPFinding `json:"http"`
}

// Analysis is the model's reading of the Evidence.
type Analysis struct {
	Summary      string `json:"summary"`
	LikelyCause  string `json:"likely_cause"`
	SuggestedFix string `json:"suggested_fix"`
	// Confidence is the model's own hedge. Surfaced because a diagnosis the model is
	// unsure of should not read with the same authority as one it is certain about.
	Confidence string `json:"confidence"`
}

// Diagnosis is the whole answer: what was measured, and what it means.
type Diagnosis struct {
	Evidence Evidence  `json:"evidence"`
	Analysis *Analysis `json:"analysis,omitempty"`
	// AnalysisError explains why the prose is missing when it is. The evidence is
	// still worth returning without it — a certificate that expired yesterday says
	// the same thing whether or not a model got to phrase it.
	AnalysisError string `json:"analysis_error,omitempty"`
}

// Prober measures a target. Implemented by an adapter; the domain never opens a
// socket itself, which is what keeps this package testable without a network.
type Prober interface {
	Probe(ctx context.Context, target, monitorType string) (Evidence, error)
}

// Explainer turns measurements into prose. Implemented by the AI adapter.
type Explainer interface {
	Explain(ctx context.Context, ev Evidence) (*Analysis, error)
}

// MonitorReader resolves a monitor the caller owns.
type MonitorReader interface {
	// TargetFor returns the monitor's target and type, scoped to the org, so a
	// diagnosis can never be pointed at another tenant's monitor by guessing an id.
	TargetFor(ctx context.Context, orgID, monitorID uuid.UUID) (target, monitorType string, err error)
}

// OrgPlanReader reports an org's EFFECTIVE plan.
type OrgPlanReader interface {
	Plan(ctx context.Context, orgID uuid.UUID) (plan.Plan, error)
}

// Service runs diagnoses.
type Service struct {
	monitors MonitorReader
	plans    OrgPlanReader
	prober   Prober
	explain  Explainer
}

// NewService builds a Service. explain may be nil when no model is configured: the
// probes still run and the evidence is still returned, because the measurements are
// the part that cannot be guessed at.
func NewService(monitors MonitorReader, plans OrgPlanReader, prober Prober, explain Explainer) *Service {
	return &Service{monitors: monitors, plans: plans, prober: prober, explain: explain}
}

// ErrPaidPlanRequired is returned to a Free org. It is a Validation rather than a
// Forbidden on purpose: nothing about the caller is wrong, and the UI turns it into
// an upsell rather than an error.
func errPaidPlanRequired() error {
	return apperror.Validation("AI diagnosis is available on paid plans",
		apperror.FieldError{
			Field:   "plan",
			Message: "add credit or subscribe to diagnose a failing monitor with AI",
		})
}

// Run diagnoses one of the org's monitors.
func (s *Service) Run(ctx context.Context, actor Actor, monitorID uuid.UUID) (*Diagnosis, error) {
	// Paid only. Effective plan, not the subscribed one, so a pay-as-you-go org with
	// credit qualifies — they are paying by the hour, which is paying.
	p, err := s.plans.Plan(ctx, actor.OrgID)
	if err != nil {
		return nil, err
	}
	if p == plan.Free {
		return nil, errPaidPlanRequired()
	}

	// Ownership is checked by the query, not after it: the org id is part of the
	// lookup, so another tenant's monitor is simply not found.
	target, monitorType, err := s.monitors.TargetFor(ctx, actor.OrgID, monitorID)
	if err != nil {
		return nil, err
	}

	ev, err := s.prober.Probe(ctx, target, monitorType)
	if err != nil {
		return nil, err
	}

	out := &Diagnosis{Evidence: ev}
	if s.explain == nil {
		out.AnalysisError = "AI analysis is not configured on this deployment"
		return out, nil
	}
	analysis, err := s.explain.Explain(ctx, ev)
	if err != nil {
		// A model that is slow, down, or talking nonsense must not cost the user the
		// evidence. Degrade to the facts rather than failing the request.
		out.AnalysisError = "AI analysis is unavailable right now — the measurements below are still complete"
		return out, nil
	}
	out.Analysis = analysis
	return out, nil
}
