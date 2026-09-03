import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import { DEFAULT_LOCALE, LOCALE_COOKIE, resolveLocale } from "./config";

type Messages = Record<string, unknown>;

// English under the active locale, key by key, so a message missing from a
// half-translated locale renders its English text rather than the raw key. next-intl does
// NOT fall back across locales on its own — this merge is what makes that true, and it's
// what lets new copy ship in en.json alone and appear (in English) everywhere at once.
function withEnglishFallback(base: Messages, over: Messages): Messages {
  const out: Messages = { ...base };
  for (const [k, v] of Object.entries(over)) {
    const b = out[k];
    out[k] =
      b && v && typeof b === "object" && typeof v === "object" && !Array.isArray(b) && !Array.isArray(v)
        ? withEnglishFallback(b as Messages, v as Messages)
        : v;
  }
  return out;
}

// Runs per request (this app uses next-intl WITHOUT URL-based routing). The locale comes
// from a cookie the language switcher sets, defaulting to English. Reading the cookie
// makes pages dynamic rather than statically cached — an accepted trade for not moving
// every route under an [locale] segment, and immaterial for a dashboard that renders per
// user anyway.
//
// A missing translation for a key falls back to English (onError/getMessageFallback
// defaults), so a half-translated language shows English for the gaps rather than an
// error — which is what makes shipping "the labels, not yet the prose" safe.
export default getRequestConfig(async () => {
  const store = await cookies();
  const locale = resolveLocale(store.get(LOCALE_COOKIE)?.value);
  const messages = (await import(`../messages/${locale}.json`)).default as Messages;
  if (locale === DEFAULT_LOCALE) {
    return { locale, messages };
  }
  const english = (await import(`../messages/${DEFAULT_LOCALE}.json`)).default as Messages;
  return { locale, messages: withEnglishFallback(english, messages) };
});
