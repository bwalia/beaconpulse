"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState, ReactNode } from "react";
import { AuthProvider } from "@/lib/auth";
import { initPwa } from "@/lib/pwa";

export function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            retry: 1,
            staleTime: 10_000,
            // A monitoring UI must never answer "is it up?" with a stale yes. Coming
            // back to the tab refetches instead of waiting out the remaining poll
            // interval, and a dropped connection resyncs the moment it returns —
            // both are moments the user is most likely to be checking after trouble.
            refetchOnWindowFocus: true,
            refetchOnReconnect: true,
          },
        },
      }),
  );

  // Service worker + install-prompt capture, once for the whole app.
  useEffect(() => initPwa(), []);

  return (
    <QueryClientProvider client={client}>
      <AuthProvider>{children}</AuthProvider>
    </QueryClientProvider>
  );
}
