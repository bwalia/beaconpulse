package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/domain/insight"
	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// MonitorHandler exposes monitor CRUD, enable/pause, and per-monitor metrics.
type MonitorHandler struct {
	svc       *monitor.Service
	insight   *insight.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewMonitorHandler builds a MonitorHandler.
func NewMonitorHandler(svc *monitor.Service, insightSvc *insight.Service, v *validate.Validator, a *middleware.Authenticator) *MonitorHandler {
	return &MonitorHandler{svc: svc, insight: insightSvc, validator: v, auth: a}
}

// Routes returns the authenticated monitor routes.
func (h *MonitorHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.list)
	r.Get("/usage", h.usage)
	r.Get("/{id}", h.get)
	r.Get("/{id}/metrics", h.metrics)
	r.With(h.auth.RequireWriter).Post("/", h.create)
	r.With(h.auth.RequireWriter).Patch("/{id}", h.update)
	r.With(h.auth.RequireWriter).Delete("/{id}", h.delete)
	r.With(h.auth.RequireWriter).Post("/{id}/pause", h.pause)
	r.With(h.auth.RequireWriter).Post("/{id}/resume", h.resume)
	return r
}

// ---- DTOs ----

type monitorSettingsDTO struct {
	Method                string            `json:"method,omitempty" validate:"omitempty,oneof=GET POST HEAD PUT DELETE PATCH"`
	ValidStatusCodes      []int             `json:"valid_status_codes,omitempty" validate:"omitempty,dive,gte=100,lte=599"`
	BodyKeyword           string            `json:"body_keyword,omitempty" validate:"omitempty,max=500"`
	BodyNotKeyword        string            `json:"body_not_keyword,omitempty" validate:"omitempty,max=500"`
	FollowRedirects       bool              `json:"follow_redirects,omitempty"`
	Headers               map[string]string `json:"headers,omitempty"`
	SkipTLSVerify         bool              `json:"skip_tls_verify,omitempty"`
	SSLExpiryWarningDays  int               `json:"ssl_expiry_warning_days,omitempty" validate:"omitempty,gte=1,lte=825"`
	ResponseTimeWarningMS int               `json:"response_time_warning_ms,omitempty" validate:"omitempty,gte=1"`
	AlertSensitivity      string            `json:"alert_sensitivity,omitempty" validate:"omitempty,oneof=immediate balanced relaxed"`
	DNSQueryName          string            `json:"dns_query_name,omitempty" validate:"omitempty,max=253"`
	DNSQueryType          string            `json:"dns_query_type,omitempty" validate:"omitempty,oneof=A AAAA CNAME MX TXT NS SOA CAA"`
	DNSExpectedIPs        []string          `json:"dns_expected_ips,omitempty"`
}

func (d monitorSettingsDTO) toDomain() monitor.Settings {
	return monitor.Settings{
		Method:                d.Method,
		ValidStatusCodes:      d.ValidStatusCodes,
		BodyKeyword:           d.BodyKeyword,
		BodyNotKeyword:        d.BodyNotKeyword,
		FollowRedirects:       d.FollowRedirects,
		Headers:               d.Headers,
		SkipTLSVerify:         d.SkipTLSVerify,
		SSLExpiryWarningDays:  d.SSLExpiryWarningDays,
		ResponseTimeWarningMS: d.ResponseTimeWarningMS,
		AlertSensitivity:      d.AlertSensitivity,
		DNSQueryName:          d.DNSQueryName,
		DNSQueryType:          d.DNSQueryType,
		DNSExpectedIPs:        d.DNSExpectedIPs,
	}
}

type createMonitorRequest struct {
	ProjectID       string             `json:"project_id" validate:"required,uuid"`
	Name            string             `json:"name" validate:"required,min=1,max=200"`
	Type            string             `json:"type" validate:"required,oneof=http https ssl tcp icmp dns"`
	Target          string             `json:"target" validate:"required,max=2048"`
	Enabled         *bool              `json:"enabled"`
	Public          *bool              `json:"public"`
	IntervalSeconds int                `json:"interval_seconds" validate:"omitempty,gte=10,lte=86400"`
	TimeoutSeconds  int                `json:"timeout_seconds" validate:"omitempty,gte=1,lte=300"`
	Settings        monitorSettingsDTO `json:"settings"`
}

type updateMonitorRequest struct {
	Name            *string             `json:"name" validate:"omitempty,min=1,max=200"`
	Target          *string             `json:"target" validate:"omitempty,max=2048"`
	Enabled         *bool               `json:"enabled"`
	Public          *bool               `json:"public"`
	IntervalSeconds *int                `json:"interval_seconds" validate:"omitempty,gte=10,lte=86400"`
	TimeoutSeconds  *int                `json:"timeout_seconds" validate:"omitempty,gte=1,lte=300"`
	Settings        *monitorSettingsDTO `json:"settings"`
}

