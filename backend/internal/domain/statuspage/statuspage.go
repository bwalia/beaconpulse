// Package statuspage is the bounded context for the PUBLIC status page — the one
// surface Beacon serves to callers with no credentials at all.
//
// That single fact drives every decision here:
//
//   - The types in this package are a deliberately narrow projection. A Monitor
//     carries a name and a status and nothing else. There is no Target, no
//     Settings, no interval, no IDs of other tenants' objects. The projection is
//     the security boundary, so it is expressed as its own type rather than by
//     remembering to omit fields from monitor.Monitor at the JSON layer — a
//     future field added to the domain model cannot silently appear in public.
//
//   - Exposure is opt-in twice: the org must publish a page, and each monitor
//     must be individually published onto it (see migration 0004).
//
//   - Reads are DB-only. The authenticated dashboard computes uptime from
//     Prometheus, but doing that here would let anyone with the URL drive
//     unbounded PromQL. A status page needs "is it up right now", which the
//     monitors table already knows.
package statuspage

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
)

// Overall summarises a whole page in one word, so the page can lead with an
// answer instead of making a reader scan every row.
type Overall string

const (
	// OverallOperational — every published monitor is up.
	OverallOperational Overall = "operational"
	// OverallDegraded — some are degraded, or a subset is down.
	OverallDegraded Overall = "degraded"
	// OverallOutage — every published monitor is down.
	OverallOutage Overall = "outage"
	// OverallMaintenance — planned work is in progress and nothing is genuinely
	// down. A neutral state, distinct from an outage, so routine deploys never read
	// as a failure to the customer's own users.
	OverallMaintenance Overall = "under_maintenance"
	// OverallUnknown — nothing has reported yet.
	OverallUnknown Overall = "unknown"
)

// Monitor is the PUBLIC view of a monitored resource.
//
// Add nothing to this struct without asking: "am I willing to show this to an
// anonymous stranger?". Target, IP, interval, and check config all fail that test.
type Monitor struct {
	Name   string
	Status monitor.Status
	// InMaintenance is true when an active maintenance window covers this monitor.
	// The true probe Status is still carried (and shown), so a real failure that
	// coincides with planned work is not hidden — it is just relabelled in the
	// headline so planned work does not read as an outage.
	InMaintenance bool
	LastCheckedAt *time.Time
}

// Group is a project, rendered as a section of the page.
type Group struct {
	Name        string
	Environment string
	Monitors    []Monitor
}

// MaintenanceNotice is a currently-active planned-maintenance window covering at
// least one monitor on this page. Only the human-facing title and times are
// exposed — never scope ids or internal identifiers.
type MaintenanceNotice struct {
	Title    string
	StartsAt time.Time
	EndsAt   time.Time
}

// Page is a whole rendered status page.
type Page struct {
	OrgName string
	Title   string
	Overall Overall
	Groups  []Group
	// Maintenances are the active planned-maintenance windows to surface as a
	// banner. Empty when no window is active.
	Maintenances []MaintenanceNotice
	UpdatedAt    time.Time
}

// Repository loads the published projection for one org slug.
type Repository interface {
	// GetBySlug returns the page for a published org, or nil when the slug does
	// not exist OR the org has not enabled its status page. Both cases collapse
	// to the same nil so a caller cannot use this endpoint to discover which
	// organizations exist.
	GetBySlug(ctx context.Context, slug string) (*Page, error)
}

// Service exposes the public read.
type Service struct {
	repo Repository
}

// NewService builds a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Get returns the page for slug, or nil if there is nothing published there.
func (s *Service) Get(ctx context.Context, slug string) (*Page, error) {
	p, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	p.Overall = Summarise(p.Groups)
	if p.Title == "" {
		p.Title = p.OrgName
	}
	return p, nil
}

