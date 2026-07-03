"use client";

import { useState } from "react";
import { useBilling, useChangePlan } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { ApiRequestError } from "@/lib/api";
import { Button, Card } from "@/components/ui";
import type { PlanInfo } from "@/lib/types";

export default function BillingPage() {
  const { data, isLoading } = useBilling();
  const { user } = useAuth();
  const changePlan = useChangePlan();
  const [notice, setNotice] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  const canManage = user?.role === "owner" || user?.role === "admin";
  const current = data?.current_plan;
  const currentIdx = data ? data.plans.findIndex((p) => p.id === current) : -1;

  const onSwitch = async (p: PlanInfo) => {
    setNotice(null);
    try {
      await changePlan.mutateAsync(p.id);
      setNotice({ kind: "ok", text: `You're now on the ${p.name} plan.` });
    } catch (e) {
      setNotice({ kind: "err", text: e instanceof ApiRequestError ? e.message : "Could not change plan" });
    }
  };

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Plans &amp; billing</h1>
        <p className="text-sm text-slate-500">Choose the plan that fits your monitoring needs.</p>
      </div>

      {notice && (
        <div
          className={`rounded-lg px-4 py-2 text-sm ${
            notice.kind === "ok"
              ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
              : "bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300"
          }`}
        >
          {notice.text}
        </div>
      )}

      {isLoading ? (
        <p className="text-slate-500">Loading…</p>
      ) : (
        <div className="grid gap-5 md:grid-cols-3">
          {data?.plans.map((p, idx) => {
            const isCurrent = p.id === current;
            const isUpgrade = idx > currentIdx;
            return (
              <Card
                key={p.id}
                className={`relative flex flex-col ${
                  isCurrent ? "ring-2 ring-brand-500" : ""
                }`}
              >
                {isCurrent && (
                  <span className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-brand-600 px-3 py-0.5 text-xs font-medium text-white">
                    Current plan
                  </span>
                )}
                <h2 className="text-lg font-bold">{p.name}</h2>
                <p className="mt-1">
                  <span className="text-3xl font-bold">${p.price_monthly}</span>
                  <span className="text-sm text-slate-500">/mo</span>
                </p>
                <ul className="mt-4 flex-1 space-y-2 text-sm">
                  {p.features.map((f) => (
                    <li key={f} className="flex items-start gap-2 text-slate-600 dark:text-slate-300">
                      <span className="text-emerald-500">✓</span>
                      {f}
                    </li>
                  ))}
                </ul>
                <div className="mt-5">
                  {isCurrent ? (
                    <Button variant="secondary" className="w-full" disabled>
                      Your plan
                    </Button>
                  ) : canManage ? (
                    <Button
                      variant={isUpgrade ? "primary" : "secondary"}
                      className="w-full"
                      disabled={changePlan.isPending}
                      onClick={() => onSwitch(p)}
                    >
                      {changePlan.isPending ? "Switching…" : isUpgrade ? `Upgrade to ${p.name}` : `Switch to ${p.name}`}
                    </Button>
                  ) : (
                    <Button variant="secondary" className="w-full" disabled>
                      Owner only
                    </Button>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      <p className="text-center text-xs text-slate-400">
        This is a self-serve switch for the demo. A production deployment routes upgrades through a payment
        provider (e.g. Stripe Checkout) and sets the plan from the billing webhook.
      </p>
    </div>
  );
}
