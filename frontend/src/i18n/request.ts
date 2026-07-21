import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import { LOCALE_COOKIE, resolveLocale } from "./config";

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
  return {
    locale,
    messages: (await import(`../messages/${locale}.json`)).default,
  };
});
