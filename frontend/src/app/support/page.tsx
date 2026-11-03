import type { Metadata } from "next";
import Link from "next/link";

import { brand } from "@/brand";
import { LegalPage } from "@/components/legal/legal-page";
import { getLegal } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Support",
  description: `How to get help with ${brand.name} on the web, iPhone and iPad.`,
  alternates: { canonical: "/support" },
};

// The App Store "Support URL": app review requires a page that offers a way to
// contact us. The address is the one set at /platform → Company & contact.
export default async function SupportPage() {
  const legal = await getLegal();
  return (
    <LegalPage title="Support">
      <p>
        Need help with {brand.name} on the web, iPhone or iPad? Email{" "}
        <a href={`mailto:${legal.supportEmail}`}>{legal.supportEmail}</a> and we&apos;ll get back to you.
      </p>

      <h2>Help yourself</h2>
      <ul>
        <li>
          <Link href="/docs">Documentation</Link> — setting up monitors, alerts, notification channels and status pages.
        </li>
        <li>
          Forgot your password? <Link href="/forgot-password">Reset it here</Link>.
        </li>
        <li>
          Not getting push alerts on your phone? In the app, open Settings → Notification Channels and tap Test. If
          nothing arrives, check that notifications are allowed for the app in your phone&apos;s Settings.
        </li>
        <li>
          To delete your account and all its data: in the app, Settings → Delete Account; on the web, Account → Delete
          account.
        </li>
      </ul>

      <p>
        {legal.entity}
        {legal.address ? `, ${legal.address}` : ""}
      </p>
    </LegalPage>
  );
}
