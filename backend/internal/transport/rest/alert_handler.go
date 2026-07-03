package rest

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
)

// AlertHandler receives Alertmanager webhook deliveries and fans them out to the
// notification dispatcher. It is unauthenticated by JWT (Alertmanager cannot
// present one) and instead validated by a shared bearer token.
type AlertHandler struct {
	dispatcher   *notification.Dispatcher
	webhookToken string
}

// NewAlertHandler builds an AlertHandler.
func NewAlertHandler(dispatcher *notification.Dispatcher, webhookToken string) *AlertHandler {
	return &AlertHandler{dispatcher: dispatcher, webhookToken: webhookToken}
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
	for _, a := range payload.Alerts {
		orgID, err := uuid.Parse(a.Labels["org_id"])
		if err != nil {
			continue // an alert we cannot attribute to a tenant is skipped
		}
		projectID, _ := uuid.Parse(a.Labels["project_id"])

		status := notification.StatusFiring
		if strings.EqualFold(a.Status, "resolved") {
			status = notification.StatusResolved
		}
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

	// Dispatch is best-effort and may involve slow network calls; run it without
	// blocking the Alertmanager response beyond the request lifetime is not
	// desirable (context would be cancelled), so we dispatch synchronously but
	// the dispatcher never fails the request.
	h.dispatcher.DispatchAlerts(r.Context(), events)
	httpx.OK(w, map[string]any{"received": len(events)})
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
