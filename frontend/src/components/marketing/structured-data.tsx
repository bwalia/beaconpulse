import { brand } from "@/brand";
import { PLANS } from "@/lib/plans";
import { absoluteUrl, siteUrl } from "@/lib/site";

// schema.org JSON-LD for the landing page: Organization + WebSite + SoftwareApplication.
// The SoftwareApplication carries an Offer per pricing tier, so search engines can show
// price-rich results for the product. Emitted server-side as a raw <script> — this is the
// one place dangerouslySetInnerHTML is the correct tool (JSON-LD has no React equivalent),
// and the payload is our own static data, so there is nothing to escape from user input.
export function StructuredData() {
  const ogImage = absoluteUrl("/opengraph-image");

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
        offers: PLANS.map((p) => ({
          "@type": "Offer",
          name: p.name,
          price: p.price,
          priceCurrency: p.currency,
          ...(p.perMonth
            ? { priceSpecification: { "@type": "UnitPriceSpecification", price: p.price, priceCurrency: p.currency, unitCode: "MON" } }
            : {}),
        })),
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
