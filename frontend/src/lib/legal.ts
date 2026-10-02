import { brand } from "@/brand";
import { apiInternal, siteUrl } from "@/lib/site";

// Bump when either policy's substance changes.
export const LEGAL_UPDATED = "2 October 2026";

export type LegalDetails = {
  /** The company that operates the service. */
  entity: string;
  /** Registered address, when set. */
  address?: string;
  /** Governing law of the Terms. */
  jurisdiction: string;
  /** Where account, legal and data requests go. */
  supportEmail: string;
};

// Brand defaults, used for anything a platform admin hasn't set at /platform.
const defaults: LegalDetails = {
  entity: brand.legal?.entity ?? brand.name,
  address: brand.legal?.address,
  jurisdiction: brand.legal?.jurisdiction ?? "England and Wales",
  supportEmail: brand.legal?.contactEmail ?? `support@${new URL(siteUrl).hostname.replace(/^www\./, "")}`,
};

type PublicSite = { support_email?: string; legal_entity?: string; legal_address?: string };

// getLegal returns the company & contact details the Terms and Privacy pages print:
// the values an admin set at /platform, read live from the API (cached for a minute,
// so a change shows without a redeploy), each falling back to the brand default when
// blank. Never throws: at build time or if the API is unreachable it returns the
// defaults and corrects itself on the next revalidation.
export async function getLegal(): Promise<LegalDetails> {
  let site: PublicSite = {};
  try {
    const res = await fetch(`${apiInternal}/api/v1/public/site`, { next: { revalidate: 60 } });
    if (res.ok) site = (await res.json()) as PublicSite;
  } catch {
    // use the defaults
  }
  return {
    entity: site.legal_entity || defaults.entity,
    address: site.legal_address || defaults.address,
    jurisdiction: defaults.jurisdiction,
    supportEmail: site.support_email || defaults.supportEmail,
  };
}
