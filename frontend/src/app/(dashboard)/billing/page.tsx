"use client";

import { useState } from "react";
import { useBilling, useChangePlan } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, PageHeader, Skeleton } from "@/components/ui";
import type { PlanInfo } from "@/lib/types";
import { CheckIcon } from "@/components/icons";

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
    <div className="space-y-6">
      <PageHeader title="Plans & billing" subtitle="Choose the plan that fits your monitoring needs." />

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

      {isLoading ? (
        <div className="grid gap-4 md:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-80 w-full rounded-xl" />
          ))}
        </div>
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

      <p className="text-center text-xs text-slate-500 dark:text-slate-400">
        This is a self-serve switch for the demo. A production deployment routes upgrades through a payment
        provider (e.g. Stripe Checkout) and sets the plan from the billing webhook.
      </p>
    </div>
  );
}
