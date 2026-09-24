package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/githubactions"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/notification"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
	"beacon/internal/platform/ratelimit"
)

// GitHubActionsHandler serves the PUBLIC ingest endpoint the "Beacon Notify" GitHub
// Action POSTs to. Like the heartbeat ping endpoint it is UNAUTHENTICATED by design:
// the token in the URL is the credential (a capability URL). A valid failed run is
// fanned out to the org's channels through the same dispatcher the Alertmanager
// webhook uses, so github alerts reach Telegram/Slack/email/iOS push unchanged.
type GitHubActionsHandler struct {
	svc        *githubactions.Service
	dispatcher *notification.Dispatcher
	limiter    *ratelimit.KeyedLimiter
}

// NewGitHubActionsHandler builds the handler. A workflow reports once per run, so a
// generous 1 req/s per token (burst 5) is far above real use and still caps a leaked
// token hard.
func NewGitHubActionsHandler(svc *githubactions.Service, dispatcher *notification.Dispatcher) *GitHubActionsHandler {
	return &GitHubActionsHandler{
		svc:        svc,
		dispatcher: dispatcher,
		limiter:    ratelimit.New(1, 5, 50000),
	}
}

// Routes returns the PUBLIC (unauthenticated) ingest routes.
func (h *GitHubActionsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/ingest/{token}", h.ingest)
	return r
}

// ingestRequest mirrors the JSON the Action sends. Everything but status is context
// for the alert body.
type ingestRequest struct {
	Status     string `json:"status"`
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
	RunID      string `json:"run_id"`
	RunNumber  string `json:"run_number"`
	RunAttempt string `json:"run_attempt"`
	Branch     string `json:"branch"`
	SHA        string `json:"sha"`
	Actor      string `json:"actor"`
	Event      string `json:"event"`
	RunURL     string `json:"run_url"`
}

func (h *GitHubActionsHandler) ingest(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	// Rate-limit per token before any DB work, so a flood on one leaked URL cannot
	// amplify into database or notification load.
	if !h.limiter.Allow(token) {
		httpx.OK(w, map[string]any{"status": "rate_limited"})
		return
	}

	var req ingestRequest
	// Cap the body: the payload is a handful of short strings.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httpx.Error(w, r, apperror.Validation("malformed ingest payload"))
		return
	}

	ev := githubactions.Event{
		Status:     req.Status,
		Repository: req.Repository,
		Workflow:   req.Workflow,
		RunID:      req.RunID,
		RunNumber:  req.RunNumber,
		RunAttempt: req.RunAttempt,
		Branch:     req.Branch,
		SHA:        req.SHA,
		Actor:      req.Actor,
		EventName:  req.Event,
		RunURL:     req.RunURL,
	}

	res, err := h.svc.Ingest(r.Context(), token, ev)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if res.Alert != githubactions.AlertNone {
		h.dispatch(r, res, ev)
	}

	// Fast, tiny 2xx so the Action step is cheap and never slows a build.
	httpx.OK(w, map[string]any{"status": "ok"})
}

// dispatch builds the AlertEvent and fans it out on a detached context, so a slow
// notification send never holds the Action's HTTP request open.
func (h *GitHubActionsHandler) dispatch(r *http.Request, res *githubactions.Result, ev githubactions.Event) {
	m := res.Monitor
	status := notification.StatusFiring
	if res.Alert == githubactions.AlertResolved {
		status = notification.StatusResolved
	}
	now := time.Now().UTC()

	workflow := ev.Workflow
	if workflow == "" {
		workflow = "GitHub Actions workflow"
	}
	summary := fmt.Sprintf("%s failed on %s", workflow, m.Target)
	if status == notification.StatusResolved {
		summary = fmt.Sprintf("%s recovered on %s", workflow, m.Target)
	}

	event := notification.AlertEvent{
		Status:      status,
		AlertName:   "WorkflowFailed",
		Severity:    "critical",
		OrgID:       m.OrgID,
		ProjectID:   m.ProjectID,
		MonitorID:   m.ID.String(),
		MonitorName: m.Name,
		MonitorType: string(monitor.TypeGitHubActions),
		Target:      m.Target,
		Summary:     summary,
		Description: describeRun(ev),
		StartsAt:    now,
	}
	if status == notification.StatusResolved {
		event.EndsAt = now
	}

	log := logger.FromContext(r.Context())
	bg := context.WithoutCancel(r.Context())
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("github ingest dispatch panicked", slog.Any("panic", rec))
			}
		}()
		ctx, cancel := context.WithTimeout(bg, 2*time.Minute)
		defer cancel()
		h.dispatcher.DispatchAlerts(ctx, []notification.AlertEvent{event})
	}()
}

// describeRun renders the human-readable body of the alert from the run's context.
func describeRun(ev githubactions.Event) string {
	var b strings.Builder
	if ev.Workflow != "" {
		fmt.Fprintf(&b, "Workflow: %s\n", ev.Workflow)
	}
	if ev.RunNumber != "" {
		fmt.Fprintf(&b, "Run: #%s", ev.RunNumber)
		if ev.RunAttempt != "" && ev.RunAttempt != "1" {
			fmt.Fprintf(&b, " (attempt %s)", ev.RunAttempt)
		}
		b.WriteByte('\n')
	}
	if ev.Branch != "" {
		fmt.Fprintf(&b, "Branch: %s\n", ev.Branch)
	}
	if ev.EventName != "" {
		fmt.Fprintf(&b, "Trigger: %s\n", ev.EventName)
	}
	if ev.Actor != "" {
		fmt.Fprintf(&b, "By: %s\n", ev.Actor)
	}
	if ev.RunURL != "" {
		fmt.Fprintf(&b, "%s", ev.RunURL)
	}
	return strings.TrimRight(b.String(), "\n")
}
