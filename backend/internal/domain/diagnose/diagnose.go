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
	"errors"
	"fmt"
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

// Meter prices a diagnosis. The two plan shapes need two different answers —
// pay-as-you-go spends credit per run, a subscription spends a monthly allowance —
// and the service picks; the Meter only does what it is told.
type Meter interface {
	// ChargeCredit debits costSeconds in ONE statement, refusing rather than
	// overdrawing, and reports whether it succeeded. Atomicity is the point: a
	// read-then-write would let two fast clicks both see a sufficient balance and
	// both spend it, handing out a diagnosis nobody paid for.
	ChargeCredit(ctx context.Context, orgID uuid.UUID, costSeconds int64) (bool, error)
	// RefundCredit returns a charge for work that was not delivered.
	RefundCredit(ctx context.Context, orgID uuid.UUID, costSeconds int64) error
	// CountRunsSince counts recorded diagnoses, for the subscription quota.
	CountRunsSince(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error)
	// RecordRun writes a delivered diagnosis to the ledger.
	RecordRun(ctx context.Context, orgID, monitorID uuid.UUID, costSeconds int64) error
}

// Service runs diagnoses.
type Service struct {
	monitors    MonitorReader
	plans       OrgPlanReader
	prober      Prober
	explain     Explainer
	meter       Meter
	costSeconds int64
	now         func() time.Time
}

// NewService builds a Service. explain may be nil when no model is configured: the
// probes still run and the evidence is still returned, because the measurements are
// the part that cannot be guessed at.
//
// costSeconds is what one diagnosis costs a pay-as-you-go org, in monitor-seconds.
func NewService(monitors MonitorReader, plans OrgPlanReader, prober Prober, explain Explainer, meter Meter, costSeconds int64) *Service {
	if costSeconds <= 0 {
		costSeconds = DefaultCostSeconds
	}
	return &Service{
		monitors: monitors, plans: plans, prober: prober, explain: explain,
		meter: meter, costSeconds: costSeconds, now: time.Now,
	}
}

// DefaultCostSeconds is 30 monitor-minutes, about 10¢ at the standard rate.
//
// Set well above what a diagnosis actually costs to serve, but deliberately low
// enough that nobody rations themselves: this button is pressed during an outage, and
// a price that makes someone hesitate to ask why their site is down has defeated the
// feature it is protecting.
const DefaultCostSeconds int64 = 30 * 60

// monthStart is when the subscription allowance last reset: the 1st, UTC. A calendar
// month rather than a rolling window because a quota you cannot predict the reset of
// is one you have to ration against, and UTC so the answer does not depend on where
// the reader is standing.
func monthStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
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

	// Pay for it before doing it. Charging afterwards would let a handful of
	// simultaneous clicks all pass the balance check and all be served.
	charged, err := s.meterIn(ctx, actor.OrgID, p)
	if err != nil {
		return nil, err
	}

	// From here the org has paid, so every path out either delivers an analysis or
	// gives the money back. Nothing in between.
	ev, err := s.prober.Probe(ctx, target, monitorType)
	if err != nil {
		s.refund(ctx, actor.OrgID, charged)
		return nil, err
	}

	out := &Diagnosis{Evidence: ev}
	analysis, aerr := s.analyze(ctx, ev)
	if aerr != nil {
		// The model failed, so this is not the thing they paid for. Refund and keep
		// the evidence: measurements are still worth reading, but they are not a
		// diagnosis and must not be billed as one.
		s.refund(ctx, actor.OrgID, charged)
		out.AnalysisError = aerr.Error()
		return out, nil
	}

	out.Analysis = analysis
	// Recorded only now, once an answer exists. A quota spent on an answer nobody
	// received is the same theft as a charge for one.
	if err := s.meter.RecordRun(ctx, actor.OrgID, monitorID, charged); err != nil {
		// The diagnosis is done and paid for; failing the request now would charge
		// them for something we then refuse to hand over. Under-counting a quota is
		// the cheaper mistake.
		return out, nil
	}
	return out, nil
}

// meterIn takes payment for one diagnosis and returns what was charged in
// monitor-seconds (zero for a subscription, which spends allowance instead).
func (s *Service) meterIn(ctx context.Context, orgID uuid.UUID, p plan.Plan) (int64, error) {
	if p == plan.PayAsYouGo {
		ok, err := s.meter.ChargeCredit(ctx, orgID, s.costSeconds)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, apperror.Validation("not enough credit for an AI diagnosis",
				apperror.FieldError{
					Field: "credit",
					Message: fmt.Sprintf("a diagnosis costs %d monitor-minutes of credit — add more to run one",
						s.costSeconds/60),
				})
		}
		return s.costSeconds, nil
	}

	// A subscribed tier: flat fee, so the ceiling is a count rather than a balance.
	limit := plan.LimitsFor(p).MonthlyDiagnoses
	used, err := s.meter.CountRunsSince(ctx, orgID, monthStart(s.now()))
	if err != nil {
		return 0, err
	}
	if used >= limit {
		return 0, apperror.Validation("monthly AI diagnosis limit reached",
			apperror.FieldError{
				Field: "plan",
				Message: fmt.Sprintf("your plan includes %d diagnoses per month and %d have been used; the allowance resets on the 1st",
					limit, used),
			})
	}
	return 0, nil
}

// refund returns a charge. Best-effort by design: the caller is already handling a
// failure, and turning a failed diagnosis into a failed request as well would leave
// the user with neither an answer nor an explanation.
func (s *Service) refund(ctx context.Context, orgID uuid.UUID, charged int64) {
	if charged > 0 {
		_ = s.meter.RefundCredit(ctx, orgID, charged)
	}
}

func (s *Service) analyze(ctx context.Context, ev Evidence) (*Analysis, error) {
	if s.explain == nil {
		return nil, errors.New("AI analysis is not configured on this deployment")
	}
	a, err := s.explain.Explain(ctx, ev)
	if err != nil {
		return nil, errors.New("AI analysis is unavailable right now — you have not been charged, and the measurements below are still complete")
	}
	return a, nil
}
