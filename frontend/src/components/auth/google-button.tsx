"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { brand } from "@/brand";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";

// Minimal shape of the Google Identity Services API we call. Declared inline so the
// feature adds no dependency and no @types package — GIS ships no bundled types.
interface GoogleCredentialResponse {
  credential: string;
}

interface GoogleAccountsId {
  initialize(config: {
    client_id: string;
    callback: (response: GoogleCredentialResponse) => void;
  }): void;
  renderButton(parent: HTMLElement, options: Record<string, unknown>): void;
}

declare global {
  interface Window {
    google?: { accounts: { id: GoogleAccountsId } };
  }
}

const GIS_SRC = "https://accounts.google.com/gsi/client";
const GIS_SCRIPT_ID = "google-identity-services";

/**
 * Load the GIS script exactly once for the whole app, resolving when
 * `window.google.accounts.id` is ready. The module-level promise is the double-load
 * guard: two buttons mounting at once share one injection, and a repeat mount reuses
 * the already-resolved promise rather than adding a second <script>.
 */
let gisPromise: Promise<void> | null = null;
function loadGis(): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  if (window.google?.accounts?.id) return Promise.resolve();
  if (gisPromise) return gisPromise;

  gisPromise = new Promise<void>((resolve, reject) => {
    const existing = document.getElementById(GIS_SCRIPT_ID);
    if (existing) {
      existing.addEventListener("load", () => resolve());
      existing.addEventListener("error", () => reject(new Error("GIS failed to load")));
      return;
    }
    const script = document.createElement("script");
    script.id = GIS_SCRIPT_ID;
    script.src = GIS_SRC;
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("GIS failed to load"));
    document.head.appendChild(script);
  });
  return gisPromise;
}

/**
 * "Continue with Google" — the official Google Identity Services button.
 *
 * Renders nothing unless the active brand supplies a `googleClientId`, so each
 * white-label opts in with its own Google app and un-configured brands show nothing.
 * On the credential callback it hands the ID token to the auth context, which stores
 * the tokens exactly like a password login before redirecting.
 */
export function GoogleButton() {
  const clientId = brand.googleClientId;
  const { loginWithGoogle } = useAuth();
  const router = useRouter();
  const [theme] = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  // Latest-value ref so the GIS callback — created once when the widget is built —
  // always reaches the current handlers without re-initializing the widget on every
  // render. Written in an effect (never during render) to stay React-Compiler-safe.
  const handlersRef = useRef({ loginWithGoogle, router });
  useEffect(() => {
    handlersRef.current = { loginWithGoogle, router };
  });

  useEffect(() => {
    if (!clientId) return;
    const container = containerRef.current;
    if (!container) return;

    let cancelled = false;

    void loadGis()
      .then(() => {
        if (cancelled || !window.google) return;
        // html.dark is the single source of truth (see lib/theme). For an explicit
        // choice trust `theme`; for "system" read the class the theme layer applied.
        const dark = theme === "dark" || (theme !== "light" && document.documentElement.classList.contains("dark"));

        window.google.accounts.id.initialize({
          client_id: clientId,
          callback: (response) => {
            setError(null);
            handlersRef.current
              .loginWithGoogle(response.credential)
              .then(() => handlersRef.current.router.replace("/dashboard"))
              .catch(() => setError("Google sign-in failed. Please try again."));
          },
        });

        // Clear before (re)rendering so a theme change replaces the button rather
        // than stacking a second one beneath it.
        container.replaceChildren();
        window.google.accounts.id.renderButton(container, {
          type: "standard",
          theme: dark ? "filled_black" : "outline",
          size: "large",
          text: "continue_with",
          shape: "pill",
          logo_alignment: "left",
        });
      })
      .catch(() => {
        if (!cancelled) setError("Google sign-in failed to load.");
      });

    return () => {
      cancelled = true;
    };
  }, [clientId, theme]);

  if (!clientId) return null;

  return (
    <div className="flex flex-col items-center gap-2">
      <div ref={containerRef} className="flex justify-center" />
      {error && (
        <p role="alert" className="text-sm font-medium text-red-600 dark:text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
