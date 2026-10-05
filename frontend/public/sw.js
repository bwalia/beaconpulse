// Service worker: makes the app installable and shows alert push notifications.
//
// Deliberately NO fetch handler and no caching. A monitoring dashboard must never
// answer "is it up?" from a stale cache — an offline error is more honest than an
// old green screen — and skipping fetch interception keeps every request as fast
// as it is without a worker.

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

// Payload shape: { title, body, url, tag } — see backend/internal/adapter/notifier/webpush.go.
self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = { body: event.data ? event.data.text() : "" };
  }
  event.waitUntil(
    self.registration.showNotification(data.title || "Monitoring alert", {
      body: data.body || "",
      icon: "/pwa-icon/192",
      // One notification per monitor: a newer alert replaces the older one, and
      // renotify makes the replacement buzz again instead of updating silently.
      tag: data.tag || undefined,
      renotify: Boolean(data.tag),
      data: { url: data.url || "/dashboard" },
    }),
  );
});

// Clicking a notification focuses an open app window (navigating it to the alert's
// page) or opens a new one.
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = new URL(event.notification.data?.url || "/dashboard", self.location.origin).href;
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const open = windows.find((w) => new URL(w.url).origin === self.location.origin);
      if (open) {
        await open.focus();
        try {
          if (open.navigate) return await open.navigate(url);
        } catch {
          // Not controlled by this worker yet — fall through to a new window.
        }
      }
      return self.clients.openWindow(url);
    })(),
  );
});
