"use client";

import { motion } from "framer-motion";
import { useState } from "react";

import { useConfirm } from "@/components/confirm";
import { Button, Card } from "@/components/ui";
import { AlertTriangleIcon, CheckCircleIcon } from "@/components/icons";
import { ApiRequestError } from "@/lib/api";
import { useBilling, useDiagnose } from "@/lib/hooks";
import { useRevealVariants } from "@/lib/motion";
import type { BillingInfo, Diagnosis } from "@/lib/types";

/**
 * The result of an AI diagnosis.
 *
 * The measurements are shown, not just the prose, and that is the point rather than
 * decoration. The model is reading evidence it cannot verify and can be wrong; an
 * engineer mid-incident should be able to check the answer against what was actually
 * observed instead of taking it on faith. So the confidence is stated plainly, and
 * the raw findings sit right underneath, where a wrong conclusion is visibly wrong.
 */

const CONFIDENCE: Record<string, { label: string; className: string }> = {
  high: {
    label: "High confidence",
    className: "bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200",
  },
  medium: {
    label: "Medium confidence",
    className: "bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200",
  },
  low: {
    label: "Low confidence — check the evidence",
    className: "bg-slate-200 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
  },
};

function Row({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <div className="flex gap-3 py-1 text-sm">
      <span className="w-32 shrink-0 text-slate-500 dark:text-slate-400">{label}</span>
      <span className={`min-w-0 break-words font-mono text-xs ${bad ? "text-red-600 dark:text-red-400" : "text-slate-800 dark:text-slate-200"}`}>
        {value}
      </span>
    </div>
  );
}

function Section({ title, ok, children }: { title: string; ok: boolean | null; children: React.ReactNode }) {
  return (
    <div className="border-t border-slate-200 py-3 first:border-t-0 dark:border-slate-800">
      <p className="mb-1 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
        {title}
        {ok === true && <CheckCircleIcon className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />}
        {ok === false && <AlertTriangleIcon className="h-3.5 w-3.5 text-red-600 dark:text-red-400" />}
      </p>
      {children}
    </div>
  );
}

