// Package githubactions is the bounded context for the PUSH side of github_actions
// monitors: recording the result of a workflow run reported by the "Beacon Notify"
// GitHub Action.
//
// Like the heartbeat context it is deliberately small and separate, because it is
// reached over an UNAUTHENTICATED endpoint — the token in the URL is the only
// credential. It resolves a monitor by the hash of that token, decides whether the
// reported run should alert, and stamps the monitor's status. It never calls the
// GitHub API (so there is no rate limit to hit) and never reads a stored GitHub
// credential (there is none — only our own ingest token's hash is kept).
package githubactions

import (
	"context"
	"strings"
	"time"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
)

// Event is one workflow run's outcome as reported by the Action. All fields are
// best-effort context for the alert; only Status drives the decision.
type Event struct {
	// Status is the job/run status the Action reports (GitHub's own vocabulary:
	// success, failure, cancelled, …). Case-insensitive.
	Status     string
	Repository string // owner/repo
	Workflow   string // workflow name, e.g. "CI"
	RunID      string
	RunNumber  string
	RunAttempt string
	Branch     string
	SHA        string
	Actor      string
	EventName  string // the git event that triggered the run (push, pull_request, …)
	RunURL     string // link to the run on github.com
}

// AlertKind is what an ingested event should trigger.
type AlertKind int

const (
	// AlertNone: the run needs no notification (a routine success on a monitor that
	// does not confirm successes, or an outcome we neither alert nor recover on,
	// like cancelled).
	AlertNone AlertKind = iota
	// AlertFiring: a failed run — page the org's channels.
	AlertFiring
	// AlertResolved: a success that follows a failure — send the recovery notice.
	AlertResolved
	// AlertSucceeded: a success on a monitor that confirms every successful run
	// (GitHubNotifyOnSuccess) and was not previously down — a positive "it ran".
	AlertSucceeded
)

// Result is the decision for one ingested event. The transport turns it into a
// notification so this context stays free of any dependency on the notification
// domain.
type Result struct {
	// Ignored is true when the event was accepted but deliberately produced no
	// action (unknown-but-valid outcome, a workflow filtered out, or a paused
	// monitor). The endpoint still returns 200 so the Action never fails a build.
	Ignored bool
	Monitor *monitor.Monitor
	Alert   AlertKind
}

// Repository is the slice of monitor persistence this context needs. It is
// satisfied by the postgres monitor repository without widening the monitor
// domain's own interface.
type Repository interface {
	// GitHubByTokenHash resolves the monitor for an ingest token's hash, or
	// (nil, nil) if none matches.
	GitHubByTokenHash(ctx context.Context, hash string) (*monitor.Monitor, error)
	// ApplyStatusUpdates writes observed statuses back (reused from the status-sync
	// path; it skips paused/disabled monitors).
	ApplyStatusUpdates(ctx context.Context, updates []monitor.StatusUpdate) (int64, error)
}

// Service records workflow-run results.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService builds a github_actions ingest Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Ingest records a run's outcome for the given ingest token and returns what, if
// anything, the caller should notify.
//
// An unknown token returns NotFound — the SAME error as any missing resource, so
// the endpoint is not an oracle for which tokens exist (the token space is 256-bit,
// so guessing is infeasible regardless).
func (s *Service) Ingest(ctx context.Context, token string, ev Event) (*Result, error) {
	if token == "" {
		return nil, apperror.NotFound("unknown ingest token")
	}
	m, err := s.repo.GitHubByTokenHash(ctx, crypto.SHA256Hex(token))
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("unknown ingest token")
	}

	// A paused monitor accepts the report (so the Action still succeeds) but stays
	// quiet — pausing a monitor is an explicit "stop alerting me about this".
	if !m.Enabled {
		return &Result{Ignored: true, Monitor: m}, nil
	}

	// Optional workflow filter: when set, only runs of that workflow act on this
	// monitor. A report for a different workflow is accepted and ignored.
	if wf := m.Settings.GitHubWorkflow; wf != "" && ev.Workflow != "" && !strings.EqualFold(wf, ev.Workflow) {
		return &Result{Ignored: true, Monitor: m}, nil
	}

	status, alertable := classify(ev.Status)
	if !alertable {
		// An outcome we neither fail nor recover on (cancelled, skipped, …): leave
		// the monitor's state untouched and send nothing.
		return &Result{Ignored: true, Monitor: m}, nil
	}

	res := &Result{Monitor: m, Alert: AlertNone}
	switch status {
	case monitor.StatusDown:
		// Every failed run is a real, distinct event — alert on each, matching the
		// product promise "notify whenever a workflow fails".
		res.Alert = AlertFiring
	case monitor.StatusUp:
		switch {
		case m.LastStatus == monitor.StatusDown:
			// Recovery: we were down and are now green — always worth announcing.
			res.Alert = AlertResolved
		case m.Settings.GitHubNotifyOnSuccess:
			// This monitor confirms every successful run (e.g. a backup), so send a
			// positive green notice even though nothing was broken.
			res.Alert = AlertSucceeded
		}
		// Otherwise a routine green run is not news — stay quiet.
	}

	// Record the new status (best-effort: a bookkeeping failure must not lose the
	// alert the caller is about to send).
	_, _ = s.repo.ApplyStatusUpdates(ctx, []monitor.StatusUpdate{{
		MonitorID: m.ID, Status: status, CheckedAt: s.now().UTC(),
	}})
	return res, nil
}

// classify maps a reported job/run status to a monitor status and whether it is an
// outcome we act on. Failure-like outcomes go down; success goes up; anything else
// (cancelled, skipped, neutral, in-progress, empty) is accepted but not acted on.
func classify(status string) (monitor.Status, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failure", "failed", "timed_out", "startup_failure", "action_required":
		return monitor.StatusDown, true
	case "success", "succeeded":
		return monitor.StatusUp, true
	default:
		return monitor.StatusUnknown, false
	}
}
