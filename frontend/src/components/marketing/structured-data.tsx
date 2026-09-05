import { brand } from "@/brand";
import { PLANS, type LivePlans } from "@/lib/plans";
import { absoluteUrl, siteUrl } from "@/lib/site";

// schema.org JSON-LD for the landing page: Organization + WebSite + SoftwareApplication.
// The SoftwareApplication carries an Offer per pricing tier, so search engines can show
// price-rich results for the product. Emitted server-side as a raw <script> — this is the
// one place dangerouslySetInnerHTML is the correct tool (JSON-LD has no React equivalent),
// and the payload is our own static data, so there is nothing to escape from user input.
export function StructuredData({ live }: { live: LivePlans | null }) {
  const ogImage = absoluteUrl("/opengraph-image");

  // Prefer live prices so the Offers match the cards; fall back to the static figure.
  const priceById = new Map((live?.plans ?? []).map((p) => [p.id, p.price_monthly]));
  const priceOf = (id: string, fallback: number) => priceById.get(id) ?? fallback;

  const graph = {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "Organization",
        "@id": `${siteUrl}/#organization`,
        name: brand.name,
        url: siteUrl,
        logo: ogImage,
      },
      {
        "@type": "WebSite",
        "@id": `${siteUrl}/#website`,
        name: brand.name,
        url: siteUrl,
        publisher: { "@id": `${siteUrl}/#organization` },
      },
      {
        "@type": "SoftwareApplication",
        name: brand.name,
        description: brand.description,
        url: siteUrl,
        image: ogImage,
        applicationCategory: "BusinessApplication",
        operatingSystem: "Web, iOS, iPadOS",
        publisher: { "@id": `${siteUrl}/#organization` },
        offers: PLANS.map((p) => {
          // PAYG's entry price ("from $1") is not a catalog price, so it stays static.
          const price = p.id === "payg" ? p.price : priceOf(p.id, p.price);
          return {
            "@type": "Offer",
            name: p.name,
            price,
            priceCurrency: p.currency,
            ...(p.perMonth
              ? { priceSpecification: { "@type": "UnitPriceSpecification", price, priceCurrency: p.currency, unitCode: "MON" } }
              : {}),
          };
        }),
      },
    ],
  };

  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(graph) }}
    />
  );
}
