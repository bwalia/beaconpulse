"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { brand } from "@/brand";
import { BellIcon, CheckCircleIcon } from "@/components/icons";
import { Button, Card } from "@/components/ui";
import { api } from "@/lib/api";
import {
  currentSubscription,
  disablePush,
  enablePush,
  isIOS,
  isStandalone,
  promptInstall,
  pushSupported,
  useInstallPrompt,
  type WebPushConfig,
} from "@/lib/pwa";

type PushState = "loading" | "on" | "off" | "denied" | "needs-install" | "unsupported" | "unavailable";

// detect works out where this browser stands. Async (even its synchronous checks)
// so the effect below only ever sets state after an await.
async function detect(cfg: WebPushConfig): Promise<PushState> {
  if (!pushSupported()) return isIOS() && !isStandalone() ? "needs-install" : "unsupported";
  if (!cfg.enabled) return "unavailable";
  if (Notification.permission === "denied") return "denied";
  return (await currentSubscription(cfg)) ? "on" : "off";
}

/**
 * "Alerts on this device": install the app and turn on push notifications for this
 * browser. Each browser opts in separately; alerts then go to every browser anyone
 * in the org has turned on, through the auto-created "Browser push" channel.
 */
export function DeviceAlertsCard() {
  const qc = useQueryClient();
  const { data: cfg } = useQuery({
    queryKey: ["webpush-config"],
    queryFn: () => api.get<WebPushConfig>("/api/v1/devices/webpush"),
    staleTime: Infinity,
  });
  const install = useInstallPrompt();
  const [state, setState] = useState<PushState>("loading");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Client-only facts; read after mount so server and client render the same HTML.
  const [env, setEnv] = useState<{ standalone: boolean; ios: boolean } | null>(null);

  useEffect(() => {
    let live = true;
    Promise.resolve().then(() => live && setEnv({ standalone: isStandalone(), ios: isIOS() }));
    if (cfg) detect(cfg).then((s) => live && setState(s), () => live && setState("off"));
    return () => {
      live = false;
    };
  }, [cfg]);

  async function run(action: () => Promise<PushState>) {
    setBusy(true);
    setError(null);
    try {
      setState(await action());
      // The first browser to opt in creates the org's "Browser push" channel.
      qc.invalidateQueries({ queryKey: ["channels"] });
    } catch {
      setError("That didn't work. Check this site's notification permission in your browser and try again.");
    } finally {
      setBusy(false);
    }
  }

  const turnOn = () =>
    run(async () => {
      const permission = await enablePush(cfg!);
      return permission === "granted" ? "on" : permission === "denied" ? "denied" : "off";
    });
  const turnOff = () =>
    run(async () => {
      await disablePush();
      return "off";
    });

  return (
    <Card>
      <div className="flex items-start gap-3">
        <span className="mt-0.5 rounded-lg bg-brand-50 p-2 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300">
          <BellIcon className="h-5 w-5" />
        </span>
        <div className="min-w-0 flex-1 space-y-3">
          <div>
            <h2 className="font-semibold text-slate-900 dark:text-white">Alerts on this device</h2>
            <p className="text-sm text-slate-600 dark:text-slate-300">
              Get a notification the moment a monitor goes down, even with {brand.shortName} closed.
            </p>
          </div>

          {/* Install: offered when the browser supports it; iOS has no prompt, only a how-to. */}
          {env?.standalone ? (
            <p className="flex items-center gap-1.5 text-sm text-emerald-700 dark:text-emerald-400">
              <CheckCircleIcon className="h-4 w-4" /> Installed as an app
            </p>
          ) : install ? (
            <div className="flex flex-wrap items-center gap-3">
              <Button variant="secondary" onClick={() => promptInstall()}>
                Install {brand.shortName} app
              </Button>
              <span className="text-sm text-slate-500 dark:text-slate-400">Opens in its own window, from your dock or home screen.</span>
            </div>
          ) : env?.ios ? (
            <p className="text-sm text-slate-600 dark:text-slate-300">
              To install on iPhone or iPad: tap <span className="font-medium">Share</span>, then{" "}
              <span className="font-medium">Add to Home Screen</span>.
            </p>
          ) : null}

          <div className="flex flex-wrap items-center gap-3">
            {state === "loading" && <span className="text-sm text-slate-500 dark:text-slate-400">Checking notifications…</span>}
            {state === "off" && (
              <Button onClick={turnOn} disabled={busy}>
                {busy ? "Turning on…" : "Turn on notifications"}
              </Button>
            )}
            {state === "on" && (
              <>
                <span className="flex items-center gap-1.5 text-sm font-medium text-emerald-700 dark:text-emerald-400">
                  <CheckCircleIcon className="h-4 w-4" /> Notifications are on for this device
                </span>
                <Button variant="secondary" size="sm" onClick={turnOff} disabled={busy}>
                  Turn off
                </Button>
              </>
            )}
            {state === "denied" && (
              <p className="text-sm text-amber-700 dark:text-amber-400">
                Notifications are blocked for this site. Allow them in your browser&apos;s site settings, then reload.
              </p>
            )}
            {state === "needs-install" && (
              <p className="text-sm text-slate-600 dark:text-slate-300">
                On iPhone and iPad, notifications work once {brand.shortName} is on your Home Screen. Add it, open it from
                there, and turn them on here.
              </p>
            )}
            {state === "unsupported" && (
              <p className="text-sm text-slate-500 dark:text-slate-400">This browser doesn&apos;t support push notifications.</p>
            )}
            {state === "unavailable" && (
              <p className="text-sm text-slate-500 dark:text-slate-400">Browser notifications aren&apos;t set up on this server yet.</p>
            )}
          </div>
          {error && (
            <p role="alert" className="text-sm text-red-700 dark:text-red-400">
              {error}
            </p>
          )}
          {state === "on" && (
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Alerts reach every device where someone in your organization turned this on. Use{" "}
              <span className="font-medium">Send test</span> on the Browser push channel below to try it.
            </p>
          )}
        </div>
      </div>
    </Card>
  );
}
