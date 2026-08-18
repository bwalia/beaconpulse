// Package settings manages platform-wide, operator-tunable configuration: plan
// pricing and limits, the pay-as-you-go rate, and the premium email/domain allowlist.
//
// These are GLOBAL (they affect every tenant), so they are guarded by a platform-admin
// allowlist seeded from the environment — the trust root that cannot itself be edited
// in-app. Everything else is stored in the database and edited live from the admin
// page; on change the service reloads it into the plan package's live snapshot, which
// is what enforcement and the pricing UI actually read.
package settings

import (
	"context"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/emailmatch"
)

// PlanConfig is one tier's editable pricing and limits.
type PlanConfig struct {
	Plan               plan.Plan
	PriceMonthly       int
	MaxMonitors        int
	MinIntervalSeconds int
	MonthlyDiagnoses   int
}

// Settings is the full editable platform configuration.
type Settings struct {
	MonitorHoursPerDollar int
	Plans                 []PlanConfig
	// PremiumGrants are emails and/or domains that get Pro free (bypassing billing).
	PremiumGrants []string
	UpdatedAt     time.Time
}

// editableTiers are the tiers exposed in the admin page, in display order. PayAsYouGo
// is intentionally omitted: it is not a subscribable tier and its limits track Pro.
var editableTiers = []plan.Plan{plan.Free, plan.Starter, plan.Pro}

// Repository persists the single platform-settings row.
type Repository interface {
	// Load returns the stored settings. ok=false means none has been saved yet
	// (fresh install), so the caller should seed from defaults.
	Load(ctx context.Context) (s Settings, ok bool, err error)
	Save(ctx context.Context, s Settings, updatedBy uuid.UUID) error
}

// Service implements the platform-settings use cases.
type Service struct {
	repo     Repository
	auditlog audit.Recorder
	admins   []string // platform-admin emails/domains (env-seeded trust root)
}

// NewService wires the settings service. admins is the platform-operator allowlist.
func NewService(repo Repository, auditlog audit.Recorder, admins []string) *Service {
	return &Service{repo: repo, auditlog: auditlog, admins: emailmatch.Normalize(admins)}
}

// IsPlatformAdmin reports whether email may view and change platform settings.
func (s *Service) IsPlatformAdmin(email string) bool { return emailmatch.Match(s.admins, email) }

// Get returns the current settings as stored, falling back to what is live (defaults)
// if nothing has been saved yet.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	st, ok, err := s.repo.Load(ctx)
	if err != nil {
		return Settings{}, err
	}
	if !ok {
		return fromLive(), nil
	}
	return st, nil
}

// Reload reads the stored settings and installs them into the plan package's live
// snapshot, seeding the store from whatever is currently live on a fresh install.
// Called at startup (both binaries) and after every Update; the worker also calls it
// periodically so live edits made through the API reach the control-plane cap.
func (s *Service) Reload(ctx context.Context) error {
	st, ok, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	if !ok {
		// Fresh install: persist whatever is live now (built-in defaults, already
		// overlaid with any env baseline the binary applied at startup) so the admin
		// page opens on real values rather than blanks.
		st = fromLive()
		if err := s.repo.Save(ctx, st, uuid.Nil); err != nil {
			return err
		}
	}
	plan.Apply(toPlanConfig(st))
	return nil
}

