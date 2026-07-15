// Package plan defines subscription plans and their resource limits. Limits are
// enforced when tenants create or update monitors so that one organization
// cannot overload the shared monitoring engines (Prometheus/Blackbox). Values
// live here in code — not the database — so they can be tuned without a
// migration; the organization row only stores which plan it is on.
package plan

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

// Limits are the per-organization resource caps a plan grants.
type Limits struct {
	// MaxMonitors caps the number of non-deleted monitors an org may have.
	MaxMonitors int
	// MinIntervalSeconds is the fastest check interval the org may configure.
	MinIntervalSeconds int
}

// registry maps each plan to its limits. Tune here, no migration needed.
var registry = map[Plan]Limits{
	Free:       {MaxMonitors: 10, MinIntervalSeconds: 60},
	Starter:    {MaxMonitors: 50, MinIntervalSeconds: 30},
	Pro:        {MaxMonitors: 500, MinIntervalSeconds: 10},
	PayAsYouGo: {MaxMonitors: 500, MinIntervalSeconds: 30},
}

// Valid reports whether p is a known plan.
func (p Plan) Valid() bool {
	_, ok := registry[p]
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

// LimitsFor returns the limits for a plan, falling back to Free for unknown
// values (defensive: an unexpected DB value must never grant unlimited access).
func LimitsFor(p Plan) Limits {
	if l, ok := registry[p]; ok {
		return l
	}
	return registry[Free]
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

// Catalog returns the ordered list of purchasable plans for the pricing page.
func Catalog() []Info {
	return []Info{
		{Free, "Free", 0, registry[Free], []string{
			"10 monitors", "60s minimum interval", "Telegram alerts", "Community support",
		}},
		{Starter, "Starter", 19, registry[Starter], []string{
			"50 monitors", "30s minimum interval", "All alert channels", "Email support",
		}},
		{Pro, "Pro", 79, registry[Pro], []string{
			"500 monitors", "10s minimum interval", "Priority alerting", "Priority support",
		}},
	}
}
