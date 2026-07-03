// Package metrics owns the application's own Prometheus instrumentation (the
// API monitoring itself). It exposes a dedicated registry — separate from the
// monitored-target metrics Prometheus scrapes from Blackbox — served at /metrics
// on the API. Beacon thus practices the observability it provides.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registry and the application-level collectors.
type Metrics struct {
	registry    *prometheus.Registry
	reqTotal    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
	inflight    prometheus.Gauge
}

// New builds a Metrics with process/go collectors and HTTP instrumentation
// registered.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &Metrics{
		registry: reg,
		reqTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "beacon_http_requests_total",
			Help: "Total HTTP requests processed, by method, route and status.",
		}, []string{"method", "route", "status"}),
		reqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "beacon_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, by method and route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "beacon_http_inflight_requests",
			Help: "Number of HTTP requests currently being served.",
		}),
	}
	reg.MustRegister(m.reqTotal, m.reqDuration, m.inflight)
	return m
}

// Handler returns the /metrics HTTP handler bound to this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// IncInflight / DecInflight track in-progress requests.
func (m *Metrics) IncInflight() { m.inflight.Inc() }
func (m *Metrics) DecInflight() { m.inflight.Dec() }

// ObserveRequest records a completed HTTP request.
func (m *Metrics) ObserveRequest(method, route, status string, d time.Duration) {
	m.reqTotal.WithLabelValues(method, route, status).Inc()
	m.reqDuration.WithLabelValues(method, route).Observe(d.Seconds())
}