// Update validates and persists new settings, then reloads them live. Restricted to
// platform admins — the actor email is checked here, not just at the edge, so the rule
// holds no matter which caller reaches it.
func (s *Service) Update(ctx context.Context, actorID uuid.UUID, actorEmail string, in Settings) (Settings, error) {
	if !s.IsPlatformAdmin(actorEmail) {
		return Settings{}, apperror.Forbidden("only platform operators can change pricing and plans")
	}
	if err := validate(in); err != nil {
		return Settings{}, err
	}
	if err := s.repo.Save(ctx, normalize(in), actorID); err != nil {
		return Settings{}, err
	}
	if err := s.Reload(ctx); err != nil {
		return Settings{}, err
	}
	uid := actorID
	_ = s.auditlog.Record(ctx, audit.Entry{
		UserID:       &uid,
		Action:       "platform.settings_updated",
		ResourceType: "platform",
		ResourceID:   "settings",
		Metadata: map[string]any{
			"monitor_hours_per_dollar": in.MonitorHoursPerDollar,
			"premium_grants":           len(normalize(in).PremiumGrants),
		},
	})
	return s.Get(ctx)
}

// fromLive snapshots the plan package's live config into Settings.
func fromLive() Settings {
	c := plan.Snapshot()
	out := Settings{MonitorHoursPerDollar: c.HoursPerDollar, PremiumGrants: c.Grants}
	for _, p := range editableTiers {
		l := c.Limits[p]
		out.Plans = append(out.Plans, PlanConfig{
			Plan:               p,
			PriceMonthly:       c.Prices[p],
			MaxMonitors:        l.MaxMonitors,
			MinIntervalSeconds: l.MinIntervalSeconds,
			MonthlyDiagnoses:   l.MonthlyDiagnoses,
		})
	}
	return out
}

// toPlanConfig turns stored Settings into a plan.Config to Apply. Missing tiers are
// filled from defaults by plan.Apply, so a partial row never zeroes a cap.
func toPlanConfig(s Settings) plan.Config {
	c := plan.DefaultConfig()
	c.HoursPerDollar = s.MonitorHoursPerDollar
	c.Grants = s.PremiumGrants
	for _, p := range s.Plans {
		c.Prices[p.Plan] = p.PriceMonthly
		c.Limits[p.Plan] = plan.Limits{
			MaxMonitors:        p.MaxMonitors,
			MinIntervalSeconds: p.MinIntervalSeconds,
			MonthlyDiagnoses:   p.MonthlyDiagnoses,
		}
	}
	return c
}

func normalize(s Settings) Settings {
	s.PremiumGrants = emailmatch.Normalize(s.PremiumGrants)
	return s
}

// validate bounds every editable value. Ceilings are generous but finite: they exist
// so a typo can't set a zero interval (which would hammer the probers) or a negative
// price, not to constrain a real operator.
func validate(s Settings) error {
	if s.MonitorHoursPerDollar < 1 || s.MonitorHoursPerDollar > 1_000_000 {
		return apperror.Validation("monitor hours per dollar must be between 1 and 1,000,000",
			apperror.FieldError{Field: "monitor_hours_per_dollar", Message: "out of range"})
	}
	for _, p := range s.Plans {
		if !p.Plan.Subscribable() {
			return apperror.Validation("unknown plan",
				apperror.FieldError{Field: "plans", Message: "unknown plan " + string(p.Plan)})
		}
		if p.MaxMonitors < 1 || p.MaxMonitors > 1_000_000 {
			return apperror.Validation("max monitors out of range",
				apperror.FieldError{Field: "max_monitors", Message: "must be between 1 and 1,000,000"})
		}
		if p.MinIntervalSeconds < 5 || p.MinIntervalSeconds > 86_400 {
			return apperror.Validation("minimum interval out of range",
				apperror.FieldError{Field: "min_interval_seconds", Message: "must be between 5 and 86,400"})
		}
		if p.MonthlyDiagnoses < 0 || p.MonthlyDiagnoses > 1_000_000 {
			return apperror.Validation("monthly diagnoses out of range",
				apperror.FieldError{Field: "monthly_diagnoses", Message: "must be between 0 and 1,000,000"})
		}
		if p.PriceMonthly < 0 || p.PriceMonthly > 1_000_000 {
			return apperror.Validation("price out of range",
				apperror.FieldError{Field: "price_monthly", Message: "must be between 0 and 1,000,000"})
		}
	}
	return nil
}
