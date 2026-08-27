package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/device"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// DeviceHandler registers the push-notification tokens a user's mobile devices
// enroll after signing in.
type DeviceHandler struct {
	svc       *device.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
}

func NewDeviceHandler(svc *device.Service, v *validate.Validator, a *middleware.Authenticator) *DeviceHandler {
	return &DeviceHandler{svc: svc, validator: v, auth: a}
}

// Routes mounts device registration.
//
// RequireSession, not Require: a device belongs to a signed-in person and its
// token is stored against their user id. An API key acts for the org with no real
// user behind it, so letting it register a device would attach a token to a
// non-existent user — registration is deliberately human-only.
func (h *DeviceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.auth.RequireSession)
	r.Post("/", h.register)
	r.Delete("/", h.unregister)
	return r
}

func deviceActor(r *http.Request) device.Actor {
	p := mustPrincipal(r)
	return device.Actor{UserID: p.UserID, OrgID: p.OrgID}
}

type registerDeviceRequest struct {
	Token    string `json:"token" validate:"required,max=512"`
	Platform string `json:"platform" validate:"omitempty,oneof=ios android"`
}

type unregisterDeviceRequest struct {
	Token string `json:"token" validate:"required,max=512"`
}

func (h *DeviceHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	d, err := h.svc.Register(r.Context(), deviceActor(r), device.RegisterInput{
		Token:    req.Token,
		Platform: device.Platform(req.Platform),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Created(w, map[string]any{"data": d})
}

func (h *DeviceHandler) unregister(w http.ResponseWriter, r *http.Request) {
	var req unregisterDeviceRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Unregister(r.Context(), deviceActor(r), req.Token); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}
