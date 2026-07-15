package rest

import (
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/insight"
	"beacon/internal/domain/maintenance"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
)

// overviewBuckets is the number of samples every window is reduced to. Keeping it
// constant means the *step* widens with the window (24h → 30m slots, 30d → 15h
// slots) rather than the sample count exploding, so a 30-day request costs
// Prometheus the same as a 1-hour one.
const overviewBuckets = 48

// allowedWindowHours bounds what a caller may ask for. Without an allowlist a
// client could request an arbitrarily long range and make Prometheus do unbounded
// work on our behalf.
var allowedWindowHours = map[int]bool{1: true, 6: true, 24: true, 168: true, 720: true}

// parseWindowHours reads the optional `hours` query value, defaulting to 24.
func parseWindowHours(raw string) (int, error) {
	if raw == "" {
		return 24, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || !allowedWindowHours[n] {
		return 0, apperror.Validation("hours must be one of: 1, 6, 24, 168, 720")
	}
	return n, nil
}

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

// Overview returns the org-wide dashboard metrics for the requested window
// (?hours=1|6|24|168|720, default 24).
func (h *InsightHandler) Overview(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	hours, err := parseWindowHours(r.URL.Query().Get("hours"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	o, err := h.svc.Overview(r.Context(), p.OrgID, time.Duration(hours)*time.Hour, overviewBuckets)
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
	svc   *insight.Service
	maint *maintenance.Service
}

// NewInsightHandler builds an InsightHandler. maint may be nil, in which case
// alerts are returned without maintenance annotations.
func NewInsightHandler(svc *insight.Service, maint *maintenance.Service) *InsightHandler {
	return &InsightHandler{svc: svc, maint: maint}
}

type alertResponse struct {
	Name        string     `json:"name"`
	Severity    string     `json:"severity"`
	MonitorID   string     `json:"monitor_id"`
	MonitorName string     `json:"monitor_name"`
	MonitorType string     `json:"monitor_type"`
	Target      string     `json:"target"`
	Since       *time.Time `json:"since,omitempty"`
	// InMaintenance is true when the alert's monitor is under an active window — its
	// notification was suppressed, so the UI can label it rather than imply it paged.
	InMaintenance bool `json:"in_maintenance"`
}

// ActiveAlerts returns the firing alerts for the caller's organization. Filtering
// (by severity) and pagination happen server-side: the alerts come from Prometheus
// in one read, then the handler sorts (critical first), filters, and slices — so a
// large incident doesn't ship hundreds of rows to the browser at once.
func (h *InsightHandler) ActiveAlerts(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	limit, offset := paginationParams(r, 20, 200)
	severity := r.URL.Query().Get("severity")

	alerts, err := h.svc.ActiveAlerts(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	// Batch-resolve which monitors are under maintenance so suppressed alerts can be
	// labelled. Best-effort: on failure, no labels rather than a failed request.
	var underMaintenance map[uuid.UUID]bool
	if h.maint != nil {
		if m, mErr := h.maint.ActiveMonitorIDs(r.Context(), p.OrgID, time.Now().UTC()); mErr != nil {
			logger.FromContext(r.Context()).Warn("active alerts: maintenance annotation failed",
				slog.String("error", mErr.Error()))
		} else {
			underMaintenance = m
		}
	}

	all := make([]alertResponse, 0, len(alerts))
	for _, a := range alerts {
		if severity != "" && a.Severity != severity {
			continue
		}
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
		if underMaintenance != nil {
			if id, pErr := uuid.Parse(a.MonitorID); pErr == nil && underMaintenance[id] {
				ar.InMaintenance = true
			}
		}
		all = append(all, ar)
	}

	// Stable, meaningful order so pages don't reshuffle: critical first, then name.
	sort.SliceStable(all, func(i, j int) bool {
		ci, cj := all[i].Severity == "critical", all[j].Severity == "critical"
		if ci != cj {
			return ci
		}
		return all[i].Name < all[j].Name
	})

	total := len(all)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	httpx.OK(w, newListResponse(all[start:end], total, limit, offset))
}
