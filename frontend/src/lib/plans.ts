// The public pricing tiers. One source for both the marketing pricing section and the
// schema.org Offers emitted for rich results. Display text (localised names, taglines,
// feature bullets) lives in the marketing i18n catalog, keyed by `id`; the money-shaped
// facts live here.
//
// ponytail: static marketing figures, mirrored from /docs/plans and the DB seed defaults.
// Platform admins can change the LIVE prices/limits at /platform (DB-backed); if you
// change them there, update these too — or wire a public /api/v1/plans endpoint and fetch
// it. Kept static to match /docs/plans and avoid a network call on the landing page.
export interface Plan {
  id: "free" | "starter" | "pro" | "payg";
  /** English name for schema.org Offer (structured data is emitted in the base language). */
  name: string;
  /** Numeric amount for schema.org Offer.price. PAYG uses its entry price. */
  price: number;
  currency: "USD";
  /** How the price reads on the card, e.g. "$0", "$19", "from $1". */
  priceLabel: string;
  /** Show a "/month" suffix and mark this as a recurring subscription. */
  perMonth?: boolean;
  /** The visually highlighted plan. */
  featured?: boolean;
}

export const PLANS: Plan[] = [
  { id: "free", name: "Free", price: 0, currency: "USD", priceLabel: "$0" },
  { id: "starter", name: "Starter", price: 19, currency: "USD", priceLabel: "$19", perMonth: true, featured: true },
  { id: "pro", name: "Pro", price: 79, currency: "USD", priceLabel: "$79", perMonth: true },
  { id: "payg", name: "Pay-as-you-go", price: 1, currency: "USD", priceLabel: "from $1" },
];

// The live pricing served by GET /api/v1/public/plans — the operator-tuned figures the
// landing page fetches server-side so admin price changes show without a redeploy. Only
// the subscribable tiers (free/starter/pro) appear; PAYG is described from the rate.
// Fetched in app/page.tsx; the static PLANS above are the build-time / fetch-failure
// fallback and the schema.org base. Type only — the fetch lives server-side.
export interface LivePlan {
  id: string;
  /** Operator-editable one-line pitch; defaults applied server-side. */
  tagline: string;
  price_monthly: number;
  max_monitors: number;
  min_interval_seconds: number;
  monthly_diagnoses: number;
  /** Operator-editable marketing bullets; defaults applied server-side. The numeric
   *  bullets (monitors/interval/AI) are derived on the card from the numbers above. */
  highlights: string[];
}
export interface LivePlans {
  monitor_hours_per_dollar: number;
  plans: LivePlan[];
}
