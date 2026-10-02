import type { Metadata } from "next";
import Link from "next/link";

import { brand } from "@/brand";
import { LegalPage } from "@/components/legal/legal-page";
import { getSupportEmail, legal } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Terms of Service",
  description: `The terms that govern your use of ${brand.name}: accounts, acceptable use, plans and billing, and our responsibilities to each other.`,
  alternates: { canonical: "/terms" },
};

export default async function TermsPage() {
  const supportEmail = await getSupportEmail();
  const mail = `mailto:${supportEmail}`;
  return (
    <LegalPage title="Terms of Service">
      <p>
        These terms are an agreement between you and {legal.entity} (&ldquo;we&rdquo;, &ldquo;us&rdquo;) for your use
        of {brand.name} (the &ldquo;Service&rdquo;). By creating an account or using the Service you accept them. If you
        use the Service for an organisation, you confirm you are authorised to accept these terms on its behalf.
      </p>

      <h2>1. The Service</h2>
      <p>
        {brand.name} monitors websites, APIs, servers, certificates, scheduled jobs and CI workflows, alerts you when
        something fails, and hosts optional public status pages. Features and limits depend on your plan, as described
        on our <Link href="/#pricing">pricing page</Link>. We may improve or change features over time; if a change
        materially reduces a paid feature you rely on, we will tell you in advance.
      </p>

      <h2>2. Your account</h2>
      <ul>
        <li>You must be at least 16 and give accurate information, including a working email address.</li>
        <li>
          Keep your password and API keys secret. You are responsible for activity under your account and its API
          keys. Tell us promptly at <a href={mail}>{supportEmail}</a> if you suspect unauthorised access.
        </li>
        <li>
          You can delete your account at any time from the <Link href="/account">Account</Link> page. Deletion is
          permanent and cancels any active subscription.
        </li>
      </ul>

      <h2>3. Acceptable use</h2>
      <p>
        The Service sends real network traffic to the targets you configure, so you must only monitor systems that you
        own or are authorised to monitor. You must not use the Service to:
      </p>
      <ul>
        <li>probe, scan, load-test or attack systems you are not authorised to test, or to cause a denial of service;</li>
        <li>break the law, infringe others&apos; rights, or publish unlawful, misleading or harmful content on a status page;</li>
        <li>send spam or malicious content through notification channels;</li>
        <li>circumvent plan limits, rate limits or security controls, or interfere with the Service or other customers;</li>
        <li>resell or provide the Service to third parties as a standalone product without our written permission.</li>
      </ul>
      <p>We may pause monitors or suspend accounts that breach this section, with notice where practical.</p>

      <h2>4. Your data</h2>
      <p>
        You keep ownership of everything you put into the Service. You give us permission to store, process and
        transmit it only as needed to run the Service for you — for example, to probe your targets and deliver your
        alerts. Status pages you publish are public by design. Our <Link href="/privacy">Privacy Policy</Link> explains
        how we handle personal data.
      </p>

      <h2>5. Plans, billing and cancellation</h2>
      <ul>
        <li>
          Paid subscriptions are billed monthly in advance through our payment provider, Stripe, and renew
          automatically until cancelled. Prices are shown before you pay and include or add tax as stated at checkout.
        </li>
        <li>
          You can cancel at any time from Billing &rarr; Manage billing. Cancellation takes effect at the end of the
          current billing period; you keep paid features until then.
        </li>
        <li>
          Pay-as-you-go credit is prepaid, is consumed as your monitors run, and does not expire while your account is
          open. It is non-refundable and non-transferable, except where the law requires otherwise.
        </li>
        <li>
          If a payment fails we will let you know; if it is not resolved, your account moves to the free plan&apos;s
          limits, which may pause monitors above that limit.
        </li>
        <li>We may change prices with at least 30 days&apos; notice; changes apply from your next billing period.</li>
      </ul>

      <h2>6. Availability and alerts</h2>
      <p>
        We work hard to keep the Service running and your alerts timely, but we do not guarantee uninterrupted
        availability or that every alert will be delivered, unless we have agreed a service level with you in writing.
        Alerts depend on networks and third-party services (such as email, Slack or Telegram) outside our control. Do
        not rely on the Service as the only safeguard for systems where a missed alert could cause injury or serious
        loss.
      </p>

      <h2>7. Third-party services</h2>
      <p>
        Integrations you connect — GitHub, Slack, Telegram, Discord, Microsoft Teams, Stripe, Google or Apple sign-in —
        are provided by third parties under their own terms. We are not responsible for them.
      </p>

      <h2>8. Suspension and termination</h2>
      <p>
        You may stop using the Service and delete your account at any time. We may suspend or end your access if you
        materially breach these terms, if required by law, or if your use puts the Service or others at risk. Where we
        end the Service for reasons other than your breach, we will refund any prepaid subscription fees for the unused
        period.
      </p>

      <h2>9. Liability</h2>
      <p>
        The Service is provided &ldquo;as is&rdquo; to the extent the law allows. We are not liable for indirect or
        consequential loss, or for loss of profit, revenue, data or goodwill. Our total liability to you in any 12-month
        period is limited to the greater of the fees you paid us in that period and £100. Nothing in these terms limits
        liability for death or personal injury caused by negligence, for fraud, or for anything else that cannot
        lawfully be limited, and nothing affects your statutory rights as a consumer.
      </p>

      <h2>10. Changes to these terms</h2>
      <p>
        We may update these terms. For material changes we will give you at least 30 days&apos; notice by email or in
        the app. If you do not agree, you may delete your account before the change takes effect.
      </p>

      <h2>11. Governing law</h2>
      <p>
        These terms are governed by the law of {legal.jurisdiction}, and its courts have jurisdiction over any dispute,
        except that consumers may also bring proceedings where they live.
      </p>

      <h2>12. Contact</h2>
      <p>
        Questions about these terms? Email <a href={mail}>{supportEmail}</a>
        {legal.address ? ` or write to ${legal.entity}, ${legal.address}` : ""}.
      </p>
    </LegalPage>
  );
}