export function DiagnosePanel({ d }: { d: Diagnosis }) {
  const reveal = useRevealVariants();
  const { evidence: e, analysis } = d;
  const conf = analysis ? (CONFIDENCE[analysis.confidence] ?? CONFIDENCE.low) : null;

  return (
    <motion.div initial="hidden" animate="show" variants={reveal}>
      <Card className="mt-3 p-5">
        {analysis ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <h4 className="font-semibold text-slate-900 dark:text-white">{analysis.summary}</h4>
              {conf && (
                <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${conf.className}`}>
                  {conf.label}
                </span>
              )}
            </div>
            <dl className="mt-3 space-y-3 text-sm">
              <div>
                <dt className="font-medium text-slate-700 dark:text-slate-200">Likely cause</dt>
                <dd className="mt-0.5 text-slate-600 dark:text-slate-300">{analysis.likely_cause}</dd>
              </div>
              <div>
                <dt className="font-medium text-slate-700 dark:text-slate-200">Suggested fix</dt>
                <dd className="mt-0.5 whitespace-pre-line text-slate-600 dark:text-slate-300">
                  {analysis.suggested_fix}
                </dd>
              </div>
            </dl>
          </>
        ) : (
          <p className="flex items-start gap-2 text-sm text-amber-800 dark:text-amber-300">
            <AlertTriangleIcon className="mt-0.5 h-4 w-4 shrink-0" />
            {d.analysis_error ?? "No analysis was produced."}
          </p>
        )}

        {/* The receipts. Always shown, even with a confident answer above: this is
            what was actually measured, and it is the only part that cannot be
            hallucinated. */}
        <details className="mt-4 group">
          <summary className="cursor-pointer text-xs font-semibold uppercase tracking-wide text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200">
            What we measured
          </summary>
          <div className="mt-2 rounded-lg bg-slate-50 px-4 py-2 dark:bg-slate-900/60">
            <Section title="DNS" ok={e.dns.resolved}>
              <Row label="resolved" value={String(e.dns.resolved)} bad={!e.dns.resolved} />
              {e.dns.addresses?.length ? <Row label="addresses" value={e.dns.addresses.join(", ")} /> : null}
              {e.dns.cname ? <Row label="cname" value={e.dns.cname} /> : null}
              {e.dns.nameservers?.length ? <Row label="nameservers" value={e.dns.nameservers.join(", ")} /> : null}
              <Row label="lookup" value={`${e.dns.lookup_ms} ms`} />
              {e.dns.error ? <Row label="error" value={e.dns.error} bad /> : null}
            </Section>

            <Section title="TCP" ok={e.tcp.attempted ? e.tcp.connected : null}>
              {e.tcp.attempted ? (
                <>
                  <Row label="connected" value={String(e.tcp.connected)} bad={!e.tcp.connected} />
                  {e.tcp.address ? <Row label="address" value={e.tcp.address} /> : null}
                  <Row label="connect" value={`${e.tcp.connect_ms} ms`} />
                  {e.tcp.error ? <Row label="error" value={e.tcp.error} bad /> : null}
                </>
              ) : (
                <Row label="—" value="not attempted: an earlier step failed first" />
              )}
            </Section>

            <Section title="TLS" ok={e.tls.attempted ? e.tls.handshake_ok && !e.tls.expired && e.tls.hostname_ok : null}>
              {e.tls.attempted ? (
                <>
                  <Row label="handshake" value={String(e.tls.handshake_ok)} bad={!e.tls.handshake_ok} />
                  {e.tls.issuer ? <Row label="issuer" value={e.tls.issuer} /> : null}
                  {e.tls.subject ? <Row label="subject" value={e.tls.subject} /> : null}
                  {e.tls.not_after ? (
                    <Row
                      label="expires"
                      value={`${e.tls.not_after} (${e.tls.days_remaining} days)`}
                      bad={e.tls.expired || (e.tls.days_remaining ?? 99) < 14}
                    />
                  ) : null}
                  {e.tls.attempted && !e.tls.hostname_ok ? (
                    <Row label="hostname" value="the certificate does not cover this hostname" bad />
                  ) : null}
                  {e.tls.error ? <Row label="error" value={e.tls.error} bad /> : null}
                </>
              ) : (
                <Row label="—" value="not attempted: not a TLS target, or an earlier step failed first" />
              )}
            </Section>

            <Section title="HTTP" ok={e.http.attempted ? (e.http.status_code ?? 0) < 400 : null}>
              {e.http.attempted ? (
                <>
                  {e.http.status_code ? (
                    <Row label="status" value={String(e.http.status_code)} bad={e.http.status_code >= 400} />
                  ) : null}
                  <Row label="response" value={`${e.http.response_ms} ms`} />
                  {e.http.redirect_chain?.length ? (
                    <Row label="redirects" value={e.http.redirect_chain.join(" → ")} />
                  ) : null}
                  {e.http.server ? <Row label="server" value={e.http.server} /> : null}
                  {e.http.error ? <Row label="error" value={e.http.error} bad /> : null}
                </>
              ) : (
                <Row label="—" value="not attempted: not an HTTP target, or an earlier step failed first" />
              )}
            </Section>

            <p className="border-t border-slate-200 pt-2 text-[11px] text-slate-400 dark:border-slate-800">
              Probed {new Date(e.checked_at).toLocaleString()} · {e.target}
            </p>
          </div>
        </details>
      </Card>
    </motion.div>
  );
}

/** The Free-tier state: an honest description of what they'd get, not a nag. */
export function DiagnoseUpsell({ onDismiss }: { onDismiss: () => void }) {
  return (
    <Card className="mt-3 border-brand-200 p-5 dark:border-brand-900">
      <h4 className="font-semibold text-slate-900 dark:text-white">AI diagnosis is on paid plans</h4>
      <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
        We check DNS, the port, the TLS certificate and the response from our probes, then explain what
        broke and how to fix it. Add credit and it works by the hour — no subscription needed.
      </p>
      <div className="mt-4 flex gap-2">
        <Button onClick={() => (window.location.href = "/billing")}>Add credit</Button>
        <Button variant="secondary" onClick={onDismiss}>
          Not now
        </Button>
      </div>
    </Card>
  );
}

/**
 * isFailing decides where the button is offered at all. A "Diagnose" button on a
 * healthy monitor invites people to spend a model's time proving nothing is wrong.
 */
export function isFailing(m: { enabled: boolean; last_status: string }): boolean {
  return m.enabled && (m.last_status === "down" || m.last_status === "degraded");
}

/**
 * useDiagnoseControl owns the whole diagnose interaction — the request, the paid-plan
 * offer, the failure card — so every surface that offers the button behaves the same.
 * Two pages showing the same feature is exactly how one of them ends up quietly
 * rendering a 422 as a red error.
 */
export function useDiagnoseControl(monitorId: string) {
  const diagnose = useDiagnose();
  const confirm = useConfirm();
  const { data: billing } = useBilling();
  const [showUpsell, setShowUpsell] = useState(false);

  const run = async () => {
    setShowUpsell(false);

    // Say the price before taking it. A charge nobody agreed to is a surprise on a
    // statement, and "you never told me" is a fair complaint if the only place the
    // cost appears is after it has been spent. Free orgs skip straight through — the
    // server refuses them and the reply becomes the upsell, so quoting them a price
    // for something they cannot buy would just be noise.
    if (billing && billing.effective_plan !== "free") {
      const ok = await confirm({
        title: "Run AI diagnosis?",
        body: <DiagnoseCost info={billing} />,
        confirmLabel: "Run diagnosis",
      });
      if (!ok) return;
    }

    diagnose.mutate(monitorId, {
      // 422 is the paid-plan gate, not a failure: the server is saying the caller is
      // fine and the plan is not. That is an offer, not an error.
      onError: (err) => {
        if (err instanceof ApiRequestError && err.status === 422) setShowUpsell(true);
      },
    });
  };

  const panel = showUpsell ? (
    <DiagnoseUpsell onDismiss={() => setShowUpsell(false)} />
  ) : diagnose.data ? (
    <DiagnosePanel d={diagnose.data} />
  ) : diagnose.isError ? (
    <Card className="mt-3 p-4 text-sm text-red-700 dark:text-red-400">
      {diagnose.error instanceof ApiRequestError
        ? diagnose.error.message
        : "The diagnosis could not be run."}
    </Card>
  ) : null;

  return { run, isPending: diagnose.isPending, panel, label: diagnoseLabel(billing, diagnose.isPending) };
}


/**
 * What this click costs, stated before it is spent.
 *
 * The two plans are charged differently, so they are told different things — quoting
 * a subscriber a cash price for something their subscription already covers would be
 * worse than saying nothing. Both are told the refund rule, because "you are only
 * charged if we return a diagnosis" is the part that makes the price fair, and it is
 * true: a failed model refunds and is never recorded.
 */
function DiagnoseCost({ info }: { info: BillingInfo }) {
  const costMinutes = Math.round(info.diagnosis_cost_seconds / 60);
  const creditMinutes = Math.floor(info.credit_seconds / 60);

  if (info.effective_plan === "payg") {
    const enough = info.credit_seconds >= info.diagnosis_cost_seconds;
    return (
      <div className="space-y-2">
        <p>
          This uses{" "}
          <span className="font-semibold text-slate-900 dark:text-slate-100">
            {costMinutes} monitor-minutes
          </span>{" "}
          of your pay-as-you-go credit.
        </p>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          You have {creditMinutes.toLocaleString()} monitor-minutes left
          {!enough && " — that is not enough for a diagnosis, so this will be declined"}.
        </p>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          You are only charged if we return a diagnosis. If the analysis fails, the credit
          is refunded.
        </p>
      </div>
    );
  }

  const left = Math.max(0, info.monthly_diagnoses - info.diagnoses_used_this_month);
  return (
    <div className="space-y-2">
      <p>
        This uses{" "}
        <span className="font-semibold text-slate-900 dark:text-slate-100">1 of your {info.monthly_diagnoses}</span>{" "}
        AI diagnoses for this month. No extra charge — it is included in your plan.
      </p>
      <p className="text-sm text-slate-500 dark:text-slate-400">
        You have used {info.diagnoses_used_this_month} this month, so {left} remain. The
        allowance resets on the 1st.
      </p>
      <p className="text-sm text-slate-500 dark:text-slate-400">
        A failed analysis does not count against your allowance.
      </p>
    </div>
  );
}


/**
 * The button's own label. It carries the price so the cost is visible without having
 * to press anything and read a dialog — the dialog is where you consent, not where
 * you find out.
 */
export function diagnoseLabel(info: BillingInfo | undefined, pending: boolean): string {
  if (pending) return "Diagnosing…";
  if (!info || info.effective_plan === "free") return "Diagnose with AI";
  if (info.effective_plan === "payg") {
    return `Diagnose with AI · ${Math.round(info.diagnosis_cost_seconds / 60)} min credit`;
  }
  const left = Math.max(0, info.monthly_diagnoses - info.diagnoses_used_this_month);
  return `Diagnose with AI · ${left} left`;
}
