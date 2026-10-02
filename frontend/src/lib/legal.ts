import { brand } from "@/brand";
import { siteUrl } from "@/lib/site";

// The operator details the Terms and Privacy Policy name, resolved from the brand with
// safe defaults so every white-label renders complete policies. Set `brand.legal` to
// the real registered company before launch.
export const legal = {
  entity: brand.legal?.entity ?? brand.name,
  address: brand.legal?.address,
  jurisdiction: brand.legal?.jurisdiction ?? "England and Wales",
  contactEmail: brand.legal?.contactEmail ?? `support@${new URL(siteUrl).hostname.replace(/^www\./, "")}`,
  // Bump when either policy's substance changes.
  updated: "2 October 2026",
};
