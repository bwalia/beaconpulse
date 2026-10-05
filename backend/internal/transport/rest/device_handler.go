package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/device"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest/middleware"
)

// DeviceHandler registers the push-notification tokens a user's devices enroll
// after signing in: mobile apps (APNs tokens) and browsers (web push subscriptions).
type DeviceHandler struct {
	svc       *device.Service
	validator *validate.Validator
	auth      *middleware.Authenticator
	// webPushKey is the VAPID public key browsers subscribe with; empty when
	// browser push isn't configured, which hides the option in the UI.
	webPushKey string
}

// WithWebPushKey enables browser push registration hints (GET /devices/webpush).
func (h *DeviceHandler) WithWebPushKey(key string) *DeviceHandler {
	h.webPushKey = key
	return h
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
	r.Get("/webpush", h.webPushConfig)
	return r
}

// webPushConfig tells the browser whether it can subscribe, and with which key.
// The key is public by design (it is the applicationServerKey).
func (h *DeviceHandler) webPushConfig(w http.ResponseWriter, _ *http.Request) {
	httpx.OK(w, map[string]any{"enabled": h.webPushKey != "", "public_key": h.webPushKey})
}

func deviceActor(r *http.Request) device.Actor {
	p := mustPrincipal(r)
	return device.Actor{UserID: p.UserID, OrgID: p.OrgID}
}

// registerDeviceRequest is an APNs token, or a browser subscription in the shape
// PushSubscription.toJSON() produces (token = its endpoint, plus keys).
type registerDeviceRequest struct {
	Token    string `json:"token" validate:"required,max=2048"`
	Platform string `json:"platform" validate:"omitempty,oneof=ios android web"`
	Keys     struct {
		P256dh string `json:"p256dh" validate:"omitempty,max=200"`
		Auth   string `json:"auth" validate:"omitempty,max=100"`
	} `json:"keys"`
}

type unregisterDeviceRequest struct {
	Token string `json:"token" validate:"required,max=2048"`
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
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
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
