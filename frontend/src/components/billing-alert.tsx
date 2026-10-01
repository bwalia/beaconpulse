"use client";

import Link from "next/link";

import { useBilling } from "@/lib/hooks";

// Stripe subscription statuses that mean a payment problem the customer must act on:
// their card failed and the paid tier is lapsing or already lapsed. "canceled" is
// deliberately excluded — that is an expected end state, not a failure to flag.
const DUNNING = new Set(["past_due", "unpaid", "incomplete"]);

// BillingAlert shows a persistent banner when the org's subscription is in a
// failed-payment state. It lives in the dashboard layout so it is seen from whatever
// page the user is on — a silent drop to the Free limits is exactly what otherwise
// goes unnoticed until monitors start disappearing. Renders nothing in the common
// (healthy / never-subscribed) case.
export function BillingAlert() {
  const { data } = useBilling();
  if (!data || !DUNNING.has(data.subscription_status)) return null;

  const unpaid = data.subscription_status === "unpaid";
  return (
    <div
      role="alert"
      className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
    >
      <span>
        <strong className="font-semibold">Payment problem.</strong>{" "}
        {unpaid
          ? "Your subscription is unpaid and has dropped to the Free limits. Update your card to restore your plan."
          : "We couldn't charge your card. Update it to keep your plan before it lapses to Free."}
      </span>
      <Link
        href="/billing"
        className="shrink-0 rounded-md bg-red-600 px-3 py-1.5 font-medium text-white hover:bg-red-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500"
      >
        Update payment method
      </Link>
    </div>
  );
}