type monitorResponse struct {
	ID              string           `json:"id"`
	OrgID           string           `json:"org_id"`
	ProjectID       string           `json:"project_id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Target          string           `json:"target"`
	Enabled         bool             `json:"enabled"`
	Public          bool             `json:"public"`
	IntervalSeconds int              `json:"interval_seconds"`
	TimeoutSeconds  int              `json:"timeout_seconds"`
	Settings        monitor.Settings `json:"settings"`
	LastStatus      string           `json:"last_status"`
	LastCheckedAt   *time.Time       `json:"last_checked_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

func presentMonitor(m *monitor.Monitor) monitorResponse {
	return monitorResponse{
		ID:              m.ID.String(),
		OrgID:           m.OrgID.String(),
		ProjectID:       m.ProjectID.String(),
		Name:            m.Name,
		Type:            string(m.Type),
		Target:          m.Target,
		Enabled:         m.Enabled,
		Public:          m.Public,
		IntervalSeconds: m.IntervalSeconds,
		TimeoutSeconds:  m.TimeoutSeconds,
		Settings:        m.Settings,
		LastStatus:      string(m.LastStatus),
		LastCheckedAt:   m.LastCheckedAt,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// ---- handlers ----

func (h *MonitorHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r, 50, 200)
	f := monitor.ListFilter{
		Type:   r.URL.Query().Get("type"),
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
		Limit:  limit,
		Offset: offset,
	}
	if pid := r.URL.Query().Get("project_id"); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil {
			httpx.Error(w, r, apperror.Validation("invalid project_id filter",
				apperror.FieldError{Field: "project_id", Message: "must be a valid UUID"}))
			return
		}
		f.ProjectID = &id
	}
	if e := r.URL.Query().Get("enabled"); e == "true" || e == "false" {
		v := e == "true"
		f.Enabled = &v
	}

	items, total, err := h.svc.List(r.Context(), monitorActor(r), f)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]monitorResponse, 0, len(items))
	for i := range items {
		out = append(out, presentMonitor(&items[i]))
	}
	httpx.OK(w, newListResponse(out, total, limit, offset))
}

type usageResponse struct {
	Plan               string `json:"plan"`
	MonitorsUsed       int    `json:"monitors_used"`
	MonitorsLimit      int    `json:"monitors_limit"`
	MinIntervalSeconds int    `json:"min_interval_seconds"`
}

// usage returns the caller org's monitor usage against its plan limits.
func (h *MonitorHandler) usage(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Usage(r.Context(), monitorActor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, usageResponse{
		Plan:               u.Plan,
		MonitorsUsed:       u.MonitorsUsed,
		MonitorsLimit:      u.MaxMonitors,
		MinIntervalSeconds: u.MinIntervalSeconds,
	})
}

func (h *MonitorHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	m, err := h.svc.Get(r.Context(), monitorActor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentMonitor(m))
}

func (h *MonitorHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createMonitorRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		httpx.Error(w, r, apperror.Validation("invalid project_id",
			apperror.FieldError{Field: "project_id", Message: "must be a valid UUID"}))
		return
	}
	m, err := h.svc.Create(r.Context(), monitorActor(r), monitor.CreateInput{
		ProjectID:       projectID,
		Name:            req.Name,
		Type:            monitor.Type(req.Type),
		Target:          req.Target,
		Enabled:         req.Enabled,
		Public:          req.Public,
		IntervalSeconds: req.IntervalSeconds,
		TimeoutSeconds:  req.TimeoutSeconds,
		Settings:        req.Settings.toDomain(),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, presentMonitor(m))
}

func (h *MonitorHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateMonitorRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	in := monitor.UpdateInput{
		Name:            req.Name,
		Target:          req.Target,
		Enabled:         req.Enabled,
		Public:          req.Public,
		IntervalSeconds: req.IntervalSeconds,
		TimeoutSeconds:  req.TimeoutSeconds,
	}
	if req.Settings != nil {
		s := req.Settings.toDomain()
		in.Settings = &s
	}
	m, err := h.svc.Update(r.Context(), monitorActor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentMonitor(m))
}

func (h *MonitorHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), monitorActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

type metricPoint struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type monitorMetricsResponse struct {
	MonitorID         string        `json:"monitor_id"`
	WindowHours       int           `json:"window_hours"`
	UptimePercent     float64       `json:"uptime_percent"`
	ResponseMsCurrent float64       `json:"response_ms_current"`
	ResponseMsAvg     float64       `json:"response_ms_avg"`
	Up                []metricPoint `json:"up"`
	ResponseMs        []metricPoint `json:"response_ms"`
}

// metrics returns org-scoped Prometheus metrics for a single monitor. The
// insight service verifies the monitor belongs to the caller's org.
func (h *MonitorHandler) metrics(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	p := mustPrincipal(r)
	m, err := h.insight.MonitorMetrics(r.Context(), p.OrgID, id, 24*time.Hour)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, monitorMetricsResponse{
		MonitorID:         m.MonitorID,
		WindowHours:       m.WindowHours,
		UptimePercent:     m.UptimePercent,
		ResponseMsCurrent: m.ResponseMsCurrent,
		ResponseMsAvg:     m.ResponseMsAvg,
		Up:                toMetricPoints(m.Up),
		ResponseMs:        toMetricPoints(m.ResponseMs),
	})
}

func toMetricPoints(pts []insight.Point) []metricPoint {
	out := make([]metricPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, metricPoint{T: p.T, V: p.V})
	}
	return out
}

func (h *MonitorHandler) pause(w http.ResponseWriter, r *http.Request)  { h.setEnabled(w, r, false) }
func (h *MonitorHandler) resume(w http.ResponseWriter, r *http.Request) { h.setEnabled(w, r, true) }

func (h *MonitorHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	m, err := h.svc.SetEnabled(r.Context(), monitorActor(r), id, enabled)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentMonitor(m))
}
