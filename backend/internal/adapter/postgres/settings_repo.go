package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/plan"
	"beacon/internal/domain/settings"
	"beacon/internal/platform/apperror"
)

// SettingsRepository persists the single platform-settings row (id = 1).
type SettingsRepository struct {
	pool *pgxpool.Pool
}

// NewSettingsRepository builds a SettingsRepository.
func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

var _ settings.Repository = (*SettingsRepository)(nil)

// planJSON is the on-disk shape of one tier inside the `plans` JSONB column. New fields
// append cleanly — JSONB is schemaless, so no migration is needed when a tier grows a
// field; older rows simply read the zero value (empty tagline/features → built-in
// default applies).
type planJSON struct {
	Plan               string   `json:"plan"`
	PriceMonthly       int      `json:"price_monthly"`
	MaxMonitors        int      `json:"max_monitors"`
	MinIntervalSeconds int      `json:"min_interval_seconds"`
	MonthlyDiagnoses   int      `json:"monthly_diagnoses"`
	Tagline            string   `json:"tagline,omitempty"`
	Features           []string `json:"features,omitempty"`
}

// Load reads the settings row. ok=false when no row exists yet (fresh install).
func (r *SettingsRepository) Load(ctx context.Context) (settings.Settings, bool, error) {
	var (
		s        settings.Settings
		plansRaw []byte
	)
	err := r.pool.QueryRow(ctx,
		`SELECT monitor_hours_per_dollar, plans, premium_grants, updated_at
		   FROM platform_settings WHERE id = 1`).
		Scan(&s.MonitorHoursPerDollar, &plansRaw, &s.PremiumGrants, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return settings.Settings{}, false, nil
		}
		return settings.Settings{}, false, apperror.Internal(fmt.Errorf("load platform settings: %w", err))
	}
	var rows []planJSON
	if len(plansRaw) > 0 {
		if err := json.Unmarshal(plansRaw, &rows); err != nil {
			return settings.Settings{}, false, apperror.Internal(fmt.Errorf("decode plans: %w", err))
		}
	}
	for _, p := range rows {
		s.Plans = append(s.Plans, settings.PlanConfig{
			Plan:               plan.Plan(p.Plan),
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.MaxMonitors,
			MinIntervalSeconds: p.MinIntervalSeconds,
			MonthlyDiagnoses:   p.MonthlyDiagnoses,
			Tagline:            p.Tagline,
			Features:           p.Features,
		})
	}
	return s, true, nil
}

// Save upserts the singleton row. An UPSERT (not an UPDATE) so the first writer — the
// API or the worker, whichever starts first — seeds the row without a race.
func (r *SettingsRepository) Save(ctx context.Context, s settings.Settings, updatedBy uuid.UUID) error {
	rows := make([]planJSON, 0, len(s.Plans))
	for _, p := range s.Plans {
		rows = append(rows, planJSON{
			Plan:               string(p.Plan),
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.MaxMonitors,
			MinIntervalSeconds: p.MinIntervalSeconds,
			MonthlyDiagnoses:   p.MonthlyDiagnoses,
			Tagline:            p.Tagline,
			Features:           p.Features,
		})
	}
	plansRaw, err := json.Marshal(rows)
	if err != nil {
		return apperror.Internal(fmt.Errorf("encode plans: %w", err))
	}
	grants := s.PremiumGrants
	if grants == nil {
		grants = []string{}
	}
	var by *uuid.UUID
	if updatedBy != uuid.Nil {
		by = &updatedBy
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO platform_settings (id, monitor_hours_per_dollar, plans, premium_grants, updated_at, updated_by)
		 VALUES (1, $1, $2, $3, now(), $4)
		 ON CONFLICT (id) DO UPDATE SET
		     monitor_hours_per_dollar = EXCLUDED.monitor_hours_per_dollar,
		     plans                    = EXCLUDED.plans,
		     premium_grants           = EXCLUDED.premium_grants,
		     updated_at               = now(),
		     updated_by               = EXCLUDED.updated_by`,
		s.MonitorHoursPerDollar, plansRaw, grants, by)
	if err != nil {
		return apperror.Internal(fmt.Errorf("save platform settings: %w", err))
	}
	return nil
}
