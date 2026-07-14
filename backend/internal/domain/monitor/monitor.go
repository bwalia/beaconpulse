// Package monitor is the bounded context for monitored resources. It owns the
// Monitor aggregate, its typed per-type Settings, validation, and the use cases
// for managing monitors. Crucially, it does not know *how* probing happens: it
// depends on the Syncer interface to push desired state into the monitoring
// control plane (Prometheus/Blackbox), keeping the domain decoupled from
// infrastructure.
package monitor

import (
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
)

// Type selects the probe kind. Only the types listed here as "supported" are
// currently generatable by the control plane; the database permits additional
// types reserved for future modules (server, kubernetes, api, domain, ...).
type Type string

const (
	TypeHTTP  Type = "http"
	TypeHTTPS Type = "https"
	TypeSSL   Type = "ssl"
	TypeTCP   Type = "tcp"
	TypeICMP  Type = "icmp"
	TypeDNS   Type = "dns"
	// TypeHeartbeat is a PUSH monitor: Beacon does not probe it; the customer's
	// job pings a capability URL, and silence past interval+grace alerts. It has
	// no probe target and generates no Blackbox/scrape config.
	TypeHeartbeat Type = "heartbeat"
)

// HeartbeatTarget is the placeholder stored in a heartbeat's target column. The
// schema requires a non-empty target; a heartbeat has none, and nothing reads
// this value — it exists only to satisfy the NOT NULL / length CHECK.
const HeartbeatTarget = "(heartbeat)"

// Status is the last observed health of a monitor.
type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
	StatusUnknown  Status = "unknown"
	StatusPaused   Status = "paused"
)

// SupportedTypes are the monitor types the control plane can currently probe.
var SupportedTypes = map[Type]bool{
	TypeHTTP: true, TypeHTTPS: true, TypeSSL: true,
	TypeTCP: true, TypeICMP: true, TypeDNS: true,
	TypeHeartbeat: true,
}

// Sensitivity controls how long a monitor must stay down before its MonitorDown
// alert fires — the "for" duration of the generated alert rule. It trades
// detection speed against noise from transient blips.
const (
	SensitivityImmediate = "immediate" // fire on the first failed check
	SensitivityBalanced  = "balanced"  // default: a sustained failure (~2 checks)
	SensitivityRelaxed   = "relaxed"   // only prolonged outages (~5 min)
)

// ValidSensitivity reports whether s is a known sensitivity level.
func ValidSensitivity(s string) bool {
	switch s {
	case SensitivityImmediate, SensitivityBalanced, SensitivityRelaxed:
		return true
	default:
		return false
	}
}

// Settings holds the union of type-specific probe configuration. Only the
// fields relevant to a monitor's Type are meaningful; validation enforces this.
// Persisted as JSONB so new fields do not require migrations.
type Settings struct {
	// ---- HTTP / HTTPS / SSL ----
	Method           string            `json:"method,omitempty"`
	ValidStatusCodes []int             `json:"valid_status_codes,omitempty"`
	BodyKeyword      string            `json:"body_keyword,omitempty"`
	BodyNotKeyword   string            `json:"body_not_keyword,omitempty"`
	FollowRedirects  bool              `json:"follow_redirects,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	SkipTLSVerify    bool              `json:"skip_tls_verify,omitempty"`
	// SSLExpiryWarningDays sets the alert threshold for certificate expiry.
	SSLExpiryWarningDays int `json:"ssl_expiry_warning_days,omitempty"`
	// ResponseTimeWarningMS, when > 0, generates a slow-response alert rule.
	ResponseTimeWarningMS int `json:"response_time_warning_ms,omitempty"`

	// ---- Alerting ----
	// AlertSensitivity controls the MonitorDown alert's "for" duration
	// (immediate|balanced|relaxed). Defaults to balanced.
	AlertSensitivity string `json:"alert_sensitivity,omitempty"`

	// ---- DNS ----
	DNSQueryName   string   `json:"dns_query_name,omitempty"`
	DNSQueryType   string   `json:"dns_query_type,omitempty"`
	DNSExpectedIPs []string `json:"dns_expected_ips,omitempty"`
}

// Monitor is a single monitored resource.
type Monitor struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Type      Type
	Target    string
	Enabled   bool
	// PingToken is the heartbeat capability token (nil for probed monitors) — the
	// opaque credential embedded in the ping URL, not the monitor id.
	PingToken *string
	// LastPingAt is when the last heartbeat ping arrived (nil until first ping;
	// seeded to CreatedAt on a new heartbeat so silence-from-the-start still alerts).
	LastPingAt *time.Time
	// GraceSeconds is the slack beyond the interval before a missed ping alerts.
	// Zero for non-heartbeat monitors.
	GraceSeconds int
	// Public publishes this monitor onto the org's public status page.
	// Defaults to false: enabling a status page must never retroactively
	// expose an endpoint that was added before the page existed.
	Public          bool
	IntervalSeconds int
	TimeoutSeconds  int
	Settings        Settings
	LastStatus      Status
	LastCheckedAt   *time.Time
	CreatedBy       *uuid.UUID
	UpdatedBy       *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Actor is the authenticated caller performing a monitor operation.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}
