"use client";

import { useEffect, useState } from "react";

import { useBilling, useStartSubscription, useStartTopUp, useUsage } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, PageHeader, Skeleton } from "@/components/ui";
import { CheckIcon } from "@/components/icons";
import type { BillingInfo, PlanInfo } from "@/lib/types";

type Notice = { kind: "ok" | "err"; text: string } | null;

const PLAN_LABEL: Record<string, string> = {
  free: "Free",
  starter: "Starter",
  pro: "Pro",
  payg: "Pay-as-you-go",
};

export default function BillingPage() {
  const { data, isLoading } = useBilling();
  const { data: usage } = useUsage();
  const { user } = useAuth();
  const [notice, setNotice] = useState<Notice>(null);

  const canManage = user?.role === "owner" || user?.role === "admin";

  // Surface the Stripe Checkout return (?checkout=success|cancel) as a notice.
  useEffect(() => {
    const p = new URLSearchParams(window.location.search).get("checkout");
    if (p === "success") {
      setNotice({ kind: "ok", text: "Payment received — your account updates within a few seconds." });
    } else if (p === "cancel") {
      setNotice({ kind: "err", text: "Checkout canceled. Nothing was charged." });
    }
    if (p) window.history.replaceState(null, "", window.location.pathname);
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Plans & billing"
        subtitle="Pay as you go by the hour, or subscribe monthly. It's your call."
      />

      {notice && (
        <div
          role={notice.kind === "ok" ? "status" : "alert"}
          className={`rounded-lg px-4 py-2 text-sm font-medium ${
            notice.kind === "ok"
              ? "bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200"
              : "bg-red-50 text-red-800 dark:bg-red-900/30 dark:text-red-200"
          }`}
        >
          {notice.text}
        </div>
      )}

      {data && !data.billing_enabled && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
          Payments are not configured on this deployment, so checkout is disabled. Everything below is
          read-only.
        </div>
      )}

      {isLoading || !data ? (
        <>
          <Skeleton className="h-44 w-full rounded-xl" />
          <div className="grid gap-4 md:grid-cols-3">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-72 w-full rounded-xl" />
            ))}
          </div>
        </>
      ) : (
        <>
          <CreditCard
            info={data}
            monitors={usage?.monitors_used ?? 0}
            canManage={canManage}
            setNotice={setNotice}
          />
          <PlansGrid info={data} canManage={canManage} setNotice={setNotice} />
        </>
      )}
    </div>
  );
}

// ---- Pay-as-you-go credit ----

function CreditCard({
  info,
  monitors,
  canManage,
  setNotice,
}: {
  info: BillingInfo;
  monitors: number;
  canManage: boolean;
  setNotice: (n: Notice) => void;
}) {
  const topUp = useStartTopUp();
  const [dollars, setDollars] = useState("5");

  const monitorHours = info.credit_seconds / 3600;
  // "How long will this last?" — credit ÷ (enabled monitors) at the current count.
  const wallHours = monitors > 0 ? info.credit_seconds / monitors / 3600 : null;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setNotice(null);
    const amount = Math.round(parseFloat(dollars) * 100);
    if (!Number.isFinite(amount) || amount < 100) {
      setNotice({ kind: "err", text: "Enter at least $1." });
      return;
    }
    try {
      const { checkout_url } = await topUp.mutateAsync(amount);
      window.location.href = checkout_url; // hand off to Stripe Checkout
    } catch (err) {
      setNotice({ kind: "err", text: err instanceof ApiRequestError ? err.message : "Could not start checkout" });
    }
  };

  const active = info.effective_plan === "payg";

  return (
    <Card className={`border-l-4 ${active ? "border-l-brand-500" : "border-l-slate-300 dark:border-l-slate-700"}`}>
      <div className="flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
            Pay-as-you-go balance
          </h2>
          <p className="mt-1 text-4xl font-bold tabular-nums text-slate-900 dark:text-white">
            {monitorHours.toLocaleString(undefined, { maximumFractionDigits: 1 })}{" "}
            <span className="text-lg font-medium text-slate-500 dark:text-slate-400">monitor-hours</span>
          </p>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
            {wallHours !== null && info.credit_seconds > 0 ? (
              <>
                ≈{" "}
                <span className="font-medium text-slate-900 dark:text-white tabular-nums">
                  {wallHours.toLocaleString(undefined, { maximumFractionDigits: 1 })} hours
                </span>{" "}
                at your current {monitors} monitor{monitors === 1 ? "" : "s"}.
              </>
            ) : (
              <>Each monitor uses one hour of credit per hour it runs.</>
            )}{" "}
            <span className="text-slate-500 dark:text-slate-400">
              $1 = {info.monitor_hours_per_dollar} monitor-hours.
            </span>
          </p>
        </div>

        <form onSubmit={submit} className="flex shrink-0 items-stretch gap-2">
          <div className="flex items-stretch overflow-hidden rounded-lg border border-slate-300 focus-within:ring-2 focus-within:ring-brand-500 dark:border-slate-700">
            <span className="flex items-center bg-slate-50 px-3 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
              $
            </span>
            <input
              type="number"
              min={1}
              step={1}
              value={dollars}
              onChange={(e) => setDollars(e.target.value)}
              aria-label="Top-up amount in dollars"
              className="w-24 bg-white px-3 py-2.5 text-base tabular-nums text-slate-900 focus:outline-none dark:bg-slate-900 dark:text-white"
            />
          </div>
          <Button type="submit" disabled={!canManage || !info.billing_enabled || topUp.isPending}>
            {topUp.isPending ? "Starting…" : "Add credit"}
          </Button>
        </form>
      </div>
      {!canManage && (
        <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">Only owners and admins can add credit.</p>
      )}
    </Card>
  );
}

