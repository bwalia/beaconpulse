"use client";

import { useState } from "react";
import { brand } from "@/brand";
import { useAuth } from "@/lib/auth";
import { usePlatformSettings, useUpdatePlatformSettings } from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Label, PageHeader, Skeleton, Textarea } from "@/components/ui";
import { CheckCircleIcon, LockIcon } from "@/components/icons";
import type { PlanSetting } from "@/lib/types";

// The operator console for platform-GLOBAL configuration: the pay-as-you-go rate,
// per-tier pricing and limits, and the premium email/domain allowlist. Everything
// here affects every tenant, so the page is only reachable by platform operators
// (is_platform_admin) — the backend enforces the same, this is just the surface.
export default function PlatformPage() {
  const { user } = useAuth();
  const isAdmin = !!user?.is_platform_admin;
  const { data, isLoading, isError } = usePlatformSettings(isAdmin);
  const update = useUpdatePlatformSettings();

  const [rate, setRate] = useState(5);
  const [plans, setPlans] = useState<PlanSetting[]>([]);
  const [grants, setGrants] = useState("");
  const [seededKey, setSeededKey] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Seed the form from fetched settings on first arrival and whenever they change on
  // the server (e.g. right after a save re-writes the cache). Done during render — the
  // React-endorsed way to derive state from a changing input, without an effect. The
  // guard makes it run once per change, so it converges rather than looping.
  const dataKey = data ? data.updated_at ?? "loaded" : null;
  if (data && dataKey !== seededKey) {
    setRate(data.monitor_hours_per_dollar);
    setPlans(data.plans);
    setGrants(data.premium_grants.join("\n"));
    setSeededKey(dataKey);
  }

  if (!isAdmin) {
    return (
      <EmptyState icon={<LockIcon className="h-5 w-5" />} title="Platform operators only">
        This screen sets pricing, limits and premium access for the whole platform. Ask an
        operator to add your email to <code className="rounded bg-slate-100 px-1 dark:bg-slate-800">BEACON_PLATFORM_ADMIN_EMAILS</code>.
      </EmptyState>
    );
  }

  type NumericPlanField = "price_monthly" | "max_monitors" | "min_interval_seconds" | "monthly_diagnoses";
  const setField = (i: number, field: NumericPlanField, value: string) =>
    setPlans((prev) => prev.map((p, idx) => (idx === i ? { ...p, [field]: Number(value) } : p)));
  const setTagline = (i: number, value: string) =>
    setPlans((prev) => prev.map((p, idx) => (idx === i ? { ...p, tagline: value } : p)));
  // Textarea holds one bullet per line; kept verbatim while typing (blanks trimmed on save).
  const setFeatures = (i: number, value: string) =>
    setPlans((prev) => prev.map((p, idx) => (idx === i ? { ...p, features: value.split("\n") } : p)));

  const onSave = async () => {
    setErr(null);
    setSaved(false);
    // Accept commas or newlines; drop blanks. The server lowercases and de-dupes.
    const premium_grants = grants
      .split(/[\n,]/)
      .map((s) => s.trim())
      .filter(Boolean);
    try {
      await update.mutateAsync({
        monitor_hours_per_dollar: Number(rate),
        plans: plans.map((p) => ({
          plan: p.plan,
          price_monthly: Number(p.price_monthly),
          max_monitors: Number(p.max_monitors),
          min_interval_seconds: Number(p.min_interval_seconds),
          monthly_diagnoses: Number(p.monthly_diagnoses),
          tagline: p.tagline.trim(),
          features: (p.features ?? []).map((f) => f.trim()).filter(Boolean),
        })),
        premium_grants,
      });
      setSaved(true);
    } catch (e) {
      setErr(e instanceof ApiRequestError ? e.message : "Failed to save settings");
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Platform"
        subtitle="Pricing, plan limits and premium access — applied live across every tenant."
        actions={
          <Button onClick={onSave} disabled={update.isPending || isLoading}>
            {update.isPending ? "Saving…" : "Save changes"}
          </Button>
        }
      />

      {saved && (
        <div className="flex items-center gap-2 rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-2 text-sm text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300">
          <CheckCircleIcon className="h-4 w-4" />
          Saved. New pricing and limits are live now.
        </div>
      )}
      {err && (
        <div className="rounded-lg border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
          {err}
        </div>
      )}

      {isLoading ? (
        <div className="grid gap-4">
          <Skeleton className="h-28 w-full rounded-xl" />
          <Skeleton className="h-64 w-full rounded-xl" />
          <Skeleton className="h-40 w-full rounded-xl" />
        </div>
      ) : isError || !data ? (
        // A load error (not the non-admin gate) — don't render an empty form the admin
        // could submit over real values.
        <EmptyState title="Couldn’t load settings">
          Something went wrong reading the platform settings. Reload the page to try again.
        </EmptyState>
      ) : (
        <>
          {/* Pay-as-you-go rate */}
          <Card>
            <h2 className="text-lg font-semibold">Pay-as-you-go rate</h2>
            <p className="mt-0.5 text-sm text-slate-500 dark:text-slate-400">
              How much monitoring $1 of credit buys. One monitor probed for an hour costs one monitor-hour.
            </p>
            <div className="mt-4 flex flex-wrap items-end gap-3">
              <div>
                <Label htmlFor="rate">Monitor-hours per $1</Label>
                <input
                  id="rate"
                  type="number"
                  min={1}
                  value={rate}
                  onChange={(e) => setRate(Number(e.target.value))}
                  className="w-40 rounded-lg border border-slate-300 bg-white px-3 py-2 text-base text-slate-900 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/30 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                />
              </div>
              <p className="pb-2 text-sm text-slate-500 dark:text-slate-400">
                = ${rate > 0 ? (100 / rate / 100).toFixed(4) : "—"} per monitor-hour
              </p>
            </div>
          </Card>

          {/* Per-plan pricing & limits */}
          <div>
            <h2 className="mb-3 text-lg font-semibold">Plans</h2>
            <div className="grid gap-4 lg:grid-cols-3">
              {plans.map((p, i) => (
                <Card key={p.plan}>
                  <div className="mb-3 flex items-baseline justify-between">
                    <h3 className="text-base font-semibold">{p.name || p.plan}</h3>
                    <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-medium uppercase tracking-wide text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                      {p.plan}
                    </span>
                  </div>
                  <div className="grid gap-3">
                    <NumberField
                      label="Price / month (USD)"
                      value={p.price_monthly}
                      min={0}
                      onChange={(v) => setField(i, "price_monthly", v)}
                    />
                    <NumberField
                      label="Max monitors"
                      value={p.max_monitors}
                      min={1}
                      onChange={(v) => setField(i, "max_monitors", v)}
                    />
                    <NumberField
                      label="Min interval (seconds)"
                      value={p.min_interval_seconds}
                      min={5}
                      onChange={(v) => setField(i, "min_interval_seconds", v)}
                    />
                    <NumberField
                      label="AI diagnoses / month"
                      value={p.monthly_diagnoses}
                      min={0}
                      onChange={(v) => setField(i, "monthly_diagnoses", v)}
                    />
                    <label className="block">
                      <span className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">Tagline</span>
                      <input
                        type="text"
                        value={p.tagline}
                        onChange={(e) => setTagline(i, e.target.value)}
                        placeholder="For a small team running real services."
                        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/30 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">
                        Feature bullets
                      </span>
                      <Textarea
                        rows={3}
                        value={(p.features ?? []).join("\n")}
                        onChange={(e) => setFeatures(i, e.target.value)}
                        placeholder={"All alert channels\nEmail support"}
                        className="text-sm"
                      />
                      <span className="mt-1 block text-xs text-slate-500 dark:text-slate-400">
                        One per line. The monitors, interval and AI-summary bullets are added automatically
                        from the numbers above — leave blank to use the defaults.
                      </span>
                    </label>
                  </div>
                </Card>
              ))}
            </div>
          </div>

          {/* Premium access allowlist */}
          <Card>
            <h2 className="text-lg font-semibold">Premium access</h2>
            <p className="mt-0.5 text-sm text-slate-500 dark:text-slate-400">
              Emails or domains that get the <span className="font-medium">Pro</span> tier for free — no
              billing. Use your own address or domain so {brand.name} is free for you and your team. One per
              line. A bare domain (e.g. <code className="rounded bg-slate-100 px-1 dark:bg-slate-800">workstation.co.uk</code>)
              covers every address at that domain and its subdomains; a full address matches just that person.
            </p>
            <div className="mt-4">
              <Label htmlFor="grants">Allowlist</Label>
              <Textarea
                id="grants"
                rows={5}
                value={grants}
                onChange={(e) => setGrants(e.target.value)}
                placeholder={"you@example.com\nworkstation.co.uk"}
                className="font-mono text-sm"
              />
            </div>
          </Card>

          <div className="flex items-center gap-3">
            <Button onClick={onSave} disabled={update.isPending}>
              {update.isPending ? "Saving…" : "Save changes"}
            </Button>
            {data?.updated_at && (
              <span className="text-xs text-slate-500 dark:text-slate-400">
                Last changed {new Date(data.updated_at).toLocaleString()}
              </span>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function NumberField({
  label,
  value,
  min,
  onChange,
}: {
  label: string;
  value: number;
  min?: number;
  onChange: (v: string) => void;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">{label}</span>
      <input
        type="number"
        min={min}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-base tabular-nums text-slate-900 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/30 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
      />
    </label>
  );
}
