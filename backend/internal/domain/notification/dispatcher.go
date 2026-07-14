package notification

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/crypto"
	"beacon/internal/platform/logger"
)

// MaintenanceChecker reports whether alerts for a monitor are currently
// suppressed by an active maintenance window. It is the single suppression point
// for every alert source (probed monitors and heartbeats alike). Optional — a nil
// checker disables suppression and every alert dispatches.
type MaintenanceChecker interface {
	IsSuppressed(ctx context.Context, orgID, monitorID uuid.UUID, at time.Time) (bool, error)
}

// Dispatcher fans an Alertmanager webhook's alerts out to the notification
// channels of the owning organization. It is best-effort: a failed delivery to
// one channel is logged and audited but never blocks the others.
type Dispatcher struct {
	repo       Repository
	cipher     *crypto.Cipher
	registry   map[ChannelType]Notifier
	projects   ProjectLookup
	auditlog   audit.Recorder
	suppressor MaintenanceChecker // optional maintenance-window suppression; nil disables it
	dashURL    string
	analyzer   Analyzer      // optional AI enrichment; nil disables it
	aiTimeout  time.Duration // per-alert budget for the analyzer
	now        func() time.Time
}

// NewDispatcher wires the dispatcher. analyzer and suppressor may each be nil, in
// which case alerts are delivered without AI enrichment / without maintenance
// suppression respectively.
func NewDispatcher(repo Repository, cipher *crypto.Cipher, registry map[ChannelType]Notifier, projects ProjectLookup, auditlog audit.Recorder, suppressor MaintenanceChecker, dashboardURL string, analyzer Analyzer, aiTimeout time.Duration) *Dispatcher {
	if aiTimeout <= 0 {
		aiTimeout = 20 * time.Second
	}
	return &Dispatcher{
		repo:       repo,
		cipher:     cipher,
		registry:   registry,
		projects:   projects,
		auditlog:   auditlog,
		suppressor: suppressor,
		dashURL:    strings.TrimRight(dashboardURL, "/"),
		analyzer:   analyzer,
		aiTimeout:  aiTimeout,
		now:        time.Now,
	}
}

// DispatchAlerts delivers every event to the enabled channels of its org.
func (d *Dispatcher) DispatchAlerts(ctx context.Context, events []AlertEvent) {
	log := logger.FromContext(ctx)
	// Cache channels per org so we hit the DB once per org, not once per alert.
	channelsByOrg := map[uuid.UUID][]Channel{}

	for _, ev := range events {
		if ev.OrgID == uuid.Nil {
			continue
		}
		// Maintenance suppression: if an active window covers this monitor, the
		// alert is planned — record it (never silent) and skip delivery. Checked
		// against "now" so a window ending re-arms alerting immediately, and
		// fail-open so an infra error here can never silence a real alert.
		if d.suppressor != nil {
			if mid, err := uuid.Parse(ev.MonitorID); err == nil {
				suppressed, err := d.suppressor.IsSuppressed(ctx, ev.OrgID, mid, d.now().UTC())
				switch {
				case err != nil:
					log.Error("dispatch: maintenance check failed; delivering anyway",
						slog.String("org_id", ev.OrgID.String()),
						slog.String("monitor_id", ev.MonitorID),
						slog.String("error", err.Error()))
				case suppressed:
					log.Info("dispatch: alert suppressed by maintenance window",
						slog.String("org_id", ev.OrgID.String()),
						slog.String("monitor_id", ev.MonitorID),
						slog.String("alert", ev.AlertName))
					d.recordSuppressed(ctx, ev)
					continue
				}
			}
		}
		channels, ok := channelsByOrg[ev.OrgID]
		if !ok {
			var err error
			channels, err = d.repo.ListEnabledByOrg(ctx, ev.OrgID)
			if err != nil {
				log.Error("dispatch: list channels failed",
					slog.String("org_id", ev.OrgID.String()), slog.String("error", err.Error()))
				channels = nil
			}
			channelsByOrg[ev.OrgID] = channels
		}
		if len(channels) == 0 {
			continue
		}

		msg := d.render(ctx, ev)
		// Ask the AI to triage a firing alert before we deliver it. Recoveries
		// (resolved) need no analysis — they are good news. Enrichment is
		// best-effort and never blocks or fails delivery.
		if ev.Status == StatusFiring {
			msg.Analysis = d.enrich(ctx, ev)
		}
		for i := range channels {
			d.deliver(ctx, &channels[i], msg)
		}
	}
}