// Summarise reduces every published monitor to a single headline status.
//
// The rules are deliberately conservative — a status page that under-reports an
// outage is worse than useless:
//
//   - any monitor down, and not all of them  -> degraded
//   - every live monitor down                -> outage
//   - any monitor degraded                   -> degraded
//   - otherwise-healthy with planned work    -> under_maintenance
//   - nothing has ever reported              -> unknown
//
// Maintenance overrides down in the HEADLINE: a monitor covered by an active
// window is excluded from the up/down/degraded tally, so planned work can never
// push the page to "outage". Its true probe state is still shown on its own row —
// this only changes the one-word summary, it does not hide a coincident failure.
//
// A monitor whose status is still "unknown" (never checked) is not counted as up:
// silence is not evidence of health.
func Summarise(groups []Group) Overall {
	var total, up, down, degraded, maint int
	for _, g := range groups {
		for _, m := range g.Monitors {
			total++
			if m.InMaintenance {
				maint++
				continue
			}
			switch m.Status {
			case monitor.StatusUp:
				up++
			case monitor.StatusDown:
				down++
			case monitor.StatusDegraded:
				degraded++
			}
		}
	}
	active := total - maint // monitors not shielded by a window
	switch {
	case total == 0:
		return OverallUnknown
	case active > 0 && down == active:
		return OverallOutage
	case down > 0 || degraded > 0:
		return OverallDegraded
	case maint > 0:
		// Nothing genuinely down or degraded, but planned work is in progress.
		return OverallMaintenance
	case up == active:
		return OverallOperational
	default:
		// Some monitors exist but have never reported.
		return OverallUnknown
	}
}

// ---- Authenticated settings side ----
//
// Everything above serves anonymous readers. The types below are the owner's
// controls: whether the org publishes a page at all, and what it is called.
// Deliberately in the same package (they describe one feature) but behind a
// separate repository interface, so the public read can never accidentally be
// handed something that can write.

// Settings is an org's status-page configuration.
type Settings struct {
	// Slug is read-only here: it is the org's identity, reused as the page URL
	// rather than minted separately so the two cannot drift.
	Slug    string
	OrgName string
	Enabled bool
	Title   string
	// PublishedCount is how many monitors are currently on the page. Surfaced so
	// the UI can warn about the "enabled but empty" trap.
	PublishedCount int
}

// SettingsRepository reads and writes an org's status-page settings.
type SettingsRepository interface {
	Get(ctx context.Context, orgID uuid.UUID) (*Settings, error)
	Update(ctx context.Context, orgID uuid.UUID, s Settings) error
	PublishedCount(ctx context.Context, orgID uuid.UUID) (int, error)
}

// SettingsService implements the owner-facing use cases.
type SettingsService struct {
	repo     SettingsRepository
	auditlog audit.Recorder
}

// NewSettingsService wires the settings service.
func NewSettingsService(repo SettingsRepository, auditlog audit.Recorder) *SettingsService {
	return &SettingsService{repo: repo, auditlog: auditlog}
}

// Get returns the caller org's settings, including the published-monitor count.
func (s *SettingsService) Get(ctx context.Context, orgID uuid.UUID) (*Settings, error) {
	set, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}
	n, err := s.repo.PublishedCount(ctx, orgID)
	if err != nil {
		return nil, err
	}
	set.PublishedCount = n
	return set, nil
}

// UpdateInput is a partial update of the status-page settings.
type UpdateInput struct {
	Enabled *bool
	Title   *string
}

// Update changes the org's status-page settings.
//
// Requires a writer: turning the page on changes what the entire internet can
// see about this org, which is not a read-only act.
func (s *SettingsService) Update(ctx context.Context, actor auth.Role, orgID, userID uuid.UUID, in UpdateInput) (*Settings, error) {
	if !actor.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit changing the status page")
	}

	cur, err := s.repo.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	changed := map[string]any{}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
		changed["status_page_enabled"] = cur.Enabled
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if len(title) > 120 {
			return nil, apperror.Validation("title is too long",
				apperror.FieldError{Field: "title", Message: "must be 120 characters or fewer"})
		}
		cur.Title = title
		changed["status_page_title"] = title
	}

	if err := s.repo.Update(ctx, orgID, *cur); err != nil {
		return nil, err
	}

	// Publishing is security-relevant, so it is audited like a permission change.
	if len(changed) > 0 && s.auditlog != nil {
		org := orgID
		uid := userID
		_ = s.auditlog.Record(ctx, audit.Entry{
			OrgID:        &org,
			UserID:       &uid,
			Action:       audit.ActionStatusPageUpdated,
			ResourceType: "status_page",
			ResourceID:   org.String(),
			Metadata:     changed,
		})
	}

	return s.Get(ctx, orgID)
}
