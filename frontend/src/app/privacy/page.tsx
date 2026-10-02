import type { Metadata } from "next";
import Link from "next/link";

import { brand } from "@/brand";
import { LegalPage } from "@/components/legal/legal-page";
import { legal } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description: `How ${brand.name} collects, uses, shares and protects your personal data, and the rights you have over it.`,
  alternates: { canonical: "/privacy" },
};

// Every statement here must stay true of the running product. When a feature starts
// collecting or sharing something new, update this page (and legal.updated) with it.
export default function PrivacyPage() {
  const mail = `mailto:${legal.contactEmail}`;
  return (
    <LegalPage title="Privacy Policy">
      <p>
        This policy explains how {legal.entity} (&ldquo;we&rdquo;, &ldquo;us&rdquo;) handles personal data when you
        use {brand.name} (the &ldquo;Service&rdquo;), including our website, web app and mobile apps. We are the
        controller of the personal data described here. Questions? Email{" "}
        <a href={mail}>{legal.contactEmail}</a>
        {legal.address ? `, or write to ${legal.address}` : ""}.
      </p>

      <h2>What we collect</h2>
      <ul>
        <li>
          <strong>Account details</strong> — your name, email address and organisation name. Your password is stored
          only as a one-way hash; we can never read it. If you sign in with Google or Apple we receive your account
          identifier, email address and (from Google) your name.
        </li>
        <li>
          <strong>What you ask us to monitor</strong> — monitor names, the URLs, hosts and repositories you watch, and
          their settings, including any request headers you add. Notification settings (such as Slack, Telegram or
          webhook details and SMTP credentials) are encrypted at rest.
        </li>
        <li>
          <strong>Monitoring results</strong> — uptime, response times, certificate details and alert history for your
          monitors. For GitHub Actions monitors we receive each run&apos;s outcome, repository, workflow, branch, commit
          and the GitHub username that triggered it.
        </li>
        <li>
          <strong>Billing</strong> — your plan, credit balance and payment-provider customer reference. Card details
          go directly to Stripe; we never see or store your full card number.
        </li>
        <li>
          <strong>Mobile push</strong> — if you enable notifications in our app, a device push token so we can deliver
          alerts to that device.
        </li>
        <li>
          <strong>Security and usage records</strong> — the IP address and browser or app details used to sign in,
          and a log of significant account actions, which we keep to protect your account and investigate abuse.
        </li>
      </ul>

      <h2>How we use it, and our legal basis</h2>
      <ul>
        <li>
          <strong>To provide the Service</strong> (performance of our contract with you): running your monitors,
          sending your alerts, publishing status pages you choose to make public, and processing payments.
        </li>
        <li>
          <strong>To keep it secure and working</strong> (our legitimate interests): preventing fraud and abuse,
          rate-limiting, debugging and capacity planning.
        </li>
        <li>
          <strong>To send account email</strong> (contract): alerts you configure, password resets and receipts. We do
          not send marketing email without your consent.
        </li>
        <li>
          <strong>To meet legal obligations</strong>: for example, keeping billing records for tax purposes.
        </li>
      </ul>
      <p>
        Optional AI features summarise alerts and diagnose failing monitors by processing the relevant monitor and
        alert details with a language model. We do not use your data to train AI models, and we do not sell your
        personal data.
      </p>

      <h2>Who we share it with</h2>
      <p>We share personal data only with service providers that help us run the Service, and only as needed:</p>
      <ul>
        <li>
          <strong>Stripe</strong> — payment processing, invoices and the billing portal.
        </li>
        <li>
          <strong>Google and Apple</strong> — only if you choose to sign in with them; Apple also delivers push
          notifications to iOS devices.
        </li>
        <li>
          <strong>Our email delivery provider</strong> — to send alerts, password resets and other account email.
        </li>
        <li>
          <strong>Our hosting and infrastructure providers</strong> — to store and process data on our behalf.
        </li>
      </ul>
      <p>
        When you connect a notification channel (Slack, Telegram, Discord, Microsoft Teams, email or a webhook), we
        send alert details to that service at your direction; its own privacy policy then applies. Anything you put on
        a public status page is visible to anyone. We may also disclose data where the law requires it.
      </p>
      <p>
        Some providers process data outside the UK. Where they do, the transfer is protected by an adequacy decision
        or standard contractual clauses.
      </p>

      <h2>Cookies and local storage</h2>
      <p>
        We use only what the Service needs to work: your browser&apos;s local storage keeps you signed in and remembers
        your theme and language, and a secure, HTTP-only cookie authorises access to the monitoring console. We use no
        advertising or analytics cookies and do not track you across other sites, so there is no cookie banner to
        accept.
      </p>

      <h2>How long we keep it</h2>
      <ul>
        <li>Account and monitor data: for as long as your account exists.</li>
        <li>Monitoring metrics: a rolling window (30 days by default), after which they expire automatically.</li>
        <li>
          When you delete your account, your organisation, users, monitors, notification settings, API keys, devices
          and activity log are erased from our live systems immediately. Copies in backups expire on their normal
          rotation.
        </li>
        <li>
          Invoices and payment records are retained by Stripe and by us for as long as tax law requires, even after
          you delete your account.
        </li>
      </ul>

      <h2>Your rights</h2>
      <p>
        Under UK and EU data-protection law you can ask to access, correct, export or erase your personal data, and to
        restrict or object to how we use it. You can delete your account yourself at any time from the{" "}
        <Link href="/account">Account</Link> page in the app. For anything else, email{" "}
        <a href={mail}>{legal.contactEmail}</a> and we will respond within one month. If you are unhappy with how we
        handle your data, you can complain to the UK Information Commissioner&apos;s Office (
        <a href="https://ico.org.uk/make-a-complaint/" rel="noopener noreferrer">
          ico.org.uk
        </a>
        ) or your local data-protection authority.
      </p>

      <h2>Security</h2>
      <p>
        Traffic to the Service is encrypted in transit, passwords are hashed, and notification secrets are encrypted at
        rest. Access to production systems is restricted to staff who need it. No system is perfectly secure, but we
        will notify you and the regulator without undue delay if a breach affects your data.
      </p>

      <h2>Children</h2>
      <p>The Service is for businesses and professionals and is not directed at anyone under 16.</p>

      <h2>Changes to this policy</h2>
      <p>
        If we change this policy materially we will update the date above and, for significant changes, tell you by
        email or in the app before they take effect. See also our <Link href="/terms">Terms of Service</Link>.
      </p>
    </LegalPage>
  );
}
