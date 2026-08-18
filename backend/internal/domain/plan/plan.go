// Package plan defines subscription plans and their resource limits. Limits are
// enforced when tenants create or update monitors so that one organization
// cannot overload the shared monitoring engines (Prometheus/Blackbox).
//
// Values are OPERATOR-TUNABLE at runtime: a platform operator edits pricing, limits,
// the pay-as-you-go rate and the premium allowlist from the admin settings page, and
// those land here via Apply. The built-in defaults below are the fallback (and the
// seed for a fresh install). The active configuration is held in one atomically
// swapped snapshot — lock-free to read on the hot enforcement paths, single-writer on
// change — rather than scattered globals; the settings service is that single writer.
package plan

import (
	"fmt"
	"sync/atomic"
	"time"

	"beacon/internal/platform/emailmatch"
)

// Plan identifies a subscription tier.
type Plan string

const (
	Free    Plan = "free"
	Starter Plan = "starter"
	Pro     Plan = "pro"
	// PayAsYouGo is not a subscribable tier and is never stored on the org row;
	// it is the effective tier while a pay-as-you-go credit balance remains. Its
	// limits are generous (cost is self-limiting: more monitors burn credit faster).
	PayAsYouGo Plan = "payg"
)

// MonthStart is when a monthly allowance last reset: the 1st, UTC.
//
// It lives here, in the package that defines the allowance, because two callers need
// it — the service that spends the quota and the page that reports what is left — and
// a quota whose reset is defined twice is a quota that eventually disagrees with the
// number shown to the customer.
//
// A calendar month rather than a rolling window: a reset you cannot predict is one you
// have to ration against. UTC so the answer does not depend on where the reader is.
func MonthStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Limits are the per-organization resource caps a plan grants.
type Limits struct {
	// MaxMonitors caps the number of non-deleted monitors an org may have.
	MaxMonitors int
	// MinIntervalSeconds is the fastest check interval the org may configure.
	MinIntervalSeconds int
	// MonthlyDiagnoses caps AI diagnoses per calendar month for SUBSCRIBED tiers,
	// which pay a flat fee and so need a ceiling on a per-use cost.
	//
	// Zero for Free (which cannot diagnose at all) and for pay-as-you-go, which is
	// metered per run against its credit instead — that org has already paid for
	// each diagnosis, and capping it too would be charging twice.
	MonthlyDiagnoses int
}

// defaultLimits is the built-in per-tier caps: the fallback when nothing is
// configured, and the seed a fresh install writes to the DB on first start.
//
// Diagnosis quotas track each tier's monitor cap rather than its price: the number of
// times you need to ask why something broke follows how much you are watching, not
// what you paid. Generous enough that a normal team never meets the ceiling, low
// enough that one subscriber cannot consume unbounded GPU for a flat fee.
var defaultLimits = map[Plan]Limits{
	Free:       {MaxMonitors: 10, MinIntervalSeconds: 60, MonthlyDiagnoses: 0},
	Starter:    {MaxMonitors: 50, MinIntervalSeconds: 30, MonthlyDiagnoses: 100},
	Pro:        {MaxMonitors: 500, MinIntervalSeconds: 10, MonthlyDiagnoses: 1000},
	PayAsYouGo: {MaxMonitors: 500, MinIntervalSeconds: 30, MonthlyDiagnoses: 0},
}

// defaultPrices is the built-in monthly USD price of each subscribable tier.
var defaultPrices = map[Plan]int{Free: 0, Starter: 19, Pro: 79}

// featureTail is the non-numeric marketing copy for each tier. The monitor-count and
// interval bullets are generated from the live limits (so they never disagree with the
// caps actually applied); these are appended after them.
var featureTail = map[Plan][]string{
	Free:    {"Telegram alerts", "Community support"},
	Starter: {"All alert channels", "Email support"},
	Pro:     {"Priority alerting", "Priority support"},
}

// Config is the live, operator-tunable plan configuration: per-tier limits and price,
// the pay-as-you-go rate ($1 buys this many monitor-hours), and the premium
// email/domain allowlist (matching accounts get Pro free).
type Config struct {
	Limits         map[Plan]Limits
	Prices         map[Plan]int
	HoursPerDollar int
	Grants         []string
}

// live holds the active configuration. Read lock-free via atomic.Load on every
// enforcement path; replaced wholesale by Apply. Never mutate a loaded Config.
var live atomic.Pointer[Config]

// admins is the platform-operator allowlist (env-seeded), held separately from the
// DB-driven Config so a settings reload never clobbers it. Platform operators get Pro
// for their own org automatically — it is theirs to run — without having to add
// themselves to the premium grant list.
var admins atomic.Pointer[[]string]

func init() {
	c := DefaultConfig()
	live.Store(&c)
	empty := []string{}
	admins.Store(&empty)
}

// DefaultConfig returns a fresh copy of the built-in configuration.
func DefaultConfig() Config {
	lim := make(map[Plan]Limits, len(defaultLimits))
	for k, v := range defaultLimits {
		lim[k] = v
	}
	prices := make(map[Plan]int, len(defaultPrices))
	for k, v := range defaultPrices {
		prices[k] = v
	}
	return Config{Limits: lim, Prices: prices, HoursPerDollar: 5}
}

