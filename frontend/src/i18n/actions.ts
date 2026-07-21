"use server";

import { cookies } from "next/headers";

import { LOCALE_COOKIE, resolveLocale } from "./config";

/**
 * Persist the chosen language.
 *
 * A server action rather than client-side document.cookie so the cookie is set with
 * HTTP semantics and the very next render already reads it — the switcher calls this and
 * refreshes, and the page comes back translated with no flash of the old language.
 */
export async function setLocale(value: string) {
  const locale = resolveLocale(value);
  const store = await cookies();
  store.set(LOCALE_COOKIE, locale, {
    path: "/",
    maxAge: 60 * 60 * 24 * 365, // a year — a language choice should outlast the session
    sameSite: "lax",
  });
}
