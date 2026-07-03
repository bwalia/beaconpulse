package rest

import (
	"net/http"
	"time"

	"beacon/internal/domain/insight"
	"beacon/internal/platform/httpx"
)

type monitorUptimeResponse struct {
	MonitorID     string        `json:"monitor_id"`
	MonitorName   string        `json:"monitor_name"`
	Target        string        `json:"target"`
	AvgResponseMs float64       `json:"avg_response_ms"`
	Points        []metricPoint `json:"points"`
}

type overviewResponse struct {
	WindowHours    int                     `json:"window_hours"`
	UptimePercent  float64                 `json:"uptime_percent"`
	AvgResponseMs  float64                 `json:"avg_response_ms"`
	UptimeSeries   []metricPoint           `json:"uptime_series"`
	ResponseSeries []metricPoint           `json:"response_series"`
	Monitors       []monitorUptimeResponse `json:"monitors"`
}

// Overview returns the org-wide dashboard metrics.
func (h *InsightHandler) Overview(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	o, err := h.svc.Overview(r.Context(), p.OrgID, 24*time.Hour, 48)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	monitors := make([]monitorUptimeResponse, 0, len(o.Monitors))
	for _, m := range o.Monitors {
		monitors = append(monitors, monitorUptimeResponse{
			MonitorID:     m.MonitorID,
			MonitorName:   m.MonitorName,
			Target:        m.Target,
			AvgResponseMs: m.AvgResponseMs,
			Points:        toMetricPoints(m.Points),
		})
	}
	httpx.OK(w, overviewResponse{
		WindowHours:    o.WindowHours,
		UptimePercent:  o.UptimePercent,
		AvgResponseMs:  o.AvgResponseMs,
		UptimeSeries:   toMetricPoints(o.UptimeSeries),
		ResponseSeries: toMetricPoints(o.ResponseSeries),
		Monitors:       monitors,
	})
}

// InsightHandler serves tenant-scoped read models over Prometheus (active alerts
// and, via the monitor handler, per-monitor metrics). Everything is filtered to
// the caller's organization so no cross-tenant data is exposed.
type InsightHandler struct {
	svc *insight.Service
}

// NewInsightHandler builds an InsightHandler.
func NewInsightHandler(svc *insight.Service) *InsightHandler {
	return &InsightHandler{svc: svc}
}

type alertResponse struct {
	Name        string     `json:"name"`
	Severity    string     `json:"severity"`
	MonitorID   string     `json:"monitor_id"`
	MonitorName string     `json:"monitor_name"`
	MonitorType string     `json:"monitor_type"`
	Target      string     `json:"target"`
	Since       *time.Time `json:"since,omitempty"`
}

// ActiveAlerts returns the firing alerts for the caller's organization.
func (h *InsightHandler) ActiveAlerts(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	alerts, err := h.svc.ActiveAlerts(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]alertResponse, 0, len(alerts))
	for _, a := range alerts {
		ar := alertResponse{
			Name:        a.Name,
			Severity:    a.Severity,
			MonitorID:   a.MonitorID,
			MonitorName: a.MonitorName,
			MonitorType: a.MonitorType,
			Target:      a.Target,
		}
		if !a.Since.IsZero() {
			since := a.Since
			ar.Since = &since
		}
		out = append(out, ar)
	}
	httpx.OK(w, map[string]any{"data": out})
}
