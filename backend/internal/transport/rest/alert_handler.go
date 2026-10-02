package rest

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
)

// AlertHandler receives Alertmanager webhook deliveries and fans them out to the
// notification dispatcher. It is unauthenticated by JWT (Alertmanager cannot
// present one) and instead validated by a shared bearer token.
type AlertHandler struct {
	dispatcher   *notification.Dispatcher
	webhookToken string
	// operators receives platform-health alerts (label scope="platform"). Nil = they
	// are logged and dropped (no platform email relay configured).
	operators OperatorAlerter
}

// OperatorAlerter delivers a platform-health alert to the platform operators.
// Implemented by the notifier package over the platform SMTP relay.
type OperatorAlerter interface {
	NotifyOperators(ctx context.Context, msg notification.Message) error
}

// NewAlertHandler builds an AlertHandler. operators may be nil.
func NewAlertHandler(dispatcher *notification.Dispatcher, webhookToken string, operators OperatorAlerter) *AlertHandler {
	return &AlertHandler{dispatcher: dispatcher, webhookToken: webhookToken, operators: operators}
}

// platformMessage renders a platform-scoped alert for the operators' inbox.
func platformMessage(status notification.AlertStatus, labels, annotations map[string]string, startsAt, endsAt time.Time) notification.Message {
	title := annotations["summary"]
	if title == "" {
		title = labels["alertname"]
	}
	msg := notification.Message{
		Status:      status,
		Severity:    labels["severity"],
		Title:       title,
		Environment: labels["cluster"],
		Description: annotations["description"],
		Timestamp:   startsAt,
	}
	if status == notification.StatusResolved && !endsAt.IsZero() {
		msg.Timestamp = endsAt
		msg.Duration = endsAt.Sub(startsAt)
	}
	return msg
}

// alertmanagerPayload mirrors the subset of the Alertmanager webhook body Beacon
// consumes. See https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type alertmanagerPayload struct {
	Alerts []struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
		EndsAt      time.Time         `json:"endsAt"`
	} `json:"alerts"`
}

// Webhook validates the shared secret, parses the payload, and dispatches.
func (h *AlertHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		httpx.Error(w, r, apperror.Unauthorized("invalid webhook token"))
		return
	}

	var payload alertmanagerPayload
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&payload); err != nil {
		httpx.Error(w, r, apperror.Validation("malformed alertmanager payload"))
		return
	}

	events := make([]notification.AlertEvent, 0, len(payload.Alerts))
	var platform []notification.Message
	for _, a := range payload.Alerts {
		status := notification.StatusFiring
		if strings.EqualFold(a.Status, "resolved") {
			status = notification.StatusResolved
		}
		// Platform health (the chart's platform rules) goes to the operators, never
		// to a tenant — these alerts carry no org_id by design.
		if a.Labels["scope"] == "platform" {
			platform = append(platform, platformMessage(status, a.Labels, a.Annotations, a.StartsAt, a.EndsAt))
			continue
		}
		orgID, err := uuid.Parse(a.Labels["org_id"])
		if err != nil {
			continue // an alert we cannot attribute to a tenant is skipped
		}
		projectID, _ := uuid.Parse(a.Labels["project_id"])

		events = append(events, notification.AlertEvent{
			Status:      status,
			AlertName:   a.Labels["alertname"],
			Severity:    a.Labels["severity"],
			OrgID:       orgID,
			ProjectID:   projectID,
			MonitorID:   a.Labels["monitor_id"],
			MonitorName: a.Labels["monitor_name"],
			MonitorType: a.Labels["monitor_type"],
			Target:      a.Labels["instance"],
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			StartsAt:    a.StartsAt,
			EndsAt:      a.EndsAt,
		})
	}

	// Dispatch is best-effort and may involve slow network calls (AI enrichment,
	// then per-channel delivery). We run it in the background on a context
	// detached from the request — so a slow model can't hold the Alertmanager
	// connection open and trigger a webhook timeout + retry — but retaining the
	// request's values (request id, logger) for correlated logs.
	log := logger.FromContext(r.Context())
	bg := context.WithoutCancel(r.Context())
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("alert dispatch panicked", slog.Any("panic", rec))
			}
		}()
		ctx, cancel := context.WithTimeout(bg, 2*time.Minute)
		defer cancel()
		for _, msg := range platform {
			if h.operators == nil {
				log.Warn("platform alert dropped: no operator email relay configured", slog.String("alert", msg.Title))
				continue
			}
			if err := h.operators.NotifyOperators(ctx, msg); err != nil {
				log.Error("platform alert email failed", slog.String("alert", msg.Title), slog.String("error", err.Error()))
			}
		}
		if len(events) > 0 {
			h.dispatcher.DispatchAlerts(ctx, events)
		}
	}()

	httpx.OK(w, map[string]any{"received": len(events) + len(platform)})
}

// authorized checks the bearer token when one is configured. When no token is
// set (development), all requests are accepted.
func (h *AlertHandler) authorized(r *http.Request) bool {
	if h.webhookToken == "" {
		return true
	}
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	token := strings.TrimSpace(got[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.webhookToken)) == 1
}