// enrich runs the analyzer against a firing event within a bounded context. It
// returns nil (and logs) on any error, timeout, or when no analyzer is wired, so
// the caller can always proceed to deliver.
func (d *Dispatcher) enrich(ctx context.Context, ev AlertEvent) *AlertAnalysis {
	if d.analyzer == nil {
		return nil
	}
	log := logger.FromContext(ctx)
	aiCtx, cancel := context.WithTimeout(ctx, d.aiTimeout)
	defer cancel()

	analysis, err := d.analyzer.Analyze(aiCtx, ev)
	if err != nil {
		log.Warn("dispatch: AI enrichment failed; delivering without analysis",
			slog.String("monitor", ev.MonitorName), slog.String("error", err.Error()))
		return nil
	}
	return analysis
}

func (d *Dispatcher) deliver(ctx context.Context, ch *Channel, msg Message) {
	log := logger.FromContext(ctx)
	notifier, ok := d.registry[ch.Type]
	if !ok {
		return // no notifier for this type yet
	}
	dec, err := decryptChannel(d.cipher, ch)
	if err != nil {
		log.Error("dispatch: decrypt channel failed", slog.String("channel", ch.ID.String()))
		return
	}
	if err := notifier.Send(ctx, dec, msg); err != nil {
		log.Warn("dispatch: delivery failed",
			slog.String("channel", ch.Name), slog.String("type", string(ch.Type)), slog.String("error", err.Error()))
		d.record(ctx, ch, "notification.failed", map[string]any{"error": err.Error(), "monitor": msg.MonitorName})
		return
	}
	d.record(ctx, ch, "notification.sent", map[string]any{"monitor": msg.MonitorName, "status": string(msg.Status)})
}

func (d *Dispatcher) render(ctx context.Context, ev AlertEvent) Message {
	project, environment := "", ""
	if d.projects != nil && ev.ProjectID != uuid.Nil {
		if name, env, ok := d.projects.Project(ctx, ev.OrgID, ev.ProjectID); ok {
			project, environment = name, env
		}
	}
	title := ev.Summary
	if title == "" {
		title = ev.AlertName
	}
	ts := ev.StartsAt
	if ev.Status == StatusResolved && !ev.EndsAt.IsZero() {
		ts = ev.EndsAt
	}
	return Message{
		Status:       ev.Status,
		Severity:     ev.Severity,
		Title:        title,
		MonitorName:  ev.MonitorName,
		MonitorType:  ev.MonitorType,
		Target:       ev.Target,
		Project:      project,
		Environment:  environment,
		Description:  ev.Description,
		Timestamp:    ts,
		Duration:     ev.Duration(d.now().UTC()),
		DashboardURL: d.dashURL,
	}
}

func (d *Dispatcher) record(ctx context.Context, ch *Channel, action audit.Action, md map[string]any) {
	org := ch.OrgID
	_ = d.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, Action: action,
		ResourceType: "notification_channel", ResourceID: ch.ID.String(), Metadata: md,
	})
}

// recordSuppressed writes the audit trail for an alert withheld by a maintenance
// window, keyed on the monitor. Best-effort, like every other audit call here.
func (d *Dispatcher) recordSuppressed(ctx context.Context, ev AlertEvent) {
	org := ev.OrgID
	_ = d.auditlog.Record(ctx, audit.Entry{
		OrgID:        &org,
		Action:       audit.ActionAlertSuppressed,
		ResourceType: "monitor",
		ResourceID:   ev.MonitorID,
		Metadata: map[string]any{
			"alert":    ev.AlertName,
			"status":   string(ev.Status),
			"severity": ev.Severity,
			"monitor":  ev.MonitorName,
		},
	})
}

// decryptChannel decrypts a channel's secret into a Decrypted value for sending.
func decryptChannel(cipher *crypto.Cipher, ch *Channel) (Decrypted, error) {
	dec := Decrypted{Type: ch.Type, Name: ch.Name, Config: ch.Config}
	if ch.SecretEncrypted != "" {
		secret, err := cipher.DecryptString(ch.SecretEncrypted)
		if err != nil {
			return Decrypted{}, err
		}
		dec.Secret = secret
	}
	return dec, nil
}
