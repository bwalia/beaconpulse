"use client";

// Installable-app (PWA) and browser push helpers. The service worker is
// public/sw.js; the server side is POST/DELETE /api/v1/devices with
// platform "web" and GET /api/v1/devices/webpush for the VAPID key.

import { useSyncExternalStore } from "react";

import { api } from "@/lib/api";

/* ---------------- install prompt ---------------- */

type InstallPromptEvent = Event & { prompt: () => Promise<void> };

// Chromium fires beforeinstallprompt once per page load, often before the page
// that offers "Install" mounts — so it is captured app-wide and kept here.
let installEvent: InstallPromptEvent | null = null;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((f) => f());

/** Registers the service worker and captures the install prompt. Call once, app-wide. */
export function initPwa() {
  if (!("serviceWorker" in navigator)) return;
  navigator.serviceWorker.register("/sw.js").catch(() => {});
  window.addEventListener("beforeinstallprompt", (e) => {
    installEvent = e as InstallPromptEvent;
    emit();
  });
  window.addEventListener("appinstalled", () => {
    installEvent = null;
    emit();
  });
}

/** The pending install prompt, or null when the browser isn't offering one. */
export function useInstallPrompt() {
  return useSyncExternalStore(
    (f) => {
      listeners.add(f);
      return () => listeners.delete(f);
    },
    () => installEvent,
    () => null,
  );
}

export async function promptInstall() {
  const e = installEvent;
  installEvent = null;
  emit();
  await e?.prompt();
}

export const isStandalone = () =>
  matchMedia("(display-mode: standalone)").matches || (navigator as { standalone?: boolean }).standalone === true;

// iPadOS reports itself as a Mac; touch support gives it away.
export const isIOS = () =>
  /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);

/* ---------------- push ---------------- */

export type WebPushConfig = { enabled: boolean; public_key: string };

export const pushSupported = () => "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;

const toB64url = (buf: ArrayBuffer | null) =>
  buf
    ? btoa(String.fromCharCode(...new Uint8Array(buf)))
        .replace(/\+/g, "-")
        .replace(/\//g, "_")
        .replace(/=+$/, "")
    : "";

const fromB64url = (s: string) =>
  Uint8Array.from(atob(s.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));

async function registration() {
  await navigator.serviceWorker.register("/sw.js");
  return navigator.serviceWorker.ready; // subscribe() needs an active worker
}

async function subscribe(reg: ServiceWorkerRegistration, key: string) {
  const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: fromB64url(key) });
  const j = sub.toJSON();
  await api.post("/api/v1/devices", { token: j.endpoint, platform: "web", keys: j.keys });
  return sub;
}

/**
 * This browser's subscription, or null when push is off here. An existing one is
 * re-saved (the server prunes subscriptions a push service reports gone) and
 * renewed if the server's key changed since it was made.
 */
export async function currentSubscription(cfg: WebPushConfig): Promise<PushSubscription | null> {
  const reg = await registration();
  const sub = await reg.pushManager.getSubscription();
  if (!sub) return null;
  if (toB64url(sub.options.applicationServerKey) !== cfg.public_key) {
    await sub.unsubscribe();
    return Notification.permission === "granted" ? subscribe(reg, cfg.public_key) : null;
  }
  const j = sub.toJSON();
  await api.post("/api/v1/devices", { token: j.endpoint, platform: "web", keys: j.keys });
  return sub;
}

/** Asks for permission (must run from a click) and subscribes this browser. */
export async function enablePush(cfg: WebPushConfig): Promise<NotificationPermission> {
  const permission = await Notification.requestPermission();
  if (permission === "granted") await subscribe(await registration(), cfg.public_key);
  return permission;
}

/** Stops push on this browser: tells the server, then drops the subscription. Best-effort; also runs on sign-out. */
export async function disablePush() {
  if (!pushSupported()) return;
  const reg = await navigator.serviceWorker.getRegistration();
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) return;
  await api.delete("/api/v1/devices", { token: sub.endpoint }).catch(() => {});
  await sub.unsubscribe().catch(() => {});
}