// ---- Subscriptions ----

function PlansGrid({
  info,
  canManage,
  setNotice,
}: {
  info: BillingInfo;
  canManage: boolean;
  setNotice: (n: Notice) => void;
}) {
  const subscribe = useStartSubscription();

  const onSubscribe = async (p: PlanInfo) => {
    setNotice(null);
    try {
      const { checkout_url } = await subscribe.mutateAsync(p.id);
      window.location.href = checkout_url;
    } catch (err) {
      setNotice({ kind: "err", text: err instanceof ApiRequestError ? err.message : "Could not start checkout" });
    }
  };

  const subscribed = info.subscribed_plan;
  const subActive =
    info.subscription_status === "active" || info.subscription_status === "trialing";

  return (
    <div>
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-lg font-semibold">Monthly subscriptions</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Currently effective: <span className="font-medium">{PLAN_LABEL[info.effective_plan] ?? info.effective_plan}</span>
        </p>
      </div>
      <div className="grid gap-5 md:grid-cols-3">
        {info.plans.map((p) => {
          const isCurrent = subActive ? p.id === subscribed : p.id === "free";
          const isPaid = p.id === "starter" || p.id === "pro";
          return (
            <Card key={p.id} className={`relative flex flex-col ${isCurrent ? "ring-2 ring-brand-500" : ""}`}>
              {isCurrent && (
                <span className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-brand-600 px-3 py-0.5 text-xs font-medium text-white">
                  Current plan
                </span>
              )}
              <h3 className="text-lg font-bold">{p.name}</h3>
              <p className="mt-1">
                <span className="text-3xl font-bold tabular-nums">${p.price_monthly}</span>
                <span className="text-sm text-slate-500 dark:text-slate-400">/mo</span>
              </p>
              <ul className="mt-4 flex-1 space-y-2 text-sm">
                {p.features.map((f) => (
                  <li key={f} className="flex items-start gap-2 text-slate-600 dark:text-slate-300">
                    <CheckIcon className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                    {f}
                  </li>
                ))}
              </ul>
              <div className="mt-5">
                {isCurrent ? (
                  <Button variant="secondary" className="w-full" disabled>
                    Your plan
                  </Button>
                ) : isPaid ? (
                  <Button
                    variant="primary"
                    className="w-full"
                    disabled={!canManage || !info.billing_enabled || subscribe.isPending}
                    onClick={() => onSubscribe(p)}
                  >
                    {subscribe.isPending ? "Starting…" : `Subscribe to ${p.name}`}
                  </Button>
                ) : (
                  <Button variant="secondary" className="w-full" disabled>
                    Default tier
                  </Button>
                )}
              </div>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
