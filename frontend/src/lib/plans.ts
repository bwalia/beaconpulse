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
