"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { brand } from "@/brand";
import { useAuth } from "@/lib/auth";
import { socialButtonClass } from "@/components/auth/opsapi-button";

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

/** The official four-colour Google "G". */
function GoogleG() {
  return (
    <svg width="20" height="20" viewBox="0 0 48 48" aria-hidden="true" className="shrink-0">
      <path
        fill="#EA4335"
        d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
      />
      <path
        fill="#4285F4"
        d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
      />
      <path
        fill="#FBBC05"
        d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
      />
      <path
        fill="#34A853"
        d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
      />
    </svg>
  );
}

/**
 * "Continue with Google".
 *
 * GIS gives no way to fully theme its rendered button (Roboto font, capped width,
 * fixed radius), so it clashes with our app-font, full-width, rounded-lg form. We
 * therefore render OUR OWN button for the look, and lay Google's REAL button on
 * top of it invisibly (opacity-0) — scaled to cover the whole surface so a click
 * anywhere triggers Google's own, reliable credential flow (no One-Tap cooldown,
 * no synthetic clicks). The wrapper's focus-within ring keeps it keyboard-visible,
 * and GIS supplies the accessible name for screen readers.
 *
 * Renders nothing unless the active brand supplies a `googleClientId`.
 */
export function GoogleButton() {
  const clientId = brand.googleClientId;
  const { loginWithGoogle } = useAuth();
  const router = useRouter();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const gisRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  // Latest-value ref so the GIS callback — created once when the widget is built —
  // always reaches the current handlers without re-initializing on every render.
  const handlersRef = useRef({ loginWithGoogle, router });
  useEffect(() => {
    handlersRef.current = { loginWithGoogle, router };
  });

  useEffect(() => {
    if (!clientId) return;
    const wrapper = wrapperRef.current;
    const gis = gisRef.current;
    if (!wrapper || !gis) return;

    let cancelled = false;
    let ro: ResizeObserver | null = null;

    void loadGis()
      .then(() => {
        if (cancelled || !window.google) return;

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

        gis.replaceChildren();
        window.google.accounts.id.renderButton(gis, {
          type: "standard",
          theme: "outline",
          size: "large",
          text: "continue_with",
          shape: "rectangular",
          width: 400,
        });

        // Stretch the (invisible) real button to cover the visible one exactly, so
        // every click lands on Google's button. Re-runs when GIS finishes rendering
        // (gis resizes 0→actual) and whenever the form column resizes.
        const cover = () => {
          const sw = gis.offsetWidth;
          const sh = gis.offsetHeight;
          if (!sw || !sh) return;
          gis.style.transform = `scale(${wrapper.clientWidth / sw}, ${wrapper.clientHeight / sh})`;
        };
        ro = new ResizeObserver(cover);
        ro.observe(wrapper);
        ro.observe(gis);
        cover();
      })
      .catch(() => {
        if (!cancelled) setError("Google sign-in failed to load.");
      });

    return () => {
      cancelled = true;
      ro?.disconnect();
    };
  }, [clientId]);

  if (!clientId) return null;

  return (
    <div className="flex flex-col gap-2">
      <div ref={wrapperRef} className="relative rounded-lg focus-within:ring-2 focus-within:ring-brand-500 focus-within:ring-offset-2 dark:focus-within:ring-offset-slate-950">
        {/* What the user sees. Decorative: the real (overlaid) Google button owns
            the interaction and the accessible name. */}
        <div aria-hidden className={socialButtonClass}>
          <GoogleG />
          Continue with Google
        </div>
        {/* Google's real button — invisible, top-left anchored, scaled to cover. */}
        <div
          ref={gisRef}
          className="absolute left-0 top-0 origin-top-left overflow-hidden opacity-0"
        />
      </div>
      {error && (
        <p role="alert" className="text-sm font-medium text-red-600 dark:text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
