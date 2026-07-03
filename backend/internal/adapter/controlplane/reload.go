package controlplane

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Reloader triggers hot-reloads of Prometheus and Blackbox via their lifecycle
// endpoints (both must be started with --web.enable-lifecycle).
type Reloader struct {
	client           *http.Client
	prometheusReload string
	blackboxReload   string
}

// NewReloader builds a Reloader with a bounded HTTP timeout.
func NewReloader(prometheusReloadURL, blackboxReloadURL string) *Reloader {
	return &Reloader{
		client:           &http.Client{Timeout: 10 * time.Second},
		prometheusReload: prometheusReloadURL,
		blackboxReload:   blackboxReloadURL,
	}
}

// ReloadPrometheus posts to Prometheus's reload endpoint.
func (r *Reloader) ReloadPrometheus(ctx context.Context) error {
	return r.post(ctx, "prometheus", r.prometheusReload)
}

// ReloadBlackbox posts to Blackbox's reload endpoint.
func (r *Reloader) ReloadBlackbox(ctx context.Context) error {
	return r.post(ctx, "blackbox", r.blackboxReload)
}

func (r *Reloader) post(ctx context.Context, name, url string) error {
	if url == "" {
		return nil // reload disabled for this component
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("controlplane: build %s reload request: %w", name, err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("controlplane: %s reload request: %w", name, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("controlplane: %s reload returned %d", name, resp.StatusCode)
	}
	return nil
}
