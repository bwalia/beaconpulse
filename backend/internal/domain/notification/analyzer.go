package notification

import "context"

// Severity levels an Analyzer may assign. These are the model's *assessed impact*
// and are distinct from the alert's rule-defined severity label — an alert fired
// as "warning" may be assessed "high" by the model (or vice-versa).
const (
	AISeverityHigh   = "high"
	AISeverityMedium = "medium"
	AISeverityLow    = "low"
)

// AlertAnalysis is an AI-generated triage of a firing alert: how severe the model
// judges it to be, the most likely cause, and a concrete suggested fix. It is
// advisory only — a notification is delivered with or without it.
type AlertAnalysis struct {
	// Severity is the model's assessed impact: high | medium | low.
	Severity string
	// Summary is a one-line human assessment of what is happening.
	Summary string
	// LikelyCause is the model's best guess at the root cause.
	LikelyCause string
	// SuggestedFix is a concrete next step an operator can take.
	SuggestedFix string
	// Model records which model produced this analysis, for transparency.
	Model string
}

// Analyzer turns a firing AlertEvent into an AlertAnalysis by prompting an LLM.
// Implementations must honour ctx cancellation/deadline and return an error
// rather than block indefinitely; the dispatcher treats any error as "no
// analysis" and delivers the alert unenriched.
type Analyzer interface {
	Analyze(ctx context.Context, ev AlertEvent) (*AlertAnalysis, error)
}
