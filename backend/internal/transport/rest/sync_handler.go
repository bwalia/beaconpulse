package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/monitor"
	"beacon/internal/domain/configsync"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// SyncHandler applies a declared set of monitors — the endpoint a CI workflow calls.
type SyncHandler struct {
	svc       *configsync.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

func NewSyncHandler(svc *configsync.Service, v *validate.Validator, a *middleware.Authenticator) *SyncHandler {
	return &SyncHandler{svc: svc, validator: v, auth: a}
}

func (h *SyncHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.RequireWriter)
	r.Post("/", h.apply)
	return r
}

// syncMonitorRequest is one declared monitor.
//
// Every field a monitor can be configured with is here, because the point of the API
// is that a file in a repository can express anything the dashboard can — a workflow
// that has to fall back to the UI for one setting is a workflow nobody trusts.
type syncMonitorRequest struct {
	Name   string `json:"name" validate:"required,max=120"`
	Type   string `json:"type" validate:"required,oneof=http https ssl tcp icmp dns heartbeat"`
	Target string `json:"target" validate:"omitempty,max=500"`

	IntervalSeconds int `json:"interval_seconds" validate:"omitempty,min=10,max=86400"`
	TimeoutSeconds  int `json:"timeout_seconds" validate:"omitempty,min=1,max=300"`
	GraceSeconds    int `json:"grace_seconds" validate:"omitempty,min=0,max=86400"`

	Enabled *bool `json:"enabled"`
	Public  *bool `json:"public"`

	// Settings is the per-type configuration, matching the monitor's own shape so the
	// documented JSON and the stored JSON are the same thing.
	Settings *monitor.Settings `json:"settings"`
}

type syncRequest struct {
	// Project groups the monitors; created on first use. Empty means "Default".
	Project  string               `json:"project" validate:"omitempty,max=120"`
	Monitors []syncMonitorRequest `json:"monitors" validate:"required,dive"`

	// Prune deletes monitors in the project that this request no longer declares.
	// Absent means false — see configsync.Input.Prune for why that default is load-bearing.
	Prune bool `json:"prune"`
	// DryRun reports the plan and changes nothing.
	DryRun bool `json:"dry_run"`
}

// apply reconciles the declaration.
//
// Always 200 when the request itself was valid, even if individual monitors failed —
// their outcome is in the body. A 4xx would tell a workflow "nothing happened" when
// ninety-nine of a hundred monitors were in fact applied, and the retry it then
// performs is the duplicate-creating behaviour this endpoint exists to prevent.
func (h *SyncHandler) apply(w http.ResponseWriter, r *http.Request) {
	var req syncRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	p := mustPrincipal(r)
	in := configsync.Input{
		Project: req.Project,
		Prune:   req.Prune,
		DryRun:  req.DryRun,
	}
	for _, m := range req.Monitors {
		d := configsync.DesiredMonitor{
			Name:            m.Name,
			Type:            m.Type,
			Target:          m.Target,
			IntervalSeconds: m.IntervalSeconds,
			TimeoutSeconds:  m.TimeoutSeconds,
			GraceSeconds:    m.GraceSeconds,
			Enabled:         m.Enabled,
			Public:          m.Public,
		}
		if m.Settings != nil {
			d.Settings = *m.Settings
		}
		in.Monitors = append(in.Monitors, d)
	}

	res, err := h.svc.Apply(r.Context(), configsync.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, res)
}
