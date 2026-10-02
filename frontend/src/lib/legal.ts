import { brand } from "@/brand";
import { apiInternal, siteUrl } from "@/lib/site";

// The operator details the Terms and Privacy Policy name, resolved from the brand with
// safe defaults so every white-label renders complete policies. Set `brand.legal` to
// the real registered company before launch.
export const legal = {
  entity: brand.legal?.entity ?? brand.name,
  address: brand.legal?.address,
  jurisdiction: brand.legal?.jurisdiction ?? "England and Wales",
  // Bump when either policy's substance changes.
  updated: "2 October 2026",
};

// Used only until a platform admin sets the support email at /platform.
const fallbackSupportEmail =
  brand.legal?.contactEmail ?? `support@${new URL(siteUrl).hostname.replace(/^www\./, "")}`;

// getSupportEmail returns the public support address an admin set at /platform, read
// live from the API (cached for a minute) so a change shows on the site without a
// redeploy. Never throws: at build time or if the API is unreachable it falls back to
// the brand default and corrects itself on the next revalidation.
export async function getSupportEmail(): Promise<string> {
  try {
    const res = await fetch(`${apiInternal}/api/v1/public/site`, { next: { revalidate: 60 } });
    if (res.ok) {
      const { support_email } = (await res.json()) as { support_email?: string };
      if (support_email) return support_email;
    }
  } catch {
    // fall through to the default
  }
  return fallbackSupportEmail;
}
