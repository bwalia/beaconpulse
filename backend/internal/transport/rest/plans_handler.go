package rest

import (
	"net/http"

	"beacon/internal/domain/plan"
	"beacon/internal/platform/httpx"
)

// The PUBLIC pricing surface for the marketing site. It exposes exactly what the
// landing page prints — per-tier price/limits and the pay-as-you-go rate — read from
// the LIVE, operator-tuned plan config, so a price change made at /platform shows up
// on the public site without a code change or redeploy. No auth and no per-tenant
// data: it is the same information any visitor sees on the pricing page.

type publicPlanResponse struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Tagline            string   `json:"tagline"`
	PriceMonthly       int      `json:"price_monthly"`
	MaxMonitors        int      `json:"max_monitors"`
	MinIntervalSeconds int      `json:"min_interval_seconds"`
	MonthlyDiagnoses   int      `json:"monthly_diagnoses"`
	// Highlights are the operator-editable marketing bullets (defaults applied). The
	// numeric bullets (monitors/interval/AI) are derived by the landing page from the
	// numbers above, so they stay localised and can't drift from the caps.
	Highlights []string `json:"highlights"`
}

type publicPlansResponse struct {
	// MonitorHoursPerDollar is the pay-as-you-go rate: $1 of credit buys this many
	// monitor-hours (one monitor probed for one hour).
	MonitorHoursPerDollar int                  `json:"monitor_hours_per_dollar"`
	Plans                 []publicPlanResponse `json:"plans"`
}

// publicPlans serves the live catalog for the landing page. Cached briefly (the page
// also revalidates on its side) so an operator's price change propagates within a
// minute without a config read per view.
func publicPlans(w http.ResponseWriter, r *http.Request) {
	out := publicPlansResponse{MonitorHoursPerDollar: plan.HoursPerDollar()}
	for _, p := range plan.Catalog() {
		out.Plans = append(out.Plans, publicPlanResponse{
			ID:                 string(p.Plan),
			Name:               p.Name,
			Tagline:            p.Tagline,
			PriceMonthly:       p.PriceMonthly,
			MaxMonitors:        p.Limits.MaxMonitors,
			MinIntervalSeconds: p.Limits.MinIntervalSeconds,
			MonthlyDiagnoses:   p.Limits.MonthlyDiagnoses,
			Highlights:         p.Highlights,
		})
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httpx.OK(w, out)
}