// Apply installs c as the live configuration, filling any missing tier/price from the
// defaults (so a partial config never zeroes a cap) and normalizing the grant list.
// Single-writer: called at startup and whenever platform settings change.
func Apply(c Config) {
	d := DefaultConfig()
	if c.Limits == nil {
		c.Limits = map[Plan]Limits{}
	}
	for p, l := range d.Limits {
		if _, ok := c.Limits[p]; !ok {
			c.Limits[p] = l
		}
	}
	if c.Prices == nil {
		c.Prices = map[Plan]int{}
	}
	for p, pr := range d.Prices {
		if _, ok := c.Prices[p]; !ok {
			c.Prices[p] = pr
		}
	}
	if c.HoursPerDollar <= 0 {
		c.HoursPerDollar = d.HoursPerDollar
	}
	c.Grants = emailmatch.Normalize(c.Grants)
	live.Store(&c)
}

// Snapshot returns a deep copy of the live configuration, safe for the caller to read
// or mutate. Used by the settings service to present what is currently applied.
func Snapshot() Config {
	c := live.Load()
	lim := make(map[Plan]Limits, len(c.Limits))
	for k, v := range c.Limits {
		lim[k] = v
	}
	prices := make(map[Plan]int, len(c.Prices))
	for k, v := range c.Prices {
		prices[k] = v
	}
	return Config{
		Limits:         lim,
		Prices:         prices,
		HoursPerDollar: c.HoursPerDollar,
		Grants:         append([]string(nil), c.Grants...),
	}
}

// HoursPerDollar is the live pay-as-you-go rate: $1 buys this many monitor-hours.
func HoursPerDollar() int { return live.Load().HoursPerDollar }

// PriceOf returns the live monthly USD price of a tier.
func PriceOf(p Plan) int { return live.Load().Prices[p] }

// IsGranted reports whether email is on the premium allowlist (exact address or
// domain/subdomain). Such accounts get Pro free, bypassing billing.
func IsGranted(email string) bool { return emailmatch.Match(live.Load().Grants, email) }

// SetAdmins installs the platform-operator allowlist (env-seeded). Called once at
// startup by each binary; independent of the DB-driven config, so a settings reload
// leaves it untouched.
func SetAdmins(list []string) {
	n := emailmatch.Normalize(list)
	admins.Store(&n)
}

// IsAdmin reports whether email is a platform operator. Operators get Pro for their
// own org automatically (see Resolve), on top of being able to edit platform settings.
func IsAdmin(email string) bool {
	p := admins.Load()
	if p == nil {
		return false
	}
	return emailmatch.Match(*p, email)
}

// Valid reports whether p is a known plan.
func (p Plan) Valid() bool {
	_, ok := defaultLimits[p]
	return ok
}

// Subscribable reports whether p is a tier a customer can subscribe to (Free is
// the implicit default; PayAsYouGo is credit-based, not a subscription).
func (p Plan) Subscribable() bool {
	return p == Free || p == Starter || p == Pro
}

// Effective resolves the plan whose limits actually apply right now: the
// subscribed tier while its Stripe subscription is active, otherwise pay-as-you-go
// while credit remains, otherwise Free. Computed (never stored) so it can't go stale.
func Effective(subscribed Plan, subscriptionActive bool, creditSeconds int64) Plan {
	if subscriptionActive && (subscribed == Starter || subscribed == Pro) {
		return subscribed
	}
	if creditSeconds > 0 {
		return PayAsYouGo
	}
	return Free
}

// Resolve is Effective plus the two free-Pro paths: an org whose owner email is a
// platform operator, or is on the premium allowlist, gets Pro regardless of
// subscription or credit. This is the one place both grants are applied, so
// enforcement (monitor create, control-plane cap) and the billing page all agree on
// who is premium — and only those owners are, never every user.
func Resolve(subscribed Plan, subscriptionActive bool, creditSeconds int64, ownerEmail string) Plan {
	if IsAdmin(ownerEmail) || IsGranted(ownerEmail) {
		return Pro
	}
	return Effective(subscribed, subscriptionActive, creditSeconds)
}

// LimitsFor returns the live limits for a plan, falling back to Free for unknown
// values (defensive: an unexpected DB value must never grant unlimited access).
func LimitsFor(p Plan) Limits {
	c := live.Load()
	if l, ok := c.Limits[p]; ok {
		return l
	}
	return c.Limits[Free]
}

// Info is the customer-facing description of a plan, used by the billing/pricing
// page. Prices are USD per month.
type Info struct {
	Plan         Plan
	Name         string
	PriceMonthly int
	Limits       Limits
	Features     []string
}

// Catalog returns the ordered list of purchasable plans for the pricing page, built
// from the LIVE configuration so an operator's price/limit changes show immediately.
func Catalog() []Info {
	c := live.Load()
	names := map[Plan]string{Free: "Free", Starter: "Starter", Pro: "Pro"}
	out := make([]Info, 0, 3)
	for _, p := range []Plan{Free, Starter, Pro} {
		l := c.Limits[p]
		out = append(out, Info{
			Plan:         p,
			Name:         names[p],
			PriceMonthly: c.Prices[p],
			Limits:       l,
			Features:     features(p, l),
		})
	}
	return out
}

func features(p Plan, l Limits) []string {
	out := []string{
		fmt.Sprintf("%d monitors", l.MaxMonitors),
		fmt.Sprintf("%ds minimum interval", l.MinIntervalSeconds),
	}
	return append(out, featureTail[p]...)
}
