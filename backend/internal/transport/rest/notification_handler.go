package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// NotificationHandler exposes notification-channel CRUD plus a "send test"
// action. Channel secrets are never returned; responses expose only a
// has_secret flag.
type NotificationHandler struct {
	svc       *notification.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

// NewNotificationHandler builds a NotificationHandler.
func NewNotificationHandler(svc *notification.Service, v *validate.Validator, a *middleware.Authenticator) *NotificationHandler {
	return &NotificationHandler{svc: svc, validator: v, auth: a}
}

// Routes returns the authenticated channel routes.
func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.Require)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.With(h.auth.RequireWriter).Post("/", h.create)
	r.With(h.auth.RequireWriter).Patch("/{id}", h.update)
	r.With(h.auth.RequireWriter).Delete("/{id}", h.delete)
	r.With(h.auth.RequireWriter).Post("/{id}/test", h.test)
	return r
}

// ---- DTOs ----

type createChannelRequest struct {
	Name    string            `json:"name" validate:"required,min=1,max=200"`
	Type    string            `json:"type" validate:"required,oneof=telegram slack discord email webhook teams"`
	Enabled *bool             `json:"enabled"`
	Config  map[string]string `json:"config"`
	Secret  string            `json:"secret"`
}

type updateChannelRequest struct {
	Name    *string           `json:"name" validate:"omitempty,min=1,max=200"`
	Enabled *bool             `json:"enabled"`
	Config  map[string]string `json:"config"`
	Secret  *string           `json:"secret"`
}

type channelResponse struct {
	ID        string            `json:"id"`
	OrgID     string            `json:"org_id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Enabled   bool              `json:"enabled"`
	Config    map[string]string `json:"config"`
	HasSecret bool              `json:"has_secret"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func presentChannel(c *notification.Channel) channelResponse {
	cfg := c.Config
	if cfg == nil {
		cfg = map[string]string{}
	}
	return channelResponse{
		ID:        c.ID.String(),
		OrgID:     c.OrgID.String(),
		Name:      c.Name,
		Type:      string(c.Type),
		Enabled:   c.Enabled,
		Config:    cfg,
		HasSecret: c.HasSecret(),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func notificationActor(r *http.Request) notification.Actor {
	p := mustPrincipal(r)
	return notification.Actor{UserID: p.UserID, OrgID: p.OrgID, Role: p.Role}
}

// ---- handlers ----

func (h *NotificationHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), notificationActor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]channelResponse, 0, len(items))
	for i := range items {
		out = append(out, presentChannel(&items[i]))
	}
	httpx.OK(w, map[string]any{"data": out})
}

func (h *NotificationHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := h.svc.Get(r.Context(), notificationActor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentChannel(c))
}

func (h *NotificationHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := h.svc.Create(r.Context(), notificationActor(r), notification.CreateInput{
		Name:    req.Name,
		Type:    notification.ChannelType(req.Type),
		Config:  req.Config,
		Secret:  req.Secret,
		Enabled: req.Enabled,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, presentChannel(c))
}

func (h *NotificationHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateChannelRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	c, err := h.svc.Update(r.Context(), notificationActor(r), id, notification.UpdateInput{
		Name:    req.Name,
		Enabled: req.Enabled,
		Config:  req.Config,
		Secret:  req.Secret,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentChannel(c))
}

func (h *NotificationHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), notificationActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *NotificationHandler) test(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.SendTest(r.Context(), notificationActor(r), id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, map[string]any{"status": "sent"})
}
